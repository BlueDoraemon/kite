// Package openai implements a Kite provider for any OpenAI-compatible
// chat completions API. It speaks the wire format only; the core types stay
// in package kite.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/BlueDoraemon/kite/internal/kite"
)

// retryableStatuses are HTTP status codes that warrant a retry with backoff.
var retryableStatuses = map[int]bool{
	http.StatusTooManyRequests: true,
	http.StatusInternalServerError: true,
	http.StatusBadGateway:         true,
	http.StatusServiceUnavailable: true,
	http.StatusGatewayTimeout:     true,
}

// defaultTimeout bounds a single chat completion request.
const defaultTimeout = 5 * time.Minute

// Provider talks to an OpenAI-compatible chat completions endpoint.
type Provider struct {
	// BaseURL is the API root, for example "https://api.openai.com/v1".
	BaseURL string
	// APIKey is sent as the Bearer token. May be empty for local servers.
	APIKey string
	// Model is the model identifier to request.
	Model string
	// HTTPClient is used for requests. Defaults to http.DefaultClient.
	HTTPClient *http.Client
	// Timeout bounds a single chat completion request. Zero means 5 minutes.
	Timeout time.Duration
	// MaxRetries is how many times a retryable failure is retried with
	// exponential backoff. Zero means no retries.
	MaxRetries int
}

// New returns a Provider configured for the given base URL, key, and model.
func New(baseURL, apiKey, model string) *Provider {
	return &Provider{BaseURL: baseURL, APIKey: apiKey, Model: model}
}

// Complete sends the session and the available tools to the model and
// returns its reply.
func (p *Provider) Complete(ctx context.Context, session *kite.Session, tools []kite.Tool) (kite.Reply, error) {
	client := p.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	timeout := p.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	var lastErr error
	for attempt := 0; attempt <= p.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(1<<(attempt-1)) * time.Second
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return kite.Reply{}, ctx.Err()
			}
		}

		// Bound each attempt so a stalled endpoint cannot hang the loop,
		// and rebuild the request per attempt so the body can be replayed.
		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		req, err := p.buildRequest(attemptCtx, session, tools)
		if err != nil {
			cancel()
			return kite.Reply{}, err
		}
		resp, err := client.Do(req)
		cancel()
		if err != nil {
			lastErr = fmt.Errorf("openai: request: %w", err)
			// A cancelled or expired context is the caller's signal to
			// stop; everything else is a transport failure worth
			// retrying.
			if ctx.Err() != nil {
				return kite.Reply{}, ctx.Err()
			}
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<24))
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("openai: read response: %w", readErr)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("openai: status %s: %s", resp.Status, truncate(string(body), 500))
			if !retryableStatuses[resp.StatusCode] {
				return kite.Reply{}, lastErr
			}
			continue
		}

		var chat chatResponse
		if err := json.Unmarshal(body, &chat); err != nil {
			return kite.Reply{}, fmt.Errorf("openai: decode response: %w", err)
		}
		if len(chat.Choices) == 0 {
			return kite.Reply{}, fmt.Errorf("openai: no choices in response")
		}
		return replyFromChoice(chat.Choices[0])
	}
	return kite.Reply{}, lastErr
}

func (p *Provider) buildRequest(ctx context.Context, session *kite.Session, tools []kite.Tool) (*http.Request, error) {
	payload := chatRequest{
		Model:    p.Model,
		Messages: toWireMessages(session.Messages),
		Tools:    toWireTools(tools),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("openai: encode request: %w", err)
	}

	url := strings.TrimRight(p.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	return req, nil
}

// Wire types for the chat completions API. They live here, not in package
// kite, because they are provider-specific.

type chatRequest struct {
	Model      string        `json:"model"`
	Messages   []wireMessage `json:"messages"`
	Tools      []wireTool    `json:"tools,omitempty"`
	ToolChoice string        `json:"tool_choice,omitempty"`
}

type wireMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
}

type wireTool struct {
	Type     string       `json:"type"`
	Function wireFunction `json:"function"`
}

type wireFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type wireToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function wireCallFunc `json:"function"`
}

type wireCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
}

type chatChoice struct {
	Message wireMessage `json:"message"`
}

func toWireTools(tools []kite.Tool) []wireTool {
	out := make([]wireTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, wireTool{
			Type: "function",
			Function: wireFunction{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Schema(),
			},
		})
	}
	return out
}

func toWireMessages(msgs []kite.Message) []wireMessage {
	out := make([]wireMessage, 0, len(msgs))
	for _, m := range msgs {
		w := wireMessage{Role: string(m.Role), Content: m.Content, ToolCallID: m.ToolCallID}
		if len(m.ToolCalls) > 0 {
			w.ToolCalls = toWireToolCalls(m.ToolCalls)
		}
		out = append(out, w)
	}
	return out
}

func toWireToolCalls(calls []kite.ToolCall) []wireToolCall {
	out := make([]wireToolCall, 0, len(calls))
	for _, c := range calls {
		out = append(out, wireToolCall{
			ID:   c.ID,
			Type: "function",
			Function: wireCallFunc{
				Name:      c.Name,
				Arguments: c.Input,
			},
		})
	}
	return out
}

func replyFromChoice(c chatChoice) (kite.Reply, error) {
	r := kite.Reply{Text: c.Message.Content}
	for _, tc := range c.Message.ToolCalls {
		r.ToolCalls = append(r.ToolCalls, kite.ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: tc.Function.Arguments,
		})
	}
	return r, nil
}

// truncate cuts s to at most n bytes without splitting a UTF-8 rune.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := s[:n]
	for len(cut) > 0 && !utf8.RuneStart(cut[len(cut)-1]) {
		cut = cut[:len(cut)-1]
	}
	return cut + "..."
}

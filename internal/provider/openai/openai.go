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

	"github.com/BlueDoraemon/kite/internal/kite"
)

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
}

// New returns a Provider configured for the given base URL, key, and model.
func New(baseURL, apiKey, model string) *Provider {
	return &Provider{BaseURL: baseURL, APIKey: apiKey, Model: model}
}

// Complete sends the session and the available tools to the model and
// returns its reply.
func (p *Provider) Complete(ctx context.Context, session *kite.Session, tools []kite.Tool) (kite.Reply, error) {
	req, err := p.buildRequest(ctx, session, tools)
	if err != nil {
		return kite.Reply{}, err
	}
	client := p.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return kite.Reply{}, fmt.Errorf("openai: request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<24))
	if err != nil {
		return kite.Reply{}, fmt.Errorf("openai: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return kite.Reply{}, fmt.Errorf("openai: status %s: %s", resp.Status, truncate(string(body), 500))
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

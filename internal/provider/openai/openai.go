// Package openai implements a Kite provider for any OpenAI-compatible
// chat completions API, streaming via Server-Sent Events. It speaks the wire
// format only; the core types stay in package core.
package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/BlueDoraemon/kite-core/internal/core"
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
	// Timeout bounds a single chat completion request. Zero means 5 minutes.
	Timeout time.Duration
}

// New returns a Provider configured for the given base URL, key, and model.
func New(baseURL, apiKey, model string) *Provider {
	return &Provider{BaseURL: baseURL, APIKey: apiKey, Model: model}
}

// Complete sends the session and the available tools to the model and
// streams the reply back as events.
func (p *Provider) Complete(ctx context.Context, session *core.Session, tools []core.Tool, onEvent func(core.ProviderEvent)) error {
	req, err := p.buildRequest(ctx, session, tools)
	if err != nil {
		return err
	}
	client := p.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("openai: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return &core.Error{Code: "http_" + fmt.Sprint(resp.StatusCode), Message: truncate(string(body), 500)}
	}

	// Stream the SSE response.
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)
	var (
		textBuf strings.Builder
		calls   map[int]*pendingCall
		usage   *core.Usage
	)
	calls = make(map[int]*pendingCall)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimPrefix(line, "data:")
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			break
		}
		var chunk chatChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return &core.Error{Code: "malformed_sse", Message: "malformed SSE chunk"}
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		if choice.Delta != nil {
			if choice.Delta.Content != "" {
				textBuf.WriteString(choice.Delta.Content)
				onEvent(core.ProviderEvent{Text: choice.Delta.Content})
			}
			for _, tc := range choice.Delta.ToolCalls {
				pc := calls[tc.Index]
				if pc == nil {
					pc = &pendingCall{index: tc.Index}
					calls[tc.Index] = pc
				}
				if tc.ID != "" {
					pc.id = tc.ID
				}
				if tc.Function.Name != "" {
					pc.name = tc.Function.Name
				}
				pc.args.WriteString(tc.Function.Arguments)
			}
		}
		if chunk.Usage != nil {
			usage = &core.Usage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}
	}
	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return &core.Error{Code: "stream", Message: "stream read failed"}
	}

	// Emit completed tool calls.
	for _, pc := range calls {
		call := &core.ToolCall{
			ID:    pc.id,
			Name:  pc.name,
			Input: pc.args.String(),
		}
		onEvent(core.ProviderEvent{ToolCall: call})
	}
	if usage != nil {
		onEvent(core.ProviderEvent{Usage: usage})
	}
	onEvent(core.ProviderEvent{Done: true})
	return nil
}

// pendingCall accumulates a fragmented tool call.
type pendingCall struct {
	index int
	id    string
	name  string
	args  strings.Builder
}

func (p *Provider) buildRequest(ctx context.Context, session *core.Session, tools []core.Tool) (*http.Request, error) {
	payload := chatRequest{
		Model:    p.Model,
		Messages: toWireMessages(session.BuildContext()),
		Tools:    toWireTools(tools),
		Stream:   true,
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
	req.Header.Set("Accept", "text/event-stream")
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	return req, nil
}

// Wire types for the chat completions API. They live here, not in package
// core, because they are provider-specific.

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []wireMessage `json:"messages"`
	Tools    []wireTool    `json:"tools,omitempty"`
	Stream   bool          `json:"stream"`
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

// chatChunk is one SSE data payload.
type chatChunk struct {
	Choices []chatChoice `json:"choices"`
	Usage   *chatUsage   `json:"usage"`
}

type chatChoice struct {
	Delta *chatDelta `json:"delta"`
}

type chatDelta struct {
	Content    string        `json:"content,omitempty"`
	ToolCalls []chatToolCall `json:"tool_calls,omitempty"`
}

type chatToolCall struct {
	Index    int          `json:"index"`
	ID      string       `json:"id"`
	Function chatCallFunc `json:"function"`
}

type chatCallFunc struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func toWireTools(tools []core.Tool) []wireTool {
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

func toWireMessages(msgs []core.Message) []wireMessage {
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

func toWireToolCalls(calls []core.ToolCall) []wireToolCall {
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

// truncate cuts s to at most n bytes without splitting a UTF-8 rune.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := s[:n]
	for len(cut) > 0 && !utf8RuneStart(cut[len(cut)-1]) {
		cut = cut[:len(cut)-1]
	}
	return cut + "..."
}

// utf8RuneStart reports whether b is the first byte of a UTF-8 rune.
func utf8RuneStart(b byte) bool {
	return b&0xC0 != 0x80
}

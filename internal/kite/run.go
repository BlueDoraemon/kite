package kite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ErrNoTool is returned when the model asks for a tool that is not registered.
var ErrNoTool = errors.New("unknown tool")

// Run drives the agent loop: send the session to the provider, run any tool
// calls the model makes, feed the results back, and repeat until the model
// finishes or the context is cancelled.
func Run(ctx context.Context, provider Provider, opts RunOptions) error {
	if opts.Session == nil {
		return errors.New("kite: nil session")
	}
	if provider == nil {
		return errors.New("kite: nil provider")
	}
	if opts.Stdout == nil {
		opts.Stdout = io.Discard
	}
	emit := opts.OnEvent
	if emit == nil {
		emit = func(Event) error { return nil }
	}

	turns := 0
	for {
		if opts.MaxTurns > 0 && turns >= opts.MaxTurns {
			return errors.New("kite: max turns reached")
		}
		turns++

		reply, err := provider.Complete(ctx, opts.Session, opts.Tools)
		if err != nil {
			return fmt.Errorf("kite: provider: %w", err)
		}

		// Record the assistant message with its text and tool calls, then
		// run each call and feed its result back so the provider can pair
		// them with their calls.
		assistant := Message{Role: RoleAssistant, Content: reply.Text, ToolCalls: reply.ToolCalls}
		opts.Session.Messages = append(opts.Session.Messages, assistant)
		if reply.Text != "" {
			if err := emit(Event{Type: "message", Message: &assistant}); err != nil {
				return err
			}
			if opts.Print {
				fmt.Fprintln(opts.Stdout, reply.Text)
			}
		}

		if len(reply.ToolCalls) == 0 {
			if reply.Finish {
				if err := emit(Event{Type: "finish", Text: reply.FinishText}); err != nil {
					return err
				}
				if opts.Print && reply.FinishText != "" {
					fmt.Fprintln(opts.Stdout, reply.FinishText)
				}
			}
			return nil
		}

		for _, call := range reply.ToolCalls {
			if err := emit(Event{Type: "tool_call", Call: &call}); err != nil {
				return err
			}
			if opts.Print {
				fmt.Fprintf(opts.Stdout, "[%s] %s\n", call.Name, call.Input)
			}

			tool := findTool(opts.Tools, call.Name)
			var output string
			var runErr error
			if tool == nil {
				runErr = fmt.Errorf("%w: %s", ErrNoTool, call.Name)
			} else {
				output, runErr = tool.Run(ctx, call.Input)
			}
			if runErr != nil {
				output = fmt.Sprintf("error: %v", runErr)
			}

			result := &ToolResult{CallID: call.ID, Output: output}
			opts.Session.Messages = append(opts.Session.Messages, Message{
				Role:       RoleTool,
				Content:    output,
				ToolCallID: call.ID,
			})
			if err := emit(Event{Type: "tool_result", Result: result}); err != nil {
				return err
			}
			if opts.Print {
				fmt.Fprintln(opts.Stdout, output)
			}
		}
	}
}

func findTool(tools []Tool, name string) Tool {
	for _, t := range tools {
		if t.Name() == name {
			return t
		}
	}
	return nil
}

// JSONInput is a helper for tools that accept a JSON object of arguments.
type JSONInput map[string]json.RawMessage

// ParseInput decodes a tool input string into a map of named arguments.
func ParseInput(input string) (JSONInput, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(input), &m); err != nil {
		return nil, fmt.Errorf("invalid JSON input: %w", err)
	}
	return m, nil
}

// String returns the string value of an argument, or def if absent.
func (m JSONInput) String(key, def string) string {
	if raw, ok := m[key]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	}
	return def
}

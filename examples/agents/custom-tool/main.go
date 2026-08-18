// Command custom-tool demonstrates how to implement a custom kite.Tool and
// pass it to a session.
//
// Usage:
//
//	go run ./examples/agents/custom-tool
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/BlueDoraemon/kite-core"
)

// upperTool is a custom tool that uppercases its input.
type upperTool struct{}

func (upperTool) Name() string        { return "upper" }
func (upperTool) Description() string { return "Uppercase the input text." }
func (upperTool) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{"type": "string", "description": "The text to uppercase."},
		},
		"required": []string{"text"},
	}
}
func (upperTool) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}
	return strings.ToUpper(args.Text), nil
}

type exampleProvider struct{ turn int }

func (p *exampleProvider) Complete(_ context.Context, _ *kite.Session, _ []kite.Tool, onEvent func(kite.ProviderEvent)) error {
	p.turn++
	if p.turn == 1 {
		onEvent(kite.ProviderEvent{ToolCall: &kite.ToolCall{ID: "call_1", Name: "upper", Input: `{"text":"hello"}`}})
	} else {
		onEvent(kite.ProviderEvent{Text: "The tool returned HELLO."})
	}
	onEvent(kite.ProviderEvent{Done: true})
	return nil
}

func main() {
	provider := &exampleProvider{}
	sess, err := kite.NewSession(kite.Config{
		Provider:   provider,
		Model:      "example",
		WorkingDir: ".",
		// A custom tool on its own; nil Tools installs the built-ins.
		Tools: []kite.Tool{upperTool{}},
	})
	if err != nil {
		panic(err)
	}

	ch, err := sess.Prompt(context.Background(), "Call the upper tool with the text 'hello'.")
	if err != nil {
		panic(err)
	}
	for ev := range ch {
		if ev.Type == kite.EventTextDelta {
			fmt.Print(ev.Payload.(*kite.TextDeltaPayload).Text)
		}
	}
	fmt.Println()
}

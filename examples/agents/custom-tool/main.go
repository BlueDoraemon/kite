// Command custom-tool demonstrates how to implement a custom kite.Tool and
// pass it to a session.
//
// Usage:
//
//	KITE_API_KEY=sk-... go run ./examples/agents/custom-tool
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/BlueDoraemon/kite-core"
	"github.com/BlueDoraemon/kite-core/internal/provider/openai"
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

func main() {
	apiKey := os.Getenv("KITE_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "set KITE_API_KEY to run this example")
		os.Exit(2)
	}
	provider := openai.New(
		envOr("KITE_BASE_URL", "https://api.openai.com/v1"),
		apiKey,
		envOr("KITE_MODEL", "gpt-4o-mini"),
	)

	sess, err := kite.NewSession(kite.Config{
		Provider:   provider,
		Model:      provider.Model,
		WorkingDir: ".",
		// A custom tool on its own; nil Tools installs the built-ins.
		Tools: []kite.Tool{upperTool{}},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		os.Exit(2)
	}

	ch, err := sess.Prompt(context.Background(), "Call the upper tool with the text 'hello'.")
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		os.Exit(2)
	}
	for ev := range ch {
		if ev.Type == kite.EventTextDelta {
			fmt.Print(ev.Payload.(*kite.TextDeltaPayload).Text)
		}
	}
	fmt.Println()
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

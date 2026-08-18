// Command go-session demonstrates the Kite Go API: create a session,
// drive a prompt, and read the resulting events.
//
// Usage:
//
//	KITE_API_KEY=sk-... go run ./examples/agents/go-session
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/BlueDoraemon/kite-core"
	"github.com/BlueDoraemon/kite-core/internal/provider/openai"
)

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
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		os.Exit(2)
	}

	ch, err := sess.Prompt(context.Background(), "Reply with exactly the word ok.")
	if err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		os.Exit(2)
	}

	for ev := range ch {
		switch ev.Type {
		case kite.EventTextDelta:
			fmt.Print(ev.Payload.(*kite.TextDeltaPayload).Text)
		case kite.EventSessionCompleted:
			res := ev.Payload.(*kite.SessionCompletedPayload).Result
			fmt.Printf("\nstatus: %s\n", res.Status)
		case kite.EventSessionFailed:
			e := ev.Payload.(*kite.SessionFailedPayload).Error
			fmt.Fprintln(os.Stderr, "failed:", e.Message)
			os.Exit(1)
		}
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

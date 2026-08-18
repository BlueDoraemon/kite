// Command go-session demonstrates the Kite Go API: create a session,
// drive a prompt, and read the resulting events.
//
// Usage:
//
//	go run ./examples/agents/go-session
package main

import (
	"context"
	"fmt"

	"github.com/BlueDoraemon/kite-core"
)

type exampleProvider struct{}

func (exampleProvider) Complete(_ context.Context, _ *kite.Session, _ []kite.Tool, onEvent func(kite.ProviderEvent)) error {
	onEvent(kite.ProviderEvent{Text: "ok"})
	onEvent(kite.ProviderEvent{Done: true})
	return nil
}

func main() {
	sess, err := kite.NewSession(kite.Config{
		Provider:   exampleProvider{},
		Model:      "example",
		WorkingDir: ".",
	})
	if err != nil {
		panic(err)
	}

	ch, err := sess.Prompt(context.Background(), "Reply with exactly the word ok.")
	if err != nil {
		panic(err)
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
			panic(e)
		}
	}
}

// Command schemagen regenerates the versioned JSON schemas under
// docs/schemas/v1. It fails if the committed schemas differ from what the
// generator produces.
package main

import (
	"fmt"
	"os"

	"github.com/BlueDoraemon/kite-core/internal/schemagen"
)

func main() {
	dir := "docs/schemas/v1"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	written, err := schemagen.Generate(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "schemagen:", err)
		os.Exit(1)
	}
	for _, w := range written {
		fmt.Println("wrote", w)
	}
	// Verify the committed schemas match (idempotent generation).
	if err := schemagen.Check(dir); err != nil {
		fmt.Fprintln(os.Stderr, "schemagen:", err)
		os.Exit(1)
	}
}

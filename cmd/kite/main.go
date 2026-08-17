// Command kite is a minimal command-line agent that can explain and modify a
// repository. It drives a model through an OpenAI-compatible API and gives it
// read, edit, and bash tools to work with the current directory.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/BlueDoraemon/kite/internal/kite"
	"github.com/BlueDoraemon/kite/internal/provider/openai"
	"github.com/BlueDoraemon/kite/internal/tools"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "kite:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("kite", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		baseURL = fs.String("base-url", envOr("KITE_BASE_URL", "https://api.openai.com/v1"), "OpenAI-compatible API base URL")
		model   = fs.String("model", envOr("KITE_MODEL", "gpt-4o-mini"), "model to use")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) < 2 || rest[0] != "run" {
		return fmt.Errorf("usage: kite run <prompt> [flags]")
	}
	prompt := strings.Join(rest[1:], " ")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	provider := openai.New(*baseURL, os.Getenv("KITE_API_KEY"), *model)
	toolSet := &tools.Set{Dir: dir}

	session := &kite.Session{
		Model: *model,
		Messages: []kite.Message{
			{Role: kite.RoleUser, Content: prompt},
		},
	}

	err = kite.Run(ctx, provider, kite.RunOptions{
		Session: session,
		Tools:   toolSet.All(),
		Stdout:  os.Stdout,
		Print:   true,
	})
	if err != nil {
		return err
	}

	// The prompt may have asked for a change; report what the working tree
	// looks like so the user can review it.
	if diff := gitDiff(dir); diff != "" {
		fmt.Fprintln(os.Stdout, "\n--- working tree diff ---")
		fmt.Fprintln(os.Stdout, diff)
	}
	return nil
}

func gitDiff(dir string) string {
	cmd := exec.Command("git", "diff")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return string(out)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

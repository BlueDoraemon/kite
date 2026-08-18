package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// BashTimeout is how long a command may run before it is killed, along with
// any processes it started.
const BashTimeout = 30 * time.Second

// maxBashOutput caps how much of a command's output is returned to the model.
const maxBashOutput = 32 * 1024

// Bash returns a Tool that runs a shell command in the working directory.
// The command runs with a 30 second timeout and its process tree is killed
// on timeout.
func (s *Set) Bash() *Tool {
	return &Tool{
		name:        "bash",
		description: "Run a shell command in the repository working directory. The command runs with a 30 second timeout. Set purpose to \"verification\" to mark a verification run.",
		specs: []argSpec{
			{name: "command", typ: "string", desc: "The shell command to run.", required: true},
			{name: "working_dir", typ: "string", desc: "Optional relative working directory for the command."},
			{name: "purpose", typ: "string", desc: "Optional purpose; \"verification\" marks a verification run."},
		},
		run: func(ctx context.Context, args map[string]any) (string, error) {
			command := str(args, "command")
			wd := str(args, "working_dir")
			dir := s.Dir
			if wd != "" {
				resolved, err := s.resolve(wd)
				if err != nil {
					return "", err
				}
				dir = resolved
			}
			return runShell(ctx, dir, command)
		},
	}
}

// runShell runs a shell command and returns its combined output.
func runShell(ctx context.Context, dir, command string) (string, error) {
	cmd := newShellCommand(ctx, dir, command)
	// The tool's own timeout bounds runaway commands even when the caller's
	// context has no deadline.
	timer := time.AfterFunc(BashTimeout, func() {
		killProcessTree(cmd)
	})
	defer timer.Stop()
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if len(out) > maxBashOutput {
		out = out[:maxBashOutput]
		out = append(out, []byte("\n... output truncated ...")...)
	}
	result := string(out)
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return fmt.Sprintf("exit status %d\n%s", ee.ExitCode(), result), nil
		}
		return "", err
	}
	if strings.TrimSpace(result) == "" {
		return "(no output)", nil
	}
	return result, nil
}

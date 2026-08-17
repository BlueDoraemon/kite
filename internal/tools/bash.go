package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// maxBashOutput caps how much of a command's output is returned to the model.
const maxBashOutput = 32 * 1024

// Bash returns a Tool that runs a shell command in the working directory.
func (s *Set) Bash() *Tool {
	return &Tool{
		name:        "bash",
		description: "Run a shell command in the repository working directory. The command runs with a 30 second timeout.",
		specs: []argSpec{
			{name: "command", typ: "string", desc: "The shell command to run.", required: true},
		},
		run: func(ctx context.Context, args map[string]any) (string, error) {
			command := str(args, "command")
			cmd := exec.CommandContext(ctx, "sh", "-c", command)
			cmd.Dir = s.Dir
			// The tool's own timeout bounds runaway commands even when
			// the caller's context has no deadline.
			timer := time.AfterFunc(30*time.Second, func() {
				cmd.Process.Kill()
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
		},
	}
}

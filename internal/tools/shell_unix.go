//go:build !windows

package tools

import (
	"context"
	"os/exec"
	"syscall"
)

// newShellCommand builds a shell command for POSIX systems using sh -c.
func newShellCommand(ctx context.Context, dir, command string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	// Put the command in its own process group so the timeout can kill the
	// whole tree, not just the shell.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// killProcessTree kills the command and its children by killing the process
// group (negative pid).
func killProcessTree(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

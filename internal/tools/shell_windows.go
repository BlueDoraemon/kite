//go:build windows

package tools

import (
	"context"
	"os/exec"
)

// newShellCommand builds a shell command for Windows using cmd.exe /C.
func newShellCommand(ctx context.Context, dir, command string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "cmd.exe", "/C", command)
	cmd.Dir = dir
	return cmd
}

// killProcessTree kills the command and its children on Windows.
func killProcessTree(cmd *exec.Cmd) {
	if cmd.Process != nil {
		// taskkill /T kills the process tree.
		kill := exec.Command("taskkill", "/PID", itoa(cmd.Process.Pid), "/T", "/F")
		_ = kill.Run()
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

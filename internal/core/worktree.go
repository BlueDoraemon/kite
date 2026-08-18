package core

import (
	"encoding/json"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// snapshotWorktree records the current worktree state so changes can be
// determined relative to it. It returns the set of paths that are already
// modified or untracked at session start.
func snapshotWorktree(dir string) map[string]bool {
	if dir == "" {
		return map[string]bool{}
	}
	cmd := exec.Command("git", "status", "--porcelain", "-z")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]bool{}
	}
	snap := map[string]bool{}
	for _, entry := range strings.Split(string(out), "\x00") {
		if entry == "" {
			continue
		}
		// Format: XY path, or XY path -> path for renames.
		fields := strings.SplitN(entry, " ", 2)
		if len(fields) == 2 {
			snap[fields[1]] = true
		}
	}
	return snap
}

// diffWorktree returns the files changed relative to the baseline snapshot
// (including modifications to files that were already dirty), and whether the
// list is complete (true inside a Git repository).
func diffWorktree(dir string, baseline map[string]bool) ([]string, bool) {
	if dir == "" {
		return nil, false
	}
	cmd := exec.Command("git", "status", "--porcelain", "-z")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Not a Git repository: the list would be incomplete.
		return nil, false
	}
	changed := map[string]bool{}
	for _, entry := range strings.Split(string(out), "\x00") {
		if entry == "" {
			continue
		}
		fields := strings.SplitN(entry, " ", 2)
		if len(fields) == 2 {
			changed[fields[1]] = true
		}
	}
	// Include paths that were dirty at baseline and are still dirty.
	for p := range baseline {
		changed[p] = true
	}
	out2 := make([]string, 0, len(changed))
	for p := range changed {
		out2 = append(out2, p)
	}
	sort.Strings(out2)
	return out2, true
}

// recordChangedFiles tracks files changed by an edit tool call.
func recordChangedFiles(changed map[string]bool, _ string, tool, input string) {
	if tool != "edit" {
		return
	}
	var args struct {
		Path string `json:"path"`
	}
	if json.Unmarshal([]byte(input), &args) == nil && args.Path != "" {
		changed[args.Path] = true
	}
}

// isVerification reports whether a bash tool call has purpose "verification".
func isVerification(input string) bool {
	var args struct {
		Purpose string `json:"purpose"`
	}
	if json.Unmarshal([]byte(input), &args) != nil {
		return false
	}
	return args.Purpose == "verification"
}

// verificationCommand extracts the command from a verification bash call.
func verificationCommand(input string) string {
	var args struct {
		Command string `json:"command"`
	}
	if json.Unmarshal([]byte(input), &args) != nil {
		return ""
	}
	return args.Command
}

// exitCodeFromOutput parses the exit code from a bash tool result. The bash
// tool reports a non-zero exit as "exit status N\n<output>"; a result without
// that prefix is treated as exit code 0.
func exitCodeFromOutput(output string) int {
	if !strings.HasPrefix(output, "exit status ") {
		return 0
	}
	rest := strings.TrimPrefix(output, "exit status ")
	code := 1
	if i := strings.IndexByte(rest, '\n'); i >= 0 {
		rest = rest[:i]
	}
	if n, err := strconv.Atoi(strings.TrimSpace(rest)); err == nil {
		code = n
	}
	return code
}

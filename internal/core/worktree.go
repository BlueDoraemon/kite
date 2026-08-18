package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type worktreeSnapshot struct {
	complete bool
	paths    map[string]string
}

func snapshotWorktree(dir string) worktreeSnapshot {
	snap := worktreeSnapshot{paths: map[string]string{}}
	if dir == "" {
		return snap
	}
	cmd := exec.Command("git", "status", "--porcelain=v1", "-z", "--untracked-files=all")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return snap
	}
	snap.complete = true
	for path, status := range parsePorcelainZ(out) {
		snap.paths[path] = status + ":" + pathFingerprint(dir, path)
	}
	return snap
}

func parsePorcelainZ(data []byte) map[string]string {
	entries := strings.Split(string(data), "\x00")
	out := make(map[string]string)
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if len(entry) < 4 || entry[2] != ' ' {
			continue
		}
		status := entry[:2]
		path := entry[3:]
		if status[0] == 'R' || status[0] == 'C' || status[1] == 'R' || status[1] == 'C' {
			i++
		}
		out[path] = status
	}
	return out
}

func pathFingerprint(dir, path string) string {
	abs := filepath.Join(dir, filepath.FromSlash(path))
	info, err := os.Lstat(abs)
	if err != nil {
		return "missing"
	}
	h := sha256.New()
	h.Write([]byte(info.Mode().String()))
	if info.Mode().IsRegular() {
		if data, err := os.ReadFile(abs); err == nil {
			h.Write(data)
		}
	} else if info.Mode()&os.ModeSymlink != 0 {
		if target, err := os.Readlink(abs); err == nil {
			h.Write([]byte(target))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func diffWorktree(dir string, baseline worktreeSnapshot) ([]string, bool) {
	current := snapshotWorktree(dir)
	if !current.complete {
		return nil, false
	}
	changed := make([]string, 0)
	for path, signature := range current.paths {
		if baseline.paths[path] != signature {
			changed = append(changed, path)
		}
	}
	for path := range baseline.paths {
		if _, ok := current.paths[path]; !ok {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return changed, true
}

func worktreeChanged(before, after worktreeSnapshot) bool {
	if !before.complete || !after.complete || len(before.paths) != len(after.paths) {
		return true
	}
	for path, signature := range before.paths {
		if after.paths[path] != signature {
			return true
		}
	}
	return false
}

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

func isVerification(input string) bool {
	var args struct {
		Purpose string `json:"purpose"`
	}
	if json.Unmarshal([]byte(input), &args) != nil {
		return false
	}
	return args.Purpose == "verification"
}

func verificationCommand(input string) string {
	var args struct {
		Command string `json:"command"`
	}
	if json.Unmarshal([]byte(input), &args) != nil {
		return ""
	}
	return args.Command
}

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

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// maxInstructionsSize caps how large an AGENTS.md file may be.
const maxInstructionsSize = 64 * 1024

// SystemInstructions is the fixed system prompt Kite sends to the model.
const SystemInstructions = `You are Kite, a minimal agent runtime that can explain and modify a repository.
You have access to tools for reading files, editing files, running shell commands,
and retrieving stored artifacts. Work inside the repository working directory
only. Prefer the smallest change that satisfies the request. When you make a
change, verify it with a bash command with purpose "verification" before
finishing.`

// Instructions is the repository instructions loaded from AGENTS.md.
type Instructions struct {
	// Path is the absolute source path of the loaded file, or "" if none.
	Path string
	// Content is the file content, or "" if none.
	Content string
}

// LoadInstructions loads the nearest AGENTS.md between the working directory
// and the repository root. Files larger than 64 KiB are rejected.
func LoadInstructions(dir string) (Instructions, error) {
	if dir == "" {
		return Instructions{}, nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Instructions{}, err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	root := abs
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = abs
	if out, err := cmd.Output(); err == nil {
		if candidate := filepath.Clean(string(bytesTrimSpace(out))); candidate != "." {
			root = candidate
			if resolved, err := filepath.EvalSymlinks(root); err == nil {
				root = resolved
			}
		}
	}
	cur := abs
	for {
		candidate := filepath.Join(cur, "AGENTS.md")
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			if st.Size() > maxInstructionsSize {
				return Instructions{}, fmt.Errorf("kite: AGENTS.md at %s exceeds %d bytes", candidate, maxInstructionsSize)
			}
			data, err := os.ReadFile(candidate)
			if err != nil {
				return Instructions{}, err
			}
			return Instructions{Path: candidate, Content: string(data)}, nil
		}
		if cur == root {
			return Instructions{}, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return Instructions{}, nil
		}
		cur = parent
	}
}

func bytesTrimSpace(data []byte) []byte {
	start, end := 0, len(data)
	for start < end && (data[start] == ' ' || data[start] == '\n' || data[start] == '\r' || data[start] == '\t') {
		start++
	}
	for end > start && (data[end-1] == ' ' || data[end-1] == '\n' || data[end-1] == '\r' || data[end-1] == '\t') {
		end--
	}
	return data[start:end]
}

// buildContext returns the deterministic context for a session: fixed system
// instructions, repository instructions, completed messages, and bounded tool
// previews.
func buildContext(s *Session) []Message {
	msgs := make([]Message, 0, len(s.Messages)+2)
	msgs = append(msgs, Message{Role: RoleSystem, Content: SystemInstructions})

	if s.cfg.WorkingDir != "" {
		if inst, err := LoadInstructions(s.cfg.WorkingDir); err == nil && inst.Content != "" {
			msgs = append(msgs, Message{Role: RoleSystem, Content: "Repository instructions (" + inst.Path + "):\n" + inst.Content})
		}
	}

	msgs = append(msgs, s.Messages...)
	return msgs
}

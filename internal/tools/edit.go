package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Edit returns a Tool that replaces an exact block of text in a file.
// Writes are atomic and preserve file permissions.
func (s *Set) Edit() *Tool {
	return &Tool{
		name:        "edit",
		description: "Replace an exact block of text in a file within the repository. The old_text must match exactly, including whitespace. Writes are atomic.",
		specs: []argSpec{
			{name: "path", typ: "string", desc: "Repository-relative path to the file.", required: true},
			{name: "old_text", typ: "string", desc: "Exact text to find and replace.", required: true},
			{name: "new_text", typ: "string", desc: "Replacement text.", required: true},
			{name: "apply_all", typ: "boolean", desc: "Replace every occurrence instead of just the first."},
		},
		run: func(ctx context.Context, args map[string]any) (string, error) {
			path := str(args, "path")
			oldText := str(args, "old_text")
			newText := str(args, "new_text")
			applyAll := boolv(args, "apply_all")
			if oldText == "" {
				return "", fmt.Errorf("old_text must not be empty")
			}
			abs, err := s.resolve(path)
			if err != nil {
				return "", err
			}
			st, err := os.Stat(abs)
			if err != nil {
				return "", err
			}
			data, err := os.ReadFile(abs)
			if err != nil {
				return "", err
			}
			content := string(data)
			count := strings.Count(content, oldText)
			if count == 0 {
				return "", fmt.Errorf("old_text not found in %s", path)
			}
			var replaced string
			if applyAll {
				replaced = strings.ReplaceAll(content, oldText, newText)
			} else {
				replaced = strings.Replace(content, oldText, newText, 1)
			}
			if err := writeFileAtomic(abs, []byte(replaced), st.Mode()); err != nil {
				return "", err
			}
			return fmt.Sprintf("Replaced %d occurrence(s) in %s.", count, path), nil
		},
	}
}

// writeFileAtomic replaces a file by writing a temp file in the same
// directory and renaming it over the target, so a crash or interruption
// never leaves a truncated file behind.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".kite-edit-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

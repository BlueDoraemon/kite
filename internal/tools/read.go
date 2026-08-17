package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// Read returns a Tool that prints a file (or directory listing) with line
// numbers.
func (s *Set) Read() *Tool {
	return &Tool{
		name:        "read",
		description: "Read a file, or list a directory, within the repository. Returns line-numbered content.",
		specs: []argSpec{
			{name: "path", typ: "string", desc: "Repository-relative path to read.", required: true},
		},
		run: func(ctx context.Context, args map[string]any) (string, error) {
			path := str(args, "path")
			abs, err := s.resolve(path)
			if err != nil {
				return "", err
			}
			info, err := os.Stat(abs)
			if err != nil {
				return "", err
			}
			if info.IsDir() {
				return s.listDir(abs)
			}
			data, err := os.ReadFile(abs)
			if err != nil {
				return "", err
			}
			return lineNumbered(string(data)), nil
		},
	}
}

// lineNumbered prefixes each line with its 1-based number.
func lineNumbered(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var sb strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&sb, "%4d\t%s\n", i+1, line)
	}
	return sb.String()
}

func (s *Set) listDir(abs string) (string, error) {
	entries, err := os.ReadDir(abs)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		sb.WriteString(name)
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

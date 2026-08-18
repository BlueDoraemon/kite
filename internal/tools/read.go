package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// Read returns a Tool that prints a file (or directory listing) with line
// numbers. Large files are stored as artifacts and referenced in the result.
func (s *Set) Read() *Tool {
	return &Tool{
		name:        "read",
		description: "Read a file, or list a directory, within the repository. Returns line-numbered content; large files are stored as an artifact referenced in the result.",
		specs: []argSpec{
			{name: "path", typ: "string", desc: "Repository-relative path to read.", required: true},
			{name: "start_line", typ: "integer", desc: "1-based first line to read (inclusive)."},
			{name: "end_line", typ: "integer", desc: "1-based last line to read (inclusive)."},
		},
		run: func(ctx context.Context, args map[string]any) (string, error) {
			path := str(args, "path")
			startLine := intv(args, "start_line")
			endLine := intv(args, "end_line")
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
			content := lineNumbered(string(data))
			if startLine > 0 || endLine > 0 {
				content = lineRange(content, startLine, endLine)
			}
			return content, nil
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

// lineRange returns only the requested line range of line-numbered content.
func lineRange(content string, startLine, endLine int) string {
	lines := strings.Split(content, "\n")
	if startLine <= 0 {
		startLine = 1
	}
	if endLine <= 0 || endLine > len(lines) {
		endLine = len(lines)
	}
	if startLine > endLine {
		return ""
	}
	return strings.Join(lines[startLine-1:endLine], "\n") + "\n"
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

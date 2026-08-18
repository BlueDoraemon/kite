package kite_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestExamplesCompile ensures every Go example builds.
func TestExamplesCompile(t *testing.T) {
	dirs := []string{
		"examples/agents/go-session",
		"examples/agents/custom-tool",
		"examples/agents/rpc-client",
	}
	for _, d := range dirs {
		cmd := exec.Command("go", "build", "-o", "/dev/null", ".")
		cmd.Dir = d
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("example %s failed to build: %v\n%s", d, err, out)
		}
	}
}

// TestSchemasMatchGenerated ensures the committed schemas match the
// generator output.
func TestSchemasMatchGenerated(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd/schemagen")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("schema generation check failed: %v\n%s", err, out)
	}
}

// TestMarkdownLinksResolve ensures internal markdown links point at real
// files.
func TestMarkdownLinksResolve(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	filepath.Walk(filepath.Join(root, "docs"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(line, "](") {
				continue
			}
			// Extract the link target.
			start := strings.Index(line, "](")
			end := strings.Index(line[start:], ")")
			if end < 0 {
				continue
			}
			target := line[start+2 : start+end]
			if strings.HasPrefix(target, "http") || strings.HasPrefix(target, "#") {
				continue
			}
			// Resolve relative to the doc file.
			resolved := filepath.Join(filepath.Dir(f), target)
			if _, err := os.Stat(resolved); err != nil {
				t.Fatalf("%s: link %q does not resolve", f, target)
			}
		}
	}
}

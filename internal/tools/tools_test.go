package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BlueDoraemon/kite-core/internal/core"
)

func TestReadFileShowsLineNumbers(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(fpath, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := (&Set{Dir: dir}).Read()
	out, err := tool.Run(context.Background(), `{"path":"a.txt"}`)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	want := "   1\tone\n   2\ttwo\n   3\tthree\n"
	if out != want {
		t.Fatalf("read output:\n%q\nwant:\n%q", out, want)
	}
}

func TestReadLineRange(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(fpath, []byte("one\ntwo\nthree\nfour\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := (&Set{Dir: dir}).Read()
	out, err := tool.Run(context.Background(), `{"path":"a.txt","start_line":2,"end_line":3}`)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !strings.Contains(out, "two") || !strings.Contains(out, "three") {
		t.Fatalf("range output = %q", out)
	}
	if strings.Contains(out, "one") || strings.Contains(out, "four") {
		t.Fatalf("range output includes out-of-range lines: %q", out)
	}
}

func TestReadLargeFileStoredAsArtifact(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("large-content\n", 3000)
	if err := os.WriteFile(filepath.Join(dir, "large.txt"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := (&Set{Dir: dir}).Read().Run(context.Background(), `{"path":"large.txt"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "stored as artifact") {
		t.Fatalf("large read should be stored as artifact, got %q", out)
	}
}

func TestReadRejectsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(dir, "link.txt")); err != nil {
		t.Fatal(err)
	}
	tool := (&Set{Dir: dir}).Read()
	_, err := tool.Run(context.Background(), `{"path":"link.txt"}`)
	if err == nil {
		t.Fatal("expected error for symlink escaping the working directory")
	}
}

func TestEditReplacesTextAtomically(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(fpath, []byte("hello foo world foo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tool := (&Set{Dir: dir}).Edit()
	out, err := tool.Run(context.Background(), `{"path":"b.txt","old_text":"foo","new_text":"bar"}`)
	if err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	if !strings.Contains(out, "Replaced 2") {
		t.Fatalf("edit output = %q", out)
	}
	data, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}
	// Default is replace-first only.
	if string(data) != "hello bar world foo\n" {
		t.Fatalf("file after edit = %q", string(data))
	}
	// Mode is preserved.
	st, _ := os.Stat(fpath)
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v, want 0600", st.Mode().Perm())
	}
	// No temp files left.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".kite-edit-") {
			t.Fatalf("temp file left: %s", e.Name())
		}
	}
}

func TestEditApplyAll(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "c.txt")
	if err := os.WriteFile(fpath, []byte("foo foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := (&Set{Dir: dir}).Edit()
	if _, err := tool.Run(context.Background(), `{"path":"c.txt","old_text":"foo","new_text":"bar","apply_all":true}`); err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	data, _ := os.ReadFile(fpath)
	if string(data) != "bar bar\n" {
		t.Fatalf("file after apply_all edit = %q", string(data))
	}
}

func TestBashRunsCommandInDir(t *testing.T) {
	dir := t.TempDir()
	tool := (&Set{Dir: dir}).Bash()
	out, err := tool.Run(context.Background(), `{"command":"pwd"}`)
	if err != nil {
		t.Fatalf("bash failed: %v", err)
	}
	abs, _ := filepath.Abs(dir)
	if !strings.Contains(out, abs) {
		t.Fatalf("bash output = %q, want cwd %q", out, abs)
	}
}

func TestBashCapturesExitCode(t *testing.T) {
	dir := t.TempDir()
	tool := (&Set{Dir: dir}).Bash()
	out, err := tool.Run(context.Background(), `{"command":"echo boom >&2; exit 3"}`)
	if err != nil {
		t.Fatalf("bash failed: %v", err)
	}
	if !strings.Contains(out, "exit status 3") || !strings.Contains(out, "boom") {
		t.Fatalf("bash output = %q, want exit status 3 and stderr 'boom'", out)
	}
}

func TestBashWorkingDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	tool := (&Set{Dir: dir}).Bash()
	out, err := tool.Run(context.Background(), `{"command":"pwd","working_dir":"sub"}`)
	if err != nil {
		t.Fatalf("bash failed: %v", err)
	}
	abs, _ := filepath.Abs(sub)
	if !strings.Contains(out, abs) {
		t.Fatalf("bash output = %q, want cwd %q", out, abs)
	}
}

func TestArtifactPaging(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	t.Setenv("KITE_DATA_DIR", dataDir)
	store, err := core.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	content := strings.Repeat("x", 100)
	sessionID := "sess_000000000000000000000001"
	artifactID := "art_000000000000000000000001"
	if err := store.StoreArtifact(sessionID, artifactID, []byte(content)); err != nil {
		t.Fatal(err)
	}
	tool := (&Set{Dir: dir, Store: store}).Artifact()
	out, err := tool.Run(context.Background(), `{"id":"art_000000000000000000000001","offset":0,"limit":50}`)
	if err != nil {
		t.Fatalf("artifact failed: %v", err)
	}
	if len(out) != 50 {
		t.Fatalf("artifact output = %d bytes, want 50", len(out))
	}
}

func TestSmallBashOutputStaysInline(t *testing.T) {
	dir := t.TempDir()
	tool := (&Set{Dir: dir}).Bash()
	out, err := tool.Run(context.Background(), `{"command":"printf hello"}`)
	if err != nil {
		t.Fatalf("bash failed: %v", err)
	}
	if out != "hello" {
		t.Fatalf("bash output = %q, want inline 'hello'", out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".kite")); !os.IsNotExist(err) {
		t.Fatalf("no artifact dir expected for small output, err = %v", err)
	}
}

func TestLargeBashOutputStoredAsArtifact(t *testing.T) {
	dir := t.TempDir()
	tool := (&Set{Dir: dir}).Bash()
	// 20 KiB of output exceeds the inline cap, so it must become an artifact.
	out, err := tool.Run(context.Background(), `{"command":"head -c 20480 /dev/zero | tr '\\0' 'a'"}`)
	if err != nil {
		t.Fatalf("bash failed: %v", err)
	}
	if strings.Contains(out, "stored as artifact") == false || strings.Contains(out, "artifacts/") == false {
		t.Fatalf("bash output = %q, want artifact reference", out)
	}
	// Parse the artifact path out and confirm it exists and matches size.
	idx := strings.Index(out, ".kite/")
	if idx < 0 {
		t.Fatalf("no artifact path in output: %q", out)
	}
	rest := out[idx:]
	rel := strings.Fields(rest)[0]
	full := filepath.Join(dir, rel)
	data, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("artifact file %s unreadable: %v", full, err)
	}
	if len(data) != 20480 {
		t.Fatalf("artifact size = %d, want 20480", len(data))
	}
	// The result preview must be a truncated head/tail, not the full output.
	if len(out) >= 20480 {
		t.Fatalf("tool result should carry a preview, got %d bytes inline", len(out))
	}
}

func TestLargeReadStoredAsArtifact(t *testing.T) {
	dir := t.TempDir()
	big := strings.Repeat("x", 20*1024) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "big.txt"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := (&Set{Dir: dir}).Read()
	out, err := tool.Run(context.Background(), `{"path":"big.txt"}`)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if strings.Contains(out, "stored as artifact") == false {
		t.Fatalf("read output = %q, want artifact reference", out)
	}
	// The artifact must contain the line-numbered content.
	idx := strings.Index(out, ".kite/")
	if idx < 0 {
		t.Fatalf("no artifact path in output: %q", out)
	}
	full := filepath.Join(dir, strings.Fields(out[idx:])[0])
	data, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("artifact file unreadable: %v", err)
	}
	if !strings.Contains(string(data), "   1\txxxxxxxx") {
		t.Fatalf("artifact should carry line-numbered content")
	}
}

func TestSmallReadStaysInline(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "src.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	set := &Set{Dir: dir}
	out, err := set.Read().Run(context.Background(), `{"path":"src.go"}`)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if strings.Contains(out, "stored as artifact") {
		t.Fatalf("small file should be inline, got %q", out)
	}
}

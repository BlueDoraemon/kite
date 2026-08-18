package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
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

func TestReadDirectoryListsEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := (&Set{Dir: dir}).Read()
	out, err := tool.Run(context.Background(), `{"path":"."}`)
	if err != nil {
		t.Fatalf("read dir failed: %v", err)
	}
	if !strings.Contains(out, "f.txt") || !strings.Contains(out, "sub/") {
		t.Fatalf("dir listing = %q, want entry f.txt and sub/", out)
	}
}

func TestReadRejectsEscapingDir(t *testing.T) {
	dir := t.TempDir()
	tool := (&Set{Dir: dir}).Read()
	_, err := tool.Run(context.Background(), `{"path":"../secret"}`)
	if err == nil {
		t.Fatal("expected error for path escaping the working directory")
	}
}

func TestEditReplacesText(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(fpath, []byte("hello foo world foo\n"), 0o644); err != nil {
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

func TestEditMissingTextErrors(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "d.txt")
	if err := os.WriteFile(fpath, []byte("abc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tool := (&Set{Dir: dir}).Edit()
	_, err := tool.Run(context.Background(), `{"path":"d.txt","old_text":"zzz","new_text":"q"}`)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
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

func TestBashCapturesStderrAndExitCode(t *testing.T) {
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

func TestBashMissingCommandErrors(t *testing.T) {
	tool := (&Set{Dir: "."}).Bash()
	_, err := tool.Run(context.Background(), `{"command":"_nonexistent_cmd_123"}`)
	if err != nil {
		t.Fatalf("expected exit-status result, got error: %v", err)
	}
}

func TestEditPreservesModeAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "m.txt")
	if err := os.WriteFile(fpath, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tool := (&Set{Dir: dir}).Edit()
	if _, err := tool.Run(context.Background(), `{"path":"m.txt","old_text":"old","new_text":"new"}`); err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	st, err := os.Stat(fpath)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v, want 0600", st.Mode().Perm())
	}
	data, _ := os.ReadFile(fpath)
	if string(data) != "new\n" {
		t.Fatalf("file content = %q, want %q", string(data), "new\n")
	}
	// No temp files may be left behind.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".kite-edit-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}

func TestBashTimeoutKillsChildProcesses(t *testing.T) {
	// Spawn a child that outlives the shell; the process-group kill must
	// take it down too. The command runs longer than BashTimeout, so the
	// tool returns before the child could exit on its own.
	dir := t.TempDir()
	marker := filepath.Join(dir, "child-alive")
	command := fmt.Sprintf("sh -c 'sleep 100 & echo $! > %s; wait'", marker)
	tool := (&Set{Dir: dir}).Bash()
	_, err := tool.Run(context.Background(), `{"command":"`+command+`"}`)
	if err != nil {
		t.Fatalf("bash failed: %v", err)
	}
	// The child pid was recorded; it must be gone now.
	data, rerr := os.ReadFile(marker)
	if rerr != nil {
		t.Fatalf("marker not written: %v", rerr)
	}
	pid := strings.TrimSpace(string(data))
	if pid == "" {
		t.Fatal("no child pid recorded")
	}
	// Give the kill a moment to land, then check the process is gone.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("child process %s still alive after bash timeout", pid)
}

// processAlive reports whether a pid exists, using a zero signal.
func processAlive(pid string) bool {
	p, err := strconv.Atoi(pid)
	if err != nil {
		return false
	}
	err = syscall.Kill(p, 0)
	return err == nil || err == syscall.EPERM
}

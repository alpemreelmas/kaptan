package executor_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alpemreelmas/kaptan/agent/executor"
)

func writeScript(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.sh")
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return path
}

func TestRunScript_Success(t *testing.T) {
	script := writeScript(t, "#!/bin/bash\necho hello\necho world\n")
	var lines []string
	code, err := executor.RunScript(script, context.Background(), func(line string, isErr bool) error {
		lines = append(lines, line)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if len(lines) != 2 || lines[0] != "hello" || lines[1] != "world" {
		t.Fatalf("unexpected lines: %v", lines)
	}
}

func TestRunScript_ExitNonZero(t *testing.T) {
	script := writeScript(t, "#!/bin/bash\necho failing\nexit 42\n")
	var lines []string
	code, err := executor.RunScript(script, context.Background(), func(line string, isErr bool) error {
		lines = append(lines, line)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 42 {
		t.Fatalf("expected exit 42, got %d", code)
	}
}

func TestRunScript_Stderr(t *testing.T) {
	script := writeScript(t, "#!/bin/bash\necho stdout\necho stderr >&2\n")
	var stdout, stderr []string
	executor.RunScript(script, context.Background(), func(line string, isErr bool) error {
		if isErr {
			stderr = append(stderr, line)
		} else {
			stdout = append(stdout, line)
		}
		return nil
	})
	if len(stdout) == 0 || stdout[0] != "stdout" {
		t.Fatalf("expected stdout line, got %v", stdout)
	}
	if len(stderr) == 0 || stderr[0] != "stderr" {
		t.Fatalf("expected stderr line, got %v", stderr)
	}
}

func TestRunScript_NotFound(t *testing.T) {
	code, err := executor.RunScript("/nonexistent/script.sh", context.Background(), func(string, bool) error { return nil })
	if err == nil {
		t.Fatal("expected error for missing script")
	}
	if !strings.Contains(err.Error(), "script not found") {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
}

func TestRunScript_Cancelled(t *testing.T) {
	script := writeScript(t, "#!/bin/bash\nsleep 10\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	code, _ := executor.RunScript(script, ctx, func(string, bool) error { return nil })
	if code == 0 {
		t.Fatal("expected non-zero exit on cancelled context")
	}
}

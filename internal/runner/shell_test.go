package runner

import (
	"strings"
	"testing"
)

func TestRunEchoHello(t *testing.T) {
	r := Run("echo hello", "")
	if r.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", r.ExitCode)
	}
	if strings.TrimSpace(r.Stdout) != "hello" {
		t.Errorf("expected stdout 'hello', got %q", r.Stdout)
	}
}

func TestRunCaptureStderr(t *testing.T) {
	r := Run("echo error >&2", "")
	if r.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", r.ExitCode)
	}
	if strings.TrimSpace(r.Stderr) != "error" {
		t.Errorf("expected stderr 'error', got %q", r.Stderr)
	}
}

func TestRunNonZeroExitCode(t *testing.T) {
	r := Run("exit 42", "")
	if r.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", r.ExitCode)
	}
}

func TestRunPipesWork(t *testing.T) {
	r := Run("echo hello world | wc -w", "")
	if r.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", r.ExitCode)
	}
	if strings.TrimSpace(r.Stdout) != "2" {
		t.Errorf("expected stdout '2', got %q", strings.TrimSpace(r.Stdout))
	}
}

func TestRunMultiLineStdout(t *testing.T) {
	r := Run("printf 'line1\nline2\nline3'", "")
	if r.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", r.ExitCode)
	}
	lines := strings.Split(r.Stdout, "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d: %q", len(lines), r.Stdout)
	}
}

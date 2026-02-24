package artifact

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewCreatesRunDir(t *testing.T) {
	dir := t.TempDir()
	store, err := New("run-123", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stepsDir := filepath.Join(store.BaseDir, "steps")
	info, err := os.Stat(stepsDir)
	if err != nil {
		t.Fatalf("steps dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected steps to be a directory")
	}
}

func TestWriteStepOutput(t *testing.T) {
	dir := t.TempDir()
	store, _ := New("run-456", dir)

	err := store.WriteStepOutput("s1", "out-data", "err-data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stdout, _ := os.ReadFile(filepath.Join(store.BaseDir, "steps", "s1.stdout"))
	if string(stdout) != "out-data" {
		t.Errorf("expected stdout 'out-data', got %q", string(stdout))
	}
	stderr, _ := os.ReadFile(filepath.Join(store.BaseDir, "steps", "s1.stderr"))
	if string(stderr) != "err-data" {
		t.Errorf("expected stderr 'err-data', got %q", string(stderr))
	}
}

func TestWriteResult(t *testing.T) {
	dir := t.TempDir()
	store, _ := New("run-789", dir)

	result := map[string]string{"status": "ok"}
	err := store.WriteResult(result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(store.BaseDir, "result.json"))
	var obj map[string]string
	json.Unmarshal(data, &obj)
	if obj["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", obj["status"])
	}
}

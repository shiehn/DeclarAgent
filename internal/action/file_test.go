package action

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileWriteCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	fw := &FileWrite{}
	_, err := fw.Execute(map[string]string{"path": path, "content": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", string(data))
	}
}

func TestFileWriteOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	os.WriteFile(path, []byte("old"), 0o644)
	fw := &FileWrite{}
	_, err := fw.Execute(map[string]string{"path": path, "content": "new"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "new" {
		t.Errorf("expected 'new', got %q", string(data))
	}
}

func TestFileWriteReturnsPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	fw := &FileWrite{}
	out, err := fw.Execute(map[string]string{"path": path, "content": "x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["path"] != path {
		t.Errorf("expected path %q, got %q", path, out["path"])
	}
}

func TestFileAppendCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	fa := &FileAppend{}
	_, err := fa.Execute(map[string]string{"path": path, "content": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", string(data))
	}
}

func TestFileAppendAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	os.WriteFile(path, []byte("aaa"), 0o644)
	fa := &FileAppend{}
	_, err := fa.Execute(map[string]string{"path": path, "content": "bbb"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "aaabbb" {
		t.Errorf("expected 'aaabbb', got %q", string(data))
	}
}

func TestFileWriteMissingParamsErrors(t *testing.T) {
	fw := &FileWrite{}
	_, err := fw.Execute(map[string]string{"content": "x"})
	if err == nil {
		t.Fatal("expected error for missing path")
	}
	_, err = fw.Execute(map[string]string{"path": "/tmp/x"})
	if err == nil {
		t.Fatal("expected error for missing content")
	}
}

func TestFileAppendMissingParamsErrors(t *testing.T) {
	fa := &FileAppend{}
	_, err := fa.Execute(map[string]string{"content": "x"})
	if err == nil {
		t.Fatal("expected error for missing path")
	}
	_, err = fa.Execute(map[string]string{"path": "/tmp/x"})
	if err == nil {
		t.Fatal("expected error for missing content")
	}
}

func TestFileWriteDryRun(t *testing.T) {
	fw := &FileWrite{}
	desc := fw.DryRun(map[string]string{"path": "/tmp/f.txt", "content": "abc"})
	if desc == "" {
		t.Fatal("expected non-empty dry run description")
	}
}

func TestFileAppendDryRun(t *testing.T) {
	fa := &FileAppend{}
	desc := fa.DryRun(map[string]string{"path": "/tmp/f.txt", "content": "abc"})
	if desc == "" {
		t.Fatal("expected non-empty dry run description")
	}
}

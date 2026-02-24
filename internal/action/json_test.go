package action

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeJSON(t *testing.T, dir string, obj map[string]any) string {
	t.Helper()
	path := filepath.Join(dir, "data.json")
	data, _ := json.Marshal(obj)
	os.WriteFile(path, data, 0o644)
	return path
}

func TestJSONGetTopLevelKey(t *testing.T) {
	dir := t.TempDir()
	path := writeJSON(t, dir, map[string]any{"version": "1.0"})
	jg := &JSONGet{}
	out, err := jg.Execute(map[string]string{"file": path, "path": "version"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["value"] != "1.0" {
		t.Errorf("expected '1.0', got %q", out["value"])
	}
}

func TestJSONGetNestedKey(t *testing.T) {
	dir := t.TempDir()
	path := writeJSON(t, dir, map[string]any{"a": map[string]any{"b": "deep"}})
	jg := &JSONGet{}
	out, err := jg.Execute(map[string]string{"file": path, "path": "a.b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["value"] != "deep" {
		t.Errorf("expected 'deep', got %q", out["value"])
	}
}

func TestJSONGetMissingKeyError(t *testing.T) {
	dir := t.TempDir()
	path := writeJSON(t, dir, map[string]any{"a": "b"})
	jg := &JSONGet{}
	_, err := jg.Execute(map[string]string{"file": path, "path": "nope"})
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestJSONGetMissingFileError(t *testing.T) {
	jg := &JSONGet{}
	_, err := jg.Execute(map[string]string{"file": "/nonexistent/file.json", "path": "x"})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestJSONSetTopLevelKey(t *testing.T) {
	dir := t.TempDir()
	path := writeJSON(t, dir, map[string]any{"a": "1"})
	js := &JSONSet{}
	_, err := js.Execute(map[string]string{"file": path, "path": "b", "value": "2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	var obj map[string]any
	json.Unmarshal(data, &obj)
	if obj["b"] != "2" {
		t.Errorf("expected b='2', got %v", obj["b"])
	}
}

func TestJSONSetNestedKeyCreatesIntermediates(t *testing.T) {
	dir := t.TempDir()
	path := writeJSON(t, dir, map[string]any{})
	js := &JSONSet{}
	_, err := js.Execute(map[string]string{"file": path, "path": "a.b.c", "value": "deep"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	var obj map[string]any
	json.Unmarshal(data, &obj)
	a, _ := obj["a"].(map[string]any)
	b, _ := a["b"].(map[string]any)
	if b["c"] != "deep" {
		t.Errorf("expected nested value 'deep', got %v", b["c"])
	}
}

func TestJSONSetCreatesFileIfNotExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.json")
	js := &JSONSet{}
	_, err := js.Execute(map[string]string{"file": path, "path": "key", "value": "val"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	var obj map[string]any
	json.Unmarshal(data, &obj)
	if obj["key"] != "val" {
		t.Errorf("expected key='val', got %v", obj["key"])
	}
}

func TestJSONSetPreservesExistingKeys(t *testing.T) {
	dir := t.TempDir()
	path := writeJSON(t, dir, map[string]any{"existing": "keep"})
	js := &JSONSet{}
	_, err := js.Execute(map[string]string{"file": path, "path": "new", "value": "added"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(path)
	var obj map[string]any
	json.Unmarshal(data, &obj)
	if obj["existing"] != "keep" {
		t.Errorf("expected existing='keep', got %v", obj["existing"])
	}
	if obj["new"] != "added" {
		t.Errorf("expected new='added', got %v", obj["new"])
	}
}

func TestJSONGetDryRun(t *testing.T) {
	jg := &JSONGet{}
	desc := jg.DryRun(map[string]string{"file": "f.json", "path": "x"})
	if desc == "" {
		t.Fatal("expected non-empty dry run description")
	}
}

func TestJSONSetDryRun(t *testing.T) {
	js := &JSONSet{}
	desc := js.DryRun(map[string]string{"file": "f.json", "path": "x", "value": "v"})
	if desc == "" {
		t.Fatal("expected non-empty dry run description")
	}
}

package action

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileWrite implements file.write action.
type FileWrite struct{}

func (f *FileWrite) Execute(params map[string]string) (map[string]string, error) {
	path := params["path"]
	content := params["content"]
	if path == "" {
		return nil, fmt.Errorf("file.write: missing required param 'path'")
	}
	if content == "" {
		return nil, fmt.Errorf("file.write: missing required param 'content'")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("file.write: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("file.write: %w", err)
	}
	return map[string]string{"path": path}, nil
}

func (f *FileWrite) DryRun(params map[string]string) string {
	return fmt.Sprintf("Would write %d bytes to %s", len(params["content"]), params["path"])
}

// FileAppend implements file.append action.
type FileAppend struct{}

func (f *FileAppend) Execute(params map[string]string) (map[string]string, error) {
	path := params["path"]
	content := params["content"]
	if path == "" {
		return nil, fmt.Errorf("file.append: missing required param 'path'")
	}
	if content == "" {
		return nil, fmt.Errorf("file.append: missing required param 'content'")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("file.append: %w", err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("file.append: %w", err)
	}
	defer file.Close()
	if _, err := file.WriteString(content); err != nil {
		return nil, fmt.Errorf("file.append: %w", err)
	}
	return map[string]string{"path": path}, nil
}

func (f *FileAppend) DryRun(params map[string]string) string {
	return fmt.Sprintf("Would append %d bytes to %s", len(params["content"]), params["path"])
}

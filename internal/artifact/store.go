package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Store manages artifact storage for a run.
type Store struct {
	RunID   string
	BaseDir string // defaults to .declaragent/runs/<run_id>
}

// New creates a store for a given run ID, rooted at workDir.
func New(runID, workDir string) (*Store, error) {
	base := filepath.Join(workDir, ".declaragent", "runs", runID)
	if err := os.MkdirAll(filepath.Join(base, "steps"), 0o755); err != nil {
		return nil, fmt.Errorf("creating artifact dir: %w", err)
	}
	return &Store{RunID: runID, BaseDir: base}, nil
}

// WriteStepOutput writes stdout/stderr for a step.
func (s *Store) WriteStepOutput(stepID, stdout, stderr string) error {
	if stdout != "" {
		if err := os.WriteFile(filepath.Join(s.BaseDir, "steps", stepID+".stdout"), []byte(stdout), 0o644); err != nil {
			return err
		}
	}
	if stderr != "" {
		if err := os.WriteFile(filepath.Join(s.BaseDir, "steps", stepID+".stderr"), []byte(stderr), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// WriteResult writes the final result JSON.
func (s *Store) WriteResult(result any) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.BaseDir, "result.json"), data, 0o644)
}

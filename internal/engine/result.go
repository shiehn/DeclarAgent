package engine

import dagerrors "github.com/stevehiehn/declaragent/internal/errors"

// Result is the structured output of a plan execution.
type Result struct {
	RunID        string            `json:"run_id"`
	Success      bool              `json:"success"`
	FailedStepID string            `json:"failed_step_id,omitempty"`
	Steps        []StepResult      `json:"steps"`
	Outputs      map[string]string `json:"outputs,omitempty"`
	Artifacts    []string          `json:"artifacts,omitempty"`
	Errors       []dagerrors.RunError `json:"errors,omitempty"`
}

// StepResult describes the outcome of a single step.
type StepResult struct {
	ID          string `json:"id"`
	Status      string `json:"status"` // success, failed, skipped, blocked, dry-run
	ExitCode    int    `json:"exit_code,omitempty"`
	StdoutRef   string `json:"stdout_ref,omitempty"`
	StderrRef   string `json:"stderr_ref,omitempty"`
	Duration    string `json:"duration,omitempty"`
	Description string `json:"description,omitempty"` // for explain/dry-run
	Command     string `json:"command,omitempty"`      // resolved command for explain
	DryRunInfo  string `json:"dry_run_info,omitempty"` // for dry-run of actions
}

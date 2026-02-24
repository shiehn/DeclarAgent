package engine

import (
	"github.com/google/uuid"
	"github.com/stevehiehn/declaragent/internal/template"
)

// RunContext holds state for a plan execution.
type RunContext struct {
	RunID    string
	WorkDir  string
	Inputs   map[string]string
	TmplCtx  *template.Context
	Approve  bool // allow destructive steps
}

// NewRunContext creates a new execution context.
func NewRunContext(workDir string, inputs map[string]string, approve bool) *RunContext {
	return &RunContext{
		RunID:   uuid.New().String(),
		WorkDir: workDir,
		Inputs:  inputs,
		TmplCtx: &template.Context{
			Inputs:      inputs,
			StepOutputs: map[string]map[string]string{},
		},
		Approve: approve,
	}
}

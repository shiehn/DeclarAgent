package errors

import "fmt"

// Error type constants
const (
	ValidationError    = "VALIDATION_ERROR"
	PreconditionFailed = "PRECONDITION_FAILED"
	ToolNotFound       = "TOOL_NOT_FOUND"
	PermissionDenied   = "PERMISSION_DENIED"
	Transient          = "TRANSIENT"
	StepFailed         = "STEP_FAILED"
	Timeout            = "TIMEOUT"
	Cancelled          = "CANCELLED"
	SideEffectBlocked  = "SIDE_EFFECT_BLOCKED"
)

// RunError is a structured error for agent consumption.
type RunError struct {
	Type      string `json:"type"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message"`
	StepID    string `json:"step_id,omitempty"`
	Retryable bool   `json:"retryable"`
	Hint      string `json:"hint,omitempty"`
}

func (e *RunError) Error() string {
	if e.StepID != "" {
		return fmt.Sprintf("[%s] step %s: %s", e.Type, e.StepID, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

func NewValidationError(msg, hint string) *RunError {
	return &RunError{Type: ValidationError, Message: msg, Hint: hint}
}

func NewStepError(stepID, msg, hint string) *RunError {
	return &RunError{Type: StepFailed, StepID: stepID, Message: msg, Hint: hint}
}

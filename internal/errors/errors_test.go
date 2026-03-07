package errors

import (
	"strings"
	"testing"
)

func TestRunErrorWithStepID(t *testing.T) {
	e := &RunError{Type: StepFailed, StepID: "deploy", Message: "exit code 1"}
	got := e.Error()
	if !strings.Contains(got, "[STEP_FAILED]") {
		t.Errorf("expected type in output, got %q", got)
	}
	if !strings.Contains(got, "step deploy") {
		t.Errorf("expected step ID in output, got %q", got)
	}
	if !strings.Contains(got, "exit code 1") {
		t.Errorf("expected message in output, got %q", got)
	}
}

func TestRunErrorWithoutStepID(t *testing.T) {
	e := &RunError{Type: ValidationError, Message: "missing name"}
	got := e.Error()
	if !strings.Contains(got, "[VALIDATION_ERROR]") {
		t.Errorf("expected type in output, got %q", got)
	}
	if strings.Contains(got, "step") {
		t.Errorf("did not expect 'step' in output, got %q", got)
	}
	if !strings.Contains(got, "missing name") {
		t.Errorf("expected message in output, got %q", got)
	}
}

func TestNewValidationError(t *testing.T) {
	e := NewValidationError("bad plan", "check the name field")
	if e.Type != ValidationError {
		t.Errorf("expected type %q, got %q", ValidationError, e.Type)
	}
	if e.Message != "bad plan" {
		t.Errorf("expected message %q, got %q", "bad plan", e.Message)
	}
	if e.Hint != "check the name field" {
		t.Errorf("expected hint %q, got %q", "check the name field", e.Hint)
	}
	if e.StepID != "" {
		t.Errorf("expected empty step ID, got %q", e.StepID)
	}
}

func TestNewStepError(t *testing.T) {
	e := NewStepError("build", "command failed", "check logs")
	if e.Type != StepFailed {
		t.Errorf("expected type %q, got %q", StepFailed, e.Type)
	}
	if e.StepID != "build" {
		t.Errorf("expected step ID %q, got %q", "build", e.StepID)
	}
	if e.Message != "command failed" {
		t.Errorf("expected message %q, got %q", "command failed", e.Message)
	}
	if e.Hint != "check logs" {
		t.Errorf("expected hint %q, got %q", "check logs", e.Hint)
	}
}

func TestErrorConstants(t *testing.T) {
	constants := map[string]string{
		"ValidationError":    ValidationError,
		"PreconditionFailed": PreconditionFailed,
		"ToolNotFound":       ToolNotFound,
		"PermissionDenied":   PermissionDenied,
		"Transient":          Transient,
		"StepFailed":         StepFailed,
		"Timeout":            Timeout,
		"Cancelled":          Cancelled,
		"SideEffectBlocked":  SideEffectBlocked,
	}
	seen := map[string]bool{}
	for name, val := range constants {
		if val == "" {
			t.Errorf("constant %s is empty", name)
		}
		if seen[val] {
			t.Errorf("duplicate constant value %q for %s", val, name)
		}
		seen[val] = true
	}
}

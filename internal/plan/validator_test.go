package plan

import (
	"testing"
)

func validPlan() *Plan {
	return &Plan{
		Name: "test",
		Steps: []Step{
			{ID: "s1", Run: "echo hello", Outputs: map[string]string{"msg": "stdout"}},
		},
	}
}

func TestValidateAcceptsValidPlan(t *testing.T) {
	p := validPlan()
	if err := Validate(p, map[string]string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRejectsDuplicateStepIDs(t *testing.T) {
	p := &Plan{
		Name: "test",
		Steps: []Step{
			{ID: "s1", Run: "echo a"},
			{ID: "s1", Run: "echo b"},
		},
	}
	err := Validate(p, map[string]string{})
	if err == nil {
		t.Fatal("expected error for duplicate step IDs")
	}
}

func TestValidateRejectsBothRunAndAction(t *testing.T) {
	p := &Plan{
		Name: "test",
		Steps: []Step{
			{ID: "s1", Run: "echo a", Action: "file.write"},
		},
	}
	err := Validate(p, map[string]string{})
	if err == nil {
		t.Fatal("expected error for step with both run and action")
	}
}

func TestValidateRejectsNeitherRunNorAction(t *testing.T) {
	p := &Plan{
		Name: "test",
		Steps: []Step{
			{ID: "s1"},
		},
	}
	err := Validate(p, map[string]string{})
	if err == nil {
		t.Fatal("expected error for step with neither run nor action")
	}
}

func TestValidateRejectsForwardReferences(t *testing.T) {
	p := &Plan{
		Name: "test",
		Steps: []Step{
			{ID: "s1", Run: "echo {{steps.s2.outputs.msg}}"},
			{ID: "s2", Run: "echo hi", Outputs: map[string]string{"msg": "stdout"}},
		},
	}
	err := Validate(p, map[string]string{})
	if err == nil {
		t.Fatal("expected error for forward reference")
	}
}

func TestValidateRejectsUnknownStepRefs(t *testing.T) {
	p := &Plan{
		Name: "test",
		Steps: []Step{
			{ID: "s1", Run: "echo {{steps.nonexistent.outputs.msg}}"},
		},
	}
	err := Validate(p, map[string]string{})
	if err == nil {
		t.Fatal("expected error for unknown step reference")
	}
}

func TestValidateRejectsUnknownOutputRefs(t *testing.T) {
	p := &Plan{
		Name: "test",
		Steps: []Step{
			{ID: "s1", Run: "echo hi", Outputs: map[string]string{"msg": "stdout"}},
			{ID: "s2", Run: "echo {{steps.s1.outputs.nonexistent}}"},
		},
	}
	err := Validate(p, map[string]string{})
	if err == nil {
		t.Fatal("expected error for unknown output reference")
	}
}

func TestValidateRejectsUnknownActionName(t *testing.T) {
	p := &Plan{
		Name: "test",
		Steps: []Step{
			{ID: "s1", Action: "bogus.action"},
		},
	}
	err := Validate(p, map[string]string{})
	if err == nil {
		t.Fatal("expected error for unknown action name")
	}
}

func TestValidateRejectsMissingRequiredInput(t *testing.T) {
	p := &Plan{
		Name: "test",
		Inputs: map[string]Input{
			"env": {Required: true},
		},
		Steps: []Step{
			{ID: "s1", Run: "echo hi"},
		},
	}
	err := Validate(p, map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing required input")
	}
}

func TestValidateAcceptsOptionalInputWithDefault(t *testing.T) {
	p := &Plan{
		Name: "test",
		Inputs: map[string]Input{
			"env": {Required: true, Default: "staging"},
		},
		Steps: []Step{
			{ID: "s1", Run: "echo {{inputs.env}}"},
		},
	}
	err := Validate(p, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

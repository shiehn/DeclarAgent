package template

import (
	"testing"
)

func TestResolveInputs(t *testing.T) {
	ctx := &Context{
		Inputs:      map[string]string{"name": "world"},
		StepOutputs: map[string]map[string]string{},
	}
	result, err := Resolve("hello {{inputs.name}}", ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello world" {
		t.Errorf("expected 'hello world', got %q", result)
	}
}

func TestResolveStepOutputs(t *testing.T) {
	ctx := &Context{
		Inputs: map[string]string{},
		StepOutputs: map[string]map[string]string{
			"s1": {"version": "1.0"},
		},
	}
	result, err := Resolve("v={{steps.s1.outputs.version}}", ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "v=1.0" {
		t.Errorf("expected 'v=1.0', got %q", result)
	}
}

func TestResolveMultipleTemplates(t *testing.T) {
	ctx := &Context{
		Inputs: map[string]string{"env": "prod"},
		StepOutputs: map[string]map[string]string{
			"s1": {"ver": "2.0"},
		},
	}
	result, err := Resolve("deploy {{inputs.env}} {{steps.s1.outputs.ver}}", ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "deploy prod 2.0" {
		t.Errorf("expected 'deploy prod 2.0', got %q", result)
	}
}

func TestResolveErrorOnUnresolvedInput(t *testing.T) {
	ctx := &Context{
		Inputs:      map[string]string{},
		StepOutputs: map[string]map[string]string{},
	}
	_, err := Resolve("{{inputs.missing}}", ctx)
	if err == nil {
		t.Fatal("expected error for unresolved input")
	}
}

func TestResolveErrorOnUnresolvedStepRef(t *testing.T) {
	ctx := &Context{
		Inputs:      map[string]string{},
		StepOutputs: map[string]map[string]string{},
	}
	_, err := Resolve("{{steps.nope.outputs.val}}", ctx)
	if err == nil {
		t.Fatal("expected error for unresolved step ref")
	}
}

func TestResolvePassthroughNoTemplates(t *testing.T) {
	ctx := &Context{
		Inputs:      map[string]string{},
		StepOutputs: map[string]map[string]string{},
	}
	result, err := Resolve("plain string", ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "plain string" {
		t.Errorf("expected 'plain string', got %q", result)
	}
}

func TestResolveEmptyString(t *testing.T) {
	ctx := &Context{
		Inputs:      map[string]string{},
		StepOutputs: map[string]map[string]string{},
	}
	result, err := Resolve("", ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

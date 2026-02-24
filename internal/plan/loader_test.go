package plan

import (
	"testing"
)

func TestLoadMinimalPlan(t *testing.T) {
	yaml := []byte(`
name: minimal
steps:
  - id: s1
    run: echo hello
`)
	p, err := Load(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "minimal" {
		t.Errorf("expected name 'minimal', got %q", p.Name)
	}
	if len(p.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(p.Steps))
	}
	if p.Steps[0].ID != "s1" {
		t.Errorf("expected step id 's1', got %q", p.Steps[0].ID)
	}
	if p.Steps[0].Run != "echo hello" {
		t.Errorf("expected run 'echo hello', got %q", p.Steps[0].Run)
	}
}

func TestLoadFullFeaturedPlan(t *testing.T) {
	yaml := []byte(`
name: full
description: A full plan
inputs:
  env:
    required: true
    description: target environment
    default: staging
steps:
  - id: s1
    description: get version
    run: echo 1.0
    outputs:
      version: stdout
  - id: s2
    description: write config
    action: file.write
    params:
      path: /tmp/out.txt
      content: "{{steps.s1.outputs.version}}"
    destructive: true
`)
	p, err := Load(yaml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "full" {
		t.Errorf("expected name 'full', got %q", p.Name)
	}
	if p.Description != "A full plan" {
		t.Errorf("expected description 'A full plan', got %q", p.Description)
	}
	if len(p.Inputs) != 1 {
		t.Fatalf("expected 1 input, got %d", len(p.Inputs))
	}
	inp := p.Inputs["env"]
	if !inp.Required {
		t.Error("expected env input to be required")
	}
	if inp.Default != "staging" {
		t.Errorf("expected default 'staging', got %q", inp.Default)
	}
	if len(p.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(p.Steps))
	}
	if p.Steps[1].Action != "file.write" {
		t.Errorf("expected action 'file.write', got %q", p.Steps[1].Action)
	}
	if !p.Steps[1].Destructive {
		t.Error("expected step s2 to be destructive")
	}
	if p.Steps[0].Outputs["version"] != "stdout" {
		t.Error("expected output 'version' mapped to 'stdout'")
	}
}

func TestLoadRejectsInvalidYAML(t *testing.T) {
	yaml := []byte(`:::not valid yaml[[[`)
	_, err := Load(yaml)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadRejectsEmptyPlan(t *testing.T) {
	yaml := []byte(`
name: empty
steps: []
`)
	_, err := Load(yaml)
	if err == nil {
		t.Fatal("expected error for empty plan")
	}
}

func TestLoadRejectsPlanWithNoName(t *testing.T) {
	yaml := []byte(`
steps:
  - id: s1
    run: echo hi
`)
	_, err := Load(yaml)
	if err == nil {
		t.Fatal("expected error for plan with no name")
	}
}

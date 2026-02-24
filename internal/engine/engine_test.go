package engine

import (
	"testing"

	"github.com/stevehiehn/declaragent/internal/plan"
	"github.com/stevehiehn/declaragent/internal/template"
)

func makeCtx(t *testing.T, inputs map[string]string, approve bool) *RunContext {
	t.Helper()
	dir := t.TempDir()
	if inputs == nil {
		inputs = map[string]string{}
	}
	return &RunContext{
		RunID:   "test-run",
		WorkDir: dir,
		Inputs:  inputs,
		TmplCtx: &template.Context{
			Inputs:      inputs,
			StepOutputs: map[string]map[string]string{},
		},
		Approve: approve,
	}
}

func TestExplainModeReturnsSteps(t *testing.T) {
	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{ID: "s1", Run: "echo hello"},
			{ID: "s2", Run: "echo world"},
		},
	}
	ctx := makeCtx(t, nil, false)
	result, err := Execute(p, ctx, ModeExplain)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(result.Steps))
	}
	for _, sr := range result.Steps {
		if sr.Status != "explain" {
			t.Errorf("expected status 'explain', got %q", sr.Status)
		}
	}
}

func TestRunModeExecutesAndCollectsOutputs(t *testing.T) {
	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{ID: "s1", Run: "echo hello", Outputs: map[string]string{"msg": "stdout"}},
		},
	}
	ctx := makeCtx(t, nil, false)
	result, err := Execute(p, ctx, ModeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Steps[0].Status != "success" {
		t.Errorf("expected status 'success', got %q", result.Steps[0].Status)
	}
	// Check output was captured in template context
	if ctx.TmplCtx.StepOutputs["s1"]["msg"] != "hello" {
		t.Errorf("expected output 'hello', got %q", ctx.TmplCtx.StepOutputs["s1"]["msg"])
	}
}

func TestRunModeFailFast(t *testing.T) {
	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{ID: "s1", Run: "exit 1"},
			{ID: "s2", Run: "echo should-not-run"},
		},
	}
	ctx := makeCtx(t, nil, false)
	result, err := Execute(p, ctx, ModeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure")
	}
	if result.Steps[0].Status != "failed" {
		t.Errorf("expected s1 status 'failed', got %q", result.Steps[0].Status)
	}
	if result.Steps[1].Status != "skipped" {
		t.Errorf("expected s2 status 'skipped', got %q", result.Steps[1].Status)
	}
}

func TestTemplateDataFlowsBetweenSteps(t *testing.T) {
	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{ID: "s1", Run: "echo world", Outputs: map[string]string{"msg": "stdout"}},
			{ID: "s2", Run: "echo hello {{steps.s1.outputs.msg}}"},
		},
	}
	ctx := makeCtx(t, nil, false)
	result, err := Execute(p, ctx, ModeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, errors: %v", result.Errors)
	}
	if result.Steps[1].Command != "echo hello world" {
		t.Errorf("expected resolved command 'echo hello world', got %q", result.Steps[1].Command)
	}
}

func TestDryRunMode(t *testing.T) {
	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{ID: "s1", Run: "echo hello"},
		},
	}
	ctx := makeCtx(t, nil, false)
	result, err := Execute(p, ctx, ModeDryRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Steps[0].Status != "dry-run" {
		t.Errorf("expected status 'dry-run', got %q", result.Steps[0].Status)
	}
	if result.Steps[0].DryRunInfo == "" {
		t.Error("expected non-empty dry run info")
	}
}

func TestDestructiveBlocking(t *testing.T) {
	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{ID: "s1", Run: "rm -rf /", Destructive: true},
			{ID: "s2", Run: "echo after"},
		},
	}
	ctx := makeCtx(t, nil, false) // approve=false
	result, err := Execute(p, ctx, ModeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure due to blocked destructive step")
	}
	if result.Steps[0].Status != "blocked" {
		t.Errorf("expected status 'blocked', got %q", result.Steps[0].Status)
	}
	if result.Steps[1].Status != "skipped" {
		t.Errorf("expected s2 status 'skipped', got %q", result.Steps[1].Status)
	}
}

func TestRunWithApproveAllowsDestructive(t *testing.T) {
	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{ID: "s1", Run: "echo destructive", Destructive: true},
		},
	}
	ctx := makeCtx(t, nil, true) // approve=true
	result, err := Execute(p, ctx, ModeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success with --approve")
	}
	if result.Steps[0].Status != "success" {
		t.Errorf("expected status 'success', got %q", result.Steps[0].Status)
	}
}

func TestMixedShellAndActionSteps(t *testing.T) {
	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{ID: "s1", Run: "echo hello"},
			{
				ID:     "s2",
				Action: "file.write",
				Params: map[string]string{
					"path":    "", // will be set below
					"content": "test content",
				},
			},
		},
	}
	ctx := makeCtx(t, nil, false)
	// Set the file path to a temp location
	p.Steps[1].Params["path"] = ctx.WorkDir + "/mixed.txt"

	result, err := Execute(p, ctx, ModeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, errors: %v", result.Errors)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(result.Steps))
	}
	for _, sr := range result.Steps {
		if sr.Status != "success" {
			t.Errorf("step %s: expected 'success', got %q", sr.ID, sr.Status)
		}
	}
}

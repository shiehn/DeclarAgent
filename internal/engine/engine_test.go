package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dagerrors "github.com/shiehn/declaragent/internal/errors"
	"github.com/shiehn/declaragent/internal/plan"
	"github.com/shiehn/declaragent/internal/template"
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
			{ID: "s2", Run: "echo hello ${{steps.s1.outputs.msg}}"},
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

func TestHTTPStepRunMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{
				ID: "fetch",
				HTTP: &plan.HTTPRequest{
					URL:    srv.URL,
					Method: "GET",
				},
				Outputs: map[string]string{"body": "stdout"},
			},
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
	if result.Steps[0].Status != "success" {
		t.Errorf("expected status 'success', got %q", result.Steps[0].Status)
	}
	if !strings.Contains(result.Steps[0].StdoutRef, "ok") {
		t.Errorf("expected response body in stdout, got %q", result.Steps[0].StdoutRef)
	}
	if ctx.TmplCtx.StepOutputs["fetch"]["body"] == "" {
		t.Error("expected output to be captured")
	}
}

func TestHTTPStepExplainMode(t *testing.T) {
	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{
				ID:   "fetch",
				HTTP: &plan.HTTPRequest{URL: "https://example.com/api", Method: "POST"},
			},
		},
	}
	ctx := makeCtx(t, nil, false)
	result, err := Execute(p, ctx, ModeExplain)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Steps[0].Status != "explain" {
		t.Errorf("expected status 'explain', got %q", result.Steps[0].Status)
	}
	if !strings.Contains(result.Steps[0].Command, "POST") {
		t.Errorf("expected command to contain method, got %q", result.Steps[0].Command)
	}
}

func TestHTTPStepDryRunMode(t *testing.T) {
	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{
				ID:   "fetch",
				HTTP: &plan.HTTPRequest{URL: "https://example.com/api"},
			},
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
	if !strings.Contains(result.Steps[0].DryRunInfo, "GET") {
		t.Errorf("expected default method GET in dry run info, got %q", result.Steps[0].DryRunInfo)
	}
}

func TestHTTPStepDefaultMethodIsGET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{ID: "fetch", HTTP: &plan.HTTPRequest{URL: srv.URL}},
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
}

func TestHTTPStepWithBodyAndHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Token") != "secret" {
			t.Errorf("expected header X-Token=secret, got %q", r.Header.Get("X-Token"))
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Write([]byte("created"))
	}))
	defer srv.Close()

	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{
				ID: "create",
				HTTP: &plan.HTTPRequest{
					URL:     srv.URL,
					Method:  "POST",
					Headers: map[string]string{"X-Token": "secret"},
					Body:    `{"key":"value"}`,
				},
			},
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
}

func TestHTTPStepErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{ID: "fetch", HTTP: &plan.HTTPRequest{URL: srv.URL}},
		},
	}
	ctx := makeCtx(t, nil, false)
	result, err := Execute(p, ctx, ModeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for 500 response")
	}
	if result.Steps[0].Status != "failed" {
		t.Errorf("expected status 'failed', got %q", result.Steps[0].Status)
	}
}

func TestHTTPStepDestructiveBlocked(t *testing.T) {
	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{
				ID:          "destroy",
				HTTP:        &plan.HTTPRequest{URL: "https://example.com/delete", Method: "DELETE"},
				Destructive: true,
			},
		},
	}
	ctx := makeCtx(t, nil, false) // approve=false
	result, err := Execute(p, ctx, ModeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for blocked destructive step")
	}
	if result.Steps[0].Status != "blocked" {
		t.Errorf("expected status 'blocked', got %q", result.Steps[0].Status)
	}
}

func TestHTTPStepTemplateResolution(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{
				ID: "fetch",
				HTTP: &plan.HTTPRequest{
					URL: "${{inputs.base_url}}/api",
				},
				Outputs: map[string]string{"body": "stdout"},
			},
		},
	}
	ctx := makeCtx(t, map[string]string{"base_url": srv.URL}, false)
	result, err := Execute(p, ctx, ModeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, errors: %v", result.Errors)
	}
}

func TestActionStepUnknownAction(t *testing.T) {
	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{ID: "bad", Action: "nonexistent.action"},
		},
	}
	ctx := makeCtx(t, nil, false)
	_, err := Execute(p, ctx, ModeRun)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
	runErr, ok := err.(*dagerrors.RunError)
	if !ok {
		t.Fatalf("expected *RunError, got %T", err)
	}
	if runErr.Type != dagerrors.ToolNotFound {
		t.Errorf("expected TOOL_NOT_FOUND error type, got %q", runErr.Type)
	}
}

func TestActionStepExplainMode(t *testing.T) {
	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{
				ID:     "write",
				Action: "file.write",
				Params: map[string]string{"path": "/tmp/test.txt", "content": "hello"},
			},
		},
	}
	ctx := makeCtx(t, nil, false)
	result, err := Execute(p, ctx, ModeExplain)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Steps[0].Status != "explain" {
		t.Errorf("expected status 'explain', got %q", result.Steps[0].Status)
	}
	if !strings.Contains(result.Steps[0].Command, "file.write") {
		t.Errorf("expected action name in command, got %q", result.Steps[0].Command)
	}
}

func TestActionStepDryRunMode(t *testing.T) {
	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{
				ID:     "write",
				Action: "file.write",
				Params: map[string]string{"path": "/tmp/test.txt", "content": "hello"},
			},
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

func TestActionStepDestructiveBlocked(t *testing.T) {
	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{
				ID:          "write",
				Action:      "file.write",
				Params:      map[string]string{"path": "/tmp/test.txt", "content": "hello"},
				Destructive: true,
			},
		},
	}
	ctx := makeCtx(t, nil, false) // approve=false
	result, err := Execute(p, ctx, ModeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for blocked destructive action")
	}
	if result.Steps[0].Status != "blocked" {
		t.Errorf("expected status 'blocked', got %q", result.Steps[0].Status)
	}
}

func TestActionStepRelativePath(t *testing.T) {
	p := &plan.Plan{
		Name: "test",
		Steps: []plan.Step{
			{
				ID:      "write",
				Action:  "file.write",
				Params:  map[string]string{"path": "out.txt", "content": "hello"},
				Outputs: map[string]string{"result": "path"},
			},
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
	// The path should have been resolved relative to workdir
	if ctx.TmplCtx.StepOutputs["write"]["result"] == "" {
		t.Error("expected output to be captured")
	}
}

func TestEmptyPlan(t *testing.T) {
	p := &plan.Plan{Name: "empty"}
	ctx := makeCtx(t, nil, false)
	result, err := Execute(p, ctx, ModeRun)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success for empty plan")
	}
	if len(result.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(result.Steps))
	}
}

func TestResultJSON(t *testing.T) {
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
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}
	var decoded Result
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if decoded.RunID != result.RunID {
		t.Errorf("expected run_id %q, got %q", result.RunID, decoded.RunID)
	}
	if !decoded.Success {
		t.Error("expected success=true after round-trip")
	}
	if len(decoded.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(decoded.Steps))
	}
}

func TestNewRunContext(t *testing.T) {
	inputs := map[string]string{"env": "prod"}
	ctx := NewRunContext("/tmp", inputs, true)
	if ctx.RunID == "" {
		t.Error("expected non-empty run ID")
	}
	if ctx.WorkDir != "/tmp" {
		t.Errorf("expected workdir /tmp, got %q", ctx.WorkDir)
	}
	if !ctx.Approve {
		t.Error("expected approve=true")
	}
	if ctx.TmplCtx == nil {
		t.Fatal("expected non-nil template context")
	}
	if ctx.TmplCtx.Inputs["env"] != "prod" {
		t.Errorf("expected input env=prod, got %q", ctx.TmplCtx.Inputs["env"])
	}
}

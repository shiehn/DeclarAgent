package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stevehiehn/declaragent/internal/engine"
	dagerrors "github.com/stevehiehn/declaragent/internal/errors"
	"github.com/stevehiehn/declaragent/internal/plan"
)

// startTestAPI creates a test HTTP server with multiple routes for E2E testing.
func startTestAPI(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	})

	mux.HandleFunc("/data/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/data/")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"%s","name":"item-%s"}`, id, id)
	})

	mux.HandleFunc("/validate", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var obj map[string]any
		if err := json.Unmarshal(body, &obj); err != nil {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"invalid JSON"}`))
			return
		}
		if _, ok := obj["name"]; !ok {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"missing field: name"}`))
			return
		}
		w.Write([]byte(`{"valid":true}`))
	})

	mux.HandleFunc("/error/500", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"internal server error"}`))
	})

	mux.HandleFunc("/error/404", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"error":"not found"}`))
	})

	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"slow-ok"}`))
	})

	mux.HandleFunc("/headers", func(w http.ResponseWriter, r *http.Request) {
		headers := map[string]string{}
		for k, v := range r.Header {
			headers[k] = v[0]
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(headers)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// ============================================================
// Section 2: HTTP Step Tests
// ============================================================

func TestHTTPGetE2E(t *testing.T) {
	srv := startTestAPI(t)
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", fmt.Sprintf(`
name: http-get
steps:
  - id: get_status
    http:
      url: %s/status
    outputs:
      body: stdout
`, srv.URL))
	p := loadPlan(t, dir+"/plan.yaml")
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("expected success, failed at %s", result.FailedStepID)
	}
	if !strings.Contains(result.Steps[0].StdoutRef, `"status":"ok"`) {
		t.Fatalf("expected status ok in response, got %q", result.Steps[0].StdoutRef)
	}
}

func TestHTTPPostWithBodyE2E(t *testing.T) {
	srv := startTestAPI(t)
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", fmt.Sprintf(`
name: http-post
steps:
  - id: post_echo
    http:
      url: %s/echo
      method: POST
      body: '{"greeting":"hello"}'
    outputs:
      body: stdout
`, srv.URL))
	p := loadPlan(t, dir+"/plan.yaml")
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("expected success, failed at %s", result.FailedStepID)
	}
	if !strings.Contains(result.Steps[0].StdoutRef, `"greeting":"hello"`) {
		t.Fatalf("expected echoed body, got %q", result.Steps[0].StdoutRef)
	}
}

func TestHTTPHeadersE2E(t *testing.T) {
	srv := startTestAPI(t)
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", fmt.Sprintf(`
name: http-headers
steps:
  - id: send_headers
    http:
      url: %s/headers
      headers:
        X-Custom-Token: my-secret-token
        X-Request-Id: req-123
    outputs:
      body: stdout
`, srv.URL))
	p := loadPlan(t, dir+"/plan.yaml")
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("expected success, failed at %s", result.FailedStepID)
	}
	body := result.Steps[0].StdoutRef
	if !strings.Contains(body, "my-secret-token") {
		t.Fatalf("expected custom header in response, got %q", body)
	}
	if !strings.Contains(body, "req-123") {
		t.Fatalf("expected request id header in response, got %q", body)
	}
}

func TestHTTPChainOutputsE2E(t *testing.T) {
	srv := startTestAPI(t)
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", fmt.Sprintf(`
name: http-chain
steps:
  - id: fetch_data
    http:
      url: %s/data/42
    outputs:
      body: stdout
  - id: echo_back
    http:
      url: %s/echo
      method: POST
      body: '${{steps.fetch_data.outputs.body}}'
    outputs:
      body: stdout
`, srv.URL, srv.URL))
	p := loadPlan(t, dir+"/plan.yaml")
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("expected success, failed at %s", result.FailedStepID)
	}
	// Step 2 should echo back step 1's response
	if !strings.Contains(result.Steps[1].StdoutRef, `"id":"42"`) {
		t.Fatalf("expected chained data in step 2, got %q", result.Steps[1].StdoutRef)
	}
}

func TestHTTPErrorStatusE2E(t *testing.T) {
	srv := startTestAPI(t)
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", fmt.Sprintf(`
name: http-500
steps:
  - id: server_error
    http:
      url: %s/error/500
`, srv.URL))
	p := loadPlan(t, dir+"/plan.yaml")
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatal("expected failure for 500 error")
	}
	if result.Steps[0].Status != "failed" {
		t.Fatalf("expected failed status, got %s", result.Steps[0].Status)
	}
	if !strings.Contains(result.Steps[0].StderrRef, "500") {
		t.Fatalf("expected 500 in error, got %q", result.Steps[0].StderrRef)
	}
}

func TestHTTP404E2E(t *testing.T) {
	srv := startTestAPI(t)
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", fmt.Sprintf(`
name: http-404
steps:
  - id: not_found
    http:
      url: %s/error/404
`, srv.URL))
	p := loadPlan(t, dir+"/plan.yaml")
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatal("expected failure for 404")
	}
	if result.Steps[0].Status != "failed" {
		t.Fatalf("expected failed status, got %s", result.Steps[0].Status)
	}
}

func TestHTTPPostValidationFailureE2E(t *testing.T) {
	srv := startTestAPI(t)
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", fmt.Sprintf(`
name: http-validation
steps:
  - id: bad_post
    http:
      url: %s/validate
      method: POST
      body: '{"age":25}'
`, srv.URL))
	p := loadPlan(t, dir+"/plan.yaml")
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatal("expected failure for validation error")
	}
	if !strings.Contains(result.Steps[0].StderrRef, "400") {
		t.Fatalf("expected 400 in error, got %q", result.Steps[0].StderrRef)
	}
}

// ============================================================
// Section 3: Bash ↔ HTTP Chaining Tests
// ============================================================

func TestBashToHTTPChainE2E(t *testing.T) {
	srv := startTestAPI(t)
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", fmt.Sprintf(`
name: bash-to-http
steps:
  - id: gen_id
    run: printf "99"
    outputs:
      id: stdout
  - id: fetch
    http:
      url: %s/data/${{steps.gen_id.outputs.id}}
    outputs:
      body: stdout
`, srv.URL))
	p := loadPlan(t, dir+"/plan.yaml")
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("expected success, failed at %s: %v", result.FailedStepID, result.Errors)
	}
	if !strings.Contains(result.Steps[1].StdoutRef, `"id":"99"`) {
		t.Fatalf("expected id 99 in response, got %q", result.Steps[1].StdoutRef)
	}
}

func TestHTTPToBashChainE2E(t *testing.T) {
	srv := startTestAPI(t)
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", fmt.Sprintf(`
name: http-to-bash
steps:
  - id: fetch
    http:
      url: %s/data/77
    outputs:
      body: stdout
  - id: process
    run: echo "fetched ${{steps.fetch.outputs.body}}"
    outputs:
      result: stdout
`, srv.URL))
	p := loadPlan(t, dir+"/plan.yaml")
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("expected success, failed at %s", result.FailedStepID)
	}
	if !strings.Contains(result.Steps[1].StdoutRef, "fetched") || !strings.Contains(result.Steps[1].StdoutRef, "77") {
		t.Fatalf("expected fetched data with id 77 in bash output, got %q", result.Steps[1].StdoutRef)
	}
}

func TestMultiStepChainE2E(t *testing.T) {
	srv := startTestAPI(t)
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", fmt.Sprintf(`
name: multi-chain
steps:
  - id: gen
    run: printf "55"
    outputs:
      val: stdout
  - id: http1
    http:
      url: %s/data/${{steps.gen.outputs.val}}
    outputs:
      body: stdout
  - id: extract
    run: printf "${{steps.http1.outputs.body}}"
    outputs:
      data: stdout
  - id: http2
    http:
      url: %s/echo
      method: POST
      body: '${{steps.extract.outputs.data}}'
    outputs:
      final: stdout
`, srv.URL, srv.URL))
	p := loadPlan(t, dir+"/plan.yaml")
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("expected success, failed at %s: %v", result.FailedStepID, result.Errors)
	}
	if len(result.Steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(result.Steps))
	}
	// Final step should echo back the data from step 1→2→3
	if !strings.Contains(result.Steps[3].StdoutRef, "55") {
		t.Fatalf("expected chained data with id 55 in final step, got %q", result.Steps[3].StdoutRef)
	}
}

// ============================================================
// Section 4: Bash Exit Code & Error Handling Tests
// ============================================================

func TestBashExitCode0E2E(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", `
name: exit0
steps:
  - id: ok
    run: echo "success"
`)
	p := loadPlan(t, dir+"/plan.yaml")
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Steps[0].ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.Steps[0].ExitCode)
	}
}

func TestBashExitCode1E2E(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", `
name: exit1
steps:
  - id: fail
    run: exit 1
`)
	p := loadPlan(t, dir+"/plan.yaml")
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatal("expected failure")
	}
	if result.Steps[0].ExitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", result.Steps[0].ExitCode)
	}
}

func TestBashExitCode2E2E(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", `
name: exit2
steps:
  - id: fail
    run: exit 2
`)
	p := loadPlan(t, dir+"/plan.yaml")
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatal("expected failure")
	}
	if result.Steps[0].ExitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", result.Steps[0].ExitCode)
	}
}

func TestBashStderrCapturedE2E(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", `
name: stderr
steps:
  - id: err
    run: echo "error message" >&2 && exit 1
`)
	p := loadPlan(t, dir+"/plan.yaml")
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatal("expected failure")
	}
	if !strings.Contains(result.Steps[0].StderrRef, "error message") {
		t.Fatalf("expected stderr captured, got %q", result.Steps[0].StderrRef)
	}
}

func TestBashPipeFailureE2E(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", `
name: pipe-fail
steps:
  - id: pipe
    run: echo hello | grep nonexistent
`)
	p := loadPlan(t, dir+"/plan.yaml")
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatal("expected failure from pipe")
	}
	if result.Steps[0].ExitCode == 0 {
		t.Fatal("expected non-zero exit code")
	}
}

func TestBashCommandNotFoundE2E(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", `
name: cmd-not-found
steps:
  - id: bad
    run: totally_nonexistent_command_xyz_123
`)
	p := loadPlan(t, dir+"/plan.yaml")
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatal("expected failure for nonexistent command")
	}
	if result.Steps[0].ExitCode == 0 {
		t.Fatal("expected non-zero exit code")
	}
}

// ============================================================
// Section 5: Validation Feedback Tests
// ============================================================

func TestValidationMissingNameE2E(t *testing.T) {
	_, err := plan.Load([]byte(`
steps:
  - id: s1
    run: echo hi
`))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "no name") {
		t.Fatalf("expected 'no name' in error, got %q", err.Error())
	}
}

func TestValidationDuplicateStepIDE2E(t *testing.T) {
	p, err := plan.Load([]byte(`
name: dup
steps:
  - id: s1
    run: echo a
  - id: s1
    run: echo b
`))
	if err != nil {
		t.Fatal(err)
	}
	verr := plan.Validate(p, nil)
	if verr == nil {
		t.Fatal("expected validation error for duplicate step id")
	}
	if !strings.Contains(verr.Error(), "duplicate") {
		t.Fatalf("expected 'duplicate' in error, got %q", verr.Error())
	}
}

func TestValidationForwardReferenceE2E(t *testing.T) {
	p, err := plan.Load([]byte(`
name: fwd
steps:
  - id: s1
    run: echo "${{steps.s2.outputs.val}}"
  - id: s2
    run: echo hi
    outputs:
      val: stdout
`))
	if err != nil {
		t.Fatal(err)
	}
	verr := plan.Validate(p, nil)
	if verr == nil {
		t.Fatal("expected validation error for forward reference")
	}
	if !strings.Contains(verr.Error(), "s2") {
		t.Fatalf("expected step id 's2' in error, got %q", verr.Error())
	}
}

func TestValidationUnknownActionE2E(t *testing.T) {
	p, err := plan.Load([]byte(`
name: bad-action
steps:
  - id: s1
    action: totally.fake
    with:
      foo: bar
`))
	if err != nil {
		t.Fatal(err)
	}
	verr := plan.Validate(p, nil)
	if verr == nil {
		t.Fatal("expected validation error for unknown action")
	}
	if !strings.Contains(verr.Error(), "totally.fake") {
		t.Fatalf("expected action name in error, got %q", verr.Error())
	}
}

func TestValidationMissingRequiredInputE2E(t *testing.T) {
	p, err := plan.Load([]byte(`
name: missing-input
inputs:
  token:
    required: true
steps:
  - id: s1
    run: echo "${{inputs.token}}"
`))
	if err != nil {
		t.Fatal(err)
	}
	verr := plan.Validate(p, map[string]string{})
	if verr == nil {
		t.Fatal("expected validation error for missing required input")
	}
	if !strings.Contains(verr.Error(), "token") {
		t.Fatalf("expected input name in error, got %q", verr.Error())
	}
}

func TestValidationBothRunAndActionE2E(t *testing.T) {
	p, err := plan.Load([]byte(`
name: both
steps:
  - id: s1
    run: echo hi
    action: file.write
    with:
      path: x
      content: y
`))
	if err != nil {
		t.Fatal(err)
	}
	verr := plan.Validate(p, nil)
	if verr == nil {
		t.Fatal("expected validation error for both run and action")
	}
	if !strings.Contains(verr.Error(), "multiple") {
		t.Fatalf("expected 'multiple' in error, got %q", verr.Error())
	}
}

func TestValidationHTTPMissingURLStepE2E(t *testing.T) {
	p, err := plan.Load([]byte(`
name: no-url
steps:
  - id: s1
    http:
      method: GET
`))
	if err != nil {
		t.Fatal(err)
	}
	verr := plan.Validate(p, nil)
	if verr == nil {
		t.Fatal("expected validation error for HTTP without URL")
	}
	if !strings.Contains(verr.Error(), "url") {
		t.Fatalf("expected 'url' in error, got %q", verr.Error())
	}
}

func TestValidationFeedbackContainsHintE2E(t *testing.T) {
	p, err := plan.Load([]byte(`
name: hint-test
steps:
  - id: s1
    run: echo hi
    action: file.write
    with:
      path: x
      content: y
`))
	if err != nil {
		t.Fatal(err)
	}
	verr := plan.Validate(p, nil)
	if verr == nil {
		t.Fatal("expected validation error")
	}
	runErr, ok := verr.(*dagerrors.RunError)
	if !ok {
		t.Fatalf("expected *RunError, got %T", verr)
	}
	if runErr.Hint == "" {
		t.Fatal("expected non-empty Hint field")
	}
}

// ============================================================
// Section 6 & 7: LLM/MCP tests are in internal/mcp/mcp_e2e_test.go
// ============================================================

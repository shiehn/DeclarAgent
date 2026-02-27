package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stevehiehn/declaragent/internal/engine"
	"github.com/stevehiehn/declaragent/internal/plan"
)

func TestSimpleShellPlanE2E(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", `
name: simple
steps:
  - id: hello
    run: echo "hello world"
    outputs:
      message: stdout
  - id: check
    run: echo "got ${{steps.hello.outputs.message}}"
    outputs:
      result: stdout
`)
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	p := loadPlan(t, filepath.Join(dir, "plan.yaml"))
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("expected success, got failure at step %s", result.FailedStepID)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(result.Steps))
	}
	if result.Steps[0].Status != "success" || result.Steps[1].Status != "success" {
		t.Fatal("expected both steps success")
	}
}

func TestDataFlowE2E(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", `
name: dataflow
steps:
  - id: produce
    run: echo "my-value"
    outputs:
      val: stdout
  - id: consume
    run: echo "received ${{steps.produce.outputs.val}}"
    outputs:
      result: stdout
`)
	p := loadPlan(t, filepath.Join(dir, "plan.yaml"))
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	// step 2 stdout should contain "received my-value"
	stdout := result.Steps[1].StdoutRef
	if !strings.Contains(stdout, "received my-value") {
		t.Fatalf("expected 'received my-value' in stdout, got %q", stdout)
	}
}

func TestBuiltinActionsE2E(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", `
name: actions-test
steps:
  - id: write_json
    action: json.set
    with:
      file: `+filepath.Join(dir, "data.json")+`
      path: foo.bar
      value: hello
  - id: read_json
    action: json.get
    with:
      file: `+filepath.Join(dir, "data.json")+`
      path: foo.bar
    outputs:
      val: value
  - id: write_file
    action: file.write
    with:
      path: `+filepath.Join(dir, "out.txt")+`
      content: done
`)
	p := loadPlan(t, filepath.Join(dir, "plan.yaml"))
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("expected success, failed at %s: %v", result.FailedStepID, result.Errors)
	}
	// Check file exists
	data, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "done" {
		t.Fatalf("expected 'done', got %q", string(data))
	}
}

func TestFailFastE2E(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", `
name: failfast
steps:
  - id: step1
    run: echo ok
  - id: step2
    run: exit 1
  - id: step3
    run: echo "should not run"
`)
	p := loadPlan(t, filepath.Join(dir, "plan.yaml"))
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatal("expected failure")
	}
	if result.FailedStepID != "step2" {
		t.Fatalf("expected failed_step_id=step2, got %s", result.FailedStepID)
	}
	if result.Steps[2].Status != "skipped" {
		t.Fatalf("expected step3 skipped, got %s", result.Steps[2].Status)
	}
}

func TestDestructiveSafetyE2E(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", `
name: destructive-test
steps:
  - id: safe
    run: echo ok
  - id: dangerous
    run: echo "boom"
    destructive: true
`)
	p := loadPlan(t, filepath.Join(dir, "plan.yaml"))

	// Without approve
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatal("expected failure due to blocked destructive step")
	}
	if result.FailedStepID != "dangerous" {
		t.Fatalf("expected failed at dangerous, got %s", result.FailedStepID)
	}

	// With approve
	ctx2 := engine.NewRunContext(dir, map[string]string{}, true)
	result2, err := engine.Execute(p, ctx2, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if !result2.Success {
		t.Fatal("expected success with approve")
	}
}

func TestDryRunE2E(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", `
name: dryrun-test
steps:
  - id: step1
    run: echo hello
  - id: write_file
    action: file.write
    with:
      path: `+filepath.Join(dir, "should-not-exist.txt")+`
      content: nope
`)
	p := loadPlan(t, filepath.Join(dir, "plan.yaml"))
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeDryRun)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatal("expected dry-run success")
	}
	// File should NOT exist
	if _, err := os.Stat(filepath.Join(dir, "should-not-exist.txt")); !os.IsNotExist(err) {
		t.Fatal("file should not exist after dry-run")
	}
}

func TestArtifactPersistenceE2E(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", `
name: artifact-test
steps:
  - id: hello
    run: echo "hello artifact"
`)
	p := loadPlan(t, filepath.Join(dir, "plan.yaml"))
	ctx := engine.NewRunContext(dir, map[string]string{}, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	// Check artifact dir
	artifactDir := filepath.Join(dir, ".declaragent", "runs", ctx.RunID)
	if _, err := os.Stat(filepath.Join(artifactDir, "result.json")); err != nil {
		t.Fatalf("result.json should exist: %v", err)
	}
	// Verify result.json is valid JSON
	data, _ := os.ReadFile(filepath.Join(artifactDir, "result.json"))
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("result.json should be valid JSON: %v", err)
	}
	if parsed["run_id"] != result.RunID {
		t.Fatal("run_id mismatch in result.json")
	}
}

func TestDeterminismE2E(t *testing.T) {
	dir := t.TempDir()
	writePlan(t, dir, "plan.yaml", `
name: determinism
steps:
  - id: hello
    run: echo "deterministic"
    outputs:
      msg: stdout
`)
	p := loadPlan(t, filepath.Join(dir, "plan.yaml"))

	ctx1 := engine.NewRunContext(dir, map[string]string{}, false)
	r1, _ := engine.Execute(p, ctx1, engine.ModeRun)
	ctx2 := engine.NewRunContext(dir, map[string]string{}, false)
	r2, _ := engine.Execute(p, ctx2, engine.ModeRun)

	// Outputs should match
	if r1.Steps[0].StdoutRef != r2.Steps[0].StdoutRef {
		t.Fatal("outputs should be deterministic")
	}
}

func TestGitWorkflowE2E(t *testing.T) {
	// Check git is available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	// Init git repo
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	// Create initial commit
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0o644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "init")
	// Create branch
	runGit(t, dir, "checkout", "-b", "ABC-123-fix-login")
	// Create fakejira.json
	os.WriteFile(filepath.Join(dir, "fakejira.json"), []byte("{}"), 0o644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add jira")

	writePlan(t, dir, "plan.yaml", `
name: git-jira-workflow
description: Extract ticket from branch, update local Jira, append changelog, commit
inputs:
  commit_message:
    required: true
    description: The commit message
steps:
  - id: check_clean
    name: Ensure working tree is clean
    run: git diff --quiet && git diff --cached --quiet
  - id: get_branch
    run: git rev-parse --abbrev-ref HEAD
    outputs:
      branch: stdout
  - id: extract_ticket
    run: echo "${{steps.get_branch.outputs.branch}}" | grep -oE '[A-Z]+-[0-9]+'
    outputs:
      ticket_id: stdout
  - id: update_jira
    action: json.set
    with:
      file: fakejira.json
      path: "${{steps.extract_ticket.outputs.ticket_id}}.status"
      value: "In Review"
  - id: append_changelog
    action: file.append
    with:
      path: CHANGELOG.md
      content: "- [${{steps.extract_ticket.outputs.ticket_id}}] ${{inputs.commit_message}}\n"
  - id: commit
    run: git add -A && git commit -m "[${{steps.extract_ticket.outputs.ticket_id}}] ${{inputs.commit_message}}"
    destructive: true
`)
	p := loadPlan(t, filepath.Join(dir, "plan.yaml"))
	ctx := engine.NewRunContext(dir, map[string]string{"commit_message": "fix login bug"}, true)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("expected success, failed at %s: %v", result.FailedStepID, result.Errors)
	}

	// Verify fakejira.json updated
	data, _ := os.ReadFile(filepath.Join(dir, "fakejira.json"))
	if !strings.Contains(string(data), "In Review") {
		t.Fatal("fakejira.json should contain 'In Review'")
	}

	// Verify CHANGELOG.md
	cl, _ := os.ReadFile(filepath.Join(dir, "CHANGELOG.md"))
	if !strings.Contains(string(cl), "[ABC-123] fix login bug") {
		t.Fatalf("CHANGELOG should contain ticket ref, got %q", string(cl))
	}

	// Verify git commit
	out, _ := exec.Command("git", "-C", dir, "log", "--oneline", "-1").Output()
	if !strings.Contains(string(out), "[ABC-123] fix login bug") {
		t.Fatalf("git log should show commit, got %q", string(out))
	}
}

func writePlan(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func loadPlan(t *testing.T, path string) *plan.Plan {
	t.Helper()
	p, err := plan.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}

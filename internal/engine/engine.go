package engine

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/stevehiehn/declaragent/internal/action"
	"github.com/stevehiehn/declaragent/internal/artifact"
	dagerrors "github.com/stevehiehn/declaragent/internal/errors"
	"github.com/stevehiehn/declaragent/internal/plan"
	"github.com/stevehiehn/declaragent/internal/runner"
	"github.com/stevehiehn/declaragent/internal/template"
)

// Mode controls execution behavior.
type Mode int

const (
	ModeExplain Mode = iota
	ModeDryRun
	ModeRun
)

// Execute runs a plan in the given mode.
func Execute(p *plan.Plan, ctx *RunContext, mode Mode) (*Result, error) {
	result := &Result{
		RunID:   ctx.RunID,
		Success: true,
		Outputs: map[string]string{},
	}

	var store *artifact.Store
	if mode == ModeRun {
		var err error
		store, err = artifact.New(ctx.RunID, ctx.WorkDir)
		if err != nil {
			return nil, err
		}
		result.Artifacts = []string{store.BaseDir}
	}

	failed := false
	for _, step := range p.Steps {
		if failed {
			result.Steps = append(result.Steps, StepResult{ID: step.ID, Status: "skipped"})
			continue
		}

		sr, err := executeStep(step, ctx, mode)
		if err != nil {
			return nil, err
		}

		result.Steps = append(result.Steps, *sr)

		if sr.Status == "failed" || sr.Status == "blocked" {
			result.Success = false
			result.FailedStepID = step.ID
			failed = true
			if sr.Status == "failed" {
				result.Errors = append(result.Errors, dagerrors.RunError{
					Type:    dagerrors.StepFailed,
					StepID:  step.ID,
					Message: fmt.Sprintf("step %q failed with exit code %d", step.ID, sr.ExitCode),
					Hint:    fmt.Sprintf("Check %s for details", sr.StderrRef),
				})
			} else {
				result.Errors = append(result.Errors, dagerrors.RunError{
					Type:    dagerrors.SideEffectBlocked,
					StepID:  step.ID,
					Message: fmt.Sprintf("step %q is destructive and --approve was not set", step.ID),
					Hint:    "Re-run with --approve to allow destructive steps",
				})
			}
		}

		// Store artifacts for run mode
		if mode == ModeRun && store != nil && sr.Status == "success" {
			_ = store.WriteStepOutput(step.ID, sr.StdoutRef, sr.StderrRef)
		}
	}

	if mode == ModeRun && store != nil {
		_ = store.WriteResult(result)
	}

	return result, nil
}

func executeStep(step plan.Step, ctx *RunContext, mode Mode) (*StepResult, error) {
	sr := &StepResult{ID: step.ID, Description: step.Description}

	if step.Run != "" {
		return executeRunStep(step, ctx, mode, sr)
	}
	if step.HTTP != nil {
		return executeHTTPStep(step, ctx, mode, sr)
	}
	return executeActionStep(step, ctx, mode, sr)
}

func executeRunStep(step plan.Step, ctx *RunContext, mode Mode, sr *StepResult) (*StepResult, error) {
	resolved, err := template.Resolve(step.Run, ctx.TmplCtx)
	if err != nil {
		return nil, fmt.Errorf("resolving template for step %q: %w", step.ID, err)
	}
	sr.Command = resolved

	if mode == ModeExplain {
		sr.Status = "explain"
		registerPlaceholderOutputs(step, ctx)
		return sr, nil
	}

	if step.Destructive && !ctx.Approve {
		sr.Status = "blocked"
		registerPlaceholderOutputs(step, ctx)
		return sr, nil
	}

	if mode == ModeDryRun {
		sr.Status = "dry-run"
		sr.DryRunInfo = fmt.Sprintf("Would run: %s", resolved)
		registerPlaceholderOutputs(step, ctx)
		return sr, nil
	}

	// ModeRun
	start := time.Now()
	shellResult := runner.Run(resolved, ctx.WorkDir)
	sr.Duration = time.Since(start).Round(time.Millisecond).String()
	sr.ExitCode = shellResult.ExitCode
	sr.StdoutRef = shellResult.Stdout
	sr.StderrRef = shellResult.Stderr

	if shellResult.ExitCode != 0 {
		sr.Status = "failed"
		return sr, nil
	}

	sr.Status = "success"

	// Extract outputs
	if step.Outputs != nil {
		if ctx.TmplCtx.StepOutputs[step.ID] == nil {
			ctx.TmplCtx.StepOutputs[step.ID] = map[string]string{}
		}
		for name, source := range step.Outputs {
			if source == "stdout" {
				ctx.TmplCtx.StepOutputs[step.ID][name] = strings.TrimSpace(shellResult.Stdout)
			}
		}
	}

	return sr, nil
}

func executeHTTPStep(step plan.Step, ctx *RunContext, mode Mode, sr *StepResult) (*StepResult, error) {
	// Resolve templates in HTTP fields
	resolvedURL, err := template.Resolve(step.HTTP.URL, ctx.TmplCtx)
	if err != nil {
		return nil, fmt.Errorf("resolving url for step %q: %w", step.ID, err)
	}

	method := step.HTTP.Method
	if method == "" {
		method = "GET"
	}
	sr.Command = fmt.Sprintf("%s %s", method, resolvedURL)

	if mode == ModeExplain {
		sr.Status = "explain"
		registerPlaceholderOutputs(step, ctx)
		return sr, nil
	}

	if step.Destructive && !ctx.Approve {
		sr.Status = "blocked"
		registerPlaceholderOutputs(step, ctx)
		return sr, nil
	}

	if mode == ModeDryRun {
		sr.Status = "dry-run"
		sr.DryRunInfo = fmt.Sprintf("Would send %s to %s", method, resolvedURL)
		registerPlaceholderOutputs(step, ctx)
		return sr, nil
	}

	// Build action params from HTTP struct
	params := map[string]string{
		"url":    resolvedURL,
		"method": method,
	}

	if step.HTTP.Body != "" {
		resolvedBody, err := template.Resolve(step.HTTP.Body, ctx.TmplCtx)
		if err != nil {
			return nil, fmt.Errorf("resolving body for step %q: %w", step.ID, err)
		}
		params["body"] = resolvedBody
	}

	for k, v := range step.HTTP.Headers {
		resolvedHeader, err := template.Resolve(v, ctx.TmplCtx)
		if err != nil {
			return nil, fmt.Errorf("resolving header %q for step %q: %w", k, step.ID, err)
		}
		params["header_"+k] = resolvedHeader
	}

	// Execute via the http action
	act, _ := action.Get("http")
	start := time.Now()
	outputs, err := act.Execute(params)
	sr.Duration = time.Since(start).Round(time.Millisecond).String()

	if err != nil {
		sr.Status = "failed"
		sr.StderrRef = err.Error()
		return sr, nil
	}

	sr.Status = "success"
	sr.StdoutRef = outputs["stdout"]

	// Extract outputs (stdout is the response body)
	if step.Outputs != nil {
		if ctx.TmplCtx.StepOutputs[step.ID] == nil {
			ctx.TmplCtx.StepOutputs[step.ID] = map[string]string{}
		}
		for name, source := range step.Outputs {
			if val, ok := outputs[source]; ok {
				ctx.TmplCtx.StepOutputs[step.ID][name] = strings.TrimSpace(val)
			}
		}
	}

	return sr, nil
}

func executeActionStep(step plan.Step, ctx *RunContext, mode Mode, sr *StepResult) (*StepResult, error) {
	act, err := action.Get(step.Action)
	if err != nil {
		return nil, &dagerrors.RunError{Type: dagerrors.ToolNotFound, StepID: step.ID, Message: err.Error()}
	}

	// Resolve templates in params
	resolvedParams := map[string]string{}
	for k, v := range step.Params {
		resolved, err := template.Resolve(v, ctx.TmplCtx)
		if err != nil {
			return nil, fmt.Errorf("resolving param %q for step %q: %w", k, step.ID, err)
		}
		// Resolve relative file paths against workdir
		if (k == "path" || k == "file") && !filepath.IsAbs(resolved) && ctx.WorkDir != "" {
			resolved = filepath.Join(ctx.WorkDir, resolved)
		}
		resolvedParams[k] = resolved
	}

	if mode == ModeExplain {
		sr.Status = "explain"
		sr.Command = fmt.Sprintf("action: %s", step.Action)
		sr.DryRunInfo = act.DryRun(resolvedParams)
		registerPlaceholderOutputs(step, ctx)
		return sr, nil
	}

	if step.Destructive && !ctx.Approve {
		sr.Status = "blocked"
		registerPlaceholderOutputs(step, ctx)
		return sr, nil
	}

	if mode == ModeDryRun {
		sr.Status = "dry-run"
		sr.DryRunInfo = act.DryRun(resolvedParams)
		registerPlaceholderOutputs(step, ctx)
		return sr, nil
	}

	// ModeRun
	start := time.Now()
	outputs, err := act.Execute(resolvedParams)
	sr.Duration = time.Since(start).Round(time.Millisecond).String()

	if err != nil {
		sr.Status = "failed"
		sr.StderrRef = err.Error()
		return sr, nil
	}

	sr.Status = "success"

	// Register outputs
	if step.Outputs != nil {
		if ctx.TmplCtx.StepOutputs[step.ID] == nil {
			ctx.TmplCtx.StepOutputs[step.ID] = map[string]string{}
		}
		for name, source := range step.Outputs {
			if val, ok := outputs[source]; ok {
				ctx.TmplCtx.StepOutputs[step.ID][name] = val
			}
		}
	}

	return sr, nil
}

// registerPlaceholderOutputs sets placeholder values for outputs so subsequent
// steps can resolve templates in explain/dry-run modes.
func registerPlaceholderOutputs(step plan.Step, ctx *RunContext) {
	if len(step.Outputs) == 0 {
		return
	}
	if ctx.TmplCtx.StepOutputs[step.ID] == nil {
		ctx.TmplCtx.StepOutputs[step.ID] = map[string]string{}
	}
	for name, source := range step.Outputs {
		ctx.TmplCtx.StepOutputs[step.ID][name] = fmt.Sprintf("<%s.%s>", step.ID, source)
	}
}

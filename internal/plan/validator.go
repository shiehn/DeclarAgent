package plan

import (
	"fmt"
	"regexp"

	dagerrors "github.com/stevehiehn/declaragent/internal/errors"
)

var knownActions = map[string]bool{
	"file.write":  true,
	"file.append": true,
	"json.get":    true,
	"json.set":    true,
	"env.get":     true,
}

var templateRefRe = regexp.MustCompile(`\{\{steps\.([^.}]+)\.outputs\.([^}]+)\}\}`)
var templateInputRe = regexp.MustCompile(`\{\{inputs\.([^}]+)\}\}`)

// Validate checks a plan for structural correctness.
func Validate(p *Plan, providedInputs map[string]string) error {
	seen := map[string]int{}
	stepOutputs := map[string]map[string]bool{}

	// Check required inputs (skip if providedInputs is nil, e.g. validate-only mode)
	if providedInputs != nil {
		for name, inp := range p.Inputs {
			if inp.Required {
				if _, ok := providedInputs[name]; !ok {
					if inp.Default == "" {
						return &dagerrors.RunError{
							Type:    dagerrors.ValidationError,
							Message: fmt.Sprintf("missing required input %q", name),
							Hint:    fmt.Sprintf("Provide --input %s=<value>", name),
						}
					}
				}
			}
		}
	}

	for i, s := range p.Steps {
		// Duplicate ID check
		if s.ID == "" {
			return &dagerrors.RunError{
				Type:    dagerrors.ValidationError,
				Message: fmt.Sprintf("step at index %d has no id", i),
			}
		}
		if _, dup := seen[s.ID]; dup {
			return &dagerrors.RunError{
				Type:    dagerrors.ValidationError,
				Message: fmt.Sprintf("duplicate step id %q", s.ID),
			}
		}
		seen[s.ID] = i

		// run XOR action
		hasRun := s.Run != ""
		hasAction := s.Action != ""
		if hasRun && hasAction {
			return &dagerrors.RunError{
				Type:    dagerrors.ValidationError,
				Message: fmt.Sprintf("step %q has both run and action", s.ID),
				Hint:    "A step must have either run or action, not both",
			}
		}
		if !hasRun && !hasAction {
			return &dagerrors.RunError{
				Type:    dagerrors.ValidationError,
				Message: fmt.Sprintf("step %q has neither run nor action", s.ID),
				Hint:    "A step must have either run or action",
			}
		}

		// Check action name
		if hasAction {
			if !knownActions[s.Action] {
				return &dagerrors.RunError{
					Type:    dagerrors.ToolNotFound,
					Message: fmt.Sprintf("step %q: unknown action %q", s.ID, s.Action),
					Hint:    "Known actions: file.write, file.append, json.get, json.set, env.get",
				}
			}
		}

		// Collect template refs from run, params, outputs
		refs := collectTemplateRefs(s)
		for _, ref := range refs {
			// Check forward references
			idx, exists := seen[ref.stepID]
			if !exists {
				return &dagerrors.RunError{
					Type:    dagerrors.ValidationError,
					Message: fmt.Sprintf("step %q references unknown step %q", s.ID, ref.stepID),
				}
			}
			if idx >= i {
				return &dagerrors.RunError{
					Type:    dagerrors.ValidationError,
					Message: fmt.Sprintf("step %q has forward reference to step %q", s.ID, ref.stepID),
				}
			}
			// Check output name exists
			if outs, ok := stepOutputs[ref.stepID]; ok {
				if !outs[ref.outputName] {
					return &dagerrors.RunError{
						Type:    dagerrors.ValidationError,
						Message: fmt.Sprintf("step %q references non-existent output %q on step %q", s.ID, ref.outputName, ref.stepID),
					}
				}
			} else {
				return &dagerrors.RunError{
					Type:    dagerrors.ValidationError,
					Message: fmt.Sprintf("step %q references step %q which has no outputs", s.ID, ref.stepID),
				}
			}
		}

		// Check input refs
		inputRefs := collectInputRefs(s)
		for _, name := range inputRefs {
			if _, ok := p.Inputs[name]; !ok {
				return &dagerrors.RunError{
					Type:    dagerrors.ValidationError,
					Message: fmt.Sprintf("step %q references unknown input %q", s.ID, name),
				}
			}
		}

		// Register outputs
		if len(s.Outputs) > 0 {
			stepOutputs[s.ID] = map[string]bool{}
			for k := range s.Outputs {
				stepOutputs[s.ID][k] = true
			}
		}
	}

	return nil
}

type templateRef struct {
	stepID     string
	outputName string
}

func collectTemplateRefs(s Step) []templateRef {
	var refs []templateRef
	for _, str := range stepStrings(s) {
		for _, m := range templateRefRe.FindAllStringSubmatch(str, -1) {
			refs = append(refs, templateRef{stepID: m[1], outputName: m[2]})
		}
	}
	return refs
}

func collectInputRefs(s Step) []string {
	var refs []string
	for _, str := range stepStrings(s) {
		for _, m := range templateInputRe.FindAllStringSubmatch(str, -1) {
			refs = append(refs, m[1])
		}
	}
	return refs
}

func stepStrings(s Step) []string {
	var strs []string
	strs = append(strs, s.Run)
	for _, v := range s.Params {
		strs = append(strs, v)
	}
	return strs
}

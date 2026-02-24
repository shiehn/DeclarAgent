package template

import (
	"fmt"
	"regexp"
)

var stepRefRe = regexp.MustCompile(`\{\{steps\.([^.}]+)\.outputs\.([^}]+)\}\}`)
var inputRefRe = regexp.MustCompile(`\{\{inputs\.([^}]+)\}\}`)

// Context holds available values for template resolution.
type Context struct {
	Inputs      map[string]string
	StepOutputs map[string]map[string]string // stepID → outputName → value
}

// Resolve replaces all {{steps.X.outputs.Y}} and {{inputs.Z}} in s.
func Resolve(s string, ctx *Context) (string, error) {
	var resolveErr error

	result := stepRefRe.ReplaceAllStringFunc(s, func(match string) string {
		m := stepRefRe.FindStringSubmatch(match)
		stepID, outputName := m[1], m[2]
		outs, ok := ctx.StepOutputs[stepID]
		if !ok {
			resolveErr = fmt.Errorf("unresolved step reference %q", stepID)
			return match
		}
		val, ok := outs[outputName]
		if !ok {
			resolveErr = fmt.Errorf("unresolved output %q on step %q", outputName, stepID)
			return match
		}
		return val
	})
	if resolveErr != nil {
		return "", resolveErr
	}

	result = inputRefRe.ReplaceAllStringFunc(result, func(match string) string {
		m := inputRefRe.FindStringSubmatch(match)
		name := m[1]
		val, ok := ctx.Inputs[name]
		if !ok {
			resolveErr = fmt.Errorf("unresolved input %q", name)
			return match
		}
		return val
	})
	if resolveErr != nil {
		return "", resolveErr
	}

	return result, nil
}

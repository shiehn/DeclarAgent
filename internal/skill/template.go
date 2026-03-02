package skill

import (
	"io"
	"text/template"

	"github.com/shiehn/declaragent/internal/plan"
)

// PlanInfo pairs a filename with its parsed plan.
type PlanInfo struct {
	Filename string
	Plan     *plan.Plan
}

// TemplateData holds values passed to the SKILL.md template.
type TemplateData struct {
	Binary   string
	PlansDir string
	Plans    []PlanInfo
}

var skillTemplate = template.Must(template.New("skill").Parse(`---
name: declaragent
description: Discover, validate, dry-run, and execute DeclarAgent YAML workflow plans
allowed-tools:
  - Bash
---

# DeclarAgent CLI

You can manage and execute DeclarAgent YAML workflow plans using the ` + "`{{.Binary}}`" + ` CLI.
Always use ` + "`--json`" + ` for machine-readable output.

## Commands

### Validate
` + "```bash" + `
{{.Binary}} validate <plan.yaml> --json
` + "```" + `
Check plan structure. Always validate first.

### Explain
` + "```bash" + `
{{.Binary}} explain <plan.yaml> --input key=value --json
` + "```" + `
Show resolved steps without executing.

### Dry-run
` + "```bash" + `
{{.Binary}} dry-run <plan.yaml> --input key=value --json
` + "```" + `
Simulate execution. Use before ` + "`run`" + `.

### Run
` + "```bash" + `
{{.Binary}} run <plan.yaml> --input key=value --json
` + "```" + `
Execute the plan. Add ` + "`--approve`" + ` only when the user explicitly confirms destructive operations.

## Best Practices
1. Always validate → dry-run → run
2. Use ` + "`--json`" + ` for structured output
3. Never use ` + "`--approve`" + ` without user confirmation
4. Provide all required inputs via ` + "`--input key=value`" + `
{{- if .Plans}}

## Available Plans
{{range .Plans}}
### {{.Plan.Name}}
{{- if .Plan.Description}}
{{.Plan.Description}}
{{- end}}
- **File:** ` + "`{{.Filename}}`" + `
{{- if .Plan.Inputs}}
- **Inputs:**
{{- range $name, $inp := .Plan.Inputs}}
  - ` + "`{{$name}}`" + `{{if $inp.Required}} *(required)*{{end}}{{if $inp.Default}} (default: ` + "`{{$inp.Default}}`" + `){{end}}{{if $inp.Description}} — {{$inp.Description}}{{end}}
{{- end}}
{{- end}}
{{end}}
{{- end}}
`))

// Render writes the SKILL.md content to w.
func Render(w io.Writer, data TemplateData) error {
	return skillTemplate.Execute(w, data)
}

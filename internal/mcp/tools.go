package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/stevehiehn/declaragent/internal/engine"
	"github.com/stevehiehn/declaragent/internal/plan"
)

type toolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

var builtinTools = []toolDef{
	{Name: "plan.validate", Description: "Validate a plan YAML file", InputSchema: map[string]any{
		"type": "object", "properties": map[string]any{"file": map[string]any{"type": "string"}}, "required": []string{"file"}}},
	{Name: "plan.explain", Description: "Explain a plan without executing", InputSchema: map[string]any{
		"type": "object", "properties": map[string]any{"file": map[string]any{"type": "string"}, "inputs": map[string]any{"type": "object"}}, "required": []string{"file"}}},
	{Name: "plan.dry_run", Description: "Dry-run a plan", InputSchema: map[string]any{
		"type": "object", "properties": map[string]any{"file": map[string]any{"type": "string"}, "inputs": map[string]any{"type": "object"}}, "required": []string{"file"}}},
	{Name: "plan.run", Description: "Execute a plan", InputSchema: map[string]any{
		"type": "object", "properties": map[string]any{"file": map[string]any{"type": "string"}, "inputs": map[string]any{"type": "object"}, "approve": map[string]any{"type": "boolean"}}, "required": []string{"file"}}},
	{Name: "plan.schema", Description: "Return the plan YAML schema", InputSchema: map[string]any{
		"type": "object", "properties": map[string]any{}}},
}

// loadPlanTools reads all YAML files from plansDir and generates MCP tool definitions.
func loadPlanTools(plansDir string) []toolDef {
	if plansDir == "" {
		return nil
	}
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		return nil
	}
	var tools []toolDef
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		p, err := plan.LoadFile(filepath.Join(plansDir, e.Name()))
		if err != nil {
			continue
		}
		tools = append(tools, planToToolDef(p))
	}
	return tools
}

// planToToolDef converts a Plan into an MCP tool definition.
func planToToolDef(p *plan.Plan) toolDef {
	properties := map[string]any{}
	var required []string

	for name, inp := range p.Inputs {
		prop := map[string]any{"type": "string"}
		if inp.Description != "" {
			prop["description"] = inp.Description
		}
		if inp.Default != "" {
			prop["default"] = inp.Default
		}
		properties[name] = prop
		if inp.Required {
			required = append(required, name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	desc := p.Description
	if desc == "" {
		desc = "Execute the " + p.Name + " plan"
	}

	return toolDef{
		Name:        p.Name,
		Description: desc,
		InputSchema: schema,
	}
}

func dispatch(req JSONRPCRequest, workDir string, plansDir string) *JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return &JSONRPCResponse{Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "declaragent", "version": "0.2.0"},
		}}
	case "tools/list":
		allTools := append([]toolDef{}, builtinTools...)
		allTools = append(allTools, loadPlanTools(plansDir)...)
		return &JSONRPCResponse{Result: map[string]any{"tools": allTools}}
	case "tools/call":
		return handleToolCall(req.Params, workDir, plansDir)
	case "notifications/initialized":
		return &JSONRPCResponse{Result: map[string]any{}}
	case "ping":
		return &JSONRPCResponse{Result: map[string]any{}}
	default:
		return &JSONRPCResponse{Error: &RPCError{Code: -32601, Message: "Method not found"}}
	}
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func handleToolCall(params json.RawMessage, workDir string, plansDir string) *JSONRPCResponse {
	var tc toolCallParams
	if err := json.Unmarshal(params, &tc); err != nil {
		return &JSONRPCResponse{Error: &RPCError{Code: -32602, Message: "Invalid params"}}
	}

	var args struct {
		File    string            `json:"file"`
		Inputs  map[string]string `json:"inputs"`
		Approve bool              `json:"approve"`
	}
	json.Unmarshal(tc.Arguments, &args)
	if args.Inputs == nil {
		args.Inputs = map[string]string{}
	}

	switch tc.Name {
	case "plan.validate":
		return toolValidate(args.File, workDir)
	case "plan.explain":
		return toolExecute(args.File, args.Inputs, workDir, engine.ModeExplain, false)
	case "plan.dry_run":
		return toolExecute(args.File, args.Inputs, workDir, engine.ModeDryRun, false)
	case "plan.run":
		return toolExecute(args.File, args.Inputs, workDir, engine.ModeRun, args.Approve)
	case "plan.schema":
		return &JSONRPCResponse{Result: toolContent(schemaText)}
	default:
		// Check if it matches a shipped plan name
		return toolExecuteShippedPlan(tc.Name, tc.Arguments, workDir, plansDir)
	}
}

func toolValidate(file, workDir string) *JSONRPCResponse {
	p, err := plan.LoadFile(resolvePath(file, workDir))
	if err != nil {
		return &JSONRPCResponse{Result: toolContent(err.Error())}
	}
	if err := plan.Validate(p, map[string]string{}); err != nil {
		return &JSONRPCResponse{Result: toolContent("Validation failed: " + err.Error())}
	}
	return &JSONRPCResponse{Result: toolContent("Plan is valid.")}
}

func toolExecute(file string, inputs map[string]string, workDir string, mode engine.Mode, approve bool) *JSONRPCResponse {
	p, err := plan.LoadFile(resolvePath(file, workDir))
	if err != nil {
		return &JSONRPCResponse{Result: toolContent(err.Error())}
	}
	for name, inp := range p.Inputs {
		if _, ok := inputs[name]; !ok && inp.Default != "" {
			inputs[name] = inp.Default
		}
	}
	if err := plan.Validate(p, inputs); err != nil {
		return &JSONRPCResponse{Result: toolContent(err.Error())}
	}
	ctx := engine.NewRunContext(workDir, inputs, approve)
	result, err := engine.Execute(p, ctx, mode)
	if err != nil {
		return &JSONRPCResponse{Result: toolContent(err.Error())}
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &JSONRPCResponse{Result: toolContent(string(data))}
}

// toolExecuteShippedPlan finds a plan by name in plansDir and executes it.
func toolExecuteShippedPlan(name string, rawArgs json.RawMessage, workDir string, plansDir string) *JSONRPCResponse {
	if plansDir == "" {
		return &JSONRPCResponse{Error: &RPCError{Code: -32602, Message: "Unknown tool: " + name}}
	}

	// Find the plan file by matching plan name
	planFile := findPlanFile(name, plansDir)
	if planFile == "" {
		return &JSONRPCResponse{Error: &RPCError{Code: -32602, Message: "Unknown tool: " + name}}
	}

	p, err := plan.LoadFile(planFile)
	if err != nil {
		return &JSONRPCResponse{Result: toolContent(err.Error())}
	}

	// Parse inputs from arguments
	var inputs map[string]string
	if rawArgs != nil {
		json.Unmarshal(rawArgs, &inputs)
	}
	if inputs == nil {
		inputs = map[string]string{}
	}

	// Apply defaults
	for inputName, inp := range p.Inputs {
		if _, ok := inputs[inputName]; !ok && inp.Default != "" {
			inputs[inputName] = inp.Default
		}
	}

	if err := plan.Validate(p, inputs); err != nil {
		return &JSONRPCResponse{Result: toolContent(err.Error())}
	}

	ctx := engine.NewRunContext(workDir, inputs, false)
	result, err := engine.Execute(p, ctx, engine.ModeRun)
	if err != nil {
		return &JSONRPCResponse{Result: toolContent(err.Error())}
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &JSONRPCResponse{Result: toolContent(string(data))}
}

// findPlanFile searches plansDir for a plan with the given name.
func findPlanFile(name string, plansDir string) string {
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(plansDir, e.Name())
		p, err := plan.LoadFile(path)
		if err != nil {
			continue
		}
		if p.Name == name {
			return path
		}
	}
	return ""
}

func toolContent(text string) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}

func resolvePath(file, workDir string) string {
	if filepath.IsAbs(file) {
		return file
	}
	return filepath.Join(workDir, file)
}

const schemaText = `Plan YAML Schema:
  name: string (required)
  description: string (optional)
  inputs:
    <name>:
      required: bool
      description: string
      default: string
  steps:
    - id: string (required, unique)
      name: string (human-readable step label)
      run: string (shell command)
      action: string (built-in action name)
      with: map[string]string (for actions)
      http:
        url: string (required)
        method: string (default: GET)
        headers: map[string]string
        body: string (template-resolved)
      outputs:
        <name>: stdout
      destructive: bool
  Note: Each step must have exactly one of: run, action, or http`

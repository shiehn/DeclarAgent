package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/stevehiehn/declaragent/internal/engine"
	"github.com/stevehiehn/declaragent/internal/plan"
)

type toolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

var tools = []toolDef{
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

func dispatch(req JSONRPCRequest, workDir string) *JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return &JSONRPCResponse{Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "declaragent", "version": "0.1.0"},
		}}
	case "tools/list":
		return &JSONRPCResponse{Result: map[string]any{"tools": tools}}
	case "tools/call":
		return handleToolCall(req.Params, workDir)
	default:
		return &JSONRPCResponse{Error: &RPCError{Code: -32601, Message: "Method not found"}}
	}
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func handleToolCall(params json.RawMessage, workDir string) *JSONRPCResponse {
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
		return &JSONRPCResponse{Error: &RPCError{Code: -32602, Message: "Unknown tool"}}
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

func toolContent(text string) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}

func resolvePath(file, workDir string) string {
	if filepath.IsAbs(file) {
		return file
	}
	return filepath.Join(workDir, file)
}

// Keep OS out of init for testability
func init() {
	wd, _ := os.Getwd()
	_ = wd
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
      description: string
      run: string (shell command, mutually exclusive with action)
      action: string (built-in action name, mutually exclusive with run)
      params: map[string]string (for actions)
      outputs:
        <name>: stdout
      destructive: bool`

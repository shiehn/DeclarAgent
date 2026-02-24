package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitializeResponse(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}
	resp := dispatch(req, "/tmp", "")
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	m, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	if m["protocolVersion"] != "2024-11-05" {
		t.Errorf("unexpected protocol version: %v", m["protocolVersion"])
	}
	serverInfo, _ := m["serverInfo"].(map[string]any)
	if serverInfo["name"] != "declaragent" {
		t.Errorf("unexpected server name: %v", serverInfo["name"])
	}
}

func TestToolsList(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}
	resp := dispatch(req, "/tmp", "")
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	m, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	toolsList, ok := m["tools"].([]toolDef)
	if !ok {
		t.Fatal("expected tools list")
	}
	if len(toolsList) == 0 {
		t.Fatal("expected at least one tool")
	}
	found := false
	for _, tool := range toolsList {
		if tool.Name == "plan.validate" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected plan.validate tool in list")
	}
}

func TestToolCallValidate(t *testing.T) {
	dir := t.TempDir()
	planFile := filepath.Join(dir, "plan.yaml")
	err := os.WriteFile(planFile, []byte("name: test\nsteps:\n  - id: s1\n    run: echo hello\n"), 0o644)
	if err != nil {
		t.Fatalf("failed to write plan file: %v", err)
	}

	params, _ := json.Marshal(map[string]any{
		"name":      "plan.validate",
		"arguments": map[string]any{"file": planFile},
	})

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  params,
	}
	resp := dispatch(req, dir, "")
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	data, _ := json.Marshal(resp.Result)
	if !strings.Contains(string(data), "Plan is valid") {
		t.Errorf("expected 'Plan is valid' in result, got %s", string(data))
	}
}

func TestToolsListIncludesPlans(t *testing.T) {
	dir := t.TempDir()
	planFile := filepath.Join(dir, "greet.yaml")
	os.WriteFile(planFile, []byte("name: greet\ndescription: Say hello\ninputs:\n  name:\n    default: World\nsteps:\n  - id: s1\n    run: echo hello\n"), 0o644)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 10, Method: "tools/list"}
	resp := dispatch(req, "/tmp", dir)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	m := resp.Result.(map[string]any)
	toolsList := m["tools"].([]toolDef)

	found := false
	for _, tool := range toolsList {
		if tool.Name == "greet" {
			found = true
			if tool.Description != "Say hello" {
				t.Errorf("expected description 'Say hello', got %q", tool.Description)
			}
		}
	}
	if !found {
		t.Error("expected 'greet' plan tool in tools list")
	}
}

func TestShippedPlanExecution(t *testing.T) {
	dir := t.TempDir()
	planFile := filepath.Join(dir, "greet.yaml")
	os.WriteFile(planFile, []byte("name: greet\ndescription: Say hello\ninputs:\n  name:\n    default: World\nsteps:\n  - id: s1\n    run: echo hello {{inputs.name}}\n    outputs:\n      msg: stdout\n"), 0o644)

	params, _ := json.Marshal(map[string]any{
		"name":      "greet",
		"arguments": map[string]any{"name": "Alice"},
	})

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 11, Method: "tools/call", Params: params}
	resp := dispatch(req, dir, dir)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	data, _ := json.Marshal(resp.Result)
	if !strings.Contains(string(data), "hello Alice") {
		t.Errorf("expected output to contain 'hello Alice', got %s", string(data))
	}
}

func TestMalformedJSONError(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "nonexistent/method",
	}
	resp := dispatch(req, "/tmp", "")
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected error code -32601, got %d", resp.Error.Code)
	}
}

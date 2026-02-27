package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// helper to call dispatch and return response
func callDispatch(t *testing.T, method string, params any, workDir, plansDir string) *JSONRPCResponse {
	t.Helper()
	var rawParams json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		rawParams = b
	}
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  rawParams,
	}
	resp := dispatch(req, workDir, plansDir)
	resp.JSONRPC = "2.0"
	resp.ID = req.ID
	return resp
}

func writePlanFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// responseText extracts the text from a tool content response
func responseText(t *testing.T, resp *JSONRPCResponse) string {
	t.Helper()
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	m, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type: %T", resp.Result)
	}
	content, ok := m["content"].([]map[string]any)
	if !ok {
		// Try via JSON round-trip for interface conversion
		b, _ := json.Marshal(m["content"])
		var arr []map[string]any
		if err := json.Unmarshal(b, &arr); err != nil {
			t.Fatalf("content is not []map: %T", m["content"])
		}
		content = arr
	}
	if len(content) == 0 {
		t.Fatal("empty content")
	}
	return content[0]["text"].(string)
}

// ============================================================
// Section 6: LLM-Perspective Tests
// ============================================================

func TestLLMCallsPlanValidateE2E(t *testing.T) {
	dir := t.TempDir()
	writePlanFile(t, dir, "good.yaml", `
name: good
steps:
  - id: s1
    run: echo hi
`)
	resp := callDispatch(t, "tools/call", map[string]any{
		"name":      "plan.validate",
		"arguments": map[string]any{"file": filepath.Join(dir, "good.yaml")},
	}, dir, "")
	text := responseText(t, resp)
	if !strings.Contains(text, "valid") {
		t.Fatalf("expected 'valid' in response, got %q", text)
	}
}

func TestLLMCallsPlanRunE2E(t *testing.T) {
	// Start a real HTTP server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"result":"from-server"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	writePlanFile(t, dir, "run.yaml", fmt.Sprintf(`
name: run-test
steps:
  - id: fetch
    http:
      url: %s/api
    outputs:
      body: stdout
  - id: check
    run: echo "got ${{steps.fetch.outputs.body}}"
    outputs:
      result: stdout
`, srv.URL))
	resp := callDispatch(t, "tools/call", map[string]any{
		"name":      "plan.run",
		"arguments": map[string]any{"file": filepath.Join(dir, "run.yaml")},
	}, dir, "")
	text := responseText(t, resp)
	if !strings.Contains(text, `"success":true`) && !strings.Contains(text, `"success": true`) {
		t.Fatalf("expected success in result, got %q", text)
	}
}

func TestLLMCallsShippedPlanE2E(t *testing.T) {
	dir := t.TempDir()
	plansDir := t.TempDir()
	writePlanFile(t, plansDir, "greet.yaml", `
name: greet
description: A greeting plan
inputs:
  who:
    required: true
    description: Who to greet
steps:
  - id: say_hi
    run: echo "Hello ${{inputs.who}}"
    outputs:
      msg: stdout
`)
	resp := callDispatch(t, "tools/call", map[string]any{
		"name":      "greet",
		"arguments": map[string]any{"who": "World"},
	}, dir, plansDir)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	text := responseText(t, resp)
	if !strings.Contains(text, "Hello World") {
		t.Fatalf("expected 'Hello World' in result, got %q", text)
	}
}

func TestLLMReceivesStructuredErrorE2E(t *testing.T) {
	dir := t.TempDir()
	writePlanFile(t, dir, "bad.yaml", `
name: bad
steps:
  - id: s1
    run: echo hi
    action: file.write
    with:
      path: x
      content: y
`)
	resp := callDispatch(t, "tools/call", map[string]any{
		"name":      "plan.run",
		"arguments": map[string]any{"file": filepath.Join(dir, "bad.yaml")},
	}, dir, "")
	text := responseText(t, resp)
	if !strings.Contains(text, "multiple") {
		t.Fatalf("expected validation error about multiple, got %q", text)
	}
}

func TestLLMCallsPlanExplainE2E(t *testing.T) {
	dir := t.TempDir()
	writePlanFile(t, dir, "explain.yaml", `
name: explain-test
steps:
  - id: s1
    run: echo hello
    outputs:
      msg: stdout
  - id: s2
    run: echo "${{steps.s1.outputs.msg}}"
`)
	resp := callDispatch(t, "tools/call", map[string]any{
		"name":      "plan.explain",
		"arguments": map[string]any{"file": filepath.Join(dir, "explain.yaml")},
	}, dir, "")
	text := responseText(t, resp)
	// Should show step info without executing
	if !strings.Contains(text, "explain") {
		t.Fatalf("expected 'explain' in result, got %q", text)
	}
	if !strings.Contains(text, "s1") {
		t.Fatalf("expected step id 's1' in result, got %q", text)
	}
}

func TestLLMCallsPlanDryRunE2E(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "should-not-exist.txt")
	writePlanFile(t, dir, "dryrun.yaml", fmt.Sprintf(`
name: dryrun-test
steps:
  - id: s1
    action: file.write
    with:
      path: %s
      content: nope
`, outFile))
	resp := callDispatch(t, "tools/call", map[string]any{
		"name":      "plan.dry_run",
		"arguments": map[string]any{"file": filepath.Join(dir, "dryrun.yaml")},
	}, dir, "")
	text := responseText(t, resp)
	if !strings.Contains(text, "dry-run") {
		t.Fatalf("expected 'dry-run' in result, got %q", text)
	}
	// File should NOT exist
	if _, err := os.Stat(outFile); !os.IsNotExist(err) {
		t.Fatal("file should not exist after dry-run")
	}
}

func TestLLMCallsPlanSchemaE2E(t *testing.T) {
	resp := callDispatch(t, "tools/call", map[string]any{
		"name":      "plan.schema",
		"arguments": map[string]any{},
	}, "", "")
	text := responseText(t, resp)
	if !strings.Contains(text, "name: string") {
		t.Fatalf("expected schema content, got %q", text)
	}
	if !strings.Contains(text, "steps:") {
		t.Fatalf("expected 'steps:' in schema, got %q", text)
	}
}

// ============================================================
// Section 7: MCP Tool Metadata & Annotations Tests
// ============================================================

func TestMCPInitializeProtocolE2E(t *testing.T) {
	resp := callDispatch(t, "initialize", nil, "", "")
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	m := resp.Result.(map[string]any)
	if m["protocolVersion"] != "2024-11-05" {
		t.Fatalf("expected protocol version 2024-11-05, got %v", m["protocolVersion"])
	}
	caps, ok := m["capabilities"].(map[string]any)
	if !ok {
		t.Fatal("expected capabilities map")
	}
	if _, ok := caps["tools"]; !ok {
		t.Fatal("expected tools capability")
	}
	info, ok := m["serverInfo"].(map[string]any)
	if !ok {
		t.Fatal("expected serverInfo map")
	}
	if info["name"] != "declaragent" {
		t.Fatalf("expected server name 'declaragent', got %v", info["name"])
	}
}

func TestMCPBuiltinToolsHaveDescriptionsE2E(t *testing.T) {
	resp := callDispatch(t, "tools/list", nil, "", "")
	m := resp.Result.(map[string]any)
	b, _ := json.Marshal(m["tools"])
	var tools []toolDef
	json.Unmarshal(b, &tools)

	for _, tool := range builtinTools {
		found := false
		for _, t2 := range tools {
			if t2.Name == tool.Name {
				found = true
				if t2.Description == "" {
					t.Errorf("tool %q has empty description", tool.Name)
				}
				break
			}
		}
		if !found {
			t.Errorf("builtin tool %q not found in tools/list", tool.Name)
		}
	}
}

func TestMCPBuiltinToolsHaveInputSchemaE2E(t *testing.T) {
	for _, tool := range builtinTools {
		schema, ok := tool.InputSchema.(map[string]any)
		if !ok {
			t.Errorf("tool %q: inputSchema is not a map", tool.Name)
			continue
		}
		if schema["type"] != "object" {
			t.Errorf("tool %q: inputSchema type should be 'object', got %v", tool.Name, schema["type"])
		}
	}
}

func TestMCPPlanAsToolMetadataE2E(t *testing.T) {
	plansDir := t.TempDir()
	writePlanFile(t, plansDir, "deploy.yaml", `
name: deploy
description: Deploy the application
inputs:
  env:
    required: true
    description: Target environment
  version:
    description: Version to deploy
    default: latest
steps:
  - id: s1
    run: echo "deploying ${{inputs.env}} ${{inputs.version}}"
`)
	resp := callDispatch(t, "tools/list", nil, "", plansDir)
	m := resp.Result.(map[string]any)
	b, _ := json.Marshal(m["tools"])
	var tools []json.RawMessage
	json.Unmarshal(b, &tools)

	var deployTool map[string]any
	for _, raw := range tools {
		var t2 map[string]any
		json.Unmarshal(raw, &t2)
		if t2["name"] == "deploy" {
			deployTool = t2
			break
		}
	}
	if deployTool == nil {
		t.Fatal("expected 'deploy' tool in list")
	}
	if deployTool["description"] != "Deploy the application" {
		t.Fatalf("unexpected description: %v", deployTool["description"])
	}

	schema := deployTool["inputSchema"].(map[string]any)
	props := schema["properties"].(map[string]any)
	if _, ok := props["env"]; !ok {
		t.Fatal("expected 'env' in properties")
	}
	if _, ok := props["version"]; !ok {
		t.Fatal("expected 'version' in properties")
	}
	required := schema["required"].([]any)
	found := false
	for _, r := range required {
		if r == "env" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected 'env' in required")
	}
}

func TestMCPPlanAsToolDefaultsPreservedE2E(t *testing.T) {
	plansDir := t.TempDir()
	writePlanFile(t, plansDir, "defaults.yaml", `
name: with-defaults
inputs:
  color:
    description: Favorite color
    default: blue
steps:
  - id: s1
    run: echo "${{inputs.color}}"
`)
	resp := callDispatch(t, "tools/list", nil, "", plansDir)
	m := resp.Result.(map[string]any)
	b, _ := json.Marshal(m["tools"])
	var tools []json.RawMessage
	json.Unmarshal(b, &tools)

	for _, raw := range tools {
		var t2 map[string]any
		json.Unmarshal(raw, &t2)
		if t2["name"] == "with-defaults" {
			schema := t2["inputSchema"].(map[string]any)
			props := schema["properties"].(map[string]any)
			color := props["color"].(map[string]any)
			if color["default"] != "blue" {
				t.Fatalf("expected default 'blue', got %v", color["default"])
			}
			return
		}
	}
	t.Fatal("tool 'with-defaults' not found")
}

func TestMCPToolsListIncludesAllBuiltinsE2E(t *testing.T) {
	resp := callDispatch(t, "tools/list", nil, "", "")
	m := resp.Result.(map[string]any)
	b, _ := json.Marshal(m["tools"])
	var tools []map[string]any
	json.Unmarshal(b, &tools)

	builtinNames := map[string]bool{
		"plan.validate": true,
		"plan.explain":  true,
		"plan.dry_run":  true,
		"plan.run":      true,
		"plan.schema":   true,
	}
	for name := range builtinNames {
		found := false
		for _, tool := range tools {
			if tool["name"] == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("builtin tool %q not found in tools/list", name)
		}
	}
	if len(tools) != 5 {
		t.Errorf("expected exactly 5 builtin tools, got %d", len(tools))
	}
}

func TestMCPDiscoversPlanToolsFromDirectoryE2E(t *testing.T) {
	plansDir := t.TempDir()
	writePlanFile(t, plansDir, "alpha.yaml", `
name: alpha
steps:
  - id: s1
    run: echo alpha
`)
	writePlanFile(t, plansDir, "beta.yaml", `
name: beta
steps:
  - id: s1
    run: echo beta
`)
	resp := callDispatch(t, "tools/list", nil, "", plansDir)
	m := resp.Result.(map[string]any)
	b, _ := json.Marshal(m["tools"])
	var tools []map[string]any
	json.Unmarshal(b, &tools)

	names := map[string]bool{}
	for _, tool := range tools {
		names[tool["name"].(string)] = true
	}
	if !names["alpha"] {
		t.Error("expected 'alpha' tool")
	}
	if !names["beta"] {
		t.Error("expected 'beta' tool")
	}
	// 5 builtins + 2 plan tools
	if len(tools) != 7 {
		t.Errorf("expected 7 tools, got %d", len(tools))
	}
}

func TestMCPPlanToolExecutionE2E(t *testing.T) {
	dir := t.TempDir()
	plansDir := t.TempDir()
	writePlanFile(t, plansDir, "hello.yaml", `
name: hello
inputs:
  name:
    required: true
steps:
  - id: greet
    run: echo "hi ${{inputs.name}}"
    outputs:
      msg: stdout
`)
	resp := callDispatch(t, "tools/call", map[string]any{
		"name":      "hello",
		"arguments": map[string]any{"name": "Alice"},
	}, dir, plansDir)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	text := responseText(t, resp)
	if !strings.Contains(text, "hi Alice") {
		t.Fatalf("expected 'hi Alice' in result, got %q", text)
	}
}

func TestMCPUnknownToolReturnsErrorE2E(t *testing.T) {
	resp := callDispatch(t, "tools/call", map[string]any{
		"name":      "nonexistent.tool",
		"arguments": map[string]any{},
	}, "", "")
	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
	if resp.Error.Code != -32602 {
		t.Fatalf("expected error code -32602, got %d", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "nonexistent.tool") {
		t.Fatalf("expected tool name in error, got %q", resp.Error.Message)
	}
}

// ============================================================
// Section 8: MCP Server Startup & Discoverability Tests
// ============================================================

func TestMCPServerStartsAndListensE2E(t *testing.T) {
	s := &SSEServer{
		workDir:  t.TempDir(),
		plansDir: "",
		clients:  make(map[string]*sseClient),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", s.handleSSE)
	mux.HandleFunc("/message", s.handleMessage)
	mux.HandleFunc("/health", s.handleHealth)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body["status"])
	}
}

func TestMCPSSEClientConnectsE2E(t *testing.T) {
	s := &SSEServer{
		workDir:  t.TempDir(),
		plansDir: "",
		clients:  make(map[string]*sseClient),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", s.handleSSE)
	mux.HandleFunc("/message", s.handleMessage)
	mux.HandleFunc("/health", s.handleHealth)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Connect to SSE endpoint
	resp, err := http.Get(srv.URL + "/sse")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}

	// Read the first event (endpoint)
	scanner := bufio.NewScanner(resp.Body)
	var endpointEvent string
	deadline := time.After(2 * time.Second)
	done := make(chan string, 1)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				done <- line
				return
			}
		}
	}()
	select {
	case line := <-done:
		endpointEvent = line
	case <-deadline:
		t.Fatal("timeout waiting for endpoint event")
	}

	if !strings.Contains(endpointEvent, "/message?sessionId=") {
		t.Fatalf("expected endpoint URL in event, got %q", endpointEvent)
	}
}

func TestMCPSSEToolsCallViaHTTPE2E(t *testing.T) {
	s := &SSEServer{
		workDir:  t.TempDir(),
		plansDir: "",
		clients:  make(map[string]*sseClient),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", s.handleSSE)
	mux.HandleFunc("/message", s.handleMessage)
	mux.HandleFunc("/health", s.handleHealth)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// POST a tools/list JSON-RPC request to /message
	reqBody := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	resp, err := http.Post(srv.URL+"/message", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var rpcResp JSONRPCResponse
	json.NewDecoder(resp.Body).Decode(&rpcResp)
	if rpcResp.Error != nil {
		t.Fatalf("unexpected error: %s", rpcResp.Error.Message)
	}
	m, ok := rpcResp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", rpcResp.Result)
	}
	b, _ := json.Marshal(m["tools"])
	var tools []map[string]any
	json.Unmarshal(b, &tools)
	if len(tools) != 5 {
		t.Fatalf("expected 5 builtin tools, got %d", len(tools))
	}
}

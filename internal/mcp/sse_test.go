package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func startTestSSEServer(t *testing.T) int {
	t.Helper()
	port := 19200 + int(time.Now().UnixNano()%100)
	go func() {
		if err := ServeSSE(port, t.TempDir(), ""); err != nil {
			// Server stopped
		}
	}()
	// Wait for server to start
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
		if err == nil {
			resp.Body.Close()
			return port
		}
	}
	t.Fatal("SSE server failed to start")
	return 0
}

func TestHealthEndpoint(t *testing.T) {
	port := startTestSSEServer(t)
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var data map[string]any
	json.NewDecoder(resp.Body).Decode(&data)
	if data["status"] != "ok" {
		t.Errorf("expected status ok, got %v", data["status"])
	}
}

func TestSSEConnect(t *testing.T) {
	port := startTestSSEServer(t)
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/sse", port))
	if err != nil {
		t.Fatalf("SSE connect failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}

	// Read the first event (endpoint)
	scanner := bufio.NewScanner(resp.Body)
	var eventType, eventData string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			eventData = strings.TrimPrefix(line, "data: ")
		} else if line == "" && eventType != "" {
			break
		}
	}
	if eventType != "endpoint" {
		t.Errorf("expected endpoint event, got %s", eventType)
	}
	if !strings.Contains(eventData, "/message") {
		t.Errorf("expected message URL in endpoint data, got %s", eventData)
	}
}

func TestMessageEndpoint(t *testing.T) {
	port := startTestSSEServer(t)

	// Send an initialize request
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}
	body, _ := json.Marshal(req)
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/message", port),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("message request failed: %v", err)
	}
	defer resp.Body.Close()

	var rpcResp JSONRPCResponse
	json.NewDecoder(resp.Body).Decode(&rpcResp)
	if rpcResp.Error != nil {
		t.Fatalf("unexpected error: %v", rpcResp.Error)
	}
	m, ok := rpcResp.Result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	if m["protocolVersion"] != "2024-11-05" {
		t.Errorf("unexpected protocol version: %v", m["protocolVersion"])
	}
}

func TestToolsListViaMessage(t *testing.T) {
	port := startTestSSEServer(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "tools/list"}
	body, _ := json.Marshal(req)
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/message", port),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var rpcResp JSONRPCResponse
	json.NewDecoder(resp.Body).Decode(&rpcResp)
	if rpcResp.Error != nil {
		t.Fatalf("unexpected error: %v", rpcResp.Error)
	}
	data, _ := json.Marshal(rpcResp.Result)
	if !strings.Contains(string(data), "plan.validate") {
		t.Error("expected plan.validate in tools list")
	}
}

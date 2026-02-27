package mcp

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

// sseClient represents a connected SSE client.
type sseClient struct {
	id     string
	events chan []byte
	done   chan struct{}
}

// SSEServer holds state for the SSE transport.
type SSEServer struct {
	workDir  string
	plansDir string
	mu       sync.Mutex
	clients  map[string]*sseClient
	nextID   int
}

// ServeSSE starts the MCP server with SSE transport on the given port.
func ServeSSE(port int, workDir, plansDir string) error {
	s := &SSEServer{
		workDir:  workDir,
		plansDir: plansDir,
		clients:  make(map[string]*sseClient),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sse", s.handleSSE)
	mux.HandleFunc("/message", s.handleMessage)
	mux.HandleFunc("/health", s.handleHealth)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	log.Printf("[DeclarAgent] SSE server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *SSEServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.mu.Lock()
	count := len(s.clients)
	s.mu.Unlock()
	json.NewEncoder(w).Encode(map[string]any{
		"status":          "ok",
		"connectedAgents": count,
	})
}

func (s *SSEServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create client
	s.mu.Lock()
	s.nextID++
	clientID := fmt.Sprintf("client-%d", s.nextID)
	client := &sseClient{
		id:     clientID,
		events: make(chan []byte, 64),
		done:   make(chan struct{}),
	}
	s.clients[clientID] = client
	s.mu.Unlock()

	log.Printf("[DeclarAgent] SSE client connected: %s", clientID)

	// Send the endpoint event per MCP SSE spec
	// The message URL includes the client ID so responses route back correctly
	messageURL := fmt.Sprintf("http://%s/message?sessionId=%s", r.Host, clientID)
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", messageURL)
	flusher.Flush()

	// Stream events until client disconnects
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			delete(s.clients, clientID)
			s.mu.Unlock()
			close(client.done)
			log.Printf("[DeclarAgent] SSE client disconnected: %s", clientID)
			return
		case data := <-client.events:
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *SSEServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("sessionId")

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(&JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: -32700, Message: "Parse error"},
		})
		return
	}

	resp := dispatch(req, s.workDir, s.plansDir)
	resp.JSONRPC = "2.0"
	resp.ID = req.ID

	respData, _ := json.Marshal(resp)

	// If there's a connected SSE client, send via SSE stream
	if sessionID != "" {
		s.mu.Lock()
		client, ok := s.clients[sessionID]
		s.mu.Unlock()
		if ok {
			select {
			case client.events <- respData:
			default:
				log.Printf("[DeclarAgent] SSE client %s buffer full, dropping message", sessionID)
			}
		}
	}

	// Also return the response directly in the HTTP response
	// This supports both SSE-streamed and request-response patterns
	w.Header().Set("Content-Type", "application/json")
	w.Write(respData)
}

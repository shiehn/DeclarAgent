package action

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPAction executes an HTTP request.
type HTTPAction struct {
	client *http.Client
}

// NewHTTPAction creates an HTTP action with a default timeout.
func NewHTTPAction() *HTTPAction {
	return &HTTPAction{
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// Execute sends the HTTP request and returns the response body as stdout output.
func (h *HTTPAction) Execute(params map[string]string) (map[string]string, error) {
	url := params["url"]
	if url == "" {
		return nil, fmt.Errorf("http: url is required")
	}

	method := params["method"]
	if method == "" {
		method = "GET"
	}

	var bodyReader io.Reader
	if body, ok := params["body"]; ok && body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("http: failed to create request: %w", err)
	}

	// Parse headers from params (header_<name> = value)
	for k, v := range params {
		if strings.HasPrefix(k, "header_") {
			req.Header.Set(strings.TrimPrefix(k, "header_"), v)
		}
	}

	// If body is provided and no content-type set, default to application/json
	if bodyReader != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("http: failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http: %d %s", resp.StatusCode, string(respBody))
	}

	return map[string]string{
		"stdout":      string(respBody),
		"status_code": fmt.Sprintf("%d", resp.StatusCode),
	}, nil
}

// DryRun returns a description of what the HTTP request would do.
func (h *HTTPAction) DryRun(params map[string]string) string {
	method := params["method"]
	if method == "" {
		method = "GET"
	}
	return fmt.Sprintf("Would send %s request to %s", method, params["url"])
}

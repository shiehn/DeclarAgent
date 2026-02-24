package action

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPActionGET(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Write([]byte(`{"ip":"1.2.3.4"}`))
	}))
	defer server.Close()

	act := NewHTTPAction()
	outputs, err := act.Execute(map[string]string{
		"url":    server.URL,
		"method": "GET",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outputs["status_code"] != "200" {
		t.Errorf("expected status 200, got %s", outputs["status_code"])
	}

	var body map[string]string
	json.Unmarshal([]byte(outputs["stdout"]), &body)
	if body["ip"] != "1.2.3.4" {
		t.Errorf("expected ip=1.2.3.4, got %s", body["ip"])
	}
}

func TestHTTPActionPOST(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected auth header, got %q", r.Header.Get("Authorization"))
		}
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	act := NewHTTPAction()
	outputs, err := act.Execute(map[string]string{
		"url":                  server.URL,
		"method":               "POST",
		"body":                 `{"data":"test"}`,
		"header_Authorization": "Bearer test-token",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outputs["stdout"] != "ok" {
		t.Errorf("expected 'ok', got %q", outputs["stdout"])
	}
}

func TestHTTPActionError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	act := NewHTTPAction()
	_, err := act.Execute(map[string]string{
		"url": server.URL,
	})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestHTTPActionMissingURL(t *testing.T) {
	act := NewHTTPAction()
	_, err := act.Execute(map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
}

func TestHTTPActionDefaultMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET (default), got %s", r.Method)
		}
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	act := NewHTTPAction()
	_, err := act.Execute(map[string]string{"url": server.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPActionDryRun(t *testing.T) {
	act := NewHTTPAction()
	result := act.DryRun(map[string]string{"url": "https://example.com", "method": "POST"})
	if result != "Would send POST request to https://example.com" {
		t.Errorf("unexpected dry run output: %s", result)
	}
}

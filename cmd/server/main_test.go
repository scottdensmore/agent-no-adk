package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"sre-triage-agent/internal/a2a"
)

func TestResolveConfig(t *testing.T) {
	// Test CLI flags override env vars and defaults
	t.Run("flags provided", func(t *testing.T) {
		port, model := resolveConfig("9090", "gemini-test-model")
		if port != "9090" {
			t.Errorf("expected port 9090, got %s", port)
		}
		if model != "gemini-test-model" {
			t.Errorf("expected model gemini-test-model, got %s", model)
		}
	})

	// Test environment variables used when flags empty
	t.Run("env vars fallback", func(t *testing.T) {
		os.Setenv("PORT", "7070")
		os.Setenv("GEMINI_MODEL", "gemini-env-model")
		defer func() {
			os.Unsetenv("PORT")
			os.Unsetenv("GEMINI_MODEL")
		}()

		port, model := resolveConfig("", "")
		if port != "7070" {
			t.Errorf("expected port 7070, got %s", port)
		}
		if model != "gemini-env-model" {
			t.Errorf("expected model gemini-env-model, got %s", model)
		}
	})

	// Test default fallbacks
	t.Run("default fallback", func(t *testing.T) {
		os.Unsetenv("PORT")
		os.Unsetenv("GEMINI_MODEL")

		port, model := resolveConfig("", "")
		if port != "8080" {
			t.Errorf("expected default port 8080, got %s", port)
		}
		if model != "gemini-3.8-flash" {
			t.Errorf("expected default model gemini-3.8-flash, got %s", model)
		}
	})
}

func TestSetupHandler_NilAgentFallback(t *testing.T) {
	handler := setupHandler(nil)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	reqBody := `{"jsonrpc":"2.0","id":"1","method":"message/send","params":{"message":{"parts":[{"kind":"text","text":"ping"}]}}}`
	resp, err := http.Post(ts.URL+"/a2a/invoke", "application/json", bytes.NewBufferString(reqBody))
	if err != nil {
		t.Fatalf("failed to post invoke: %v", err)
	}
	defer resp.Body.Close()

	var rpcResp a2a.JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		t.Fatalf("failed to decode invoke response: %v", err)
	}
	if rpcResp.Result == nil || len(rpcResp.Result.Parts) == 0 {
		t.Fatalf("expected non-empty result parts, got %+v", rpcResp.Result)
	}
	expected := "Agent credentials not configured. Set GEMINI_API_KEY or GOOGLE_CLOUD_PROJECT with ADC."
	if rpcResp.Result.Parts[0].Text != expected {
		t.Errorf("expected fallback message %q, got %q", expected, rpcResp.Result.Parts[0].Text)
	}
}

func TestServerRouterHealth(t *testing.T) {
	mockInvoker := func(ctx context.Context, prompt string, contextID string) (string, error) {
		return "OK", nil
	}

	handler := a2a.NewHandler("sre-triage-agent", mockInvoker)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/.well-known/agent-card.json")
	if err != nil {
		t.Fatalf("failed to fetch agent card: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var card a2a.AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		t.Fatalf("failed to decode agent card: %v", err)
	}
	if card.Name != "sre-triage-agent" {
		t.Errorf("expected agent name 'sre-triage-agent', got %q", card.Name)
	}
}

func TestServerRootStatus(t *testing.T) {
	mockInvoker := func(ctx context.Context, prompt string, contextID string) (string, error) {
		return "OK", nil
	}

	handler := a2a.NewHandler("sre-triage-agent", mockInvoker)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("failed to fetch root: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode root response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", body["status"])
	}
}

func TestServerA2AInvoke(t *testing.T) {
	mockInvoker := func(ctx context.Context, prompt string, contextID string) (string, error) {
		return "Triage analysis complete: Incident severity SEV-1", nil
	}

	handler := a2a.NewHandler("sre-triage-agent", mockInvoker)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	reqBody := `{"jsonrpc":"2.0","id":"test-1","method":"message/send","params":{"message":{"parts":[{"kind":"text","text":"Analyze outage"}],"contextId":"ctx-123"}}}`
	resp, err := http.Post(ts.URL+"/a2a/invoke", "application/json", bytes.NewBufferString(reqBody))
	if err != nil {
		t.Fatalf("failed to post invoke: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var rpcResp a2a.JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		t.Fatalf("failed to decode invoke response: %v", err)
	}
	if rpcResp.Result == nil || len(rpcResp.Result.Parts) == 0 {
		t.Fatalf("expected non-empty result parts, got %+v", rpcResp.Result)
	}
	if rpcResp.Result.Parts[0].Text != "Triage analysis complete: Incident severity SEV-1" {
		t.Errorf("unexpected answer text: %q", rpcResp.Result.Parts[0].Text)
	}
}

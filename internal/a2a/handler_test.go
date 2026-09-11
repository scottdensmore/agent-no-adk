package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func mockInvoker(_ context.Context, message string, contextID string) (string, error) {
	return "Echo response for: " + message + " [ctx: " + contextID + "]", nil
}

func errorInvoker(_ context.Context, _ string, _ string) (string, error) {
	return "", errors.New("backend model timeout")
}

func TestAgentCardEndpoint(t *testing.T) {
	handler := NewHandler("sre-triage-agent", mockInvoker)

	// Test root agent card
	req := httptest.NewRequest("GET", "/.well-known/agent-card.json", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		t.Fatalf("failed to decode agent card: %v", err)
	}
	if card.Name != "sre-triage-agent" {
		t.Errorf("expected name 'sre-triage-agent', got %q", card.Name)
	}
	if card.PreferredTransport != "JSONRPC" {
		t.Errorf("expected preferredTransport 'JSONRPC', got %q", card.PreferredTransport)
	}
	if len(card.Skills) == 0 || card.Skills[0].ID != "sre_incident_triage" {
		t.Errorf("expected skill 'sre_incident_triage', got %v", card.Skills)
	}
	if !card.Capabilities.Streaming {
		t.Errorf("expected streaming capability to be true")
	}

	// Test prefixed agent card for Agent Runtime passthrough
	reqPrefixed := httptest.NewRequest("GET", "/a2a/sre-triage-agent/.well-known/agent-card.json", nil)
	wPrefixed := httptest.NewRecorder()
	handler.ServeHTTP(wPrefixed, reqPrefixed)
	if wPrefixed.Code != http.StatusOK {
		t.Fatalf("expected 200 for prefixed agent card, got %d", wPrefixed.Code)
	}
	var cardPrefixed AgentCard
	if err := json.NewDecoder(wPrefixed.Body).Decode(&cardPrefixed); err != nil {
		t.Fatalf("failed to decode prefixed agent card: %v", err)
	}
	if cardPrefixed.Name != "sre-triage-agent" {
		t.Errorf("expected name 'sre-triage-agent', got %q", cardPrefixed.Name)
	}
}

func TestA2AMessageSend(t *testing.T) {
	handler := NewHandler("sre-triage-agent", mockInvoker)

	payload := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "req-1",
		Method:  "message/send",
		Params: MessageSendParams{
			Message: A2AMessage{
				Role:      "user",
				Parts:     []A2APart{{Kind: "text", Text: "Analyze incident"}},
				ContextID: "test-session-42",
			},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/a2a/invoke", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(b))
	}

	var jsonResp JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if jsonResp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %v", jsonResp.Error)
	}
	if jsonResp.Result == nil {
		t.Fatalf("expected non-nil result")
	}
	if jsonResp.Result.ContextID != "test-session-42" {
		t.Errorf("expected contextId 'test-session-42', got %q", jsonResp.Result.ContextID)
	}
	if len(jsonResp.Result.Parts) == 0 || !strings.Contains(jsonResp.Result.Parts[0].Text, "Echo response") {
		t.Errorf("unexpected message response: %v", jsonResp.Result.Parts)
	}
	if jsonResp.Result.Role != "agent" {
		t.Errorf("expected role 'agent', got %q", jsonResp.Result.Role)
	}
}

func TestA2ATasksCreate(t *testing.T) {
	handler := NewHandler("sre-triage-agent", mockInvoker)

	payload := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "req-tasks-1",
		Method:  "tasks/create",
		Params: MessageSendParams{
			Message: A2AMessage{
				Role:      "user",
				Parts:     []A2APart{{Kind: "text", Text: "Parse logs"}},
				ContextID: "task-ctx-100",
			},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/a2a/sre-triage-agent/invoke", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var jsonResp JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if jsonResp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %v", jsonResp.Error)
	}
	if jsonResp.Result == nil || jsonResp.Result.ContextID != "task-ctx-100" {
		t.Errorf("expected contextId 'task-ctx-100', got %v", jsonResp.Result)
	}
}

func TestHealthCheck(t *testing.T) {
	handler := NewHandler("sre-triage-agent", mockInvoker)

	// Test /list-apps
	req := httptest.NewRequest("GET", "/list-apps", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var apps []string
	if err := json.NewDecoder(w.Body).Decode(&apps); err != nil {
		t.Fatalf("failed to decode list-apps: %v", err)
	}
	if len(apps) == 0 || apps[0] != "sre-triage-agent" {
		t.Errorf("expected ['sre-triage-agent'], got %v", apps)
	}

	// Test GET /
	reqRoot := httptest.NewRequest("GET", "/", nil)
	wRoot := httptest.NewRecorder()
	handler.ServeHTTP(wRoot, reqRoot)

	if wRoot.Code != http.StatusOK {
		t.Fatalf("expected 200 for root, got %d", wRoot.Code)
	}
	var rootResp map[string]string
	if err := json.NewDecoder(wRoot.Body).Decode(&rootResp); err != nil {
		t.Fatalf("failed to decode root response: %v", err)
	}
	if rootResp["status"] != "ok" || rootResp["agent"] != "sre-triage-agent" {
		t.Errorf("unexpected root response: %v", rootResp)
	}
}

func TestRootPostInvoke(t *testing.T) {
	handler := NewHandler("sre-triage-agent", mockInvoker)

	payload := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "req-root-post",
		Method:  "message/send",
		Params: MessageSendParams{
			Message: A2AMessage{
				Parts: []A2APart{
					{Kind: "text", Text: "Line 1"},
					{Kind: "text", Text: "Line 2"},
				},
				ContextID: "ctx-root",
			},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var jsonResp JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if jsonResp.Result == nil {
		t.Fatalf("expected non-nil result")
	}
	if !strings.Contains(jsonResp.Result.Parts[0].Text, "Line 1\nLine 2") {
		t.Errorf("expected joined prompt, got %q", jsonResp.Result.Parts[0].Text)
	}
}

func TestContextIDAutoGeneration(t *testing.T) {
	handler := NewHandler("sre-triage-agent", mockInvoker)

	payload := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "req-no-ctx",
		Method:  "message/send",
		Params: MessageSendParams{
			Message: A2AMessage{
				Parts: []A2APart{{Kind: "text", Text: "hello"}},
			},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest("POST", "/a2a/invoke", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var jsonResp JSONRPCResponse
	if err := json.NewDecoder(w.Body).Decode(&jsonResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if jsonResp.Result == nil || jsonResp.Result.ContextID == "" {
		t.Fatalf("expected generated UUID contextID, got %v", jsonResp.Result)
	}
}

func TestJSONRPC_Errors(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		handler := NewHandler("sre-triage-agent", mockInvoker)
		req := httptest.NewRequest("POST", "/a2a/invoke", strings.NewReader("not a json"))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		var jsonResp JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&jsonResp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if jsonResp.Error == nil {
			t.Fatalf("expected error, got nil")
		}
		if jsonResp.Error.Code != -32700 {
			t.Errorf("expected code -32700, got %d", jsonResp.Error.Code)
		}
	})

	t.Run("method not found", func(t *testing.T) {
		handler := NewHandler("sre-triage-agent", mockInvoker)
		payload := `{"jsonrpc":"2.0","id":"123","method":"unsupported/method","params":{}}`
		req := httptest.NewRequest("POST", "/a2a/invoke", strings.NewReader(payload))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		var jsonResp JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&jsonResp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if jsonResp.Error == nil {
			t.Fatalf("expected error, got nil")
		}
		if jsonResp.Error.Code != -32601 {
			t.Errorf("expected code -32601, got %d", jsonResp.Error.Code)
		}
	})

	t.Run("invoker error", func(t *testing.T) {
		handler := NewHandler("sre-triage-agent", errorInvoker)
		payload := `{"jsonrpc":"2.0","id":"err-req","method":"message/send","params":{"message":{"parts":[{"kind":"text","text":"fail"}]}}}`
		req := httptest.NewRequest("POST", "/a2a/invoke", strings.NewReader(payload))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		var jsonResp JSONRPCResponse
		if err := json.NewDecoder(w.Body).Decode(&jsonResp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if jsonResp.Error == nil {
			t.Fatalf("expected error, got nil")
		}
		if jsonResp.Error.Code != -32000 {
			t.Errorf("expected code -32000, got %d", jsonResp.Error.Code)
		}
		if !strings.Contains(jsonResp.Error.Message, "backend model timeout") {
			t.Errorf("expected error message to contain timeout, got %q", jsonResp.Error.Message)
		}
	})
}

func TestResolveBaseURL(t *testing.T) {
	t.Run("APP_URL env takes precedence", func(t *testing.T) {
		os.Setenv("APP_URL", "https://custom-agent.run.app/")
		defer os.Unsetenv("APP_URL")

		handler := NewHandler("sre-triage-agent", mockInvoker)
		req := httptest.NewRequest("GET", "/.well-known/agent-card.json", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		var card AgentCard
		_ = json.NewDecoder(w.Body).Decode(&card)
		if card.URL != "https://custom-agent.run.app/a2a/invoke" {
			t.Errorf("expected url 'https://custom-agent.run.app/a2a/invoke', got %q", card.URL)
		}
	})

	t.Run("X-Forwarded-Proto header", func(t *testing.T) {
		os.Unsetenv("APP_URL")

		handler := NewHandler("sre-triage-agent", mockInvoker)
		req := httptest.NewRequest("GET", "/.well-known/agent-card.json", nil)
		req.Host = "agent.internal:8080"
		req.Header.Set("X-Forwarded-Proto", "https")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		var card AgentCard
		_ = json.NewDecoder(w.Body).Decode(&card)
		if card.URL != "https://agent.internal:8080/a2a/invoke" {
			t.Errorf("expected url 'https://agent.internal:8080/a2a/invoke', got %q", card.URL)
		}
	})
}

func TestA2A1_0_Methods(t *testing.T) {
	handler := NewHandler("sre-triage-agent", mockInvoker)

	for _, method := range []string{"SendMessage", "SendStreamingMessage"} {
		t.Run(method, func(t *testing.T) {
			payload := JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      "req-v1-" + method,
				Method:  method,
				Params: MessageSendParams{
					Message: A2AMessage{
						Role:      "user",
						Parts:     []A2APart{{Text: "Test v1 method"}},
						ContextID: "ctx-v1",
					},
				},
			}
			body, _ := json.Marshal(payload)

			req := httptest.NewRequest("POST", "/a2a/invoke", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}

			var jsonResp JSONRPCResponse
			if err := json.NewDecoder(w.Body).Decode(&jsonResp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if jsonResp.Result == nil || jsonResp.Result.ContextID != "ctx-v1" {
				t.Errorf("unexpected result: %+v", jsonResp.Result)
			}
		})
	}
}

func TestUnmatchedPathReturns404(t *testing.T) {
	handler := NewHandler("sre-triage-agent", mockInvoker)

	paths := []string{
		"/a2a/.well-known/agent-card.json",
		"/unknown-path",
		"/api/v1/invalid",
	}

	for _, path := range paths {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for path %q, got %d", path, w.Code)
		}
	}

	// Paths containing "." are canonicalized with 301 redirect by http.ServeMux
	reqDot := httptest.NewRequest("GET", "/a2a/./.well-known/agent-card.json", nil)
	wDot := httptest.NewRecorder()
	handler.ServeHTTP(wDot, reqDot)
	if wDot.Code != http.StatusMovedPermanently {
		t.Errorf("expected 301 for path with dot, got %d", wDot.Code)
	}
}

func TestA2AMessageSerialization(t *testing.T) {
	msg := A2AMessage{
		Role:      "agent",
		Parts:     []A2APart{{Text: "Triage complete"}},
		ContextID: "ctx-test-ser",
	}

	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	rawStr := string(b)
	if !strings.Contains(rawStr, `"message"`) || !strings.Contains(rawStr, `"ROLE_AGENT"`) {
		t.Errorf("expected serialized A2AMessage to contain 'message' and 'ROLE_AGENT', got %s", rawStr)
	}

	// Test Unmarshal from wrapped format
	var unmarshaled A2AMessage
	if err := json.Unmarshal(b, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal wrapped message: %v", err)
	}
	if unmarshaled.Role != "agent" {
		t.Errorf("expected role 'agent', got %q", unmarshaled.Role)
	}
	if len(unmarshaled.Parts) == 0 || unmarshaled.Parts[0].Text != "Triage complete" {
		t.Errorf("unexpected parts: %+v", unmarshaled.Parts)
	}

	// Test Unmarshal from direct unwrapped format
	directJSON := `{"role":"ROLE_AGENT","parts":[{"text":"Direct"}],"contextId":"ctx-dir"}`
	var directMsg A2AMessage
	if err := json.Unmarshal([]byte(directJSON), &directMsg); err != nil {
		t.Fatalf("failed to unmarshal direct: %v", err)
	}
	if directMsg.Role != "agent" || directMsg.Parts[0].Text != "Direct" {
		t.Errorf("unexpected direct message: %+v", directMsg)
	}
}

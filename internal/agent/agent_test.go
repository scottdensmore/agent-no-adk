package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"google.golang.org/genai"
)

func TestSystemPromptContent(t *testing.T) {
	prompt := GetSystemInstruction()
	if !strings.Contains(prompt, "SRE Incident Triage") {
		t.Errorf("expected SRE persona in instruction: %s", prompt)
	}
	if !strings.Contains(prompt, "CRITICAL") || !strings.Contains(prompt, "WARNING") || !strings.Contains(prompt, "INFO") {
		t.Errorf("missing severity guidelines: %s", prompt)
	}
	if !strings.Contains(prompt, "format_incident_report") {
		t.Errorf("missing mention of format_incident_report tool: %s", prompt)
	}
	if !strings.Contains(prompt, "read_log_file") {
		t.Errorf("missing mention of read_log_file tool: %s", prompt)
	}
	if !strings.Contains(prompt, "parse_log_snippet") {
		t.Errorf("missing mention of parse_log_snippet tool: %s", prompt)
	}
	if !strings.Contains(prompt, "[REDACTED]") {
		t.Errorf("missing redaction guideline: %s", prompt)
	}
}

func TestMockAgentLoop(t *testing.T) {
	// Verifies agent configuration and validation without making live network calls
	cfg := Config{
		Model: "gemini-3.8-flash",
	}
	agent, err := NewAgentWithDispatcher(cfg, func(name string, args map[string]any) (any, error) {
		return "tool response", nil
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	if agent.ModelName != "gemini-3.8-flash" {
		t.Errorf("expected model 'gemini-3.8-flash', got %s", agent.ModelName)
	}
}

func TestAgentDefaultModel(t *testing.T) {
	agent, err := NewAgentWithDispatcher(Config{}, func(name string, args map[string]any) (any, error) {
		return nil, nil
	})
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	if agent.ModelName != "gemini-3.8-flash" {
		t.Errorf("expected default model 'gemini-3.8-flash', got %s", agent.ModelName)
	}
}

func TestUninitializedAgentInvoke(t *testing.T) {
	agent, err := NewAgentWithDispatcher(Config{Model: "gemini-3.8-flash"}, nil)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	resp, err := agent.Invoke(context.Background(), "Triage logs", "ctx-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp, "uninitialized") {
		t.Errorf("expected uninitialized notice, got: %s", resp)
	}
}

func TestNewAgentWithAPIKey(t *testing.T) {
	ctx := context.Background()
	agent, err := NewAgent(ctx, Config{
		APIKey: "fake-test-key",
	})
	if err != nil {
		t.Fatalf("failed to create agent with APIKey: %v", err)
	}
	if agent.ModelName != "gemini-3.8-flash" {
		t.Errorf("expected default model 'gemini-3.8-flash', got %s", agent.ModelName)
	}
	if agent.client == nil {
		t.Errorf("expected initialized genai.Client")
	}
}

func TestAgentReActLoopWithMockServer(t *testing.T) {
	turnCount := 0
	var dispatchedTools []string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		turnCount++
		w.Header().Set("Content-Type", "application/json")

		if turnCount == 1 {
			// Turn 1: Model responds with a function call to read_log_file
			resp := map[string]any{
				"candidates": []map[string]any{
					{
						"content": map[string]any{
							"role": "model",
							"parts": []map[string]any{
								{
									"functionCall": map[string]any{
										"name": "read_log_file",
										"args": map[string]any{
											"file_path": "sample_logs/gateway_timeout.log",
										},
									},
								},
							},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Turn 2: Model returns final markdown incident report
		resp := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"role": "model",
						"parts": []map[string]any{
							{
								"text": "# SRE Incident Report\nSeverity: WARNING\nIssue: Gateway Timeout",
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend: genai.BackendGeminiAPI,
		APIKey:  "test-api-key",
		HTTPOptions: genai.HTTPOptions{
			BaseURL: mockServer.URL + "/",
		},
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	dispatcher := func(name string, args map[string]any) (any, error) {
		dispatchedTools = append(dispatchedTools, name)
		return map[string]any{"lines": []string{"log line 1", "log line 2"}}, nil
	}

	agent := &Agent{
		client:     client,
		ModelName:  "gemini-3.8-flash",
		dispatcher: dispatcher,
	}

	result, err := agent.Invoke(ctx, "Please triage the gateway timeout logs.", "ctx-react-test")
	if err != nil {
		t.Fatalf("agent.Invoke failed: %v", err)
	}

	if len(dispatchedTools) != 1 || dispatchedTools[0] != "read_log_file" {
		t.Errorf("expected read_log_file to be dispatched, got: %v", dispatchedTools)
	}
	if !strings.Contains(result, "# SRE Incident Report") {
		t.Errorf("expected final report in output, got: %s", result)
	}
}

func TestAgentMaxTurns(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Always return a function call to loop indefinitely
		resp := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"role": "model",
						"parts": []map[string]any{
							{
								"functionCall": map[string]any{
									"name": "read_log_file",
									"args": map[string]any{
										"file_path": "sample.log",
									},
								},
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend: genai.BackendGeminiAPI,
		APIKey:  "test-api-key",
		HTTPOptions: genai.HTTPOptions{
			BaseURL: mockServer.URL + "/",
		},
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	callCount := 0
	agent := &Agent{
		client:    client,
		ModelName: "gemini-3.8-flash",
		dispatcher: func(name string, args map[string]any) (any, error) {
			callCount++
			return map[string]any{"status": "ok"}, nil
		},
	}

	result, err := agent.Invoke(ctx, "Loop test", "ctx-loop")
	if err != nil {
		t.Fatalf("agent.Invoke failed: %v", err)
	}
	if !strings.Contains(result, "maximum triage loop iterations") {
		t.Errorf("expected max iterations message, got: %s", result)
	}
	if callCount != 8 {
		t.Errorf("expected 8 tool calls before hitting max turn limit, got: %d", callCount)
	}
}

func TestNewAgentWithClientOverride(t *testing.T) {
	mockClient := &genai.Client{}
	agent, err := NewAgent(context.Background(), Config{
		Model:  "gemini-custom",
		Client: mockClient,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent.client != mockClient {
		t.Errorf("expected client override to be set")
	}
	if agent.ModelName != "gemini-custom" {
		t.Errorf("expected model 'gemini-custom', got: %s", agent.ModelName)
	}
}

func TestNewAgentModelFromEnv(t *testing.T) {
	os.Setenv("GEMINI_MODEL", "gemini-env-override")
	defer os.Unsetenv("GEMINI_MODEL")

	agent, err := NewAgent(context.Background(), Config{
		APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent.ModelName != "gemini-env-override" {
		t.Errorf("expected model 'gemini-env-override', got: %s", agent.ModelName)
	}
}

func TestInvoke_EmptyCandidates(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []any{},
		})
	}))
	defer mockServer.Close()

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend: genai.BackendGeminiAPI,
		APIKey:  "test-api-key",
		HTTPOptions: genai.HTTPOptions{
			BaseURL: mockServer.URL + "/",
		},
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	agent := &Agent{
		client:    client,
		ModelName: "gemini-3.8-flash",
	}

	result, err := agent.Invoke(ctx, "Test prompt", "ctx-empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "No response generated by model." {
		t.Errorf("expected 'No response generated by model.', got: %s", result)
	}
}

func TestInvoke_GenerateContentError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend: genai.BackendGeminiAPI,
		APIKey:  "test-api-key",
		HTTPOptions: genai.HTTPOptions{
			BaseURL: mockServer.URL + "/",
		},
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	agent := &Agent{
		client:    client,
		ModelName: "gemini-3.8-flash",
	}

	_, err = agent.Invoke(ctx, "Test prompt", "ctx-err")
	if err == nil {
		t.Fatalf("expected error from Invoke, got nil")
	}
	if !strings.Contains(err.Error(), "gemini generate content error") {
		t.Errorf("expected 'gemini generate content error', got: %v", err)
	}
}

func TestInvoke_ToolDispatcherErrorAndScalarResult(t *testing.T) {
	turnCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		turnCount++
		w.Header().Set("Content-Type", "application/json")
		if turnCount == 1 {
			resp := map[string]any{
				"candidates": []map[string]any{
					{
						"content": map[string]any{
							"role": "model",
							"parts": []map[string]any{
								{
									"functionCall": map[string]any{
										"name": "failing_tool",
										"args": map[string]any{},
									},
								},
								{
									"functionCall": map[string]any{
										"name": "scalar_tool",
										"args": map[string]any{},
									},
								},
							},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		resp := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"role": "model",
						"parts": []map[string]any{
							{
								"text": "Handled errors and scalar values smoothly.",
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend: genai.BackendGeminiAPI,
		APIKey:  "test-api-key",
		HTTPOptions: genai.HTTPOptions{
			BaseURL: mockServer.URL + "/",
		},
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	agent := &Agent{
		client:    client,
		ModelName: "gemini-3.8-flash",
		dispatcher: func(name string, args map[string]any) (any, error) {
			if name == "failing_tool" {
				return nil, fmt.Errorf("tool failed purposely")
			}
			return "simple string output", nil
		},
	}

	result, err := agent.Invoke(ctx, "Error test prompt", "ctx-err-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Handled errors") {
		t.Errorf("unexpected result: %s", result)
	}
}

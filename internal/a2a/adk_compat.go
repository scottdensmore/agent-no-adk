package a2a

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// handleAppsRouter routes /apps/... subpaths like /app-info and /sessions
func (h *Handler) handleAppsRouter(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/app-info") {
		h.handleAppInfo(w, r)
		return
	}
	if strings.Contains(r.URL.Path, "/sessions") {
		h.handleSessions(w, r)
		return
	}
	http.NotFound(w, r)
}

// handleAppInfo serves GET /apps/{app_name}/app-info
func (h *Handler) handleAppInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"rootAgentName": h.appName,
		"agents": map[string]any{
			h.appName: map[string]any{
				"name":        h.appName,
				"description": "Autonomous SRE incident triage assistant",
			},
		},
	})
}

// handleSessions serves POST /apps/{app_name}/users/{user_id}/sessions
func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	sessionID := uuid.New().String()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id": sessionID,
	})
}

// handleRunSSE serves POST /run_sse
func (h *Handler) handleRunSSE(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AppName    string `json:"appName"`
		UserID     string `json:"userId"`
		SessionID  string `json:"sessionId"`
		NewMessage struct {
			Role  string `json:"role"`
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"newMessage"`
	}

	rawBody, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 10<<20))
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	var promptBuilder strings.Builder
	for _, p := range body.NewMessage.Parts {
		if p.Text != "" {
			if promptBuilder.Len() > 0 {
				promptBuilder.WriteString("\n")
			}
			promptBuilder.WriteString(p.Text)
		}
	}

	sessionID := body.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, hasFlusher := w.(http.Flusher)

	writeSSE := func(author string, role string, part map[string]any) {
		payload := map[string]any{
			"author": author,
			"content": map[string]any{
				"role":  role,
				"parts": []map[string]any{part},
			},
		}
		if b, err := json.Marshal(payload); err == nil {
			fmt.Fprintf(w, "data: %s\n\n", b)
			if hasFlusher {
				flusher.Flush()
			}
		}
	}

	if h.streamInvoker != nil {
		_, err := h.streamInvoker(r.Context(), promptBuilder.String(), sessionID, func(ev StreamEvent) {
			switch ev.Type {
			case "functionCall":
				writeSSE(h.appName, "model", map[string]any{
					"functionCall": map[string]any{
						"name": ev.Name,
						"args": ev.Args,
					},
				})
			case "functionResponse":
				writeSSE(h.appName, "tool", map[string]any{
					"functionResponse": map[string]any{
						"name":     ev.Name,
						"response": ev.Response,
					},
				})
			case "text":
				writeSSE(h.appName, "model", map[string]any{
					"text": ev.Text,
				})
			}
		})
		if err != nil {
			writeSSE(h.appName, "model", map[string]any{
				"text": fmt.Sprintf("Error executing agent: %v", err),
			})
		}
		return
	}

	answer, err := h.invoker(r.Context(), promptBuilder.String(), sessionID)
	if err != nil {
		answer = fmt.Sprintf("Error executing agent: %v", err)
	}

	writeSSE(h.appName, "model", map[string]any{
		"text": answer,
	})
}

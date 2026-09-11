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

	answer, err := h.invoker(r.Context(), promptBuilder.String(), sessionID)
	if err != nil {
		answer = fmt.Sprintf("Error executing agent: %v", err)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	eventPayload := map[string]any{
		"author": h.appName,
		"content": map[string]any{
			"role": "model",
			"parts": []map[string]any{
				{"text": answer},
			},
		},
	}
	eventJSON, _ := json.Marshal(eventPayload)

	fmt.Fprintf(w, "data: %s\n\n", eventJSON)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type AgentInvoker func(ctx context.Context, message string, contextID string) (string, error)

type Handler struct {
	appName string
	invoker AgentInvoker
}

func NewHandler(appName string, invoker AgentInvoker) http.Handler {
	h := &Handler{
		appName: appName,
		invoker: invoker,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", h.handleAgentCard)
	mux.HandleFunc(fmt.Sprintf("/a2a/%s/.well-known/agent-card.json", appName), h.handleAgentCard)
	mux.HandleFunc("/a2a/invoke", h.handleInvoke)
	mux.HandleFunc(fmt.Sprintf("/a2a/%s/invoke", appName), h.handleInvoke)
	mux.HandleFunc("/list-apps", h.handleListApps)
	mux.HandleFunc("/", h.handleRoot)

	return mux
}

func (h *Handler) resolveBaseURL(r *http.Request) string {
	if envURL := os.Getenv("APP_URL"); envURL != "" {
		return strings.TrimSuffix(envURL, "/")
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

func (h *Handler) handleAgentCard(w http.ResponseWriter, r *http.Request) {
	baseURL := h.resolveBaseURL(r)
	invokeURL := fmt.Sprintf("%s/a2a/invoke", baseURL)

	card := AgentCard{
		ProtocolVersion:    "1.0",
		Name:               h.appName,
		Description:        "Autonomous SRE incident triage assistant for log parsing, anomaly classification, and postmortem generation.",
		URL:                invokeURL,
		PreferredTransport: "JSONRPC",
		SupportedInterfaces: []AgentInterface{
			{
				URL:             invokeURL,
				ProtocolBinding: "JSONRPC",
				ProtocolVersion: "1.0",
			},
		},
		Capabilities: AgentCapabilities{
			Streaming: true,
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain", "text/markdown"},
		Skills: []AgentSkill{
			{
				ID:          "sre_incident_triage",
				Name:        "SRE Incident Triage",
				Description: "Parse server logs, classify incident severity, identify root causes, and format markdown incident reports.",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(card)
}

func (h *Handler) handleListApps(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode([]string{h.appName})
}

func (h *Handler) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		h.handleInvoke(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"agent":  h.appName,
	})
}

func (h *Handler) handleInvoke(w http.ResponseWriter, r *http.Request) {
	ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	tr := otel.Tracer("sre-triage-agent")
	ctx, span := tr.Start(ctx, "a2a.invoke", trace.WithSpanKind(trace.SpanKindServer))
	defer span.End()

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "request body read error")
		h.writeError(w, nil, -32700, "Request body too large or failed to read")
		return
	}

	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "parse error")
		h.writeError(w, nil, -32700, "Parse error")
		return
	}

	if req.Method != "message/send" && req.Method != "tasks/create" && req.Method != "SendMessage" && req.Method != "SendStreamingMessage" {
		span.SetStatus(codes.Error, "method not found")
		h.writeError(w, req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
		return
	}

	var promptText strings.Builder
	for _, part := range req.Params.Message.Parts {
		if part.Text != "" {
			if promptText.Len() > 0 {
				promptText.WriteString("\n")
			}
			promptText.WriteString(part.Text)
		}
	}

	contextID := req.Params.Message.ContextID
	if contextID == "" {
		contextID = uuid.New().String()
	}

	answer, err := h.invoker(ctx, promptText.String(), contextID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		h.writeError(w, req.ID, -32000, fmt.Sprintf("Agent execution failed: %v", err))
		return
	}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: &A2AMessage{
			Role: "agent",
			Parts: []A2APart{
				{
					Text: answer,
				},
			},
			ContextID: contextID,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) writeError(w http.ResponseWriter, id any, code int, msg string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: msg,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

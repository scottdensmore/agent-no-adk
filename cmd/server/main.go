package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sre-triage-agent/internal/a2a"
	"sre-triage-agent/internal/agent"
	"sre-triage-agent/internal/telemetry"
)

func resolveConfig(portFlag, modelFlag string) (port string, model string) {
	port = portFlag
	if port == "" {
		port = os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
	}

	model = modelFlag
	if model == "" {
		model = os.Getenv("GEMINI_MODEL")
		if model == "" {
			model = "gemini-3.8-flash"
		}
	}
	return port, model
}

func setupHandler(sreAgent *agent.Agent) http.Handler {
	invoker := func(ctx context.Context, message string, contextID string) (string, error) {
		if sreAgent == nil {
			return "Agent credentials not configured. Set GEMINI_API_KEY or GOOGLE_CLOUD_PROJECT with ADC.", nil
		}
		return sreAgent.Invoke(ctx, message, contextID)
	}

	return a2a.NewHandler("sre-triage-agent", invoker)
}

func main() {
	portFlag := flag.String("port", "", "Port to listen on (defaults to $PORT or 8080)")
	modelFlag := flag.String("model", "", "Gemini model (defaults to $GEMINI_MODEL or gemini-3.8-flash)")
	flag.Parse()

	port, model := resolveConfig(*portFlag, *modelFlag)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize OpenTelemetry Cloud Trace
	shutdownTracer, err := telemetry.InitTracer(ctx, "sre-triage-agent")
	if err != nil {
		log.Printf("Warning: OpenTelemetry tracer initialization failed: %v", err)
	} else {
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			if err := shutdownTracer(shutdownCtx); err != nil {
				log.Printf("Error shutting down tracer: %v", err)
			}
		}()
	}

	// Initialize Cloud Logging structured JSON logger
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	slog.SetDefault(telemetry.NewGCPLogger(projectID, os.Stdout, slog.LevelInfo))

	sreAgent, err := agent.NewAgent(ctx, agent.Config{
		Model: model,
	})
	if err != nil {
		slog.Warn("Live GenAI client initialization failed. Starting server in fallback mode.", "error", err)
	}

	handler := setupHandler(sreAgent)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	go func() {
		slog.Info("SRE Triage Agent listening", "port", port, "model", model)
		slog.Info("Agent Card endpoint", "url", "http://localhost:"+port+"/.well-known/agent-card.json")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown error", "error", err)
	}
	slog.Info("Server exited cleanly.")
}

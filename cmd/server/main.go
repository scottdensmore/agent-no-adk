package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sre-triage-agent/internal/a2a"
	"sre-triage-agent/internal/agent"
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

	sreAgent, err := agent.NewAgent(ctx, agent.Config{
		Model: model,
	})
	if err != nil {
		log.Printf("Warning: Live GenAI client initialization failed (%v). Starting server in fallback mode.", err)
	}

	handler := setupHandler(sreAgent)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	go func() {
		log.Printf("SRE Triage Agent listening on port %s", port)
		log.Printf("Agent Card: http://localhost:%s/.well-known/agent-card.json", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("Server exited cleanly.")
}

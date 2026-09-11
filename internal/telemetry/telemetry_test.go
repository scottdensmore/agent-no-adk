package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestGCPStructuredLogger(t *testing.T) {
	var buf bytes.Buffer
	projectID := "test-gcp-project"
	logger := NewGCPLogger(projectID, &buf, slog.LevelInfo)

	// Set up an in-memory tracer provider to generate real spans
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()
	tracer := tp.Tracer("test-tracer")

	ctx, span := tracer.Start(context.Background(), "test-operation")
	defer span.End()

	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	logger.InfoContext(ctx, "analyzing server incident", "service", "order-service")

	var logMap map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logMap); err != nil {
		t.Fatalf("failed to decode structured log JSON: %v, raw: %s", err, buf.String())
	}

	if sev, ok := logMap["severity"].(string); !ok || sev != "INFO" {
		t.Errorf("expected severity 'INFO', got %v", logMap["severity"])
	}

	if msg, ok := logMap["message"].(string); !ok || msg != "analyzing server incident" {
		t.Errorf("expected message 'analyzing server incident', got %v", logMap["message"])
	}

	expectedTrace := "projects/" + projectID + "/traces/" + traceID
	if trace, ok := logMap["logging.googleapis.com/trace"].(string); !ok || trace != expectedTrace {
		t.Errorf("expected trace %q, got %v", expectedTrace, logMap["logging.googleapis.com/trace"])
	}

	if sID, ok := logMap["logging.googleapis.com/spanId"].(string); !ok || sID != spanID {
		t.Errorf("expected spanId %q, got %v", spanID, logMap["logging.googleapis.com/spanId"])
	}
}

func TestInitTracer_Fallback(t *testing.T) {
	ctx := context.Background()
	shutdown, err := InitTracer(ctx, "sre-triage-agent")
	if err != nil {
		t.Fatalf("InitTracer failed: %v", err)
	}
	defer func() { _ = shutdown(ctx) }()

	tr := otel.Tracer("test")
	_, span := tr.Start(ctx, "fallback-span")
	if span == nil {
		t.Fatal("expected non-nil span from tracer")
	}
	span.End()
}

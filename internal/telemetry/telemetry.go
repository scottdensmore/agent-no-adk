package telemetry

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"

	cloudtrace "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const TracerName = "sre-triage-agent"

// Tracer returns the default OpenTelemetry tracer for the agent.
func Tracer() trace.Tracer {
	return otel.Tracer(TracerName)
}

// InitTracer configures Cloud Trace export via OpenTelemetry.
// If GCP credentials are not available, it safely falls back to an in-memory tracer provider.
func InitTracer(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	if serviceName == "" {
		serviceName = "sre-triage-agent"
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		res = resource.Default()
	}

	var opts []cloudtrace.Option
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID != "" {
		opts = append(opts, cloudtrace.WithProjectID(projectID))
	}

	exporter, err := cloudtrace.New(opts...)
	var tp *sdktrace.TracerProvider
	if err != nil {
		log.Printf("Telemetry: Cloud Trace exporter unavailable (%v). Using local TracerProvider.", err)
		tp = sdktrace.NewTracerProvider(sdktrace.WithResource(res))
	} else {
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(res),
		)
		log.Printf("Telemetry: Cloud Trace exporter enabled for service %q", serviceName)
	}

	otel.SetTracerProvider(tp)

	return func(shutdownCtx context.Context) error {
		return tp.Shutdown(shutdownCtx)
	}, nil
}

// gcpLogHandler formats logs as structured JSON compatible with Google Cloud Logging.
type gcpLogHandler struct {
	projectID string
	handler   slog.Handler
}

// NewGCPLogger returns an slog.Logger configured to output Google Cloud Logging JSON with trace correlation.
func NewGCPLogger(projectID string, w io.Writer, level slog.Level) *slog.Logger {
	if w == nil {
		w = os.Stdout
	}
	if projectID == "" {
		projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}

	jsonHandler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.MessageKey:
				return slog.Attr{Key: "message", Value: a.Value}
			case slog.TimeKey:
				return slog.Attr{Key: "timestamp", Value: a.Value}
			case slog.LevelKey:
				lvl := a.Value.Any().(slog.Level)
				sev := "DEFAULT"
				switch {
				case lvl >= slog.LevelError:
					sev = "ERROR"
				case lvl >= slog.LevelWarn:
					sev = "WARNING"
				case lvl >= slog.LevelInfo:
					sev = "INFO"
				case lvl >= slog.LevelDebug:
					sev = "DEBUG"
				}
				return slog.Attr{Key: "severity", Value: slog.StringValue(sev)}
			}
			return a
		},
	})

	return slog.New(&gcpLogHandler{
		projectID: projectID,
		handler:   jsonHandler,
	})
}

func (h *gcpLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *gcpLogHandler) Handle(ctx context.Context, r slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		traceID := span.SpanContext().TraceID().String()
		spanID := span.SpanContext().SpanID().String()

		if h.projectID != "" {
			r.AddAttrs(slog.String("logging.googleapis.com/trace", fmt.Sprintf("projects/%s/traces/%s", h.projectID, traceID)))
		} else {
			r.AddAttrs(slog.String("logging.googleapis.com/trace", traceID))
		}
		r.AddAttrs(slog.String("logging.googleapis.com/spanId", spanID))
		r.AddAttrs(slog.Bool("logging.googleapis.com/trace_sampled", span.SpanContext().IsSampled()))
	}

	return h.handler.Handle(ctx, r)
}

func (h *gcpLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &gcpLogHandler{
		projectID: h.projectID,
		handler:   h.handler.WithAttrs(attrs),
	}
}

func (h *gcpLogHandler) WithGroup(name string) slog.Handler {
	return &gcpLogHandler{
		projectID: h.projectID,
		handler:   h.handler.WithGroup(name),
	}
}

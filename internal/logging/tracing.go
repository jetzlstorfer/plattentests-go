package logging

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracerProvider *sdktrace.TracerProvider
	tracer         trace.Tracer
)

// InitTracing initializes the OpenTelemetry tracing with console exporter
func InitTracing(serviceName string) error {
	// Create console trace exporter
	exporter, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		return fmt.Errorf("failed to create stdout trace exporter: %w", err)
	}

	// Create trace provider
	tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(nil), // Use default resource
	)

	// Set global trace provider
	otel.SetTracerProvider(tracerProvider)

	// Create tracer
	tracer = tracerProvider.Tracer(serviceName)

	return nil
}

// ShutdownTracing gracefully shuts down the tracer
func ShutdownTracing() error {
	if tracerProvider != nil {
		return tracerProvider.Shutdown(context.Background())
	}
	return nil
}

// StartSpan starts a new span with the given name
// Returns the new context with the span and the span itself
func StartSpan(ctx context.Context, spanName string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if tracer == nil {
		// If tracing is not initialized, return the original context with a no-op span
		return ctx, trace.SpanFromContext(ctx)
	}

	ctx, span := tracer.Start(ctx, spanName)
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
	return ctx, span
}

// SpanFromContext returns the current span from the context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// AddSpanEvent adds an event to the current span
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// SetSpanStatus sets the status of the current span
func SetSpanStatus(span trace.Span, err error, msg string) {
	if err != nil {
		span.SetStatus(codes.Error, msg)
		span.RecordError(err)
	} else {
		span.SetStatus(codes.Ok, msg)
	}
}

// AddSpanAttributes adds attributes to the current span
func AddSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attrs...)
	}
}

// InfoWithSpan logs an info message and adds it as a span event
func InfoWithSpan(ctx context.Context, msg string, args ...interface{}) {
	Info(msg, args...)
	if len(args) > 0 {
		AddSpanEvent(ctx, fmt.Sprintf(msg, args...))
	} else {
		AddSpanEvent(ctx, msg)
	}
}

// ErrorWithSpan logs an error message and records it in the span
func ErrorWithSpan(ctx context.Context, err error, msg string, args ...interface{}) {
	formattedMsg := msg
	if len(args) > 0 {
		formattedMsg = fmt.Sprintf(msg, args...)
		Error("%s", formattedMsg)
	} else {
		Error("%s", msg)
	}

	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetStatus(codes.Error, formattedMsg)
		if err != nil {
			span.RecordError(err)
		}
	}
}

// Attribute creates an attribute key-value pair
func Attribute(key string, value interface{}) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	case bool:
		return attribute.Bool(key, v)
	default:
		return attribute.String(key, fmt.Sprintf("%v", v))
	}
}

# OpenTelemetry Tracing and Logging

This project now uses OpenTelemetry for both logging and distributed tracing. This document explains how they work together and when to use each.

## Overview

### Logging
- **Purpose**: Record discrete events and messages
- **Use case**: Debug messages, info statements, warnings, errors
- **Output**: Console (stdout) with pretty-printed format
- **Package**: `internal/logging` - `Init()`, `Info()`, `Debug()`, `Warn()`, `Error()`, `Fatal()`

### Tracing
- **Purpose**: Track the execution flow and timing of operations
- **Use case**: Performance monitoring, request tracing, understanding call hierarchies
- **Output**: Console (stdout) with pretty-printed format showing spans
- **Package**: `internal/logging` - `InitTracing()`, `StartSpan()`, `AddSpanEvent()`

## Key Concepts

### Traces and Spans
- **Trace**: A complete end-to-end journey of a request through your system
- **Span**: A single unit of work within a trace (like a function call or operation)
- **Parent-Child Relationships**: Spans can have parent spans, creating a hierarchy

### Combining Logs and Traces
You can attach log events to spans, creating a correlation between logs and the execution flow:

```go
ctx, span := logging.StartSpan(ctx, "operation-name")
defer span.End()

logging.InfoWithSpan(ctx, "Processing item: %s", itemName)
// This logs to console AND adds an event to the span
```

## Usage Examples

### Basic Span Creation

```go
func MyFunction() {
    ctx := context.Background()
    ctx, span := logging.StartSpan(ctx, "MyFunction")
    defer span.End()

    // Your code here
}
```

### Nested Spans (Parent-Child)

```go
func OuterFunction() {
    ctx := context.Background()
    ctx, outerSpan := logging.StartSpan(ctx, "OuterFunction")
    defer outerSpan.End()

    // Inner operation
    ctx, innerSpan := logging.StartSpan(ctx, "InnerOperation")
    // Do work
    innerSpan.End()
}
```

### Adding Attributes to Spans

```go
ctx, span := logging.StartSpan(ctx, "fetch-user")
defer span.End()

logging.AddSpanAttributes(ctx,
    logging.Attribute("user_id", userID),
    logging.Attribute("retry_count", retries))
```

### Adding Events to Spans

```go
logging.AddSpanEvent(ctx, "Cache miss",
    logging.Attribute("key", cacheKey))
```

### Error Handling with Spans

```go
err := doSomething()
if err != nil {
    logging.ErrorWithSpan(ctx, err, "Failed to do something: %v", err)
    // This logs the error AND marks the span as failed
}
```

## Initialization

Each application entry point should initialize both logging and tracing:

```go
func main() {
    // Initialize logging
    if err := logging.Init(); err != nil {
        logging.Fatal("Failed to initialize logging: %v", err)
    }
    defer logging.Shutdown()

    // Initialize tracing
    if err := logging.InitTracing("service-name"); err != nil {
        logging.Fatal("Failed to initialize tracing: %v", err)
    }
    defer logging.ShutdownTracing()

    // Your application code
}
```

## When to Use What

### Use Logging When:
- ✅ You want to record a specific event or state
- ✅ You need debug information
- ✅ You want to log errors or warnings
- ✅ Simple status updates

### Use Tracing When:
- ✅ You want to measure operation duration
- ✅ You need to understand the flow through multiple functions
- ✅ You want to track performance bottlenecks
- ✅ You need distributed tracing across services

### Use Both (Logging + Tracing) When:
- ✅ You want logs correlated with specific operations
- ✅ You need both the "what happened" (logs) and "how long it took" (traces)
- ✅ You're debugging performance issues

## Output Format

### Console Logs
```json
{
  "Severity": "INFO",
  "Body": "Fetching records of the week"
}
```

### Console Traces
```json
{
  "Name": "GET /",
  "SpanContext": {
    "TraceID": "abc123...",
    "SpanID": "def456..."
  },
  "StartTime": "2026-03-03T15:00:00Z",
  "EndTime": "2026-03-03T15:00:01Z",
  "Attributes": [
    {"Key": "count", "Value": {"IntValue": 25}}
  ],
  "Events": [
    {"Name": "Records fetched", "Attributes": [...]}
  ]
}
```

## Migration to OTLP Backend

When you're ready to send telemetry to an observability backend (like Jaeger, Tempo, or Grafana Cloud):

1. Replace the stdout exporters with OTLP exporters in `internal/logging/logger.go` and `internal/logging/tracing.go`
2. Configure the OTLP endpoint via environment variables
3. No code changes needed in your application logic!

## Do You Need Logging?

**Short answer**: Yes, for now!

While tracing provides detailed execution flow and timing, logging is still valuable for:
- Quick debugging during development
- Simple event recording
- Human-readable output in the console
- Error messages that don't fit the span model

**In the future**: When you have a full observability platform, you might rely more on traces with span events, but logs still serve a purpose for development and simple diagnostics.

## Best Practices

1. **Always use context**: Pass context through your call chain to maintain span relationships
2. **Defer span.End()**: Always defer span ending to ensure it's called even on errors
3. **Add meaningful attributes**: Include relevant data that helps debugging
4. **Use descriptive span names**: Use kebab-case like "fetch-records" or "search-spotify-track"
5. **Keep spans focused**: Each span should represent one logical operation
6. **Record errors properly**: Use `ErrorWithSpan()` to capture errors in both logs and traces

## Example: Full Request Flow

```go
func HandleRequest(c *gin.Context) {
    ctx, span := logging.StartSpan(c.Request.Context(), "HandleRequest")
    defer span.End()

    logging.InfoWithSpan(ctx, "Handling request")

    // Fetch data
    ctx, fetchSpan := logging.StartSpan(ctx, "fetch-data")
    data, err := FetchData(ctx)
    if err != nil {
        logging.ErrorWithSpan(ctx, err, "Failed to fetch data")
        fetchSpan.End()
        return
    }
    logging.AddSpanEvent(ctx, "Data fetched", logging.Attribute("count", len(data)))
    fetchSpan.End()

    // Process data
    ctx, processSpan := logging.StartSpan(ctx, "process-data")
    result := ProcessData(ctx, data)
    logging.AddSpanEvent(ctx, "Data processed")
    processSpan.End()

    logging.InfoWithSpan(ctx, "Request completed successfully")
}
```

This creates a trace hierarchy:
```
HandleRequest
├── fetch-data
└── process-data
```

Each span records its duration and any events/errors that occurred.

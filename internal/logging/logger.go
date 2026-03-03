package logging

import (
	"context"
	"fmt"
	stdlog "log"
	"os"

	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	logapi "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

// Logger wraps OpenTelemetry logging functionality
type Logger struct {
	provider *sdklog.LoggerProvider
	logger   logapi.Logger
	ctx      context.Context
}

var defaultLogger *Logger

// Init initializes the global logger with a console exporter
func Init() error {
	// Create console exporter
	exporter, err := stdoutlog.New(
		stdoutlog.WithPrettyPrint(),
	)
	if err != nil {
		return fmt.Errorf("failed to create stdout exporter: %w", err)
	}

	// Create logger provider
	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
	)

	// Create logger
	logger := provider.Logger("plattentests")

	defaultLogger = &Logger{
		provider: provider,
		logger:   logger,
		ctx:      context.Background(),
	}

	return nil
}

// Shutdown gracefully shuts down the logger
func Shutdown() error {
	if defaultLogger != nil && defaultLogger.provider != nil {
		return defaultLogger.provider.Shutdown(context.Background())
	}
	return nil
}

// Info logs an informational message
func Info(msg string, args ...interface{}) {
	if defaultLogger == nil {
		stdlog.Printf("[INFO] "+msg, args...)
		return
	}

	var record logapi.Record
	record.SetSeverity(logapi.SeverityInfo)
	if len(args) > 0 {
		record.SetBody(logapi.StringValue(fmt.Sprintf(msg, args...)))
	} else {
		record.SetBody(logapi.StringValue(msg))
	}
	defaultLogger.logger.Emit(defaultLogger.ctx, record)
}

// Debug logs a debug message
func Debug(msg string, args ...interface{}) {
	if defaultLogger == nil {
		stdlog.Printf("[DEBUG] "+msg, args...)
		return
	}

	var record logapi.Record
	record.SetSeverity(logapi.SeverityDebug)
	if len(args) > 0 {
		record.SetBody(logapi.StringValue(fmt.Sprintf(msg, args...)))
	} else {
		record.SetBody(logapi.StringValue(msg))
	}
	defaultLogger.logger.Emit(defaultLogger.ctx, record)
}

// Warn logs a warning message
func Warn(msg string, args ...interface{}) {
	if defaultLogger == nil {
		stdlog.Printf("[WARN] "+msg, args...)
		return
	}

	var record logapi.Record
	record.SetSeverity(logapi.SeverityWarn)
	if len(args) > 0 {
		record.SetBody(logapi.StringValue(fmt.Sprintf(msg, args...)))
	} else {
		record.SetBody(logapi.StringValue(msg))
	}
	defaultLogger.logger.Emit(defaultLogger.ctx, record)
}

// Error logs an error message
func Error(msg string, args ...interface{}) {
	if defaultLogger == nil {
		stdlog.Printf("[ERROR] "+msg, args...)
		return
	}

	var record logapi.Record
	record.SetSeverity(logapi.SeverityError)
	if len(args) > 0 {
		record.SetBody(logapi.StringValue(fmt.Sprintf(msg, args...)))
	} else {
		record.SetBody(logapi.StringValue(msg))
	}
	defaultLogger.logger.Emit(defaultLogger.ctx, record)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, args ...interface{}) {
	Error(msg, args...)
	if defaultLogger != nil {
		_ = Shutdown()
	}
	os.Exit(1)
}

package observability

import (
	"io"
	"log/slog"
	"os"
)

// ConfigureLogging installs JSON logging on stdout and redirects the standard
// library logger through the same handler.
func ConfigureLogging(serviceName string) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service.name", serviceName)
	slog.SetDefault(logger)
}

func newJSONLogger(output io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(output, nil))
}

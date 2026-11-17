package observability

import (
	"bytes"
	"encoding/json"
	"log"
	"log/slog"
	"testing"
)

func TestStandardLoggerUsesJSONHandler(t *testing.T) {
	previous := slog.Default()
	var output bytes.Buffer
	slog.SetDefault(newJSONLogger(&output))
	t.Cleanup(func() {
		slog.SetDefault(previous)
	})

	log.Print("legacy log message")

	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &record); err != nil {
		t.Fatalf("expected standard logger output to be JSON: %v", err)
	}
	if record["msg"] != "legacy log message" {
		t.Fatalf("unexpected log message: %v", record["msg"])
	}
	if record["level"] != "INFO" {
		t.Fatalf("unexpected log level: %v", record["level"])
	}
}

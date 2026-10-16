package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestLoggerUsesStructuredSchema(t *testing.T) {
	var output bytes.Buffer
	newLogger(&output, "test-version").Info("test message", "request_id", "req-1")

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("decode log entry: %v", err)
	}

	want := map[string]string{
		"level":           "info",
		"message":         "test message",
		"service.name":    "timepilot",
		"service.version": "test-version",
		"request_id":      "req-1",
	}
	for key, value := range want {
		if got := entry[key]; got != value {
			t.Errorf("%s = %v, want %q", key, got, value)
		}
	}
	if _, ok := entry["timestamp"]; !ok {
		t.Error("timestamp is missing")
	}
	if _, ok := entry["time"]; ok {
		t.Error("unexpected default time field")
	}
	if _, ok := entry["msg"]; ok {
		t.Error("unexpected default msg field")
	}
}

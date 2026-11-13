package httpapi

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"timepilot/internal/planner"
)

func TestHTTPMetricsUseRouteTemplatesAndCountErrors(t *testing.T) {
	m := NewMetrics()
	m.EnableDemo()
	h := New(newFakeStore(), planner.Disabled(), slog.New(slog.NewTextHandler(io.Discard, nil)), "test", m)
	perform(t, h, "POST", "/api/v1/tasks", `{"title":"Demo","status":"todo"}`)
	perform(t, h, "GET", "/api/v1/tasks/1", "")
	perform(t, h, "GET", "/api/v1/tasks/999", "")
	perform(t, m.Handler(), "POST", "/demo/mode", `{"mode":"down"}`)
	perform(t, h, "POST", "/api/v1/tasks", `{"title":"Demo","status":"todo"}`)
	perform(t, h, "GET", "/healthz", "")
	body := perform(t, m.Handler(), "GET", "/metrics", "").Body.String()
	for _, sample := range []string{
		`timepilot_http_requests_total{method="POST",route="/api/v1/tasks",status="201"} 1`,
		`timepilot_http_requests_total{method="POST",route="/api/v1/tasks",status="503"} 1`,
		`timepilot_http_requests_total{method="GET",route="/api/v1/tasks/{id}",status="200"} 1`,
		`timepilot_http_requests_total{method="GET",route="/api/v1/tasks/{id}",status="404"} 1`,
		`timepilot_http_request_duration_seconds_count{method="GET",route="/api/v1/tasks/{id}"} 2`,
		`timepilot_http_in_flight_requests 0`,
	} {
		if !strings.Contains(body, sample) {
			t.Errorf("missing %q", sample)
		}
	}
	if strings.Contains(body, `route="/api/v1/tasks/999"`) {
		t.Fatal("raw IDs in metric labels")
	}
}

func TestHTTPMetricsCountRecoveredPanic(t *testing.T) {
	m := NewMetrics()
	h := New(brokenCreateStore{Store: newFakeStore(), panicOnCreate: true}, planner.Disabled(), slog.New(slog.NewTextHandler(io.Discard, nil)), "test", m)
	perform(t, h, "POST", "/api/v1/tasks", `{"title":"Demo","status":"todo"}`)
	body := perform(t, m.Handler(), "GET", "/metrics", "").Body.String()
	if !strings.Contains(body, `timepilot_http_requests_total{method="POST",route="/api/v1/tasks",status="500"} 1`) {
		t.Fatal(body)
	}
}

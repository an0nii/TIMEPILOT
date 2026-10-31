package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"timepilot/internal/planner"
)

func TestBusinessMetricsCountOnlySuccessfulOperations(t *testing.T) {
	metrics := NewMetrics()
	handler := New(newFakeStore(), planner.Disabled(), slog.New(slog.NewTextHandler(io.Discard, nil)), "test", metrics)
	steps := []struct {
		method, path, body string
		status             int
	}{
		{http.MethodPost, "/api/v1/tasks", `{"title":"Demo","status":"todo"}`, 201},
		{http.MethodGet, "/api/v1/tasks/1", "", 200},
		{http.MethodPut, "/api/v1/tasks/1", `{"title":"Demo","status":"done"}`, 200},
		{http.MethodDelete, "/api/v1/tasks/1", "", 204},
		{http.MethodPost, "/api/v1/tasks", `{"title":"","status":"todo"}`, 400},
		{http.MethodGet, "/api/v1/tasks/1", "", 404},
		{http.MethodPut, "/api/v1/tasks/1", `{"title":"Missing","status":"done"}`, 404},
		{http.MethodDelete, "/api/v1/tasks/1", "", 404},
		{http.MethodGet, "/api/v1/tasks", "", 200},
		{http.MethodGet, "/healthz", "", 200},
	}
	for _, step := range steps {
		if response := perform(t, handler, step.method, step.path, step.body); response.Code != step.status {
			t.Fatalf("%s %s: got %d, want %d", step.method, step.path, response.Code, step.status)
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	request.Header.Set("Accept", "application/openmetrics-text; version=1.0.0")
	response := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(response, request)
	if response.Code != 200 || !strings.Contains(response.Header().Get("Content-Type"), "application/openmetrics-text") {
		t.Fatalf("unexpected metrics response: %d %s", response.Code, response.Header())
	}
	for _, sample := range []string{
		"timepilot_tasks_created_total{sli_version=\"1\"} 1.0\n", "timepilot_tasks_viewed_total{sli_version=\"1\"} 1.0\n",
		"timepilot_tasks_updated_total{sli_version=\"1\"} 1.0\n", "timepilot_tasks_deleted_total 1.0\n",
		"timepilot_task_attempts_total{operation=\"create\",sli_version=\"1\"} 1.0\n",
		"timepilot_task_attempts_total{operation=\"view\",sli_version=\"1\"} 1.0\n",
		"timepilot_task_attempts_total{operation=\"update\",sli_version=\"1\"} 1.0\n",
		"go_goroutines ", "go_memstats_alloc_bytes ", "# EOF\n",
	} {
		if !strings.Contains(response.Body.String(), sample) {
			t.Errorf("metrics response missing %q", sample)
		}
	}
	if response := perform(t, handler, http.MethodGet, "/metrics", ""); response.Code != http.StatusNotFound {
		t.Fatalf("public HTTP port exposes metrics: status %d", response.Code)
	}
}

package httpapi

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"timepilot/internal/planner"
	"timepilot/internal/prober"
	"timepilot/internal/task"
)

func TestDemoIsDisabledByDefault(t *testing.T) {
	m := NewMetrics()
	if response := perform(t, m.Handler(), "POST", "/demo/mode", `{"mode":"down"}`); response.Code != 404 {
		t.Fatalf("demo control enabled by default: %d", response.Code)
	}
}

func TestSLIFailuresAndRecovery(t *testing.T) {
	m := NewMetrics()
	m.EnableDemo()
	handler := New(newFakeStore(), planner.Disabled(), slog.New(slog.NewTextHandler(io.Discard, nil)), "test", m)
	if r := perform(t, handler, "POST", "/api/v1/tasks", `{"title":"Canary","status":"todo"}`); r.Code != 201 {
		t.Fatal(r.Code)
	}
	if r := perform(t, m.Handler(), "POST", "/demo/mode", `{"mode":"down"}`); r.Code != 200 {
		t.Fatal(r.Code)
	}
	for _, method := range []string{"POST", "GET", "PUT"} {
		path := "/api/v1/tasks/1"
		if method == "POST" {
			path = "/api/v1/tasks"
		}
		if r := perform(t, handler, method, path, `{"title":"Canary","status":"todo"}`); r.Code != 503 {
			t.Fatalf("%s: %d", method, r.Code)
		}
	}
	if r := perform(t, handler, "GET", "/healthz", ""); r.Code != 200 {
		t.Fatal("faults affected health check")
	}
	if r := perform(t, m.Handler(), "POST", "/demo/mode", `{"mode":"healthy"}`); r.Code != 200 {
		t.Fatal(r.Code)
	}
	if r := perform(t, handler, "GET", "/api/v1/tasks/1", ""); r.Code != 200 {
		t.Fatal(r.Code)
	}
	if r := perform(t, handler, "PUT", "/api/v1/tasks/1", `{"title":"Canary","status":"done"}`); r.Code != 200 {
		t.Fatal(r.Code)
	}
	body := perform(t, m.Handler(), "GET", "/metrics", "").Body.String()
	for _, sample := range []string{
		`timepilot_task_attempts_total{operation="create",sli_version="1"} 2`,
		`timepilot_task_attempts_total{operation="view",sli_version="1"} 2`,
		`timepilot_task_attempts_total{operation="update",sli_version="1"} 2`,
		"timepilot_tasks_created_total{sli_version=\"1\"} 1\n", "timepilot_tasks_viewed_total{sli_version=\"1\"} 1\n", "timepilot_tasks_updated_total{sli_version=\"1\"} 1\n",
	} {
		if !strings.Contains(body, sample) {
			t.Errorf("missing %q", sample)
		}
	}
}

func TestDegradedModeFailsHalfTheOperations(t *testing.T) {
	m := NewMetrics()
	m.EnableDemo()
	perform(t, m.Handler(), "POST", "/demo/mode", `{"mode":"degraded"}`)
	h := New(newFakeStore(), planner.Disabled(), slog.New(slog.NewTextHandler(io.Discard, nil)), "test", m)
	counts := map[int]int{}
	for i := 0; i < 12; i++ {
		counts[perform(t, h, "POST", "/api/v1/tasks", `{"title":"Demo","status":"todo"}`).Code]++
	}
	if counts[201] != 6 || counts[503] != 6 {
		t.Fatalf("unexpected results: %v", counts)
	}
}

type brokenCreateStore struct {
	task.Store
	panicOnCreate bool
}

func (s brokenCreateStore) Create(context.Context, task.Input) (task.Task, error) {
	if s.panicOnCreate {
		panic("test panic")
	}
	return task.Task{}, fmt.Errorf("database unavailable")
}

func TestSLIIncludesDatabaseErrorsAndPanics(t *testing.T) {
	for _, panicOnCreate := range []bool{false, true} {
		m := NewMetrics()
		h := New(brokenCreateStore{Store: newFakeStore(), panicOnCreate: panicOnCreate}, planner.Disabled(), slog.New(slog.NewTextHandler(io.Discard, nil)), "test", m)
		if r := perform(t, h, "POST", "/api/v1/tasks", `{"title":"Demo","status":"todo"}`); r.Code != 500 {
			t.Fatal(r.Code)
		}
		body := perform(t, m.Handler(), "GET", "/metrics", "").Body.String()
		if !strings.Contains(body, `timepilot_task_attempts_total{operation="create",sli_version="1"} 1`) || !strings.Contains(body, "timepilot_tasks_created_total{sli_version=\"1\"} 0\n") {
			t.Fatal(body)
		}
	}
}

func TestProberExercisesThreeOperationsAndKeepsOnlyItsAnchor(t *testing.T) {
	store := newFakeStore()
	server := httptest.NewServer(testHandler(store))
	defer server.Close()
	p := prober.New(prober.Config{TargetURL: server.URL, Timeout: time.Second})
	p.Cycle(context.Background())
	p.Cycle(context.Background())
	for _, operation := range []string{"create", "view", "update"} {
		body := perform(t, p.Handler(), "GET", "/metrics", "").Body.String()
		sample := fmt.Sprintf(`timepilot_probe_requests_total{operation="%s",result="success"} 2`, operation)
		if !strings.Contains(body, sample) {
			t.Fatalf("missing %s", sample)
		}
	}
	if len(store.items) != 1 {
		t.Fatalf("prober left %d tasks, want one owned anchor", len(store.items))
	}
}

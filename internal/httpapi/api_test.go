package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"timepilot/internal/planner"
	"timepilot/internal/task"
)

type fakeStore struct {
	items  map[int64]task.Task
	nextID int64
}

func newFakeStore() *fakeStore {
	return &fakeStore{items: make(map[int64]task.Task), nextID: 1}
}

func (s *fakeStore) Ping(context.Context) error { return nil }

func (s *fakeStore) List(context.Context) ([]task.Task, error) {
	result := make([]task.Task, 0, len(s.items))
	for _, item := range s.items {
		result = append(result, item)
	}
	return result, nil
}

func (s *fakeStore) Get(_ context.Context, id int64) (task.Task, error) {
	item, ok := s.items[id]
	if !ok {
		return task.Task{}, task.ErrNotFound
	}
	return item, nil
}

func (s *fakeStore) Create(_ context.Context, input task.Input) (task.Task, error) {
	now := time.Now().UTC()
	item := task.Task{ID: s.nextID, Title: input.Title, Description: input.Description, Status: input.Status, CreatedAt: now, UpdatedAt: now}
	s.items[item.ID] = item
	s.nextID++
	return item, nil
}

func (s *fakeStore) Update(_ context.Context, id int64, input task.Input) (task.Task, error) {
	item, ok := s.items[id]
	if !ok {
		return task.Task{}, task.ErrNotFound
	}
	item.Title, item.Description, item.Status = input.Title, input.Description, input.Status
	s.items[id] = item
	return item, nil
}

func (s *fakeStore) Delete(_ context.Context, id int64) error {
	if _, ok := s.items[id]; !ok {
		return task.ErrNotFound
	}
	delete(s.items, id)
	return nil
}

func testHandler(store task.Store) http.Handler {
	return New(store, planner.Disabled(), slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
}

func TestTaskCRUD(t *testing.T) {
	handler := testHandler(newFakeStore())

	created := perform(t, handler, http.MethodPost, "/api/v1/tasks", `{"title":"Plan the day","description":"Run service","status":"todo"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", created.Code, created.Body.String())
	}
	var item task.Task
	if err := json.NewDecoder(created.Body).Decode(&item); err != nil {
		t.Fatal(err)
	}

	updated := perform(t, handler, http.MethodPut, "/api/v1/tasks/1", `{"title":"Plan the day","description":"Done","status":"done"}`)
	if updated.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updated.Code, updated.Body.String())
	}

	got := perform(t, handler, http.MethodGet, "/api/v1/tasks/1", "")
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"status":"done"`) {
		t.Fatalf("get response = %d %s", got.Code, got.Body.String())
	}

	deleted := perform(t, handler, http.MethodDelete, "/api/v1/tasks/1", "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", deleted.Code)
	}

	missing := perform(t, handler, http.MethodGet, "/api/v1/tasks/1", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d", missing.Code)
	}
}

func TestRejectsInvalidTask(t *testing.T) {
	response := perform(t, testHandler(newFakeStore()), http.MethodPost, "/api/v1/tasks", `{"title":"","status":"unknown"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestHealthEndpoints(t *testing.T) {
	handler := testHandler(newFakeStore())
	for _, path := range []string{"/healthz", "/readyz"} {
		response := perform(t, handler, http.MethodGet, path, "")
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, response.Code)
		}
	}
}

func TestAIStatusAndDisabledPlan(t *testing.T) {
	handler := testHandler(newFakeStore())
	status := perform(t, handler, http.MethodGet, "/api/v1/ai/status", "")
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"enabled":false`) {
		t.Fatalf("AI status = %d %s", status.Code, status.Body.String())
	}
	plan := perform(t, handler, http.MethodPost, "/api/v1/ai/plan", `{"goal":"Finish tasks","available_minutes":60}`)
	if plan.Code != http.StatusServiceUnavailable {
		t.Fatalf("AI plan status = %d, want %d", plan.Code, http.StatusServiceUnavailable)
	}
}

func perform(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

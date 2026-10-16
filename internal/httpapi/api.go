package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"timepilot/internal/planner"
	"timepilot/internal/task"
)

const maxBodySize = 1 << 20

type API struct {
	store   task.Store
	logger  *slog.Logger
	version string
	planner planner.Planner
}

func New(store task.Store, aiPlanner planner.Planner, logger *slog.Logger, version string) http.Handler {
	api := &API{store: store, planner: aiPlanner, logger: logger, version: version}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", api.health)
	mux.HandleFunc("GET /readyz", api.ready)
	mux.HandleFunc("GET /api/v1/tasks", api.list)
	mux.HandleFunc("POST /api/v1/tasks", api.create)
	mux.HandleFunc("GET /api/v1/tasks/{id}", api.get)
	mux.HandleFunc("PUT /api/v1/tasks/{id}", api.update)
	mux.HandleFunc("DELETE /api/v1/tasks/{id}", api.delete)
	mux.HandleFunc("GET /api/v1/ai/status", api.aiStatus)
	mux.HandleFunc("POST /api/v1/ai/plan", api.createPlan)

	assets, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(fmt.Sprintf("load embedded frontend: %v", err))
	}
	mux.Handle("GET /", http.FileServer(http.FS(assets)))

	return api.recoverPanic(api.accessLog(mux))
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": a.version})
}

func (a *API) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := a.store.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *API) list(w http.ResponseWriter, r *http.Request) {
	items, err := a.store.List(r.Context())
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *API) get(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	item, err := a.store.Get(r.Context(), id)
	if err != nil {
		a.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) create(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeInput(w, r)
	if !ok {
		return
	}
	item, err := a.store.Create(r.Context(), input)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (a *API) update(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	input, ok := decodeInput(w, r)
	if !ok {
		return
	}
	item, err := a.store.Update(r.Context(), id, input)
	if err != nil {
		a.storeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	if err := a.store.Delete(r.Context(), id); err != nil {
		a.storeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) storeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, task.ErrNotFound) {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	a.internalError(w, r, err)
}

func (a *API) internalError(w http.ResponseWriter, r *http.Request, err error) {
	a.logger.Error("request failed",
		"http.method", r.Method,
		"url.path", r.URL.Path,
		"error", err,
	)
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func decodeInput(w http.ResponseWriter, r *http.Request) (task.Input, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input task.Input
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return task.Input{}, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "body must contain one JSON object")
		return task.Input{}, false
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	if !input.Valid() {
		writeError(w, http.StatusBadRequest, "title must be 1-200 characters; status must be todo, in_progress or done")
		return task.Input{}, false
	}
	return input, true
}

func taskID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return 0, false
	}
	return id, true
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSONStatus(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	writeJSONStatus(w, status, value)
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type statusWriter struct {
	http.ResponseWriter
	status    int
	bodyBytes int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	written, err := w.ResponseWriter.Write(body)
	w.bodyBytes += written
	return written, err
}

func (a *API) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)
		a.logger.Info("request",
			"http.method", r.Method,
			"url.path", r.URL.Path,
			"http.status_code", wrapped.status,
			"http.response.body.size", wrapped.bodyBytes,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
}

func (a *API) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				a.logger.Error("panic recovered", "error", recovered)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

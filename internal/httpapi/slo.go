package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"sync/atomic"
)

type demoControl struct {
	enabled bool
	mode    atomic.Uint32
	serial  atomic.Uint64
}

func (d *demoControl) shouldFail() bool {
	if !d.enabled {
		return false
	}
	switch d.mode.Load() {
	case 1:
		return d.serial.Add(1)%2 == 0
	case 2:
		return true
	default:
		return false
	}
}

func (d *demoControl) setMode(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Mode string `json:"mode"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "expected a JSON object with mode")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "expected one JSON object")
		return
	}
	var mode uint32
	switch input.Mode {
	case "healthy":
		mode = 0
	case "degraded":
		mode = 1
	case "down":
		mode = 2
	default:
		writeError(w, http.StatusBadRequest, "mode must be healthy, degraded or down")
		return
	}
	d.mode.Store(mode)
	writeJSON(w, http.StatusOK, map[string]string{"mode": input.Mode})
}

// taskOperation measures completed business operations. Invalid client requests
// are excluded. Transport failures before the server are measured by the prober.
func (a *API) taskOperation(operation string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		if a.metrics.demo.shouldFail() {
			writeError(response, http.StatusServiceUnavailable, "SLO demo: injected failure")
		} else {
			a.recoverPanic(next).ServeHTTP(response, r)
		}
		if (response.status >= 200 && response.status < 300) || response.status >= 500 {
			a.metrics.attempts.WithLabelValues(operation).Inc()
		}
	}
}

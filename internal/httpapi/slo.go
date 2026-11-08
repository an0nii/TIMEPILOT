package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
)

type demoControl struct {
	enabled     bool
	mode        atomic.Uint32
	serial      atomic.Uint64
	runtimeMu   sync.Mutex
	runtimeStop chan struct{}
}

// runtimePressure is bounded and only exposed when SLO_DEMO_ENABLED=true.
// Repeated enable calls do not create more goroutines. Healthy mode releases them.
func (d *demoControl) runtimePressure(enabled bool) {
	d.runtimeMu.Lock()
	defer d.runtimeMu.Unlock()
	if d.runtimeStop != nil && !enabled {
		close(d.runtimeStop)
		d.runtimeStop = nil
	}
	if enabled && d.runtimeStop == nil {
		d.runtimeStop = make(chan struct{})
		for i := 0; i < 128; i++ {
			go func(stop <-chan struct{}) { <-stop }(d.runtimeStop)
		}
	}
}

func (d *demoControl) setRuntime(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Enabled *bool `json:"enabled"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || input.Enabled == nil {
		writeError(w, http.StatusBadRequest, "expected enabled: true or false")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "expected one JSON object")
		return
	}
	d.runtimePressure(*input.Enabled)
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": *input.Enabled})
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
	if mode == 0 {
		d.runtimePressure(false)
	}
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

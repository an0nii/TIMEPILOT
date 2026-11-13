package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (a *API) observeHTTP(next http.Handler, mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		a.metrics.inFlight.Inc()
		defer a.metrics.inFlight.Dec()
		response := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		// Handler returns the registered pattern on Go 1.22 as well. Never
		// label by the raw URL or task ID: that creates unbounded cardinality.
		_, route := mux.Handler(r)
		next.ServeHTTP(response, r)
		if _, path, ok := strings.Cut(route, " "); ok {
			route = path
		}
		if route == "" {
			route = "unmatched"
		}
		method := r.Method
		switch method {
		case "GET", "HEAD", "POST", "PUT", "DELETE", "PATCH", "OPTIONS":
		default:
			method = "OTHER"
		}
		a.metrics.httpRequests.WithLabelValues(method, route, strconv.Itoa(response.status)).Inc()
		a.metrics.httpDuration.WithLabelValues(method, route).Observe(time.Since(started).Seconds())
	})
}

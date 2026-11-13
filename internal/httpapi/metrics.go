package httpapi

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics belongs to one application process; counters reset on process restart.
type Metrics struct {
	registry     *prometheus.Registry
	created      prometheus.Counter
	viewed       prometheus.Counter
	updated      prometheus.Counter
	deleted      prometheus.Counter
	attempts     *prometheus.CounterVec
	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
	inFlight     prometheus.Gauge
	demo         demoControl
}

func NewMetrics() *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "timepilot_http_requests_total", Help: "Completed HTTP requests by bounded method, route template and status code.",
		}, []string{"method", "route", "status"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "timepilot_http_request_duration_seconds", Help: "HTTP handler duration in seconds, including database work.",
			Buckets: []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		}, []string{"method", "route"}),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{Name: "timepilot_http_in_flight_requests", Help: "HTTP requests currently executing; health checks included, metrics server excluded."}),
		attempts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "timepilot_task_attempts_total", Help: "Completed task operations with a 2xx or 5xx response; 4xx excluded.",
			ConstLabels: prometheus.Labels{"sli_version": "1"},
		}, []string{"operation"}),
		created: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "timepilot_tasks_created_total", Help: "Number of successfully created tasks.",
			ConstLabels: prometheus.Labels{"sli_version": "1"},
		}),
		viewed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "timepilot_tasks_viewed_total", Help: "Number of successful individual task reads.",
			ConstLabels: prometheus.Labels{"sli_version": "1"},
		}),
		updated: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "timepilot_tasks_updated_total", Help: "Number of successfully updated tasks.",
			ConstLabels: prometheus.Labels{"sli_version": "1"},
		}),
		deleted: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "timepilot_tasks_deleted_total", Help: "Number of successfully deleted tasks.",
		}),
	}
	m.registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.created, m.viewed, m.updated, m.deleted, m.attempts, m.httpRequests, m.httpDuration, m.inFlight,
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "timepilot_demo_mode", Help: "Fault injection mode: 0 healthy, 1 degraded, 2 down. Disabled by default.",
		}, func() float64 { return float64(m.demo.mode.Load()) }))
	for _, operation := range []string{"create", "view", "update"} {
		m.attempts.WithLabelValues(operation)
	}
	return m
}

func (m *Metrics) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{EnableOpenMetrics: true}))
	if m.demo.enabled {
		mux.HandleFunc("POST /demo/mode", m.demo.setMode)
		mux.HandleFunc("POST /demo/runtime", m.demo.setRuntime)
	}
	return mux
}

// EnableDemo must be called at startup, before the HTTP handlers are served.
func (m *Metrics) EnableDemo() { m.demo.enabled = true }

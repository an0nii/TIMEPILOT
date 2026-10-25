package httpapi

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics belongs to one application process; counters reset on process restart.
type Metrics struct {
	registry *prometheus.Registry
	created  prometheus.Counter
	viewed   prometheus.Counter
	updated  prometheus.Counter
	deleted  prometheus.Counter
}

func NewMetrics() *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		created: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "timepilot_tasks_created_total", Help: "Number of successfully created tasks.",
		}),
		viewed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "timepilot_tasks_viewed_total", Help: "Number of successful individual task reads.",
		}),
		updated: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "timepilot_tasks_updated_total", Help: "Number of successfully updated tasks.",
		}),
		deleted: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "timepilot_tasks_deleted_total", Help: "Number of successfully deleted tasks.",
		}),
	}
	m.registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.created, m.viewed, m.updated, m.deleted)
	return m
}

func (m *Metrics) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{EnableOpenMetrics: true}))
	return mux
}

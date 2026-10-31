// Package prober exercises the real task API independently of the web process.
package prober

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"timepilot/internal/task"
)

const canaryTitle = "[slo-probe] canary"

type Config struct {
	TargetURL string
	Addr      string
	Interval  time.Duration
	Timeout   time.Duration
}

func LoadConfig() (Config, error) {
	cfg := Config{TargetURL: strings.TrimRight(os.Getenv("PROBE_TARGET_URL"), "/"), Addr: os.Getenv("PROBE_ADDR"), Interval: 5 * time.Second, Timeout: 2 * time.Second}
	if cfg.TargetURL == "" {
		cfg.TargetURL = "http://127.0.0.1:8080"
	}
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:9091"
	}
	u, err := url.Parse(cfg.TargetURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Config{}, fmt.Errorf("PROBE_TARGET_URL must be an absolute HTTP(S) URL")
	}
	for name, value := range map[string]*time.Duration{"PROBE_INTERVAL": &cfg.Interval, "PROBE_TIMEOUT": &cfg.Timeout} {
		if raw := os.Getenv(name); raw != "" {
			parsed, err := time.ParseDuration(raw)
			if err != nil || parsed <= 0 {
				return Config{}, fmt.Errorf("%s must be a positive duration", name)
			}
			*value = parsed
		}
	}
	return cfg, nil
}

type Prober struct {
	cfg      Config
	client   *http.Client
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	success  *prometheus.GaugeVec
	anchorID int64
}

func New(cfg Config) *Prober {
	p := &Prober{
		cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}, registry: prometheus.NewRegistry(),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "timepilot_probe_requests_total", Help: "Synthetic task operations, including HTTP, transport, timeout and response validation failures.",
		}, []string{"operation", "result"}),
		success: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "timepilot_probe_success", Help: "Last synthetic operation result: 1 success, 0 failure.",
		}, []string{"operation"}),
	}
	for _, operation := range []string{"create", "view", "update"} {
		p.requests.WithLabelValues(operation, "success")
		p.requests.WithLabelValues(operation, "failure")
		p.success.WithLabelValues(operation).Set(0)
	}
	p.registry.MustRegister(p.requests, p.success)
	return p
}

func (p *Prober) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(p.registry, promhttp.HandlerOpts{EnableOpenMetrics: true}))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	return mux
}

func (p *Prober) record(operation string, good bool) {
	result, value := "failure", 0.0
	if good {
		result, value = "success", 1
	}
	p.requests.WithLabelValues(operation, result).Inc()
	p.success.WithLabelValues(operation).Set(value)
}

func (p *Prober) operation(ctx context.Context, operation, method, path string, input *task.Input, wantStatus int, wantID int64) (task.Task, bool) {
	var body io.Reader
	if input != nil {
		data, _ := json.Marshal(input)
		body = bytes.NewReader(data)
	}
	r, err := http.NewRequestWithContext(ctx, method, p.cfg.TargetURL+path, body)
	var item task.Task
	if err == nil {
		r.Header.Set("Content-Type", "application/json")
		response, requestErr := p.client.Do(r)
		if requestErr != nil {
			err = requestErr
		} else {
			defer response.Body.Close()
			if response.StatusCode != wantStatus {
				err = fmt.Errorf("unexpected status %d", response.StatusCode)
			} else {
				err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&item)
				if err == nil && (item.ID <= 0 || item.Title != canaryTitle || (wantID > 0 && item.ID != wantID) || (input != nil && item.Status != input.Status)) {
					err = fmt.Errorf("unexpected task response")
				}
			}
		}
	}
	p.record(operation, err == nil)
	return item, err == nil
}

func (p *Prober) deleteOwnTask(ctx context.Context, id int64) {
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, p.cfg.TargetURL+"/api/v1/tasks/"+strconv.FormatInt(id, 10), nil)
	if err != nil {
		return
	}
	if response, err := p.client.Do(request); err == nil {
		response.Body.Close()
	}
}

// Cycle keeps one owned anchor for independent read/update probes. New tasks from
// subsequent create probes are removed immediately; no user task is selected.
func (p *Prober) Cycle(ctx context.Context) {
	input := task.Input{Title: canaryTitle, Description: "Owned by the SLO prober", Status: task.StatusTodo}
	created, good := p.operation(ctx, "create", http.MethodPost, "/api/v1/tasks", &input, http.StatusCreated, 0)
	if good {
		if p.anchorID == 0 {
			p.anchorID = created.ID
		} else {
			p.deleteOwnTask(ctx, created.ID)
		}
	}
	if p.anchorID == 0 {
		p.record("view", false)
		p.record("update", false)
		return
	}
	path := "/api/v1/tasks/" + strconv.FormatInt(p.anchorID, 10)
	p.operation(ctx, "view", http.MethodGet, path, nil, http.StatusOK, p.anchorID)
	input.Status = task.StatusInProgress
	p.operation(ctx, "update", http.MethodPut, path, &input, http.StatusOK, p.anchorID)
}

func Run(ctx context.Context, cfg Config, logger *slog.Logger) error {
	p := New(cfg)
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("start probe metrics: %w", err)
	}
	server := &http.Server{Handler: p.Handler(), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second}
	defer server.Close()
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	logger.Info("SLO prober started", "target", cfg.TargetURL, "interval", cfg.Interval.String())
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
		defer cancel()
		if p.anchorID > 0 {
			p.deleteOwnTask(cleanupCtx, p.anchorID)
		}
	}()
	p.Cycle(ctx)
	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return server.Shutdown(shutdownCtx)
		case err := <-errCh:
			return fmt.Errorf("probe metrics server: %w", err)
		case <-ticker.C:
			p.Cycle(ctx)
		}
	}
}

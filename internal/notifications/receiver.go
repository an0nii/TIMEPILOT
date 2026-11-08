// Package notifications provides a small, local-only Alertmanager webhook inbox.
// It is a teaching receiver, not a durable production notification service.
package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type Alert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

type Notification struct {
	Received time.Time `json:"received"`
	Status   string    `json:"status"`
	Receiver string    `json:"receiver"`
	Alerts   []Alert   `json:"alerts"`
}

type Receiver struct {
	mu     sync.Mutex
	items  []Notification
	logger *slog.Logger
}

func New(logger *slog.Logger) *Receiver { return &Receiver{logger: logger} }

func (r *Receiver) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("POST /webhook", r.receive)
	mux.HandleFunc("GET /api/notifications", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(r.snapshot())
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = page.Execute(w, r.snapshot())
	})
	return mux
}

func (r *Receiver) snapshot() []Notification {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Notification{}, r.items...)
}

func (r *Receiver) receive(w http.ResponseWriter, req *http.Request) {
	var n Notification
	d := json.NewDecoder(http.MaxBytesReader(w, req.Body, 64<<10))
	if err := d.Decode(&n); err != nil || len(n.Alerts) == 0 || (n.Status != "firing" && n.Status != "resolved") {
		http.Error(w, "expected an Alertmanager webhook", http.StatusBadRequest)
		return
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		http.Error(w, "expected one JSON object", http.StatusBadRequest)
		return
	}
	n.Received = time.Now().UTC()
	r.mu.Lock()
	r.items = append([]Notification{n}, r.items...)
	if len(r.items) > 100 {
		r.items = r.items[:100]
	}
	r.mu.Unlock()
	for _, a := range n.Alerts {
		r.logger.Info("alert notification received", "status", a.Status, "alertname", a.Labels["alertname"])
	}
	w.WriteHeader(http.StatusNoContent)
}

func Run(ctx context.Context, addr string, logger *slog.Logger) error {
	server := &http.Server{Addr: addr, Handler: New(logger).Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	defer server.Close()
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	logger.Info("notification inbox started", "address", addr)
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}

var page = template.Must(template.New("inbox").Parse(`<!doctype html>
<html lang="ru"><meta charset="utf-8"><meta http-equiv="refresh" content="5">
<title>TimePilot — уведомления</title><style>
body{background:#0b1220;color:#e2e8f0;font:17px system-ui;max-width:1100px;margin:32px auto;padding:0 20px}
article{background:#152238;border:1px solid #334155;border-radius:12px;padding:20px;margin:18px 0}
.firing{color:#fda4af}.resolved{color:#86efac}h2{margin:8px 0}p{line-height:1.5}code{color:#93c5fd}
</style><h1>TimePilot · Уведомления Alertmanager</h1>
<p>Настоящие webhook-уведомления из minikube. Обновление каждые 5 секунд. Последние 100 сообщений; память очищается при перезапуске.</p>
{{range .}}<article><strong class="{{.Status}}">{{.Status}}</strong> · {{.Received.Format "2006-01-02 15:04:05 UTC"}} · receiver: {{.Receiver}}
{{range .Alerts}}<h2 class="{{.Status}}">{{.Status}} · {{index .Labels "alertname"}}</h2>
<p>{{index .Annotations "summary"}}</p><p>{{index .Annotations "description"}}</p>
<code>service={{index .Labels "service"}} · severity={{index .Labels "severity"}} · category={{index .Labels "category"}} {{with index .Labels "pod"}}· pod={{.}}{{end}}</code>{{end}}</article>
{{else}}<p>Пока нет уведомлений. Запустите демонстрацию и дождитесь firing.</p>{{end}}</html>`))

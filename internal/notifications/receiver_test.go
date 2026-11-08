package notifications

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhookAndEscapedInbox(t *testing.T) {
	r := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	h := r.Handler()
	for _, status := range []string{"firing", "resolved"} {
		body := `{"status":"` + status + `","receiver":"inbox","alerts":[{"status":"` + status + `","labels":{"alertname":"Test"},"annotations":{"summary":"<script>alert(1)</script>"}}]}`
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", "/webhook", strings.NewReader(body)))
		if w.Code != 204 {
			t.Fatalf("webhook: %d", w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if strings.Contains(w.Body.String(), "<script>") || !strings.Contains(w.Body.String(), "resolved") || !strings.Contains(w.Body.String(), "firing") {
		t.Fatal(w.Body.String())
	}
	if len(r.snapshot()) != 2 || r.snapshot()[0].Status != "resolved" {
		t.Fatal("missing notifications")
	}
	for _, body := range []string{`{}`, `{"status":"firing","alerts":[{}]} {}`, strings.Repeat("x", 65537)} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", "/webhook", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatal(w.Code)
		}
	}
	for i := 0; i < 105; i++ {
		r.receive(httptest.NewRecorder(), httptest.NewRequest("POST", "/webhook", strings.NewReader(`{"status":"firing","alerts":[{}]}`)))
	}
	if len(r.snapshot()) != 100 {
		t.Fatal("unbounded inbox")
	}
}

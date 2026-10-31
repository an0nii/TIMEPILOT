package prober

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProberCountsHTTPTimeoutTransportAndInvalidResponses(t *testing.T) {
	for _, scenario := range []string{"http", "timeout", "transport", "invalid"} {
		t.Run(scenario, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch scenario {
				case "timeout":
					select {
					case <-r.Context().Done():
					case <-time.After(150 * time.Millisecond):
					}
				case "invalid":
					w.WriteHeader(201)
					fmt.Fprint(w, `{"id":1,"title":"wrong task","status":"todo"}`)
				default:
					w.WriteHeader(503)
				}
			}))
			defer server.Close()
			if scenario == "transport" {
				server.Close()
			}
			p := New(Config{TargetURL: server.URL, Timeout: 50 * time.Millisecond})
			p.Cycle(context.Background())
			response := httptest.NewRecorder()
			p.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
			for _, operation := range []string{"create", "view", "update"} {
				if !strings.Contains(response.Body.String(), fmt.Sprintf(`timepilot_probe_requests_total{operation="%s",result="failure"} 1`, operation)) {
					t.Fatal(response.Body.String())
				}
				if !strings.Contains(response.Body.String(), fmt.Sprintf(`timepilot_probe_requests_total{operation="%s",result="success"} 0`, operation)) {
					t.Fatal(response.Body.String())
				}
			}
		})
	}
}

func TestConfigRejectsInvalidTargetAndDurations(t *testing.T) {
	for _, entry := range []struct{ name, value string }{
		{"PROBE_TARGET_URL", "file:///etc/passwd"}, {"PROBE_INTERVAL", "0s"}, {"PROBE_TIMEOUT", "-1s"},
	} {
		t.Run(entry.name, func(t *testing.T) {
			t.Setenv("PROBE_TARGET_URL", "http://localhost:8080")
			t.Setenv(entry.name, entry.value)
			if _, err := LoadConfig(); err == nil {
				t.Fatal("expected config error")
			}
		})
	}
}

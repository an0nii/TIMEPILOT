package planner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"timepilot/internal/config"
)

func TestProviders(t *testing.T) {
	tests := []struct {
		provider     string
		path         string
		response     string
		header       string
		headerPrefix string
	}{
		{provider: "openai", path: "/responses", response: `{"output":[{"content":[{"type":"output_text","text":"OpenAI plan"}]}]}`, header: "Authorization", headerPrefix: "Bearer test-key"},
		{provider: "anthropic", path: "/messages", response: `{"content":[{"type":"text","text":"Anthropic plan"}]}`, header: "x-api-key", headerPrefix: "test-key"},
		{provider: "gemini", path: "/models/test-model:generateContent", response: `{"candidates":[{"content":{"parts":[{"text":"Gemini plan"}]}}]}`, header: "x-goog-api-key", headerPrefix: "test-key"},
	}

	for _, test := range tests {
		t.Run(test.provider, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != test.path {
					t.Errorf("path = %q, want %q", r.URL.Path, test.path)
				}
				if value := r.Header.Get(test.header); !strings.HasPrefix(value, test.headerPrefix) {
					t.Errorf("%s = %q", test.header, value)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, test.response)
			}))
			defer server.Close()

			service, err := New(config.AIConfig{
				Provider: test.provider,
				APIKey:   "test-key",
				Model:    "test-model",
				BaseURL:  server.URL,
				Timeout:  time.Second,
			})
			if err != nil {
				t.Fatal(err)
			}
			plan, err := service.Plan(context.Background(), Request{Goal: "Finish", AvailableMinutes: 60})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(plan, "plan") {
				t.Fatalf("plan = %q", plan)
			}
		})
	}
}

func TestDisabled(t *testing.T) {
	service := Disabled()
	if service.Info().Enabled {
		t.Fatal("disabled planner reports enabled")
	}
	if _, err := service.Plan(context.Background(), Request{}); err != ErrDisabled {
		t.Fatalf("Plan() error = %v, want ErrDisabled", err)
	}
}

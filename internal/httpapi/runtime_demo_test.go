package httpapi

import (
	"runtime"
	"testing"
	"time"
)

func TestRuntimeDemoIsBoundedAndReleased(t *testing.T) {
	m := NewMetrics()
	if r := perform(t, m.Handler(), "POST", "/demo/runtime", `{"enabled":true}`); r.Code != 404 {
		t.Fatal(r.Code)
	}
	m.EnableDemo()
	defer m.demo.runtimePressure(false)
	baseline := runtime.NumGoroutine()
	for i := 0; i < 3; i++ {
		if r := perform(t, m.Handler(), "POST", "/demo/runtime", `{"enabled":true}`); r.Code != 200 {
			t.Fatal(r.Code)
		}
	}
	if got := runtime.NumGoroutine() - baseline; got < 128 || got > 132 {
		t.Fatalf("unbounded or missing goroutines: %d", got)
	}
	if r := perform(t, m.Handler(), "POST", "/demo/mode", `{"mode":"healthy"}`); r.Code != 200 {
		t.Fatal(r.Code)
	}
	deadline := time.Now().Add(time.Second)
	for runtime.NumGoroutine() > baseline+2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if runtime.NumGoroutine() > baseline+2 {
		t.Fatal("demo goroutines leaked")
	}
	for _, input := range []string{`{}`, `{"enabled":"true"}`, `{"enabled":true} {}`, `{"enabled":true,"unknown":1}`} {
		if r := perform(t, m.Handler(), "POST", "/demo/runtime", input); r.Code != 400 {
			t.Fatalf("%s: %d", input, r.Code)
		}
	}
}

package health_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yourusername/kaptan/agent/health"
)

func TestCheck_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ok, code, _ := health.Check(srv.URL, 5*time.Second)
	if !ok || code != 200 {
		t.Fatalf("expected ok/200, got ok=%v code=%d", ok, code)
	}
}

func TestCheck_NotOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()

	ok, code, _ := health.Check(srv.URL, 5*time.Second)
	if ok || code != 503 {
		t.Fatalf("expected not-ok/503, got ok=%v code=%d", ok, code)
	}
}

func TestCheck_EmptyURL(t *testing.T) {
	ok, code, msg := health.Check("", 5*time.Second)
	if !ok {
		t.Fatal("empty URL should return ok=true (no check)")
	}
	if code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}
	_ = msg
}

func TestCheck_Unreachable(t *testing.T) {
	ok, code, _ := health.Check("http://127.0.0.1:19999/health", 500*time.Millisecond)
	if ok {
		t.Fatal("expected not-ok for unreachable server")
	}
	if code != 0 {
		t.Fatalf("expected code 0 on connection error, got %d", code)
	}
}

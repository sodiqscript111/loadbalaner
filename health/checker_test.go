package health_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"loadbalancer/backend"
	"loadbalancer/health"
)

func TestHealthCheckerTCP(t *testing.T) {
	// Start a mock TCP listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock TCP listener: %v", err)
	}
	addr := ln.Addr().String()

	pool := backend.NewServerPool()
	b := backend.NewBackend(addr, nil)
	pool.AddBackend(b)

	cfg := health.CheckerConfig{
		Interval:         100 * time.Millisecond,
		Timeout:          500 * time.Millisecond,
		CheckType:        health.CheckTypeTCP,
		SuccessThreshold: 1,
		FailureThreshold: 1,
	}

	checker := health.NewHealthChecker(pool, cfg)
	ctx := context.Background()

	// Initial check -> listener is running, backend should be alive
	checker.CheckBackend(ctx, b)
	if !b.IsAlive() {
		t.Fatalf("expected backend to be alive while TCP listener is open")
	}

	// Close the listener
	_ = ln.Close()

	// Check again -> should mark backend down
	checker.CheckBackend(ctx, b)
	if b.IsAlive() {
		t.Fatalf("expected backend to be marked down after TCP listener closed")
	}
}

func TestHealthCheckerHTTP(t *testing.T) {
	isUp := true
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isUp {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"UP"}`))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"DOWN"}`))
		}
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	b := backend.NewBackend(u.Host, u)
	pool := backend.NewServerPool()
	pool.AddBackend(b)

	cfg := health.CheckerConfig{
		Interval:         100 * time.Millisecond,
		Timeout:          500 * time.Millisecond,
		CheckType:        health.CheckTypeHTTP,
		HTTPPath:         "/healthz",
		SuccessThreshold: 1,
		FailureThreshold: 2,
	}

	checker := health.NewHealthChecker(pool, cfg)
	ctx := context.Background()

	// 1. Initial healthy check
	checker.CheckBackend(ctx, b)
	if !b.IsAlive() {
		t.Fatalf("expected backend to be healthy")
	}

	// 2. Server fails once (FailureThreshold=2, so not yet marked down)
	isUp = false
	checker.CheckBackend(ctx, b)
	if !b.IsAlive() {
		t.Fatalf("expected backend to still be alive after only 1 failure (threshold=2)")
	}

	// 3. Second failure triggers DOWN transition
	checker.CheckBackend(ctx, b)
	if b.IsAlive() {
		t.Fatalf("expected backend to be marked DOWN after 2 consecutive failures")
	}

	// 4. Server recovers -> 1 success marks it back UP
	isUp = true
	checker.CheckBackend(ctx, b)
	if !b.IsAlive() {
		t.Fatalf("expected backend to be marked back UP after success")
	}
}

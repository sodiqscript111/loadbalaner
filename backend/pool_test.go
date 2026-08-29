package backend_test

import (
	"testing"

	"loadbalancer/backend"
)

func TestBackendState(t *testing.T) {
	b := backend.NewBackend("127.0.0.1:8081", nil)

	if !b.IsAlive() {
		t.Fatalf("expected backend to be alive initially")
	}

	b.SetAlive(false)
	if b.IsAlive() {
		t.Fatalf("expected backend to be marked down")
	}

	b.IncConns()
	b.IncConns()
	if b.GetActiveConns() != 2 {
		t.Fatalf("expected active conns 2, got %d", b.GetActiveConns())
	}

	b.DecConns()
	if b.GetActiveConns() != 1 {
		t.Fatalf("expected active conns 1, got %d", b.GetActiveConns())
	}
}

func TestServerPool(t *testing.T) {
	pool := backend.NewServerPool()

	b1 := backend.NewBackend("127.0.0.1:8081", nil)
	b2 := backend.NewBackend("127.0.0.1:8082", nil)
	b3 := backend.NewBackend("127.0.0.1:8083", nil)

	pool.AddBackend(b1)
	pool.AddBackend(b2)
	pool.AddBackend(b3)

	if pool.GetBackendCount() != 3 {
		t.Fatalf("expected 3 backends, got %d", pool.GetBackendCount())
	}

	if pool.GetAliveCount() != 3 {
		t.Fatalf("expected 3 alive backends, got %d", pool.GetAliveCount())
	}

	b2.SetAlive(false)

	if pool.GetAliveCount() != 2 {
		t.Fatalf("expected 2 alive backends after marking one down, got %d", pool.GetAliveCount())
	}

	aliveList := pool.GetAliveBackends()
	if len(aliveList) != 2 {
		t.Fatalf("expected 2 alive items in list, got %d", len(aliveList))
	}
	for _, b := range aliveList {
		if b.Addr == "127.0.0.1:8082" {
			t.Fatalf("found dead backend in alive list")
		}
	}
}

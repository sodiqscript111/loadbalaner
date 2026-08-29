package utils_test

import (
	"net"
	"testing"

	"loadbalancer/backend"
	"loadbalancer/utils"
)

func TestSelectHealthyBackend(t *testing.T) {
	pool := backend.NewServerPool()
	b1 := backend.NewBackend("127.0.0.1:8081", nil)
	b2 := backend.NewBackend("127.0.0.1:8082", nil)
	pool.AddBackend(b1)
	pool.AddBackend(b2)

	tuple := utils.FiveTuple{
		SrcIP:    net.ParseIP("192.168.1.10"),
		DstIP:    net.ParseIP("127.0.0.1"),
		SrcPort:  54321,
		DstPort:  8080,
		Protocol: utils.TCP,
	}

	selected := utils.SelectHealthyBackend(tuple, pool)
	if selected == nil {
		t.Fatalf("expected a backend to be selected")
	}

	// Mark b1 down -> selection must always return b2
	b1.SetAlive(false)
	selected = utils.SelectHealthyBackend(tuple, pool)
	if selected == nil || selected.Addr != "127.0.0.1:8082" {
		t.Fatalf("expected b2 to be selected when b1 is down, got %v", selected)
	}

	// Mark all down -> selection returns nil
	b2.SetAlive(false)
	selected = utils.SelectHealthyBackend(tuple, pool)
	if selected != nil {
		t.Fatalf("expected nil when all backends are down, got %v", selected)
	}
}

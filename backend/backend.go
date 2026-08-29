package backend

import (
	"net/url"
	"sync/atomic"
)

// Backend represents a target upstream server node.
type Backend struct {
	URL          *url.URL
	Addr         string // e.g. "127.0.0.1:8081"
	alive        atomic.Bool
	activeConns  atomic.Int64
	failCount    atomic.Int32
	successCount atomic.Int32
}

// NewBackend creates a new backend instance initialized as alive.
func NewBackend(addr string, rawURL *url.URL) *Backend {
	b := &Backend{
		Addr: addr,
		URL:  rawURL,
	}
	b.alive.Store(true)
	return b
}

// IsAlive returns whether the backend is currently healthy.
func (b *Backend) IsAlive() bool {
	return b.alive.Load()
}

// SetAlive sets the alive status of the backend.
func (b *Backend) SetAlive(alive bool) {
	b.alive.Store(alive)
}

// IncConns increments the active connection counter.
func (b *Backend) IncConns() {
	b.activeConns.Add(1)
}

// DecConns decrements the active connection counter.
func (b *Backend) DecConns() {
	b.activeConns.Add(-1)
}

// GetActiveConns returns current active connections.
func (b *Backend) GetActiveConns() int64 {
	return b.activeConns.Load()
}

// RecordFailure increments consecutive failure counter and resets success counter.
func (b *Backend) RecordFailure() int32 {
	b.successCount.Store(0)
	return b.failCount.Add(1)
}

// RecordSuccess increments consecutive success counter and resets failure counter.
func (b *Backend) RecordSuccess() int32 {
	b.failCount.Store(0)
	return b.successCount.Add(1)
}

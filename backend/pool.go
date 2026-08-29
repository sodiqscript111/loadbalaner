package backend

import (
	"sync"
)

// ServerPool holds a collection of backends and coordinates their access.
type ServerPool struct {
	mu       sync.RWMutex
	backends []*Backend
}

// NewServerPool creates an empty ServerPool.
func NewServerPool() *ServerPool {
	return &ServerPool{
		backends: make([]*Backend, 0),
	}
}

// AddBackend adds a backend to the pool.
func (p *ServerPool) AddBackend(b *Backend) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.backends = append(p.backends, b)
}

// GetBackends returns all backends in the pool.
func (p *ServerPool) GetBackends() []*Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()
	copied := make([]*Backend, len(p.backends))
	copy(copied, p.backends)
	return copied
}

// GetAliveBackends returns only the healthy backends.
func (p *ServerPool) GetAliveBackends() []*Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var alive []*Backend
	for _, b := range p.backends {
		if b.IsAlive() {
			alive = append(alive, b)
		}
	}
	return alive
}

// GetBackendCount returns the total number of backends.
func (p *ServerPool) GetBackendCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.backends)
}

// GetAliveCount returns the number of alive backends.
func (p *ServerPool) GetAliveCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	count := 0
	for _, b := range p.backends {
		if b.IsAlive() {
			count++
		}
	}
	return count
}

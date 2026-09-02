package backend

import (
	"runtime"
	"sync"
	"sync/atomic"
)

type ServerPool struct {
	backends     [2][]*Backend
	readCounters [2]atomic.Int32
	activeIndex  atomic.Int32
	writerMutex  sync.Mutex
}

func NewServerPool() *ServerPool {
	return &ServerPool{
		backends: [2][]*Backend{
			make([]*Backend, 0),
			make([]*Backend, 0),
		},
	}
}

func (p *ServerPool) AddBackend(b *Backend) {
	p.writerMutex.Lock()
	defer p.writerMutex.Unlock()

	active := p.activeIndex.Load()
	inactive := 1 - active

	p.backends[inactive] = make([]*Backend, len(p.backends[active]), len(p.backends[active])+1)
	copy(p.backends[inactive], p.backends[active])

	p.backends[inactive] = append(p.backends[inactive], b)

	p.activeIndex.Store(inactive)

	for p.readCounters[active].Load() > 0 {
		runtime.Gosched()
	}
}

func (p *ServerPool) GetBackends() []*Backend {
	active := p.activeIndex.Load()
	p.readCounters[active].Add(1)
	defer p.readCounters[active].Add(-1)

	copied := make([]*Backend, len(p.backends[active]))
	copy(copied, p.backends[active])
	return copied
}

func (p *ServerPool) GetAliveBackends() []*Backend {
	active := p.activeIndex.Load()
	p.readCounters[active].Add(1)
	defer p.readCounters[active].Add(-1)

	var alive []*Backend
	for _, b := range p.backends[active] {
		if b.IsAlive() {
			alive = append(alive, b)
		}
	}
	return alive
}

func (p *ServerPool) GetBackendCount() int {
	active := p.activeIndex.Load()
	p.readCounters[active].Add(1)
	defer p.readCounters[active].Add(-1)
	return len(p.backends[active])
}

func (p *ServerPool) GetAliveCount() int {
	active := p.activeIndex.Load()
	p.readCounters[active].Add(1)
	defer p.readCounters[active].Add(-1)

	count := 0
	for _, b := range p.backends[active] {
		if b.IsAlive() {
			count++
		}
	}
	return count
}

package resource

import (
	"context"
	"sync"
	"time"
)

// DynamicGate limits concurrent heavy work to a changing slot count.
type DynamicGate struct {
	mu     sync.Mutex
	limit  int
	active int
}

func NewDynamicGate(initialLimit int) *DynamicGate {
	if initialLimit < 1 {
		initialLimit = 1
	}
	return &DynamicGate{limit: initialLimit}
}

func (g *DynamicGate) SetLimit(n int) {
	if n < 1 {
		n = 1
	}
	g.mu.Lock()
	g.limit = n
	g.mu.Unlock()
}

func (g *DynamicGate) Limit() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.limit
}

func (g *DynamicGate) Acquire(ctx context.Context) error {
	if g == nil {
		return nil
	}
	for {
		g.mu.Lock()
		if g.active < g.limit {
			g.active++
			g.mu.Unlock()
			return nil
		}
		g.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// TryAcquire grants a slot without blocking.
func (g *DynamicGate) TryAcquire() bool {
	if g == nil {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.active < g.limit {
		g.active++
		return true
	}
	return false
}

func (g *DynamicGate) Release() {
	if g == nil {
		return
	}
	g.mu.Lock()
	if g.active > 0 {
		g.active--
	}
	g.mu.Unlock()
}

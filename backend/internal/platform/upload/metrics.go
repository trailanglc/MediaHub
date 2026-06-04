package upload

import "sync/atomic"

// Metrics tracks coarse upload counters (process-local).
type Metrics struct {
	InitTotal     atomic.Uint64
	ChunkTotal    atomic.Uint64
	CompleteTotal atomic.Uint64
	FailTotal     atomic.Uint64
}

var Default = &Metrics{}

func (m *Metrics) Snapshot() map[string]uint64 {
	return map[string]uint64{
		"init_total":     m.InitTotal.Load(),
		"chunk_total":    m.ChunkTotal.Load(),
		"complete_total": m.CompleteTotal.Load(),
		"fail_total":     m.FailTotal.Load(),
	}
}

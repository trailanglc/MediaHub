package platform

import "sync/atomic"

// UploadMetrics tracks coarse upload counters (process-local).
type UploadMetrics struct {
	InitTotal     atomic.Uint64
	ChunkTotal    atomic.Uint64
	CompleteTotal atomic.Uint64
	FailTotal     atomic.Uint64
}

var Upload = &UploadMetrics{}

func (m *UploadMetrics) Snapshot() map[string]uint64 {
	return map[string]uint64{
		"init_total":     m.InitTotal.Load(),
		"chunk_total":    m.ChunkTotal.Load(),
		"complete_total": m.CompleteTotal.Load(),
		"fail_total":     m.FailTotal.Load(),
	}
}

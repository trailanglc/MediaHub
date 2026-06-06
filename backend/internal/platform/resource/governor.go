package resource

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	scaleUpMinHeadroomPercent = 30.0 // headroom must stay ≥30% (usage ≤70%) before scaling up
	scaleUpRecoveryTicks      = 3    // consecutive healthy samples before scale-up
	scaleDownHysteresisTicks  = 1    // scale down quickly under pressure
	scaleUpHysteresisTicks    = 2
)

// Governor samples metrics, publishes snapshots, and updates a dynamic gate.
type Governor struct {
	Log      *zap.Logger
	RDB      *redis.Client
	Policy   PolicyConfig
	Interval time.Duration
	Publish  bool
	SnapTTL  time.Duration

	Gate *DynamicGate

	mu                 sync.Mutex
	lastSlots          int
	pendingSlots       int
	pendingCount       int
	hysteresisTicks    int
	scaleUpReadyTicks  int
}

// NewGovernor wires a governor; gate may be nil if only publishing/reading.
func NewGovernor(log *zap.Logger, rdb *redis.Client, policy PolicyConfig, gate *DynamicGate, interval time.Duration, publish bool) *Governor {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &Governor{
		Log:             log,
		RDB:             rdb,
		Policy:          policy,
		Interval:        interval,
		Publish:         publish,
		SnapTTL:         defaultSnapshotTTL,
		Gate:            gate,
		lastSlots:       policy.ConvertMin,
		hysteresisTicks: 2,
	}
}

var latestLimits atomic.Value // stores Limits

// Run blocks until ctx is cancelled.
func (g *Governor) Run(ctx context.Context) {
	if g == nil {
		return
	}
	g.tick(ctx)
	ticker := time.NewTicker(g.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.tick(ctx)
		}
	}
}

func (g *Governor) tick(ctx context.Context) {
	snap, err := Collect(ctx, g.RDB)
	if err != nil {
		if g.Log != nil {
			g.Log.Warn("resource collect failed", zap.Error(err))
		}
		return
	}
	lim := LimitsFromSnapshot(snap, g.Policy)
	latestLimits.Store(lim)

	if g.Publish && g.RDB != nil {
		if err := Publish(ctx, g.RDB, snap, g.SnapTTL); err != nil && g.Log != nil {
			g.Log.Warn("resource publish failed", zap.Error(err))
		}
	}

	slots := g.applyHysteresis(lim.ConvertSlots, snap.HeadroomPercent)
	if g.Gate != nil {
		g.Gate.SetLimit(slots)
	}
	if g.Log != nil {
		g.Log.Debug("resource limits",
			zap.Float64("headroom", snap.HeadroomPercent),
			zap.Float64("pressure", snap.Pressure),
			zap.Int("convert_slots", slots),
			zap.Int("ffmpeg_threads", lim.FFmpegThreads),
		)
	}
}

func (g *Governor) applyHysteresis(target int, headroom float64) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.lastSlots == 0 {
		g.lastSlots = target
		return target
	}
	diff := target - g.lastSlots
	if diff == 0 {
		g.pendingCount = 0
		g.pendingSlots = target
		return g.lastSlots
	}

	required := g.hysteresisTicks
	if diff < 0 {
		g.scaleUpReadyTicks = 0
		required = scaleDownHysteresisTicks
	} else {
		if headroom < scaleUpMinHeadroomPercent {
			g.scaleUpReadyTicks = 0
			g.pendingCount = 0
			return g.lastSlots
		}
		g.scaleUpReadyTicks++
		if g.scaleUpReadyTicks < scaleUpRecoveryTicks {
			return g.lastSlots
		}
		required = scaleUpHysteresisTicks
	}

	if g.pendingSlots != target {
		g.pendingSlots = target
		g.pendingCount = 1
		return g.lastSlots
	}
	g.pendingCount++
	if g.pendingCount >= required && abs(diff) >= 1 {
		g.lastSlots = target
		g.pendingCount = 0
	}
	return g.lastSlots
}

// SeedLatestLimits sets initial limits (e.g. worker startup before first sample).
func SeedLatestLimits(policy PolicyConfig) {
	snap := &Snapshot{
		CPUIdlePercent: 100, RAMIdlePercent: 100, RedisIdlePercent: 100,
		HeadroomPercent: 100, Pressure: 0, CPUCount: 4,
	}
	latestLimits.Store(LimitsFromSnapshot(snap, policy))
}

// LatestLimits returns the most recently computed limits (or conservative defaults).
func LatestLimits(policy PolicyConfig) Limits {
	if v := latestLimits.Load(); v != nil {
		if lim, ok := v.(Limits); ok {
			return lim
		}
	}
	return conservativeLimits(policy)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

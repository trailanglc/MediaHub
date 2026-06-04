package resource

import (
	"testing"
	"time"
)

func TestLimitsFromSnapshot_headroomScaling(t *testing.T) {
	pc := DefaultPolicyConfig(1, 4, 40, 15, 85)
	tests := []struct {
		name     string
		headroom float64
		wantMin  int
		wantMax  int
	}{
		{"idle", 90, 3, 4},
		{"moderate", 50, 3, 4},
		{"busy", 15, 1, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := &Snapshot{
				CPUIdlePercent:   tt.headroom,
				RAMIdlePercent:   tt.headroom,
				RedisIdlePercent: tt.headroom,
				HeadroomPercent:  tt.headroom,
				Pressure:         1 - tt.headroom/100,
				CPUCount:         8,
			}
			lim := LimitsFromSnapshot(snap, pc)
			if lim.ConvertSlots < tt.wantMin || lim.ConvertSlots > tt.wantMax {
				t.Fatalf("slots=%d want [%d,%d]", lim.ConvertSlots, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestLimitsFromSnapshot_deferAndBusy(t *testing.T) {
	pc := DefaultPolicyConfig(1, 2, 40, 15, 85)
	snap := &Snapshot{
		CPUIdlePercent: 5, RAMIdlePercent: 10, RedisIdlePercent: 5,
		HeadroomPercent: 5, Pressure: 0.95, CPUCount: 4,
	}
	lim := LimitsFromSnapshot(snap, pc)
	if !lim.DeferConvert {
		t.Fatal("expected defer convert on low RAM")
	}
	if !lim.SystemBusy {
		t.Fatal("expected system busy")
	}
}

func TestRedisUnderPressure(t *testing.T) {
	pc := DefaultPolicyConfig(1, 2, 40, 15, 85)
	if !redisUnderPressure(10, pc.RedisMaxUsedPercent) {
		t.Fatal("expected pressure when redis idle 10% and max used 85%")
	}
	if redisUnderPressure(20, pc.RedisMaxUsedPercent) {
		t.Fatal("expected no pressure when redis idle 20%")
	}
	snap := &Snapshot{
		CPUIdlePercent: 50, RAMIdlePercent: 50, RedisIdlePercent: 10,
		HeadroomPercent: 10, CPUCount: 4,
	}
	lim := LimitsFromSnapshot(snap, pc)
	if !lim.SkipRedisCacheSet {
		t.Fatal("expected skip redis cache set when redis at 90% used")
	}
}

func TestLimitsFromSnapshot_storageSlots(t *testing.T) {
	pc := DefaultPolicyConfig(1, 2, 40, 15, 85)
	idle := &Snapshot{CPUIdlePercent: 90, RAMIdlePercent: 90, RedisIdlePercent: 90, HeadroomPercent: 90, CPUCount: 8}
	busy := &Snapshot{CPUIdlePercent: 10, RAMIdlePercent: 10, RedisIdlePercent: 10, HeadroomPercent: 10, CPUCount: 8}
	if LimitsFromSnapshot(idle, pc).StorageImmediateSlots < LimitsFromSnapshot(busy, pc).StorageImmediateSlots {
		t.Fatalf("idle slots %d should be >= busy %d",
			LimitsFromSnapshot(idle, pc).StorageImmediateSlots,
			LimitsFromSnapshot(busy, pc).StorageImmediateSlots)
	}
}

func TestEffectiveTTL(t *testing.T) {
	base := 45 * time.Second
	lim := Limits{CacheTTLFactor: 0.6}
	got := EffectiveTTL(base, 15*time.Second, lim)
	if got != 27*time.Second {
		t.Fatalf("got %v want 27s", got)
	}
}

package resource

import (
	"math"
	"time"

	"github.com/anhtuanlc/mediahub/internal/platform/capacity"
)

// PolicyConfig holds tunables for limit derivation.
type PolicyConfig struct {
	ConvertMin              int
	ConvertMax              int
	CPUReserveTargetPercent int
	RAMMinIdlePercent       int
	RedisMaxUsedPercent     int
	SchedulerBatchMin       int
	SchedulerBatchMax       int
	StorageSlotsMin         int
	StorageSlotsMax         int
	BusyPressureThreshold   float64
}

// DefaultPolicyConfig maps application config into policy defaults.
func DefaultPolicyConfig(convertMin, convertMax, cpuReserve, ramMinIdle, redisMaxUsed int) PolicyConfig {
	if convertMin < 1 {
		convertMin = 1
	}
	if convertMax < convertMin {
		convertMax = convertMin
	}
	if cpuReserve < 10 {
		cpuReserve = 40
	}
	return PolicyConfig{
		ConvertMin:              convertMin,
		ConvertMax:              convertMax,
		CPUReserveTargetPercent: cpuReserve,
		RAMMinIdlePercent:       ramMinIdle,
		RedisMaxUsedPercent:     redisMaxUsed,
		SchedulerBatchMin:       10,
		SchedulerBatchMax:       50,
		StorageSlotsMin:         2,
		StorageSlotsMax:         8,
		BusyPressureThreshold:   1 - capacity.MinIdlePercentForMaxUsage()/100,
	}
}

// LimitsFromSnapshot computes operational limits from a snapshot.
func LimitsFromSnapshot(snap *Snapshot, pc PolicyConfig) Limits {
	def := conservativeLimits(pc)
	if snap == nil {
		return def
	}

	headroom := snap.HeadroomPercent
	minIdle := capacity.MinIdlePercentForMaxUsage()
	target := float64(pc.CPUReserveTargetPercent)
	if target < minIdle {
		target = minIdle
	}
	if target <= 0 {
		target = minIdle
	}

	scale := headroom / target
	if scale > 1 {
		scale = 1
	}
	if scale < 0 {
		scale = 0
	}
	if headroom < minIdle {
		scale = 0
	}

	slots := pc.ConvertMin
	if pc.ConvertMax > pc.ConvertMin {
		span := float64(pc.ConvertMax - pc.ConvertMin)
		slots = pc.ConvertMin + int(math.Floor(span*scale+0.5))
	} else {
		slots = pc.ConvertMax
	}
	if slots < pc.ConvertMin {
		slots = pc.ConvertMin
	}
	if slots > pc.ConvertMax {
		slots = pc.ConvertMax
	}

	cpuCount := snap.CPUCount
	if cpuCount < 1 {
		cpuCount = 1
	}
	threads := int(math.Max(1, math.Floor(float64(cpuCount)*headroom/100)))
	if slots > 0 && threads > cpuCount/slots {
		threads = int(math.Max(1, float64(cpuCount)/float64(slots)))
	}
	if threads > cpuCount {
		threads = cpuCount
	}

	ttlFactor := 0.5 + 0.5*scale
	if snap.Pressure <= 0.5 {
		ttlFactor = 1.0
	}

	batchSpan := float64(pc.SchedulerBatchMax - pc.SchedulerBatchMin)
	batch := pc.SchedulerBatchMin + int(math.Floor(batchSpan*scale+0.5))
	slotSpan := float64(pc.StorageSlotsMax - pc.StorageSlotsMin)
	immSlots := pc.StorageSlotsMin + int(math.Floor(slotSpan*scale+0.5))

	ramMinIdle := float64(pc.RAMMinIdlePercent)
	if ramMinIdle < minIdle {
		ramMinIdle = minIdle
	}
	deferConvert := snap.RAMIdlePercent < ramMinIdle || snap.CPUIdlePercent < minIdle
	skipSet := redisUnderPressure(snap.RedisIdlePercent, pc.RedisMaxUsedPercent)
	busy := snap.Pressure >= pc.BusyPressureThreshold || headroom < minIdle

	return Limits{
		ConvertSlots:           slots,
		FFmpegThreads:          threads,
		CacheTTLFactor:         round2(ttlFactor),
		SchedulerDeletionBatch: batch,
		StorageImmediateSlots:  immSlots,
		DeferConvert:           deferConvert,
		SkipRedisCacheSet:      skipSet,
		SystemBusy:             busy,
	}
}

// redisUnderPressure is true when used memory is at or above RedisMaxUsedPercent.
func redisUnderPressure(redisIdlePercent float64, maxUsedPercent int) bool {
	if maxUsedPercent <= 0 || maxUsedPercent > 100 {
		maxUsedPercent = 85
	}
	minIdle := float64(100 - maxUsedPercent)
	return redisIdlePercent < minIdle
}

func conservativeLimits(pc PolicyConfig) Limits {
	return Limits{
		ConvertSlots:           pc.ConvertMin,
		FFmpegThreads:          1,
		CacheTTLFactor:         1,
		SchedulerDeletionBatch: pc.SchedulerBatchMin,
		StorageImmediateSlots:  pc.StorageSlotsMin,
	}
}

// EffectiveTTL scales a base TTL by cache factor with a minimum floor.
func EffectiveTTL(base, minTTL time.Duration, lim Limits) time.Duration {
	if lim.CacheTTLFactor <= 0 || lim.CacheTTLFactor >= 1 {
		return base
	}
	d := time.Duration(float64(base) * lim.CacheTTLFactor)
	if d < minTTL {
		return minTTL
	}
	if d < time.Second {
		return time.Second
	}
	return d
}

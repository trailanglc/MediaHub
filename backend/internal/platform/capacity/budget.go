package capacity

import "math"

// Resource usage ceilings used for autoscale planning and runtime throttling.
const (
	MaxUsagePercent     = 90.0
	RAMWarnUsagePercent = 80.0
	MinRAMHeadroomGiB   = 1
	EstConvertRAMGiB    = 3
)

// MinIdlePercentForMaxUsage is the minimum idle % that corresponds to MaxUsagePercent.
func MinIdlePercentForMaxUsage() float64 {
	return 100 - MaxUsagePercent
}

// RAMMinIdlePercent returns the minimum RAM idle % to keep at least MinRAMHeadroomGiB
// while never planning above MaxUsagePercent.
func RAMMinIdlePercent(totalRAMBytes uint64) int {
	minIdle := int(MinIdlePercentForMaxUsage())
	if totalRAMBytes == 0 {
		return minIdle
	}
	headroomBytes := uint64(MinRAMHeadroomGiB) << 30
	fromHeadroom := int(math.Ceil(float64(headroomBytes) / float64(totalRAMBytes) * 100))
	if fromHeadroom > minIdle {
		minIdle = fromHeadroom
	}
	if minIdle > 50 {
		minIdle = 50
	}
	return minIdle
}

// ConvertSlotsBudget plans max concurrent converts from CPU and RAM without exceeding MaxUsagePercent.
func ConvertSlotsBudget(cpus, ramGiB int) int {
	if cpus < 1 {
		cpus = 1
	}
	if ramGiB < 1 {
		ramGiB = 1
	}

	cpuSlots := int(math.Floor(float64(cpus) * MaxUsagePercent / 100))
	if cpuSlots < 1 {
		cpuSlots = 1
	}

	usableRAMGiB := float64(ramGiB)*MaxUsagePercent/100 - float64(MinRAMHeadroomGiB)
	if usableRAMGiB < float64(EstConvertRAMGiB) {
		usableRAMGiB = float64(EstConvertRAMGiB)
	}
	ramSlots := int(math.Floor(usableRAMGiB / float64(EstConvertRAMGiB)))
	if ramSlots < 1 {
		ramSlots = 1
	}

	slots := cpuSlots
	if ramSlots < slots {
		slots = ramSlots
	}
	if slots > 32 {
		slots = 32
	}
	return slots
}

// UsageLevel classifies a utilization percent for health warnings.
func UsageLevel(usedPercent float64) string {
	switch {
	case usedPercent >= MaxUsagePercent:
		return "critical"
	case usedPercent >= RAMWarnUsagePercent:
		return "warning"
	default:
		return ""
	}
}

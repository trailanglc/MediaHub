package config

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/anhtuanlc/mediahub/internal/platform/capacity"
	"github.com/shirou/gopsutil/v4/mem"
)

// hostCapacity describes the machine for deriving defaults.
type hostCapacity struct {
	CPUs         int
	RAMGiB       int
	RAMTotalBytes uint64
}

func detectHostCapacity() hostCapacity {
	c := hostCapacity{CPUs: runtime.NumCPU(), RAMGiB: 4}
	if c.CPUs < 1 {
		c.CPUs = 1
	}
	if vm, err := mem.VirtualMemory(); err == nil && vm.Total > 0 {
		c.RAMTotalBytes = vm.Total
		c.RAMGiB = int(vm.Total / (1024 * 1024 * 1024))
		if c.RAMGiB < 1 {
			c.RAMGiB = 1
		}
	}
	return c
}

func envUnset(key string) bool {
	return strings.TrimSpace(os.Getenv(key)) == ""
}

func applyAutoscale(cfg *Config) {
	if cfg == nil {
		return
	}
	host := detectHostCapacity()
	var notes []string

	if envUnset("CONVERT_MAX_CONCURRENT") {
		v := capacity.ConvertSlotsBudget(host.CPUs, host.RAMGiB)
		cfg.ConvertMaxConcurrent = v
		notes = append(notes, fmt.Sprintf("CONVERT_MAX_CONCURRENT=%d (%d CPU, %d GiB RAM, max %.0f%% usage)", v, host.CPUs, host.RAMGiB, capacity.MaxUsagePercent))
	}
	if envUnset("CONVERT_MIN_CONCURRENT") {
		cfg.ConvertMinConcurrent = autoConvertMin(host)
		notes = append(notes, fmt.Sprintf("CONVERT_MIN_CONCURRENT=%d", cfg.ConvertMinConcurrent))
	}
	if cfg.ConvertMinConcurrent > cfg.ConvertMaxConcurrent {
		cfg.ConvertMinConcurrent = cfg.ConvertMaxConcurrent
	}

	if envUnset("CONVERT_QUEUE_MAX_DEPTH") {
		cfg.ConvertQueueMaxDepth = autoConvertQueueMaxDepth(cfg.ConvertMaxConcurrent)
		notes = append(notes, fmt.Sprintf("CONVERT_QUEUE_MAX_DEPTH=%d", cfg.ConvertQueueMaxDepth))
	}

	if envUnset("DB_MAX_CONNS") {
		cfg.DBMaxConns = autoDBMaxConns(host)
		notes = append(notes, fmt.Sprintf("DB_MAX_CONNS=%d", cfg.DBMaxConns))
	}
	if envUnset("DB_MIN_CONNS") {
		cfg.DBMinConns = autoDBMinConns(host)
		notes = append(notes, fmt.Sprintf("DB_MIN_CONNS=%d", cfg.DBMinConns))
	}
	if cfg.DBMinConns > cfg.DBMaxConns {
		cfg.DBMinConns = cfg.DBMaxConns
	}

	if envUnset("REDIS_POOL_SIZE") {
		cfg.RedisPoolSize = autoRedisPoolSize(host)
		notes = append(notes, fmt.Sprintf("REDIS_POOL_SIZE=%d", cfg.RedisPoolSize))
	}
	if envUnset("REDIS_MIN_IDLE_CONNS") {
		cfg.RedisMinIdleConns = autoRedisMinIdle(host)
		notes = append(notes, fmt.Sprintf("REDIS_MIN_IDLE_CONNS=%d", cfg.RedisMinIdleConns))
	}
	if cfg.RedisMinIdleConns > cfg.RedisPoolSize {
		cfg.RedisMinIdleConns = cfg.RedisPoolSize
	}

	if envUnset("RESOURCE_CPU_RESERVE_PERCENT") {
		cfg.ResourceCPUReservePercent = int(capacity.MinIdlePercentForMaxUsage())
		notes = append(notes, fmt.Sprintf("RESOURCE_CPU_RESERVE_PERCENT=%d (max %.0f%% CPU)", cfg.ResourceCPUReservePercent, capacity.MaxUsagePercent))
	}

	if envUnset("RESOURCE_RAM_MIN_IDLE_PERCENT") {
		cfg.ResourceRAMMinIdlePercent = capacity.RAMMinIdlePercent(host.RAMTotalBytes)
		notes = append(notes, fmt.Sprintf("RESOURCE_RAM_MIN_IDLE_PERCENT=%d (headroom >= %d GiB)", cfg.ResourceRAMMinIdlePercent, capacity.MinRAMHeadroomGiB))
	}

	if envUnset("UPLOAD_INIT_PER_MINUTE") {
		cfg.UploadInitPerMinute = autoUploadInitPerMinute(host)
		notes = append(notes, fmt.Sprintf("UPLOAD_INIT_PER_MINUTE=%d", cfg.UploadInitPerMinute))
	}

	cfg.autoscaleNotes = notes
}

// AutoscaleNotes lists env-derived values applied at startup (empty if everything was set in .env).
func (c *Config) AutoscaleNotes() []string {
	if c == nil {
		return nil
	}
	return append([]string(nil), c.autoscaleNotes...)
}

func autoConvertQueueMaxDepth(convertMax int) int {
	if convertMax < 1 {
		convertMax = 1
	}
	n := convertMax * 25
	if n < 50 {
		n = 50
	}
	if n > 200 {
		n = 200
	}
	return n
}

func autoConvertMin(host hostCapacity) int {
	if host.CPUs <= 2 || host.RAMGiB <= 4 {
		return 1
	}
	return 2
}

func autoDBMaxConns(host hostCapacity) int {
	n := int(float64(host.CPUs*3) * capacity.MaxUsagePercent / 100)
	if n < 10 {
		n = 10
	}
	if n > 72 {
		n = 72
	}
	return n
}

func autoDBMinConns(host hostCapacity) int {
	n := host.CPUs / 2
	if n < 2 {
		n = 2
	}
	if n > 16 {
		n = 16
	}
	return n
}

func autoRedisPoolSize(host hostCapacity) int {
	n := int(float64(host.CPUs*4) * capacity.MaxUsagePercent / 100)
	if n < 16 {
		n = 16
	}
	if n > 115 {
		n = 115
	}
	return n
}

func autoRedisMinIdle(host hostCapacity) int {
	n := int(float64(host.CPUs*2) * capacity.MaxUsagePercent / 100)
	if n < 4 {
		n = 4
	}
	if n > 29 {
		n = 29
	}
	return n
}

func autoUploadInitPerMinute(host hostCapacity) int {
	n := int(float64(host.CPUs*15) * capacity.MaxUsagePercent / 100)
	if n < 30 {
		n = 30
	}
	if n > 270 {
		n = 270
	}
	return n
}

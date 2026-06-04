package resource

import "time"

const (
	RedisKeySnapshot = "system:resource:v1"
	defaultSnapshotTTL = 15 * time.Second
)

// Snapshot captures host and Redis pressure at a point in time.
type Snapshot struct {
	CPUIdlePercent    float64   `json:"cpu_idle_percent"`
	RAMIdlePercent    float64   `json:"ram_idle_percent"`
	RedisIdlePercent  float64   `json:"redis_idle_percent"`
	HeadroomPercent   float64   `json:"headroom_percent"`
	Pressure          float64   `json:"pressure"`
	CPUCount          int       `json:"cpu_count"`
	At                time.Time `json:"at"`
}

// Limits are derived policy outputs for consumers.
type Limits struct {
	ConvertSlots            int     `json:"convert_slots"`
	FFmpegThreads           int     `json:"ffmpeg_threads"`
	CacheTTLFactor          float64 `json:"cache_ttl_factor"`
	SchedulerDeletionBatch  int     `json:"scheduler_deletion_batch"`
	StorageImmediateSlots   int     `json:"storage_immediate_slots"`
	DeferConvert            bool    `json:"defer_convert"`
	SkipRedisCacheSet       bool    `json:"skip_redis_cache_set"`
	SystemBusy              bool    `json:"system_busy"`
}

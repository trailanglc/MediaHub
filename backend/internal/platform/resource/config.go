package resource

import (
	"github.com/anhtuanlc/mediahub/internal/config"
	"github.com/redis/go-redis/v9"
)

// PolicyFromConfig builds policy from application config.
func PolicyFromConfig(cfg *config.Config) PolicyConfig {
	if cfg == nil {
		return DefaultPolicyConfig(1, 2, 40, 15, 85)
	}
	return DefaultPolicyConfig(
		cfg.ConvertMinConcurrent,
		cfg.ConvertMaxConcurrent,
		cfg.ResourceCPUReservePercent,
		cfg.ResourceRAMMinIdlePercent,
		cfg.ResourceRedisMaxUsedPercent,
	)
}

// NewReaderFromConfig creates a reader for API/scheduler processes.
func NewReaderFromConfig(cfg *config.Config, rdb *redis.Client) *Reader {
	if cfg == nil || !cfg.ResourceGovernorEnabled {
		return &Reader{Enabled: false}
	}
	return &Reader{
		RDB:     rdb,
		Policy:  PolicyFromConfig(cfg),
		Enabled: true,
	}
}

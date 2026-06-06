package config

import "testing"

func TestApplyAutoscale_UsesEnvWhenSet(t *testing.T) {
	t.Setenv("CONVERT_MAX_CONCURRENT", "9")
	t.Setenv("DB_MAX_CONNS", "25")
	t.Setenv("REDIS_POOL_SIZE", "40")

	cfg := &Config{
		ConvertMaxConcurrent: getEnvInt("CONVERT_MAX_CONCURRENT", 0),
		DBMaxConns:           getEnvInt("DB_MAX_CONNS", 0),
		RedisPoolSize:        getEnvInt("REDIS_POOL_SIZE", 0),
	}
	applyAutoscale(cfg)

	if cfg.ConvertMaxConcurrent != 9 {
		t.Fatalf("ConvertMaxConcurrent=%d want 9", cfg.ConvertMaxConcurrent)
	}
	if cfg.DBMaxConns != 25 {
		t.Fatalf("DBMaxConns=%d want 25", cfg.DBMaxConns)
	}
	if cfg.RedisPoolSize != 40 {
		t.Fatalf("RedisPoolSize=%d want 40", cfg.RedisPoolSize)
	}
}

func TestApplyAutoscale_FillsUnset(t *testing.T) {
	t.Setenv("CONVERT_MAX_CONCURRENT", "")
	t.Setenv("CONVERT_MIN_CONCURRENT", "")
	t.Setenv("DB_MAX_CONNS", "")
	t.Setenv("DB_MIN_CONNS", "")
	t.Setenv("REDIS_POOL_SIZE", "")
	t.Setenv("REDIS_MIN_IDLE_CONNS", "")
	t.Setenv("RESOURCE_CPU_RESERVE_PERCENT", "")
	t.Setenv("RESOURCE_RAM_MIN_IDLE_PERCENT", "")
	t.Setenv("UPLOAD_INIT_PER_MINUTE", "")

	cfg := &Config{}
	applyAutoscale(cfg)

	if cfg.ConvertMaxConcurrent < 1 {
		t.Fatalf("ConvertMaxConcurrent: %d", cfg.ConvertMaxConcurrent)
	}
	if cfg.ConvertMinConcurrent < 1 {
		t.Fatalf("ConvertMinConcurrent: %d", cfg.ConvertMinConcurrent)
	}
	if cfg.DBMaxConns < 10 {
		t.Fatalf("DBMaxConns: %d", cfg.DBMaxConns)
	}
	if cfg.RedisPoolSize < 16 {
		t.Fatalf("RedisPoolSize: %d", cfg.RedisPoolSize)
	}
	if cfg.ResourceCPUReservePercent != 10 {
		t.Fatalf("ResourceCPUReservePercent=%d want 10", cfg.ResourceCPUReservePercent)
	}
	if cfg.ResourceRAMMinIdlePercent < 10 {
		t.Fatalf("ResourceRAMMinIdlePercent: %d", cfg.ResourceRAMMinIdlePercent)
	}
	if len(cfg.AutoscaleNotes()) == 0 {
		t.Fatal("expected autoscale notes")
	}
}

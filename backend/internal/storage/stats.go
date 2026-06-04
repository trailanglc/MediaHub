package storage

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/minio/madmin-go/v3"
)

// StorageStats describes bucket usage and backing volume capacity when available.
type StorageStats struct {
	Bucket      string
	UsedBytes   int64
	TotalBytes  int64
	FreeBytes   int64
	ObjectCount int64
}

func (s *StorageStats) UsedPercent() float64 {
	if s.TotalBytes <= 0 {
		return 0
	}
	return float64(s.UsedBytes) / float64(s.TotalBytes) * 100
}

// Stats returns used/free capacity for the configured bucket (cached).
func (s *S3Storage) Stats(ctx context.Context) (*StorageStats, error) {
	if s.statsCache == nil {
		return s.statsUncached(ctx)
	}
	return s.statsCache.GetOrCompute(ctx, s.statsUncached)
}

func (s *S3Storage) statsUncached(ctx context.Context) (*StorageStats, error) {
	if stats, err := s.statsViaMadmin(ctx); err == nil {
		return stats, nil
	}
	return s.statsViaListObjects(ctx)
}

func (s *S3Storage) statsViaMadmin(ctx context.Context) (*StorageStats, error) {
	host, secure, err := endpointHostSecure(s.endpoint)
	if err != nil {
		return nil, err
	}

	adm, err := madmin.New(host, s.accessKey, s.secretKey, secure)
	if err != nil {
		return nil, err
	}

	usage, err := adm.DataUsageInfo(ctx)
	if err != nil {
		return nil, err
	}

	stats := &StorageStats{Bucket: s.bucket}
	if bu, ok := usage.BucketsUsage[s.bucket]; ok {
		stats.UsedBytes = int64(bu.Size)
		stats.ObjectCount = int64(bu.ObjectsCount)
	}

	si, err := adm.StorageInfo(ctx)
	if err != nil {
		return stats, nil
	}

	var total, free uint64
	for _, disk := range si.Disks {
		total += disk.TotalSpace
		free += disk.AvailableSpace
	}
	if total > 0 {
		stats.TotalBytes = int64(total)
		if free > 0 {
			stats.FreeBytes = int64(free)
		} else if stats.UsedBytes > 0 {
			stats.FreeBytes = int64(total) - stats.UsedBytes
			if stats.FreeBytes < 0 {
				stats.FreeBytes = 0
			}
		}
	}
	return stats, nil
}

const listStatsFallbackMaxObjects = 50_000

func (s *S3Storage) statsViaListObjects(ctx context.Context) (*StorageStats, error) {
	stats := &StorageStats{Bucket: s.bucket}
	if s.quotaBytes > 0 {
		stats.TotalBytes = s.quotaBytes
	}

	var token *string
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("list objects: %w", err)
		}
		for _, obj := range out.Contents {
			if obj.Size != nil {
				stats.UsedBytes += *obj.Size
			}
			stats.ObjectCount++
			if stats.ObjectCount >= listStatsFallbackMaxObjects {
				return stats, nil
			}
		}
		if !aws.ToBool(out.IsTruncated) {
			break
		}
		token = out.NextContinuationToken
	}

	if stats.TotalBytes > 0 {
		stats.FreeBytes = stats.TotalBytes - stats.UsedBytes
		if stats.FreeBytes < 0 {
			stats.FreeBytes = 0
		}
	}
	return stats, nil
}

func endpointHostSecure(endpoint string) (host string, secure bool, err error) {
	if endpoint == "" {
		return "", false, fmt.Errorf("empty endpoint")
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", false, err
	}
	if u.Host == "" {
		return endpoint, false, nil
	}
	return u.Host, u.Scheme == "https", nil
}

// StatsDetails formats stats for the health API details map.
func StatsDetails(stats *StorageStats, cachedAt time.Time, cacheTTL time.Duration) map[string]string {
	d := map[string]string{
		"bucket":       stats.Bucket,
		"used_bytes":   strconv.FormatInt(stats.UsedBytes, 10),
		"object_count": strconv.FormatInt(stats.ObjectCount, 10),
	}
	if stats.TotalBytes > 0 {
		d["total_bytes"] = strconv.FormatInt(stats.TotalBytes, 10)
		d["free_bytes"] = strconv.FormatInt(stats.FreeBytes, 10)
		d["used_percent"] = fmt.Sprintf("%.2f", stats.UsedPercent())
	}
	if stats.ObjectCount >= listStatsFallbackMaxObjects {
		d["stats_partial"] = "true"
		d["stats_note"] = fmt.Sprintf("list fallback capped at %d objects", listStatsFallbackMaxObjects)
	}
	if !cachedAt.IsZero() && cacheTTL > 0 {
		d["stats_cached_at"] = cachedAt.UTC().Format(time.RFC3339)
		d["stats_cache_ttl_seconds"] = strconv.Itoa(int(cacheTTL.Seconds()))
	}
	return d
}

package platform

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

type MemoryStats struct {
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

type DiskStats struct {
	Path        string  `json:"path"`
	TotalBytes  uint64  `json:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes"`
	FreeBytes   uint64  `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

type HostStats struct {
	Hostname       string      `json:"hostname"`
	OS             string      `json:"os"`
	Platform       string      `json:"platform"`
	KernelVersion  string      `json:"kernel_version"`
	UptimeSeconds  uint64      `json:"uptime_seconds"`
	CPUPercent     float64     `json:"cpu_percent"`
	CPUCount       int         `json:"cpu_count"`
	Memory         MemoryStats `json:"memory"`
	Disks          []DiskStats `json:"disks"`
}

func CollectHostStats(ctx context.Context) (*HostStats, error) {
	_ = ctx
	info, err := host.Info()
	if err != nil {
		return nil, err
	}

	cpuCount, _ := cpu.Counts(true)
	cpuPct := 0.0
	if percents, err := cpu.Percent(150*time.Millisecond, false); err == nil && len(percents) > 0 {
		cpuPct = percents[0]
	}

	vm, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	disks := collectDiskUsage()

	return &HostStats{
		Hostname:      info.Hostname,
		OS:            info.OS,
		Platform:      info.Platform,
		KernelVersion: info.KernelVersion,
		UptimeSeconds: info.Uptime,
		CPUPercent:    round2(cpuPct),
		CPUCount:      cpuCount,
		Memory: MemoryStats{
			TotalBytes:     vm.Total,
			UsedBytes:      vm.Used,
			AvailableBytes: vm.Available,
			UsedPercent:    round2(vm.UsedPercent),
		},
		Disks: disks,
	}, nil
}

func collectDiskUsage() []DiskStats {
	paths := []string{"/"}
	// macOS root volume is often on /
	if parts, err := disk.Partitions(false); err == nil {
		seen := map[string]bool{"/": true}
		for _, p := range parts {
			if p.Mountpoint == "/" || p.Mountpoint == "/System/Volumes/Data" {
				if !seen[p.Mountpoint] {
					paths = append(paths, p.Mountpoint)
					seen[p.Mountpoint] = true
				}
			}
		}
	}

	var out []DiskStats
	seenPath := map[string]bool{}
	for _, path := range paths {
		if seenPath[path] {
			continue
		}
		seenPath[path] = true
		usage, err := disk.Usage(path)
		if err != nil {
			continue
		}
		out = append(out, DiskStats{
			Path:        path,
			TotalBytes:  usage.Total,
			UsedBytes:   usage.Used,
			FreeBytes:   usage.Free,
			UsedPercent: round2(usage.UsedPercent),
		})
	}
	return out
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

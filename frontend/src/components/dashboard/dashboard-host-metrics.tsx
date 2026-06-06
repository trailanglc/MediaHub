"use client";

import type { SystemOverviewResponse } from "@/lib/api/api-client";
import { MetricCard } from "@/components/system/metric-card";
import { Skeleton } from "@/components/ui/skeleton";

export function DashboardHostMetrics({
  data,
  loading,
}: {
  data?: SystemOverviewResponse;
  loading: boolean;
}) {
  if (loading) {
    return (
      <div className="grid gap-4 sm:grid-cols-3">
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-36 rounded-xl" />
        ))}
      </div>
    );
  }

  const host = data?.host;
  if (!host) {
    return (
      <p className="text-sm text-muted-foreground">
        Không có dữ liệu host metrics.
      </p>
    );
  }

  const disk = host.disks?.[0];
  const diskPercent = disk?.used_percent ?? 0;

  return (
    <section aria-label="Tài nguyên máy chủ" className="space-y-3">
      <h2 className="text-lg font-semibold tracking-tight">Tài nguyên host</h2>
      <div className="grid gap-4 sm:grid-cols-3">
        <MetricCard
          title="CPU"
          value={`${host.cpu_percent.toFixed(1)}%`}
          sub="Mức sử dụng CPU máy chủ API"
          percent={host.cpu_percent}
        />
        <MetricCard
          title="RAM"
          value={`${host.memory.used_percent.toFixed(1)}%`}
          sub="Bộ nhớ máy chủ"
          percent={host.memory.used_percent}
        />
        <MetricCard
          title={disk ? `Disk ${disk.path}` : "Disk"}
          value={`${diskPercent.toFixed(1)}%`}
          sub={disk ? `Ổ đĩa ${disk.path}` : "Ổ đĩa hệ thống"}
          percent={diskPercent}
        />
      </div>
    </section>
  );
}

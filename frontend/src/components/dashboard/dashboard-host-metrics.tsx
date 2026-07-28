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
      <div className="grid grid-cols-3 gap-2 sm:gap-4">
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-24 rounded-xl sm:h-36" />
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
    <section aria-label="Tài nguyên máy chủ" className="space-y-2 sm:space-y-3">
      <h2 className="text-base font-semibold tracking-tight sm:text-lg">
        Tài nguyên host
      </h2>
      <div className="grid grid-cols-3 gap-2 sm:gap-4">
        <MetricCard
          title="CPU"
          value={`${host.cpu_percent.toFixed(1)}%`}
          sub="Mức sử dụng CPU máy chủ API"
          percent={host.cpu_percent}
          compact
        />
        <MetricCard
          title="RAM"
          value={`${host.memory.used_percent.toFixed(1)}%`}
          sub="Bộ nhớ máy chủ"
          percent={host.memory.used_percent}
          compact
        />
        <MetricCard
          title={disk ? `Disk ${disk.path}` : "Disk"}
          value={`${diskPercent.toFixed(1)}%`}
          sub={disk ? `Ổ đĩa ${disk.path}` : "Ổ đĩa hệ thống"}
          percent={diskPercent}
          compact
        />
      </div>
    </section>
  );
}

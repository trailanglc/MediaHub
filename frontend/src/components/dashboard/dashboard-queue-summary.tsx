"use client";

import Link from "next/link";
import type { SystemOverviewResponse } from "@/lib/api/api-client";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";

function countMap(m?: Record<string, number>, keys?: string[]) {
  if (!m) return 0;
  if (!keys) {
    return Object.values(m).reduce((a, b) => a + b, 0);
  }
  return keys.reduce((a, k) => a + (m[k] ?? 0), 0);
}

export function DashboardQueueSummary({
  data,
  loading,
}: {
  data?: SystemOverviewResponse;
  loading: boolean;
}) {
  if (loading) {
    return <Skeleton className="h-40 rounded-xl sm:h-56" />;
  }

  const queue = data?.queue;
  if (!queue) {
    return (
      <Card size="sm">
        <CardContent className="py-6 text-sm text-muted-foreground sm:py-8">
          Không có dữ liệu queue.
        </CardContent>
      </Card>
    );
  }

  const jobs = queue.convert_jobs ?? {};
  const pending = countMap(jobs, ["pending", "queued"]);
  const active = countMap(jobs, ["processing", "running"]);
  const failed = countMap(jobs, ["failed", "error"]);
  const depthPercent =
    queue.max_depth != null && queue.max_depth > 0
      ? Math.min(100, (queue.depth / queue.max_depth) * 100)
      : null;

  const stats = [
    {
      value: queue.depth,
      label: queue.max_depth
        ? `Độ sâu / ${queue.max_depth}`
        : "Độ sâu",
    },
    { value: pending, label: "Chờ" },
    { value: active, label: "Đang chạy" },
    { value: failed, label: "Thất bại" },
  ];

  return (
    <Card size="sm" className="gap-3 py-3 sm:gap-4 sm:py-4">
      <CardHeader className="flex flex-row items-start justify-between gap-2 px-3 pb-0 sm:px-4">
        <div className="min-w-0">
          <CardTitle className="text-base sm:text-lg">Convert queue</CardTitle>
          <CardDescription className="text-xs sm:text-sm">
            Hàng đợi FFmpeg (Asynq)
          </CardDescription>
        </div>
        {queue.paused ? <Badge variant="secondary">Paused</Badge> : null}
      </CardHeader>
      <CardContent className="space-y-3 px-3 sm:space-y-4 sm:px-4">
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 sm:gap-3">
          {stats.map((s) => (
            <div
              key={s.label}
              className="rounded-lg border bg-muted/30 p-2.5 sm:p-3"
            >
              <p className="text-xl font-bold tabular-nums sm:text-2xl">
                {s.value}
              </p>
              <p className="text-[11px] text-muted-foreground sm:text-xs">
                {s.label}
              </p>
            </div>
          ))}
        </div>

        {depthPercent != null ? <Progress value={depthPercent} /> : null}

        <Link
          href="/system/queue"
          className="text-xs font-medium underline-offset-4 hover:underline sm:text-sm"
        >
          Quản lý queue
        </Link>
      </CardContent>
    </Card>
  );
}

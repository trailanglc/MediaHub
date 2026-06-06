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
    return <Skeleton className="h-56 rounded-xl" />;
  }

  const queue = data?.queue;
  if (!queue) {
    return (
      <Card>
        <CardContent className="py-8 text-sm text-muted-foreground">
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

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-2">
        <div>
          <CardTitle>Convert queue</CardTitle>
          <CardDescription>Hàng đợi FFmpeg (Asynq)</CardDescription>
        </div>
        {queue.paused ? <Badge variant="secondary">Paused</Badge> : null}
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-4">
          <div className="rounded-lg border bg-muted/30 p-3">
            <p className="text-2xl font-bold tabular-nums">{queue.depth}</p>
            <p className="text-xs text-muted-foreground">
              Độ sâu{queue.max_depth ? ` / ${queue.max_depth}` : ""}
            </p>
          </div>
          <div className="rounded-lg border bg-muted/30 p-3">
            <p className="text-2xl font-bold tabular-nums">{pending}</p>
            <p className="text-xs text-muted-foreground">Chờ</p>
          </div>
          <div className="rounded-lg border bg-muted/30 p-3">
            <p className="text-2xl font-bold tabular-nums">{active}</p>
            <p className="text-xs text-muted-foreground">Đang chạy</p>
          </div>
          <div className="rounded-lg border bg-muted/30 p-3">
            <p className="text-2xl font-bold tabular-nums">{failed}</p>
            <p className="text-xs text-muted-foreground">Thất bại</p>
          </div>
        </div>

        {depthPercent != null ? (
          <Progress value={depthPercent} />
        ) : null}

        <Link
          href="/system/queue"
          className="text-sm font-medium underline-offset-4 hover:underline"
        >
          Quản lý queue
        </Link>
      </CardContent>
    </Card>
  );
}

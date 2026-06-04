"use client";

import { useQuery } from "@tanstack/react-query";
import { fetchQueueStatus, fetchStreamAnalytics } from "@/lib/api-client";
import { PageError, PageLoading } from "@/components/feedback/page-states";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { PageHeader } from "@/components/ui/page-header";

function countMap(m?: Record<string, number>, keys?: string[]) {
  if (!m) return 0;
  if (!keys) {
    return Object.values(m).reduce((a, b) => a + b, 0);
  }
  return keys.reduce((a, k) => a + (m[k] ?? 0), 0);
}

export function SystemQueuePanel() {
  const queue = useQuery({
    queryKey: ["system-queue"],
    queryFn: fetchQueueStatus,
    refetchInterval: 10_000,
  });
  const analytics = useQuery({
    queryKey: ["stream-analytics"],
    queryFn: fetchStreamAnalytics,
    refetchInterval: 30_000,
  });

  if (queue.isLoading) return <PageLoading />;
  if (queue.isError) return <PageError onRetry={() => queue.refetch()} />;

  const jobs = queue.data?.convert_jobs ?? {};
  const hls = queue.data?.video_hls ?? {};
  const pending = countMap(jobs, ["pending", "queued"]);
  const active = countMap(jobs, ["processing", "running"]);
  const failed = countMap(jobs, ["failed", "error"]);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Queue"
        description="Hàng đợi chuyển mã FFmpeg (Asynq) và trạng thái HLS."
      />
      <div className="grid gap-4 sm:grid-cols-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-3xl font-bold tabular-nums">
              {queue.data?.queue_depth ?? "—"}
            </CardTitle>
            <CardDescription>Độ sâu queue Redis</CardDescription>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-3xl font-bold tabular-nums">{pending}</CardTitle>
            <CardDescription>Job chờ</CardDescription>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-3xl font-bold tabular-nums">{active}</CardTitle>
            <CardDescription>Đang chạy</CardDescription>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-3xl font-bold tabular-nums">{failed}</CardTitle>
            <CardDescription>Thất bại</CardDescription>
          </CardHeader>
        </Card>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Video HLS</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground space-y-1">
          {Object.keys(hls).length === 0 ? (
            <p>Chưa có dữ liệu.</p>
          ) : (
            Object.entries(hls).map(([k, v]) => (
              <p key={k}>
                {k}: <span className="font-medium text-foreground">{v}</span>
              </p>
            ))
          )}
        </CardContent>
      </Card>
      {analytics.data && Object.keys(analytics.data).length > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>Stream analytics</CardTitle>
            <CardDescription>Redis counters (playlist access)</CardDescription>
          </CardHeader>
          <CardContent className="text-sm space-y-1">
            {Object.entries(analytics.data).map(([k, v]) => (
              <p key={k}>
                {k}: <span className="font-medium">{String(v)}</span>
              </p>
            ))}
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}

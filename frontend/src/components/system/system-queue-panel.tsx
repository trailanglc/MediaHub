"use client";

import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  fetchQueueStatus,
  fetchStreamAnalytics,
  retryConvertVideo,
  VIDEOS_QUERY_KEY,
} from "@/lib/api/api-client";
import { PageError, PageLoading } from "@/components/feedback/page-states";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { PageHeader } from "@/components/ui/page-header";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { Loader2Icon } from "lucide-react";

function countMap(m?: Record<string, number>, keys?: string[]) {
  if (!m) return 0;
  if (!keys) {
    return Object.values(m).reduce((a, b) => a + b, 0);
  }
  return keys.reduce((a, k) => a + (m[k] ?? 0), 0);
}

export function SystemQueuePanel() {
  const qc = useQueryClient();
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

  const retryMut = useMutation({
    mutationFn: (videoPublicId: string) => retryConvertVideo(videoPublicId),
    onSuccess: () => {
      toast.success("Đã thử lại chuyển mã");
      void qc.invalidateQueries({ queryKey: ["system-queue"] });
      void qc.invalidateQueries({ queryKey: [VIDEOS_QUERY_KEY] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  if (queue.isLoading) return <PageLoading />;
  if (queue.isError) return <PageError onRetry={() => queue.refetch()} />;

  const jobs = queue.data?.convert_jobs ?? {};
  const hls = queue.data?.video_hls ?? {};
  const failedJobs = queue.data?.failed_jobs ?? [];
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

      {failedJobs.length > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>Job convert thất bại</CardTitle>
            <CardDescription>
              Thử lại tạo job mới khi video ở trạng thái failed.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {failedJobs.map((job) => (
              <div
                key={job.job_public_id}
                className="flex flex-col gap-2 rounded-lg border p-3 sm:flex-row sm:items-center sm:justify-between"
              >
                <div className="min-w-0 space-y-1 text-sm">
                  <p className="font-medium truncate">
                    <Link
                      href={`/videos/${job.video_public_id}`}
                      className="hover:underline"
                    >
                      {job.video_name}
                    </Link>
                  </p>
                  <p className="text-muted-foreground text-xs">
                    Job {job.job_public_id.slice(0, 8)}… · {job.attempts}/
                    {job.max_attempts} lần
                    {job.finished_at ? ` · ${job.finished_at}` : ""}
                  </p>
                  {job.error ? (
                    <p className="text-destructive text-xs line-clamp-2">{job.error}</p>
                  ) : null}
                </div>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={retryMut.isPending}
                  onClick={() => retryMut.mutate(job.video_public_id)}
                >
                  {retryMut.isPending && (
                    <Loader2Icon className="mr-2 size-4 animate-spin" />
                  )}
                  Thử lại
                </Button>
              </div>
            ))}
          </CardContent>
        </Card>
      ) : null}

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

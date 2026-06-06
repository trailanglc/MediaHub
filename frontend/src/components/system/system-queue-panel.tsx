"use client";

import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  cancelConvertVideo,
  deleteFailedConvertJob,
  fetchQueueStatus,
  fetchStreamAnalytics,
  pauseConvertQueue,
  resumeConvertQueue,
  retryConvertVideo,
  SYSTEM_OVERVIEW_QUERY_KEY,
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
import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import { StreamAnalyticsSection } from "@/components/system/stream-analytics-section";
import { toast } from "sonner";
import { Loader2Icon, Trash2Icon } from "lucide-react";

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
      void qc.invalidateQueries({ queryKey: SYSTEM_OVERVIEW_QUERY_KEY });
      void qc.invalidateQueries({ queryKey: [VIDEOS_QUERY_KEY] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const deleteMut = useMutation({
    mutationFn: (jobPublicId: string) => deleteFailedConvertJob(jobPublicId),
    onSuccess: () => {
      toast.success("Đã xóa bản ghi job thất bại");
      void qc.invalidateQueries({ queryKey: ["system-queue"] });
      void qc.invalidateQueries({ queryKey: SYSTEM_OVERVIEW_QUERY_KEY });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const cancelMut = useMutation({
    mutationFn: (videoPublicId: string) => cancelConvertVideo(videoPublicId),
    onSuccess: () => {
      toast.success("Đã hủy job convert");
      void qc.invalidateQueries({ queryKey: ["system-queue"] });
      void qc.invalidateQueries({ queryKey: SYSTEM_OVERVIEW_QUERY_KEY });
      void qc.invalidateQueries({ queryKey: [VIDEOS_QUERY_KEY] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const pauseMut = useMutation({
    mutationFn: pauseConvertQueue,
    onSuccess: () => {
      toast.success("Đã tạm dừng queue convert");
      void qc.invalidateQueries({ queryKey: ["system-queue"] });
      void qc.invalidateQueries({ queryKey: SYSTEM_OVERVIEW_QUERY_KEY });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const resumeMut = useMutation({
    mutationFn: resumeConvertQueue,
    onSuccess: () => {
      toast.success("Đã tiếp tục queue convert");
      void qc.invalidateQueries({ queryKey: ["system-queue"] });
      void qc.invalidateQueries({ queryKey: SYSTEM_OVERVIEW_QUERY_KEY });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  if (queue.isLoading) return <PageLoading />;
  if (queue.isError) return <PageError onRetry={() => queue.refetch()} />;

  const jobs = queue.data?.convert_jobs ?? {};
  const hls = queue.data?.video_hls ?? {};
  const failedJobs = queue.data?.failed_jobs ?? [];
  const runningJobs = queue.data?.running_jobs ?? [];
  const pending = countMap(jobs, ["pending", "queued"]);
  const active = countMap(jobs, ["processing", "running"]);
  const failed = countMap(jobs, ["failed", "error"]);
  const queueDepth = queue.data?.queue_depth ?? 0;
  const queueMax = queue.data?.queue_max_depth ?? 0;
  const queuePaused = queue.data?.queue_paused ?? false;
  const depthPercent =
    queueMax > 0 ? Math.min(100, (queueDepth / queueMax) * 100) : null;
  const deletionJobs = queue.data?.storage_deletion_jobs ?? {};

  return (
    <div className="space-y-6">
      <PageHeader
        title="Queue"
        description="Hàng đợi chuyển mã FFmpeg (Asynq) và trạng thái HLS."
        actions={
          <div className="flex flex-wrap items-center gap-2">
            {queuePaused ? <Badge variant="secondary">Paused</Badge> : null}
            {queuePaused ? (
              <Button
                size="sm"
                variant="outline"
                disabled={resumeMut.isPending}
                onClick={() => resumeMut.mutate()}
              >
                Tiếp tục queue
              </Button>
            ) : (
              <Button
                size="sm"
                variant="outline"
                disabled={pauseMut.isPending}
                onClick={() => pauseMut.mutate()}
              >
                Tạm dừng queue
              </Button>
            )}
          </div>
        }
      />
      <div className="grid gap-4 sm:grid-cols-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-3xl font-bold tabular-nums">{queueDepth}</CardTitle>
            <CardDescription>
              Độ sâu queue Redis
              {queueMax > 0 ? ` / ${queueMax}` : ""}
            </CardDescription>
          </CardHeader>
          {depthPercent != null ? (
            <CardContent className="pt-0">
              <Progress value={depthPercent} />
            </CardContent>
          ) : null}
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

      <StreamAnalyticsSection data={analytics.data} />

      {runningJobs.length > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>Job đang chạy / chờ</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {runningJobs.map((job) => (
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
                    {job.status} · Job {job.job_public_id.slice(0, 8)}… · {job.attempts}/
                    {job.max_attempts} lần
                    {job.started_at ? ` · ${job.started_at}` : ""}
                  </p>
                </div>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={cancelMut.isPending}
                  onClick={() => cancelMut.mutate(job.video_public_id)}
                >
                  {cancelMut.isPending && (
                    <Loader2Icon className="mr-2 size-4 animate-spin" />
                  )}
                  Hủy
                </Button>
              </div>
            ))}
          </CardContent>
        </Card>
      ) : null}

      {failedJobs.length > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>Job convert thất bại</CardTitle>
            <CardDescription>
              Thử lại convert hoặc xóa bản ghi job khỏi danh sách (không xóa video).
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
                <div className="flex shrink-0 flex-wrap gap-2">
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
                  <Button
                    size="sm"
                    variant="ghost"
                    className="text-destructive hover:text-destructive"
                    disabled={deleteMut.isPending}
                    onClick={() => deleteMut.mutate(job.job_public_id)}
                  >
                    {deleteMut.isPending ? (
                      <Loader2Icon className="mr-2 size-4 animate-spin" />
                    ) : (
                      <Trash2Icon className="mr-2 size-4" />
                    )}
                    Xóa
                  </Button>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      ) : null}

      {Object.keys(deletionJobs).length > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>Storage deletion queue</CardTitle>
            <CardDescription>Job xóa object S3 (PostgreSQL scheduler)</CardDescription>
          </CardHeader>
          <CardContent className="text-sm text-muted-foreground space-y-1">
            {Object.entries(deletionJobs).map(([k, v]) => (
              <p key={k}>
                {k}: <span className="font-medium text-foreground">{v}</span>
              </p>
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
    </div>
  );
}

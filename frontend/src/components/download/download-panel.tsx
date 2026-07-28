"use client";

import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import {
  analyzeDownload,
  cancelDownloadJob,
  createDownloadJob,
  deleteDownloadJob,
  downloadJobsStreamURL,
  fetchDownloadJobs,
  retryDownloadJob,
  DOWNLOAD_JOBS_QUERY_KEY,
  type CreateDownloadJobInput,
  type DownloadAnalyzeResult,
  type DownloadCandidate,
  type DownloadJob,
  type DownloadJobEvent,
  type ListDownloadJobsResponse,
} from "@/lib/api/api-client";
import {
  DownloadEnqueueDialog,
  type DownloadEnqueueValues,
} from "@/components/download/download-enqueue-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PageHeader } from "@/components/ui/page-header";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";
import { FilmIcon, DownloadIcon, Loader2Icon } from "lucide-react";

const stageLabel: Record<string, string> = {
  queued: "Trong hàng đợi",
  starting: "Bắt đầu",
  download: "Đang tải",
  hls_mirror: "Đang mirror HLS",
  ytdlp: "yt-dlp",
  upload: "Upload storage",
  convert_queued: "Chờ convert",
  done: "Xong",
};

const statusLabel: Record<DownloadJob["status"], string> = {
  pending: "Chờ",
  running: "Đang chạy",
  succeeded: "Thành công",
  failed: "Lỗi",
  cancelled: "Đã hủy",
};

function formatBytes(n: number) {
  if (!n || n <= 0) return "";
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

/** Bytes/sec → human label (e.g. 12.4 MB/s). */
function formatSpeed(bps: number) {
  if (!bps || bps <= 0) return "";
  if (bps < 1024) return `${Math.round(bps)} B/s`;
  if (bps < 1024 * 1024) return `${(bps / 1024).toFixed(1)} KB/s`;
  if (bps < 1024 * 1024 * 1024) return `${(bps / (1024 * 1024)).toFixed(1)} MB/s`;
  return `${(bps / (1024 * 1024 * 1024)).toFixed(2)} GB/s`;
}

function formatEta(seconds: number) {
  if (!Number.isFinite(seconds) || seconds <= 0) return "";
  const s = Math.round(seconds);
  if (s < 60) return `~${s}s`;
  const m = Math.floor(s / 60);
  const rem = s % 60;
  if (m < 60) return rem > 0 ? `~${m}m ${rem}s` : `~${m}m`;
  const h = Math.floor(m / 60);
  const rm = m % 60;
  return rm > 0 ? `~${h}h ${rm}m` : `~${h}h`;
}

/** Estimate live download speed from SSE byte progress. */
function useDownloadSpeed(bytesDone: number, active: boolean) {
  const prev = useRef<{ bytes: number; t: number } | null>(null);
  const [speed, setSpeed] = useState(0);

  useEffect(() => {
    if (!active) {
      prev.current = null;
      setSpeed(0);
      return;
    }
    const now = performance.now();
    const sample = prev.current;
    if (sample && bytesDone >= sample.bytes) {
      const dt = (now - sample.t) / 1000;
      if (dt >= 0.4) {
        const instant = (bytesDone - sample.bytes) / dt;
        setSpeed((prevSpeed) =>
          prevSpeed > 0 ? prevSpeed * 0.55 + instant * 0.45 : instant,
        );
        prev.current = { bytes: bytesDone, t: now };
      }
    } else {
      prev.current = { bytes: bytesDone, t: now };
      if (bytesDone === 0) setSpeed(0);
    }
  }, [bytesDone, active]);

  return speed;
}

function candidateLabel(c: DownloadCandidate) {
  const parts: string[] = [];
  if (c.height) parts.push(`${c.height}p`);
  if (c.ext) parts.push(c.ext);
  if (c.is_hls) parts.push("HLS");
  if (c.format_note) parts.push(c.format_note);
  if (c.filesize) parts.push(formatBytes(c.filesize));
  return parts.join(" · ") || c.id;
}

function statusRank(s: DownloadJob["status"] | string) {
  switch (s) {
    case "pending":
      return 1;
    case "running":
      return 2;
    case "succeeded":
    case "failed":
    case "cancelled":
      return 3;
    default:
      return 0;
  }
}

/** Newest first — stable across progress SSE updates. */
function sortJobsNewestFirst(jobs: DownloadJob[]): DownloadJob[] {
  return [...jobs].sort((a, b) => {
    const ta = Date.parse(a.created_at) || 0;
    const tb = Date.parse(b.created_at) || 0;
    if (tb !== ta) return tb - ta;
    return b.public_id.localeCompare(a.public_id);
  });
}

function mergeJobsPreferFresher(
  base: DownloadJob[],
  incoming: DownloadJob[],
): DownloadJob[] {
  const byID = new Map(base.map((j) => [j.public_id, j]));
  for (const next of incoming) {
    const prev = byID.get(next.public_id);
    if (!prev) {
      byID.set(next.public_id, next);
      continue;
    }
    const preferNextStatus = statusRank(next.status) >= statusRank(prev.status);
    const status = preferNextStatus ? next.status : prev.status;
    const active = status === "pending" || status === "running";
    if (active) {
      byID.set(next.public_id, {
        ...prev,
        ...next,
        status,
        progress_pct: Math.max(prev.progress_pct, next.progress_pct),
        bytes_done: Math.max(prev.bytes_done, next.bytes_done),
        bytes_total: Math.max(prev.bytes_total, next.bytes_total),
        progress_stage: next.progress_stage || prev.progress_stage,
        source_url: next.source_url || prev.source_url,
        kind: next.kind || prev.kind,
        // Keep original created_at so list order stays fixed.
        created_at: prev.created_at || next.created_at,
      });
    } else {
      byID.set(next.public_id, {
        ...prev,
        ...(preferNextStatus ? next : prev),
        status,
        created_at: prev.created_at || next.created_at,
      });
    }
  }
  return sortJobsNewestFirst([...byID.values()]);
}

function applyJobEvent(
  prev: ListDownloadJobsResponse | undefined,
  ev: DownloadJobEvent,
): ListDownloadJobsResponse | undefined {
  if (!prev) {
    if (ev.status === "deleted") return prev;
    return {
      items: [
        {
          public_id: ev.public_id,
          source_url: ev.source_url ?? "",
          kind: ev.kind ?? "direct",
          status: (ev.status as DownloadJob["status"]) || "pending",
          title: ev.title ?? undefined,
          thumbnail_url: ev.thumbnail_url ?? undefined,
          progress_pct: ev.progress_pct ?? 0,
          progress_stage: ev.progress_stage,
          bytes_done: ev.bytes_done ?? 0,
          bytes_total: ev.bytes_total ?? 0,
          video_public_id: ev.video_public_id ?? undefined,
          last_error: ev.last_error ?? undefined,
          created_at: ev.created_at ?? new Date().toISOString(),
          updated_at: ev.updated_at ?? new Date().toISOString(),
        },
      ],
    };
  }
  if (ev.status === "deleted") {
    return {
      ...prev,
      items: prev.items.filter((j) => j.public_id !== ev.public_id),
    };
  }
  const next: DownloadJob = {
    public_id: ev.public_id,
    source_url: ev.source_url ?? "",
    kind: ev.kind ?? "direct",
    status: (ev.status as DownloadJob["status"]) || "pending",
    title: ev.title ?? undefined,
    thumbnail_url: ev.thumbnail_url ?? undefined,
    progress_pct: ev.progress_pct ?? 0,
    progress_stage: ev.progress_stage,
    bytes_done: ev.bytes_done ?? 0,
    bytes_total: ev.bytes_total ?? 0,
    video_public_id: ev.video_public_id ?? undefined,
    last_error: ev.last_error ?? undefined,
    created_at: ev.created_at ?? new Date().toISOString(),
    updated_at: ev.updated_at ?? new Date().toISOString(),
  };
  return {
    ...prev,
    items: mergeJobsPreferFresher(prev.items, [next]),
  };
}

export function DownloadPanel() {
  const qc = useQueryClient();
  const [url, setUrl] = useState("");
  const [analyze, setAnalyze] = useState<DownloadAnalyzeResult | null>(null);
  const [enqueueDraft, setEnqueueDraft] = useState<{
    defaultTitle: string;
    subtitle?: string;
    input: Omit<CreateDownloadJobInput, "title" | "parent_public_id">;
  } | null>(null);

  const jobsQuery = useQuery({
    queryKey: [DOWNLOAD_JOBS_QUERY_KEY],
    queryFn: async () => fetchDownloadJobs({ limit: "30" }),
    // Live updates via SSE — download runs on worker; reload only reattaches UI.
    refetchOnWindowFocus: true,
    staleTime: 5_000,
  });

  useEffect(() => {
    let closed = false;
    let es: EventSource | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let attempt = 0;

    const clearReconnect = () => {
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
    };

    const connect = () => {
      if (closed) return;
      clearReconnect();
      es?.close();
      es = new EventSource(downloadJobsStreamURL(), {
        withCredentials: true,
      });

      es.addEventListener("snapshot", (raw) => {
        attempt = 0;
        try {
          const data = JSON.parse((raw as MessageEvent).data) as {
            items?: DownloadJob[];
          };
          const incoming = data.items ?? [];
          // Server snapshot is authoritative for the page list.
          qc.setQueryData<ListDownloadJobsResponse>(
            [DOWNLOAD_JOBS_QUERY_KEY],
            (old) => ({
              items: incoming,
              next_cursor: old?.next_cursor,
            }),
          );
        } catch {
          /* ignore malformed */
        }
      });

      es.addEventListener("job", (raw) => {
        attempt = 0;
        try {
          const ev = JSON.parse(
            (raw as MessageEvent).data,
          ) as DownloadJobEvent;
          qc.setQueryData<ListDownloadJobsResponse>(
            [DOWNLOAD_JOBS_QUERY_KEY],
            (old) => applyJobEvent(old, ev),
          );
        } catch {
          /* ignore malformed */
        }
      });

      es.onerror = () => {
        if (closed) return;
        // Stop browser auto-reconnect storm; reconnect with backoff ourselves.
        es?.close();
        es = null;
        attempt += 1;
        if (attempt > 12) {
          return;
        }
        const delay = Math.min(30_000, 1000 * 2 ** Math.min(attempt - 1, 5));
        reconnectTimer = setTimeout(connect, delay);
      };
    };

    connect();

    return () => {
      closed = true;
      clearReconnect();
      es?.close();
    };
  }, [qc]);

  const analyzeMut = useMutation({
    mutationFn: () => analyzeDownload(url.trim()),
    onSuccess: (data) => {
      setAnalyze(data);
      if (data.kind === "direct" || data.kind === "hls") {
        toast.success(
          data.kind === "hls"
            ? "Phát hiện HLS — giữ nguyên stream khi tải"
            : "Phát hiện file trực tiếp",
        );
      } else if (!data.candidates?.length) {
        toast.error("Không tìm thấy video trên trang");
      }
    },
    onError: (err: Error) => toast.error(err.message || "Phân tích thất bại"),
  });

  const createMut = useMutation({
    mutationFn: createDownloadJob,
    onSuccess: () => {
      toast.success("Đã thêm vào hàng đợi tải");
      setAnalyze(null);
      setEnqueueDraft(null);
      void qc.invalidateQueries({ queryKey: [DOWNLOAD_JOBS_QUERY_KEY] });
    },
    onError: (err: Error) => toast.error(err.message || "Không thể tạo job"),
  });

  const cancelMut = useMutation({
    mutationFn: cancelDownloadJob,
    onSuccess: () => {
      toast.message("Đã hủy job");
      void qc.invalidateQueries({ queryKey: [DOWNLOAD_JOBS_QUERY_KEY] });
    },
    onError: (err: Error) => toast.error(err.message),
  });

  const retryMut = useMutation({
    mutationFn: retryDownloadJob,
    onSuccess: () => {
      toast.success("Đã đưa lại vào hàng đợi");
      void qc.invalidateQueries({ queryKey: [DOWNLOAD_JOBS_QUERY_KEY] });
    },
    onError: (err: Error) => toast.error(err.message),
  });

  const deleteMut = useMutation({
    mutationFn: deleteDownloadJob,
    onSuccess: () => {
      toast.message("Đã xóa job");
      void qc.invalidateQueries({ queryKey: [DOWNLOAD_JOBS_QUERY_KEY] });
    },
    onError: (err: Error) => toast.error(err.message),
  });

  function openEnqueueDirect() {
    if (!analyze) return;
    setEnqueueDraft({
      defaultTitle: analyze.filename || analyze.title || "video",
      subtitle:
        analyze.kind === "hls" ? "Stream HLS sẽ được mirror." : undefined,
      input: {
        url: url.trim(),
        resolved_url: analyze.final_url,
        kind: analyze.kind,
      },
    });
  }

  function openEnqueueCandidate(c: DownloadCandidate) {
    setEnqueueDraft({
      defaultTitle: c.title || analyze?.title || "video",
      subtitle: candidateLabel(c),
      input: {
        url: url.trim(),
        resolved_url: c.url,
        kind: c.is_hls ? "hls" : "ytdlp",
        format_id: c.id,
        thumbnail_url: c.thumbnail,
        is_hls: c.is_hls,
        ext: c.ext,
        candidate: c,
      },
    });
  }

  function confirmEnqueue(values: DownloadEnqueueValues) {
    if (!enqueueDraft) return;
    createMut.mutate({
      ...enqueueDraft.input,
      title: values.title,
      parent_public_id: values.parentPublicId,
    });
  }

  const jobs = jobsQuery.data?.items ?? [];

  return (
    <div className="space-y-8">
      <PageHeader
        title="Download"
        description="Dán link file, HLS (.m3u8) hoặc trang web chứa video sẽ được phân tích và đưa vào hàng đợi tải."
      />

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Nhập URL</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="download-url">Link</Label>
            <div className="flex flex-col gap-2 sm:flex-row">
              <Input
                id="download-url"
                placeholder="https://…"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && url.trim()) analyzeMut.mutate();
                }}
              />
              <Button
                disabled={!url.trim() || analyzeMut.isPending}
                onClick={() => analyzeMut.mutate()}
              >
                {analyzeMut.isPending ? (
                  <Loader2Icon className="size-4 animate-spin" />
                ) : (
                  "Phân tích"
                )}
              </Button>
            </div>
          </div>

          {analyze && (analyze.kind === "direct" || analyze.kind === "hls") && (
            <div className="flex flex-wrap items-center gap-3 rounded-md border p-3">
              <Badge variant="secondary">
                {analyze.kind === "hls" ? "HLS stream" : "File trực tiếp"}
              </Badge>
              <span className="text-sm text-muted-foreground">
                {analyze.filename || analyze.final_url}
              </span>
              <Button
                size="sm"
                className="ml-auto"
                disabled={createMut.isPending}
                onClick={openEnqueueDirect}
              >
                <DownloadIcon className="size-4" />
                Tải xuống
              </Button>
            </div>
          )}

          {analyze?.kind === "website" && (
            <div className="space-y-3">
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <span>Nguồn phân tích:</span>
                <Badge variant="outline">{analyze.source ?? "website"}</Badge>
                {analyze.title && (
                  <span className="font-medium text-foreground">
                    {analyze.title}
                  </span>
                )}
              </div>
              {!analyze.candidates?.length ? (
                <p className="text-sm text-muted-foreground">
                  Không có ứng viên video.
                </p>
              ) : (
                <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                  {analyze.candidates.map((c) => (
                    <button
                      key={`${c.id}-${c.url}`}
                      type="button"
                      disabled={createMut.isPending}
                      onClick={() => openEnqueueCandidate(c)}
                      className="overflow-hidden rounded-md border text-left transition-shadow hover:shadow-md disabled:opacity-60"
                    >
                      <div className="relative aspect-video bg-muted">
                        {c.thumbnail ? (
                          // eslint-disable-next-line @next/next/no-img-element
                          <img
                            src={c.thumbnail}
                            alt=""
                            className="size-full object-cover"
                          />
                        ) : (
                          <div className="flex size-full items-center justify-center text-muted-foreground">
                            <FilmIcon className="size-8 opacity-40" />
                          </div>
                        )}
                        <Badge className="absolute right-2 top-2" variant="secondary">
                          {c.is_hls ? "HLS" : c.ext || "file"}
                        </Badge>
                      </div>
                      <div className="space-y-1 p-3">
                        <p className="line-clamp-2 text-sm font-medium">
                          {c.title || "Video"}
                        </p>
                        <p className="text-xs text-muted-foreground">
                          {candidateLabel(c)}
                        </p>
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold">Hàng đợi gần đây</h2>
        {jobsQuery.isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-16 w-full" />
            <Skeleton className="h-16 w-full" />
          </div>
        ) : jobs.length === 0 ? (
          <p className="text-sm text-muted-foreground">Chưa có job tải nào.</p>
        ) : (
          <ul className="space-y-3">
            {jobs.map((job) => (
              <JobRow
                key={job.public_id}
                job={job}
                onCancel={() => cancelMut.mutate(job.public_id)}
                onRetry={() => retryMut.mutate(job.public_id)}
                onDelete={() => deleteMut.mutate(job.public_id)}
                busy={
                  cancelMut.isPending ||
                  retryMut.isPending ||
                  deleteMut.isPending
                }
              />
            ))}
          </ul>
        )}
      </section>

      <DownloadEnqueueDialog
        open={enqueueDraft != null}
        defaultTitle={enqueueDraft?.defaultTitle ?? ""}
        subtitle={enqueueDraft?.subtitle}
        loading={createMut.isPending}
        onClose={() => {
          if (!createMut.isPending) setEnqueueDraft(null);
        }}
        onConfirm={confirmEnqueue}
      />
    </div>
  );
}

function JobRow({
  job,
  onCancel,
  onRetry,
  onDelete,
  busy,
}: {
  job: DownloadJob;
  onCancel: () => void;
  onRetry: () => void;
  onDelete: () => void;
  busy: boolean;
}) {
  const active = job.status === "pending" || job.status === "running";
  const transferring =
    job.status === "running" &&
    (job.progress_stage === "download" ||
      job.progress_stage === "hls_mirror" ||
      job.progress_stage === "ytdlp" ||
      job.progress_stage === "upload");
  const speed = useDownloadSpeed(job.bytes_done, transferring);
  const speedLabel = formatSpeed(speed);
  const etaLabel =
    speed > 0 && job.bytes_total > job.bytes_done
      ? formatEta((job.bytes_total - job.bytes_done) / speed)
      : "";
  const canRetry =
    job.status === "cancelled" || job.status === "failed";
  const canDelete =
    job.status === "cancelled" || job.status === "failed";
  return (
    <li className="rounded-md border p-3">
      <div className="flex flex-wrap items-start gap-2">
        <div className="min-w-0 flex-1 space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <p className="truncate text-sm font-medium">
              {job.title || job.source_url}
            </p>
            <Badge
              variant={
                job.status === "failed"
                  ? "destructive"
                  : job.status === "succeeded"
                    ? "default"
                    : "secondary"
              }
            >
              {statusLabel[job.status]}
            </Badge>
            <Badge variant="outline">{job.kind}</Badge>
          </div>
          <p className="truncate text-xs text-muted-foreground">
            {job.source_url}
          </p>
          {active && (
            <div className="space-y-1 pt-1">
              <Progress value={job.progress_pct} className="h-2" />
              <p className="text-xs text-muted-foreground">
                {stageLabel[job.progress_stage ?? ""] ??
                  job.progress_stage ??
                  "…"}{" "}
                · {job.progress_pct}%
                {job.bytes_total > 0
                  ? ` · ${formatBytes(job.bytes_done)} / ${formatBytes(job.bytes_total)}`
                  : job.bytes_done > 0
                    ? ` · ${formatBytes(job.bytes_done)}`
                    : ""}
                {speedLabel ? ` · ${speedLabel}` : ""}
                {etaLabel ? ` · còn ${etaLabel}` : ""}
              </p>
            </div>
          )}
          {job.last_error && (
            <p className="text-xs text-destructive">{job.last_error}</p>
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          {job.video_public_id && (
            <Button
              size="sm"
              variant="outline"
              nativeButton={false}
              render={<Link href={`/videos/${job.video_public_id}`} />}
            >
              Mở video
            </Button>
          )}
          {active && (
            <Button size="sm" variant="ghost" disabled={busy} onClick={onCancel}>
              Hủy
            </Button>
          )}
          {canRetry && (
            <Button size="sm" variant="outline" disabled={busy} onClick={onRetry}>
              Thử lại
            </Button>
          )}
          {canDelete && (
            <Button
              size="sm"
              variant="ghost"
              disabled={busy}
              onClick={onDelete}
            >
              Xóa
            </Button>
          )}
        </div>
      </div>
    </li>
  );
}

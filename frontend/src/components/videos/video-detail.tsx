"use client";

import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  convertVideo,
  deleteVideoHLS,
  fetchVideo,
  fetchVideoHLS,
  patchStreamPolicy,
  VIDEOS_QUERY_KEY,
  type HLSStatus,
} from "@/lib/api/api-client";
import { HlsPlayer } from "@/components/videos/hls-player";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PageHeader } from "@/components/ui/page-header";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";
import { Checkbox } from "@/components/ui/checkbox";
import {
  defaultVariantSelection,
  formatSourceBitrate,
  HLS_VARIANT_OPTIONS,
  type HLSVariantId,
  type SourceProfile,
  variantsAvailableForSource,
  whyVariantBlocked,
} from "@/lib/video/hls-variants";
import { toast } from "sonner";
import { useEffect, useState } from "react";
import { CopyIcon, Loader2Icon } from "lucide-react";

const statusLabel: Record<HLSStatus, string> = {
  none: "Chưa HLS",
  pending: "Đang chờ",
  converting: "Đang chuyển mã",
  ready: "Sẵn sàng",
  failed: "Lỗi",
  deleted: "Đã xóa HLS",
};

function convertStageLabel(stage: string): string {
  const map: Record<string, string> = {
    queued: "Đang chờ trong hàng đợi",
    starting: "Bắt đầu xử lý",
    download: "Tải file gốc",
    probe: "Đọc metadata",
    encode_1080p: "Chuyển mã 1080p",
    encode_720p: "Chuyển mã 720p",
    encode_480p: "Chuyển mã 480p",
    encode_source: "Chuyển mã",
    playlist: "Tạo playlist HLS",
    upload: "Tải lên storage",
    thumbnail: "Tạo thumbnail",
    finalize: "Hoàn tất",
    done: "Xong",
  };
  return map[stage] ?? stage.replace(/_/g, " ");
}

export function VideoDetail({ id }: { id: string }) {
  const qc = useQueryClient();
  const [domainsText, setDomainsText] = useState("");
  const [selectedVariants, setSelectedVariants] = useState<HLSVariantId[]>([]);

  const { data: video, isLoading } = useQuery({
    queryKey: [VIDEOS_QUERY_KEY, id],
    queryFn: () => fetchVideo(id),
    refetchInterval: (q) => {
      const s = q.state.data?.hls_status;
      return s === "pending" || s === "converting" ? 3000 : false;
    },
    refetchOnWindowFocus: (q) => {
      const s = q.state.data?.hls_status;
      return s === "pending" || s === "converting";
    },
    staleTime: (q) => {
      const s = q.state.data?.hls_status;
      if (s === "pending" || s === "converting") return 0;
      if (s === "ready") return 5 * 60_000;
      return 30_000;
    },
  });

  const hlsReady = video?.hls_status === "ready";
  const { data: hlsAccess } = useQuery({
    queryKey: [VIDEOS_QUERY_KEY, id, "hls"],
    queryFn: () => fetchVideoHLS(id),
    enabled: hlsReady && !!video?.capabilities.stream,
    staleTime: Infinity,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });

  const convertMut = useMutation({
    mutationFn: () => convertVideo(id, { variants: [...selectedVariants] }),
    onSuccess: () => {
      toast.success("Đã bắt đầu chuyển mã HLS");
      void qc.invalidateQueries({ queryKey: [VIDEOS_QUERY_KEY, id] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const deleteHlsMut = useMutation({
    mutationFn: () => deleteVideoHLS(id),
    onSuccess: () => {
      toast.success("Đã xóa bản HLS");
      void qc.invalidateQueries({ queryKey: [VIDEOS_QUERY_KEY, id] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const policyMut = useMutation({
    mutationFn: () =>
      patchStreamPolicy(id, {
        allowed_domains: domainsText
          .split(/[\n,]/)
          .map((d) => d.trim())
          .filter(Boolean),
      }),
    onSuccess: () => {
      toast.success("Đã lưu chính sách stream");
      void qc.invalidateQueries({ queryKey: [VIDEOS_QUERY_KEY, id] });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  useEffect(() => {
    if (!video?.stream_policy) return;
    setDomainsText(video.stream_policy.allowed_domains.join("\n"));
  }, [video?.public_id, video?.stream_policy?.allowed_domains]);

  useEffect(() => {
    if (!video) return;
    const src: SourceProfile = {
      height: video.height,
      bitrateBps: video.bitrate_bps,
    };
    setSelectedVariants(defaultVariantSelection(src));
  }, [video?.public_id, video?.height, video?.bitrate_bps]);

  if (isLoading || !video) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="aspect-video w-full rounded-xl" />
      </div>
    );
  }

  const isConverting =
    video.hls_status === "pending" || video.hls_status === "converting";
  const progress = video.convert_progress;
  const canConvert =
    video.capabilities.convert &&
    ["none", "failed", "deleted"].includes(video.hls_status);
  const sourceProfile: SourceProfile = {
    height: video.height,
    bitrateBps: video.bitrate_bps,
  };
  const availableVariants = variantsAvailableForSource(sourceProfile);
  const sourceQualityLabel = [
    video.height ? `${video.width ?? "?"}×${video.height}` : null,
    formatSourceBitrate(video.bitrate_bps),
  ]
    .filter(Boolean)
    .join(" · ");

  function toggleVariant(variantId: HLSVariantId, checked: boolean) {
    setSelectedVariants((prev) => {
      if (checked) {
        return prev.includes(variantId) ? prev : [...prev, variantId];
      }
      return prev.filter((v) => v !== variantId);
    });
  }

  async function copyText(text: string, label: string) {
    await navigator.clipboard.writeText(text);
    toast.success(`Đã copy ${label}`);
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title={video.name}
        description={`Trạng thái HLS: ${statusLabel[video.hls_status]}`}
        actions={
          <Badge variant={video.hls_status === "ready" ? "default" : "secondary"}>
            {statusLabel[video.hls_status]}
          </Badge>
        }
      />

      <div className="grid gap-6 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle>Player HLS</CardTitle>
            <CardDescription>
              Chỉ phát luồng HLS đã chuyển mã — không phát file gốc.
            </CardDescription>
          </CardHeader>
          <CardContent>
            {hlsReady && hlsAccess?.master_url ? (
              <HlsPlayer src={hlsAccess.master_url} />
            ) : isConverting ? (
              <div className="flex aspect-video flex-col items-center justify-center gap-4 rounded-lg bg-muted px-6 py-8">
                <Loader2Icon className="size-10 animate-spin text-primary" />
                <div className="w-full max-w-md space-y-2 text-center">
                  <p className="text-sm font-medium">
                    {progress
                      ? convertStageLabel(progress.stage)
                      : statusLabel[video.hls_status]}
                  </p>
                  <Progress
                    value={progress?.percent ?? (video.hls_status === "pending" ? 8 : 15)}
                    className="h-2"
                  />
                  <p className="text-xs text-muted-foreground tabular-nums">
                    {progress?.percent ?? 0}%
                    {video.duration_seconds
                      ? ` · ~${Math.ceil(video.duration_seconds / 60)} phút video`
                      : ""}
                  </p>
                </div>
              </div>
            ) : (
              <div className="flex aspect-video items-center justify-center rounded-lg bg-muted text-sm text-muted-foreground">
                Chưa có HLS. Nhấn Convert để tạo luồng phát.
              </div>
            )}
          </CardContent>
        </Card>

        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Hành động</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-2">
              {canConvert && (
                <>
                  <div className="space-y-2">
                    <Label>Độ phân giải HLS</Label>
                    <p className="text-xs text-muted-foreground">
                      Chọn bản theo độ phân giải nguồn. Bitrate encode tự scale theo file gốc
                      (video YouTube 1080p thường ~2 Mbps — vẫn có thể chọn 1080p).
                      {sourceQualityLabel
                        ? ` Nguồn: ${sourceQualityLabel}.`
                        : " Chưa có metadata — sẽ đo khi convert."}
                    </p>
                    <div className="flex flex-col gap-2">
                      {HLS_VARIANT_OPTIONS.map((opt) => {
                        const available = availableVariants.includes(opt.id);
                        const checked = selectedVariants.includes(opt.id);
                        const blocked = whyVariantBlocked(opt.id, sourceProfile);
                        return (
                          <label
                            key={opt.id}
                            className="flex cursor-pointer items-center gap-2 text-sm"
                          >
                            <Checkbox
                              checked={checked}
                              disabled={!available}
                              onChange={(e) =>
                                toggleVariant(opt.id, e.target.checked)
                              }
                            />
                            <span className={!available ? "text-muted-foreground" : ""}>
                              {opt.label}
                              {!available && blocked === "resolution"
                                ? " — vượt độ phân giải nguồn"
                                : ""}
                            </span>
                          </label>
                        );
                      })}
                    </div>
                  </div>
                  <Button
                    onClick={() => convertMut.mutate()}
                    disabled={
                      convertMut.isPending || selectedVariants.length === 0
                    }
                  >
                    {convertMut.isPending && (
                      <Loader2Icon className="mr-2 size-4 animate-spin" />
                    )}
                    Convert sang HLS
                    {selectedVariants.length > 0
                      ? ` (${selectedVariants.join(", ")})`
                      : ""}
                  </Button>
                </>
              )}
              {hlsReady && hlsAccess && (
                <>
                  <Button
                    variant="outline"
                    onClick={() => copyText(hlsAccess.master_url, "URL HLS")}
                  >
                    <CopyIcon className="mr-2 size-4" />
                    Copy URL HLS
                  </Button>
                  <Button
                    variant="outline"
                    onClick={() => copyText(hlsAccess.embed_html, "embed")}
                  >
                    <CopyIcon className="mr-2 size-4" />
                    Copy embed
                  </Button>
                </>
              )}
              {hlsReady && video.capabilities.delete && (
                <Button
                  variant="destructive"
                  onClick={() => deleteHlsMut.mutate()}
                  disabled={deleteHlsMut.isPending}
                >
                  Xóa HLS (giữ file gốc)
                </Button>
              )}
              <Link
                href="/files"
                className="inline-flex h-9 items-center justify-center rounded-lg px-3 text-sm text-muted-foreground hover:bg-muted hover:text-foreground"
              >
                Mở File Manager
              </Link>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">Metadata</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <div>
                <span className="text-muted-foreground">Kích thước: </span>
                {(video.size_bytes / (1024 * 1024)).toFixed(2)} MB
              </div>
              {video.duration_seconds != null && (
                <div>
                  <span className="text-muted-foreground">Thời lượng: </span>
                  {video.duration_seconds}s
                </div>
              )}
              {video.width && video.height && (
                <div>
                  <span className="text-muted-foreground">Độ phân giải: </span>
                  {video.width}×{video.height}
                </div>
              )}
              {video.last_error && (
                <p className="text-destructive">{video.last_error}</p>
              )}
              {isConverting && progress && (
                <div className="space-y-1">
                  <p className="text-muted-foreground">Tiến độ</p>
                  <Progress value={progress.percent} className="h-1.5" />
                  <p className="text-xs text-muted-foreground">
                    {convertStageLabel(progress.stage)} ({progress.percent}%)
                  </p>
                </div>
              )}
              {video.latest_job && (
                <div className="text-muted-foreground">
                  Job: {video.latest_job.status}
                  {video.latest_job.error ? (
                    <span className="mt-1 block text-xs text-destructive break-words">
                      {video.latest_job.error}
                    </span>
                  ) : null}
                </div>
              )}
            </CardContent>
          </Card>

          {video.capabilities.update && (
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Stream policy</CardTitle>
                <CardDescription>
                  Domain được phép embed (mỗi dòng một domain). Để trống và Lưu = bỏ
                  giới hạn cho video này.
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <div>
                  <Label htmlFor="domains">Allowed domains</Label>
                  <textarea
                    id="domains"
                    className="mt-1 min-h-[80px] w-full rounded-md border bg-background px-3 py-2 text-sm"
                    value={domainsText}
                    onChange={(e) => setDomainsText(e.target.value)}
                    placeholder="example.com"
                  />
                </div>
                <div>
                  <Label>TTL token (giây)</Label>
                  <Input
                    type="number"
                    className="mt-1"
                    defaultValue={video.stream_policy?.token_ttl_seconds ?? 3600}
                    onBlur={(e) => {
                      const v = parseInt(e.target.value, 10);
                      if (!Number.isNaN(v)) {
                        void patchStreamPolicy(id, { token_ttl_seconds: v }).then(() =>
                          qc.invalidateQueries({ queryKey: [VIDEOS_QUERY_KEY, id] }),
                        );
                      }
                    }}
                  />
                </div>
                <Button
                  size="sm"
                  onClick={() => policyMut.mutate()}
                  disabled={policyMut.isPending}
                >
                  Lưu domains
                </Button>
              </CardContent>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}

"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  cleanupOrphans,
  cleanupTemp,
  fetchSystemStorage,
  SYSTEM_HEALTH_QUERY_KEY,
  type CleanupOrphansResult,
} from "@/lib/api/api-client";
import { PageError, PageLoading } from "@/components/feedback/page-states";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { PageHeader } from "@/components/ui/page-header";
import { Progress } from "@/components/ui/progress";
import { formatBytes } from "@/lib/format";
import { toast } from "@/hooks/use-app-toast";

export function SystemStoragePanel() {
  const queryClient = useQueryClient();
  const [orphanPreview, setOrphanPreview] = useState<CleanupOrphansResult | null>(null);

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ["system-storage"],
    queryFn: fetchSystemStorage,
    refetchInterval: 30_000,
  });

  const cleanupTempMut = useMutation({
    mutationFn: cleanupTemp,
    onSuccess: (r) => {
      toast.success(
        `Đã dọn temp: ${r.removed_temp_objects} object, ${r.expired_upload_sessions} session hết hạn`,
      );
      void queryClient.invalidateQueries({ queryKey: [SYSTEM_HEALTH_QUERY_KEY] });
      void refetch();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const scanOrphansMut = useMutation({
    mutationFn: () => cleanupOrphans({ dry_run: true, max_delete: 500 }),
    onSuccess: (r) => {
      setOrphanPreview(r);
      if (r.orphan_count === 0) {
        toast.success("Không tìm thấy orphan storage.");
      }
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const cleanupOrphansMut = useMutation({
    mutationFn: () => cleanupOrphans({ dry_run: false, max_delete: 500 }),
    onSuccess: (r) => {
      toast.success(`Orphan cleanup: đã xóa ${r.deleted}/${r.orphan_count}`);
      setOrphanPreview(null);
      void refetch();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  if (isLoading) return <PageLoading />;
  if (isError) return <PageError onRetry={() => refetch()} />;

  const details = data?.details ?? {};
  const used = Number(details.used_bytes);
  const total = Number(details.total_bytes);
  const percent = Number(details.used_percent);
  const showBar = Number.isFinite(used) && Number.isFinite(total) && total > 0;
  const quotaBytes = data?.quota_bytes ?? 0;
  const quotaRemaining = data?.quota_remaining_bytes;
  const quotaPercent =
    quotaBytes > 0 && Number.isFinite(used) ? Math.min(100, (used / quotaBytes) * 100) : null;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Storage"
        description="Dung lượng object storage (MinIO/S3) và công cụ dọn dẹp."
      />
      {data?.stats_partial ? (
        <Alert>
          <AlertDescription>
            Thống kê dung lượng có thể không đầy đủ (bucket lớn, fallback list object).
          </AlertDescription>
        </Alert>
      ) : null}
      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Trạng thái</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold capitalize">{data?.status ?? "—"}</p>
            <p className="text-xs text-muted-foreground mt-1">
              {data?.bucket ? `Bucket: ${data.bucket}` : null}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Đã dùng</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold tabular-nums">
              {Number.isFinite(used) ? formatBytes(used) : "—"}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Số object</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold tabular-nums">
              {details.object_count ?? "—"}
            </p>
          </CardContent>
        </Card>
      </div>
      {showBar ? (
        <Card>
          <CardHeader>
            <CardTitle>Dung lượng bucket</CardTitle>
            <CardDescription>
              {formatBytes(used)} / {formatBytes(total)} ({percent.toFixed(1)}%)
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Progress value={Math.min(100, percent)} />
          </CardContent>
        </Card>
      ) : null}
      {quotaBytes > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>Quota workspace</CardTitle>
            <CardDescription>
              {formatBytes(used)} / {formatBytes(quotaBytes)}
              {quotaRemaining != null ? ` · còn ${formatBytes(quotaRemaining)}` : null}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {quotaPercent != null ? <Progress value={quotaPercent} /> : null}
          </CardContent>
        </Card>
      ) : null}
      <Card>
        <CardHeader>
          <CardTitle>Bảo trì</CardTitle>
          <CardDescription>
            Dọn upload temp và blob MinIO không còn tham chiếu DB (giới hạn 500/lần).
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-wrap gap-2">
            <Button
              variant="secondary"
              disabled={cleanupTempMut.isPending}
              onClick={() => cleanupTempMut.mutate()}
            >
              Dọn temp upload
            </Button>
            <Button
              variant="outline"
              disabled={scanOrphansMut.isPending}
              onClick={() => scanOrphansMut.mutate()}
            >
              Quét orphan (dry-run)
            </Button>
          </div>
          {orphanPreview ? (
            <div className="rounded-lg border p-4 space-y-3 text-sm">
              <div className="flex flex-wrap items-center gap-2">
                <span>
                  Tìm thấy{" "}
                  <span className="font-semibold">{orphanPreview.orphan_count}</span> orphan
                  {orphanPreview.truncated ? " (danh sách bị cắt)" : ""}
                </span>
                {orphanPreview.truncated ? (
                  <Badge variant="secondary">truncated</Badge>
                ) : null}
              </div>
              {orphanPreview.sample_keys?.length ? (
                <ul className="text-xs text-muted-foreground space-y-1 font-mono">
                  {orphanPreview.sample_keys.slice(0, 10).map((k) => (
                    <li key={k} className="truncate">
                      {k}
                    </li>
                  ))}
                </ul>
              ) : null}
              <div className="flex flex-wrap gap-2">
                <Button
                  variant="destructive"
                  size="sm"
                  disabled={
                    cleanupOrphansMut.isPending || orphanPreview.orphan_count === 0
                  }
                  onClick={() => cleanupOrphansMut.mutate()}
                >
                  Xóa orphan ({Math.min(500, orphanPreview.orphan_count)})
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setOrphanPreview(null)}
                >
                  Hủy
                </Button>
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}

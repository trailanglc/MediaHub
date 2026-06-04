"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  cleanupOrphans,
  cleanupTemp,
  fetchSystemStorage,
  SYSTEM_HEALTH_QUERY_KEY,
} from "@/lib/api-client";
import { PageError, PageLoading } from "@/components/feedback/page-states";
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

  const cleanupOrphansMut = useMutation({
    mutationFn: () => cleanupOrphans({ dry_run: false, max_delete: 500 }),
    onSuccess: (r) => {
      toast.success(`Orphan cleanup: đã xóa ${r.deleted}/${r.orphan_count}`);
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

  return (
    <div className="space-y-6">
      <PageHeader
        title="Storage"
        description="Dung lượng object storage (MinIO/S3) và công cụ dọn dẹp."
      />
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
            <CardTitle>Dung lượng</CardTitle>
            <CardDescription>
              {formatBytes(used)} / {formatBytes(total)} ({percent.toFixed(1)}%)
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Progress value={Math.min(100, percent)} />
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
        <CardContent className="flex flex-wrap gap-2">
          <Button
            variant="secondary"
            disabled={cleanupTempMut.isPending}
            onClick={() => cleanupTempMut.mutate()}
          >
            Dọn temp upload
          </Button>
          <Button
            variant="outline"
            disabled={cleanupOrphansMut.isPending}
            onClick={() => cleanupOrphansMut.mutate()}
          >
            Dọn orphan storage
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}

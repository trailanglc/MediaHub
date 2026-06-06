"use client";

import Link from "next/link";
import type { SystemOverviewResponse } from "@/lib/api/api-client";
import { formatBytes } from "@/lib/format";
import { metricColor } from "@/lib/format";
import { Alert, AlertDescription } from "@/components/ui/alert";
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

export function DashboardStorageSummary({
  data,
  loading,
}: {
  data?: SystemOverviewResponse;
  loading: boolean;
}) {
  if (loading) {
    return <Skeleton className="h-48 rounded-xl" />;
  }

  const storage = data?.storage;
  if (!storage) {
    return (
      <Card>
        <CardContent className="py-8 text-sm text-muted-foreground">
          Không có dữ liệu storage.
        </CardContent>
      </Card>
    );
  }

  const used = Number(storage.used_bytes);
  const total = Number(storage.total_bytes);
  const percent = Number(storage.used_percent);
  const showBar = Number.isFinite(used) && Number.isFinite(total) && total > 0;
  const quotaBytes = storage.quota_bytes ?? 0;
  const quotaPercent =
    quotaBytes > 0 && Number.isFinite(used)
      ? Math.min(100, (used / quotaBytes) * 100)
      : null;

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-2">
        <div>
          <CardTitle>Object Storage</CardTitle>
          <CardDescription>MinIO/S3 và quota workspace</CardDescription>
        </div>
        <Badge variant={storage.status === "healthy" ? "secondary" : "destructive"}>
          {storage.status}
        </Badge>
      </CardHeader>
      <CardContent className="space-y-4">
        {storage.stats_partial ? (
          <Alert>
            <AlertDescription>
              Thống kê dung lượng có thể không đầy đủ.
            </AlertDescription>
          </Alert>
        ) : null}

        {showBar ? (
          <div className="space-y-2">
            <div className="flex justify-between text-sm">
              <span className="text-muted-foreground">Bucket</span>
              <span className="font-medium tabular-nums">
                {formatBytes(used)} / {formatBytes(total)}
              </span>
            </div>
            <Progress
              value={percent}
              indicatorClassName={metricColor(percent)}
            />
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">
            Đã dùng:{" "}
            {Number.isFinite(used) ? formatBytes(used) : "—"}
            {storage.object_count
              ? ` · ${storage.object_count} object`
              : ""}
          </p>
        )}

        {quotaPercent != null ? (
          <div className="space-y-2">
            <div className="flex justify-between text-sm">
              <span className="text-muted-foreground">Quota workspace</span>
              <span className="font-medium tabular-nums">
                {quotaPercent.toFixed(1)}%
              </span>
            </div>
            <Progress
              value={quotaPercent}
              indicatorClassName={metricColor(quotaPercent)}
            />
            {storage.quota_remaining_bytes != null ? (
              <p className="text-xs text-muted-foreground">
                Còn lại {formatBytes(storage.quota_remaining_bytes)}
              </p>
            ) : null}
          </div>
        ) : null}

        <Link
          href="/system/storage"
          className="text-sm font-medium underline-offset-4 hover:underline"
        >
          Mở trang Storage
        </Link>
      </CardContent>
    </Card>
  );
}

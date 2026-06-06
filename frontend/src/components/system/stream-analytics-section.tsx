"use client";

import type { StreamAnalyticsResponse } from "@/lib/api/api-client";
import { formatBytes } from "@/lib/format";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  ActivityIcon,
  HardDriveDownloadIcon,
  RadioIcon,
  TrendingUpIcon,
} from "lucide-react";

function formatCount(n: number | undefined) {
  if (n == null || !Number.isFinite(n)) return "—";
  return n.toLocaleString("vi-VN");
}

const STREAM_METRICS = [
  {
    key: "requests_today" as const,
    label: "Lượt phát hôm nay",
    hint: "Playlist .m3u8 được phát",
    icon: RadioIcon,
    format: (v: number) => formatCount(v),
  },
  {
    key: "bytes_today" as const,
    label: "Băng thông hôm nay",
    hint: "Dữ liệu segment đã phát",
    icon: ActivityIcon,
    format: (v: number) => formatBytes(v),
  },
  {
    key: "requests_total" as const,
    label: "Tổng lượt phát",
    hint: "Tích lũy từ khi bật hệ thống",
    icon: TrendingUpIcon,
    format: (v: number) => formatCount(v),
  },
  {
    key: "bytes_total" as const,
    label: "Tổng băng thông",
    hint: "Tích lũy bytes stream",
    icon: HardDriveDownloadIcon,
    format: (v: number) => formatBytes(v),
  },
] as const;

export function StreamAnalyticsGrid({
  data,
  compact = false,
}: {
  data?: StreamAnalyticsResponse;
  compact?: boolean;
}) {
  const hasAny = STREAM_METRICS.some((m) => (data?.[m.key] ?? 0) > 0);

  if (!hasAny) {
    return (
      <p className="text-sm text-muted-foreground">
        Chưa có lượt phát nào được ghi nhận. Mở video HLS ready và phát thử playlist.
      </p>
    );
  }

  return (
    <div
      className={
        compact
          ? "grid gap-3 sm:grid-cols-2"
          : "grid gap-4 sm:grid-cols-2 lg:grid-cols-4"
      }
    >
      {STREAM_METRICS.map(({ key, label, hint, icon: Icon, format }) => {
        const value = data?.[key] ?? 0;
        return (
          <div
            key={key}
            className="rounded-lg border bg-muted/30 p-4 space-y-2"
          >
            <div className="flex items-center justify-between gap-2">
              <span className="text-sm font-medium text-muted-foreground">
                {label}
              </span>
              <Icon className="size-4 text-muted-foreground shrink-0" aria-hidden />
            </div>
            <p className="text-2xl font-bold tabular-nums tracking-tight">
              {format(value)}
            </p>
            <p className="text-xs text-muted-foreground">{hint}</p>
          </div>
        );
      })}
    </div>
  );
}

export function StreamAnalyticsSection({
  data,
}: {
  data?: StreamAnalyticsResponse;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Stream analytics</CardTitle>
        <CardDescription>
          Thống kê phát HLS qua Redis
          {data?.date ? ` · ngày ${data.date}` : ""}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <StreamAnalyticsGrid data={data} />
      </CardContent>
    </Card>
  );
}

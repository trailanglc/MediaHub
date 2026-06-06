"use client";

import Link from "next/link";
import type { SystemOverviewResponse } from "@/lib/api/api-client";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  ActivityIcon,
  FileIcon,
  FilmIcon,
  HardDriveIcon,
  LayersIcon,
  RadioIcon,
} from "lucide-react";

function formatCount(n: number | undefined | null, loading: boolean) {
  if (loading) return null;
  if (n == null || !Number.isFinite(n)) return "—";
  return n.toLocaleString("vi-VN");
}

function formatPercent(raw: string | undefined, loading: boolean) {
  if (loading) return null;
  if (!raw) return "—";
  const n = Number(raw);
  return Number.isFinite(n) ? `${n.toFixed(1)}%` : "—";
}

const KPI_ITEMS = [
  {
    key: "files",
    title: "Files",
    href: "/files",
    linkLabel: "Mở File Manager",
    icon: FileIcon,
  },
  {
    key: "videos",
    title: "Videos",
    href: "/videos",
    linkLabel: "Xem danh sách video",
    icon: FilmIcon,
  },
  {
    key: "hls",
    title: "HLS ready",
    href: "/videos",
    linkLabel: "Video HLS sẵn sàng",
    icon: RadioIcon,
  },
  {
    key: "queue",
    title: "Queue depth",
    href: "/system/queue",
    linkLabel: "Chi tiết queue",
    icon: LayersIcon,
  },
  {
    key: "storage",
    title: "Storage",
    href: "/system/storage",
    linkLabel: "Chi tiết storage",
    icon: HardDriveIcon,
  },
  {
    key: "stream",
    title: "Lượt phát hôm nay",
    href: "/system/queue",
    linkLabel: "Stream analytics",
    icon: ActivityIcon,
  },
] as const;

export function DashboardKpiGrid({
  data,
  loading,
}: {
  data?: SystemOverviewResponse;
  loading: boolean;
}) {
  const values: Record<(typeof KPI_ITEMS)[number]["key"], string | null> = {
    files: formatCount(data?.content?.media_objects?.files, loading),
    videos: formatCount(data?.content?.videos_active, loading),
    hls: formatCount(data?.content?.video_hls?.ready, loading),
    queue: loading
      ? null
      : data?.queue != null
        ? `${data.queue.depth}${data.queue.max_depth ? ` / ${data.queue.max_depth}` : ""}`
        : "—",
    storage: formatPercent(data?.storage?.used_percent, loading),
    stream: formatCount(data?.stream?.requests_today, loading),
  };

  return (
    <section aria-label="Tóm tắt nhanh">
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
        {KPI_ITEMS.map(({ key, title, href, linkLabel, icon: Icon }) => (
          <Card key={key} size="sm">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">{title}</CardTitle>
              <Icon className="size-4 text-muted-foreground" aria-hidden />
            </CardHeader>
            <CardContent className="pt-0">
              {loading ? (
                <Skeleton className="h-8 w-20" />
              ) : (
                <p className="text-2xl font-bold tabular-nums">{values[key]}</p>
              )}
              <CardDescription className="mt-1.5">
                <Link
                  href={href}
                  className="font-medium text-foreground underline-offset-4 hover:underline"
                >
                  {linkLabel}
                </Link>
              </CardDescription>
            </CardContent>
          </Card>
        ))}
      </div>
    </section>
  );
}

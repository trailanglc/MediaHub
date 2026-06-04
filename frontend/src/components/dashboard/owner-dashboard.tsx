"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { fetchQueueStatus } from "@/lib/api/api-client";
import { SystemHealthDashboard } from "@/components/system/system-health-dashboard";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { PageHeader } from "@/components/ui/page-header";
import { Separator } from "@/components/ui/separator";
import { FileIcon, FilmIcon, RadioIcon } from "lucide-react";

function sumCounts(m?: Record<string, number>): number {
  if (!m) return 0;
  return Object.values(m).reduce((a, b) => a + b, 0);
}

const statCards: {
  title: string;
  href: string;
  linkLabel: string;
  icon: typeof FileIcon;
}[] = [
  {
    title: "Files",
    href: "/files",
    linkLabel: "Mở File Manager",
    icon: FileIcon,
  },
  {
    title: "Videos",
    href: "/videos",
    linkLabel: "Xem danh sách video",
    icon: FilmIcon,
  },
  {
    title: "HLS ready",
    href: "/videos",
    linkLabel: "Xem video HLS sẵn sàng",
    icon: RadioIcon,
  },
];

export function OwnerDashboard() {
  const { data: stats } = useQuery({
    queryKey: ["dashboard-stats"],
    queryFn: fetchQueueStatus,
    staleTime: 30_000,
  });

  const fileCount = stats?.media_objects?.files ?? null;
  const totalVideos = sumCounts(stats?.video_hls);
  const hlsReady = stats?.video_hls?.ready ?? 0;

  return (
    <div className="space-y-8">
      <PageHeader
        title="Dashboard"
        description="Tổng quan hệ thống MediaHub"
      />

      <section aria-label="Tóm tắt nhanh">
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {statCards.map(({ title, href, linkLabel, icon: Icon }) => {
            const count =
              title === "Files"
                ? fileCount
                : title === "Videos"
                  ? totalVideos
                  : title === "HLS ready"
                    ? hlsReady
                    : null;
            return (
              <Card key={title} size="sm">
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">{title}</CardTitle>
                  <Icon className="size-4 text-muted-foreground" aria-hidden />
                </CardHeader>
                <CardContent className="pt-0">
                  <p className="text-2xl font-bold tabular-nums">
                    {count !== null ? count : "—"}
                  </p>
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
            );
          })}
        </div>
      </section>

      <Separator />

      <section aria-label="System health">
        <SystemHealthDashboard compact embedded />
      </section>
    </div>
  );
}

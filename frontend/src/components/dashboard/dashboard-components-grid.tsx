"use client";

import Link from "next/link";
import type { SystemOverviewResponse } from "@/lib/api/api-client";
import { statusStyles } from "@/lib/format";
import { StatusBadge } from "@/components/system/status-badge";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

const COMPONENT_LABELS: Record<string, string> = {
  database: "PostgreSQL",
  redis: "Redis",
  storage: "Object Storage (MinIO)",
  worker: "Worker",
  ffmpeg: "FFmpeg",
};

const COMPONENT_ORDER = ["database", "redis", "storage", "worker", "ffmpeg"];

export function DashboardComponentsGrid({
  data,
  loading,
}: {
  data?: SystemOverviewResponse;
  loading: boolean;
}) {
  if (loading) {
    return <Skeleton className="h-64 rounded-xl" />;
  }

  const components = data?.components ?? {};

  return (
    <Card>
      <CardHeader>
        <CardTitle>Dịch vụ</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {COMPONENT_ORDER.map((name) => {
          const comp = components[name];
          if (!comp) return null;
          const s = statusStyles(comp.status);
          return (
            <div
              key={name}
              className="flex flex-wrap items-center justify-between gap-2 rounded-lg border bg-muted/40 p-3"
            >
              <div className="flex min-w-0 items-center gap-2">
                <span className={`h-2.5 w-2.5 shrink-0 rounded-full ${s.dot}`} />
                <span className="font-medium text-sm">
                  {COMPONENT_LABELS[name] ?? name}
                </span>
              </div>
              <StatusBadge status={comp.status} />
              {comp.error ? (
                <p className="w-full text-xs text-destructive">{comp.error}</p>
              ) : null}
            </div>
          );
        })}
        <Link
          href="/system/health"
          className="inline-block text-sm font-medium underline-offset-4 hover:underline"
        >
          Chi tiết System Health
        </Link>
      </CardContent>
    </Card>
  );
}

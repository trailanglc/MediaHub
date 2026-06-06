"use client";

import type { SystemOverviewResponse } from "@/lib/api/api-client";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { AlertTriangle } from "lucide-react";

export function DashboardStatusBanner({ data }: { data: SystemOverviewResponse }) {
  const queue = data.queue;
  const queueNearFull =
    queue?.max_depth != null &&
    queue.max_depth > 0 &&
    queue.depth >= queue.max_depth * 0.9;

  const hasBusy = data.resource_limits?.system_busy;
  const warnings = data.warnings ?? [];
  const hasWarnings = warnings.length > 0;

  if (!hasBusy && !queueNearFull && !hasWarnings && !data.partial) {
    return null;
  }

  return (
    <div className="space-y-3">
      {(hasBusy || queueNearFull) && (
        <Alert variant="destructive">
          <AlertTriangle className="h-4 w-4" />
          <AlertTitle>Giới hạn tải đang kích hoạt</AlertTitle>
          <AlertDescription className="text-pretty">
            {hasBusy &&
              "Hệ thống đang giảm tải (defer convert / giảm cache). "}
            {queueNearFull &&
              `Queue convert gần đầy (${queue?.depth ?? 0}/${queue?.max_depth}). `}
            Xem chi tiết tại System Health hoặc Queue.
          </AlertDescription>
        </Alert>
      )}

      {hasWarnings &&
        warnings.map((w) => (
          <Alert
            key={`${w.code}-${w.level}`}
            variant={w.level === "critical" ? "destructive" : "default"}
          >
            <AlertTriangle className="h-4 w-4" />
            <AlertTitle>{w.code}</AlertTitle>
            <AlertDescription>{w.message}</AlertDescription>
          </Alert>
        ))}

      {data.partial && data.errors && (
        <Alert>
          <AlertTriangle className="h-4 w-4" />
          <AlertTitle>Dữ liệu một phần</AlertTitle>
          <AlertDescription>
            Một số mục không tải được:{" "}
            {Object.entries(data.errors)
              .map(([k, v]) => `${k}: ${v}`)
              .join(" · ")}
          </AlertDescription>
        </Alert>
      )}
    </div>
  );
}

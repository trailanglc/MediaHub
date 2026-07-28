"use client";

import { useQuery } from "@tanstack/react-query";
import {
  fetchSystemOverview,
  SYSTEM_OVERVIEW_QUERY_KEY,
} from "@/lib/api/api-client";
import { DashboardComponentsGrid } from "@/components/dashboard/dashboard-components-grid";
import { DashboardFailedJobs } from "@/components/dashboard/dashboard-failed-jobs";
import { DashboardHostMetrics } from "@/components/dashboard/dashboard-host-metrics";
import { DashboardKpiGrid } from "@/components/dashboard/dashboard-kpi-grid";
import { DashboardQueueSummary } from "@/components/dashboard/dashboard-queue-summary";
import { DashboardQuickLinks } from "@/components/dashboard/dashboard-quick-links";
import { DashboardStatusBanner } from "@/components/dashboard/dashboard-status-banner";
import { DashboardStorageSummary } from "@/components/dashboard/dashboard-storage-summary";
import { DashboardStreamSummary } from "@/components/dashboard/dashboard-stream-summary";
import { StatusBadge } from "@/components/system/status-badge";
import { PageError } from "@/components/feedback/page-states";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/ui/page-header";
import { Separator } from "@/components/ui/separator";

const REFRESH_MS = 15_000;

export function OwnerDashboard() {
  const { data, isLoading, isError, error, dataUpdatedAt, isFetching, refetch } =
    useQuery({
      queryKey: SYSTEM_OVERVIEW_QUERY_KEY,
      queryFn: fetchSystemOverview,
      refetchInterval: () =>
        typeof document !== "undefined" &&
        document.visibilityState === "visible"
          ? REFRESH_MS
          : false,
      refetchIntervalInBackground: false,
    });

  const statusMeta = data ? (
    <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
      {isFetching && <span className="text-primary">Đang làm mới…</span>}
      <StatusBadge status={data.status} />
      <span className="tabular-nums">
        Cập nhật {new Date(dataUpdatedAt).toLocaleTimeString("vi-VN")}
      </span>
      <Button
        size="sm"
        variant="ghost"
        className="h-7 px-2"
        onClick={() => void refetch()}
      >
        Làm mới
      </Button>
    </div>
  ) : undefined;

  return (
    <div className="space-y-4 sm:space-y-8">
      <div className="sm:hidden">
        {statusMeta ?? (
          <p className="text-sm text-muted-foreground">
            Tổng quan vận hành MediaHub
          </p>
        )}
      </div>
      <div className="hidden sm:block">
        <PageHeader
          title="Dashboard"
          description="Tổng quan vận hành MediaHub"
          actions={statusMeta}
        />
      </div>

      {isError && (
        <PageError
          message={
            error instanceof Error
              ? error.message
              : "Không tải được dữ liệu dashboard"
          }
          onRetry={() => void refetch()}
        />
      )}

      {!isError && data && <DashboardStatusBanner data={data} />}

      <DashboardKpiGrid data={data} loading={isLoading} />

      <Separator className="hidden sm:block" />

      <div className="grid gap-4 sm:gap-6 lg:grid-cols-2">
        <DashboardHostMetrics data={data} loading={isLoading} />
        <DashboardStorageSummary data={data} loading={isLoading} />
      </div>

      <div className="grid gap-4 sm:gap-6 lg:grid-cols-2">
        <DashboardQueueSummary data={data} loading={isLoading} />
        <DashboardStreamSummary data={data} loading={isLoading} />
      </div>

      <div className="grid gap-4 sm:gap-6 lg:grid-cols-2">
        <DashboardFailedJobs data={data} />
        <DashboardComponentsGrid data={data} loading={isLoading} />
      </div>

      <DashboardQuickLinks />
    </div>
  );
}

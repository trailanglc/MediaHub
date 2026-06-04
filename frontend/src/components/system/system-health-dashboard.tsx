"use client";

import { useQuery } from "@tanstack/react-query";
import {
  fetchSystemHealth,
  SYSTEM_HEALTH_QUERY_KEY,
  type HealthResponse,
} from "@/lib/api-client";
import {
  formatBytes,
  formatUptime,
  metricColor,
  statusStyles,
} from "@/lib/format";
import { PageError } from "@/components/feedback/page-states";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { PageHeader } from "@/components/ui/page-header";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";

const REFRESH_MS = 15_000;

const COMPONENT_LABELS: Record<string, string> = {
  database: "PostgreSQL",
  redis: "Redis",
  storage: "Object Storage (MinIO)",
  worker: "Worker",
  ffmpeg: "FFmpeg",
};

const STORAGE_DETAIL_LABELS: Record<string, string> = {
  bucket: "Bucket",
  used_bytes: "Đã dùng",
  free_bytes: "Còn trống",
  total_bytes: "Tổng",
  used_percent: "Đã dùng %",
  object_count: "Số object",
};

function parseStorageStats(
  details?: Record<string, string>,
): {
  used: number;
  total: number;
  free: number;
  percent: number;
  objectCount: number;
  bucket?: string;
} | null {
  if (!details?.used_bytes) return null;
  const used = Number(details.used_bytes);
  if (!Number.isFinite(used)) return null;
  const total = Number(details.total_bytes) || 0;
  const free = Number(details.free_bytes) || 0;
  const percent = Number(details.used_percent);
  return {
    used,
    total,
    free,
    percent: Number.isFinite(percent)
      ? percent
      : total > 0
        ? (used / total) * 100
        : 0,
    objectCount: Number(details.object_count) || 0,
    bucket: details.bucket,
  };
}

function formatStorageDetail(key: string, value: string): string {
  if (key === "used_bytes" || key === "free_bytes" || key === "total_bytes") {
    const n = Number(value);
    return Number.isFinite(n) ? formatBytes(n) : value;
  }
  if (key === "used_percent") return `${value}%`;
  if (key === "object_count") return `${value} object`;
  return value;
}

function MetricCard({
  title,
  value,
  sub,
  percent,
}: {
  title: string;
  value: string;
  sub: string;
  percent: number;
}) {
  return (
    <Card size="sm" className="relative overflow-hidden">
      <div
        className="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-violet-500/80 via-fuchsia-500/60 to-cyan-500/80 opacity-80"
        aria-hidden
      />
      <CardHeader className="pb-0">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 pt-2">
        <p className="text-2xl font-semibold tracking-tight tabular-nums sm:text-3xl">
          {value}
        </p>
        <p className="text-xs leading-relaxed text-muted-foreground">{sub}</p>
        <Progress
          value={percent}
          indicatorClassName={metricColor(percent)}
        />
      </CardContent>
    </Card>
  );
}

function StatusBadge({ status }: { status: string }) {
  const s = statusStyles(status);
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ${s.badge}`}
    >
      <span className={`h-1.5 w-1.5 rounded-full ${s.dot}`} />
      {s.label}
    </span>
  );
}

function ComponentRow({
  name,
  comp,
}: {
  name: string;
  comp: HealthResponse["components"][string];
}) {
  const label = COMPONENT_LABELS[name] ?? name;
  const s = statusStyles(comp.status);
  const detailEntries =
    comp.details &&
    Object.entries(comp.details).filter(([k]) => k !== "note" || name !== "storage");

  return (
    <div className="rounded-lg border border-border bg-muted/40 p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <span className={`mt-1.5 h-2.5 w-2.5 shrink-0 rounded-full ${s.dot}`} />
          <div className="min-w-0">
            <p className="font-medium leading-snug">{label}</p>
            {comp.error && (
              <p className="mt-1 text-xs text-destructive">{comp.error}</p>
            )}
          </div>
        </div>
        <StatusBadge status={comp.status} />
      </div>
      {detailEntries && detailEntries.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-1.5 border-t border-border/60 pt-3">
          {detailEntries.map(([k, v]) => (
            <span
              key={k}
              className="inline-flex max-w-full rounded-md bg-background px-2 py-1 text-xs text-muted-foreground ring-1 ring-border"
            >
              <span className="truncate">
                {name === "storage" && STORAGE_DETAIL_LABELS[k]
                  ? `${STORAGE_DETAIL_LABELS[k]}: ${formatStorageDetail(k, v)}`
                  : `${k}: ${name === "storage" ? formatStorageDetail(k, v) : v}`}
              </span>
            </span>
          ))}
        </div>
      )}
    </div>
  );
}

export function SystemHealthDashboard({
  compact = false,
  embedded = false,
}: {
  compact?: boolean;
  /** Ẩn PageHeader đầy đủ khi nhúng trong Dashboard */
  embedded?: boolean;
}) {
  const { data, isLoading, error, dataUpdatedAt, isFetching, refetch } = useQuery({
    queryKey: SYSTEM_HEALTH_QUERY_KEY,
    queryFn: fetchSystemHealth,
    refetchInterval: () => {
      if (embedded) return false;
      return typeof document !== "undefined" &&
        document.visibilityState === "visible"
        ? REFRESH_MS
        : false;
    },
    refetchIntervalInBackground: false,
  });

  const overall = data?.status ?? "unknown";
  const storageStats = parseStorageStats(data?.components?.storage?.details);

  const description = embedded
    ? "Tóm tắt sức khỏe hệ thống — làm mới thủ công hoặc tại trang System Health"
    : data?.metrics_cache_ttl_seconds
      ? `Metrics cache ${data.metrics_cache_ttl_seconds}s — UI làm mới mỗi ${REFRESH_MS / 1000}s`
      : `CPU, RAM, storage và dịch vụ — làm mới mỗi ${REFRESH_MS / 1000}s`;

  const statusMeta = data ? (
    <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
      {isFetching && <span className="text-primary">Đang làm mới…</span>}
      <StatusBadge status={overall} />
      {data.metrics_cached_at && (
        <span title="Dữ liệu metrics trên server">
          Số liệu lúc{" "}
          {new Date(data.metrics_cached_at).toLocaleTimeString("vi-VN")}
        </span>
      )}
      <span className="tabular-nums">
        UI {new Date(dataUpdatedAt).toLocaleTimeString("vi-VN")}
      </span>
    </div>
  ) : null;

  return (
    <div className="space-y-6">
      {embedded ? (
        <div className="space-y-2">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 className="text-lg font-semibold tracking-tight">System health</h2>
            {statusMeta}
          </div>
          <p className="text-sm text-muted-foreground">{description}</p>
        </div>
      ) : (
        <PageHeader
          title={compact ? "System health" : "System Health"}
          description={description}
          actions={statusMeta}
        />
      )}

      {isLoading && (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-36 rounded-xl" />
          ))}
        </div>
      )}

      {error && (
        <PageError
          message={`Không kết nối được API: ${(error as Error).message}`}
          onRetry={() => void refetch()}
        />
      )}

      {data?.host && (
        <>
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <MetricCard
              title="CPU"
              value={`${data.host.cpu_percent.toFixed(1)}%`}
              sub={`${data.host.cpu_count} lõi · ${data.host.platform}`}
              percent={data.host.cpu_percent}
            />
            <MetricCard
              title="RAM"
              value={`${data.host.memory.used_percent.toFixed(1)}%`}
              sub={`${formatBytes(data.host.memory.used_bytes)} / ${formatBytes(data.host.memory.total_bytes)}`}
              percent={data.host.memory.used_percent}
            />
            <MetricCard
              title="Disk (host)"
              value={
                data.host.disks[0]
                  ? `${data.host.disks[0].used_percent.toFixed(1)}%`
                  : "—"
              }
              sub={
                data.host.disks[0]
                  ? `${data.host.disks[0].path} · ${formatBytes(data.host.disks[0].used_bytes)} / ${formatBytes(data.host.disks[0].total_bytes)}`
                  : "Không đọc được"
              }
              percent={data.host.disks[0]?.used_percent ?? 0}
            />
            {storageStats ? (
              <MetricCard
                title="Object Storage"
                value={
                  storageStats.total > 0
                    ? `${storageStats.percent.toFixed(1)}%`
                    : formatBytes(storageStats.used)
                }
                sub={
                  storageStats.total > 0
                    ? `${formatBytes(storageStats.used)} đã dùng · ${formatBytes(storageStats.free)} trống${storageStats.bucket ? ` · ${storageStats.bucket}` : ""}`
                    : `${formatBytes(storageStats.used)} đã dùng · ${storageStats.objectCount} object${storageStats.bucket ? ` · ${storageStats.bucket}` : ""}`
                }
                percent={storageStats.total > 0 ? storageStats.percent : 0}
              />
            ) : (
              <MetricCard
                title="Object Storage"
                value="—"
                sub={
                  data.components?.storage?.status === "healthy"
                    ? "Đang tải dung lượng…"
                    : "Chưa có dữ liệu"
                }
                percent={0}
              />
            )}
          </div>

          {!compact && (
            <div className="grid gap-4 lg:grid-cols-2">
              <Card size="sm">
                <CardHeader>
                  <CardTitle>Máy chủ API</CardTitle>
                </CardHeader>
                <CardContent>
                <dl className="grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
                  <dt className="text-muted-foreground">Hostname</dt>
                  <dd className="font-medium">{data.host.hostname}</dd>
                  <dt className="text-muted-foreground">Hệ điều hành</dt>
                  <dd>
                    {data.host.os} · {data.host.kernel_version}
                  </dd>
                  <dt className="text-muted-foreground">Uptime host</dt>
                  <dd>{formatUptime(data.host.uptime_seconds)}</dd>
                  <dt className="text-muted-foreground">Uptime API</dt>
                  <dd>{formatUptime(data.api_uptime_seconds)}</dd>
                  <dt className="text-muted-foreground">Go runtime</dt>
                  <dd className="font-mono text-xs">
                    {data.go_version} · {data.num_goroutine} goroutines
                  </dd>
                </dl>
                </CardContent>
              </Card>

              <Card size="sm">
                <CardHeader>
                  <CardTitle>Ổ đĩa</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  {data.host.disks.map((d) => (
                    <div key={d.path}>
                      <div className="mb-1 flex justify-between text-sm">
                        <span className="font-medium">{d.path}</span>
                        <span className="tabular-nums text-muted-foreground">
                          {formatBytes(d.free_bytes)} trống
                        </span>
                      </div>
                      <Progress
                        value={d.used_percent}
                        indicatorClassName={metricColor(d.used_percent)}
                      />
                    </div>
                  ))}
                </CardContent>
              </Card>
            </div>
          )}
        </>
      )}

      {data?.components && (
        <Card size="sm">
          <CardHeader>
            <CardTitle>Dịch vụ</CardTitle>
            <CardDescription>
              Kết nối Postgres, Redis, storage và công cụ media
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {Object.entries(data.components).map(([name, comp]) => (
              <ComponentRow key={name} name={name} comp={comp} />
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}

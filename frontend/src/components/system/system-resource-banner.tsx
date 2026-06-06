"use client";

import { useQuery } from "@tanstack/react-query";
import { AlertTriangle } from "lucide-react";
import {
  fetchSystemHealth,
  SYSTEM_HEALTH_QUERY_KEY,
  type HealthWarning,
} from "@/lib/api/api-client";
import { isOwner, useMe } from "@/hooks/use-auth";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

const BANNER_REFRESH_MS = 30_000;

function pickTopWarning(warnings: HealthWarning[] | undefined): HealthWarning | null {
  if (!warnings?.length) return null;
  const critical = warnings.find((w) => w.level === "critical");
  if (critical) return critical;
  return warnings.find((w) => w.level === "warning") ?? null;
}

export function SystemResourceBanner() {
  const { data: user } = useMe();
  const owner = isOwner(user);

  const { data } = useQuery({
    queryKey: SYSTEM_HEALTH_QUERY_KEY,
    queryFn: fetchSystemHealth,
    enabled: owner,
    refetchInterval: () =>
      typeof document !== "undefined" &&
      document.visibilityState === "visible"
        ? BANNER_REFRESH_MS
        : false,
    refetchIntervalInBackground: false,
    staleTime: 10_000,
    retry: false,
  });

  const warning = pickTopWarning(data?.warnings);
  if (!owner || !warning) {
    return null;
  }

  const ramPct = data?.host?.memory.used_percent;
  const cpuPct = data?.host?.cpu_percent;
  const detail =
    warning.code === "host_ram_high" && ramPct != null
      ? `RAM: ${ramPct.toFixed(1)}%`
      : warning.code === "host_cpu_high" && cpuPct != null
        ? `CPU: ${cpuPct.toFixed(1)}%`
        : null;

  return (
    <Alert
      variant={warning.level === "critical" ? "destructive" : "default"}
      className="rounded-none border-x-0 border-t-0"
    >
      <AlertTriangle className="h-4 w-4" />
      <AlertTitle>
        {warning.level === "critical"
          ? "Tài nguyên máy chủ quá tải"
          : "Cảnh báo tài nguyên máy chủ"}
      </AlertTitle>
      <AlertDescription className="text-pretty">
        {warning.message}
        {detail ? ` (${detail})` : ""}
      </AlertDescription>
    </Alert>
  );
}

"use client";

import type { SystemOverviewResponse } from "@/lib/api/api-client";
import { StreamAnalyticsGrid } from "@/components/system/stream-analytics-section";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

export function DashboardStreamSummary({
  data,
  loading,
}: {
  data?: SystemOverviewResponse;
  loading: boolean;
}) {
  if (loading) {
    return <Skeleton className="h-56 rounded-xl" />;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Stream analytics</CardTitle>
        <CardDescription>
          Thống kê phát HLS
          {data?.stream?.date ? ` · ngày ${data.stream.date}` : ""}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <StreamAnalyticsGrid data={data?.stream} compact />
      </CardContent>
    </Card>
  );
}

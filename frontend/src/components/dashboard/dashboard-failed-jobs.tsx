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

export function DashboardFailedJobs({
  data,
}: {
  data?: SystemOverviewResponse;
}) {
  const failedJobs = data?.queue?.failed_jobs ?? [];
  if (failedJobs.length === 0) {
    return null;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Job thất bại gần đây</CardTitle>
        <CardDescription>
          Tối đa 5 bản ghi — xem đầy đủ tại Queue
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <ul className="divide-y rounded-lg border">
          {failedJobs.map((job) => (
            <li
              key={job.job_public_id}
              className="flex flex-col gap-1 px-4 py-3 text-sm"
            >
              <Link
                href={`/videos/${job.video_public_id}`}
                className="font-medium truncate hover:underline"
              >
                {job.video_name}
              </Link>
              <p className="text-xs text-muted-foreground">
                {job.attempts}/{job.max_attempts} lần
                {job.finished_at ? ` · ${job.finished_at}` : ""}
              </p>
              {job.error ? (
                <p className="text-xs text-destructive line-clamp-2">{job.error}</p>
              ) : null}
            </li>
          ))}
        </ul>
        <Link
          href="/system/queue"
          className="text-sm font-medium underline-offset-4 hover:underline"
        >
          Xem tất cả job thất bại
        </Link>
      </CardContent>
    </Card>
  );
}

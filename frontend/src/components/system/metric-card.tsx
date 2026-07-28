"use client";

import { metricColor } from "@/lib/format";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { cn } from "@/lib/utils";

export function MetricCard({
  title,
  value,
  sub,
  percent,
  compact = false,
}: {
  title: string;
  value: string;
  sub: string;
  percent: number;
  compact?: boolean;
}) {
  return (
    <Card
      size="sm"
      className={cn(
        "relative overflow-hidden",
        compact && "gap-2 py-2.5 sm:gap-3 sm:py-3",
      )}
    >
      <div
        className="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-violet-500/80 via-fuchsia-500/60 to-cyan-500/80 opacity-80"
        aria-hidden
      />
      <CardHeader className={cn("pb-0", compact && "px-2.5 sm:px-3")}>
        <CardTitle className="truncate text-xs font-medium text-muted-foreground sm:text-sm">
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent
        className={cn(
          "space-y-3 pt-2",
          compact && "space-y-1.5 px-2.5 pt-1 sm:space-y-3 sm:px-3 sm:pt-2",
        )}
      >
        <p
          className={cn(
            "font-semibold tracking-tight tabular-nums",
            compact
              ? "text-lg sm:text-3xl"
              : "text-2xl sm:text-3xl",
          )}
        >
          {value}
        </p>
        <p
          className={cn(
            "text-xs leading-relaxed text-muted-foreground",
            compact && "line-clamp-1 sm:line-clamp-none",
          )}
        >
          {sub}
        </p>
        <Progress value={percent} indicatorClassName={metricColor(percent)} />
      </CardContent>
    </Card>
  );
}

"use client";

import { metricColor } from "@/lib/format";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";

export function MetricCard({
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
        <Progress value={percent} indicatorClassName={metricColor(percent)} />
      </CardContent>
    </Card>
  );
}

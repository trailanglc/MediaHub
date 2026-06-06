"use client";

import { statusStyles } from "@/lib/format";

export function StatusBadge({ status }: { status: string }) {
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

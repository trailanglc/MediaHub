"use client";

import { cn } from "@/lib/utils";

export function PageTransitionLoading({ className }: { className?: string }) {
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label="Đang tải trang"
      className={cn(
        "absolute inset-0 z-20 flex flex-col items-center justify-center gap-5",
        "bg-background/90 backdrop-blur-[2px]",
        "animate-in fade-in duration-200",
        className,
      )}
    >
      <div className="relative flex size-14 items-center justify-center">
        <span className="absolute inset-0 animate-ping rounded-full bg-primary/15" />
        <span className="absolute inset-1 animate-pulse rounded-full bg-primary/10" />
        <span className="relative flex size-14 items-center justify-center rounded-full border border-primary/15 bg-background shadow-sm">
          <span className="size-7 animate-spin rounded-full border-2 border-primary/20 border-t-primary" />
        </span>
      </div>

      <div className="flex flex-col items-center gap-2">
        <p className="text-sm font-medium text-foreground">Đang tải</p>
        <div className="flex items-center gap-1.5">
          {[0, 1, 2].map((i) => (
            <span
              key={i}
              className="size-1.5 rounded-full bg-primary/70 animate-bounce"
              style={{ animationDelay: `${i * 120}ms` }}
            />
          ))}
        </div>
      </div>
    </div>
  );
}

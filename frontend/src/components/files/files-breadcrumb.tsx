"use client";

import { ChevronRightIcon } from "lucide-react";
import type { BreadcrumbItem } from "@/lib/api-client";
import { cn } from "@/lib/utils";

export function FilesBreadcrumb({
  items,
  currentFolderId,
  onNavigate,
}: {
  items: BreadcrumbItem[];
  currentFolderId: string;
  onNavigate: (folderId: string) => void;
}) {
  if (items.length === 0) return null;

  const crumbs = items.filter((item, index, arr) => {
    const first = arr.findIndex((x) => x.public_id === item.public_id);
    return first === index;
  });

  return (
    <nav className="-mx-1 flex items-center gap-1 overflow-x-auto px-1 pb-1 text-sm text-muted-foreground md:mx-0 md:flex-wrap md:overflow-visible md:pb-0">
      {crumbs.map((item, i) => {
        const isLast = i === crumbs.length - 1;
        const isCurrent = item.public_id === currentFolderId;
        return (
          <span
            key={`${item.public_id}-${item.depth}`}
            className="flex shrink-0 items-center gap-1"
          >
            {i > 0 && <ChevronRightIcon className="size-3.5 shrink-0" />}
            {isLast || isCurrent ? (
              <span
                className={cn(
                  "font-medium text-foreground",
                  isCurrent && "underline-offset-4",
                )}
              >
                {item.name}
              </span>
            ) : (
              <button
                type="button"
                className="hover:text-foreground hover:underline"
                onClick={() => onNavigate(item.public_id)}
              >
                {item.name}
              </button>
            )}
          </span>
        );
      })}
    </nav>
  );
}

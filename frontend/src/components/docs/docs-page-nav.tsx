import Link from "next/link";
import { ChevronLeftIcon, ChevronRightIcon } from "lucide-react";
import { adjacentDocs, docHref, type DocPage } from "@/lib/docs/manifest";

export function DocsPageNav({ page }: { page: DocPage }) {
  const { prev, next } = adjacentDocs(page.slug);

  if (!prev && !next) return null;

  return (
    <nav
      aria-label="Điều hướng tài liệu"
      className="mt-10 grid gap-3 border-t pt-6 sm:grid-cols-2"
    >
      {prev ? (
        <Link
          href={docHref(prev.slug)}
          className="group flex flex-col rounded-lg border p-4 transition-colors hover:bg-muted/50"
        >
          <span className="flex items-center text-xs text-muted-foreground">
            <ChevronLeftIcon className="mr-1 size-3.5" />
            Trước
          </span>
          <span className="mt-1 font-medium group-hover:text-primary">
            {prev.title}
          </span>
        </Link>
      ) : (
        <div />
      )}
      {next ? (
        <Link
          href={docHref(next.slug)}
          className="group flex flex-col rounded-lg border p-4 text-right transition-colors hover:bg-muted/50 sm:col-start-2"
        >
          <span className="flex items-center justify-end text-xs text-muted-foreground">
            Tiếp
            <ChevronRightIcon className="ml-1 size-3.5" />
          </span>
          <span className="mt-1 font-medium group-hover:text-primary">
            {next.title}
          </span>
        </Link>
      ) : null}
    </nav>
  );
}

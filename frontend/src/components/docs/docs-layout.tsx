import Link from "next/link";
import { ChevronRightIcon } from "lucide-react";
import { DocsSidebar } from "@/components/docs/docs-sidebar";
import { DOC_SECTIONS } from "@/lib/docs/manifest";

function breadcrumbLabel(slug: string): string {
  if (!slug) return "Tài liệu";
  if (slug === "api") return "API Reference";
  for (const section of DOC_SECTIONS) {
    const page = section.pages.find((p) => p.slug === slug);
    if (page) return page.title;
  }
  return slug;
}

export function DocsLayout({
  slug,
  children,
}: {
  slug: string;
  children: React.ReactNode;
}) {
  const section = DOC_SECTIONS.find((s) =>
    s.pages.some((p) => p.slug === slug),
  );

  return (
    <div className="mx-auto max-w-7xl px-4 py-10 sm:px-6 sm:py-14">
      <nav
        aria-label="Breadcrumb"
        className="mb-6 flex flex-wrap items-center gap-1 text-sm text-muted-foreground"
      >
        <Link href="/docs" className="hover:text-foreground">
          Tài liệu
        </Link>
        {section && slug !== section.pages[0]?.slug && (
          <>
            <ChevronRightIcon className="size-3.5" />
            <span>{section.title}</span>
          </>
        )}
        {slug && slug !== "api" && (
          <>
            <ChevronRightIcon className="size-3.5" />
            <span className="text-foreground">{breadcrumbLabel(slug)}</span>
          </>
        )}
        {slug === "api" && (
          <>
            <ChevronRightIcon className="size-3.5" />
            <span className="text-foreground">API Reference</span>
          </>
        )}
      </nav>

      <div className="grid gap-10 lg:grid-cols-[minmax(0,15rem)_1fr]">
        <DocsSidebar className="hidden lg:block" />
        <div className="min-w-0">{children}</div>
      </div>
    </div>
  );
}

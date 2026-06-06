"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";
import {
  DOC_HUB,
  DOC_SECTIONS,
  docHref,
  type DocSection,
} from "@/lib/docs/manifest";

function SidebarLink({
  href,
  label,
  active,
}: {
  href: string;
  label: string;
  active: boolean;
}) {
  return (
    <Link
      href={href}
      className={cn(
        "block rounded-md px-2 py-1.5 text-sm transition-colors",
        active
          ? "bg-primary/10 font-medium text-primary"
          : "text-muted-foreground hover:bg-muted hover:text-foreground",
      )}
    >
      {label}
    </Link>
  );
}

function SectionBlock({ section, pathname }: { section: DocSection; pathname: string }) {
  return (
    <div className="space-y-1">
      <p className="px-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        {section.title}
      </p>
      <nav className="space-y-0.5">
        {section.pages.map((page) => {
          const href = docHref(page.slug);
          return (
            <SidebarLink
              key={page.slug}
              href={href}
              label={page.title}
              active={pathname === href}
            />
          );
        })}
      </nav>
    </div>
  );
}

export function DocsSidebar({ className }: { className?: string }) {
  const pathname = usePathname();

  return (
    <aside
      className={cn(
        "sticky top-20 max-h-[calc(100vh-6rem)] space-y-6 overflow-y-auto pb-8 pr-2",
        className,
      )}
    >
      <SidebarLink
        href={docHref(DOC_HUB.slug)}
        label={DOC_HUB.title}
        active={pathname === "/docs"}
      />
      <SidebarLink
        href="/docs/api"
        label="API Reference (OpenAPI)"
        active={pathname === "/docs/api"}
      />
      {DOC_SECTIONS.map((section) => (
        <SectionBlock key={section.id} section={section} pathname={pathname} />
      ))}
      <div className="space-y-1 border-t pt-4">
        <p className="px-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
          Tải về
        </p>
        <a
          href="/docs/openapi.yaml"
          download
          className="block rounded-md px-2 py-1.5 text-sm text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          openapi.yaml
        </a>
        <a
          href="/docs/postman-collection.json"
          download
          className="block rounded-md px-2 py-1.5 text-sm text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          Postman collection
        </a>
      </div>
    </aside>
  );
}

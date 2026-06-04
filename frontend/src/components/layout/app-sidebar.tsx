"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { isOwner, useLogout, useMe } from "@/hooks/use-auth";
import { filterNavGroups } from "@/lib/navigation";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { ThemeToggle } from "@/components/layout/theme-toggle";

function SidebarGroupLabel({ children }: { children: React.ReactNode }) {
  return (
    <div
      className="mb-1.5 mt-1 flex items-center gap-2 px-2 select-none"
      role="presentation"
    >
      <span className="h-px min-w-3 flex-1 bg-sidebar-border" aria-hidden />
      <span className="shrink-0 text-[10px] font-semibold uppercase tracking-[0.14em] text-sidebar-foreground/50">
        {children}
      </span>
      <span className="h-px min-w-3 flex-1 bg-sidebar-border" aria-hidden />
    </div>
  );
}

export function AppSidebar({ className }: { className?: string }) {
  const pathname = usePathname();
  const { data: user } = useMe();
  const logoutMutation = useLogout();
  const groups = filterNavGroups(isOwner(user));

  return (
    <aside
      className={cn(
        "sticky top-0 flex h-svh w-56 shrink-0 flex-col border-r border-sidebar-border bg-sidebar p-3 text-sidebar-foreground",
        className,
      )}
    >
      <Link
        href="/dashboard"
        className="mb-5 block shrink-0 px-2 text-lg font-bold tracking-tight text-sidebar-foreground"
      >
        MediaHub
      </Link>

      <nav
        className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto overscroll-contain"
        aria-label="Điều hướng chính"
      >
        {groups.map((group, groupIndex) => (
          <div key={group.label}>
            {groupIndex > 0 && <Separator className="mb-4 bg-sidebar-border" />}
            <SidebarGroupLabel>{group.label}</SidebarGroupLabel>
            <ul className="flex flex-col gap-0.5" role="list">
              {group.items.map((item) => {
                const active =
                  pathname === item.href ||
                  pathname.startsWith(item.href + "/");
                return (
                  <li key={item.href} role="none">
                    <Link
                      href={item.href}
                      className={cn(
                        "block rounded-md px-3 py-2 text-sm font-medium transition-colors",
                        active
                          ? "bg-sidebar-primary text-sidebar-primary-foreground shadow-sm"
                          : "text-sidebar-foreground/80 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
                      )}
                    >
                      {item.label}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </nav>

      <div className="mt-auto shrink-0 pt-3">
        <Separator className="mb-4 bg-sidebar-border" />
        <div className="flex items-center justify-between gap-2 px-1">
          <ThemeToggle />
          {user && (
            <div className="min-w-0 flex-1">
              <p className="truncate text-xs text-sidebar-foreground/70">
                {user.email}
              </p>
              <p className="text-[10px] font-medium uppercase tracking-wide text-sidebar-foreground/50">
                {user.role}
              </p>
            </div>
          )}
        </div>
        <Button
          variant="outline"
          size="sm"
          className="mt-3 w-full border-sidebar-border bg-transparent text-xs"
          disabled={logoutMutation.isPending}
          onClick={() => logoutMutation.mutate()}
        >
          {logoutMutation.isPending ? "Đang thoát..." : "Đăng xuất"}
        </Button>
      </div>
    </aside>
  );
}

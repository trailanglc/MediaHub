import Link from "next/link";
import { Button } from "@/components/ui/button";

const NAV = [
  { href: "/#features", label: "Tính năng" },
  { href: "/docs/integration", label: "Tài liệu API" },
] as const;

export function MarketingHeader() {
  return (
    <header className="sticky top-0 z-50 border-b border-border/60 bg-background/80 backdrop-blur-md">
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between gap-4 px-4 sm:px-6">
        <Link
          href="/"
          className="text-lg font-bold tracking-tight text-foreground"
        >
          MediaHub
        </Link>
        <nav
          className="hidden items-center gap-6 text-sm text-muted-foreground sm:flex"
          aria-label="Chính"
        >
          {NAV.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className="transition-colors hover:text-foreground"
            >
              {item.label}
            </Link>
          ))}
        </nav>
        <div className="flex items-center gap-2">
          <Button
            nativeButton={false}
            render={<Link href="/login" />}
            variant="ghost"
            size="sm"
            className="hidden sm:inline-flex"
          >
            Đăng nhập
          </Button>
          <Button
            nativeButton={false}
            render={<Link href="/dashboard" />}
            size="sm"
          >
            Vào ứng dụng
          </Button>
        </div>
      </div>
    </header>
  );
}

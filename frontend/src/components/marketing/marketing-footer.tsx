import Link from "next/link";

export function MarketingFooter() {
  const year = new Date().getFullYear();
  return (
    <footer className="border-t border-border/60 bg-muted/30">
      <div className="mx-auto flex max-w-6xl flex-col gap-6 px-4 py-10 sm:px-6 md:flex-row md:items-center md:justify-between">
        <div>
          <p className="font-semibold text-foreground">MediaHub</p>
          <p className="mt-1 max-w-sm text-sm text-muted-foreground">
            Nền tảng quản lý media tự host — file manager, streaming HLS, Integration API.
          </p>
        </div>
        <nav
          className="flex flex-wrap gap-x-6 gap-y-2 text-sm text-muted-foreground"
          aria-label="Footer"
        >
          <Link href="/docs/integration" className="hover:text-foreground">
            Integration API
          </Link>
          <Link href="/login" className="hover:text-foreground">
            Đăng nhập
          </Link>
          <a
            href="/docs/openapi.yaml"
            className="hover:text-foreground"
            download
          >
            OpenAPI
          </a>
        </nav>
      </div>
      <div className="border-t border-border/40 py-4 text-center text-xs text-muted-foreground">
        © {year} MediaHub · Self-hosted media platform
      </div>
    </footer>
  );
}

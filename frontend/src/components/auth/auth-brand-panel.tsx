const FEATURES = [
  "Quản lý tài sản media tập trung",
  "Phát stream HLS cho video",
  "Triển khai self-hosted, dữ liệu trong tầm kiểm soát",
] as const;

function CheckIcon() {
  return (
    <svg
      className="mt-0.5 size-5 shrink-0 text-violet-400"
      viewBox="0 0 20 20"
      fill="currentColor"
      aria-hidden
    >
      <path
        fillRule="evenodd"
        d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.483 4.79-1.88-1.88a.75.75 0 10-1.06 1.061l2.5 2.5a.75.75 0 001.137-.089l4-5.5z"
        clipRule="evenodd"
      />
    </svg>
  );
}

function BrandMark({ className = "" }: { className?: string }) {
  return (
    <div
      className={`flex size-10 items-center justify-center rounded-xl bg-gradient-to-br from-violet-500 via-fuchsia-500 to-cyan-500 text-lg font-bold text-white shadow-lg shadow-violet-500/25 ${className}`}
      aria-hidden
    >
      M
    </div>
  );
}

export function AuthBrandPanel() {
  return (
    <>
      {/* Mobile: compact header */}
      <header className="shrink-0 border-b border-zinc-800 bg-zinc-950 px-4 pt-[max(1rem,env(safe-area-inset-top))] pb-5 text-white sm:px-6 sm:pb-6 lg:hidden">
        <div className="mx-auto flex max-w-md items-center gap-3">
          <BrandMark className="size-9 shrink-0 text-base" />
          <div className="min-w-0">
            <p className="truncate text-lg font-semibold tracking-tight">
              MediaHub
            </p>
            <p className="text-pretty text-sm leading-snug text-zinc-400">
              Quản lý media &amp; streaming HLS self-hosted
            </p>
          </div>
        </div>
      </header>

      {/* Desktop: full brand panel */}
      <aside
        className="relative hidden overflow-hidden bg-zinc-950 px-12 py-16 text-white lg:flex lg:flex-col lg:justify-between"
        aria-label="Giới thiệu MediaHub"
      >
        <div
          className="pointer-events-none absolute inset-0 bg-gradient-to-br from-violet-600/20 via-fuchsia-600/10 to-cyan-600/20"
          aria-hidden
        />
        <div
          className="pointer-events-none absolute inset-0 opacity-[0.15]"
          style={{
            backgroundImage:
              "radial-gradient(circle at 1px 1px, rgb(255 255 255 / 0.15) 1px, transparent 0)",
            backgroundSize: "24px 24px",
          }}
          aria-hidden
        />
        <div className="relative">
          <div className="flex items-center gap-3">
            <BrandMark />
            <span className="text-2xl font-bold tracking-tight">MediaHub</span>
          </div>
          <p className="mt-8 max-w-sm text-lg leading-relaxed text-zinc-300">
            Self-hosted media asset manager &amp; HLS streaming — quản trị
            file, video và quyền truy cập trong một nền tảng.
          </p>
        </div>
        <ul className="relative mt-12 space-y-4">
          {FEATURES.map((text) => (
            <li key={text} className="flex gap-3 text-sm text-zinc-300">
              <CheckIcon />
              <span>{text}</span>
            </li>
          ))}
        </ul>
        <p className="relative mt-auto pt-12 text-xs text-zinc-500">
          © {new Date().getFullYear()} MediaHub
        </p>
      </aside>
    </>
  );
}

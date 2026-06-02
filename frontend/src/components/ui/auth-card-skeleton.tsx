export function AuthCardSkeleton() {
  return (
    <div
      className="animate-pulse w-full p-0 sm:rounded-xl sm:border sm:border-zinc-200 sm:bg-white sm:p-6 sm:shadow-lg sm:ring-1 sm:ring-zinc-200/80 dark:sm:border-zinc-800 dark:sm:bg-zinc-950 dark:sm:ring-zinc-800"
      aria-busy
      aria-label="Đang tải"
    >
      <div className="h-8 w-36 rounded-lg bg-zinc-200 dark:bg-zinc-800" />
      <div className="mt-3 h-4 w-full max-w-xs rounded bg-zinc-100 dark:bg-zinc-900" />
      <div className="mt-8 space-y-5">
        <div className="space-y-2">
          <div className="h-4 w-14 rounded bg-zinc-200 dark:bg-zinc-800" />
          <div className="h-10 w-full rounded-md bg-zinc-100 dark:bg-zinc-900" />
        </div>
        <div className="space-y-2">
          <div className="h-4 w-20 rounded bg-zinc-200 dark:bg-zinc-800" />
          <div className="h-10 w-full rounded-md bg-zinc-100 dark:bg-zinc-900" />
        </div>
        <div className="mt-2 h-10 w-full rounded-md bg-zinc-200 dark:bg-zinc-800" />
      </div>
    </div>
  );
}

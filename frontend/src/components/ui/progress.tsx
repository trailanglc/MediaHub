export function Progress({
  value,
  className = "",
  indicatorClassName = "",
}: {
  value: number;
  className?: string;
  indicatorClassName?: string;
}) {
  const pct = Math.min(100, Math.max(0, value));
  return (
    <div
      className={`h-2 w-full overflow-hidden rounded-full bg-muted ${className}`}
      role="progressbar"
      aria-valuenow={pct}
      aria-valuemin={0}
      aria-valuemax={100}
    >
      <div
        className={`h-full rounded-full transition-all duration-500 ease-out ${indicatorClassName}`}
        style={{ width: `${pct}%` }}
      />
    </div>
  );
}

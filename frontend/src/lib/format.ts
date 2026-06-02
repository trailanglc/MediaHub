export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}

export function formatUptime(seconds: number): string {
  const s = Math.floor(seconds);
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  if (d > 0) return `${d}d ${h}h ${m}m`;
  if (h > 0) return `${h}h ${m}m ${sec}s`;
  if (m > 0) return `${m}m ${sec}s`;
  return `${sec}s`;
}

export function metricColor(percent: number): string {
  if (percent >= 90) return "bg-red-500";
  if (percent >= 75) return "bg-amber-500";
  return "bg-emerald-500";
}

export function statusStyles(status: string): {
  dot: string;
  badge: string;
  label: string;
} {
  switch (status) {
    case "healthy":
      return {
        dot: "bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.6)]",
        badge: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400",
        label: "Khỏe",
      };
    case "unhealthy":
      return {
        dot: "bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.6)]",
        badge: "bg-red-500/10 text-red-700 dark:text-red-400",
        label: "Lỗi",
      };
    case "unknown":
      return {
        dot: "bg-zinc-400",
        badge: "bg-zinc-500/10 text-zinc-600 dark:text-zinc-400",
        label: "Chưa rõ",
      };
    default:
      return {
        dot: "bg-amber-500",
        badge: "bg-amber-500/10 text-amber-700 dark:text-amber-400",
        label: status,
      };
  }
}

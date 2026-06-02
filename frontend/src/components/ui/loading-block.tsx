import { Spinner } from "@/components/ui/spinner";

export function LoadingBlock({ label = "Đang tải..." }: { label?: string }) {
  return (
    <div className="flex items-center justify-center gap-2 py-8 text-sm text-muted-foreground">
      <Spinner className="size-4" />
      {label}
    </div>
  );
}

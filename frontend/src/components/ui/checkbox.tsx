import { forwardRef } from "react";
import { cn } from "@/lib/utils";

export const Checkbox = forwardRef<
  HTMLInputElement,
  React.ComponentProps<"input">
>(function Checkbox({ className, ...props }, ref) {
  return (
    <input
      ref={ref}
      type="checkbox"
      className={cn(
        "size-[1.125rem] shrink-0 cursor-pointer rounded border-2 border-primary/40 bg-background accent-primary shadow-sm",
        "checked:border-primary checked:bg-primary/10",
        "disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    />
  );
});

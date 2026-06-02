import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";

export function FormField({
  label,
  htmlFor,
  error,
  hint,
  required,
  children,
  className = "",
}: {
  label: string;
  htmlFor?: string;
  error?: string;
  hint?: string;
  required?: boolean;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("space-y-2", className)}>
      <Label htmlFor={htmlFor} className="text-sm font-medium">
        {label}
        {required && <span className="text-destructive"> *</span>}
      </Label>
      {children}
      {hint && !error && (
        <p className="mt-1 text-xs text-muted-foreground">{hint}</p>
      )}
      {error && <p className="mt-1 text-sm text-destructive">{error}</p>}
    </div>
  );
}

export function FormStack({
  children,
  className = "",
  onSubmit,
}: {
  children: React.ReactNode;
  className?: string;
  onSubmit?: React.FormEventHandler<HTMLFormElement>;
}) {
  const classes = cn("space-y-4", className);
  if (onSubmit) {
    return (
      <form className={classes} onSubmit={onSubmit}>
        {children}
      </form>
    );
  }
  return <div className={classes}>{children}</div>;
}

export function PageHeader({
  title,
  description,
  actions,
}: {
  title: string;
  description?: React.ReactNode;
  actions?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div className="min-w-0">
        <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
        {description && (
          <div className="mt-1 text-sm text-muted-foreground">{description}</div>
        )}
      </div>
      {actions && (
        <div className="flex flex-wrap items-center gap-2 sm:max-w-[55%] sm:justify-end">
          {actions}
        </div>
      )}
    </div>
  );
}

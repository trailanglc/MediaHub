import type { ComponentProps, ReactNode } from "react";

export function DataTable({ children }: { children: ReactNode }) {
  return (
    <div className="overflow-x-auto rounded-lg border border-border">
      <table className="w-full text-left text-sm">{children}</table>
    </div>
  );
}

export function DataTableHead({ children }: { children: ReactNode }) {
  return (
    <thead className="bg-muted/50 text-xs font-medium uppercase tracking-wide text-muted-foreground">
      {children}
    </thead>
  );
}

export function DataTableBody({ children }: { children: ReactNode }) {
  return <tbody className="divide-y divide-border">{children}</tbody>;
}

export function DataTableRow({
  children,
  className = "",
  ...props
}: ComponentProps<"tr">) {
  return (
    <tr className={`bg-card ${className}`.trim()} {...props}>
      {children}
    </tr>
  );
}

export function DataTableCell({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  return <td className={`px-4 py-3 ${className}`}>{children}</td>;
}

export function DataTableTh({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  return <th className={`px-4 py-3 ${className}`}>{children}</th>;
}

export function DataTableEmpty({
  colSpan,
  message,
}: {
  colSpan: number;
  message: string;
}) {
  return (
    <tr>
      <td colSpan={colSpan} className="px-4 py-8 text-center text-muted-foreground">
        {message}
      </td>
    </tr>
  );
}

export type NavItem = {
  href: string;
  label: string;
  ownerOnly: boolean;
};

export type NavGroup = {
  label: string;
  items: NavItem[];
};

export const NAV_GROUPS: NavGroup[] = [
  {
    label: "Nội dung",
    items: [
      { href: "/dashboard", label: "Dashboard", ownerOnly: false },
      { href: "/files", label: "Files", ownerOnly: false },
      { href: "/videos", label: "Videos", ownerOnly: false },
    ],
  },
  {
    label: "Quản trị",
    items: [
      { href: "/members", label: "Members", ownerOnly: true },
      { href: "/permissions", label: "Permissions", ownerOnly: true },
      { href: "/api-keys", label: "API Keys", ownerOnly: true },
      { href: "/settings", label: "Settings", ownerOnly: true },
    ],
  },
  {
    label: "Hệ thống",
    items: [
      { href: "/system/health", label: "System Health", ownerOnly: true },
      { href: "/system/audit-logs", label: "Audit Logs", ownerOnly: true },
      { href: "/system/storage", label: "Storage", ownerOnly: true },
      { href: "/system/queue", label: "Queue", ownerOnly: true },
    ],
  },
];

export const BREADCRUMB_LABELS: Record<string, string> = {
  dashboard: "Dashboard",
  files: "Files",
  videos: "Videos",
  members: "Members",
  permissions: "Permissions",
  "api-keys": "API Keys",
  settings: "Settings",
  security: "Security",
  system: "System",
  health: "Health",
  "audit-logs": "Audit Logs",
  storage: "Storage",
  queue: "Queue",
};

export function getBreadcrumbs(pathname: string): { label: string; href?: string }[] {
  const segments = pathname.split("/").filter(Boolean);
  if (segments.length === 0) return [{ label: "Dashboard", href: "/dashboard" }];

  const crumbs: { label: string; href?: string }[] = [];
  let path = "";

  for (let i = 0; i < segments.length; i++) {
    path += `/${segments[i]}`;
    const segment = segments[i];
    const label =
      BREADCRUMB_LABELS[segment] ??
      (segment.length > 12 ? `${segment.slice(0, 8)}…` : segment);
    const isLast = i === segments.length - 1;
    crumbs.push({ label, href: isLast ? undefined : path });
  }

  return crumbs;
}

export function filterNavGroups(showOwnerNav: boolean): NavGroup[] {
  return NAV_GROUPS.map((group) => ({
    ...group,
    items: group.items.filter((item) => !item.ownerOnly || showOwnerNav),
  })).filter((group) => group.items.length > 0);
}

/** Audit logs dashboard — Owner only (UI + API). */
export const AUDIT_LOGS_PATH = "/system/audit-logs";

export function isAuditLogsPath(pathname: string): boolean {
  return pathname === AUDIT_LOGS_PATH || pathname.startsWith(`${AUDIT_LOGS_PATH}/`);
}

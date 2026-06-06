"use client";

import Link from "next/link";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

const LINKS = [
  { href: "/members", label: "Members" },
  { href: "/permissions", label: "Permissions" },
  { href: "/api-keys", label: "API Keys" },
  { href: "/system/queue", label: "Queue" },
  { href: "/system/storage", label: "Storage" },
  { href: "/system/health", label: "System Health" },
  { href: "/system/audit-logs", label: "Audit Logs" },
  { href: "/settings", label: "Settings" },
] as const;

export function DashboardQuickLinks() {
  return (
    <Card size="sm">
      <CardHeader>
        <CardTitle>Liên kết vận hành</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-wrap gap-2">
        {LINKS.map(({ href, label }) => (
          <Button
            key={href}
            size="sm"
            variant="outline"
            nativeButton={false}
            render={<Link href={href} />}
          >
            {label}
          </Button>
        ))}
      </CardContent>
    </Card>
  );
}

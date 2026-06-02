"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { FolderIcon, FileIcon, FilmIcon, ImageIcon } from "lucide-react";
import {
  fetchMyPermissions,
  MY_PERMISSIONS_QUERY_KEY,
  type PermissionGrant,
} from "@/lib/api-client";
import { authErrorMessage, isOwner, roleLabel, useMe } from "@/hooks/use-auth";
import { permissionLabel } from "@/lib/permission-labels";
import { PageError } from "@/components/feedback/page-states";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { PageHeader } from "@/components/ui/page-header";
import { LoadingBlock } from "@/components/ui/loading-block";

type GroupedGrant = {
  resourcePublicId: string;
  resourceName: string;
  resourceType: string;
  permissions: string[];
};

function groupGrants(items: PermissionGrant[]): GroupedGrant[] {
  const map = new Map<string, GroupedGrant>();
  for (const g of items) {
    const existing = map.get(g.resource_public_id);
    if (existing) {
      if (!existing.permissions.includes(g.permission)) {
        existing.permissions.push(g.permission);
      }
      continue;
    }
    map.set(g.resource_public_id, {
      resourcePublicId: g.resource_public_id,
      resourceName: g.resource_name,
      resourceType: g.resource_type,
      permissions: [g.permission],
    });
  }
  return Array.from(map.values()).sort((a, b) =>
    a.resourceName.localeCompare(b.resourceName, "vi"),
  );
}

function resourceIcon(type: string) {
  switch (type) {
    case "folder":
      return FolderIcon;
    case "video":
      return FilmIcon;
    case "image":
      return ImageIcon;
    default:
      return FileIcon;
  }
}

function resourceHref(g: GroupedGrant): string {
  if (g.resourceType === "folder") {
    return `/files?folder=${g.resourcePublicId}`;
  }
  return "/files";
}

export function MemberDashboard() {
  const { data: user } = useMe();
  const { data, isLoading, isError, error } = useQuery({
    queryKey: MY_PERMISSIONS_QUERY_KEY,
    queryFn: fetchMyPermissions,
    enabled: Boolean(user) && !isOwner(user),
  });

  const groups = groupGrants(data?.items ?? []);
  const folderGrants = groups.filter((g) => g.resourceType === "folder");
  const otherGrants = groups.filter((g) => g.resourceType !== "folder");

  return (
    <div className="space-y-8">
      <PageHeader
        title="Dashboard"
        description={
          user?.email
            ? `Xin chào, ${user.email} — đây là các tài nguyên bạn được phép truy cập.`
            : "Tổng quan quyền truy cập của bạn"
        }
        actions={
          user?.role ? (
            <Badge variant="secondary">{roleLabel(user.role)}</Badge>
          ) : undefined
        }
      />

      <section aria-label="Truy cập nhanh">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">File Manager</CardTitle>
            <CardDescription>
              Duyệt thư mục và file theo quyền đã được cấp. Bạn chỉ thấy nội dung
              trong phạm vi chia sẻ.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Button render={<Link href="/files" />}>Mở File Manager</Button>
          </CardContent>
        </Card>
      </section>

      <section aria-label="Quyền đã được cấp" className="space-y-4">
        <h2 className="text-sm font-medium text-muted-foreground">
          Tài nguyên được chia sẻ
        </h2>

        {isLoading && <LoadingBlock />}
        {isError && (
          <PageError
            message={
              error instanceof Error
                ? authErrorMessage(error)
                : "Không tải được danh sách quyền"
            }
          />
        )}

        {!isLoading && !isError && groups.length === 0 && (
          <Card>
            <CardContent className="py-8 text-center text-sm text-muted-foreground">
              Chưa có quyền nào được gán. Liên hệ owner để được chia sẻ thư mục
              hoặc file.
            </CardContent>
          </Card>
        )}

        {!isLoading && !isError && folderGrants.length > 0 && (
          <ul className="grid gap-3 sm:grid-cols-2">
            {folderGrants.map((g) => {
              const Icon = resourceIcon(g.resourceType);
              return (
                <li key={g.resourcePublicId}>
                  <Card size="sm">
                    <CardHeader className="flex flex-row items-start gap-3 space-y-0 pb-2">
                      <Icon
                        className="mt-0.5 size-5 shrink-0 text-amber-500"
                        aria-hidden
                      />
                      <div className="min-w-0 flex-1">
                        <CardTitle className="truncate text-base">
                          {g.resourceName}
                        </CardTitle>
                        <CardDescription className="mt-1 flex flex-wrap gap-1">
                          {g.permissions.map((p) => (
                            <Badge key={p} variant="outline" className="text-xs">
                              {permissionLabel(p)}
                            </Badge>
                          ))}
                        </CardDescription>
                      </div>
                    </CardHeader>
                    <CardContent className="pt-0">
                      <Button
                        size="sm"
                        variant="secondary"
                        render={<Link href={resourceHref(g)} />}
                      >
                        Mở thư mục
                      </Button>
                    </CardContent>
                  </Card>
                </li>
              );
            })}
          </ul>
        )}

        {!isLoading && !isError && otherGrants.length > 0 && (
          <div className="space-y-2">
            <p className="text-xs text-muted-foreground">
              File / media được cấp quyền trực tiếp
            </p>
            <ul className="divide-y rounded-lg border">
              {otherGrants.map((g) => {
                const Icon = resourceIcon(g.resourceType);
                return (
                  <li
                    key={g.resourcePublicId}
                    className="flex flex-wrap items-center justify-between gap-3 px-4 py-3"
                  >
                    <div className="flex min-w-0 items-center gap-2">
                      <Icon className="size-4 shrink-0 text-muted-foreground" />
                      <span className="truncate font-medium">{g.resourceName}</span>
                      <span className="text-xs text-muted-foreground">
                        ({g.resourceType})
                      </span>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      {g.permissions.map((p) => (
                        <Badge key={p} variant="outline" className="text-xs">
                          {permissionLabel(p)}
                        </Badge>
                      ))}
                      <Button
                        size="sm"
                        variant="ghost"
                        render={<Link href={resourceHref(g)} />}
                      >
                        Mở Files
                      </Button>
                    </div>
                  </li>
                );
              })}
            </ul>
          </div>
        )}
      </section>
    </div>
  );
}

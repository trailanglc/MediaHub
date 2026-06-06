"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  ApiError,
  fetchMembers,
  fetchObject,
  fetchPermissions,
  fetchSettings,
  OBJECTS_QUERY_KEY,
  revokePermission,
  ROOT_FOLDER_PUBLIC_ID,
  SETTINGS_QUERY_KEY,
} from "@/lib/api-client";
import { authErrorMessage } from "@/hooks/use-auth";
import { toast } from "@/hooks/use-app-toast";
import { useConfirm } from "@/components/feedback/confirm-provider";
import { PageError } from "@/components/feedback/page-states";
import { FolderTree } from "@/components/files/folder-tree";
import { GrantPermissionModal } from "@/components/permissions/grant-permission-modal";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  DataTable,
  DataTableBody,
  DataTableCell,
  DataTableEmpty,
  DataTableHead,
  DataTableRow,
  DataTableTh,
} from "@/components/ui/data-table";
import { FormField } from "@/components/ui/form-field";
import { Input } from "@/components/ui/input";
import { LoadingBlock } from "@/components/ui/loading-block";
import { PageHeader } from "@/components/ui/page-header";
import { formatMediaPath } from "@/lib/format-media-path";
import { permissionLabel } from "@/lib/permissions/permission-labels";

export function PermissionsPanel() {
  const [resourceOverride, setResourceOverride] = useState<string | null>(null);
  const [grantOpen, setGrantOpen] = useState(false);
  const confirm = useConfirm();

  const queryClient = useQueryClient();

  const { data: settingsData } = useQuery({
    queryKey: SETTINGS_QUERY_KEY,
    queryFn: fetchSettings,
  });

  const defaultRootId =
    settingsData?.editable.media.default_root_folder_public_id ??
    ROOT_FOLDER_PUBLIC_ID;
  const resourceId = resourceOverride ?? defaultRootId;
  const permsKey = ["permissions", resourceId] as const;

  const { data: membersData } = useQuery({
    queryKey: ["members"],
    queryFn: () => fetchMembers(),
  });

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: permsKey,
    queryFn: () => fetchPermissions(resourceId),
    enabled: !!resourceId,
  });

  const { data: selectedResource } = useQuery({
    queryKey: [OBJECTS_QUERY_KEY, "perm-resource", resourceId],
    queryFn: () => fetchObject(resourceId),
    enabled: !!resourceId,
  });

  const revokeMutation = useMutation({
    mutationFn: (id: number) => revokePermission(id),
    onSuccess: () => {
      toast.success("Đã thu hồi quyền");
      queryClient.invalidateQueries({ queryKey: permsKey });
    },
    onError: (err) => {
      toast.error(
        err instanceof ApiError ? authErrorMessage(err) : "Không thể thu hồi quyền",
      );
    },
  });

  const items = data?.items ?? [];
  const members =
    membersData?.items.filter((m) => m.status === "active") ?? [];

  const handleRevoke = async (id: number, permission: string, email: string) => {
    const ok = await confirm({
      title: "Thu hồi quyền?",
      description: `Thu hồi quyền "${permissionLabel(permission)}" của ${email}?`,
      confirmLabel: "Thu hồi",
      variant: "destructive",
    });
    if (ok) revokeMutation.mutate(id);
  };

  return (
    <div className="space-y-6">
      <PageHeader
        title="Permissions"
        description={
          <>
            Gán quyền theo thư mục (kế thừa xuống file con). Chọn thư mục bên dưới
            hoặc mở File Manager → menu ⋯ → «Chia sẻ / quyền».
          </>
        }
        actions={
          <Button
            type="button"
            onClick={() => setGrantOpen(true)}
            disabled={!resourceId || members.length === 0}
          >
            + Gán quyền
          </Button>
        }
      />

      <Card>
        <CardHeader>
          <CardTitle>Thư mục</CardTitle>
          <CardDescription>
            Chọn thư mục để xem và gán quyền. Quyền trên folder áp dụng cho toàn bộ
            nội dung bên trong.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {selectedResource && (
            <div className="rounded-lg border border-border bg-muted/30 px-3 py-2 text-sm">
              <span className="text-muted-foreground">Đang chọn: </span>
              <span className="font-medium">{selectedResource.name}</span>
              {selectedResource.breadcrumbs && selectedResource.breadcrumbs.length > 0 && (
                <p className="mt-1 truncate text-xs text-muted-foreground">
                  {formatMediaPath(selectedResource.breadcrumbs, defaultRootId)}
                </p>
              )}
            </div>
          )}
          <FolderTree
            rootFolderId={defaultRootId}
            rootName="Root"
            selectedFolderId={resourceId}
            onSelect={(id) => {
              setResourceOverride(id === defaultRootId ? null : id);
            }}
          />
          <details className="text-sm">
            <summary className="cursor-pointer text-muted-foreground hover:text-foreground">
              Nâng cao — file / video theo public_id
            </summary>
            <FormField
              className="mt-3"
              label="Resource public_id"
              hint="Dán UUID khi cần gán quyền cho một file hoặc video cụ thể."
            >
              <Input
                className="font-mono text-xs"
                value={resourceId}
                onChange={(e) => setResourceOverride(e.target.value.trim() || null)}
              />
            </FormField>
          </details>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Quyền hiện tại</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading && <LoadingBlock />}
          {error && (
            <PageError
              message={
                error instanceof ApiError
                  ? authErrorMessage(error)
                  : "Lỗi tải permissions"
              }
              onRetry={() => void refetch()}
            />
          )}
          {!isLoading && !error && (
            <DataTable>
              <DataTableHead>
                <tr>
                  <DataTableTh>User</DataTableTh>
                  <DataTableTh>Resource</DataTableTh>
                  <DataTableTh>Permission</DataTableTh>
                  <DataTableTh>Thao tác</DataTableTh>
                </tr>
              </DataTableHead>
              <DataTableBody>
                {items.length === 0 && (
                  <DataTableEmpty
                    colSpan={4}
                    message="Chưa có quyền nào trên resource này."
                  />
                )}
                {items.map((p) => (
                  <DataTableRow key={p.id}>
                    <DataTableCell>{p.user_email}</DataTableCell>
                    <DataTableCell>
                      {p.resource_name}{" "}
                      <span className="text-xs text-muted-foreground">
                        ({p.resource_type})
                      </span>
                    </DataTableCell>
                    <DataTableCell>{permissionLabel(p.permission)}</DataTableCell>
                    <DataTableCell>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() =>
                          void handleRevoke(p.id, p.permission, p.user_email)
                        }
                        disabled={revokeMutation.isPending}
                      >
                        Thu hồi
                      </Button>
                    </DataTableCell>
                  </DataTableRow>
                ))}
              </DataTableBody>
            </DataTable>
          )}
        </CardContent>
      </Card>

      <GrantPermissionModal
        open={grantOpen}
        onClose={() => setGrantOpen(false)}
        resourceId={resourceId}
        members={members}
        onSuccess={() => queryClient.invalidateQueries({ queryKey: permsKey })}
      />
    </div>
  );
}

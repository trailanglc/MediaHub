"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import type { MediaObject } from "@/lib/api-client";
import {
  fetchMembers,
  fetchPermissions,
  revokePermission,
} from "@/lib/api-client";
import { GrantPermissionModal } from "@/components/permissions/grant-permission-modal";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DataTable,
  DataTableBody,
  DataTableCell,
  DataTableEmpty,
  DataTableHead,
  DataTableRow,
  DataTableTh,
} from "@/components/ui/data-table";
import { LoadingBlock } from "@/components/ui/loading-block";
import { toast } from "@/hooks/use-app-toast";
import { permissionLabel } from "@/lib/permissions/permission-labels";

export function ShareAccessDialog({
  open,
  object,
  onClose,
}: {
  open: boolean;
  object: MediaObject | null;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const [grantOpen, setGrantOpen] = useState(false);
  const resourceId = object?.public_id ?? "";

  const { data: perms, isLoading } = useQuery({
    queryKey: ["permissions", resourceId],
    queryFn: () => fetchPermissions(resourceId),
    enabled: open && !!resourceId,
  });

  const { data: membersData } = useQuery({
    queryKey: ["members"],
    queryFn: () => fetchMembers(),
    enabled: grantOpen,
  });

  const revokeMutation = useMutation({
    mutationFn: (id: number) => revokePermission(id),
    onSuccess: () => {
      toast.success("Đã thu hồi quyền");
      queryClient.invalidateQueries({ queryKey: ["permissions", resourceId] });
    },
    onError: () => toast.error("Không thể thu hồi quyền"),
  });

  return (
    <>
      <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Quyền truy cập — {object?.name}</DialogTitle>
          </DialogHeader>
          <div className="mb-3 flex justify-end">
            <Button type="button" size="sm" onClick={() => setGrantOpen(true)}>
              + Gán quyền
            </Button>
          </div>
          {isLoading ? (
            <LoadingBlock />
          ) : (
            <DataTable>
              <DataTableHead>
                <DataTableRow>
                  <DataTableTh>Member</DataTableTh>
                  <DataTableTh>Quyền</DataTableTh>
                  <DataTableTh> </DataTableTh>
                </DataTableRow>
              </DataTableHead>
              <DataTableBody>
                {(perms?.items ?? []).length === 0 ? (
                  <DataTableEmpty
                    colSpan={3}
                    message="Chưa có quyền được gán"
                  />
                ) : (
                  perms?.items.map((p) => (
                    <DataTableRow key={p.id}>
                      <DataTableCell>{p.user_email}</DataTableCell>
                      <DataTableCell>{permissionLabel(p.permission)}</DataTableCell>
                      <DataTableCell>
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          onClick={() => revokeMutation.mutate(p.id)}
                        >
                          Thu hồi
                        </Button>
                      </DataTableCell>
                    </DataTableRow>
                  ))
                )}
              </DataTableBody>
            </DataTable>
          )}
        </DialogContent>
      </Dialog>
      <GrantPermissionModal
        open={grantOpen}
        onClose={() => setGrantOpen(false)}
        resourceId={resourceId}
        members={membersData?.items ?? []}
        onSuccess={() => {
          setGrantOpen(false);
          queryClient.invalidateQueries({
            queryKey: ["permissions", resourceId],
          });
        }}
      />
    </>
  );
}

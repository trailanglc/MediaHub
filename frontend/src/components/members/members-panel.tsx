"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  ApiError,
  disableMember,
  fetchMembers,
  purgeMember,
  restoreMember,
  updateMember,
} from "@/lib/api-client";
import { authErrorMessage } from "@/hooks/use-auth";
import { toast } from "@/hooks/use-app-toast";
import { useConfirm } from "@/components/feedback/confirm-provider";
import { PageError } from "@/components/feedback/page-states";
import { CreateMemberModal } from "@/components/members/create-member-modal";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
import { NativeSelect } from "@/components/ui/native-select";
import { PageHeader } from "@/components/ui/page-header";

const MEMBERS_KEY = ["members"] as const;

export function MembersPanel() {
  const queryClient = useQueryClient();
  const confirm = useConfirm();
  const [createOpen, setCreateOpen] = useState(false);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: MEMBERS_KEY,
    queryFn: () => fetchMembers(),
  });

  const disableMutation = useMutation({
    mutationFn: (publicId: string) => disableMember(publicId),
    onSuccess: () => {
      toast.success("Đã vô hiệu hóa member");
      queryClient.invalidateQueries({ queryKey: MEMBERS_KEY });
    },
    onError: (err) => {
      toast.error(
        err instanceof ApiError ? authErrorMessage(err) : "Không thể vô hiệu hóa",
      );
    },
  });

  const restoreMutation = useMutation({
    mutationFn: (publicId: string) => restoreMember(publicId),
    onSuccess: () => {
      toast.success("Đã mở lại member");
      queryClient.invalidateQueries({ queryKey: MEMBERS_KEY });
    },
    onError: (err) => {
      toast.error(
        err instanceof ApiError ? authErrorMessage(err) : "Không thể mở lại",
      );
    },
  });

  const purgeMutation = useMutation({
    mutationFn: (publicId: string) => purgeMember(publicId),
    onSuccess: () => {
      toast.success("Đã xóa member vĩnh viễn");
      queryClient.invalidateQueries({ queryKey: MEMBERS_KEY });
    },
    onError: (err) => {
      toast.error(
        err instanceof ApiError ? authErrorMessage(err) : "Không thể xóa",
      );
    },
  });

  const roleMutation = useMutation({
    mutationFn: ({ publicId, newRole }: { publicId: string; newRole: string }) =>
      updateMember(publicId, { role: newRole }),
    onSuccess: () => {
      toast.success("Đã cập nhật role");
      queryClient.invalidateQueries({ queryKey: MEMBERS_KEY });
    },
    onError: (err) => {
      toast.error(
        err instanceof ApiError ? authErrorMessage(err) : "Không thể đổi role",
      );
    },
  });

  const items = data?.items ?? [];

  const handleDisable = async (publicId: string, email: string) => {
    const ok = await confirm({
      title: "Vô hiệu hóa member",
      description: `Bạn có chắc muốn vô hiệu hóa ${email}? Phiên đăng nhập của họ sẽ bị thu hồi.`,
      confirmLabel: "Vô hiệu hóa",
      variant: "destructive",
    });
    if (ok) disableMutation.mutate(publicId);
  };

  const handleRestore = async (publicId: string, email: string) => {
    const ok = await confirm({
      title: "Mở lại member",
      description: `Kích hoạt lại tài khoản ${email}? Họ có thể đăng nhập và dùng quyền đã được gán.`,
      confirmLabel: "Mở lại",
    });
    if (ok) restoreMutation.mutate(publicId);
  };

  const handlePurge = async (publicId: string, email: string) => {
    const ok = await confirm({
      title: "Xóa member vĩnh viễn",
      description: `Xóa vĩnh viễn ${email}? Toàn bộ quyền và phiên liên quan sẽ bị xóa. Không thể hoàn tác.`,
      confirmLabel: "Xóa vĩnh viễn",
      variant: "destructive",
    });
    if (ok) purgeMutation.mutate(publicId);
  };

  const actionPending =
    disableMutation.isPending ||
    restoreMutation.isPending ||
    purgeMutation.isPending;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Members"
        description="Quản lý tài khoản manager và viewer."
        actions={
          <Button type="button" onClick={() => setCreateOpen(true)}>
            + Tạo member
          </Button>
        }
      />

      {error && (
        <PageError
          message={
            error instanceof ApiError ? authErrorMessage(error) : "Lỗi tải danh sách"
          }
          onRetry={() => void refetch()}
        />
      )}

      <Card>
        <CardHeader>
          <CardTitle>Danh sách</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <LoadingBlock />
          ) : !error ? (
            <DataTable>
              <DataTableHead>
                <tr>
                  <DataTableTh>Email</DataTableTh>
                  <DataTableTh>Role</DataTableTh>
                  <DataTableTh>Status</DataTableTh>
                  <DataTableTh>Thao tác</DataTableTh>
                </tr>
              </DataTableHead>
              <DataTableBody>
                {items.length === 0 && (
                  <DataTableEmpty colSpan={4} message="Chưa có member nào." />
                )}
                {items.map((m) => (
                  <DataTableRow key={m.public_id}>
                    <DataTableCell>{m.email}</DataTableCell>
                    <DataTableCell>
                      <NativeSelect
                        className="h-8 max-w-[8rem] text-xs"
                        value={m.role}
                        disabled={
                          m.status === "disabled" || roleMutation.isPending
                        }
                        onChange={(e) =>
                          roleMutation.mutate({
                            publicId: m.public_id,
                            newRole: e.target.value,
                          })
                        }
                      >
                        <option value="viewer">viewer</option>
                        <option value="manager">manager</option>
                      </NativeSelect>
                    </DataTableCell>
                    <DataTableCell>
                      <Badge
                        variant={m.status === "active" ? "secondary" : "outline"}
                      >
                        {m.status === "active" ? "Hoạt động" : "Vô hiệu"}
                      </Badge>
                    </DataTableCell>
                    <DataTableCell>
                      <div className="flex flex-wrap gap-2">
                        {m.status === "active" ? (
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            onClick={() => void handleDisable(m.public_id, m.email)}
                            disabled={actionPending}
                          >
                            Vô hiệu hóa
                          </Button>
                        ) : (
                          <>
                            <Button
                              type="button"
                              variant="secondary"
                              size="sm"
                              onClick={() => void handleRestore(m.public_id, m.email)}
                              disabled={actionPending}
                            >
                              Mở lại
                            </Button>
                            <Button
                              type="button"
                              variant="destructive"
                              size="sm"
                              onClick={() => void handlePurge(m.public_id, m.email)}
                              disabled={actionPending}
                            >
                              Xóa vĩnh viễn
                            </Button>
                          </>
                        )}
                      </div>
                    </DataTableCell>
                  </DataTableRow>
                ))}
              </DataTableBody>
            </DataTable>
          ) : null}
        </CardContent>
      </Card>

      <CreateMemberModal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onSuccess={() =>
          queryClient.invalidateQueries({ queryKey: MEMBERS_KEY })
        }
      />
    </div>
  );
}

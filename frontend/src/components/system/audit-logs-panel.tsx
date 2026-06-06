"use client";

import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useCallback, useMemo, useState } from "react";
import { PageError } from "@/components/feedback/page-states";
import { isOwner, useMe } from "@/hooks/use-auth";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import {
  DataTable,
  DataTableBody,
  DataTableCell,
  DataTableEmpty,
  DataTableHead,
  DataTableRow,
  DataTableTh,
} from "@/components/ui/data-table";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { LoadingBlock } from "@/components/ui/loading-block";
import { PageHeader } from "@/components/ui/page-header";
import {
  AUDIT_LOG_ACTIONS_KEY,
  AUDIT_LOGS_QUERY_KEY,
  fetchAuditLogActions,
  fetchAuditLogs,
  fetchMembers,
  type AuditLogEntry,
  type AuditLogsFilterParams,
} from "@/lib/api/api-client";

type AppliedFilters = AuditLogsFilterParams & {
  actions: string[];
};

const EMPTY_FILTERS: AppliedFilters = {
  actions: [],
  action_prefix: "",
  actor_public_id: "",
  from: "",
  to: "",
};

function formatWhen(iso: string) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString("vi-VN");
}

function formatAuditIP(ip?: string) {
  if (!ip) return "—";
  return ip;
}

function toRFC3339FromLocalInput(value: string, endOfDay: boolean): string | undefined {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  if (trimmed.includes("T")) {
    const d = new Date(trimmed);
    if (Number.isNaN(d.getTime())) return undefined;
    return d.toISOString();
  }
  const d = new Date(`${trimmed}T${endOfDay ? "23:59:59" : "00:00:00"}`);
  if (Number.isNaN(d.getTime())) return undefined;
  return d.toISOString();
}

function actionBadgeVariant(action: string): "default" | "secondary" | "destructive" | "outline" {
  if (action.includes("delete") || action.includes("purge") || action.includes("revoke")) {
    return "destructive";
  }
  if (action.includes("failed")) {
    return "outline";
  }
  if (action.includes("login") || action.includes("auth")) {
    return "secondary";
  }
  return "default";
}

function metadataSummary(entry: AuditLogEntry): string | null {
  const keys = Object.keys(entry.metadata ?? {});
  if (keys.length === 0) return null;
  const parts = keys.slice(0, 3).map((k) => `${k}=${entry.metadata[k]}`);
  if (keys.length > 3) parts.push("…");
  return parts.join(", ");
}

function filtersToQueryKey(filters: AppliedFilters) {
  return [
    filters.from ?? "",
    filters.to ?? "",
    filters.actor_public_id ?? "",
    filters.action_prefix ?? "",
    (filters.actions ?? []).join("|"),
  ];
}

function buildFetchParams(filters: AppliedFilters, cursor: number): AuditLogsFilterParams {
  return {
    cursor,
    limit: 50,
    actions: filters.actions.length ? filters.actions : undefined,
    action_prefix: filters.action_prefix?.trim() || undefined,
    actor_public_id: filters.actor_public_id?.trim() || undefined,
    from: toRFC3339FromLocalInput(filters.from ?? "", false),
    to: toRFC3339FromLocalInput(filters.to ?? "", true),
  };
}

export function AuditLogsPanel() {
  const { data: user, isLoading: meLoading } = useMe();
  const owner = isOwner(user);
  const [draft, setDraft] = useState<AppliedFilters>(EMPTY_FILTERS);
  const [applied, setApplied] = useState<AppliedFilters>(EMPTY_FILTERS);

  const actionsQuery = useQuery({
    queryKey: AUDIT_LOG_ACTIONS_KEY,
    queryFn: fetchAuditLogActions,
    enabled: owner,
  });

  const membersQuery = useQuery({
    queryKey: ["members", "audit-filter"],
    queryFn: () => fetchMembers({ limit: "100" }),
    enabled: owner,
  });

  const query = useInfiniteQuery({
    queryKey: [...AUDIT_LOGS_QUERY_KEY, ...filtersToQueryKey(applied)],
    queryFn: ({ pageParam }) =>
      fetchAuditLogs(buildFetchParams(applied, pageParam ?? 0)),
    initialPageParam: 0,
    getNextPageParam: (last) => last.next_cursor,
    enabled: owner,
  });

  const items = useMemo(
    () => query.data?.pages.flatMap((p) => p.items) ?? [],
    [query.data],
  );

  const knownActions = actionsQuery.data?.actions ?? [];
  const members = membersQuery.data?.items ?? [];

  const toggleAction = useCallback((action: string, checked: boolean) => {
    setDraft((prev) => {
      const set = new Set(prev.actions);
      if (checked) set.add(action);
      else set.delete(action);
      return { ...prev, actions: Array.from(set).sort() };
    });
  }, []);

  const applyFilters = useCallback(() => {
    setApplied({
      ...draft,
      actions: [...draft.actions],
    });
  }, [draft]);

  const clearFilters = useCallback(() => {
    setDraft(EMPTY_FILTERS);
    setApplied(EMPTY_FILTERS);
  }, []);

  const hasActiveFilters =
    applied.actions.length > 0 ||
    Boolean(applied.action_prefix?.trim()) ||
    Boolean(applied.actor_public_id?.trim()) ||
    Boolean(applied.from?.trim()) ||
    Boolean(applied.to?.trim());

  if (meLoading) {
    return <LoadingBlock label="Đang kiểm tra quyền…" />;
  }

  if (!owner) {
    return null;
  }

  if (query.isError) {
    return <PageError message="Không tải được audit logs." onRetry={() => query.refetch()} />;
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Audit Logs"
        description="Lịch sử hành động nghiệp vụ và bảo mật — chỉ Owner. IP được chuẩn hóa IPv4/IPv6; cần cấu hình TRUSTED_PROXIES khi đặt sau reverse proxy."
      />

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Bộ lọc</CardTitle>
          <CardDescription>
            Lọc theo khoảng thời gian, user, action cụ thể hoặc prefix (vd. auth., video.)
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <div className="grid gap-1.5">
              <Label htmlFor="audit-from">Từ</Label>
              <Input
                id="audit-from"
                type="datetime-local"
                value={draft.from ?? ""}
                onChange={(e) => setDraft((p) => ({ ...p, from: e.target.value }))}
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="audit-to">Đến</Label>
              <Input
                id="audit-to"
                type="datetime-local"
                value={draft.to ?? ""}
                onChange={(e) => setDraft((p) => ({ ...p, to: e.target.value }))}
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="audit-user">User</Label>
              <select
                id="audit-user"
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs"
                value={draft.actor_public_id ?? ""}
                onChange={(e) =>
                  setDraft((p) => ({ ...p, actor_public_id: e.target.value }))
                }
              >
                <option value="">Tất cả users</option>
                {members.map((m) => (
                  <option key={m.public_id} value={m.public_id}>
                    {m.email} ({m.role})
                  </option>
                ))}
              </select>
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="audit-prefix">Action prefix</Label>
              <Input
                id="audit-prefix"
                placeholder="auth."
                value={draft.action_prefix ?? ""}
                onChange={(e) =>
                  setDraft((p) => ({ ...p, action_prefix: e.target.value }))
                }
              />
            </div>
          </div>

          <div className="space-y-2">
            <Label>Actions</Label>
            {actionsQuery.isLoading ? (
              <p className="text-sm text-muted-foreground">Đang tải danh sách action…</p>
            ) : knownActions.length === 0 ? (
              <p className="text-sm text-muted-foreground">Chưa có action nào.</p>
            ) : (
              <div className="max-h-48 overflow-y-auto rounded-md border p-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
                {knownActions.map((action) => (
                  <label
                    key={action}
                    className="flex cursor-pointer items-center gap-2 text-sm"
                  >
                    <Checkbox
                      checked={draft.actions.includes(action)}
                      onChange={(e) => toggleAction(action, e.target.checked)}
                    />
                    <span className="truncate font-mono text-xs">{action}</span>
                  </label>
                ))}
              </div>
            )}
            {draft.actions.length > 0 ? (
              <div className="flex flex-wrap gap-1 pt-1">
                {draft.actions.map((action) => (
                  <Badge key={action} variant="secondary" className="font-mono text-xs">
                    {action}
                  </Badge>
                ))}
              </div>
            ) : null}
          </div>

          <div className="flex flex-wrap gap-2">
            <Button type="button" onClick={applyFilters}>
              Áp dụng
            </Button>
            {hasActiveFilters ? (
              <Button type="button" variant="outline" onClick={clearFilters}>
                Xóa lọc
              </Button>
            ) : null}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Sự kiện</CardTitle>
          <CardDescription>
            {query.isLoading ? "Đang tải…" : `${items.length} bản ghi`}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {query.isLoading ? (
            <LoadingBlock label="Đang tải audit logs…" />
          ) : (
            <>
              <DataTable>
                <DataTableHead>
                  <DataTableRow>
                    <DataTableTh>Thời gian</DataTableTh>
                    <DataTableTh>Actor</DataTableTh>
                    <DataTableTh>Action</DataTableTh>
                    <DataTableTh>Target</DataTableTh>
                    <DataTableTh>IP</DataTableTh>
                    <DataTableTh>Metadata</DataTableTh>
                  </DataTableRow>
                </DataTableHead>
                <DataTableBody>
                  {items.length === 0 ? (
                    <DataTableEmpty colSpan={6} message="Không có audit log phù hợp bộ lọc." />
                  ) : (
                    items.map((entry) => (
                      <DataTableRow key={entry.id}>
                        <DataTableCell className="whitespace-nowrap text-xs tabular-nums">
                          {formatWhen(entry.created_at)}
                        </DataTableCell>
                        <DataTableCell className="max-w-[180px] truncate text-sm">
                          {entry.actor?.email ?? "—"}
                        </DataTableCell>
                        <DataTableCell>
                          <Badge variant={actionBadgeVariant(entry.action)}>
                            {entry.action}
                          </Badge>
                        </DataTableCell>
                        <DataTableCell className="text-xs text-muted-foreground">
                          {entry.target_type ?? "—"}
                          {entry.target_id != null ? ` #${entry.target_id}` : ""}
                        </DataTableCell>
                        <DataTableCell className="max-w-[160px] truncate text-xs font-mono">
                          <span title={entry.ip}>{formatAuditIP(entry.ip)}</span>
                        </DataTableCell>
                        <DataTableCell className="max-w-[220px] truncate text-xs text-muted-foreground">
                          {metadataSummary(entry) ?? "—"}
                        </DataTableCell>
                      </DataTableRow>
                    ))
                  )}
                </DataTableBody>
              </DataTable>

              {query.hasNextPage ? (
                <div className="mt-4 flex justify-center">
                  <Button
                    type="button"
                    variant="outline"
                    disabled={query.isFetchingNextPage}
                    onClick={() => query.fetchNextPage()}
                  >
                    {query.isFetchingNextPage ? "Đang tải…" : "Tải thêm"}
                  </Button>
                </div>
              ) : null}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

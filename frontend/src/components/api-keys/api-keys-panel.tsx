"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { CopyIcon, PencilIcon, XIcon } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { FolderPickerDialog } from "@/components/api-keys/folder-picker-dialog";
import { useConfirm } from "@/components/feedback/confirm-provider";
import { PageError } from "@/components/feedback/page-states";
import { Alert, AlertDescription } from "@/components/ui/alert";
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { LoadingBlock } from "@/components/ui/loading-block";
import { PageHeader } from "@/components/ui/page-header";
import { Textarea } from "@/components/ui/textarea";
import { toast } from "@/hooks/use-app-toast";
import {
  API_KEY_SCOPE_LABELS,
  createAPIKey,
  fetchAPIKeys,
  fetchObject,
  INTEGRATION_API_KEY_SCOPES,
  OBJECTS_QUERY_KEY,
  patchAPIKey,
  revokeAPIKey,
  type APIKey,
  type IntegrationAPIKeyScope,
} from "@/lib/api/api-client";

const API_KEYS_KEY = ["api-keys"] as const;

function parseLines(text: string): string[] {
  return text
    .split("\n")
    .map((s) => s.trim())
    .filter(Boolean);
}

function linesToText(lines: string[]): string {
  return lines.join("\n");
}

function formatTimestamp(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString("vi-VN");
}

function scopeBadges(scopes: string[]) {
  return scopes.map((s) => (
    <Badge key={s} variant="secondary" className="mr-1 mb-1 text-xs">
      {API_KEY_SCOPE_LABELS[s as IntegrationAPIKeyScope] ?? s}
    </Badge>
  ));
}

type KeyFormState = {
  name: string;
  scopes: IntegrationAPIKeyScope[];
  ipsText: string;
  rootFolderId: string | null;
  rootFolderName: string;
};

const defaultFormState = (): KeyFormState => ({
  name: "",
  scopes: [...INTEGRATION_API_KEY_SCOPES],
  ipsText: "",
  rootFolderId: null,
  rootFolderName: "Mặc định hệ thống",
});

function formFromKey(key: APIKey): KeyFormState {
  return {
    name: key.name,
    scopes: key.scopes.filter((s): s is IntegrationAPIKeyScope =>
      (INTEGRATION_API_KEY_SCOPES as readonly string[]).includes(s),
    ),
    ipsText: linesToText(key.allowed_ips),
    rootFolderId: key.root_folder_public_id ?? null,
    rootFolderName: key.root_folder_public_id ? "…" : "Mặc định hệ thống",
  };
}

function ScopeCheckboxes({
  scopes,
  onChange,
}: {
  scopes: IntegrationAPIKeyScope[];
  onChange: (scopes: IntegrationAPIKeyScope[]) => void;
}) {
  const toggle = (scope: IntegrationAPIKeyScope, checked: boolean) => {
    if (checked) {
      onChange([...scopes, scope]);
    } else {
      onChange(scopes.filter((s) => s !== scope));
    }
  };

  return (
    <div className="space-y-2">
      {INTEGRATION_API_KEY_SCOPES.map((scope) => (
        <label
          key={scope}
          className="flex cursor-pointer items-start gap-2 text-sm"
        >
          <Checkbox
            checked={scopes.includes(scope)}
            onChange={(e) => toggle(scope, e.target.checked)}
          />
          <span>
            <span className="font-medium">{scope}</span>
            <span className="block text-xs text-muted-foreground">
              {API_KEY_SCOPE_LABELS[scope]}
            </span>
          </span>
        </label>
      ))}
    </div>
  );
}

function KeyRestrictionsFields({
  form,
  setForm,
  folderPickerOpen,
  setFolderPickerOpen,
}: {
  form: KeyFormState;
  setForm: React.Dispatch<React.SetStateAction<KeyFormState>>;
  folderPickerOpen: boolean;
  setFolderPickerOpen: (v: boolean) => void;
}) {
  return (
    <>
      <div className="space-y-2">
        <Label>Scopes</Label>
        <ScopeCheckboxes
          scopes={form.scopes}
          onChange={(scopes) => setForm((f) => ({ ...f, scopes }))}
        />
      </div>

      <div className="space-y-2">
        <Label>Root folder</Label>
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-sm text-muted-foreground">
            {form.rootFolderName}
          </span>
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => setFolderPickerOpen(true)}
          >
            Chọn thư mục
          </Button>
        </div>
      </div>

      <div className="space-y-2">
        <Label htmlFor="allowed-ips">Allowed IPs (một IP/dòng, để trống = không giới hạn)</Label>
        <Textarea
          id="allowed-ips"
          rows={2}
          placeholder="203.0.113.10"
          value={form.ipsText}
          onChange={(e) => setForm((f) => ({ ...f, ipsText: e.target.value }))}
        />
      </div>

      <FolderPickerDialog
        open={folderPickerOpen}
        selectedFolderId={form.rootFolderId}
        onClose={() => setFolderPickerOpen(false)}
        onConfirm={(folderId, folderName) => {
          setForm((f) => ({
            ...f,
            rootFolderId: folderId,
            rootFolderName: folderName,
          }));
        }}
      />
    </>
  );
}

function buildKeyPayload(form: KeyFormState) {
  const body: {
    name: string;
    scopes: string[];
    allowed_ips: string[];
    root_folder_public_id?: string;
    clear_root_folder?: boolean;
  } = {
    name: form.name.trim(),
    scopes: form.scopes,
    allowed_ips: parseLines(form.ipsText),
  };
  if (form.rootFolderId) {
    body.root_folder_public_id = form.rootFolderId;
  } else {
    body.clear_root_folder = true;
  }
  return body;
}

export function ApiKeysPanel() {
  const qc = useQueryClient();
  const confirm = useConfirm();
  const [createForm, setCreateForm] = useState<KeyFormState>(defaultFormState);
  const [createFolderPickerOpen, setCreateFolderPickerOpen] = useState(false);
  const [newSecret, setNewSecret] = useState<string | null>(null);
  const [editKey, setEditKey] = useState<APIKey | null>(null);
  const [editForm, setEditForm] = useState<KeyFormState>(defaultFormState);
  const [editFolderPickerOpen, setEditFolderPickerOpen] = useState(false);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: API_KEYS_KEY,
    queryFn: fetchAPIKeys,
  });

  const rootFolderIds = useMemo(() => {
    const ids = new Set<string>();
    for (const k of data?.items ?? []) {
      if (k.root_folder_public_id) ids.add(k.root_folder_public_id);
    }
    if (editKey?.root_folder_public_id) ids.add(editKey.root_folder_public_id);
    return [...ids];
  }, [data?.items, editKey?.root_folder_public_id]);

  const folderNames = useQuery({
    queryKey: [OBJECTS_QUERY_KEY, "api-key-folders", rootFolderIds],
    queryFn: async () => {
      const entries = await Promise.all(
        rootFolderIds.map(async (id) => {
          const obj = await fetchObject(id);
          return [id, obj.name] as const;
        }),
      );
      return Object.fromEntries(entries);
    },
    enabled: rootFolderIds.length > 0,
  });

  const createMut = useMutation({
    mutationFn: () => {
      const payload = buildKeyPayload(createForm);
      delete payload.clear_root_folder;
      if (!createForm.rootFolderId) {
        delete payload.root_folder_public_id;
      }
      return createAPIKey(payload);
    },
    onSuccess: (res) => {
      setNewSecret(res.secret);
      setCreateForm(defaultFormState());
      toast.success("Đã tạo API key — copy secret ngay, chỉ hiển thị một lần.");
      void qc.invalidateQueries({ queryKey: API_KEYS_KEY });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const editMut = useMutation({
    mutationFn: () => {
      if (!editKey) throw new Error("missing key");
      const payload = buildKeyPayload(editForm);
      if (editForm.rootFolderId) {
        delete payload.clear_root_folder;
      }
      return patchAPIKey(editKey.public_id, payload);
    },
    onSuccess: () => {
      toast.success("Đã cập nhật API key");
      setEditKey(null);
      void qc.invalidateQueries({ queryKey: API_KEYS_KEY });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const revokeMut = useMutation({
    mutationFn: (publicId: string) => revokeAPIKey(publicId),
    onSuccess: () => {
      toast.success("Đã thu hồi key");
      void qc.invalidateQueries({ queryKey: API_KEYS_KEY });
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const copySecret = useCallback(async () => {
    if (!newSecret) return;
    await navigator.clipboard.writeText(newSecret);
    toast.success("Đã copy secret");
  }, [newSecret]);

  useEffect(() => {
    if (!editKey) return;
    setEditForm(formFromKey(editKey));
  }, [editKey]);

  useEffect(() => {
    if (!editKey?.root_folder_public_id || !folderNames.data) return;
    const name = folderNames.data[editKey.root_folder_public_id];
    if (name) {
      setEditForm((f) => ({ ...f, rootFolderName: name }));
    }
  }, [editKey?.root_folder_public_id, folderNames.data]);

  const handleRevoke = async (key: APIKey) => {
    const ok = await confirm({
      title: "Thu hồi API key?",
      description: `Key «${key.name}» sẽ không còn hoạt động. Hành động không thể hoàn tác.`,
      confirmLabel: "Thu hồi",
      variant: "destructive",
    });
    if (ok) revokeMut.mutate(key.public_id);
  };

  const rootFolderLabel = (key: APIKey) => {
    if (!key.root_folder_public_id) return "Mặc định hệ thống";
    return folderNames.data?.[key.root_folder_public_id] ?? key.root_folder_public_id.slice(0, 8) + "…";
  };

  const canSubmitCreate =
    createForm.name.trim().length > 0 && createForm.scopes.length > 0;

  const canSubmitEdit =
    editForm.name.trim().length > 0 && editForm.scopes.length > 0;

  return (
    <div className="space-y-6">
      <PageHeader
        title="API Keys"
        description="Khóa tích hợp cho website/CMS bên ngoài (upload, delivery URL, HLS embed)."
      />

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Hướng dẫn nhanh</CardTitle>
          <CardDescription>
            Gửi key trong header{" "}
            <code className="rounded bg-muted px-1 text-xs">X-API-Key: mh_…</code>
            .{" "}
            <Link
              href="/docs/integration"
              className="font-medium text-primary underline-offset-4 hover:underline"
            >
              Xem tài liệu Integration API
            </Link>
          </CardDescription>
        </CardHeader>
      </Card>

      {newSecret && (
        <Alert>
          <AlertDescription className="space-y-2">
            <p className="font-medium">
              Secret mới — chỉ hiển thị một lần, hãy copy và lưu an toàn:
            </p>
            <code className="block break-all font-mono text-xs">{newSecret}</code>
            <div className="flex flex-wrap gap-2 pt-1">
              <Button type="button" size="sm" variant="outline" onClick={copySecret}>
                <CopyIcon className="mr-1 size-3.5" />
                Copy
              </Button>
              <Button
                type="button"
                size="sm"
                variant="ghost"
                onClick={() => setNewSecret(null)}
              >
                <XIcon className="mr-1 size-3.5" />
                Đóng
              </Button>
            </div>
          </AlertDescription>
        </Alert>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Tạo key</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="key-name">Tên</Label>
            <Input
              id="key-name"
              placeholder="Production CMS"
              value={createForm.name}
              onChange={(e) =>
                setCreateForm((f) => ({ ...f, name: e.target.value }))
              }
              className="max-w-md"
            />
          </div>
          <KeyRestrictionsFields
            form={createForm}
            setForm={setCreateForm}
            folderPickerOpen={createFolderPickerOpen}
            setFolderPickerOpen={setCreateFolderPickerOpen}
          />
          <Button
            onClick={() => createMut.mutate()}
            disabled={!canSubmitCreate || createMut.isPending}
          >
            + Tạo key
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Danh sách</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <LoadingBlock />
          ) : error ? (
            <PageError message="Không tải được danh sách API keys" onRetry={refetch} />
          ) : (
            <DataTable>
              <DataTableHead>
                <DataTableRow>
                  <DataTableTh>Tên</DataTableTh>
                  <DataTableTh>Scopes</DataTableTh>
                  <DataTableTh>Root folder</DataTableTh>
                  <DataTableTh>Allowed IPs</DataTableTh>
                  <DataTableTh>Trạng thái</DataTableTh>
                  <DataTableTh>Lần cuối dùng</DataTableTh>
                  <DataTableTh>Tạo lúc</DataTableTh>
                  <DataTableTh>{" "}</DataTableTh>
                </DataTableRow>
              </DataTableHead>
              <DataTableBody>
                {(data?.items.length ?? 0) === 0 ? (
                  <DataTableEmpty colSpan={8} message="Chưa có API key nào." />
                ) : (
                  data?.items.map((k: APIKey) => (
                    <DataTableRow key={k.public_id}>
                      <DataTableCell className="font-medium">{k.name}</DataTableCell>
                      <DataTableCell>
                        <div className="flex max-w-xs flex-wrap">
                          {scopeBadges(k.scopes)}
                        </div>
                      </DataTableCell>
                      <DataTableCell className="text-sm">
                        {rootFolderLabel(k)}
                      </DataTableCell>
                      <DataTableCell className="text-xs text-muted-foreground">
                        {k.allowed_ips.length > 0
                          ? `${k.allowed_ips.length} IP`
                          : "Không giới hạn IP"}
                      </DataTableCell>
                      <DataTableCell>
                        <Badge
                          variant={k.status === "active" ? "default" : "secondary"}
                        >
                          {k.status === "active" ? "Hoạt động" : "Đã thu hồi"}
                        </Badge>
                      </DataTableCell>
                      <DataTableCell className="text-xs whitespace-nowrap">
                        {formatTimestamp(k.last_used_at)}
                      </DataTableCell>
                      <DataTableCell className="text-xs whitespace-nowrap">
                        {formatTimestamp(k.created_at)}
                      </DataTableCell>
                      <DataTableCell>
                        <div className="flex gap-1">
                          {k.status === "active" && (
                            <>
                              <Button
                                size="sm"
                                variant="outline"
                                onClick={() => setEditKey(k)}
                              >
                                <PencilIcon className="size-3.5" />
                              </Button>
                              <Button
                                size="sm"
                                variant="outline"
                                onClick={() => handleRevoke(k)}
                                disabled={revokeMut.isPending}
                              >
                                Thu hồi
                              </Button>
                            </>
                          )}
                        </div>
                      </DataTableCell>
                    </DataTableRow>
                  ))
                )}
              </DataTableBody>
            </DataTable>
          )}
        </CardContent>
      </Card>

      <Dialog
        open={!!editKey}
        onOpenChange={(v) => {
          if (!v) setEditKey(null);
        }}
      >
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Chỉnh sửa API key</DialogTitle>
            <DialogDescription>
              Secret không thể xem lại. Chỉ cập nhật tên, scopes và giới hạn truy cập.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="edit-key-name">Tên</Label>
              <Input
                id="edit-key-name"
                value={editForm.name}
                onChange={(e) =>
                  setEditForm((f) => ({ ...f, name: e.target.value }))
                }
              />
            </div>
            <KeyRestrictionsFields
              form={editForm}
              setForm={setEditForm}
              folderPickerOpen={editFolderPickerOpen}
              setFolderPickerOpen={setEditFolderPickerOpen}
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setEditKey(null)}>
              Hủy
            </Button>
            <Button
              type="button"
              onClick={() => editMut.mutate()}
              disabled={!canSubmitEdit || editMut.isPending}
            >
              Lưu
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

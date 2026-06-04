"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { PageError, PageLoading } from "@/components/feedback/page-states";
import { toast } from "@/hooks/use-app-toast";
import { authErrorMessage } from "@/hooks/use-auth";
import {
  ApiError,
  fetchSettings,
  SETTINGS_QUERY_KEY,
  UPLOAD_LIMITS_QUERY_KEY,
  updateSettings,
  type SettingsEditable,
  type SettingsPatch,
  type SettingsResponse,
} from "@/lib/api-client";
import { formatBytes } from "@/lib/format";
import { UI_COPY } from "@/lib/ui-copy";
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
import { FormField } from "@/components/ui/form-field";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { PageHeader } from "@/components/ui/page-header";
import { Spinner } from "@/components/ui/spinner";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";

const TABS = [
  "general",
  "media",
  "streaming",
  "storage",
  "security",
  "maintenance",
] as const;

type TabId = (typeof TABS)[number];

function isTabId(v: string | null): v is TabId {
  return TABS.includes(v as TabId);
}

function domainsToText(domains: string[]) {
  return domains.join("\n");
}

function textToDomains(text: string) {
  return text
    .split(/[\n,]+/)
    .map((d) => d.trim())
    .filter(Boolean);
}

function bytesToGbInput(bytes: number) {
  return String(Math.round((bytes / (1024 * 1024 * 1024)) * 100) / 100);
}

function gbInputToBytes(gb: string) {
  const n = parseFloat(gb);
  if (!Number.isFinite(n) || n <= 0) return 0;
  return Math.round(n * 1024 * 1024 * 1024);
}

type FormState = SettingsEditable & {
  domainsText: string;
  maxUploadGb: string;
  quotaGb: string;
};

function toFormState(data: SettingsResponse): FormState {
  const e = data.editable;
  return {
    ...e,
    domainsText: domainsToText(e.streaming.global_allowed_domains),
    maxUploadGb: bytesToGbInput(e.media.max_upload_bytes),
    quotaGb:
      e.storage.quota_bytes > 0 ? bytesToGbInput(e.storage.quota_bytes) : "",
  };
}

function SettingsSaveButton({
  pending,
  onClick,
}: {
  pending: boolean;
  onClick: () => void;
}) {
  return (
    <Button type="button" disabled={pending} onClick={onClick}>
      {pending ? (
        <>
          <Spinner className="size-4" />
          {UI_COPY.processing}
        </>
      ) : (
        "Lưu"
      )}
    </Button>
  );
}

export function SettingsPanel() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const tabParam = searchParams.get("tab");
  const activeTab: TabId = isTabId(tabParam) ? tabParam : "general";

  const queryClient = useQueryClient();
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: SETTINGS_QUERY_KEY,
    queryFn: fetchSettings,
  });

  const [draft, setDraft] = useState<FormState | null>(null);
  const baseForm = data ? toFormState(data) : null;
  const form = draft ?? baseForm;

  const saveMutation = useMutation({
    mutationFn: (patch: SettingsPatch) => updateSettings(patch),
    onSuccess: (resp) => {
      queryClient.setQueryData(SETTINGS_QUERY_KEY, resp);
      void queryClient.invalidateQueries({ queryKey: UPLOAD_LIMITS_QUERY_KEY });
      setDraft(null);
      toast.success(UI_COPY.saveSuccess);
    },
    onError: (err) => {
      toast.error(err instanceof ApiError ? authErrorMessage(err) : "Không thể lưu");
    },
  });

  const setTab = (tab: string) => {
    router.replace(`/settings?tab=${tab}`, { scroll: false });
  };

  if (isLoading) return <PageLoading rows={4} />;
  if (error || !data || !form || !baseForm) {
    return (
      <PageError
        message={
          error instanceof ApiError ? authErrorMessage(error) : UI_COPY.loadError
        }
        onRetry={() => void refetch()}
      />
    );
  }

  const infra = data.readonly.infrastructure;
  const locked = data.readonly.runtime_locked;

  const saveGeneral = () => {
    saveMutation.mutate({
      workspace: {
        name: form.workspace.name,
        public_url: form.workspace.public_url,
      },
      media: {
        default_root_folder_public_id: form.media.default_root_folder_public_id,
      },
    });
  };

  const saveMedia = () => {
    const bytes = gbInputToBytes(form.maxUploadGb);
    if (bytes < 1) {
      toast.error("Kích thước upload tối đa phải lớn hơn 0");
      return;
    }
    saveMutation.mutate({
      media: {
        max_upload_bytes: bytes,
        delete_empty_folders_only: form.media.delete_empty_folders_only,
      },
    });
  };

  const saveStreaming = () => {
    saveMutation.mutate({
      streaming: {
        default_token_ttl_seconds: form.streaming.default_token_ttl_seconds,
        global_allowed_domains: textToDomains(form.domainsText),
      },
    });
  };

  const saveStorage = () => {
    const quota = form.quotaGb.trim() === "" ? 0 : gbInputToBytes(form.quotaGb);
    saveMutation.mutate({ storage: { quota_bytes: quota } });
  };

  const saveSecurity = () => {
    saveMutation.mutate({
      security: {
        login_max_attempts: form.security.login_max_attempts,
        login_lockout_minutes: form.security.login_lockout_minutes,
      },
    });
  };

  const saveMaintenance = () => {
    saveMutation.mutate({
      maintenance: {
        audit_retention_days: form.maintenance.audit_retention_days,
        trash_retention_days: form.maintenance.trash_retention_days,
      },
    });
  };

  const savePending = saveMutation.isPending;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Settings"
        description="Quản lý cấu hình workspace và tích hợp hệ thống. Thông tin hạ tầng (DB, JWT, storage secret) chỉ đọc từ môi trường triển khai."
      />

      <Tabs
        value={activeTab}
        onValueChange={setTab}
        orientation="vertical"
        className="grid w-full grid-cols-1 gap-4 md:grid-cols-[13rem_minmax(0,1fr)] md:gap-8 md:items-start"
      >
        <TabsList
          variant="line"
          className="col-start-1 row-start-1 h-auto w-full shrink-0 md:w-full"
        >
          <TabsTrigger value="general">Chung</TabsTrigger>
          <TabsTrigger value="media">Media</TabsTrigger>
          <TabsTrigger value="streaming">Streaming</TabsTrigger>
          <TabsTrigger value="storage">Storage</TabsTrigger>
          <TabsTrigger value="security">Bảo mật</TabsTrigger>
          <TabsTrigger value="maintenance">Bảo trì</TabsTrigger>
        </TabsList>

        <TabsContent value="general" className="mt-0 space-y-4">
          <Card size="sm">
            <CardHeader>
              <CardTitle>Thông tin workspace</CardTitle>
              <CardDescription>Tên hiển thị và URL công khai của ứng dụng.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <FormField label="Tên workspace">
                <Input
                  value={form.workspace.name}
                  onChange={(e) =>
                    setDraft({
                      ...form,
                      workspace: { ...form.workspace, name: e.target.value },
                    })
                  }
                />
              </FormField>
              <FormField label="Public URL">
                <Input
                  value={form.workspace.public_url}
                  onChange={(e) =>
                    setDraft({
                      ...form,
                      workspace: { ...form.workspace, public_url: e.target.value },
                    })
                  }
                />
              </FormField>
              <FormField
                label="Root folder mặc định (permissions)"
                hint="UUID folder gốc dùng làm resource mặc định trên trang Permissions."
              >
                <Input
                  className="font-mono text-xs"
                  value={form.media.default_root_folder_public_id}
                  onChange={(e) =>
                    setDraft({
                      ...form,
                      media: {
                        ...form.media,
                        default_root_folder_public_id: e.target.value,
                      },
                    })
                  }
                />
              </FormField>
              <SettingsSaveButton pending={savePending} onClick={saveGeneral} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="media" className="mt-0">
          <Card size="sm">
            <CardHeader>
              <CardTitle>Upload & media</CardTitle>
              <CardDescription>
                Giới hạn dung lượng mỗi file khi upload. Mặc định 5 GB; Owner chỉnh
                tại đây — áp dụng toàn hệ thống (API + giao diện chọn file).
                Video convert thủ công theo spec.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <FormField label="Kích thước upload tối đa (GB)">
                <Input
                  type="number"
                  min={0.01}
                  step={0.1}
                  value={form.maxUploadGb}
                  onChange={(e) => setDraft({ ...form, maxUploadGb: e.target.value })}
                />
              </FormField>
              <FormField
                label="Chỉ cho xóa folder rỗng"
                hint="Bật: không xóa folder còn file/con bên trong. Tắt (mặc định): xóa cả cây thư mục."
              >
                <label className="flex cursor-pointer items-center gap-2 text-sm">
                  <Checkbox
                    checked={form.media.delete_empty_folders_only}
                    onChange={(e) =>
                      setDraft({
                        ...form,
                        media: {
                          ...form.media,
                          delete_empty_folders_only: e.target.checked,
                        },
                      })
                    }
                  />
                  Yêu cầu folder trống trước khi xóa
                </label>
              </FormField>
              <p className="text-xs text-muted-foreground">
                Hiện tại: {formatBytes(form.media.max_upload_bytes)}
              </p>
              <SettingsSaveButton pending={savePending} onClick={saveMedia} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="streaming" className="mt-0">
          <Card size="sm">
            <CardHeader>
              <CardTitle>Streaming mặc định</CardTitle>
              <CardDescription>
                Chính sách global; từng video có thể override sau trên Video Manager.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <FormField label="Token TTL mặc định (giây)">
                <Input
                  type="number"
                  min={60}
                  max={86400}
                  value={form.streaming.default_token_ttl_seconds}
                  onChange={(e) =>
                    setDraft({
                      ...form,
                      streaming: {
                        ...form.streaming,
                        default_token_ttl_seconds: Number(e.target.value),
                      },
                    })
                  }
                />
              </FormField>
              <FormField
                label="Domain allowlist (global)"
                hint="Mỗi dòng hoặc cách nhau bởi dấu phẩy. Để trống = không giới hạn domain ở cấp global."
              >
                <Textarea
                  rows={4}
                  value={form.domainsText}
                  onChange={(e) => setDraft({ ...form, domainsText: e.target.value })}
                />
              </FormField>
              <SettingsSaveButton pending={savePending} onClick={saveStreaming} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="storage" className="mt-0 space-y-4">
          <Card size="sm">
            <CardHeader>
              <CardTitle>Quota storage</CardTitle>
              <CardDescription>
                Đặt 0 hoặc để trống = không giới hạn quota runtime (theo DB).
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <FormField label="Quota (GB), để trống = unlimited">
                <Input
                  type="number"
                  min={0}
                  step={1}
                  value={form.quotaGb}
                  onChange={(e) => setDraft({ ...form, quotaGb: e.target.value })}
                />
              </FormField>
              <SettingsSaveButton pending={savePending} onClick={saveStorage} />
            </CardContent>
          </Card>

          <Card size="sm">
            <CardHeader>
              <CardTitle>Hạ tầng object storage</CardTitle>
              <CardDescription>Chỉ đọc — cấu hình qua biến môi trường / .env</CardDescription>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <ReadonlyRow label="Driver" value={infra.storage_driver} />
              <ReadonlyRow label="Endpoint" value={infra.storage_endpoint || "—"} />
              <ReadonlyRow label="Bucket" value={infra.storage_bucket || "—"} />
              <ReadonlyRow label="Region" value={infra.storage_region} />
              <ReadonlyRow label="SSL" value={infra.storage_use_ssl ? "Bật" : "Tắt"} />
              <Button
                nativeButton={false}
                render={<Link href="/system/storage" />}
                variant="outline"
                size="sm"
              >
                Xem dung lượng storage
              </Button>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="security" className="mt-0 space-y-4">
          <Card size="sm">
            <CardHeader>
              <CardTitle>Đăng nhập & khóa tài khoản</CardTitle>
              <CardDescription>
                Áp dụng cho lần đăng nhập tiếp theo (rate limit hiện khởi tạo lúc boot API).
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <FormField label="Số lần đăng nhập sai tối đa">
                <Input
                  type="number"
                  min={1}
                  max={100}
                  value={form.security.login_max_attempts}
                  onChange={(e) =>
                    setDraft({
                      ...form,
                      security: {
                        ...form.security,
                        login_max_attempts: Number(e.target.value),
                      },
                    })
                  }
                />
              </FormField>
              <FormField label="Thời gian khóa (phút)">
                <Input
                  type="number"
                  min={1}
                  max={1440}
                  value={form.security.login_lockout_minutes}
                  onChange={(e) =>
                    setDraft({
                      ...form,
                      security: {
                        ...form.security,
                        login_lockout_minutes: Number(e.target.value),
                      },
                    })
                  }
                />
              </FormField>
              <SettingsSaveButton pending={savePending} onClick={saveSecurity} />
            </CardContent>
          </Card>

          <Card size="sm">
            <CardHeader>
              <CardTitle>Chính sách mật khẩu</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 text-sm">
              <div className="flex items-center justify-between gap-2">
                <span className="text-muted-foreground">Độ dài tối thiểu</span>
                <span>{locked.min_password_length} ký tự</span>
              </div>
              <div className="flex items-center justify-between gap-2">
                <span className="text-muted-foreground">Bắt buộc mã hóa mật khẩu khi login</span>
                {locked.require_encrypted_password ? (
                  <Badge variant="secondary">Bật (production)</Badge>
                ) : (
                  <Badge variant="outline">Tắt</Badge>
                )}
              </div>
              <Alert>
                <AlertDescription>
                  JWT, setup token và secret storage không chỉnh qua giao diện. Redis:{" "}
                  {infra.redis_addr}
                </AlertDescription>
              </Alert>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="maintenance" className="mt-0 space-y-4">
          <Card size="sm">
            <CardHeader>
              <CardTitle>Nhật ký & dọn dẹp</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <FormField label="Giữ audit log (ngày)">
                <Input
                  type="number"
                  min={1}
                  max={3650}
                  value={form.maintenance.audit_retention_days}
                  onChange={(e) =>
                    setDraft({
                      ...form,
                      maintenance: {
                        ...form.maintenance,
                        audit_retention_days: Number(e.target.value),
                      },
                    })
                  }
                />
              </FormField>
              <FormField label="Giữ thùng rác (ngày, 0 = tắt auto-purge)">
                <Input
                  type="number"
                  min={0}
                  max={3650}
                  value={form.maintenance.trash_retention_days}
                  onChange={(e) =>
                    setDraft({
                      ...form,
                      maintenance: {
                        ...form.maintenance,
                        trash_retention_days: Number(e.target.value),
                      },
                    })
                  }
                />
              </FormField>
              <SettingsSaveButton pending={savePending} onClick={saveMaintenance} />
            </CardContent>
          </Card>

          <Card size="sm">
            <CardHeader>
              <CardTitle>Liên kết vận hành</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-wrap gap-2">
              <Button
                nativeButton={false}
                render={<Link href="/system/queue" />}
                variant="outline"
                size="sm"
              >
                Hàng đợi convert
              </Button>
              <Button
                nativeButton={false}
                render={<Link href="/api-keys" />}
                variant="outline"
                size="sm"
              >
                API Keys
              </Button>
              <Button
                nativeButton={false}
                render={<Link href="/docs/integration" />}
                variant="outline"
                size="sm"
              >
                Tài liệu API
              </Button>
              <Button
                nativeButton={false}
                render={<Link href="/system/health" />}
                variant="outline"
                size="sm"
              >
                System Health
              </Button>
            </CardContent>
          </Card>

          <Card size="sm">
            <CardHeader>
              <CardTitle>Hạ tầng</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <ReadonlyRow label="Môi trường" value={infra.app_env} />
              <ReadonlyRow
                label="Database"
                value={infra.db_configured ? "Đã cấu hình" : "Chưa cấu hình"}
              />
              <ReadonlyRow
                label="JWT"
                value={infra.jwt_configured ? "Đã cấu hình" : "Chưa cấu hình"}
              />
              <ReadonlyRow label="FFmpeg" value={infra.ffmpeg_path} />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}

function ReadonlyRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-wrap justify-between gap-2 border-b border-border/60 py-2 last:border-0">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-mono text-xs break-all text-right">{value}</span>
    </div>
  );
}

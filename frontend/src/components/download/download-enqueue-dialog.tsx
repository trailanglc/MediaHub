"use client";

import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import {
  fetchObject,
  fetchSettings,
  OBJECTS_QUERY_KEY,
  ROOT_FOLDER_PUBLIC_ID,
  SETTINGS_QUERY_KEY,
} from "@/lib/api/api-client";
import { FolderTree } from "@/components/files/folder-tree";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { FormField, FormStack } from "@/components/ui/form-field";
import { Input } from "@/components/ui/input";
import { LoadingBlock } from "@/components/ui/loading-block";
import { Spinner } from "@/components/ui/spinner";

const LAST_FOLDER_KEY = "mediahub.download.parentFolderId";

export type DownloadEnqueueValues = {
  title: string;
  parentPublicId: string;
};

export function DownloadEnqueueDialog({
  open,
  defaultTitle,
  subtitle,
  loading,
  onClose,
  onConfirm,
}: {
  open: boolean;
  defaultTitle: string;
  subtitle?: string;
  loading?: boolean;
  onClose: () => void;
  onConfirm: (values: DownloadEnqueueValues) => void;
}) {
  const { data: settings, isLoading: settingsLoading } = useQuery({
    queryKey: SETTINGS_QUERY_KEY,
    queryFn: fetchSettings,
    enabled: open,
  });

  const rootFolderId =
    settings?.editable.media.default_root_folder_public_id ??
    ROOT_FOLDER_PUBLIC_ID;

  const [title, setTitle] = useState(defaultTitle);
  const [folderId, setFolderId] = useState<string | null>(null);
  const [folderName, setFolderName] = useState<string | null>(null);

  const { data: folderMeta, isLoading: folderLoading } = useQuery({
    queryKey: [OBJECTS_QUERY_KEY, "folder", folderId],
    queryFn: () => fetchObject(folderId!),
    enabled: open && !!folderId,
  });

  useEffect(() => {
    if (!open) return;
    setTitle(defaultTitle.trim() || "video");
    let remembered: string | null = null;
    try {
      remembered = localStorage.getItem(LAST_FOLDER_KEY);
    } catch {
      /* ignore */
    }
    if (remembered && remembered !== rootFolderId) {
      setFolderId(remembered);
      setFolderName(null);
    } else {
      setFolderId(null);
      setFolderName(null);
    }
  }, [open, defaultTitle, rootFolderId]);

  useEffect(() => {
    if (folderMeta?.name) {
      setFolderName(folderMeta.name);
    }
  }, [folderMeta?.name]);

  const destinationLabel =
    folderId == null
      ? "Chưa chọn"
      : folderId === rootFolderId
        ? "Root"
        : (folderName ?? "…");

  const canSubmit =
    title.trim().length > 0 &&
    !!folderId &&
    folderId !== rootFolderId &&
    !loading;

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Tải xuống</DialogTitle>
          <DialogDescription>
            Chọn thư mục lưu và tên file. Không lưu trực tiếp vào Root — tạo
            folder trong Files nếu chưa có thư mục con.
            {subtitle ? ` ${subtitle}` : ""}
          </DialogDescription>
        </DialogHeader>

        <FormStack>
          <FormField label="Tên file" hint="Có thể kèm hoặc không kèm phần mở rộng">
            <Input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              autoFocus
              maxLength={255}
              placeholder="Tên video"
            />
          </FormField>

          <FormField
            label="Thư mục lưu"
            hint="Nhấn vào tên folder để chọn (bắt buộc thư mục con)"
          >
            <p className="rounded-md border bg-muted/40 px-3 py-2 text-sm font-medium text-foreground">
              {destinationLabel}
            </p>
            <div className="max-h-64 overflow-y-auto rounded-lg border bg-background p-2">
              {settingsLoading ? (
                <LoadingBlock />
              ) : (
                <FolderTree
                  rootFolderId={rootFolderId}
                  rootName="Root"
                  selectedFolderId={folderId ?? undefined}
                  disabledFolderIds={new Set([rootFolderId])}
                  showHeading={false}
                  onSelect={(id, name) => {
                    if (id === rootFolderId) return;
                    setFolderId(id);
                    setFolderName(name);
                  }}
                />
              )}
            </div>
            {folderLoading && folderId && (
              <p className="text-xs text-muted-foreground">
                Đang tải tên thư mục…
              </p>
            )}
          </FormField>
        </FormStack>

        <DialogFooter className="mt-4">
          <Button type="button" variant="outline" onClick={onClose}>
            Hủy
          </Button>
          <Button
            type="button"
            disabled={!canSubmit}
            onClick={() => {
              if (!folderId || folderId === rootFolderId) return;
              const trimmed = title.trim();
              if (!trimmed) return;
              try {
                localStorage.setItem(LAST_FOLDER_KEY, folderId);
              } catch {
                /* ignore */
              }
              onConfirm({ title: trimmed, parentPublicId: folderId });
            }}
          >
            {loading && <Spinner className="size-4" />}
            Bắt đầu tải
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

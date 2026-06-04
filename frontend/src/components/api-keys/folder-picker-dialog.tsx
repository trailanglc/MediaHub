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
import { LoadingBlock } from "@/components/ui/loading-block";

export function FolderPickerDialog({
  open,
  selectedFolderId,
  onClose,
  onConfirm,
}: {
  open: boolean;
  selectedFolderId?: string | null;
  onClose: () => void;
  onConfirm: (folderId: string | null, folderName: string) => void;
}) {
  const { data: settings, isLoading: settingsLoading } = useQuery({
    queryKey: SETTINGS_QUERY_KEY,
    queryFn: fetchSettings,
    enabled: open,
  });

  const rootFolderId =
    settings?.editable.media.default_root_folder_public_id ?? ROOT_FOLDER_PUBLIC_ID;

  const [pickedId, setPickedId] = useState<string | null>(selectedFolderId ?? null);
  const [pickedName, setPickedName] = useState("Mặc định hệ thống");

  const { data: pickedMeta, isLoading: pickedLoading } = useQuery({
    queryKey: [OBJECTS_QUERY_KEY, "folder", pickedId],
    queryFn: () => fetchObject(pickedId!),
    enabled: open && !!pickedId,
  });

  useEffect(() => {
    if (!open) return;
    setPickedId(selectedFolderId ?? null);
    setPickedName(
      selectedFolderId ? "…" : "Mặc định hệ thống",
    );
  }, [open, selectedFolderId]);

  useEffect(() => {
    if (pickedId && pickedMeta?.name) {
      setPickedName(pickedMeta.name);
    } else if (!pickedId) {
      setPickedName("Mặc định hệ thống");
    }
  }, [pickedId, pickedMeta?.name]);

  const handlePick = (id: string, name: string) => {
    setPickedId(id);
    setPickedName(name);
  };

  const handleUseDefault = () => {
    setPickedId(null);
    setPickedName("Mặc định hệ thống");
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Chọn root folder</DialogTitle>
          <DialogDescription>
            API key chỉ truy cập media trong cây thư mục này. Để trống = dùng root mặc định hệ thống.
          </DialogDescription>
        </DialogHeader>

        <div className="rounded-md border bg-muted/30 p-2 text-sm">
          Đã chọn: <span className="font-medium">{pickedName}</span>
        </div>

        <div className="max-h-64 overflow-y-auto rounded-md border p-2">
          {settingsLoading ? (
            <LoadingBlock />
          ) : (
            <>
              <button
                type="button"
                className="mb-2 w-full rounded-md px-2 py-1.5 text-left text-sm hover:bg-muted"
                onClick={handleUseDefault}
              >
                Mặc định hệ thống
              </button>
              <FolderTree
                rootFolderId={rootFolderId}
                rootName="Root"
                selectedFolderId={pickedId ?? undefined}
                onSelect={handlePick}
                showHeading={false}
              />
            </>
          )}
        </div>

        {pickedLoading && pickedId && (
          <p className="text-xs text-muted-foreground">Đang tải tên thư mục…</p>
        )}

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>
            Hủy
          </Button>
          <Button
            type="button"
            onClick={() => {
              onConfirm(pickedId, pickedName);
              onClose();
            }}
          >
            Xác nhận
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

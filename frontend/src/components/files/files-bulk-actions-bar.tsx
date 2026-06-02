"use client";

import {
  FolderInputIcon,
  PencilIcon,
  Trash2Icon,
  Undo2Icon,
  XIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";

export function FilesBulkActionsBar({
  count,
  canDelete,
  canMove,
  canRename,
  canRestore,
  canPurge,
  trashMode,
  onClear,
  onDelete,
  onMove,
  onRename,
  onRestore,
  onPurge,
}: {
  count: number;
  canDelete: boolean;
  canMove: boolean;
  canRename?: boolean;
  canRestore?: boolean;
  canPurge?: boolean;
  trashMode?: boolean;
  onClear: () => void;
  onDelete: () => void;
  onMove: () => void;
  onRename?: () => void;
  onRestore?: () => void;
  onPurge?: () => void;
}) {
  if (count === 0) return null;

  return (
    <div className="flex flex-col gap-2 rounded-lg border border-primary/30 bg-primary/5 px-3 py-2.5 sm:flex-row sm:flex-wrap sm:items-center">
      <span className="shrink-0 text-sm font-medium">Đã chọn {count} mục</span>
      <div className="flex items-center gap-2 overflow-x-auto pb-0.5 sm:ml-auto sm:flex-wrap sm:overflow-visible sm:pb-0">
        {trashMode ? (
          <>
            {canRestore && onRestore && (
              <Button type="button" size="sm" variant="secondary" onClick={onRestore}>
                <Undo2Icon className="size-4 shrink-0" />
                <span className="whitespace-nowrap">Khôi phục</span>
              </Button>
            )}
            {canPurge && onPurge && (
              <Button type="button" size="sm" variant="destructive" onClick={onPurge}>
                <Trash2Icon className="size-4" />
                Xóa vĩnh viễn
              </Button>
            )}
          </>
        ) : (
          <>
            {canRename && onRename && (
              <Button type="button" size="sm" variant="secondary" onClick={onRename}>
                <PencilIcon className="size-4" />
                Đổi tên
              </Button>
            )}
            {canMove && (
              <Button type="button" size="sm" variant="secondary" onClick={onMove}>
                <FolderInputIcon className="size-4" />
                Di chuyển
              </Button>
            )}
            {canDelete && (
              <Button type="button" size="sm" variant="destructive" onClick={onDelete}>
                <Trash2Icon className="size-4" />
                Xóa
              </Button>
            )}
          </>
        )}
        <Button type="button" size="sm" variant="ghost" onClick={onClear}>
          <XIcon className="size-4" />
          Bỏ chọn
        </Button>
      </div>
    </div>
  );
}

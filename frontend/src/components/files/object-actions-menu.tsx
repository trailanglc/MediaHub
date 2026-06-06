"use client";

import {
  MoreHorizontalIcon,
  PencilIcon,
  FolderInputIcon,
  Share2Icon,
  Trash2Icon,
  EyeIcon,
  Undo2Icon,
  SkullIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { MediaObject } from "@/lib/api-client";

export function ObjectActionsMenu({
  object,
  trashMode,
  onPreview,
  onRename,
  onMove,
  onShare,
  onDelete,
  onRestore,
  onPurge,
}: {
  object: MediaObject;
  trashMode?: boolean;
  onPreview?: () => void;
  onRename?: () => void;
  onMove?: () => void;
  onShare?: () => void;
  onDelete?: () => void;
  onRestore?: () => void;
  onPurge?: () => void;
}) {
  const caps = object.capabilities;
  const hasAny = trashMode
    ? (onRestore && caps.update) || (onPurge && caps.delete)
    : (onPreview && object.type !== "folder") ||
      (onRename && caps.update) ||
      (onMove && caps.update) ||
      (onShare && (caps.manage || caps.share)) ||
      (onDelete && caps.delete);

  if (!hasAny) return null;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button variant="ghost" size="icon-sm" aria-label="Thao tác">
            <MoreHorizontalIcon className="size-4" />
          </Button>
        }
      />
      <DropdownMenuContent align="end">
        {trashMode && onRestore && caps.update && (
          <DropdownMenuItem onClick={onRestore}>
            <Undo2Icon className="size-4" />
            Khôi phục
          </DropdownMenuItem>
        )}
        {trashMode && onPurge && caps.delete && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onClick={onPurge}>
              <SkullIcon className="size-4" />
              Xóa vĩnh viễn
            </DropdownMenuItem>
          </>
        )}
        {!trashMode && onPreview && object.type !== "folder" && (
          <DropdownMenuItem onClick={onPreview}>
            <EyeIcon className="size-4" />
            Xem trước
          </DropdownMenuItem>
        )}
        {!trashMode && onRename && caps.update && (
          <DropdownMenuItem onClick={onRename}>
            <PencilIcon className="size-4" />
            Đổi tên
          </DropdownMenuItem>
        )}
        {!trashMode && onMove && caps.update && (
          <DropdownMenuItem onClick={onMove}>
            <FolderInputIcon className="size-4" />
            Di chuyển
          </DropdownMenuItem>
        )}
        {!trashMode && onShare && (caps.manage || caps.share) && (
          <DropdownMenuItem onClick={onShare}>
            <Share2Icon className="size-4" />
            Chia sẻ / quyền
          </DropdownMenuItem>
        )}
        {!trashMode && onDelete && caps.delete && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onClick={onDelete}>
              <Trash2Icon className="size-4" />
              Xóa
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

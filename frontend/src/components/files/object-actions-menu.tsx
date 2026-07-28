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

  const itemClass =
    "gap-3 px-3 py-2.5 text-[15px] sm:gap-1.5 sm:px-1.5 sm:py-1 sm:text-sm [&_svg:not([class*='size-'])]:size-5 sm:[&_svg:not([class*='size-'])]:size-4";

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button variant="ghost" size="icon-sm" aria-label="Thao tác">
            <MoreHorizontalIcon className="size-4" />
          </Button>
        }
      />
      <DropdownMenuContent
        align="end"
        sideOffset={8}
        className="w-auto min-w-56 p-1.5 sm:min-w-44 sm:p-1"
      >
        {trashMode && onRestore && caps.update && (
          <DropdownMenuItem className={itemClass} onClick={onRestore}>
            <Undo2Icon />
            Khôi phục
          </DropdownMenuItem>
        )}
        {trashMode && onPurge && caps.delete && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              className={itemClass}
              variant="destructive"
              onClick={onPurge}
            >
              <SkullIcon />
              Xóa vĩnh viễn
            </DropdownMenuItem>
          </>
        )}
        {!trashMode && onPreview && object.type !== "folder" && (
          <DropdownMenuItem className={itemClass} onClick={onPreview}>
            <EyeIcon />
            Xem trước
          </DropdownMenuItem>
        )}
        {!trashMode && onRename && caps.update && (
          <DropdownMenuItem className={itemClass} onClick={onRename}>
            <PencilIcon />
            Đổi tên
          </DropdownMenuItem>
        )}
        {!trashMode && onMove && caps.update && (
          <DropdownMenuItem className={itemClass} onClick={onMove}>
            <FolderInputIcon />
            Di chuyển
          </DropdownMenuItem>
        )}
        {!trashMode && onShare && (caps.manage || caps.share) && (
          <DropdownMenuItem className={itemClass} onClick={onShare}>
            <Share2Icon />
            Chia sẻ / quyền
          </DropdownMenuItem>
        )}
        {!trashMode && onDelete && caps.delete && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              className={itemClass}
              variant="destructive"
              onClick={onDelete}
            >
              <Trash2Icon />
              Xóa
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

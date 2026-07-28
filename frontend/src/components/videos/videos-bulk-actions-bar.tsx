"use client";

import {
  FolderIcon,
  FolderXIcon,
  StopCircleIcon,
  Trash2Icon,
  XIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { VideoCategory } from "@/lib/api/api-client";

export function VideosBulkActionsBar({
  count,
  categories,
  canAssignCategory,
  canClearCategory,
  canCancelConvert,
  canDeleteHls,
  canDelete,
  busy,
  onClear,
  onAssignCategory,
  onClearCategory,
  onCancelConvert,
  onDeleteHls,
  onDelete,
}: {
  count: number;
  categories: VideoCategory[];
  canAssignCategory: boolean;
  canClearCategory: boolean;
  canCancelConvert: boolean;
  canDeleteHls: boolean;
  canDelete: boolean;
  busy?: boolean;
  onClear: () => void;
  onAssignCategory: (categoryPublicId: string) => void;
  onClearCategory: () => void;
  onCancelConvert: () => void;
  onDeleteHls: () => void;
  onDelete: () => void;
}) {
  if (count === 0) return null;

  return (
    <div className="flex flex-col gap-2 rounded-lg border border-primary/30 bg-primary/5 px-3 py-2.5 sm:flex-row sm:flex-wrap sm:items-center">
      <span className="shrink-0 text-sm font-medium">Đã chọn {count} video</span>
      <div className="flex items-center gap-2 overflow-x-auto pb-0.5 sm:ml-auto sm:flex-wrap sm:overflow-visible sm:pb-0">
        {canAssignCategory && (
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button
                  type="button"
                  size="sm"
                  variant="secondary"
                  disabled={busy || categories.length === 0}
                >
                  <FolderIcon className="size-4 shrink-0" />
                  <span className="whitespace-nowrap">Xếp loại</span>
                </Button>
              }
            />
            <DropdownMenuContent align="end" className="min-w-44">
              {categories.length === 0 ? (
                <DropdownMenuItem disabled>Chưa có loại</DropdownMenuItem>
              ) : (
                categories.map((c) => (
                  <DropdownMenuItem
                    key={c.public_id}
                    disabled={busy}
                    onClick={() => onAssignCategory(c.public_id)}
                  >
                    {c.name}
                  </DropdownMenuItem>
                ))
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
        {canClearCategory && (
          <Button
            type="button"
            size="sm"
            variant="secondary"
            disabled={busy}
            onClick={onClearCategory}
          >
            <FolderXIcon className="size-4 shrink-0" />
            <span className="whitespace-nowrap">Bỏ phân loại</span>
          </Button>
        )}
        {canCancelConvert && (
          <Button
            type="button"
            size="sm"
            variant="secondary"
            disabled={busy}
            onClick={onCancelConvert}
          >
            <StopCircleIcon className="size-4 shrink-0" />
            <span className="whitespace-nowrap">Hủy convert</span>
          </Button>
        )}
        {canDeleteHls && (
          <Button
            type="button"
            size="sm"
            variant="secondary"
            disabled={busy}
            onClick={onDeleteHls}
          >
            <Trash2Icon className="size-4 shrink-0" />
            <span className="whitespace-nowrap">Xóa HLS</span>
          </Button>
        )}
        {canDelete && (
          <Button
            type="button"
            size="sm"
            variant="destructive"
            disabled={busy}
            onClick={onDelete}
          >
            <Trash2Icon className="size-4" />
            Xóa
          </Button>
        )}
        <Button type="button" size="sm" variant="ghost" onClick={onClear}>
          <XIcon className="size-4" />
          Bỏ chọn
        </Button>
      </div>
    </div>
  );
}

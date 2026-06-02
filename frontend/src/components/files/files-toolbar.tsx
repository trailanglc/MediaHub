"use client";

import {
  FolderPlusIcon,
  GridIcon,
  ListIcon,
  SearchIcon,
  Trash2Icon,
  UploadIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { NativeSelect } from "@/components/ui/native-select";
import type { MediaObjectType } from "@/lib/api-client";
import { cn } from "@/lib/utils";
import type { ReactNode } from "react";

const TYPE_OPTIONS: { value: string; label: string }[] = [
  { value: "", label: "Tất cả" },
  { value: "folder", label: "Folder" },
  { value: "file", label: "File" },
  { value: "image", label: "Ảnh" },
  { value: "video", label: "Video" },
];

export function FilesToolbar({
  search,
  onSearchChange,
  searchScope,
  onSearchScopeChange,
  trashMode,
  onOpenTrash,
  onLeaveTrash,
  onEmptyTrash,
  canEmptyTrash,
  emptyTrashLoading,
  typeFilter,
  onTypeFilterChange,
  viewMode,
  onViewModeChange,
  canUpload,
  onUpload,
  onNewFolder,
  folderTreeTrigger,
}: {
  search: string;
  onSearchChange: (v: string) => void;
  searchScope: "folder" | "global";
  onSearchScopeChange: (v: "folder" | "global") => void;
  trashMode: boolean;
  onOpenTrash: () => void;
  onLeaveTrash: () => void;
  onEmptyTrash?: () => void;
  canEmptyTrash?: boolean;
  emptyTrashLoading?: boolean;
  typeFilter: MediaObjectType | "";
  onTypeFilterChange: (v: MediaObjectType | "") => void;
  viewMode: "grid" | "table";
  onViewModeChange: (v: "grid" | "table") => void;
  canUpload: boolean;
  onUpload: () => void;
  onNewFolder: () => void;
  folderTreeTrigger?: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-3 xl:flex-row xl:flex-wrap xl:items-center xl:gap-2">
      <div className="relative w-full xl:max-w-xs xl:shrink-0">
        <SearchIcon className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          placeholder={
            trashMode
              ? "Tìm trong thùng rác…"
              : searchScope === "global"
                ? "Tìm toàn bộ (≥2 ký tự)…"
                : "Tìm trong thư mục…"
          }
          value={search}
          onChange={(e) => onSearchChange(e.target.value)}
          className="h-10 pl-9 xl:h-8"
        />
      </div>

      <div className="flex min-w-0 flex-1 flex-col gap-2 xl:flex-row xl:flex-wrap xl:items-center xl:justify-end xl:gap-2">
        <div
          className={cn(
            "grid w-full gap-2",
            trashMode ? "max-w-[11rem] grid-cols-1" : "grid-cols-2",
            "xl:flex xl:w-auto xl:max-w-none",
          )}
        >
          {!trashMode && (
            <NativeSelect
              value={searchScope}
              onChange={(e) =>
                onSearchScopeChange(e.target.value as "folder" | "global")
              }
              aria-label="Phạm vi tìm kiếm"
              className="h-9 w-full min-w-0 px-2.5 text-sm xl:h-8 xl:w-auto xl:min-w-[140px]"
            >
              <option value="folder">Thư mục này</option>
              <option value="global">Toàn bộ</option>
            </NativeSelect>
          )}

          <NativeSelect
            value={typeFilter}
            onChange={(e) =>
              onTypeFilterChange(e.target.value as MediaObjectType | "")
            }
            aria-label="Lọc loại file"
            className="h-9 w-full min-w-0 px-2.5 text-sm xl:h-8 xl:w-auto xl:min-w-[120px]"
          >
            {TYPE_OPTIONS.map((o) => (
              <option key={o.value || "all"} value={o.value}>
                {o.label}
              </option>
            ))}
          </NativeSelect>
        </div>

        <div className="-mx-1 flex min-w-0 items-center gap-2 overflow-x-auto px-1 pb-0.5 xl:mx-0 xl:ml-0 xl:flex-initial xl:overflow-visible xl:pb-0">
        {folderTreeTrigger}

        <div className="hidden shrink-0 rounded-md border xl:flex">
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            className={cn(viewMode === "table" && "bg-muted")}
            onClick={() => onViewModeChange("table")}
            aria-label="Chế độ bảng"
          >
            <ListIcon className="size-4" />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            className={cn(viewMode === "grid" && "bg-muted")}
            onClick={() => onViewModeChange("grid")}
            aria-label="Chế độ lưới"
          >
            <GridIcon className="size-4" />
          </Button>
        </div>

        {trashMode ? (
          <>
            {canEmptyTrash && onEmptyTrash && (
              <Button
                type="button"
                variant="destructive"
                size="sm"
                className="min-h-10 shrink-0 xl:min-h-0"
                disabled={emptyTrashLoading}
                onClick={onEmptyTrash}
              >
                <Trash2Icon className="size-4" />
                {emptyTrashLoading ? "Đang dọn…" : "Làm rỗng"}
              </Button>
            )}
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="min-h-10 shrink-0 xl:min-h-0"
              onClick={onLeaveTrash}
            >
              Quay lại
            </Button>
          </>
        ) : (
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="min-h-10 shrink-0 xl:min-h-0"
            onClick={onOpenTrash}
            aria-label="Thùng rác"
          >
            <Trash2Icon className="size-4" />
            <span className="hidden sm:inline">Thùng rác</span>
          </Button>
        )}

        {canUpload && !trashMode && (
          <>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="min-h-10 shrink-0 xl:min-h-0"
              onClick={onNewFolder}
              aria-label="Folder mới"
            >
              <FolderPlusIcon className="size-4" />
              <span className="hidden sm:inline">Folder mới</span>
            </Button>
            <Button
              type="button"
              size="sm"
              className="min-h-10 shrink-0 xl:min-h-0"
              onClick={onUpload}
              aria-label="Upload"
            >
              <UploadIcon className="size-4" />
              <span className="hidden sm:inline">Upload</span>
            </Button>
          </>
        )}
        </div>
      </div>
    </div>
  );
}

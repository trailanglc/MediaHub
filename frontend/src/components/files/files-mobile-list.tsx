"use client";

import {
  FileIcon,
  FileVideoIcon,
  FolderIcon,
  ImageIcon,
} from "lucide-react";
import type { MediaObject } from "@/lib/api-client";
import { isSelectableObject } from "@/lib/files-selection";
import { formatBytes } from "@/lib/format";
import { formatMediaPath } from "@/lib/format-media-path";
import { Checkbox } from "@/components/ui/checkbox";
import { ObjectActionsMenu } from "@/components/files/object-actions-menu";

function TypeIcon({ type }: { type: MediaObject["type"] }) {
  switch (type) {
    case "folder":
      return <FolderIcon className="size-5 shrink-0 text-amber-500" />;
    case "image":
      return <ImageIcon className="size-5 shrink-0 text-sky-500" />;
    case "video":
      return <FileVideoIcon className="size-5 shrink-0 text-violet-500" />;
    default:
      return <FileIcon className="size-5 shrink-0 text-muted-foreground" />;
  }
}

export function FilesMobileList({
  items,
  rootFolderId,
  selectedIds,
  onToggleSelect,
  onOpenFolder,
  onPreview,
  onRename,
  onMove,
  onShare,
  onDelete,
  trashMode,
  showSearchPath,
  onRestore,
  onPurge,
}: {
  items: MediaObject[];
  rootFolderId: string;
  selectedIds: ReadonlySet<string>;
  onToggleSelect: (id: string) => void;
  onOpenFolder: (id: string) => void;
  onPreview: (obj: MediaObject) => void;
  onRename: (obj: MediaObject) => void;
  onMove: (obj: MediaObject) => void;
  onShare: (obj: MediaObject) => void;
  onDelete: (obj: MediaObject) => void;
  trashMode?: boolean;
  showSearchPath?: boolean;
  onRestore?: (obj: MediaObject) => void;
  onPurge?: (obj: MediaObject) => void;
}) {
  if (items.length === 0) {
    return (
      <p className="py-10 text-center text-sm text-muted-foreground">
        {trashMode ? "Thùng rác trống" : "Không có kết quả"}
      </p>
    );
  }

  return (
    <ul className="divide-y divide-border rounded-lg border border-border bg-card">
      {items.map((obj) => {
        const selectable = isSelectableObject(obj, rootFolderId);
        const checked = selectedIds.has(obj.public_id);
        const openItem = () =>
          trashMode || showSearchPath
            ? onPreview(obj)
            : obj.type === "folder"
              ? onOpenFolder(obj.public_id)
              : onPreview(obj);

        return (
          <li
            key={obj.public_id}
            className={checked ? "bg-primary/5" : undefined}
          >
            <div className="flex min-h-14 items-center gap-2 px-3 py-2.5">
              {selectable ? (
                <Checkbox
                  checked={checked}
                  aria-label={`Chọn ${obj.name}`}
                  className="size-5 shrink-0"
                  onChange={() => onToggleSelect(obj.public_id)}
                />
              ) : (
                <span className="size-5 shrink-0" aria-hidden />
              )}
              <button
                type="button"
                className="flex min-w-0 flex-1 items-center gap-3 text-left active:opacity-80"
                onClick={openItem}
              >
                <TypeIcon type={obj.type} />
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium leading-snug">
                    {obj.name}
                  </span>
                  <span className="mt-0.5 block truncate text-xs text-muted-foreground">
                    {showSearchPath && (
                      <>
                        {formatMediaPath(obj.breadcrumbs, rootFolderId)}
                        {" · "}
                      </>
                    )}
                    <span className="capitalize">{obj.type}</span>
                    {obj.type !== "folder" && (
                      <> · {formatBytes(obj.size_bytes)}</>
                    )}
                  </span>
                </span>
              </button>
              <ObjectActionsMenu
                object={obj}
                trashMode={trashMode}
                onPreview={() => onPreview(obj)}
                onRename={() => onRename(obj)}
                onMove={() => onMove(obj)}
                onShare={() => onShare(obj)}
                onDelete={() => onDelete(obj)}
                onRestore={onRestore ? () => onRestore(obj) : undefined}
                onPurge={onPurge ? () => onPurge(obj) : undefined}
              />
            </div>
          </li>
        );
      })}
    </ul>
  );
}

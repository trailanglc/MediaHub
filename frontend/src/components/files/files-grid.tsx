"use client";

import {
  FileIcon,
  FileVideoIcon,
  FolderIcon,
  ImageIcon,
} from "lucide-react";
import type { MediaObject, ObjectAccessURLs } from "@/lib/api-client";
import { isSelectableObject } from "@/lib/files-selection";
import { formatBytes } from "@/lib/format";
import { formatMediaPath } from "@/lib/format-media-path";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { ObjectActionsMenu } from "@/components/files/object-actions-menu";

function GridIcon({
  obj,
  urls,
}: {
  obj: MediaObject;
  urls?: ObjectAccessURLs;
}) {
  const thumb =
    urls?.thumbnail_url ?? urls?.preview_url ?? obj.preview_url ?? obj.download_url;
  if ((obj.type === "image" || obj.type === "file") && thumb) {
    return (
      // eslint-disable-next-line @next/next/no-img-element
      <img
        src={thumb}
        alt=""
        className="aspect-square w-full rounded-md object-cover"
      />
    );
  }
  const iconClass = "size-12 text-muted-foreground";
  switch (obj.type) {
    case "folder":
      return <FolderIcon className={`${iconClass} text-amber-500`} />;
    case "image":
      return <ImageIcon className={`${iconClass} text-sky-500`} />;
    case "video":
      return <FileVideoIcon className={`${iconClass} text-violet-500`} />;
    default:
      return <FileIcon className={iconClass} />;
  }
}

export function FilesGrid({
  items,
  rootFolderId,
  selectedIds,
  onToggleSelect,
  previewUrls,
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
  previewUrls?: Record<string, ObjectAccessURLs>;
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
      <p className="py-12 text-center text-sm text-muted-foreground">
        {trashMode ? "Thùng rác trống" : "Thư mục trống"}
      </p>
    );
  }

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
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
          <Card
            key={obj.public_id}
            className={`overflow-hidden ${checked ? "ring-2 ring-primary/50" : ""}`}
          >
            <button
              type="button"
              className="flex w-full flex-col items-center justify-center bg-muted/40 p-6"
              onClick={openItem}
            >
              <GridIcon obj={obj} urls={previewUrls?.[obj.public_id]} />
            </button>
            <CardHeader className="flex flex-row items-start gap-2 space-y-0 pb-2">
              {selectable ? (
                <Checkbox
                  checked={checked}
                  aria-label={`Chọn ${obj.name}`}
                  className="mt-0.5 shrink-0"
                  onChange={() => onToggleSelect(obj.public_id)}
                  onClick={(e) => e.stopPropagation()}
                />
              ) : (
                <span className="size-[1.125rem] shrink-0" aria-hidden />
              )}
              <CardTitle className="min-w-0 flex-1 truncate text-sm font-medium">
                {obj.name}
              </CardTitle>
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
            </CardHeader>
            <CardContent className="pt-0 text-xs text-muted-foreground">
              {showSearchPath && (
                <p className="mb-1 truncate" title={formatMediaPath(obj.breadcrumbs, rootFolderId)}>
                  {formatMediaPath(obj.breadcrumbs, rootFolderId)}
                </p>
              )}
              <span className="capitalize">{obj.type}</span>
              {obj.type !== "folder" && (
                <> · {formatBytes(obj.size_bytes)}</>
              )}
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}

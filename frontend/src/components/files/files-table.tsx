"use client";

import { useEffect, useRef } from "react";
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
import {
  DataTable,
  DataTableBody,
  DataTableCell,
  DataTableEmpty,
  DataTableHead,
  DataTableRow,
  DataTableTh,
} from "@/components/ui/data-table";
import { Checkbox } from "@/components/ui/checkbox";
import { ObjectActionsMenu } from "@/components/files/object-actions-menu";

function TypeIcon({ type }: { type: MediaObject["type"] }) {
  switch (type) {
    case "folder":
      return <FolderIcon className="size-4 text-amber-500" />;
    case "image":
      return <ImageIcon className="size-4 text-sky-500" />;
    case "video":
      return <FileVideoIcon className="size-4 text-violet-500" />;
    default:
      return <FileIcon className="size-4 text-muted-foreground" />;
  }
}

export function FilesTable({
  items,
  rootFolderId,
  selectedIds,
  onToggleSelect,
  onSelectAll,
  onClearSelection,
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
  onSelectAll: () => void;
  onClearSelection: () => void;
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
  const selectable = items.filter((o) => isSelectableObject(o, rootFolderId));
  const selectedSelectableCount = selectable.filter((o) =>
    selectedIds.has(o.public_id),
  ).length;
  const allSelected =
    selectable.length > 0 && selectedSelectableCount === selectable.length;
  const someSelected =
    selectedSelectableCount > 0 && selectedSelectableCount < selectable.length;

  const headerCheckboxRef = useRef<HTMLInputElement>(null);
  useEffect(() => {
    const el = headerCheckboxRef.current;
    if (el) el.indeterminate = someSelected;
  }, [someSelected]);

  return (
    <DataTable>
      <DataTableHead>
        <DataTableRow>
          <DataTableTh className="w-10">
            <Checkbox
              ref={headerCheckboxRef}
              checked={allSelected}
              disabled={selectable.length === 0}
              aria-label="Chọn tất cả"
              onChange={() => {
                if (allSelected) onClearSelection();
                else onSelectAll();
              }}
            />
          </DataTableTh>
          <DataTableTh>Tên</DataTableTh>
          {showSearchPath && <DataTableTh>Vị trí</DataTableTh>}
          <DataTableTh>Loại</DataTableTh>
          <DataTableTh>Kích thước</DataTableTh>
          <DataTableTh>Cập nhật</DataTableTh>
          <DataTableTh className="w-10"> </DataTableTh>
        </DataTableRow>
      </DataTableHead>
      <DataTableBody>
        {items.length === 0 ? (
          <DataTableEmpty
            colSpan={showSearchPath ? 7 : 6}
            message={trashMode ? "Thùng rác trống" : "Không có kết quả"}
          />
        ) : (
          items.map((obj) => {
            const selectableRow = isSelectableObject(obj, rootFolderId);
            const checked = selectedIds.has(obj.public_id);
            const openItem = () =>
              trashMode || showSearchPath
                ? onPreview(obj)
                : obj.type === "folder"
                  ? onOpenFolder(obj.public_id)
                  : onPreview(obj);
            return (
              <DataTableRow
                key={obj.public_id}
                className={checked ? "bg-primary/5" : undefined}
              >
                <DataTableCell>
                  {selectableRow ? (
                    <Checkbox
                      checked={checked}
                      aria-label={`Chọn ${obj.name}`}
                      onChange={() => onToggleSelect(obj.public_id)}
                      onClick={(e) => e.stopPropagation()}
                    />
                  ) : null}
                </DataTableCell>
                <DataTableCell>
                  <button
                    type="button"
                    className="flex items-center gap-2 text-left hover:underline"
                    onClick={openItem}
                  >
                    <TypeIcon type={obj.type} />
                    <span className="truncate">{obj.name}</span>
                  </button>
                </DataTableCell>
                {showSearchPath && (
                  <DataTableCell className="max-w-[200px] truncate text-muted-foreground">
                    {formatMediaPath(obj.breadcrumbs, rootFolderId)}
                  </DataTableCell>
                )}
                <DataTableCell className="capitalize text-muted-foreground">
                  {obj.type}
                </DataTableCell>
                <DataTableCell className="text-muted-foreground">
                  {obj.type === "folder" ? "—" : formatBytes(obj.size_bytes)}
                </DataTableCell>
                <DataTableCell className="text-muted-foreground">
                  {new Date(obj.updated_at).toLocaleString("vi-VN")}
                </DataTableCell>
                <DataTableCell>
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
                </DataTableCell>
              </DataTableRow>
            );
          })
        )}
      </DataTableBody>
    </DataTable>
  );
}

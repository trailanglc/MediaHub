"use client";

import { useQuery } from "@tanstack/react-query";
import { ChevronDownIcon, ChevronRightIcon, FolderIcon } from "lucide-react";
import { useState } from "react";
import { fetchObjects, OBJECTS_QUERY_KEY } from "@/lib/api-client";
import { cn } from "@/lib/utils";
import { LoadingBlock } from "@/components/ui/loading-block";

function FolderTreeNode({
  folderId,
  name,
  depth,
  currentFolderId,
  selectedFolderId,
  disabledFolderIds,
  onSelect,
}: {
  folderId: string;
  name: string;
  depth: number;
  currentFolderId?: string;
  selectedFolderId?: string;
  disabledFolderIds?: ReadonlySet<string>;
  onSelect: (id: string, name: string) => void;
}) {
  const [expanded, setExpanded] = useState(depth < 1);
  const { data, isLoading } = useQuery({
    queryKey: [OBJECTS_QUERY_KEY, "tree", folderId],
    queryFn: () =>
      fetchObjects({ parent_id: folderId, type: "folder", limit: "100" }),
    enabled: expanded,
  });

  const children = data?.items.filter((o) => o.type === "folder") ?? [];
  const isDisabled = disabledFolderIds?.has(folderId) ?? false;
  const isSelected = selectedFolderId === folderId;
  const isActive = !isSelected && currentFolderId === folderId;

  return (
    <div>
      <div
        className={cn(
          "flex items-center gap-1 rounded-md px-2 py-1 text-sm",
          isDisabled
            ? "cursor-not-allowed opacity-50"
            : "cursor-pointer hover:bg-muted",
          isSelected && "bg-primary/10 font-medium ring-1 ring-primary/25",
          isActive && "bg-muted font-medium",
        )}
        style={{ paddingLeft: `${depth * 12 + 8}px` }}
      >
        <button
          type="button"
          className="shrink-0 p-0.5"
          onClick={(e) => {
            e.stopPropagation();
            setExpanded((v) => !v);
          }}
          aria-label={expanded ? "Thu gọn" : "Mở rộng"}
        >
          {expanded ? (
            <ChevronDownIcon className="size-3.5" />
          ) : (
            <ChevronRightIcon className="size-3.5" />
          )}
        </button>
        <button
          type="button"
          disabled={isDisabled}
          className="flex min-w-0 flex-1 items-center gap-1.5 text-left disabled:pointer-events-none"
          onClick={() => onSelect(folderId, name)}
        >
          <FolderIcon className="size-4 shrink-0 text-amber-500" />
          <span className="truncate">{name}</span>
        </button>
      </div>
      {expanded && isLoading && (
        <div style={{ paddingLeft: `${(depth + 1) * 12 + 8}px` }}>
          <LoadingBlock />
        </div>
      )}
      {expanded &&
        children.map((child) => (
          <FolderTreeNode
            key={child.public_id}
            folderId={child.public_id}
            name={child.name}
            depth={depth + 1}
            currentFolderId={currentFolderId}
            selectedFolderId={selectedFolderId}
            disabledFolderIds={disabledFolderIds}
            onSelect={onSelect}
          />
        ))}
    </div>
  );
}

export function FolderTree({
  rootFolderId,
  rootName,
  currentFolderId,
  selectedFolderId,
  disabledFolderIds,
  onSelect,
  showHeading = true,
}: {
  rootFolderId: string;
  rootName: string;
  currentFolderId?: string;
  selectedFolderId?: string;
  disabledFolderIds?: ReadonlySet<string>;
  onSelect: (id: string, name: string) => void;
  showHeading?: boolean;
}) {
  return (
    <div className="space-y-0.5">
      {showHeading && (
        <p className="mb-2 px-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">
          Thư mục
        </p>
      )}
      <FolderTreeNode
        folderId={rootFolderId}
        name={rootName}
        depth={0}
        currentFolderId={currentFolderId}
        selectedFolderId={selectedFolderId}
        disabledFolderIds={disabledFolderIds}
        onSelect={onSelect}
      />
    </div>
  );
}

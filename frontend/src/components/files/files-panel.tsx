"use client";

import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useObjectPreviewURLs } from "@/hooks/use-object-preview-urls";
import {
  ApiError,
  createFolder,
  deleteObject,
  fetchObject,
  fetchObjects,
  fetchObjectsSearch,
  fetchSettings,
  fetchTrashObjects,
  fetchUploadLimits,
  purgeObject,
  emptyTrash,
  bulkRenameObjects,
  restoreObject,
  UPLOAD_LIMITS_QUERY_KEY,
  OBJECTS_QUERY_KEY,
  patchObject,
  ROOT_FOLDER_PUBLIC_ID,
  SETTINGS_QUERY_KEY,
  type MediaObject,
  type MediaObjectType,
} from "@/lib/api-client";
import { authErrorMessage, isOwner, useMe } from "@/hooks/use-auth";
import { toast } from "@/hooks/use-app-toast";
import { useConfirm } from "@/components/feedback/confirm-provider";
import { PageError } from "@/components/feedback/page-states";
import { FilesBreadcrumb } from "@/components/files/files-breadcrumb";
import { FilesBulkActionsBar } from "@/components/files/files-bulk-actions-bar";
import { FilesGrid } from "@/components/files/files-grid";
import { FilesMobileList } from "@/components/files/files-mobile-list";
import { FilesTable } from "@/components/files/files-table";
import {
  canBulkDelete,
  canBulkMove,
  canBulkPurge,
  canBulkRename,
  canBulkRestore,
  selectableItems,
  selectedObjects,
} from "@/lib/files-selection";
import { FilesToolbar } from "@/components/files/files-toolbar";
import { FolderTree } from "@/components/files/folder-tree";
import { FolderTreeSheet } from "@/components/files/folder-tree-sheet";
import { MoveDialog } from "@/components/files/move-dialog";
import { NewFolderDialog } from "@/components/files/new-folder-dialog";
import { PreviewDrawer } from "@/components/files/preview-drawer";
import { RenameDialog } from "@/components/files/rename-dialog";
import { ShareAccessDialog } from "@/components/files/share-access-dialog";
import { BulkRenameDialog } from "@/components/files/bulk-rename-dialog";
import { UploadDialog } from "@/components/files/upload-dialog";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { LoadingBlock } from "@/components/ui/loading-block";
import { PageHeader } from "@/components/ui/page-header";
import { parentFolderFromObject } from "@/lib/format-media-path";
import type { CreateFolderForm, MoveObjectForm, RenameObjectForm } from "@/lib/schemas/media";

/** Chờ người dùng gõ xong trước khi gọi API */
const SEARCH_DEBOUNCE_MS = 600;
/** Tìm toàn bộ: tối thiểu 2 ký tự để tránh query quá rộng */
const GLOBAL_SEARCH_MIN_LEN = 2;

export function FilesPanel() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();
  const confirm = useConfirm();

  const { data: user } = useMe();
  const { data: settings } = useQuery({
    queryKey: SETTINGS_QUERY_KEY,
    queryFn: fetchSettings,
    enabled: isOwner(user),
    retry: false,
  });

  const { data: uploadLimits } = useQuery({
    queryKey: UPLOAD_LIMITS_QUERY_KEY,
    queryFn: fetchUploadLimits,
    staleTime: 60_000,
  });

  const rootFolderId =
    settings?.editable.media.default_root_folder_public_id ?? ROOT_FOLDER_PUBLIC_ID;

  const folderId = searchParams.get("folder") ?? rootFolderId;
  const trashMode = searchParams.get("trash") === "1";
  const searchScope =
    searchParams.get("scope") === "global" ? "global" : "folder";
  const viewMode = (searchParams.get("view") as "grid" | "table") || "table";
  const typeFilter = (searchParams.get("type") as MediaObjectType | "") || "";
  const searchQ = searchParams.get("q") ?? "";

  const [searchInput, setSearchInput] = useState(searchQ);
  const skipSearchSyncRef = useRef(false);
  const searchContextKey = `${folderId}|${trashMode}|${searchScope}`;
  const prevSearchContextRef = useRef(searchContextKey);

  const updateParams = useCallback(
    (patch: Record<string, string | null>) => {
      const params = new URLSearchParams(searchParams.toString());
      Object.entries(patch).forEach(([k, v]) => {
        if (v === null || v === "") params.delete(k);
        else params.set(k, v);
      });
      router.replace(`/files?${params.toString()}`);
    },
    [router, searchParams],
  );

  // Đồng bộ ô tìm từ URL (back/forward, đổi folder) — không ghi đè khi debounce vừa commit.
  useEffect(() => {
    const contextChanged = prevSearchContextRef.current !== searchContextKey;
    if (contextChanged) {
      prevSearchContextRef.current = searchContextKey;
      setSearchInput(searchQ);
      skipSearchSyncRef.current = false;
      return;
    }
    if (skipSearchSyncRef.current) {
      skipSearchSyncRef.current = false;
      return;
    }
    setSearchInput(searchQ);
  }, [searchQ, searchContextKey]);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      const q = searchInput.trim();
      const committed = searchQ.trim();

      if (q === committed) return;

      const isGlobal = searchScope === "global" && !trashMode;

      if (isGlobal && q.length > 0 && q.length < GLOBAL_SEARCH_MIN_LEN) {
        if (committed) {
          skipSearchSyncRef.current = true;
          updateParams({ q: null });
        }
        return;
      }

      if (isGlobal && q.length === 0 && committed) {
        skipSearchSyncRef.current = true;
        updateParams({ q: null });
        return;
      }

      skipSearchSyncRef.current = true;
      updateParams({ q: q || null });
    }, SEARCH_DEBOUNCE_MS);

    return () => window.clearTimeout(timer);
  }, [searchInput, searchScope, trashMode, searchQ, updateParams]);

  const listMode = trashMode
    ? "trash"
    : searchScope === "global" && searchQ.trim()
      ? "search"
      : "list";

  const showSearchPath =
    listMode === "search" ||
    (!trashMode && searchScope === "folder" && searchQ.trim().length > 0);

  const listQueryKey = [
    OBJECTS_QUERY_KEY,
    listMode,
    folderId,
    searchScope,
    typeFilter,
    searchQ,
  ] as const;

  const {
    data: listPages,
    isLoading,
    isFetchingNextPage,
    error,
    refetch,
    fetchNextPage,
    hasNextPage,
  } = useInfiniteQuery({
    queryKey: listQueryKey,
    queryFn: async ({ pageParam }) => {
      const common = {
        type: typeFilter || undefined,
        cursor: pageParam,
        limit: "50",
      };
      if (trashMode) {
        return fetchTrashObjects({
          ...common,
          q: searchQ.trim() || undefined,
        });
      }
      if (searchScope === "global" && searchQ.trim()) {
        return fetchObjectsSearch({ ...common, q: searchQ.trim() });
      }
      return fetchObjects({
        ...common,
        parent_id: folderId,
        q: searchQ.trim() || undefined,
      });
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) =>
      last.next_cursor != null ? String(last.next_cursor) : undefined,
  });

  const items = useMemo(
    () => listPages?.pages.flatMap((p) => p.items) ?? [],
    [listPages],
  );

  const { data: folderMeta } = useQuery({
    queryKey: [OBJECTS_QUERY_KEY, "meta", folderId],
    queryFn: () => fetchObject(folderId),
  });

  const breadcrumbs = folderMeta?.breadcrumbs ?? [];
  const canUpload = folderMeta?.capabilities.upload ?? false;

  const previewableIds = useMemo(
    () =>
      items
        .filter((o) => o.type !== "folder")
        .map((o) => o.public_id)
        .slice(0, 24),
    [items],
  );
  const { data: previewUrls } = useObjectPreviewURLs(previewableIds);

  const [uploadOpen, setUploadOpen] = useState(false);
  const [folderOpen, setFolderOpen] = useState(false);
  const [renameTarget, setRenameTarget] = useState<MediaObject | null>(null);
  const [moveTarget, setMoveTarget] = useState<MediaObject | null>(null);
  const [bulkMoveTargets, setBulkMoveTargets] = useState<MediaObject[] | null>(
    null,
  );
  const [bulkRenameOpen, setBulkRenameOpen] = useState(false);
  const [bulkRenameLoading, setBulkRenameLoading] = useState(false);
  const [emptyTrashLoading, setEmptyTrashLoading] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());
  const [shareTarget, setShareTarget] = useState<MediaObject | null>(null);
  const [previewTarget, setPreviewTarget] = useState<MediaObject | null>(null);

  const invalidateList = () => {
    queryClient.invalidateQueries({ queryKey: [OBJECTS_QUERY_KEY] });
  };

  const createFolderMutation = useMutation({
    mutationFn: (values: CreateFolderForm) =>
      createFolder({ parent_id: folderId, name: values.name }),
    onSuccess: () => {
      toast.success("Đã tạo folder");
      setFolderOpen(false);
      invalidateList();
    },
    onError: (err) =>
      toast.error(err instanceof ApiError ? authErrorMessage(err) : "Lỗi tạo folder"),
  });

  const renameMutation = useMutation({
    mutationFn: ({ id, name }: { id: string; name: string }) =>
      patchObject(id, { name }),
    onSuccess: () => {
      toast.success("Đã đổi tên");
      setRenameTarget(null);
      invalidateList();
    },
    onError: (err) =>
      toast.error(err instanceof ApiError ? authErrorMessage(err) : "Lỗi đổi tên"),
  });

  const moveMutation = useMutation({
    mutationFn: ({ id, parent_id }: { id: string; parent_id: string }) =>
      patchObject(id, { parent_id }),
    onSuccess: () => {
      toast.success("Đã di chuyển");
      setMoveTarget(null);
      invalidateList();
    },
    onError: (err) =>
      toast.error(err instanceof ApiError ? authErrorMessage(err) : "Lỗi di chuyển"),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteObject(id),
    onSuccess: (res) => {
      const n = res.deleted_count;
      toast.success(n > 1 ? `Đã xóa ${n} mục` : "Đã xóa");
      invalidateList();
    },
    onError: (err) =>
      toast.error(err instanceof ApiError ? authErrorMessage(err) : "Lỗi xóa"),
  });

  const clearSelection = useCallback(() => {
    setSelectedIds(new Set());
  }, []);

  useEffect(() => {
    clearSelection();
  }, [folderId, typeFilter, searchQ, trashMode, searchScope, clearSelection]);

  const selectable = useMemo(
    () => selectableItems(items, rootFolderId),
    [items, rootFolderId],
  );

  const selection = useMemo(
    () => selectedObjects(items, selectedIds),
    [items, selectedIds],
  );

  const toggleSelect = useCallback((id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const selectAllVisible = useCallback(() => {
    setSelectedIds(new Set(selectable.map((o) => o.public_id)));
  }, [selectable]);

  const openFolder = (id: string) => {
    const isRoot = id === rootFolderId;
    updateParams({ folder: isRoot ? null : id, scope: null, q: null });
    if (id === folderId) {
      void queryClient.invalidateQueries({ queryKey: listQueryKey });
    }
  };

  const openFromSearch = (obj: MediaObject) => {
    if (obj.type === "folder") {
      openFolder(obj.public_id);
      return;
    }
    const parentId = parentFolderFromObject(obj, rootFolderId);
    openFolder(parentId);
  };

  const handlePreview = async (obj: MediaObject) => {
    if (showSearchPath) {
      openFromSearch(obj);
      return;
    }
    if (obj.type === "folder") {
      openFolder(obj.public_id);
      return;
    }
    try {
      const detail = await fetchObject(obj.public_id);
      setPreviewTarget(detail);
    } catch {
      setPreviewTarget(obj);
    }
  };

  const handleDelete = async (obj: MediaObject) => {
    const folderNote =
      obj.type === "folder"
        ? " Toàn bộ file và folder bên trong cũng sẽ bị xóa."
        : "";
    const ok = await confirm({
      title: "Xóa mục",
      description: `Xóa «${obj.name}»?${folderNote} Không thể hoàn tác từ giao diện.`,
      confirmLabel: "Xóa",
      variant: "destructive",
    });
    if (ok) deleteMutation.mutate(obj.public_id);
  };

  const handleBulkDelete = async () => {
    const targets = selection.filter((o) => o.capabilities.delete);
    if (targets.length === 0) return;
    const hasFolder = targets.some((o) => o.type === "folder");
    const ok = await confirm({
      title: "Xóa nhiều mục",
      description: hasFolder
        ? `Xóa ${targets.length} mục đã chọn? Folder sẽ xóa cả nội dung bên trong. Không thể hoàn tác từ giao diện.`
        : `Xóa ${targets.length} mục đã chọn? Không thể hoàn tác từ giao diện.`,
      confirmLabel: "Xóa",
      variant: "destructive",
    });
    if (!ok) return;
    let failed = 0;
    let totalDeleted = 0;
    for (const obj of targets) {
      try {
        const res = await deleteObject(obj.public_id);
        totalDeleted += res.deleted_count;
      } catch {
        failed += 1;
      }
    }
    clearSelection();
    invalidateList();
    if (failed > 0) {
      toast.error(`Xóa thất bại ${failed}/${targets.length} lần gọi`);
    } else if (totalDeleted > targets.length) {
      toast.success(`Đã xóa ${totalDeleted} mục (${targets.length} lựa chọn)`);
    } else {
      toast.success(`Đã xóa ${targets.length} mục`);
    }
  };

  const bulkMoveMutation = useMutation({
    mutationFn: async ({
      ids,
      parent_id,
    }: {
      ids: string[];
      parent_id: string;
    }) => {
      const results = await Promise.allSettled(
        ids.map((id) => patchObject(id, { parent_id })),
      );
      const failed = results.filter((r) => r.status === "rejected").length;
      if (failed > 0) {
        throw new Error(`failed:${failed}`);
      }
    },
    onSuccess: () => {
      toast.success("Đã di chuyển");
      setBulkMoveTargets(null);
      clearSelection();
      invalidateList();
    },
    onError: (err) => {
      const msg =
        err instanceof Error && err.message.startsWith("failed:")
          ? `Di chuyển thất bại ${err.message.slice(7)} mục`
          : err instanceof ApiError
            ? authErrorMessage(err)
            : "Lỗi di chuyển";
      toast.error(msg);
      invalidateList();
    },
  });

  const restoreMutation = useMutation({
    mutationFn: (id: string) => restoreObject(id),
    onSuccess: (data) => {
      if (data.restored_count > 1) {
        toast.success(`Đã khôi phục ${data.restored_count} mục`);
      } else {
        toast.success("Đã khôi phục");
      }
      clearSelection();
      invalidateList();
    },
    onError: (err) =>
      toast.error(err instanceof ApiError ? authErrorMessage(err) : "Không khôi phục được"),
  });

  const handleRestore = async (obj: MediaObject) => {
    const folderNote =
      obj.type === "folder"
        ? " Toàn bộ nội dung bên trong cũng sẽ được khôi phục."
        : "";
    const ok = await confirm({
      title: "Khôi phục",
      description: `Khôi phục «${obj.name}»?${folderNote}`,
      confirmLabel: "Khôi phục",
    });
    if (!ok) return;
    restoreMutation.mutate(obj.public_id);
  };

  const handlePurge = async (obj: MediaObject) => {
    const folderNote =
      obj.type === "folder"
        ? " Toàn bộ nội dung trong folder cũng bị xóa vĩnh viễn."
        : "";
    const ok = await confirm({
      title: "Xóa vĩnh viễn",
      description: `Xóa vĩnh viễn «${obj.name}»? Không thể hoàn tác.${folderNote}`,
      confirmLabel: "Xóa vĩnh viễn",
      variant: "destructive",
    });
    if (!ok) return;
    try {
      const { purged_count } = await purgeObject(obj.public_id);
      toast.success(
        purged_count > 1
          ? `Đã xóa vĩnh viễn ${purged_count} mục`
          : "Đã xóa vĩnh viễn",
      );
      clearSelection();
      invalidateList();
    } catch (err) {
      toast.error(err instanceof ApiError ? authErrorMessage(err) : "Không xóa được");
    }
  };

  const handleBulkRestore = async () => {
    const targets = selection.filter((o) => o.capabilities.update);
    const hasFolder = targets.some((o) => o.type === "folder");
    const ok = await confirm({
      title: "Khôi phục hàng loạt",
      description: `Khôi phục ${targets.length} mục?${hasFolder ? " Folder sẽ kèm nội dung bên trong." : ""}`,
      confirmLabel: "Khôi phục",
    });
    if (!ok) return;
    let failed = 0;
    let totalRestored = 0;
    for (const obj of targets) {
      try {
        const { restored_count } = await restoreObject(obj.public_id);
        totalRestored += restored_count;
      } catch {
        failed += 1;
      }
    }
    clearSelection();
    invalidateList();
    if (failed > 0) {
      toast.error(`Khôi phục thất bại ${failed}/${targets.length} lần gọi`);
    } else if (totalRestored > targets.length) {
      toast.success(`Đã khôi phục ${totalRestored} mục (${targets.length} lựa chọn)`);
    } else {
      toast.success(`Đã khôi phục ${targets.length} mục`);
    }
  };

  const handleBulkPurge = async () => {
    const targets = selection.filter((o) => o.capabilities.delete);
    const hasFolder = targets.some((o) => o.type === "folder");
    const ok = await confirm({
      title: "Xóa vĩnh viễn hàng loạt",
      description: `Xóa vĩnh viễn ${targets.length} mục? Không thể hoàn tác.${hasFolder ? " Folder sẽ kèm nội dung bên trong." : ""}`,
      confirmLabel: "Xóa vĩnh viễn",
      variant: "destructive",
    });
    if (!ok) return;
    let failed = 0;
    let totalPurged = 0;
    for (const obj of targets) {
      try {
        const { purged_count } = await purgeObject(obj.public_id);
        totalPurged += purged_count;
      } catch {
        failed += 1;
      }
    }
    clearSelection();
    invalidateList();
    if (failed > 0) {
      toast.error(`Xóa vĩnh viễn thất bại ${failed}/${targets.length} lần gọi`);
    } else if (totalPurged > targets.length) {
      toast.success(`Đã xóa vĩnh viễn ${totalPurged} mục (${targets.length} lựa chọn)`);
    } else {
      toast.success(`Đã xóa vĩnh viễn ${targets.length} mục`);
    }
  };

  const handleEmptyTrash = async () => {
    const ok = await confirm({
      title: "Làm rỗng thùng rác",
      description:
        "Xóa vĩnh viễn toàn bộ file và folder trong thùng rác? Hành động này không thể hoàn tác.",
      confirmLabel: "Làm rỗng",
      variant: "destructive",
    });
    if (!ok) return;
    setEmptyTrashLoading(true);
    try {
      const res = await emptyTrash();
      clearSelection();
      invalidateList();
      if (res.roots_purged === 0) {
        if (res.roots_skipped > 0) {
          toast.error("Không có quyền xóa các mục còn lại trong thùng rác");
        } else {
          toast.info("Thùng rác đã trống");
        }
        return;
      }
      if (res.roots_skipped > 0) {
        toast.warning(
          `Đã xóa vĩnh viễn ${res.purged_count} mục (${res.roots_skipped} mục bị bỏ qua do quyền)`,
        );
      } else if (res.purged_count > res.roots_purged) {
        toast.success(
          `Đã làm rỗng thùng rác — xóa vĩnh viễn ${res.purged_count} mục`,
        );
      } else {
        toast.success(`Đã làm rỗng thùng rác (${res.roots_purged} mục)`);
      }
    } catch (err) {
      toast.error(err instanceof ApiError ? authErrorMessage(err) : "Lỗi dọn thùng rác");
    } finally {
      setEmptyTrashLoading(false);
    }
  };

  const handleBulkRename = async (mode: "prefix" | "suffix", value: string) => {
    const targets = selection.filter((o) => o.capabilities.update);
    if (targets.length === 0) return;
    setBulkRenameLoading(true);
    try {
      const res = await bulkRenameObjects({
        object_ids: targets.map((o) => o.public_id),
        mode,
        value,
      });
      setBulkRenameOpen(false);
      clearSelection();
      invalidateList();
      toast.success(`Đã đổi tên ${res.renamed_count} mục`);
    } catch (err) {
      toast.error(err instanceof ApiError ? authErrorMessage(err) : "Lỗi đổi tên");
    } finally {
      setBulkRenameLoading(false);
    }
  };

  if (error) {
    return (
      <PageError
        message={
          error instanceof ApiError ? authErrorMessage(error) : "Không tải được file"
        }
        onRetry={() => refetch()}
      />
    );
  }

  const listProps = {
    items,
    rootFolderId,
    selectedIds,
    onToggleSelect: toggleSelect,
    trashMode,
    showSearchPath,
    onOpenFolder: openFolder,
    onPreview: handlePreview,
    onRename: setRenameTarget,
    onMove: setMoveTarget,
    onShare: setShareTarget,
    onDelete: handleDelete,
    onRestore: (obj: MediaObject) => void handleRestore(obj),
    onPurge: (obj: MediaObject) => void handlePurge(obj),
  };

  return (
    <div className="space-y-4 md:space-y-6">
      <PageHeader
        title={trashMode ? "Thùng rác" : "File Manager"}
        description={
          <span className="hidden md:inline">
            {trashMode
              ? "Chỉ hiện file và folder đã xóa. Xóa vĩnh viễn sẽ dọn sạch — thùng rác trống khi không còn mục nào."
              : "Quản lý upload, folder và metadata file."}
          </span>
        }
      />

      <div className="flex flex-col gap-4 lg:flex-row lg:gap-6">
        {!trashMode && (
          <Card className="hidden w-full shrink-0 lg:block lg:w-56 xl:w-64">
            <CardContent className="p-3">
              <FolderTree
                rootFolderId={rootFolderId}
                rootName="Root"
                currentFolderId={folderId}
                onSelect={openFolder}
              />
            </CardContent>
          </Card>
        )}

        <div className="min-w-0 flex-1 space-y-4">
          {!trashMode && searchScope === "folder" && (
            <FilesBreadcrumb
              items={breadcrumbs}
              currentFolderId={folderId}
              onNavigate={openFolder}
            />
          )}

          <FilesToolbar
            search={searchInput}
            onSearchChange={setSearchInput}
            searchScope={searchScope}
            onSearchScopeChange={(v) => updateParams({ scope: v === "global" ? "global" : null })}
            trashMode={trashMode}
            onOpenTrash={() =>
              updateParams({ trash: "1", folder: null, scope: null, q: null })
            }
            onLeaveTrash={() => updateParams({ trash: null })}
            onEmptyTrash={() => void handleEmptyTrash()}
            canEmptyTrash={
              trashMode &&
              (items.length > 0 || hasNextPage) &&
              items.some((o) => o.capabilities.delete)
            }
            emptyTrashLoading={emptyTrashLoading}
            typeFilter={typeFilter}
            onTypeFilterChange={(v) => updateParams({ type: v || null })}
            viewMode={viewMode}
            onViewModeChange={(v) => updateParams({ view: v })}
            canUpload={canUpload}
            onUpload={() => setUploadOpen(true)}
            onNewFolder={() => setFolderOpen(true)}
            folderTreeTrigger={
              !trashMode ? (
                <FolderTreeSheet
                  rootFolderId={rootFolderId}
                  currentFolderId={folderId}
                  onSelect={openFolder}
                />
              ) : undefined
            }
          />

          {selectable.length > 0 && (
            <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
              <span className="hidden text-sm text-muted-foreground md:inline">
                Chọn nhiều mục bằng checkbox ở từng dòng hoặc card.
              </span>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="w-full sm:ml-auto sm:w-auto"
                onClick={selectAllVisible}
              >
                Chọn tất cả trang này ({selectable.length})
              </Button>
            </div>
          )}

          {!trashMode ? (
            <FilesBulkActionsBar
              count={selection.length}
              canDelete={canBulkDelete(selection)}
              canMove={canBulkMove(selection)}
              canRename={canBulkRename(selection)}
              onClear={clearSelection}
              onDelete={() => void handleBulkDelete()}
              onMove={() => setBulkMoveTargets(selection)}
              onRename={() => setBulkRenameOpen(true)}
            />
          ) : (
            <FilesBulkActionsBar
              count={selection.length}
              trashMode
              canDelete={false}
              canMove={false}
              canRestore={canBulkRestore(selection)}
              canPurge={canBulkPurge(selection)}
              onClear={clearSelection}
              onDelete={() => {}}
              onMove={() => {}}
              onRestore={() => void handleBulkRestore()}
              onPurge={() => void handleBulkPurge()}
            />
          )}

          {isLoading && !items.length ? (
            <LoadingBlock />
          ) : (
            <>
              <div className="md:hidden">
                <FilesMobileList {...listProps} />
              </div>
              <div className="hidden md:block">
                {viewMode === "grid" ? (
                  <FilesGrid
                    {...listProps}
                    previewUrls={previewUrls}
                  />
                ) : (
                  <FilesTable
                    {...listProps}
                    onSelectAll={selectAllVisible}
                    onClearSelection={clearSelection}
                  />
                )}
              </div>
            </>
          )}

          {hasNextPage && (
            <div className="flex justify-center">
              <Button
                type="button"
                variant="outline"
                onClick={() => fetchNextPage()}
                disabled={isFetchingNextPage}
              >
                {isFetchingNextPage ? "Đang tải…" : "Tải thêm"}
              </Button>
            </div>
          )}
        </div>
      </div>

      <BulkRenameDialog
        open={bulkRenameOpen}
        objects={selection}
        onClose={() => setBulkRenameOpen(false)}
        onApply={(mode, value) => void handleBulkRename(mode, value)}
        loading={bulkRenameLoading}
      />
      <UploadDialog
        open={uploadOpen}
        parentId={folderId}
        maxUploadBytes={uploadLimits?.max_upload_bytes}
        onClose={() => setUploadOpen(false)}
        onSuccess={invalidateList}
      />
      <NewFolderDialog
        open={folderOpen}
        onClose={() => setFolderOpen(false)}
        onSubmit={(v) => createFolderMutation.mutate(v)}
        loading={createFolderMutation.isPending}
      />
      <RenameDialog
        open={!!renameTarget}
        object={renameTarget}
        onClose={() => setRenameTarget(null)}
        onSubmit={(v: RenameObjectForm) => {
          if (renameTarget)
            renameMutation.mutate({ id: renameTarget.public_id, name: v.name });
        }}
        loading={renameMutation.isPending}
      />
      <MoveDialog
        open={!!moveTarget}
        object={moveTarget}
        rootFolderId={rootFolderId}
        rootName="Root"
        browseFolderId={folderId}
        onClose={() => setMoveTarget(null)}
        onSubmit={(v: MoveObjectForm) => {
          if (moveTarget)
            moveMutation.mutate({ id: moveTarget.public_id, parent_id: v.parent_id });
        }}
        loading={moveMutation.isPending}
      />
      <MoveDialog
        open={!!bulkMoveTargets?.length}
        objects={bulkMoveTargets ?? undefined}
        rootFolderId={rootFolderId}
        rootName="Root"
        browseFolderId={folderId}
        onClose={() => setBulkMoveTargets(null)}
        onSubmit={(v: MoveObjectForm) => {
          if (bulkMoveTargets?.length) {
            bulkMoveMutation.mutate({
              ids: bulkMoveTargets.map((o) => o.public_id),
              parent_id: v.parent_id,
            });
          }
        }}
        loading={bulkMoveMutation.isPending}
      />
      <ShareAccessDialog
        open={!!shareTarget}
        object={shareTarget}
        onClose={() => setShareTarget(null)}
      />
      <PreviewDrawer
        object={previewTarget}
        open={!!previewTarget}
        onClose={() => setPreviewTarget(null)}
      />
    </div>
  );
}

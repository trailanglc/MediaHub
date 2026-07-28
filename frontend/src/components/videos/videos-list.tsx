"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import {
  cancelConvertVideo,
  createVideoCategory,
  deleteObject,
  deleteVideoCategory,
  deleteVideoHLS,
  fetchVideoCategories,
  fetchVideos,
  patchVideoCategory,
  patchVideoCategoryAssignment,
  VIDEO_CATEGORIES_QUERY_KEY,
  VIDEOS_QUERY_KEY,
  type HLSStatus,
  type Video,
  type VideoCategory,
} from "@/lib/api/api-client";
import { useConfirm } from "@/components/feedback/confirm-provider";
import { CategoryNameDialog } from "@/components/videos/category-name-dialog";
import { VideoActionsMenu } from "@/components/videos/video-actions-menu";
import { VideosBulkActionsBar } from "@/components/videos/videos-bulk-actions-bar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { PageHeader } from "@/components/ui/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import {
  ChevronRightIcon,
  FilmIcon,
  FolderIcon,
  FolderPlusIcon,
  LayoutGridIcon,
  ListIcon,
  MoreHorizontalIcon,
  PencilIcon,
  SearchIcon,
  Trash2Icon,
  XIcon,
} from "lucide-react";

function normalizeSearch(s: string) {
  return s
    .normalize("NFD")
    .replace(/\p{M}/gu, "")
    .toLowerCase();
}

const PAGE_SIZE = 48;
const SEARCH_DEBOUNCE_MS = 300;
const VIEW_MODE_KEY = "mediahub.videos.viewMode";

type ViewMode = "grid" | "list";

const statusLabel: Record<HLSStatus, string> = {
  none: "Chưa HLS",
  pending: "Đang chờ",
  converting: "Đang chuyển mã",
  ready: "Sẵn sàng",
  failed: "Lỗi",
  deleted: "Đã xóa HLS",
};

function statusVariant(
  status: HLSStatus,
): "default" | "secondary" | "destructive" | "outline" {
  if (status === "ready") return "default";
  if (status === "failed") return "destructive";
  if (status === "converting" || status === "pending") return "secondary";
  return "outline";
}

function VideoGridCard({
  video,
  categories,
  onChanged,
  selecting,
  checked,
  onToggleSelect,
  showCategoryLabel,
}: {
  video: Video;
  categories: VideoCategory[];
  onChanged: () => void;
  selecting: boolean;
  checked: boolean;
  onToggleSelect: () => void;
  showCategoryLabel?: boolean;
}) {
  const [thumbFailed, setThumbFailed] = useState(false);
  const showThumb = Boolean(video.thumbnail_url) && !thumbFailed;
  return (
    <div className="relative min-w-0">
      {selecting && (
        <Checkbox
          checked={checked}
          aria-label={`Chọn ${video.name}`}
          className="absolute left-1 top-1 z-10 bg-background/90 sm:left-2 sm:top-2"
          onChange={onToggleSelect}
          onClick={(e) => e.stopPropagation()}
        />
      )}
      <VideoActionsMenu
        video={video}
        categories={categories}
        onChanged={onChanged}
        triggerClassName="absolute right-1 top-1 sm:right-2 sm:top-2"
      />
      <Link href={`/videos/${video.public_id}`} className="min-w-0 block">
        <Card
          className={cn(
            "overflow-hidden transition-shadow hover:shadow-md",
            selecting && checked && "ring-2 ring-primary/50",
          )}
        >
          <div className="relative aspect-video bg-muted">
            {showThumb ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={video.thumbnail_url}
                alt=""
                className="size-full object-cover"
                onError={() => setThumbFailed(true)}
              />
            ) : (
              <div className="flex size-full items-center justify-center text-muted-foreground">
                <FilmIcon className="size-6 opacity-40 sm:size-8" />
              </div>
            )}
            <Badge
              className={cn(
                "absolute max-w-[70%] truncate px-1.5 py-0 text-[10px] sm:px-2 sm:py-0.5 sm:text-xs",
                selecting
                  ? "bottom-1 left-1 sm:bottom-2 sm:left-2"
                  : "left-1 top-1 sm:left-2 sm:top-2",
              )}
              variant={statusVariant(video.hls_status)}
            >
              {statusLabel[video.hls_status]}
            </Badge>
          </div>
          <CardHeader className="space-y-1 p-2 sm:p-3">
            <CardTitle className="line-clamp-2 text-xs font-medium leading-snug sm:line-clamp-1 sm:text-sm">
              {video.name}
            </CardTitle>
            {showCategoryLabel && video.category_name && (
              <p className="truncate text-[11px] text-muted-foreground sm:text-xs">
                {video.category_name}
              </p>
            )}
          </CardHeader>
        </Card>
      </Link>
    </div>
  );
}

function VideoListRow({
  video,
  categories,
  onChanged,
  selecting,
  checked,
  onToggleSelect,
  showCategoryLabel,
}: {
  video: Video;
  categories: VideoCategory[];
  onChanged: () => void;
  selecting: boolean;
  checked: boolean;
  onToggleSelect: () => void;
  showCategoryLabel?: boolean;
}) {
  const [thumbFailed, setThumbFailed] = useState(false);
  const showThumb = Boolean(video.thumbnail_url) && !thumbFailed;
  return (
    <div
      className={cn(
        "relative flex min-w-0 items-center gap-3 rounded-lg border bg-card p-2 transition-colors hover:bg-muted/40",
        selecting && checked && "ring-2 ring-primary/50",
      )}
    >
      {selecting && (
        <Checkbox
          checked={checked}
          aria-label={`Chọn ${video.name}`}
          className="shrink-0"
          onChange={onToggleSelect}
          onClick={(e) => e.stopPropagation()}
        />
      )}
      <Link
        href={`/videos/${video.public_id}`}
        className="flex min-w-0 flex-1 items-center gap-3"
      >
        <div className="relative aspect-video w-24 shrink-0 overflow-hidden rounded-md bg-muted sm:w-32">
          {showThumb ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={video.thumbnail_url}
              alt=""
              className="size-full object-cover"
              onError={() => setThumbFailed(true)}
            />
          ) : (
            <div className="flex size-full items-center justify-center text-muted-foreground">
              <FilmIcon className="size-5 opacity-40" />
            </div>
          )}
        </div>
        <div className="min-w-0 flex-1 space-y-1">
          <p className="line-clamp-2 text-sm font-medium leading-snug">
            {video.name}
          </p>
          <div className="flex flex-wrap items-center gap-1.5">
            <Badge
              className="truncate"
              variant={statusVariant(video.hls_status)}
            >
              {statusLabel[video.hls_status]}
            </Badge>
            {showCategoryLabel && video.category_name && (
              <span className="truncate text-xs text-muted-foreground">
                {video.category_name}
              </span>
            )}
          </div>
        </div>
      </Link>
      <div className="relative shrink-0">
        <VideoActionsMenu
          video={video}
          categories={categories}
          onChanged={onChanged}
        />
      </div>
    </div>
  );
}

function CategoryGridCard({
  category,
  onOpen,
  onRename,
  onDelete,
}: {
  category: VideoCategory;
  onOpen: () => void;
  onRename: () => void;
  onDelete: () => void;
}) {
  return (
    <div className="relative min-w-0">
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              className="absolute right-1 top-1 z-10 bg-background/80 sm:right-2 sm:top-2"
              aria-label="Thao tác loại"
              onClick={(e) => e.stopPropagation()}
            >
              <MoreHorizontalIcon className="size-4" />
            </Button>
          }
        />
        <DropdownMenuContent align="end" className="min-w-40">
          <DropdownMenuItem onClick={onRename}>
            <PencilIcon className="size-4" />
            Đổi tên
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem variant="destructive" onClick={onDelete}>
            <Trash2Icon className="size-4" />
            Xóa loại
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      <button type="button" onClick={onOpen} className="w-full text-left">
        <Card className="overflow-hidden transition-shadow hover:shadow-md">
          <div className="relative flex aspect-video items-center justify-center bg-muted">
            <FolderIcon className="size-10 text-muted-foreground opacity-70 sm:size-12" />
          </div>
          <CardHeader className="p-2 sm:p-3">
            <CardTitle className="line-clamp-2 text-xs font-medium leading-snug sm:line-clamp-1 sm:text-sm">
              {category.name}
            </CardTitle>
            <p className="text-[11px] text-muted-foreground sm:text-xs">
              {category.video_count} video
            </p>
          </CardHeader>
        </Card>
      </button>
    </div>
  );
}

function CategoryListRow({
  category,
  onOpen,
  onRename,
  onDelete,
}: {
  category: VideoCategory;
  onOpen: () => void;
  onRename: () => void;
  onDelete: () => void;
}) {
  return (
    <div className="flex min-w-0 items-center gap-3 rounded-lg border bg-card p-2 transition-colors hover:bg-muted/40">
      <button
        type="button"
        onClick={onOpen}
        className="flex min-w-0 flex-1 items-center gap-3 text-left"
      >
        <div className="flex aspect-video w-24 shrink-0 items-center justify-center rounded-md bg-muted sm:w-32">
          <FolderIcon className="size-7 text-muted-foreground opacity-70" />
        </div>
        <div className="min-w-0 flex-1 space-y-0.5">
          <p className="line-clamp-2 text-sm font-medium leading-snug">
            {category.name}
          </p>
          <p className="text-xs text-muted-foreground">
            {category.video_count} video
          </p>
        </div>
      </button>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label="Thao tác loại"
            >
              <MoreHorizontalIcon className="size-4" />
            </Button>
          }
        />
        <DropdownMenuContent align="end" className="min-w-40">
          <DropdownMenuItem onClick={onRename}>
            <PencilIcon className="size-4" />
            Đổi tên
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem variant="destructive" onClick={onDelete}>
            <Trash2Icon className="size-4" />
            Xóa loại
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

function readStoredViewMode(): ViewMode {
  try {
    const v = localStorage.getItem(VIEW_MODE_KEY);
    if (v === "list" || v === "grid") return v;
  } catch {
    /* ignore */
  }
  return "grid";
}

export function VideosList() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const categoryId = searchParams.get("category");
  const qc = useQueryClient();
  const confirm = useConfirm();

  const [searchInput, setSearchInput] = useState("");
  const [q, setQ] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("");
  const [viewMode, setViewMode] = useState<ViewMode>("grid");
  const [createOpen, setCreateOpen] = useState(false);
  const [renameTarget, setRenameTarget] = useState<VideoCategory | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());
  const [bulkBusy, setBulkBusy] = useState(false);
  const [selecting, setSelecting] = useState(false);

  useEffect(() => {
    setViewMode(readStoredViewMode());
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setQ(searchInput.trim());
    }, SEARCH_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

  useEffect(() => {
    setSelectedIds(new Set());
    setSelecting(false);
  }, [categoryId, q, statusFilter]);

  function exitSelecting() {
    setSelecting(false);
    setSelectedIds(new Set());
  }

  function changeViewMode(mode: ViewMode) {
    setViewMode(mode);
    try {
      localStorage.setItem(VIEW_MODE_KEY, mode);
    } catch {
      /* ignore */
    }
  }

  function openCategory(id: string) {
    router.push(`/videos?category=${encodeURIComponent(id)}`);
  }

  function goRoot() {
    router.push("/videos");
  }

  function clearSelection() {
    setSelectedIds(new Set());
  }

  function toggleSelect(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleSelectAll(ids: string[]) {
    setSelectedIds((prev) => {
      const allSelected =
        ids.length > 0 && ids.every((id) => prev.has(id));
      if (allSelected) return new Set();
      return new Set(ids);
    });
  }

  const categoriesQuery = useQuery({
    queryKey: [VIDEO_CATEGORIES_QUERY_KEY],
    queryFn: fetchVideoCategories,
  });
  const categories = categoriesQuery.data?.items ?? [];
  const activeCategory = categoryId
    ? categories.find((c) => c.public_id === categoryId)
    : undefined;

  const {
    data,
    isLoading,
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteQuery({
    queryKey: [
      VIDEOS_QUERY_KEY,
      "list",
      q,
      statusFilter,
      categoryId ?? (q || statusFilter ? "all" : "uncategorized"),
    ],
    queryFn: ({ pageParam }) => {
      const category = categoryId
        ? categoryId
        : q || statusFilter
          ? "all"
          : "uncategorized";
      return fetchVideos({
        q: q || undefined,
        hls_status: statusFilter || undefined,
        limit: String(PAGE_SIZE),
        cursor: pageParam ? String(pageParam) : undefined,
        category,
      });
    },
    initialPageParam: undefined as number | undefined,
    getNextPageParam: (last) => last.next_cursor,
  });

  const items = data?.pages.flatMap((p) => p.items) ?? [];
  const browsingRoot = !categoryId && !q && !statusFilter;
  const searchingGlobally = Boolean(q) && !categoryId;
  const showCategoryLabel = !categoryId && Boolean(q || statusFilter);

  const matchedCategories = useMemo(() => {
    if (!searchingGlobally) return [];
    const nq = normalizeSearch(q);
    if (!nq) return [];
    return categories.filter((c) => normalizeSearch(c.name).includes(nq));
  }, [categories, q, searchingGlobally]);

  const rootCategories = browsingRoot ? categories : matchedCategories;

  const selection = useMemo(
    () => items.filter((v) => selectedIds.has(v.public_id)),
    [items, selectedIds],
  );
  const allItemIds = useMemo(() => items.map((v) => v.public_id), [items]);
  const selectedCount = selection.length;
  const allSelected =
    items.length > 0 && items.every((v) => selectedIds.has(v.public_id));
  const someSelected =
    selectedCount > 0 && selectedCount < items.length;

  const canBulkAssign = selection.some((v) => v.capabilities.update);
  const canBulkClearCategory = selection.some(
    (v) => v.capabilities.update && v.category_public_id,
  );
  const canBulkCancel = selection.some(
    (v) =>
      v.capabilities.update &&
      (v.hls_status === "pending" || v.hls_status === "converting"),
  );
  const canBulkDeleteHls = selection.some(
    (v) => v.capabilities.delete && v.hls_status === "ready",
  );
  const canBulkDelete = selection.some((v) => v.capabilities.delete);

  function invalidateLists() {
    void qc.invalidateQueries({ queryKey: [VIDEOS_QUERY_KEY] });
    void qc.invalidateQueries({ queryKey: [VIDEO_CATEGORIES_QUERY_KEY] });
  }

  async function runBulk(
    targets: Video[],
    action: (v: Video) => Promise<unknown>,
    successMsg: (ok: number) => string,
  ) {
    if (targets.length === 0) return;
    setBulkBusy(true);
    let failed = 0;
    try {
      for (const v of targets) {
        try {
          await action(v);
        } catch {
          failed += 1;
        }
      }
      clearSelection();
      invalidateLists();
      const ok = targets.length - failed;
      if (failed > 0) {
        toast.error(`Thất bại ${failed}/${targets.length}`);
      } else if (ok > 0) {
        toast.success(successMsg(ok));
      }
    } finally {
      setBulkBusy(false);
    }
  }

  async function handleBulkAssign(categoryPublicId: string) {
    const targets = selection.filter((v) => v.capabilities.update);
    await runBulk(
      targets,
      (v) => patchVideoCategoryAssignment(v.public_id, categoryPublicId),
      (n) => `Đã xếp loại ${n} video`,
    );
  }

  async function handleBulkClearCategory() {
    const targets = selection.filter(
      (v) => v.capabilities.update && v.category_public_id,
    );
    await runBulk(
      targets,
      (v) => patchVideoCategoryAssignment(v.public_id, null),
      (n) => `Đã bỏ phân loại ${n} video`,
    );
  }

  async function handleBulkCancelConvert() {
    const targets = selection.filter(
      (v) =>
        v.capabilities.update &&
        (v.hls_status === "pending" || v.hls_status === "converting"),
    );
    await runBulk(
      targets,
      (v) => cancelConvertVideo(v.public_id),
      (n) => `Đã gửi hủy convert ${n} video`,
    );
  }

  async function handleBulkDeleteHls() {
    const targets = selection.filter(
      (v) => v.capabilities.delete && v.hls_status === "ready",
    );
    if (targets.length === 0) return;
    const ok = await confirm({
      title: "Xóa HLS hàng loạt?",
      description: `Xóa bản HLS của ${targets.length} video đã chọn? File gốc vẫn giữ.`,
      confirmLabel: "Xóa HLS",
      variant: "destructive",
    });
    if (!ok) return;
    await runBulk(
      targets,
      (v) => deleteVideoHLS(v.public_id),
      (n) => `Đã xóa HLS ${n} video`,
    );
  }

  async function handleBulkDelete() {
    const targets = selection.filter((v) => v.capabilities.delete);
    if (targets.length === 0) return;
    const ok = await confirm({
      title: "Xóa video hàng loạt?",
      description: `Chuyển ${targets.length} video đã chọn vào thùng rác (cùng file gốc trong File Manager).`,
      confirmLabel: "Xóa",
      variant: "destructive",
    });
    if (!ok) return;
    await runBulk(
      targets,
      (v) => deleteObject(v.public_id),
      (n) => `Đã xóa ${n} video`,
    );
  }

  const createMut = useMutation({
    mutationFn: (name: string) => createVideoCategory(name),
    onSuccess: () => {
      toast.success("Đã tạo loại");
      setCreateOpen(false);
      invalidateLists();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const renameMut = useMutation({
    mutationFn: ({ id, name }: { id: string; name: string }) =>
      patchVideoCategory(id, name),
    onSuccess: () => {
      toast.success("Đã đổi tên loại");
      setRenameTarget(null);
      invalidateLists();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => deleteVideoCategory(id),
    onSuccess: () => {
      toast.success("Đã xóa loại");
      if (categoryId) goRoot();
      invalidateLists();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  async function handleDeleteCategory(cat: VideoCategory) {
    const ok = await confirm({
      title: "Xóa loại?",
      description:
        "Video trong loại sẽ về chưa phân loại. Bản thân video không bị xóa.",
      confirmLabel: "Xóa loại",
      variant: "destructive",
    });
    if (ok) deleteMut.mutate(cat.public_id);
  }

  const empty =
    !isLoading &&
    items.length === 0 &&
    rootCategories.length === 0;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Videos"
        description="Quản lý video và chuyển mã HLS thủ công."
        actions={
          <div className="flex flex-wrap items-center gap-2">
            {items.length > 0 && (
              <Button
                type="button"
                variant={selecting ? "secondary" : "outline"}
                onClick={() => {
                  if (selecting) exitSelecting();
                  else setSelecting(true);
                }}
              >
                {selecting ? "Xong" : "Chọn"}
              </Button>
            )}
            <Button type="button" onClick={() => setCreateOpen(true)}>
              <FolderPlusIcon className="size-4" />
              Tạo loại
            </Button>
          </div>
        }
      />

      {categoryId && (
        <nav className="flex flex-wrap items-center gap-1 text-sm text-muted-foreground">
          <button
            type="button"
            onClick={goRoot}
            className="hover:text-foreground"
          >
            Videos
          </button>
          <ChevronRightIcon className="size-3.5 shrink-0" />
          <span className="font-medium text-foreground">
            {activeCategory?.name ?? "Loại"}
          </span>
        </nav>
      )}

      <div className="flex flex-col gap-2">
        <div className="relative max-w-md">
          <SearchIcon className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder={
              categoryId
                ? "Tìm trong loại này…"
                : "Tìm video hoặc loại theo tên…"
            }
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            className="pr-9 pl-9"
            aria-label="Tìm kiếm"
          />
          {searchInput && (
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              className="absolute top-1/2 right-1 -translate-y-1/2"
              aria-label="Xóa tìm kiếm"
              onClick={() => {
                setSearchInput("");
                setQ("");
              }}
            >
              <XIcon className="size-4" />
            </Button>
          )}
        </div>
        {(q || statusFilter) && !categoryId && (
          <p className="text-xs text-muted-foreground">
            {q
              ? "Đang tìm trên toàn bộ video (kể cả đã phân loại)."
              : "Đang lọc theo trạng thái trên toàn bộ video."}
          </p>
        )}
        <div className="flex items-center gap-2">
          <select
            className="h-9 min-w-0 flex-1 rounded-md border bg-background px-3 text-sm sm:max-w-xs sm:flex-none"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
          >
            <option value="">Tất cả trạng thái</option>
            {(Object.keys(statusLabel) as HLSStatus[]).map((s) => (
              <option key={s} value={s}>
                {statusLabel[s]}
              </option>
            ))}
          </select>
          <div className="ml-auto flex shrink-0 rounded-md border">
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              className={cn(viewMode === "list" && "bg-muted")}
              onClick={() => changeViewMode("list")}
              aria-label="Dạng danh sách"
              aria-pressed={viewMode === "list"}
            >
              <ListIcon className="size-4" />
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              className={cn(viewMode === "grid" && "bg-muted")}
              onClick={() => changeViewMode("grid")}
              aria-label="Dạng lưới"
              aria-pressed={viewMode === "grid"}
            >
              <LayoutGridIcon className="size-4" />
            </Button>
          </div>
        </div>
      </div>

      {selecting && items.length > 0 && (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Checkbox
            checked={allSelected}
            ref={(el) => {
              if (el) el.indeterminate = someSelected;
            }}
            aria-label="Chọn tất cả video đang hiện"
            onChange={() => toggleSelectAll(allItemIds)}
          />
          <span>Chọn tất cả ({items.length})</span>
        </div>
      )}

      {selecting && (
        <VideosBulkActionsBar
          count={selectedCount}
          categories={categories}
          canAssignCategory={canBulkAssign}
          canClearCategory={canBulkClearCategory}
          canCancelConvert={canBulkCancel}
          canDeleteHls={canBulkDeleteHls}
          canDelete={canBulkDelete}
          busy={bulkBusy}
          onClear={clearSelection}
          onAssignCategory={(id) => void handleBulkAssign(id)}
          onClearCategory={() => void handleBulkClearCategory()}
          onCancelConvert={() => void handleBulkCancelConvert()}
          onDeleteHls={() => void handleBulkDeleteHls()}
          onDelete={() => void handleBulkDelete()}
        />
      )}

      {(error || categoriesQuery.error) && (
        <p className="text-sm text-destructive">
          Không tải được danh sách video.
        </p>
      )}

      {isLoading || (browsingRoot && categoriesQuery.isLoading) ? (
        viewMode === "grid" ? (
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 sm:gap-3 lg:grid-cols-4 lg:gap-4">
            {Array.from({ length: 8 }).map((_, i) => (
              <Skeleton key={i} className="aspect-video rounded-xl" />
            ))}
          </div>
        ) : (
          <div className="space-y-2">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-16 rounded-lg" />
            ))}
          </div>
        )
      ) : (
        <>
          {viewMode === "grid" ? (
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 sm:gap-3 lg:grid-cols-4 lg:gap-4">
              {rootCategories.map((c) => (
                <CategoryGridCard
                  key={c.public_id}
                  category={c}
                  onOpen={() => openCategory(c.public_id)}
                  onRename={() => setRenameTarget(c)}
                  onDelete={() => void handleDeleteCategory(c)}
                />
              ))}
              {items.map((v) => (
                <VideoGridCard
                  key={v.public_id}
                  video={v}
                  categories={categories}
                  onChanged={invalidateLists}
                  selecting={selecting}
                  checked={selectedIds.has(v.public_id)}
                  onToggleSelect={() => toggleSelect(v.public_id)}
                  showCategoryLabel={showCategoryLabel}
                />
              ))}
            </div>
          ) : (
            <div className="space-y-2">
              {rootCategories.map((c) => (
                <CategoryListRow
                  key={c.public_id}
                  category={c}
                  onOpen={() => openCategory(c.public_id)}
                  onRename={() => setRenameTarget(c)}
                  onDelete={() => void handleDeleteCategory(c)}
                />
              ))}
              {items.map((v) => (
                <VideoListRow
                  key={v.public_id}
                  video={v}
                  categories={categories}
                  onChanged={invalidateLists}
                  selecting={selecting}
                  checked={selectedIds.has(v.public_id)}
                  onToggleSelect={() => toggleSelect(v.public_id)}
                  showCategoryLabel={showCategoryLabel}
                />
              ))}
            </div>
          )}
          {hasNextPage && (
            <div className="flex justify-center">
              <Button
                variant="outline"
                onClick={() => void fetchNextPage()}
                disabled={isFetchingNextPage}
              >
                {isFetchingNextPage ? "Đang tải…" : "Tải thêm"}
              </Button>
            </div>
          )}
        </>
      )}

      {empty && (
        <Card>
          <CardContent className="py-10 text-center text-sm text-muted-foreground">
            {q
              ? categoryId
                ? `Không có video khớp “${q}” trong loại này.`
                : `Không tìm thấy video hoặc loại khớp “${q}”.`
              : categoryId
                ? "Loại này chưa có video."
                : statusFilter
                  ? "Không có video với trạng thái này."
                  : "Chưa có video. Upload file video trong File Manager hoặc tạo loại để phân loại."}
          </CardContent>
        </Card>
      )}

      <CategoryNameDialog
        open={createOpen}
        title="Tạo loại"
        submitLabel="Tạo"
        onClose={() => setCreateOpen(false)}
        onSubmit={(name) => createMut.mutate(name)}
        loading={createMut.isPending}
      />
      <CategoryNameDialog
        open={Boolean(renameTarget)}
        title="Đổi tên loại"
        initialName={renameTarget?.name ?? ""}
        onClose={() => setRenameTarget(null)}
        onSubmit={(name) => {
          if (!renameTarget) return;
          renameMut.mutate({ id: renameTarget.public_id, name });
        }}
        loading={renameMut.isPending}
      />
    </div>
  );
}

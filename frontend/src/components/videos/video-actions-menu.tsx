"use client";

import { useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  CopyIcon,
  FolderIcon,
  FolderOpenIcon,
  FolderXIcon,
  MoreHorizontalIcon,
  PencilIcon,
  StopCircleIcon,
  Trash2Icon,
} from "lucide-react";
import { toast } from "sonner";
import { useConfirm } from "@/components/feedback/confirm-provider";
import { RenameDialog } from "@/components/files/rename-dialog";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  cancelConvertVideo,
  deleteObject,
  deleteVideoHLS,
  fetchVideoCategories,
  patchObject,
  patchVideoCategoryAssignment,
  VIDEO_CATEGORIES_QUERY_KEY,
  VIDEOS_QUERY_KEY,
  type MediaObject,
  type Video,
  type VideoCategory,
} from "@/lib/api/api-client";
import { cn } from "@/lib/utils";
import { useState } from "react";

export function VideoActionsMenu({
  video,
  categories: categoriesProp,
  onChanged,
  triggerClassName,
  align = "end",
}: {
  video: Video;
  categories?: VideoCategory[];
  onChanged?: () => void;
  triggerClassName?: string;
  align?: "start" | "end";
}) {
  const router = useRouter();
  const qc = useQueryClient();
  const confirm = useConfirm();
  const [renameOpen, setRenameOpen] = useState(false);

  const categoriesQuery = useQuery({
    queryKey: [VIDEO_CATEGORIES_QUERY_KEY],
    queryFn: fetchVideoCategories,
    enabled: categoriesProp === undefined && Boolean(video.capabilities.update),
  });
  const categories = categoriesProp ?? categoriesQuery.data?.items ?? [];

  function invalidate() {
    void qc.invalidateQueries({ queryKey: [VIDEOS_QUERY_KEY, video.public_id] });
    void qc.invalidateQueries({ queryKey: [VIDEOS_QUERY_KEY] });
    void qc.invalidateQueries({ queryKey: [VIDEO_CATEGORIES_QUERY_KEY] });
    onChanged?.();
  }

  const renameMut = useMutation({
    mutationFn: (name: string) => patchObject(video.public_id, { name }),
    onSuccess: () => {
      toast.success("Đã đổi tên");
      setRenameOpen(false);
      invalidate();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const deleteMut = useMutation({
    mutationFn: () => deleteObject(video.public_id),
    onSuccess: () => {
      toast.success("Đã chuyển video vào thùng rác");
      invalidate();
      router.push("/videos");
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const deleteHlsMut = useMutation({
    mutationFn: () => deleteVideoHLS(video.public_id),
    onSuccess: () => {
      toast.success("Đã xóa bản HLS");
      invalidate();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const cancelMut = useMutation({
    mutationFn: () => cancelConvertVideo(video.public_id),
    onSuccess: () => {
      toast.message("Đã gửi lệnh hủy convert");
      invalidate();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const assignMut = useMutation({
    mutationFn: (categoryPublicId: string | null) =>
      patchVideoCategoryAssignment(video.public_id, categoryPublicId),
    onSuccess: () => {
      toast.success("Đã cập nhật loại");
      invalidate();
    },
    onError: (e: Error) => toast.error(e.message),
  });

  const isConverting =
    video.hls_status === "pending" || video.hls_status === "converting";
  const canUpdate = video.capabilities.update;
  const canDelete = video.capabilities.delete;
  const canLocate = Boolean(video.parent_public_id);

  const renameObject = {
    public_id: video.public_id,
    name: video.name,
  } as MediaObject;

  async function handleDelete() {
    const ok = await confirm({
      title: "Xóa video?",
      description:
        "Video sẽ vào thùng rác cùng file gốc trong File Manager. Có thể khôi phục sau.",
      confirmLabel: "Xóa",
      variant: "destructive",
    });
    if (ok) deleteMut.mutate();
  }

  async function handleDeleteHls() {
    const ok = await confirm({
      title: "Xóa bản HLS?",
      description: "File gốc vẫn giữ. Có thể convert lại sau.",
      confirmLabel: "Xóa HLS",
      variant: "destructive",
    });
    if (ok) deleteHlsMut.mutate();
  }

  async function copyId() {
    await navigator.clipboard.writeText(video.public_id);
    toast.success("Đã copy public ID");
  }

  function locateInFiles() {
    const folder = video.parent_public_id;
    if (!folder) {
      toast.error("Không xác định được thư mục chứa file");
      return;
    }
    router.push(
      `/files?folder=${encodeURIComponent(folder)}&highlight=${encodeURIComponent(video.public_id)}`,
    );
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              className={cn("z-10 bg-background/80", triggerClassName)}
              aria-label="Thao tác video"
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
              }}
            >
              <MoreHorizontalIcon className="size-4" />
            </Button>
          }
        />
        <DropdownMenuContent align={align} className="min-w-48">
          {canUpdate && (
            <DropdownMenuItem onClick={() => setRenameOpen(true)}>
              <PencilIcon className="size-4" />
              Đổi tên
            </DropdownMenuItem>
          )}
          {canUpdate && (
            <DropdownMenuSub>
              <DropdownMenuSubTrigger>
                <FolderIcon className="size-4" />
                Chuyển vào loại
              </DropdownMenuSubTrigger>
              <DropdownMenuSubContent>
                {categories.length === 0 ? (
                  <DropdownMenuItem disabled>Chưa có loại</DropdownMenuItem>
                ) : (
                  categories.map((c) => (
                    <DropdownMenuItem
                      key={c.public_id}
                      disabled={
                        assignMut.isPending ||
                        video.category_public_id === c.public_id
                      }
                      onClick={() => assignMut.mutate(c.public_id)}
                    >
                      {c.name}
                    </DropdownMenuItem>
                  ))
                )}
              </DropdownMenuSubContent>
            </DropdownMenuSub>
          )}
          {canUpdate && video.category_public_id && (
            <DropdownMenuItem
              disabled={assignMut.isPending}
              onClick={() => assignMut.mutate(null)}
            >
              <FolderXIcon className="size-4" />
              Bỏ phân loại
            </DropdownMenuItem>
          )}
          {canLocate && (
            <DropdownMenuItem onClick={locateInFiles}>
              <FolderOpenIcon className="size-4" />
              Định vị file
            </DropdownMenuItem>
          )}
          <DropdownMenuItem onClick={() => void copyId()}>
            <CopyIcon className="size-4" />
            Copy public ID
          </DropdownMenuItem>
          {isConverting && canUpdate && (
            <DropdownMenuItem
              onClick={() => cancelMut.mutate()}
              disabled={cancelMut.isPending}
            >
              <StopCircleIcon className="size-4" />
              Hủy convert
            </DropdownMenuItem>
          )}
          {video.hls_status === "ready" && canDelete && (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onClick={() => void handleDeleteHls()}
                disabled={deleteHlsMut.isPending}
              >
                <Trash2Icon className="size-4" />
                Xóa HLS (giữ gốc)
              </DropdownMenuItem>
            </>
          )}
          {canDelete && (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                variant="destructive"
                onClick={() => void handleDelete()}
                disabled={deleteMut.isPending}
              >
                <Trash2Icon className="size-4" />
                Xóa video
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>

      <RenameDialog
        open={renameOpen}
        object={renameOpen ? renameObject : null}
        onClose={() => setRenameOpen(false)}
        onSubmit={(v) => renameMut.mutate(v.name)}
        loading={renameMut.isPending}
      />
    </>
  );
}

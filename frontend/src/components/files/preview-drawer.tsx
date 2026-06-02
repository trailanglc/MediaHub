"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ExpandIcon, ExternalLinkIcon, FileIcon } from "lucide-react";
import type { MediaObject } from "@/lib/api-client";
import { formatBytes } from "@/lib/format";
import { Button, buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import {
  Dialog,
  DialogContent,
} from "@/components/ui/dialog";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";

function PreviewMedia({
  object,
  onExpand,
}: {
  object: MediaObject;
  onExpand?: () => void;
}) {
  const url = object.preview_url ?? object.download_url ?? null;
  const isPdf = object.mime_type === "application/pdf";

  if (object.type === "image" && url) {
    return (
      <button
        type="button"
        onClick={onExpand}
        className="group relative flex w-full cursor-zoom-in items-center justify-center rounded-xl bg-muted/50 p-4 ring-1 ring-border/60 transition-colors hover:bg-muted/70"
        title="Nhấn để phóng to"
      >
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={url}
          alt={object.name}
          className="max-h-[min(70vh,720px)] w-auto max-w-full object-contain"
        />
        <span className="pointer-events-none absolute right-3 bottom-3 inline-flex items-center gap-1 rounded-md bg-background/90 px-2 py-1 text-xs text-muted-foreground opacity-0 shadow-sm transition-opacity group-hover:opacity-100">
          <ExpandIcon className="size-3.5" />
          Phóng to
        </span>
      </button>
    );
  }

  if (object.type === "video" && url) {
    return (
      <div className="overflow-hidden rounded-xl bg-black ring-1 ring-border/60">
        <video
          src={url}
          controls
          playsInline
          className="max-h-[min(70vh,720px)] w-full bg-black"
          preload="metadata"
        >
          Trình duyệt không hỗ trợ phát video.
        </video>
      </div>
    );
  }

  if (object.type === "video") {
    return (
      <p className="rounded-xl bg-muted/50 p-4 text-sm text-muted-foreground">
        Video đã lưu. Chuyển mã HLS và xem preview tại Video Manager.
      </p>
    );
  }

  if (isPdf && url) {
    return (
      <iframe
        src={url}
        title={object.name}
        className="h-[min(70vh,720px)] w-full rounded-xl border bg-background ring-1 ring-border/60"
      />
    );
  }

  if (url && object.type === "file") {
    return (
      <div className="flex flex-col items-center gap-3 rounded-xl bg-muted/50 p-8 text-center text-muted-foreground">
        <FileIcon className="size-12 opacity-60" />
        <p className="text-sm">Không có bản xem trước cho loại tệp này.</p>
        <a
          href={url}
          target="_blank"
          rel="noreferrer"
          className="inline-flex h-8 items-center gap-1.5 rounded-lg border bg-background px-3 text-sm text-foreground hover:bg-muted"
        >
          <ExternalLinkIcon className="size-3.5" />
          Mở tệp
        </a>
      </div>
    );
  }

  return null;
}

export function PreviewDrawer({
  object,
  open,
  onClose,
}: {
  object: MediaObject | null;
  open: boolean;
  onClose: () => void;
}) {
  const [lightboxOpen, setLightboxOpen] = useState(false);
  const imageUrl =
    object?.type === "image"
      ? (object.preview_url ?? object.download_url)
      : null;

  useEffect(() => {
    if (!open) setLightboxOpen(false);
  }, [open]);

  if (!object) return null;

  const handleOpenChange = (next: boolean) => {
    if (!next) {
      setLightboxOpen(false);
      onClose();
    }
  };

  return (
    <>
      <Sheet open={open} onOpenChange={handleOpenChange}>
        <SheetContent className="flex w-full flex-col gap-0 overflow-hidden p-0 sm:max-w-xl md:max-w-2xl lg:max-w-4xl">
          <SheetHeader className="shrink-0 border-b px-6 py-4 pr-14">
            <SheetTitle className="truncate pr-2">{object.name}</SheetTitle>
            <SheetDescription className="capitalize">
              {object.type}
              {object.mime_type ? ` · ${object.mime_type}` : ""}
            </SheetDescription>
          </SheetHeader>

          <div className="flex flex-1 flex-col gap-6 overflow-y-auto px-6 py-5">
            <PreviewMedia
              object={object}
              onExpand={
                imageUrl ? () => setLightboxOpen(true) : undefined
              }
            />

            <dl className="grid gap-3 text-sm sm:grid-cols-2">
              <div>
                <dt className="text-muted-foreground">Kích thước</dt>
                <dd className="font-medium">{formatBytes(object.size_bytes)}</dd>
              </div>
              {object.mime_type && (
                <div>
                  <dt className="text-muted-foreground">MIME</dt>
                  <dd className="font-medium">{object.mime_type}</dd>
                </div>
              )}
              <div className="sm:col-span-2">
                <dt className="text-muted-foreground">Cập nhật</dt>
                <dd className="font-medium">
                  {new Date(object.updated_at).toLocaleString("vi-VN")}
                </dd>
              </div>
            </dl>

            <div className="flex flex-wrap gap-2">
              {imageUrl && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setLightboxOpen(true)}
                >
                  <ExpandIcon />
                  Phóng to
                </Button>
              )}
              {(object.preview_url ?? object.download_url) && (
                <a
                  href={object.preview_url ?? object.download_url!}
                  target="_blank"
                  rel="noreferrer"
                  className={cn(buttonVariants({ variant: "outline", size: "sm" }))}
                >
                  <ExternalLinkIcon />
                  Mở tab mới
                </a>
              )}
              {object.download_url && (
                <a
                  href={object.download_url}
                  download={object.name}
                  className={cn(buttonVariants({ variant: "outline", size: "sm" }))}
                >
                  Tải xuống
                </a>
              )}
              {object.type === "video" && (
                <Link
                  href={`/videos/${object.public_id}`}
                  className={cn(buttonVariants({ size: "sm" }))}
                >
                  Mở trong Videos
                </Link>
              )}
            </div>
          </div>
        </SheetContent>
      </Sheet>

      {imageUrl && (
        <Dialog open={lightboxOpen} onOpenChange={setLightboxOpen}>
          <DialogContent className="flex max-h-[96vh] max-w-[min(96vw,1280px)] flex-col items-center justify-center border-zinc-800 bg-zinc-950 p-4 text-zinc-100 shadow-2xl sm:max-w-[min(96vw,1280px)] [&_[data-slot=dialog-close]]:text-zinc-100 [&_[data-slot=dialog-close]]:hover:bg-zinc-800">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={imageUrl}
              alt={object.name}
              className="max-h-[92vh] w-auto max-w-full object-contain"
            />
            <p className="mt-2 max-w-full truncate px-2 text-center text-xs text-white/80">
              {object.name}
            </p>
          </DialogContent>
        </Dialog>
      )}
    </>
  );
}

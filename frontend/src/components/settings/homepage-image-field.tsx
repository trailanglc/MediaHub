"use client";

import { ImageIcon, Trash2Icon, UploadIcon } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { toast } from "@/hooks/use-app-toast";
import { fetchObjectPreviewURLs } from "@/lib/api/api-client";
import { uploadFileChunked } from "@/lib/upload/chunked-upload";
import { Button } from "@/components/ui/button";
import { FormField } from "@/components/ui/form-field";
import { Spinner } from "@/components/ui/spinner";
import { cn } from "@/lib/utils";

type PreviewAspect = "square" | "video" | "wide";

const ASPECT: Record<PreviewAspect, string> = {
  square: "aspect-square max-w-[7rem]",
  video: "aspect-video max-w-xs",
  wide: "aspect-[21/9] max-w-md",
};

export function HomepageImageField({
  label,
  hint,
  objectId,
  uploadParentId,
  accept = "image/png,image/jpeg,image/webp,image/gif,image/svg+xml,image/x-icon,.ico",
  aspect = "video",
  onObjectIdChange,
}: {
  label: string;
  hint?: string;
  objectId: string;
  uploadParentId: string;
  accept?: string;
  aspect?: PreviewAspect;
  onObjectIdChange: (objectId: string) => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);

  useEffect(() => {
    if (!objectId) {
      setPreviewUrl(null);
      return;
    }
    let cancelled = false;
    void fetchObjectPreviewURLs([objectId])
      .then((resp) => {
        if (cancelled) return;
        const url =
          resp.urls[objectId]?.preview_url ??
          resp.urls[objectId]?.thumbnail_url ??
          null;
        setPreviewUrl(url);
      })
      .catch(() => {
        if (!cancelled) setPreviewUrl(null);
      });
    return () => {
      cancelled = true;
    };
  }, [objectId]);

  const handleFile = async (file: File | undefined) => {
    if (!file) return;
    const isImage =
      file.type.startsWith("image/") || file.name.toLowerCase().endsWith(".ico");
    if (!isImage) {
      toast.error("Chọn file ảnh (PNG, JPG, WebP, SVG, ICO…)");
      return;
    }
    if (!uploadParentId) {
      toast.error("Chưa cấu hình root folder upload (tab Chung)");
      return;
    }
    setUploading(true);
    try {
      const id = await uploadFileChunked(file, uploadParentId);
      onObjectIdChange(id);
      toast.success("Đã upload ảnh");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Upload thất bại");
    } finally {
      setUploading(false);
      if (inputRef.current) inputRef.current.value = "";
    }
  };

  return (
    <FormField label={label} hint={hint}>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start">
        <div
          className={cn(
            "relative flex w-full shrink-0 items-center justify-center overflow-hidden rounded-lg border border-dashed border-border bg-muted/30",
            ASPECT[aspect],
          )}
        >
          {previewUrl ? (
            // eslint-disable-next-line @next/next/no-img-element -- presigned preview URL
            <img
              src={previewUrl}
              alt=""
              className="size-full object-cover"
            />
          ) : (
            <ImageIcon className="size-8 text-muted-foreground/50" aria-hidden />
          )}
        </div>
        <div className="flex min-w-0 flex-1 flex-col gap-2">
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={uploading}
              onClick={() => inputRef.current?.click()}
            >
              {uploading ? (
                <>
                  <Spinner className="size-4" />
                  Đang upload…
                </>
              ) : (
                <>
                  <UploadIcon className="size-4" />
                  Chọn &amp; upload
                </>
              )}
            </Button>
            {objectId ? (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                disabled={uploading}
                onClick={() => onObjectIdChange("")}
              >
                <Trash2Icon className="size-4" />
                Xóa
              </Button>
            ) : null}
          </div>
          {objectId ? (
            <p className="truncate font-mono text-xs text-muted-foreground" title={objectId}>
              {objectId}
            </p>
          ) : (
            <p className="text-xs text-muted-foreground">Chưa chọn ảnh</p>
          )}
          <input
            ref={inputRef}
            type="file"
            accept={accept}
            className="sr-only"
            onChange={(e) => void handleFile(e.target.files?.[0])}
          />
        </div>
      </div>
    </FormField>
  );
}

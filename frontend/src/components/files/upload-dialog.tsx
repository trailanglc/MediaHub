"use client";

import Uppy from "@uppy/core";
import Dashboard from "@uppy/react/dashboard";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  uploadFileChunked,
  type ChunkedUploadProgress,
} from "@/lib/chunked-upload";
import { formatBytes } from "@/lib/format";
import { DEFAULT_MAX_UPLOAD_BYTES } from "@/lib/upload-limits";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Progress } from "@/components/ui/progress";
import { Spinner } from "@/components/ui/spinner";
import { toast } from "@/hooks/use-app-toast";
import "@uppy/core/css/style.min.css";
import "@uppy/dashboard/css/style.min.css";

type UploadProgressUI = {
  fileIndex: number;
  fileCount: number;
  fileName: string;
  filePercent: number;
  fileLoaded: number;
  fileTotal: number;
  overallPercent: number;
  overallLoaded: number;
  overallTotal: number;
};

const MAX_FILES_PER_BATCH = 20;

function isUploadAborted(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

export function UploadDialog({
  open,
  parentId,
  maxUploadBytes,
  onClose,
  onSuccess,
}: {
  open: boolean;
  parentId: string;
  maxUploadBytes?: number;
  onClose: () => void;
  onSuccess: () => void;
}) {
  const [uploading, setUploading] = useState(false);
  const [progress, setProgress] = useState<UploadProgressUI | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const limitBytes = maxUploadBytes ?? DEFAULT_MAX_UPLOAD_BYTES;
  const limitLabel = formatBytes(limitBytes);

  const uppy = useMemo(
    () =>
      new Uppy({
        autoProceed: false,
        restrictions: {
          maxNumberOfFiles: MAX_FILES_PER_BATCH,
          maxFileSize: limitBytes,
        },
      }),
    [limitBytes],
  );

  useEffect(() => {
    uppy.setOptions({
      restrictions: {
        maxNumberOfFiles: MAX_FILES_PER_BATCH,
        maxFileSize: limitBytes,
      },
    });
  }, [uppy, limitBytes]);

  useEffect(() => {
    if (!open) {
      abortRef.current?.abort();
      abortRef.current = null;
      uppy.cancelAll();
      setProgress(null);
      setUploading(false);
    }
  }, [open, uppy]);

  useEffect(() => {
    return () => {
      abortRef.current?.abort();
      uppy.cancelAll();
      uppy.destroy();
    };
  }, [uppy]);

  const handleCancelUpload = () => {
    abortRef.current?.abort();
  };

  const handleUpload = async () => {
    const files = uppy.getFiles();
    const queue = files.filter(
      (f): f is typeof f & { data: File } => f.data instanceof File,
    );
    if (queue.length === 0) {
      toast.error("Chọn ít nhất một file");
      return;
    }

    const overallTotal = queue.reduce((sum, f) => sum + f.data.size, 0);
    const ac = new AbortController();
    abortRef.current = ac;
    setUploading(true);
    setProgress(null);

    let ok = 0;
    let bytesCompleted = 0;
    let aborted = false;

    for (let i = 0; i < queue.length; i++) {
      if (ac.signal.aborted) {
        aborted = true;
        break;
      }
      const { data: file, id: fileId, name: fileName } = queue[i];
      const onProgress = (p: ChunkedUploadProgress) => {
        const overallLoaded = bytesCompleted + p.loaded;
        setProgress({
          fileIndex: i + 1,
          fileCount: queue.length,
          fileName,
          filePercent: p.percent,
          fileLoaded: p.loaded,
          fileTotal: p.total,
          overallPercent:
            overallTotal > 0
              ? Math.min(100, Math.round((overallLoaded / overallTotal) * 100))
              : p.percent,
          overallLoaded,
          overallTotal,
        });
      };

      try {
        await uploadFileChunked(file, parentId, onProgress, ac.signal);
        bytesCompleted += file.size;
        ok += 1;
      } catch (err) {
        if (isUploadAborted(err)) {
          aborted = true;
          break;
        }
        const msg = err instanceof Error ? err.message : "Upload thất bại";
        uppy.setFileState(fileId, { error: msg });
        toast.error(`${fileName}: ${msg}`);
      }
    }

    abortRef.current = null;
    setUploading(false);
    setProgress(null);

    if (aborted) {
      toast.info("Đã hủy upload");
      return;
    }

    if (ok > 0) {
      toast.success(`Đã upload ${ok} file`);
      uppy.cancelAll();
      onSuccess();
      onClose();
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v && !uploading) {
          uppy.cancelAll();
          onClose();
        }
      }}
    >
      <DialogContent className="w-[calc(100%-2rem)] max-w-2xl overflow-hidden sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Upload file</DialogTitle>
        </DialogHeader>

        <div
          className={
            uploading ? "pointer-events-none opacity-50" : undefined
          }
        >
          <div className="upload-dialog-uppy uppy-theme-light min-w-0 w-full">
            <Dashboard
              uppy={uppy}
              width="100%"
              proudlyDisplayPoweredByUppy={false}
              height={280}
              note={`Tối đa ${limitLabel}/file · tối đa ${MAX_FILES_PER_BATCH} file/lần · upload theo chunk`}
            />
          </div>
        </div>

        {uploading && progress && (
          <div
            className="space-y-3 rounded-lg border border-border bg-muted/40 p-4"
            aria-live="polite"
            aria-busy="true"
          >
            <div className="flex items-center gap-2 text-sm font-medium">
              <Spinner className="size-4 shrink-0" />
              <span>
                Đang upload ({progress.fileIndex}/{progress.fileCount})
              </span>
            </div>
            <p className="truncate text-sm text-muted-foreground" title={progress.fileName}>
              {progress.fileName}
            </p>

            <div className="space-y-1.5">
              <div className="flex justify-between gap-2 text-xs text-muted-foreground">
                <span>File hiện tại</span>
                <span className="tabular-nums">
                  {progress.filePercent}% · {formatBytes(progress.fileLoaded)} /{" "}
                  {formatBytes(progress.fileTotal)}
                </span>
              </div>
              <Progress
                value={progress.filePercent}
                indicatorClassName="bg-primary"
              />
            </div>

            {progress.fileCount > 1 && (
              <div className="space-y-1.5">
                <div className="flex justify-between gap-2 text-xs text-muted-foreground">
                  <span>Tổng tiến độ</span>
                  <span className="tabular-nums">
                    {progress.overallPercent}% ·{" "}
                    {formatBytes(progress.overallLoaded)} /{" "}
                    {formatBytes(progress.overallTotal)}
                  </span>
                </div>
                <Progress
                  value={progress.overallPercent}
                  indicatorClassName="bg-primary/80"
                />
              </div>
            )}
          </div>
        )}

        <DialogFooter className="mt-0">
          {uploading ? (
            <Button type="button" variant="destructive" onClick={handleCancelUpload}>
              Hủy upload
            </Button>
          ) : (
            <Button type="button" variant="outline" onClick={onClose}>
              Đóng
            </Button>
          )}
          <Button type="button" onClick={handleUpload} disabled={uploading}>
            {uploading ? (
              <>
                <Spinner className="size-4" />
                Đang upload…
              </>
            ) : (
              "Bắt đầu upload"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

"use client";

import { useState } from "react";
import type { MediaObject } from "@/lib/api-client";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { FormField } from "@/components/ui/form-field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";

export function BulkRenameDialog({
  open,
  objects,
  onClose,
  onApply,
  loading,
}: {
  open: boolean;
  objects: MediaObject[];
  onClose: () => void;
  onApply: (mode: "prefix" | "suffix", value: string) => void;
  loading?: boolean;
}) {
  const [mode, setMode] = useState<"prefix" | "suffix">("prefix");
  const [value, setValue] = useState("");

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const v = value.trim();
    if (!v) return;
    onApply(mode, v);
  };

  return (
    <Dialog open={open} onOpenChange={(v) => !v && !loading && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Đổi tên {objects.length} mục</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <FormField label="Cách đổi tên">
            <div className="flex gap-2">
              <Button
                type="button"
                size="sm"
                variant={mode === "prefix" ? "default" : "outline"}
                onClick={() => setMode("prefix")}
              >
                Thêm tiền tố
              </Button>
              <Button
                type="button"
                size="sm"
                variant={mode === "suffix" ? "default" : "outline"}
                onClick={() => setMode("suffix")}
              >
                Thêm hậu tố
              </Button>
            </div>
          </FormField>
          <FormField label={mode === "prefix" ? "Tiền tố" : "Hậu tố"}>
            <Input
              value={value}
              onChange={(e) => setValue(e.target.value)}
              placeholder={mode === "prefix" ? "vd. backup_" : "vd. _2024"}
            />
          </FormField>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose} disabled={loading}>
              Hủy
            </Button>
            <Button type="submit" disabled={loading || !value.trim()}>
              {loading && <Spinner className="size-4" />}
              Áp dụng
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

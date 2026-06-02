"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import type { MediaObject } from "@/lib/api-client";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { FormField, FormStack } from "@/components/ui/form-field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import {
  renameObjectSchema,
  type RenameObjectForm,
} from "@/lib/schemas/media";

export function RenameDialog({
  open,
  object,
  onClose,
  onSubmit,
  loading,
}: {
  open: boolean;
  object: MediaObject | null;
  onClose: () => void;
  onSubmit: (values: RenameObjectForm) => void;
  loading?: boolean;
}) {
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<RenameObjectForm>({
    resolver: zodResolver(renameObjectSchema),
  });

  useEffect(() => {
    if (object) reset({ name: object.name });
  }, [object, reset]);

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Đổi tên</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)}>
          <FormStack>
            <FormField label="Tên mới" error={errors.name?.message}>
              <Input {...register("name")} autoFocus />
            </FormField>
          </FormStack>
          <DialogFooter className="mt-4">
            <Button type="button" variant="outline" onClick={onClose}>
              Hủy
            </Button>
            <Button type="submit" disabled={loading}>
              {loading && <Spinner className="size-4" />}
              Lưu
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

"use client";

import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
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

const schema = z.object({
  name: z.string().trim().min(1, "Nhập tên loại").max(200),
});

type FormValues = z.infer<typeof schema>;

export function CategoryNameDialog({
  open,
  title,
  initialName = "",
  submitLabel = "Lưu",
  onClose,
  onSubmit,
  loading,
}: {
  open: boolean;
  title: string;
  initialName?: string;
  submitLabel?: string;
  onClose: () => void;
  onSubmit: (name: string) => void;
  loading?: boolean;
}) {
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { name: initialName },
  });

  useEffect(() => {
    if (open) reset({ name: initialName });
  }, [open, initialName, reset]);

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit((v) => onSubmit(v.name))}>
          <FormStack>
            <FormField label="Tên loại" error={errors.name?.message}>
              <Input {...register("name")} autoFocus />
            </FormField>
          </FormStack>
          <DialogFooter className="mt-4">
            <Button type="button" variant="outline" onClick={onClose}>
              Hủy
            </Button>
            <Button type="submit" disabled={loading}>
              {loading && <Spinner className="size-4" />}
              {submitLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

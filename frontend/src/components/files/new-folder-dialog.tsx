"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
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
  createFolderSchema,
  type CreateFolderForm,
} from "@/lib/schemas/media";

export function NewFolderDialog({
  open,
  onClose,
  onSubmit,
  loading,
}: {
  open: boolean;
  onClose: () => void;
  onSubmit: (values: CreateFolderForm) => void;
  loading?: boolean;
}) {
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<CreateFolderForm>({
    resolver: zodResolver(createFolderSchema),
    defaultValues: { name: "" },
  });

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) {
          reset();
          onClose();
        }
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Folder mới</DialogTitle>
        </DialogHeader>
        <form
          onSubmit={handleSubmit((values) => {
            onSubmit(values);
          })}
        >
          <FormStack>
            <FormField label="Tên folder" error={errors.name?.message}>
              <Input {...register("name")} autoFocus />
            </FormField>
          </FormStack>
          <DialogFooter className="mt-4">
            <Button type="button" variant="outline" onClick={onClose}>
              Hủy
            </Button>
            <Button type="submit" disabled={loading}>
              {loading && <Spinner className="size-4" />}
              Tạo
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

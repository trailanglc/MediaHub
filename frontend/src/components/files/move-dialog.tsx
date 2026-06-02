"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import type { MediaObject } from "@/lib/api-client";
import { fetchObject, OBJECTS_QUERY_KEY } from "@/lib/api-client";
import { FolderTree } from "@/components/files/folder-tree";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { FormField, FormStack } from "@/components/ui/form-field";
import { Spinner } from "@/components/ui/spinner";
import {
  moveObjectSchema,
  type MoveObjectForm,
} from "@/lib/schemas/media";

export function MoveDialog({
  open,
  object,
  objects,
  rootFolderId,
  rootName = "Root",
  browseFolderId,
  onClose,
  onSubmit,
  loading,
}: {
  open: boolean;
  /** Một mục (menu từng dòng). */
  object?: MediaObject | null;
  /** Nhiều mục (chọn hàng loạt). */
  objects?: MediaObject[];
  rootFolderId: string;
  rootName?: string;
  browseFolderId: string;
  onClose: () => void;
  onSubmit: (values: MoveObjectForm) => void;
  loading?: boolean;
}) {
  const targets = useMemo(() => {
    if (objects?.length) return objects;
    if (object) return [object];
    return [];
  }, [objects, object]);

  const primary = targets[0] ?? null;
  const defaultParentId =
    primary?.parent_public_id ?? browseFolderId ?? rootFolderId;

  const [pickedName, setPickedName] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    setValue,
    watch,
    reset,
    formState: { errors },
  } = useForm<MoveObjectForm>({
    resolver: zodResolver(moveObjectSchema),
    defaultValues: { parent_id: defaultParentId },
  });

  const parentId = watch("parent_id");

  const { data: defaultParentMeta } = useQuery({
    queryKey: [OBJECTS_QUERY_KEY, "folder", defaultParentId],
    queryFn: () => fetchObject(defaultParentId),
    enabled:
      open &&
      !!primary &&
      defaultParentId !== rootFolderId &&
      defaultParentId.length > 0,
  });

  const disabledFolderIds = useMemo(() => {
    const ids = new Set<string>();
    for (const t of targets) {
      if (t.type === "folder") {
        ids.add(t.public_id);
      }
    }
    return ids;
  }, [targets]);

  useEffect(() => {
    if (!open || targets.length === 0) return;
    reset({ parent_id: defaultParentId });
    setPickedName(defaultParentId === rootFolderId ? rootName : null);
  }, [open, targets.length, defaultParentId, rootFolderId, rootName, reset]);

  useEffect(() => {
    if (
      !open ||
      parentId !== defaultParentId ||
      defaultParentId === rootFolderId
    ) {
      return;
    }
    if (defaultParentMeta?.name) {
      setPickedName(defaultParentMeta.name);
    }
  }, [open, parentId, defaultParentId, rootFolderId, defaultParentMeta?.name]);

  const destinationLabel =
    parentId === rootFolderId
      ? rootName
      : (pickedName ?? defaultParentMeta?.name ?? "…");

  const handlePickFolder = (id: string, name: string) => {
    setValue("parent_id", id, { shouldValidate: true });
    setPickedName(name);
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {targets.length <= 1
              ? `Di chuyển «${primary?.name ?? ""}»`
              : `Di chuyển ${targets.length} mục`}
          </DialogTitle>
          <DialogDescription>
            Chọn thư mục đích trong cây bên dưới.
            {targets.some((t) => t.type === "folder") &&
              " Không thể chọn chính folder đang di chuyển."}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit(onSubmit)}>
          <FormStack>
            <FormField
              label="Thư mục đích"
              error={errors.parent_id?.message}
              hint="Nhấn vào tên folder để chọn"
            >
              <p className="rounded-md border bg-muted/40 px-3 py-2 text-sm font-medium text-foreground">
                {destinationLabel}
              </p>
              <div className="max-h-64 overflow-y-auto rounded-lg border bg-background p-2">
                <FolderTree
                  rootFolderId={rootFolderId}
                  rootName={rootName}
                  selectedFolderId={parentId}
                  disabledFolderIds={disabledFolderIds}
                  showHeading={false}
                  onSelect={handlePickFolder}
                />
              </div>
              <input type="hidden" {...register("parent_id")} />
            </FormField>
          </FormStack>
          <DialogFooter className="mt-4">
            <Button type="button" variant="outline" onClick={onClose}>
              Hủy
            </Button>
            <Button type="submit" disabled={loading || !parentId}>
              {loading && <Spinner className="size-4" />}
              Di chuyển
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { useState } from "react";
import { grantPermission } from "@/lib/api-client";
import type { Member } from "@/lib/api-client";
import { authErrorMessage } from "@/hooks/use-auth";
import { toast } from "@/hooks/use-app-toast";
import { Alert, AlertDescription } from "@/components/ui/alert";
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
import { NativeSelect } from "@/components/ui/native-select";
import { Spinner } from "@/components/ui/spinner";
import { UI_COPY } from "@/lib/ui-copy";

const PERMISSION_ACTIONS = [
  "read",
  "upload",
  "update",
  "delete",
  "convert",
  "share",
  "stream",
  "download",
  "manage",
] as const;

const schema = z.object({
  user_public_id: z.string().min(1, "Chọn member"),
  permission: z.enum(PERMISSION_ACTIONS),
});

type FormValues = z.infer<typeof schema>;

export function GrantPermissionModal({
  open,
  onClose,
  resourceId,
  members,
  onSuccess,
}: {
  open: boolean;
  onClose: () => void;
  resourceId: string;
  members: Member[];
  onSuccess: () => void;
}) {
  const [submitError, setSubmitError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { permission: "read", user_public_id: "" },
  });

  const onSubmit = async (values: FormValues) => {
    setSubmitError(null);
    try {
      await grantPermission({
        user_public_id: values.user_public_id,
        resource_public_id: resourceId,
        permission: values.permission,
      });
      reset();
      toast.success("Đã gán quyền");
      onSuccess();
      onClose();
    } catch (err) {
      setSubmitError(authErrorMessage(err));
    }
  };

  const handleClose = () => {
    setSubmitError(null);
    reset();
    onClose();
  };

  return (
    <Dialog open={open} onOpenChange={(next) => !next && handleClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Gán quyền</DialogTitle>
          <DialogDescription>
            Chọn member và loại quyền trên resource hiện tại.
          </DialogDescription>
        </DialogHeader>
        <FormStack onSubmit={handleSubmit(onSubmit)}>
          <FormField label="Member" error={errors.user_public_id?.message} required>
            <NativeSelect {...register("user_public_id")}>
              <option value="">Chọn member</option>
              {members.map((m) => (
                <option key={m.public_id} value={m.public_id}>
                  {m.email} ({m.role})
                </option>
              ))}
            </NativeSelect>
          </FormField>
          <FormField label="Quyền" error={errors.permission?.message} required>
            <NativeSelect {...register("permission")}>
              {PERMISSION_ACTIONS.map((a) => (
                <option key={a} value={a}>
                  {a}
                </option>
              ))}
            </NativeSelect>
          </FormField>
          {submitError && (
            <Alert variant="destructive">
              <AlertDescription>{submitError}</AlertDescription>
            </Alert>
          )}
        </FormStack>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={handleClose}>
            {UI_COPY.cancel}
          </Button>
          <Button
            type="button"
            disabled={isSubmitting || !resourceId}
            onClick={() => void handleSubmit(onSubmit)()}
          >
            {isSubmitting ? (
              <>
                <Spinner className="size-4" />
                {UI_COPY.processing}
              </>
            ) : (
              "Gán quyền"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

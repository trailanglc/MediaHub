"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { useState } from "react";
import { createMember } from "@/lib/api-client";
import {
  emailSchema,
  memberRoleSchema,
  passwordSchema,
} from "@/lib/form-schemas";
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
import { Input } from "@/components/ui/input";
import { NativeSelect } from "@/components/ui/native-select";
import { Spinner } from "@/components/ui/spinner";
import { UI_COPY } from "@/lib/ui-copy";

const schema = z.object({
  email: emailSchema,
  password: passwordSchema,
  role: memberRoleSchema,
});

type FormValues = z.infer<typeof schema>;

export function CreateMemberModal({
  open,
  onClose,
  onSuccess,
}: {
  open: boolean;
  onClose: () => void;
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
    defaultValues: { role: "viewer" },
  });

  const onSubmit = async (values: FormValues) => {
    setSubmitError(null);
    try {
      await createMember(values);
      reset();
      toast.success("Đã tạo member");
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
          <DialogTitle>Tạo member</DialogTitle>
          <DialogDescription>
            Tài khoản manager hoặc viewer cho workspace.
          </DialogDescription>
        </DialogHeader>
        <FormStack onSubmit={handleSubmit(onSubmit)}>
          <FormField label="Email" error={errors.email?.message} required>
            <Input
              type="email"
              autoComplete="email"
              placeholder="member@example.com"
              {...register("email")}
            />
          </FormField>
          <FormField label="Mật khẩu" error={errors.password?.message} required>
            <Input
              type="password"
              autoComplete="new-password"
              {...register("password")}
            />
          </FormField>
          <FormField label="Role" error={errors.role?.message} required>
            <NativeSelect {...register("role")}>
              <option value="viewer">viewer</option>
              <option value="manager">manager</option>
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
            disabled={isSubmitting}
            onClick={() => void handleSubmit(onSubmit)()}
          >
            {isSubmitting ? (
              <>
                <Spinner className="size-4" />
                {UI_COPY.processing}
              </>
            ) : (
              "Tạo member"
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { ApiError, createOwner } from "@/lib/api-client";
import { authInputClassName, authSubmitButtonClassName } from "@/lib/auth-ui";
import { emailSchema, passwordSchema } from "@/lib/form-schemas";
import { cn } from "@/lib/utils";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Spinner } from "@/components/ui/spinner";
import { AuthCard } from "@/components/ui/auth-card";
import { Button } from "@/components/ui/button";
import { FormField, FormStack } from "@/components/ui/form-field";
import { Input } from "@/components/ui/input";

const setupSchema = z
  .object({
    email: emailSchema,
    password: passwordSchema,
    confirmPassword: z.string(),
    setupToken: z.string().optional(),
  })
  .refine((data) => data.password === data.confirmPassword, {
    message: "Mật khẩu xác nhận không khớp",
    path: ["confirmPassword"],
  });

type SetupFormValues = z.infer<typeof setupSchema>;

export function SetupForm({ showSetupToken }: { showSetupToken: boolean }) {
  const router = useRouter();
  const [submitError, setSubmitError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<SetupFormValues>({
    resolver: zodResolver(setupSchema),
  });

  const onSubmit = async (values: SetupFormValues) => {
    setSubmitError(null);
    try {
      await createOwner({
        email: values.email,
        password: values.password,
        setup_token: values.setupToken || undefined,
      });
      router.push("/login?setup=done");
    } catch (err) {
      if (err instanceof ApiError) {
        const body = err.body as { message?: string };
        setSubmitError(body?.message ?? err.message);
      } else if (err instanceof Error && err.message) {
        setSubmitError(err.message);
      } else {
        setSubmitError("Không thể tạo Owner. Vui lòng thử lại.");
      }
    }
  };

  return (
    <AuthCard
      showLogo={false}
      title="Thiết lập hệ thống"
      description="Tạo tài khoản Owner đầu tiên để bắt đầu quản trị MediaHub."
    >
      <FormStack onSubmit={handleSubmit(onSubmit)} className="space-y-4 sm:space-y-5">
        <FormField label="Email" error={errors.email?.message} required>
          <Input
            type="email"
            inputMode="email"
            autoComplete="email"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            placeholder="owner@company.com"
            className={authInputClassName}
            {...register("email")}
          />
        </FormField>
        <FormField
          label="Mật khẩu"
          error={errors.password?.message}
          hint="Tối thiểu 8 ký tự, gồm chữ và số"
          required
        >
          <Input
            type="password"
            autoComplete="new-password"
            placeholder="Tạo mật khẩu mạnh"
            className={authInputClassName}
            {...register("password")}
          />
        </FormField>
        <FormField
          label="Xác nhận mật khẩu"
          error={errors.confirmPassword?.message}
          required
        >
          <Input
            type="password"
            autoComplete="new-password"
            placeholder="Nhập lại mật khẩu"
            className={authInputClassName}
            {...register("confirmPassword")}
          />
        </FormField>
        {showSetupToken && (
          <FormField
            label="Setup token"
            hint="Lấy từ biến SETUP_TOKEN trong backend/.env (production)"
          >
            <Input
              type="password"
              autoComplete="off"
              placeholder="Nhập setup token"
              className={authInputClassName}
              {...register("setupToken")}
            />
          </FormField>
        )}
        {submitError && (
          <Alert variant="destructive">
            <AlertDescription className="text-pretty text-sm">
              {submitError}
            </AlertDescription>
          </Alert>
        )}
        <Button
          type="submit"
          className={cn(authSubmitButtonClassName)}
          disabled={isSubmitting}
        >
          {isSubmitting && <Spinner className="size-4" />}
          {isSubmitting ? "Đang tạo..." : "Tạo Owner"}
        </Button>
      </FormStack>
    </AuthCard>
  );
}

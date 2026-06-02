"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { authErrorMessage, useLogin } from "@/hooks/use-auth";
import { authInputClassName, authSubmitButtonClassName } from "@/lib/auth-ui";
import { emailSchema } from "@/lib/form-schemas";
import { cn } from "@/lib/utils";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Spinner } from "@/components/ui/spinner";
import { AuthCard } from "@/components/ui/auth-card";
import { Button } from "@/components/ui/button";
import { FormField, FormStack } from "@/components/ui/form-field";
import { Input } from "@/components/ui/input";

const loginSchema = z.object({
  email: emailSchema,
  password: z.string().min(1, "Vui lòng nhập mật khẩu"),
});

type LoginFormValues = z.infer<typeof loginSchema>;

export function LoginForm({ setupRequired }: { setupRequired: boolean }) {
  const searchParams = useSearchParams();
  const setupDone = searchParams.get("setup") === "done";
  const [submitError, setSubmitError] = useState<string | null>(null);
  const loginMutation = useLogin();

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
  });

  const onSubmit = async (values: LoginFormValues) => {
    setSubmitError(null);
    try {
      await loginMutation.mutateAsync(values);
    } catch (err) {
      setSubmitError(authErrorMessage(err));
    }
  };

  return (
    <AuthCard
      showLogo={false}
      title="Đăng nhập"
      description="Truy cập bảng điều khiển để quản lý media và streaming."
      footer={
        setupRequired ? (
          <p className="text-center text-sm text-muted-foreground">
            Chưa thiết lập?{" "}
            <Link
              href="/setup"
              className="font-medium underline underline-offset-4"
            >
              Thiết lập Owner
            </Link>
          </p>
        ) : undefined
      }
    >
      {setupDone && (
        <Alert className="mb-5 border-emerald-200 bg-emerald-50 text-emerald-900 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-100">
          <AlertDescription className="text-pretty text-sm leading-relaxed">
            Tạo Owner thành công. Vui lòng đăng nhập bằng tài khoản vừa tạo.
          </AlertDescription>
        </Alert>
      )}
      <FormStack onSubmit={handleSubmit(onSubmit)} className="space-y-4 sm:space-y-5">
        <FormField label="Email" error={errors.email?.message} required>
          <Input
            type="email"
            inputMode="email"
            autoComplete="email"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            placeholder="you@company.com"
            className={authInputClassName}
            {...register("email")}
          />
        </FormField>
        <FormField label="Mật khẩu" error={errors.password?.message} required>
          <Input
            type="password"
            autoComplete="current-password"
            placeholder="Nhập mật khẩu"
            className={authInputClassName}
            {...register("password")}
          />
        </FormField>
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
          disabled={isSubmitting || loginMutation.isPending}
        >
          {(isSubmitting || loginMutation.isPending) && (
            <Spinner className="size-4" />
          )}
          {isSubmitting || loginMutation.isPending
            ? "Đang đăng nhập..."
            : "Đăng nhập"}
        </Button>
      </FormStack>
    </AuthCard>
  );
}

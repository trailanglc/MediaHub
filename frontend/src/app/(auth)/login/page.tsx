import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { Suspense } from "react";
import { LoginForm } from "@/components/auth/login-form";
import { LoginRedirect } from "@/components/auth/login-redirect";
import { AuthCardSkeleton } from "@/components/ui/auth-card-skeleton";
import { fetchSetupStatusServer } from "@/lib/setup-server";

export const metadata: Metadata = {
  title: "Đăng nhập — MediaHub",
  description: "Đăng nhập vào bảng điều khiển MediaHub",
};

export default async function LoginPage() {
  const status = await fetchSetupStatusServer();
  if (status.setup_required) {
    redirect("/setup");
  }

  return (
    <LoginRedirect>
      <Suspense fallback={<AuthCardSkeleton />}>
        <LoginForm setupRequired={false} />
      </Suspense>
    </LoginRedirect>
  );
}

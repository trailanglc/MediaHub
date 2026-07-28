"use client";

import { usePathname, useRouter } from "next/navigation";
import { useEffect, useEffectEvent } from "react";
import { PageError, PageLoading } from "@/components/feedback/page-states";
import { useMe } from "@/hooks/use-auth";
import { ApiError, isApiUnreachable } from "@/lib/api/api-client";

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const { data: user, isLoading, isError, error, refetch } = useMe();
  const router = useRouter();
  const pathname = usePathname();
  const offline = isError && isApiUnreachable(error);
  const unauthorized =
    isError && error instanceof ApiError && error.status === 401;

  const redirectToLogin = useEffectEvent(() => {
    router.replace(`/login?next=${encodeURIComponent(pathname)}`);
  });

  useEffect(() => {
    // Only bounce on real auth failure. apiFetch already tried refresh once;
    // network/5xx must not look like "logged out".
    if (!isLoading && unauthorized) {
      redirectToLogin();
    }
  }, [isLoading, unauthorized]);

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center p-8">
        <PageLoading rows={2} />
      </div>
    );
  }

  if (offline) {
    return (
      <div className="flex min-h-screen items-center justify-center p-8">
        <div className="w-full max-w-lg">
          <PageError
            message="Không kết nối được máy chủ API. Kiểm tra backend (make gateway) rồi thử lại."
            onRetry={() => void refetch()}
          />
        </div>
      </div>
    );
  }

  if (isError && !unauthorized) {
    return (
      <div className="flex min-h-screen items-center justify-center p-8">
        <div className="w-full max-w-lg">
          <PageError
            message={
              error instanceof Error
                ? error.message
                : "Không tải được phiên đăng nhập."
            }
            onRetry={() => void refetch()}
          />
        </div>
      </div>
    );
  }

  if (unauthorized || !user) {
    return null;
  }

  return <>{children}</>;
}

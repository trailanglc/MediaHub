"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useEffect } from "react";
import { safeNextPath, useMe } from "@/hooks/use-auth";
import { isApiUnreachable } from "@/lib/api/api-client";
import { AuthCardSkeleton } from "@/components/ui/auth-card-skeleton";

export function LoginRedirect({ children }: { children: React.ReactNode }) {
  const { data: user, isLoading, isError, error } = useMe();
  const router = useRouter();
  const searchParams = useSearchParams();
  const nextPath = searchParams.get("next");
  const offline = isError && isApiUnreachable(error);

  useEffect(() => {
    if (!isLoading && user) {
      router.replace(safeNextPath(nextPath));
    }
  }, [isLoading, user, router, nextPath]);

  if (isLoading) {
    return <AuthCardSkeleton />;
  }

  // API down: vẫn hiện form login (submit sẽ báo lỗi), không treo skeleton.
  if (offline) {
    return <>{children}</>;
  }

  if (user) {
    return null;
  }

  return <>{children}</>;
}

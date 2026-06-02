"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { useMe } from "@/hooks/use-auth";
import { AuthCardSkeleton } from "@/components/ui/auth-card-skeleton";

export function LoginRedirect({ children }: { children: React.ReactNode }) {
  const { data: user, isLoading } = useMe();
  const router = useRouter();

  useEffect(() => {
    if (!isLoading && user) {
      router.replace("/dashboard");
    }
  }, [isLoading, user, router]);

  if (isLoading) {
    return <AuthCardSkeleton />;
  }

  if (user) {
    return null;
  }

  return <>{children}</>;
}

"use client";

import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";
import { PageLoading } from "@/components/feedback/page-states";
import { useMe } from "@/hooks/use-auth";

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const { data: user, isLoading, isError } = useMe();
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    if (!isLoading && isError) {
      const next = encodeURIComponent(pathname);
      router.replace(`/login?next=${next}`);
    }
  }, [isLoading, isError, pathname, router]);

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center p-8">
        <PageLoading rows={2} />
      </div>
    );
  }

  if (isError || !user) {
    return null;
  }

  return <>{children}</>;
}

"use client";

import Link from "next/link";
import { isOwner, useMe } from "@/hooks/use-auth";
import { PageLoading } from "@/components/feedback/page-states";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";

export function OwnerGuard({ children }: { children: React.ReactNode }) {
  const { data: user, isLoading } = useMe();

  if (isLoading) {
    return <PageLoading rows={3} />;
  }

  if (!isOwner(user)) {
    return (
      <div className="space-y-4 py-8">
        <Alert variant="destructive">
          <AlertTitle>Không có quyền</AlertTitle>
          <AlertDescription>
            Trang này chỉ dành cho Owner. Bạn không có quyền truy cập.
          </AlertDescription>
        </Alert>
        <Button
          nativeButton={false}
          render={<Link href="/dashboard" />}
          variant="outline"
          size="sm"
        >
          Về Dashboard
        </Button>
      </div>
    );
  }

  return <>{children}</>;
}

"use client";

import { MemberDashboard } from "@/components/dashboard/member-dashboard";
import { OwnerDashboard } from "@/components/dashboard/owner-dashboard";
import { LoadingBlock } from "@/components/ui/loading-block";
import { isOwner, useMe } from "@/hooks/use-auth";

export default function DashboardPage() {
  const { data: user, isLoading } = useMe();

  if (isLoading) {
    return (
      <div className="min-h-[40vh] flex items-center justify-center">
        <LoadingBlock />
      </div>
    );
  }

  if (isOwner(user)) {
    return <OwnerDashboard />;
  }

  return <MemberDashboard />;
}

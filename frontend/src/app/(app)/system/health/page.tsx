import { OwnerGuard } from "@/components/layout/owner-guard";
import { SystemHealthDashboard } from "@/components/system/system-health-dashboard";

export default function SystemHealthPage() {
  return (
    <OwnerGuard>
      <SystemHealthDashboard />
    </OwnerGuard>
  );
}

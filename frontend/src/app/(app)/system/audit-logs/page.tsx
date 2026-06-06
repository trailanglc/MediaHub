import { OwnerGuard } from "@/components/layout/owner-guard";
import { AuditLogsPanel } from "@/components/system/audit-logs-panel";

export default function AuditLogsPage() {
  return (
    <OwnerGuard>
      <AuditLogsPanel />
    </OwnerGuard>
  );
}

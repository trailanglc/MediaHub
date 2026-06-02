import { OwnerGuard } from "@/components/layout/owner-guard";
import { PermissionsPanel } from "@/components/permissions/permissions-panel";

export default function PermissionsPage() {
  return (
    <OwnerGuard>
      <PermissionsPanel />
    </OwnerGuard>
  );
}

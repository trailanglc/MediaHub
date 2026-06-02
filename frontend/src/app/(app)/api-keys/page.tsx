import { OwnerGuard } from "@/components/layout/owner-guard";
import { ApiKeysScaffold } from "@/components/admin/feature-scaffold";

export default function ApiKeysPage() {
  return (
    <OwnerGuard>
      <ApiKeysScaffold />
    </OwnerGuard>
  );
}

import { OwnerGuard } from "@/components/layout/owner-guard";
import { ApiKeysPanel } from "@/components/api-keys/api-keys-panel";

export default function ApiKeysPage() {
  return (
    <OwnerGuard>
      <ApiKeysPanel />
    </OwnerGuard>
  );
}

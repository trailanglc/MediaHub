import { OwnerGuard } from "@/components/layout/owner-guard";
import { SystemStoragePanel } from "@/components/system/system-storage-panel";

export default function StoragePage() {
  return (
    <OwnerGuard>
      <SystemStoragePanel />
    </OwnerGuard>
  );
}

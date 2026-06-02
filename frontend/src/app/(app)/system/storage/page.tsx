import { OwnerGuard } from "@/components/layout/owner-guard";
import { StorageScaffold } from "@/components/admin/feature-scaffold";

export default function StoragePage() {
  return (
    <OwnerGuard>
      <StorageScaffold />
    </OwnerGuard>
  );
}

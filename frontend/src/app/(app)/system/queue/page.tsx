import { OwnerGuard } from "@/components/layout/owner-guard";
import { QueueScaffold } from "@/components/admin/feature-scaffold";

export default function QueuePage() {
  return (
    <OwnerGuard>
      <QueueScaffold />
    </OwnerGuard>
  );
}

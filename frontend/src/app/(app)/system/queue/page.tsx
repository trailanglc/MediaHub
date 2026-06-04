import { OwnerGuard } from "@/components/layout/owner-guard";
import { SystemQueuePanel } from "@/components/system/system-queue-panel";

export default function QueuePage() {
  return (
    <OwnerGuard>
      <SystemQueuePanel />
    </OwnerGuard>
  );
}

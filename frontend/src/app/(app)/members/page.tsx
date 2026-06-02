import { OwnerGuard } from "@/components/layout/owner-guard";
import { MembersPanel } from "@/components/members/members-panel";

export default function MembersPage() {
  return (
    <OwnerGuard>
      <MembersPanel />
    </OwnerGuard>
  );
}

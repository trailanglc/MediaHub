import { Suspense } from "react";
import { PageLoading } from "@/components/feedback/page-states";
import { OwnerGuard } from "@/components/layout/owner-guard";
import { SettingsPanel } from "@/components/settings/settings-panel";

export default function SettingsPage() {
  return (
    <OwnerGuard>
      <Suspense fallback={<PageLoading rows={4} />}>
        <SettingsPanel />
      </Suspense>
    </OwnerGuard>
  );
}

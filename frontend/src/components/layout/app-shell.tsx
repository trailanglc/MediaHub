"use client";

import { AppHeader } from "@/components/layout/app-header";
import { AppSidebar } from "@/components/layout/app-sidebar";
import { MainContentArea } from "@/components/layout/main-content-area";
import { SystemResourceBanner } from "@/components/system/system-resource-banner";

export function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen">
      <AppSidebar className="hidden lg:flex" />
      <div className="flex min-w-0 flex-1 flex-col">
        <AppHeader />
        <SystemResourceBanner />
        <main className="flex-1 overflow-auto p-4 md:p-6 lg:p-8">
          <div className="mx-auto w-full max-w-7xl 2xl:max-w-screen-2xl min-[1920px]:max-w-none">
            <MainContentArea>{children}</MainContentArea>
          </div>
        </main>
      </div>
    </div>
  );
}

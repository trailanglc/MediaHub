import { PageTransitionLoading } from "@/components/layout/page-transition-loading";

export default function AppRouteLoading() {
  return (
    <div className="relative min-h-[calc(100dvh-3.5rem)]">
      <PageTransitionLoading className="animate-none" />
    </div>
  );
}

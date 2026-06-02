import { AuthBrandPanel } from "@/components/auth/auth-brand-panel";

export default function AuthLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="flex min-h-dvh flex-col lg:grid lg:min-h-screen lg:grid-cols-2">
      <AuthBrandPanel />
      <main className="flex flex-1 flex-col items-center justify-center bg-zinc-50 px-4 py-6 pb-[max(1.5rem,env(safe-area-inset-bottom))] sm:px-6 sm:py-10 dark:bg-zinc-950 lg:min-h-screen lg:px-8 lg:py-12">
        <div className="w-full max-w-md">{children}</div>
      </main>
    </div>
  );
}

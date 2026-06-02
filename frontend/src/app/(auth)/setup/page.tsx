import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { SetupForm } from "@/components/auth/setup-form";
import { fetchSetupStatusServer } from "@/lib/setup-server";

export const metadata: Metadata = {
  title: "Thiết lập — MediaHub",
  description: "Tạo tài khoản Owner đầu tiên cho MediaHub",
};

export default async function SetupPage() {
  const status = await fetchSetupStatusServer();
  if (!status.setup_required) {
    redirect("/login");
  }

  const showSetupToken =
    process.env.NEXT_PUBLIC_REQUIRE_SETUP_TOKEN === "true";

  return <SetupForm showSetupToken={showSetupToken} />;
}

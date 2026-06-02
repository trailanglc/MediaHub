import { redirect } from "next/navigation";
import { fetchSetupStatusServer } from "@/lib/setup-server";

export default async function HomePage() {
  const status = await fetchSetupStatusServer();
  if (status.setup_required) {
    redirect("/setup");
  }
  redirect("/login");
}

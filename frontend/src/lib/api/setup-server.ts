import { env } from "@/lib/api/env";
import type { SetupStatusResponse } from "@/lib/api/api-client";

export async function fetchSetupStatusServer(): Promise<SetupStatusResponse> {
  const res = await fetch(`${env.NEXT_PUBLIC_API_URL}/api/setup/status`, {
    cache: "no-store",
  });
  if (!res.ok) {
    throw new Error("Failed to fetch setup status");
  }
  return res.json() as Promise<SetupStatusResponse>;
}

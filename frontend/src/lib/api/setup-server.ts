import { env } from "@/lib/api/env";
import type { SetupStatusResponse } from "@/lib/api/api-client";

/**
 * Server-side setup probe. Never throws — API down must not 500 login/setup RSC.
 */
export async function fetchSetupStatusServer(): Promise<SetupStatusResponse> {
  try {
    const res = await fetch(`${env.NEXT_PUBLIC_API_URL}/api/setup/status`, {
      cache: "no-store",
      signal: AbortSignal.timeout(8000),
    });
    if (!res.ok) {
      return { setup_required: false, setup_allowed: false };
    }
    return (await res.json()) as SetupStatusResponse;
  } catch {
    // ECONNREFUSED / timeout — treat as “already set up” so /login still renders.
    return { setup_required: false, setup_allowed: false };
  }
}

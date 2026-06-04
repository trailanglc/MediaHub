import { revalidateTag } from "next/cache";
import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { HOMEPAGE_CACHE_TAG } from "@/lib/marketing/homepage-config";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export async function POST() {
  const cookieStore = await cookies();
  const access = cookieStore.get("access_token")?.value;
  if (!access) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  let role: string | undefined;
  try {
    const res = await fetch(`${API_URL}/api/auth/me`, {
      headers: { Cookie: `access_token=${access}` },
      cache: "no-store",
    });
    if (!res.ok) {
      return NextResponse.json({ error: "unauthorized" }, { status: 401 });
    }
    const body = (await res.json()) as { user?: { role?: string } };
    role = body.user?.role;
  } catch {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  if (role !== "owner") {
    return NextResponse.json({ error: "forbidden" }, { status: 403 });
  }

  revalidateTag(HOMEPAGE_CACHE_TAG, "max");
  return NextResponse.json({ revalidated: true, tag: HOMEPAGE_CACHE_TAG });
}

import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { isOwnerOnlyPath } from "@/lib/navigation/navigation";

const API_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

function hasAccessCookie(request: NextRequest): boolean {
  return Boolean(request.cookies.get("access_token")?.value);
}

/** RSC / prefetch — AuthGuard phía client sẽ kiểm tra session; tránh gọi API mỗi chunk. */
function isSubsequentNavigationRequest(request: NextRequest): boolean {
  return (
    request.headers.get("RSC") === "1" ||
    request.headers.get("Next-Router-Prefetch") === "1" ||
    request.headers.get("Next-Router-State-Tree") != null
  );
}

async function fetchSessionRole(
  request: NextRequest,
): Promise<"owner" | "manager" | "viewer" | null> {
  const access = request.cookies.get("access_token")?.value;
  if (!access) return null;
  try {
    const res = await fetch(`${API_URL}/api/auth/me`, {
      headers: { Cookie: `access_token=${access}` },
      cache: "no-store",
      signal: AbortSignal.timeout(8000),
    });
    if (!res.ok) return null;
    const data = (await res.json()) as { user?: { role?: string } };
    const role = data.user?.role?.toLowerCase();
    if (role === "owner" || role === "manager" || role === "viewer") {
      return role;
    }
    return null;
  } catch {
    return null;
  }
}

export async function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;

  if (!hasAccessCookie(request)) {
    const login = new URL("/login", request.url);
    login.searchParams.set("next", pathname);
    return NextResponse.redirect(login);
  }

  // Owner-only routes: chặn non-owner (kể cả RSC prefetch).
  if (isOwnerOnlyPath(pathname)) {
    const role = await fetchSessionRole(request);
    if (role !== "owner") {
      return NextResponse.redirect(new URL("/dashboard", request.url));
    }
    return NextResponse.next();
  }

  if (isSubsequentNavigationRequest(request)) {
    return NextResponse.next();
  }

  const role = await fetchSessionRole(request);
  if (!role) {
    const login = new URL("/login", request.url);
    login.searchParams.set("next", pathname);
    return NextResponse.redirect(login);
  }

  return NextResponse.next();
}

// matcher must be a static literal (no spread/map) for Next.js compile-time parsing
export const config = {
  matcher: [
    "/dashboard",
    "/dashboard/:path*",
    "/files",
    "/files/:path*",
    "/videos",
    "/videos/:path*",
    "/members",
    "/members/:path*",
    "/permissions",
    "/permissions/:path*",
    "/api-keys",
    "/api-keys/:path*",
    "/settings",
    "/settings/:path*",
    "/system",
    "/system/:path*",
  ],
};

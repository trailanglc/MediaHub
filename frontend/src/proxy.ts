import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { isOwnerOnlyPath } from "@/lib/navigation/navigation";

const API_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

type Role = "owner" | "manager" | "viewer";

type SessionCheck =
  | { status: "ok"; role: Role; setCookies?: string[] }
  | { status: "unauthenticated"; setCookies?: string[] }
  | { status: "unreachable"; setCookies?: string[] };

function parseRole(raw: unknown): Role | null {
  const role = typeof raw === "string" ? raw.toLowerCase() : "";
  if (role === "owner" || role === "manager" || role === "viewer") {
    return role;
  }
  return null;
}

function cookieHeader(request: NextRequest): string {
  return request.cookies
    .getAll()
    .map((c) => `${c.name}=${c.value}`)
    .join("; ");
}

function collectSetCookies(res: Response): string[] {
  const anyHeaders = res.headers as Headers & {
    getSetCookie?: () => string[];
  };
  if (typeof anyHeaders.getSetCookie === "function") {
    return anyHeaders.getSetCookie();
  }
  const single = res.headers.get("set-cookie");
  return single ? [single] : [];
}

function applySetCookies(res: NextResponse, setCookies?: string[]) {
  if (!setCookies?.length) return res;
  for (const c of setCookies) {
    res.headers.append("Set-Cookie", c);
  }
  return res;
}

async function fetchMe(accessToken: string): Promise<SessionCheck> {
  try {
    const res = await fetch(`${API_URL}/api/auth/me`, {
      headers: { Cookie: `access_token=${accessToken}` },
      cache: "no-store",
      signal: AbortSignal.timeout(8000),
    });
    if (res.status === 401 || res.status === 403) {
      return { status: "unauthenticated" };
    }
    if (!res.ok) {
      return { status: "unreachable" };
    }
    const data = (await res.json()) as { user?: { role?: string } };
    const role = parseRole(data.user?.role);
    if (!role) return { status: "unauthenticated" };
    return { status: "ok", role };
  } catch {
    return { status: "unreachable" };
  }
}

async function tryRefresh(request: NextRequest): Promise<SessionCheck> {
  const refresh = request.cookies.get("refresh_token")?.value;
  if (!refresh) {
    return { status: "unauthenticated" };
  }
  try {
    const res = await fetch(`${API_URL}/api/auth/refresh`, {
      method: "POST",
      headers: {
        Cookie: cookieHeader(request),
        "Content-Type": "application/json",
      },
      cache: "no-store",
      signal: AbortSignal.timeout(8000),
    });
    const setCookies = collectSetCookies(res);
    if (res.status === 401 || res.status === 403) {
      return { status: "unauthenticated", setCookies };
    }
    if (!res.ok) {
      return { status: "unreachable", setCookies };
    }
    const data = (await res.json()) as { user?: { role?: string } };
    const role = parseRole(data.user?.role);
    if (!role) {
      return { status: "unauthenticated", setCookies };
    }
    return { status: "ok", role, setCookies };
  } catch {
    return { status: "unreachable" };
  }
}

/**
 * Resolve session for edge auth. When access is missing/expired but refresh
 * is still valid, rotate tokens here so full page loads are not bounced to /login.
 */
async function fetchSession(request: NextRequest): Promise<SessionCheck> {
  const access = request.cookies.get("access_token")?.value;
  if (access) {
    const me = await fetchMe(access);
    if (me.status === "ok" || me.status === "unreachable") {
      return me;
    }
  }

  const refreshed = await tryRefresh(request);
  return refreshed;
}

/** RSC / prefetch — AuthGuard phía client sẽ kiểm tra session; tránh gọi API mỗi chunk. */
function isSubsequentNavigationRequest(request: NextRequest): boolean {
  return (
    request.headers.get("RSC") === "1" ||
    request.headers.get("Next-Router-Prefetch") === "1" ||
    request.headers.get("Next-Router-State-Tree") != null
  );
}

function redirectLogin(request: NextRequest, setCookies?: string[]) {
  const login = new URL("/login", request.url);
  login.searchParams.set("next", request.nextUrl.pathname);
  return applySetCookies(NextResponse.redirect(login), setCookies);
}

export async function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const hasAccess = Boolean(request.cookies.get("access_token")?.value);
  const hasRefresh = Boolean(request.cookies.get("refresh_token")?.value);

  // No credentials at all → login. If only refresh remains (access MaxAge
  // elapsed), continue and refresh below instead of a false login bounce.
  if (!hasAccess && !hasRefresh) {
    return redirectLogin(request);
  }

  // Owner-only routes: chặn non-owner (kể cả RSC prefetch).
  if (isOwnerOnlyPath(pathname)) {
    const session = await fetchSession(request);
    if (session.status === "unreachable") {
      return applySetCookies(NextResponse.next(), session.setCookies);
    }
    if (session.status !== "ok" || session.role !== "owner") {
      return applySetCookies(
        NextResponse.redirect(new URL("/dashboard", request.url)),
        session.setCookies,
      );
    }
    return applySetCookies(NextResponse.next(), session.setCookies);
  }

  if (isSubsequentNavigationRequest(request)) {
    // Soft nav: do not refresh here — client apiFetch already single-flights
    // refresh on 401. Parallel proxy+client refresh revokes the session family.
    return NextResponse.next();
  }

  const session = await fetchSession(request);
  if (session.status === "unreachable") {
    return applySetCookies(NextResponse.next(), session.setCookies);
  }
  if (session.status !== "ok") {
    return redirectLogin(request, session.setCookies);
  }

  return applySetCookies(NextResponse.next(), session.setCookies);
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
    "/download",
    "/download/:path*",
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

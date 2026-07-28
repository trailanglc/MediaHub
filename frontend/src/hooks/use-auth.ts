"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter, useSearchParams } from "next/navigation";
import {
  ApiError,
  fetchMe,
  login,
  logout,
  type AuthUser,
} from "@/lib/api-client";

export const ME_QUERY_KEY = ["auth", "me"] as const;

export function useMe() {
  return useQuery({
    queryKey: ME_QUERY_KEY,
    queryFn: async () => {
      const res = await fetchMe();
      return res.user;
    },
    retry: false,
    staleTime: 5 * 60 * 1000,
  });
}

export function safeNextPath(next: string | null | undefined): string {
  if (!next) {
    return "/dashboard";
  }
  try {
    const decoded = decodeURIComponent(next);
    if (!decoded.startsWith("/") || decoded.startsWith("//")) {
      return "/dashboard";
    }
    const pathOnly = decoded.split(/[?#]/)[0] ?? decoded;
    const blocked = new Set([
      "/login",
      "/setup",
      "/logout",
      "/register",
      "/forgot-password",
    ]);
    if (blocked.has(pathOnly) || pathOnly.startsWith("/login/")) {
      return "/dashboard";
    }
    return decoded;
  } catch {
    /* ignore malformed ?next= */
  }
  return "/dashboard";
}

export function useLogin() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (body: { email: string; password: string }) => login(body),
    onSuccess: (data) => {
      queryClient.setQueryData<AuthUser>(ME_QUERY_KEY, data.user);
      router.push(safeNextPath(searchParams.get("next")));
      router.refresh();
    },
  });
}

export function useLogout() {
  const router = useRouter();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () => logout(),
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: ME_QUERY_KEY });
      router.push("/login");
      router.refresh();
    },
  });
}

export function isOwner(user: AuthUser | undefined) {
  return user?.role?.toLowerCase() === "owner";
}

export function roleLabel(role: string | undefined): string {
  switch (role?.toLowerCase()) {
    case "owner":
      return "Owner";
    case "manager":
      return "Manager";
    case "viewer":
      return "Viewer";
    default:
      return role ?? "—";
  }
}

export function authErrorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    const body = err.body as { message?: string };
    return body?.message ?? err.message;
  }
  return "Đã xảy ra lỗi. Vui lòng thử lại.";
}

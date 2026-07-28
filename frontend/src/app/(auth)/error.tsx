"use client";

import { useEffect } from "react";
import { PageError } from "@/components/feedback/page-states";

export default function AuthError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  const offline = /fetch failed|econnrefused|failed to fetch|network/i.test(
    error.message + (error.cause instanceof Error ? error.cause.message : ""),
  );

  return (
    <div className="w-full">
      <PageError
        message={
          offline
            ? "Không kết nối được máy chủ API. Chạy make gateway rồi thử lại."
            : error.message || "Đã xảy ra lỗi."
        }
        onRetry={reset}
      />
    </div>
  );
}

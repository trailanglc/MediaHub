"use client";

import { useEffect } from "react";
import { PageError } from "@/components/feedback/page-states";

export default function AppError({
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
    <div className="flex min-h-[50vh] items-center justify-center p-8">
      <div className="w-full max-w-lg">
        <PageError
          message={
            offline
              ? "Không kết nối được máy chủ API. Kiểm tra backend rồi thử lại."
              : error.message || "Đã xảy ra lỗi giao diện."
          }
          onRetry={reset}
        />
      </div>
    </div>
  );
}

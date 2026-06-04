"use client";

import Link from "next/link";
import { useInfiniteQuery } from "@tanstack/react-query";
import {
  fetchVideos,
  VIDEOS_QUERY_KEY,
  type HLSStatus,
  type Video,
} from "@/lib/api/api-client";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { PageHeader } from "@/components/ui/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { useEffect, useState } from "react";
import { FilmIcon } from "lucide-react";

const PAGE_SIZE = 48;
const SEARCH_DEBOUNCE_MS = 300;

const statusLabel: Record<HLSStatus, string> = {
  none: "Chưa HLS",
  pending: "Đang chờ",
  converting: "Đang chuyển mã",
  ready: "Sẵn sàng",
  failed: "Lỗi",
  deleted: "Đã xóa HLS",
};

function statusVariant(
  status: HLSStatus,
): "default" | "secondary" | "destructive" | "outline" {
  if (status === "ready") return "default";
  if (status === "failed") return "destructive";
  if (status === "converting" || status === "pending") return "secondary";
  return "outline";
}

function VideoCard({ video }: { video: Video }) {
  return (
    <Link href={`/videos/${video.public_id}`}>
      <Card className="overflow-hidden transition-shadow hover:shadow-md">
        <div className="relative aspect-video bg-muted">
          {video.thumbnail_url ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={video.thumbnail_url}
              alt=""
              className="size-full object-cover"
            />
          ) : (
            <div className="flex size-full items-center justify-center text-muted-foreground">
              <FilmIcon className="size-10 opacity-40" />
            </div>
          )}
          <Badge
            className="absolute right-2 top-2"
            variant={statusVariant(video.hls_status)}
          >
            {statusLabel[video.hls_status]}
          </Badge>
        </div>
        <CardHeader className="p-3">
          <CardTitle className="line-clamp-1 text-sm font-medium">
            {video.name}
          </CardTitle>
        </CardHeader>
      </Card>
    </Link>
  );
}

export function VideosList() {
  const [searchInput, setSearchInput] = useState("");
  const [q, setQ] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("");

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setQ(searchInput.trim());
    }, SEARCH_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

  const {
    data,
    isLoading,
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteQuery({
    queryKey: [VIDEOS_QUERY_KEY, "list", q, statusFilter],
    queryFn: ({ pageParam }) =>
      fetchVideos({
        q: q || undefined,
        hls_status: statusFilter || undefined,
        limit: String(PAGE_SIZE),
        cursor: pageParam ? String(pageParam) : undefined,
      }),
    initialPageParam: undefined as number | undefined,
    getNextPageParam: (last) => last.next_cursor,
  });

  const items = data?.pages.flatMap((p) => p.items) ?? [];

  return (
    <div className="space-y-6">
      <PageHeader
        title="Videos"
        description="Quản lý video và chuyển mã HLS thủ công."
      />
      <div className="flex flex-wrap gap-2">
        <Input
          placeholder="Tìm theo tên…"
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          className="max-w-xs"
        />
        <select
          className="h-9 rounded-md border bg-background px-3 text-sm"
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
        >
          <option value="">Tất cả trạng thái</option>
          {(Object.keys(statusLabel) as HLSStatus[]).map((s) => (
            <option key={s} value={s}>
              {statusLabel[s]}
            </option>
          ))}
        </select>
      </div>

      {error && (
        <p className="text-sm text-destructive">
          Không tải được danh sách video.
        </p>
      )}

      {isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="aspect-video rounded-xl" />
          ))}
        </div>
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {items.map((v) => (
              <VideoCard key={v.public_id} video={v} />
            ))}
          </div>
          {hasNextPage && (
            <div className="flex justify-center">
              <Button
                variant="outline"
                onClick={() => void fetchNextPage()}
                disabled={isFetchingNextPage}
              >
                {isFetchingNextPage ? "Đang tải…" : "Tải thêm"}
              </Button>
            </div>
          )}
        </>
      )}

      {!isLoading && items.length === 0 && (
        <Card>
          <CardContent className="py-10 text-center text-sm text-muted-foreground">
            Chưa có video. Upload file video trong File Manager.
          </CardContent>
        </Card>
      )}
    </div>
  );
}

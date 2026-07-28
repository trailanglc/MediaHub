"use client";

import { VideoActionsMenu } from "@/components/videos/video-actions-menu";
import type { Video } from "@/lib/api/api-client";

export function VideoTitleActions({ video }: { video: Video }) {
  return (
    <span className="inline-flex min-w-0 max-w-full items-center gap-1">
      <span className="truncate">{video.name}</span>
      <VideoActionsMenu
        video={video}
        align="start"
        triggerClassName="shrink-0 bg-transparent"
      />
    </span>
  );
}

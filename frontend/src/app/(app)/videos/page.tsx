import { Suspense } from "react";
import { VideosList } from "@/components/videos/videos-list";
import { PageLoading } from "@/components/feedback/page-states";

export default function VideosPage() {
  return (
    <Suspense fallback={<PageLoading rows={6} />}>
      <VideosList />
    </Suspense>
  );
}

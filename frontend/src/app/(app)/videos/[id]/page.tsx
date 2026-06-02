import { VideoDetailScaffold } from "@/components/admin/feature-scaffold";

export default async function VideoDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return <VideoDetailScaffold id={id} />;
}

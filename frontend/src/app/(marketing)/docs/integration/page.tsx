import type { Metadata } from "next";
import { IntegrationDocsPanel } from "@/components/docs/integration-docs-panel";
import { loadIntegrationDoc } from "@/lib/docs/load-integration-doc";

export const metadata: Metadata = {
  title: "Integration API",
  description:
    "Hướng dẫn tích hợp MediaHub với website/CMS: API key, upload, delivery URL, HLS convert và webhook.",
  alternates: { canonical: "/docs/integration" },
  openGraph: {
    title: "MediaHub Integration API",
    description:
      "REST API /api/v1 cho CMS: upload media, signed delivery URLs, HLS streaming.",
  },
};

export default function IntegrationDocsPage() {
  const markdown = loadIntegrationDoc();
  return (
    <div className="mx-auto max-w-6xl px-4 py-10 sm:px-6 sm:py-14">
      <IntegrationDocsPanel markdown={markdown} variant="public" />
    </div>
  );
}

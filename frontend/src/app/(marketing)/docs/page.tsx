import type { Metadata } from "next";
import { DocsLayout } from "@/components/docs/docs-layout";
import { DocsPageNav } from "@/components/docs/docs-page-nav";
import { MarkdownContent } from "@/components/docs/markdown-content";
import { DOC_HUB } from "@/lib/docs/manifest";
import { loadDocMarkdown, rewriteDocLinks } from "@/lib/docs/load-doc";

export const metadata: Metadata = {
  title: "Tài liệu",
  description:
    "Tài liệu MediaHub: cài đặt, cấu hình, Integration API và hướng dẫn dashboard.",
  alternates: { canonical: "/docs" },
};

export default function DocsHubPage() {
  const markdown = rewriteDocLinks(loadDocMarkdown(DOC_HUB));

  return (
    <DocsLayout slug="">
      <MarkdownContent source={markdown} />
      <DocsPageNav page={DOC_HUB} />
    </DocsLayout>
  );
}

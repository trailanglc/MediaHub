import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { DocsLayout } from "@/components/docs/docs-layout";
import { DocsPageNav } from "@/components/docs/docs-page-nav";
import { MarkdownContent } from "@/components/docs/markdown-content";
import { ALL_DOC_PAGES, findDocPage } from "@/lib/docs/manifest";
import { loadDocMarkdown, rewriteDocLinks } from "@/lib/docs/load-doc";

type Props = { params: Promise<{ slug: string[] }> };

export function generateStaticParams() {
  return ALL_DOC_PAGES.filter((p) => p.slug).map((p) => ({
    slug: p.slug.split("/"),
  }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug: parts } = await params;
  const slug = parts.join("/");
  const page = findDocPage(slug);
  if (!page) return { title: "Không tìm thấy" };
  return {
    title: page.title,
    alternates: { canonical: `/docs/${slug}` },
  };
}

export default async function DocSlugPage({ params }: Props) {
  const { slug: parts } = await params;
  const slug = parts.join("/");
  const page = findDocPage(slug);
  if (!page) notFound();

  const markdown = rewriteDocLinks(loadDocMarkdown(page));

  return (
    <DocsLayout slug={slug}>
      <MarkdownContent source={markdown} />
      <DocsPageNav page={page} />
    </DocsLayout>
  );
}

import type { MetadataRoute } from "next";
import { ALL_DOC_PAGES } from "@/lib/docs/manifest";

function siteUrl(): string {
  if (process.env.NEXT_PUBLIC_APP_URL) {
    return process.env.NEXT_PUBLIC_APP_URL.replace(/\/$/, "");
  }
  if (process.env.VERCEL_URL) {
    return `https://${process.env.VERCEL_URL}`;
  }
  return "http://localhost:3000";
}

export default function sitemap(): MetadataRoute.Sitemap {
  const base = siteUrl();
  const now = new Date();
  const docUrls: MetadataRoute.Sitemap = [
    {
      url: `${base}/docs`,
      lastModified: now,
      changeFrequency: "weekly",
      priority: 0.95,
    },
    {
      url: `${base}/docs/api`,
      lastModified: now,
      changeFrequency: "weekly",
      priority: 0.9,
    },
    ...ALL_DOC_PAGES.filter((p) => p.slug).map((p) => ({
      url: `${base}/docs/${p.slug}`,
      lastModified: now,
      changeFrequency: "weekly" as const,
      priority: p.slug.startsWith("integration") ? 0.85 : 0.7,
    })),
  ];

  return [
    {
      url: base,
      lastModified: now,
      changeFrequency: "monthly",
      priority: 1,
    },
    ...docUrls,
    {
      url: `${base}/login`,
      lastModified: now,
      changeFrequency: "yearly",
      priority: 0.3,
    },
  ];
}

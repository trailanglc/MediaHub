import type { MetadataRoute } from "next";

function siteUrl(): string {
  if (process.env.NEXT_PUBLIC_APP_URL) {
    return process.env.NEXT_PUBLIC_APP_URL.replace(/\/$/, "");
  }
  if (process.env.VERCEL_URL) {
    return `https://${process.env.VERCEL_URL}`;
  }
  return "http://localhost:3000";
}

export default function robots(): MetadataRoute.Robots {
  const base = siteUrl();
  return {
    rules: {
      userAgent: "*",
      allow: ["/", "/docs/"],
      disallow: ["/dashboard", "/files", "/videos", "/members", "/api-keys", "/settings", "/system"],
    },
    sitemap: `${base}/sitemap.xml`,
  };
}

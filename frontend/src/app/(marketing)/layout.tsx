import type { Metadata } from "next";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import {
  getHomepageConfig,
  homepageSiteName,
} from "@/lib/marketing/homepage-config";

export async function generateMetadata(): Promise<Metadata> {
  const hp = await getHomepageConfig();
  const siteName = homepageSiteName(hp);

  return {
    title: {
      default: hp.meta_title,
      template: `%s | ${siteName}`,
    },
    description: hp.meta_description,
    keywords: hp.keywords,
    openGraph: {
      type: "website",
      locale: "vi_VN",
      title: hp.meta_title,
      description: hp.meta_description,
      siteName,
      ...(hp.og_image_url
        ? { images: [{ url: hp.og_image_url, alt: siteName }] }
        : {}),
    },
    twitter: {
      card: hp.og_image_url ? "summary_large_image" : "summary",
      title: hp.meta_title,
      description: hp.meta_description,
      ...(hp.og_image_url ? { images: [hp.og_image_url] } : {}),
    },
    robots: {
      index: true,
      follow: true,
    },
    ...(hp.favicon_url ? { icons: { icon: hp.favicon_url } } : {}),
  };
}

export default function MarketingLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <MarketingShell>{children}</MarketingShell>;
}

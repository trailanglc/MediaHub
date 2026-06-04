import type { Metadata } from "next";
import { MarketingShell } from "@/components/marketing/marketing-shell";

const siteDescription =
  "MediaHub — nền tảng quản lý media tự host: file manager phân quyền, upload S3/MinIO, streaming HLS và Integration API cho CMS.";

export const metadata: Metadata = {
  title: {
    default: "MediaHub — Self-hosted media & HLS streaming",
    template: "%s | MediaHub",
  },
  description: siteDescription,
  keywords: [
    "media hub",
    "self-hosted",
    "HLS streaming",
    "file manager",
    "S3",
    "MinIO",
    "integration API",
  ],
  openGraph: {
    type: "website",
    locale: "vi_VN",
    title: "MediaHub — Self-hosted media & HLS streaming",
    description: siteDescription,
    siteName: "MediaHub",
  },
  twitter: {
    card: "summary_large_image",
    title: "MediaHub",
    description: siteDescription,
  },
  robots: {
    index: true,
    follow: true,
  },
};

export default function MarketingLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <MarketingShell>{children}</MarketingShell>;
}

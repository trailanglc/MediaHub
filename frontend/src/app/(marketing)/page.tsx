import type { Metadata } from "next";
import { LandingPage } from "@/components/marketing/landing-page";
import { fetchSetupStatusServer } from "@/lib/api/setup-server";

export const metadata: Metadata = {
  alternates: { canonical: "/" },
};

export default async function HomePage() {
  let setupRequired = false;
  try {
    const status = await fetchSetupStatusServer();
    setupRequired = status.setup_required;
  } catch {
    // API offline — still show landing
  }

  const jsonLd = {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    name: "MediaHub",
    applicationCategory: "MultimediaApplication",
    operatingSystem: "Self-hosted",
    description:
      "Self-hosted media asset manager with HLS streaming and Integration API.",
    offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
  };

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <LandingPage setupRequired={setupRequired} />
    </>
  );
}

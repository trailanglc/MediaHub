import type { Metadata } from "next";
import { LandingPage } from "@/components/marketing/landing-page";
import {
  getHomepageConfig,
  homepageSiteName,
} from "@/lib/marketing/homepage-config";
import {
  buildHomepageJsonLd,
  homepageJsonLdScriptContent,
} from "@/lib/marketing/homepage-schema";
import { fetchSetupStatusServer } from "@/lib/api/setup-server";

export async function generateMetadata(): Promise<Metadata> {
  const hp = await getHomepageConfig();
  return {
    alternates: { canonical: "/" },
    title: hp.meta_title,
    description: hp.meta_description,
  };
}

export default async function HomePage() {
  const [hp, setupRequired] = await Promise.all([
    getHomepageConfig(),
    fetchSetupStatusServer()
      .then((s) => s.setup_required)
      .catch(() => false),
  ]);

  const siteName = homepageSiteName(hp);
  const jsonLdBlocks = buildHomepageJsonLd(hp, siteName);

  return (
    <>
      {jsonLdBlocks.length > 0 ? (
        <script
          type="application/ld+json"
          dangerouslySetInnerHTML={{
            __html: homepageJsonLdScriptContent(jsonLdBlocks),
          }}
        />
      ) : null}
      <LandingPage config={hp} setupRequired={setupRequired} />
    </>
  );
}

import type { HomepageConfig } from "@/lib/marketing/homepage-config";

const SCHEMA_EXAMPLE = `[
  {
    "@context": "https://schema.org",
    "@type": "Organization",
    "name": "Công ty của bạn",
    "url": "https://example.com"
  }
]`;

export function schemaCustomToText(custom: unknown[] | null | undefined): string {
  if (!custom || !Array.isArray(custom) || custom.length === 0) {
    return "";
  }
  return JSON.stringify(custom, null, 2);
}

export function parseSchemaCustomText(text: string): unknown[] {
  const trimmed = text.trim();
  if (!trimmed) {
    return [];
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    throw new Error("JSON-LD custom không hợp lệ");
  }
  if (Array.isArray(parsed)) {
    if (parsed.length > 10) {
      throw new Error("Tối đa 10 schema custom");
    }
    for (const item of parsed) {
      if (typeof item !== "object" || item === null || Array.isArray(item)) {
        throw new Error("Mỗi schema phải là object JSON");
      }
    }
    return parsed;
  }
  if (typeof parsed === "object" && parsed !== null) {
    return [parsed];
  }
  throw new Error("Schema custom phải là mảng JSON hoặc một object");
}

export function buildHomepageJsonLd(
  hp: HomepageConfig,
  siteName: string,
): unknown[] {
  const blocks: unknown[] = [];

  if (hp.schema_include_default !== false) {
    blocks.push({
      "@context": "https://schema.org",
      "@type": "SoftwareApplication",
      name: siteName,
      applicationCategory: "MultimediaApplication",
      operatingSystem: "Self-hosted",
      description: hp.meta_description,
      offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
    });
  }

  const custom = Array.isArray(hp.schema_custom) ? hp.schema_custom : [];
  blocks.push(...custom);

  return blocks;
}

export function homepageJsonLdScriptContent(blocks: unknown[]): string {
  if (blocks.length === 0) {
    return "[]";
  }
  if (blocks.length === 1) {
    return JSON.stringify(blocks[0]);
  }
  return JSON.stringify(blocks);
}

export { SCHEMA_EXAMPLE };

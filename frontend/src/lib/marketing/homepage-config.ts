import { unstable_cache } from "next/cache";
import { env } from "@/lib/api/env";

export type HomepageConfig = {
  meta_title: string;
  meta_description: string;
  keywords: string[];
  favicon_object_id: string;
  favicon_url: string;
  og_image_object_id: string;
  og_image_url: string;
  hero_eyebrow: string;
  hero_title: string;
  hero_description: string;
  hero_background_object_id: string;
  hero_background_url: string;
  features_title: string;
  features_description: string;
  cta_title: string;
  cta_description: string;
  schema_include_default: boolean;
  schema_custom: unknown[];
};

export const HOMEPAGE_CACHE_TAG = "homepage-config";

export const DEFAULT_HOMEPAGE: HomepageConfig = {
  meta_title: "MediaHub — Self-hosted media & HLS streaming",
  meta_description:
    "MediaHub — nền tảng quản lý media tự host: file manager phân quyền, upload S3/MinIO, streaming HLS và Integration API cho CMS.",
  keywords: [
    "media hub",
    "self-hosted",
    "HLS streaming",
    "file manager",
    "S3",
    "MinIO",
    "integration API",
  ],
  favicon_object_id: "",
  favicon_url: "",
  og_image_object_id: "",
  og_image_url: "",
  hero_eyebrow: "Self-hosted media platform",
  hero_title: "Quản lý media & streaming trên hạ tầng của bạn",
  hero_description:
    "MediaHub gom file manager, chuyển mã HLS và API tích hợp cho CMS — monorepo Go + Next.js, sẵn sàng triển khai nội bộ hoặc cho khách hàng self-host.",
  hero_background_object_id: "",
  hero_background_url: "",
  features_title: "Tính năng chính",
  features_description:
    "Một dashboard cho team vận hành, một API cho website tích hợp.",
  cta_title: "Tích hợp CMS trong vài phút",
  cta_description:
    "Tạo API key, gọi /api/v1 để upload, lấy delivery URL và embed video HLS.",
  schema_include_default: true,
  schema_custom: [],
};

export function homepageSiteName(config: HomepageConfig): string {
  const parts = config.meta_title.split(" — ");
  return parts[0]?.trim() || "MediaHub";
}

async function fetchHomepageConfigFromAPI(): Promise<HomepageConfig> {
  const res = await fetch(`${env.NEXT_PUBLIC_API_URL}/api/public/homepage`, {
    cache: "no-store",
  });
  if (!res.ok) {
    throw new Error("Failed to fetch homepage config");
  }
  return res.json() as Promise<HomepageConfig>;
}

const loadHomepageConfigCached = unstable_cache(
  fetchHomepageConfigFromAPI,
  [HOMEPAGE_CACHE_TAG],
  { tags: [HOMEPAGE_CACHE_TAG] },
);

/** Trả cache Next.js; chỉ gọi API khi chưa có cache. Xóa cache qua revalidateTag khi lưu Settings. */
export async function getHomepageConfig(): Promise<HomepageConfig> {
  try {
    return await loadHomepageConfigCached();
  } catch {
    return DEFAULT_HOMEPAGE;
  }
}

/** Gọi sau khi Owner lưu tab Trang chủ — xóa cache render Next.js. */
export async function revalidateHomepageCache(): Promise<boolean> {
  const res = await fetch("/api/revalidate/homepage", {
    method: "POST",
    credentials: "include",
  });
  return res.ok;
}

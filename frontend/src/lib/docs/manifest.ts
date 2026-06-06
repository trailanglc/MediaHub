export type DocPage = {
  /** URL segment under /docs (empty string = hub) */
  slug: string;
  title: string;
  /** Path relative to docs root (repo docs/ or public/docs/) */
  file: string;
};

export type DocSection = {
  id: string;
  title: string;
  pages: DocPage[];
};

export const DOC_SECTIONS: DocSection[] = [
  {
    id: "getting-started",
    title: "Bắt đầu",
    pages: [
      {
        slug: "getting-started/overview",
        title: "Tổng quan hệ thống",
        file: "getting-started/01-overview.md",
      },
      {
        slug: "getting-started/installation",
        title: "Cài đặt",
        file: "getting-started/02-installation.md",
      },
      {
        slug: "getting-started/first-run",
        title: "Chạy lần đầu",
        file: "getting-started/03-first-run.md",
      },
    ],
  },
  {
    id: "configuration",
    title: "Cấu hình",
    pages: [
      {
        slug: "configuration",
        title: "Bản đồ cấu hình",
        file: "configuration/README.md",
      },
      {
        slug: "configuration/backend-env",
        title: "Backend env",
        file: "configuration/backend-env.md",
      },
      {
        slug: "configuration/deployments-env",
        title: "Deployments env",
        file: "configuration/deployments-env.md",
      },
      {
        slug: "configuration/frontend-env",
        title: "Frontend env",
        file: "configuration/frontend-env.md",
      },
      {
        slug: "configuration/storage-cdn",
        title: "Storage & CDN",
        file: "configuration/storage-cdn.md",
      },
      {
        slug: "configuration/production-tuning",
        title: "Production tuning",
        file: "configuration/production-tuning.md",
      },
    ],
  },
  {
    id: "integration",
    title: "Integration API",
    pages: [
      {
        slug: "integration",
        title: "Tổng quan API",
        file: "integration/README.md",
      },
      {
        slug: "integration/api-keys",
        title: "API Keys",
        file: "integration/01-api-keys.md",
      },
      {
        slug: "integration/authentication",
        title: "Authentication",
        file: "integration/02-authentication.md",
      },
      {
        slug: "integration/upload",
        title: "Upload",
        file: "integration/03-upload.md",
      },
      {
        slug: "integration/convert-delivery",
        title: "Convert & delivery",
        file: "integration/04-convert-delivery.md",
      },
      {
        slug: "integration/webhooks",
        title: "Webhooks",
        file: "integration/05-webhooks.md",
      },
      {
        slug: "integration/errors-quota",
        title: "Errors & quota",
        file: "integration/06-errors-quota.md",
      },
      {
        slug: "integration/production-checklist",
        title: "Production checklist",
        file: "integration/07-production-checklist.md",
      },
    ],
  },
  {
    id: "usage",
    title: "Dashboard",
    pages: [
      {
        slug: "usage",
        title: "Hướng dẫn sử dụng",
        file: "usage/README.md",
      },
      {
        slug: "usage/auth-members",
        title: "Auth & members",
        file: "usage/auth-members.md",
      },
      {
        slug: "usage/file-manager",
        title: "File Manager",
        file: "usage/file-manager.md",
      },
      {
        slug: "usage/videos-hls",
        title: "Videos & HLS",
        file: "usage/videos-hls.md",
      },
      {
        slug: "usage/system-maintenance",
        title: "System & maintenance",
        file: "usage/system-maintenance.md",
      },
    ],
  },
  {
    id: "reference",
    title: "Tham chiếu",
    pages: [
      {
        slug: "reference/architecture",
        title: "Kiến trúc",
        file: "reference/ARCHITECTURE.md",
      },
      {
        slug: "reference/glossary",
        title: "Thuật ngữ",
        file: "reference/glossary.md",
      },
    ],
  },
];

export const DOC_HUB: DocPage = {
  slug: "",
  title: "Tài liệu MediaHub",
  file: "README.md",
};

export const ALL_DOC_PAGES: DocPage[] = [
  DOC_HUB,
  ...DOC_SECTIONS.flatMap((s) => s.pages),
];

export function findDocPage(slug: string): DocPage | undefined {
  return ALL_DOC_PAGES.find((p) => p.slug === slug);
}

export function docHref(slug: string): string {
  return slug ? `/docs/${slug}` : "/docs";
}

export function adjacentDocs(slug: string): {
  prev?: DocPage;
  next?: DocPage;
} {
  const idx = ALL_DOC_PAGES.findIndex((p) => p.slug === slug);
  if (idx < 0) return {};
  return {
    prev: idx > 0 ? ALL_DOC_PAGES[idx - 1] : undefined,
    next: idx < ALL_DOC_PAGES.length - 1 ? ALL_DOC_PAGES[idx + 1] : undefined,
  };
}

import fs from "node:fs";
import path from "node:path";
import type { DocPage } from "@/lib/docs/manifest";

const DOC_ROOTS = [
  path.join(process.cwd(), "public/docs"),
  path.join(process.cwd(), "../docs"),
];

function resolveDocPath(file: string): string | null {
  for (const root of DOC_ROOTS) {
    const full = path.join(root, file);
    try {
      if (fs.statSync(full).isFile()) return full;
    } catch {
      // try next root
    }
  }
  return null;
}

export function loadDocMarkdown(page: DocPage): string {
  const resolved = resolveDocPath(page.file);
  if (!resolved) {
    return `# ${page.title}\n\nKhông tìm thấy \`${page.file}\`. Chạy \`make docs-sync\` từ repo root.`;
  }
  return fs.readFileSync(resolved, "utf8");
}

/** Rewrite relative markdown links to /docs routes where possible. */
export function rewriteDocLinks(markdown: string): string {
  return markdown
    .replace(/\]\(\/docs\/api\)/g, "](/docs/api)")
    .replace(/\]\((getting-started\/[^)]+\.md)\)/g, (_, p: string) => {
      const map: Record<string, string> = {
        "getting-started/01-overview.md": "/docs/getting-started/overview",
        "getting-started/02-installation.md":
          "/docs/getting-started/installation",
        "getting-started/03-first-run.md": "/docs/getting-started/first-run",
      };
      return `](${map[p] ?? p})`;
    })
    .replace(/\]\((configuration\/[^)]+\.md)\)/g, (_, p: string) => {
      const name = p.replace("configuration/", "").replace(".md", "");
      const slug =
        name === "README" ? "/docs/configuration" : `/docs/configuration/${name}`;
      return `](${slug})`;
    })
    .replace(/\]\((integration\/[^)]+\.md)\)/g, (_, p: string) => {
      const map: Record<string, string> = {
        "integration/README.md": "/docs/integration",
        "integration/01-api-keys.md": "/docs/integration/api-keys",
        "integration/02-authentication.md": "/docs/integration/authentication",
        "integration/03-upload.md": "/docs/integration/upload",
        "integration/04-convert-delivery.md":
          "/docs/integration/convert-delivery",
        "integration/05-webhooks.md": "/docs/integration/webhooks",
        "integration/06-errors-quota.md": "/docs/integration/errors-quota",
        "integration/07-production-checklist.md":
          "/docs/integration/production-checklist",
      };
      return `](${map[p] ?? p})`;
    })
    .replace(/\]\((usage\/[^)]+\.md)\)/g, (_, p: string) => {
      const name = p.replace("usage/", "").replace(".md", "");
      const slug = name === "README" ? "/docs/usage" : `/docs/usage/${name}`;
      return `](${slug})`;
    })
    .replace(/\]\(\.\.\/configuration\/([^)]+\.md)\)/g, (_, p: string) => {
      const name = p.replace(".md", "");
      const slug =
        name === "README" ? "/docs/configuration" : `/docs/configuration/${name}`;
      return `](${slug})`;
    })
    .replace(/\]\(\.\.\/getting-started\/([^)]+\.md)\)/g, (_, p: string) => {
      const map: Record<string, string> = {
        "02-installation.md": "/docs/getting-started/installation",
        "03-first-run.md": "/docs/getting-started/first-run",
      };
      return `](${map[p] ?? p})`;
    })
    .replace(/\]\(\.\.\/integration\/([^)]+\.md)\)/g, (_, p: string) => {
      const map: Record<string, string> = {
        "README.md": "/docs/integration",
        "01-api-keys.md": "/docs/integration/api-keys",
        "02-authentication.md": "/docs/integration/authentication",
        "03-upload.md": "/docs/integration/upload",
        "04-convert-delivery.md": "/docs/integration/convert-delivery",
        "05-webhooks.md": "/docs/integration/webhooks",
        "06-errors-quota.md": "/docs/integration/errors-quota",
        "07-production-checklist.md": "/docs/integration/production-checklist",
      };
      return `](${map[p] ?? p})`;
    })
    .replace(/\]\(\.\.\/usage\/([^)]+\.md)\)/g, (_, p: string) => {
      const name = p.replace(".md", "");
      return name === "README" ? "](/docs/usage)" : `](/docs/usage/${name})`;
    })
    .replace(/\]\(\.\.\/api\/([^)]+\.yaml)\)/g, "](/docs/openapi.yaml)")
    .replace(/\]\(api\/openapi\.yaml\)/g, "](/docs/openapi.yaml)")
    .replace(/\]\(api\/postman-collection\.json\)/g, "](/docs/postman-collection.json)");
}

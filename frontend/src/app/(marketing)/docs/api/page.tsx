import type { Metadata } from "next";
import { DocsLayout } from "@/components/docs/docs-layout";
import { ApiReferencePanel } from "@/components/docs/api-reference-panel";

export const metadata: Metadata = {
  title: "API Reference",
  description:
    "OpenAPI spec MediaHub — Integration API /api/v1 và quản lý API keys.",
  alternates: { canonical: "/docs/api" },
};

export default function DocsApiPage() {
  return (
    <DocsLayout slug="api">
      <header className="mb-6 space-y-2">
        <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">
          API Reference
        </h1>
        <p className="text-muted-foreground">
          OpenAPI 3.1 — Integration API (<code className="text-xs">/api/v1</code>
          ) và Management API (API keys). Tải{" "}
          <a
            href="/docs/openapi.yaml"
            download
            className="text-primary underline-offset-4 hover:underline"
          >
            openapi.yaml
          </a>{" "}
          hoặc{" "}
          <a
            href="/docs/postman-collection.json"
            download
            className="text-primary underline-offset-4 hover:underline"
          >
            Postman collection
          </a>
          .
        </p>
      </header>
      <ApiReferencePanel />
    </DocsLayout>
  );
}

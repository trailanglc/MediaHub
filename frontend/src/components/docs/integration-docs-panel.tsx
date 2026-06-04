"use client";

import Link from "next/link";
import { DownloadIcon, KeyIcon, LogInIcon } from "lucide-react";
import { MarkdownContent } from "@/components/docs/markdown-content";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export function IntegrationDocsPanel({
  markdown,
  variant = "public",
}: {
  markdown: string;
  variant?: "public" | "app";
}) {
  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">
          Integration API
        </h1>
        <p className="max-w-2xl text-muted-foreground">
          Hướng dẫn tích hợp MediaHub với website/CMS qua API key và{" "}
          <code className="rounded bg-muted px-1 text-xs">/api/v1</code>.
        </p>
      </header>

      <div className="flex flex-wrap gap-2">
        {variant === "app" ? (
          <Button
            nativeButton={false}
            render={<Link href="/api-keys" />}
            variant="outline"
            size="sm"
          >
            <KeyIcon className="mr-1.5 size-3.5" />
            Quản lý API Keys
          </Button>
        ) : (
          <Button
            nativeButton={false}
            render={<Link href="/login?next=/api-keys" />}
            variant="outline"
            size="sm"
          >
            <LogInIcon className="mr-1.5 size-3.5" />
            Đăng nhập để tạo API key
          </Button>
        )}
        <Button
          nativeButton={false}
          render={<a href="/docs/openapi.yaml" download />}
          variant="outline"
          size="sm"
        >
          <DownloadIcon className="mr-1.5 size-3.5" />
          OpenAPI (YAML)
        </Button>
        <Button
          nativeButton={false}
          render={<a href="/docs/postman-collection.json" download />}
          variant="outline"
          size="sm"
        >
          <DownloadIcon className="mr-1.5 size-3.5" />
          Postman collection
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Tài liệu</CardTitle>
          <CardDescription>
            Upload, convert HLS, delivery URLs, webhook và quota API key.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <MarkdownContent source={markdown} />
        </CardContent>
      </Card>
    </div>
  );
}

import fs from "node:fs";
import path from "node:path";

const DOC_CANDIDATES = [
  path.join(process.cwd(), "public/docs/integration/README.md"),
  path.join(process.cwd(), "../docs/integration/README.md"),
  path.join(process.cwd(), "public/docs/integration.md"),
  path.join(process.cwd(), "../docs/INTEGRATION.md"),
];

export function loadIntegrationDoc(): string {
  for (const filePath of DOC_CANDIDATES) {
    try {
      return fs.readFileSync(filePath, "utf8");
    } catch {
      // try next path
    }
  }
  return "# Integration Guide\n\nKhông tìm thấy tài liệu. Chạy `make docs-sync` từ repo root.";
}

import type { NextConfig } from "next";
import path from "node:path";
import { fileURLToPath } from "node:url";

const frontendRoot = path.dirname(fileURLToPath(import.meta.url));

const nextConfig: NextConfig = {
  // Giới hạn Turbopack chỉ index trong frontend/, không leo lên monorepo.
  turbopack: {
    root: frontendRoot,
  },
  // Tránh cache dev phình to (thường >2GB) gây restart liên tục và CPU cao.
  experimental: {
    turbopackFileSystemCacheForDev: false,
  },
};

export default nextConfig;

import type { NextConfig } from "next";
import path from "node:path";
import { fileURLToPath } from "node:url";

const frontendRoot = path.dirname(fileURLToPath(import.meta.url));

const nextConfig: NextConfig = {
  // Production deploy: bundle tối thiểu (không cần copy cả node_modules).
  output: "standalone",
  // Cho phép HMR khi mở UI qua IP LAN (SSH / home lab), không chỉ localhost.
  allowedDevOrigins: ["192.168.8.100"],
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

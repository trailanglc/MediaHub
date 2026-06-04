import Link from "next/link";
import {
  CloudIcon,
  FolderTreeIcon,
  KeyIcon,
  PlayCircleIcon,
  ShieldIcon,
  UploadIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";

const FEATURES = [
  {
    icon: FolderTreeIcon,
    title: "File Manager phân quyền",
    description:
      "Cây thư mục, upload lớn qua S3/MinIO, preview, thùng rác và chia sẻ theo nhánh folder.",
  },
  {
    icon: PlayCircleIcon,
    title: "Streaming HLS",
    description:
      "Chuyển mã video sang HLS nhiều bitrate, embed player và signed URL an toàn.",
  },
  {
    icon: KeyIcon,
    title: "Integration API",
    description:
      "API key cho CMS/website bên ngoài: upload, delivery URL, convert và webhook.",
  },
  {
    icon: ShieldIcon,
    title: "Owner & member",
    description:
      "JWT, phân quyền chi tiết theo folder/file, audit và cài đặt runtime cho owner.",
  },
  {
    icon: UploadIcon,
    title: "Upload linh hoạt",
    description:
      "Multipart qua API hoặc presigned PUT thẳng lên object storage — giảm tải backend.",
  },
  {
    icon: CloudIcon,
    title: "Tự host toàn bộ",
    description:
      "Postgres, Redis, MinIO — stack mở, chạy on-prem hoặc VPS của bạn.",
  },
] as const;

export function LandingPage({ setupRequired }: { setupRequired: boolean }) {
  return (
    <>
      {setupRequired && (
        <div className="border-b border-amber-500/30 bg-amber-500/10 px-4 py-2.5 text-center text-sm text-amber-950 dark:text-amber-100">
          Hệ thống chưa được cài đặt.{" "}
          <Link
            href="/setup"
            className="font-medium underline underline-offset-4"
          >
            Tạo tài khoản Owner
          </Link>
        </div>
      )}

      <section className="relative overflow-hidden border-b border-border/60">
        <div
          className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_80%_60%_at_50%_-20%,oklch(0.75_0.12_250/0.18),transparent)]"
          aria-hidden
        />
        <div className="relative mx-auto max-w-6xl px-4 py-20 sm:px-6 sm:py-28 lg:py-32">
          <p className="mb-4 text-sm font-medium uppercase tracking-widest text-primary/80">
            Self-hosted media platform
          </p>
          <h1 className="max-w-3xl text-4xl font-bold tracking-tight sm:text-5xl lg:text-6xl">
            Quản lý media &amp; streaming{" "}
            <span className="text-muted-foreground">trên hạ tầng của bạn</span>
          </h1>
          <p className="mt-6 max-w-2xl text-lg leading-relaxed text-muted-foreground">
            MediaHub gom file manager, chuyển mã HLS và API tích hợp cho CMS —
            monorepo Go + Next.js, sẵn sàng triển khai nội bộ hoặc cho khách hàng
            self-host.
          </p>
          <div className="mt-10 flex flex-wrap gap-3">
            <Button
              nativeButton={false}
              render={<Link href="/login" />}
              size="lg"
            >
              Đăng nhập
            </Button>
            <Button
              nativeButton={false}
              render={<Link href="/docs/integration" />}
              variant="outline"
              size="lg"
            >
              Xem tài liệu API
            </Button>
          </div>
        </div>
      </section>

      <section
        id="features"
        className="mx-auto max-w-6xl px-4 py-20 sm:px-6"
        aria-labelledby="features-heading"
      >
        <div className="mb-12 max-w-2xl">
          <h2 id="features-heading" className="text-2xl font-bold tracking-tight sm:text-3xl">
            Tính năng chính
          </h2>
          <p className="mt-3 text-muted-foreground">
            Một dashboard cho team vận hành, một API cho website tích hợp.
          </p>
        </div>
        <ul className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {FEATURES.map(({ icon: Icon, title, description }) => (
            <li
              key={title}
              className="rounded-xl border border-border/80 bg-card p-6 shadow-sm transition-shadow hover:shadow-md"
            >
              <div className="mb-4 flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
                <Icon className="size-5" aria-hidden />
              </div>
              <h3 className="font-semibold text-foreground">{title}</h3>
              <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
                {description}
              </p>
            </li>
          ))}
        </ul>
      </section>

      <section className="border-t border-border/60 bg-muted/20">
        <div className="mx-auto max-w-6xl px-4 py-16 sm:px-6 sm:py-20">
          <div className="rounded-2xl border border-border bg-card px-6 py-10 text-center shadow-sm sm:px-12">
            <h2 className="text-xl font-bold sm:text-2xl">
              Tích hợp CMS trong vài phút
            </h2>
            <p className="mx-auto mt-3 max-w-xl text-sm text-muted-foreground sm:text-base">
              Tạo API key, gọi <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs">/api/v1</code>{" "}
              để upload, lấy delivery URL và embed video HLS.
            </p>
            <Button
              nativeButton={false}
              render={<Link href="/docs/integration" />}
              className="mt-6"
              size="lg"
            >
              Đọc Integration Guide
            </Button>
          </div>
        </div>
      </section>
    </>
  );
}

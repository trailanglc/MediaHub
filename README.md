# MediaHub

**Tác giả:** [anhtuanlc](https://github.com/anhtuanlc)  
**Giấy phép:** phi thương mại — xem [LICENSE](LICENSE). Mọi sử dụng thương mại cần cấp phép từ anhtuanlc.

---

MediaHub là nền tảng **quản lý media tự host**: file manager phân quyền, upload lớn (S3/MinIO), thùng rác, tìm kiếm, và (đang phát triển) streaming HLS.

Monorepo gồm **backend** (Go, Gin) và **frontend** (Next.js).

## Tính năng chính

- Đăng nhập JWT + refresh, mã hóa mật khẩu RSA (production)
- Owner / member (`manager`, `viewer`), phân quyền theo folder và file
- File Manager: cây thư mục, upload multipart, preview, bulk rename, thùng rác
- Chia sẻ thư mục con — member chỉ thấy nhánh được cấp quyền
- Settings runtime (Owner), system health, bảo trì (purge trash, dọn temp, …)
- Background **scheduler** tách khỏi API (xóa storage, upload expiry, …)

## Tài liệu

| Tài liệu | Nội dung |
|----------|----------|
| [docs/INSTALL.md](docs/INSTALL.md) | Cài đặt, Docker, migrate, chạy API + scheduler + frontend |
| [docs/USAGE.md](docs/USAGE.md) | Auth, members, File Manager, settings, kiểm thử |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Bản đồ thư mục, luồng request, vai trò `cmd/*` |
| [mediahub_production_spec.md](mediahub_production_spec.md) | Đặc tả kiến trúc / production |
| [LICENSE](LICENSE) | Điều khoản sử dụng, phi thương mại, cấp phép thương mại |

## Cấu trúc repo

```txt
backend/       Go API và binary nền (xem docs/ARCHITECTURE.md)
  cmd/api          HTTP API (Gin)
  cmd/scheduler    Bảo trì định kỳ (trash, storage cleanup, …)
  cmd/worker       Hàng đợi Asynq (video convert — khi bật)
  cmd/migrate      Database migrations
  cmd/reset        Xóa dữ liệu ứng dụng (giữ volume Docker)
  cmd/orphan-cleanup  Quét blob MinIO không còn tham chiếu DB
frontend/      Next.js dashboard
deployments/   Docker Compose (Postgres, MinIO, Redis) + ví dụ nginx
docs/          Cài đặt, sử dụng, kiến trúc thư mục
scripts/       wait-postgres, security-smoke
```

Chi tiết layer backend / frontend: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Khởi động nhanh

```bash
cp deployments/.env.example deployments/.env
cp backend/.env.example backend/.env
cp frontend/.env.example frontend/.env.local

make dev-full     # infra + migrate + API + scheduler + worker + FE (một terminal)

# Hoặc từng bước / nhiều terminal: xem docs/INSTALL.md
```

Mở http://localhost:3000/setup để tạo Owner. Chi tiết: [docs/INSTALL.md](docs/INSTALL.md).

## Giấy phép (tóm tắt)

- **Được phép:** sử dụng, sửa, phân phối lại cho mục đích **phi thương mại**, kèm ghi nhận tác giả **anhtuanlc** và giữ file [LICENSE](LICENSE).
- **Không được:** dùng cho mục đích thương mại nếu chưa có **cấp phép bằng văn bản** từ anhtuanlc (lutan2212@gmail.com).

Toàn văn: [LICENSE](LICENSE).

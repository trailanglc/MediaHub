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
| [docs/README.md](docs/README.md) | **Hub tổng** — cài đặt, cấu hình, kết nối API |
| [docs/getting-started/](docs/getting-started/) | Cài đặt và chạy lần đầu |
| [docs/configuration/](docs/configuration/) | Biến môi trường, CDN, production |
| [docs/integration/](docs/integration/) | Integration API `/api/v1` cho CMS |
| [docs/usage/](docs/usage/) | Dashboard Owner/Member |
| [docs/reference/ARCHITECTURE.md](docs/reference/ARCHITECTURE.md) | Bản đồ thư mục, luồng request |
| [mediahub_production_spec.md](mediahub_production_spec.md) | Đặc tả kiến trúc / production |
| [LICENSE](LICENSE) | Điều khoản sử dụng, phi thương mại, cấp phép thương mại |

Trên web (sau `make fe`): http://localhost:3000/docs

## Cấu trúc repo

```txt
backend/       Go API và binary nền (xem docs/ARCHITECTURE.md)
  cmd/gateway      Entry backend: kiểm tra môi trường + API + scheduler + worker
  cmd/api          HTTP API riêng (debug)
  cmd/scheduler    Bảo trì định kỳ riêng (debug)
  cmd/worker       Hàng đợi Asynq riêng (debug)
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

make dev-full     # infra + migrate + make gateway
make fe           # frontend — terminal khác

# Chi tiết: docs/getting-started/02-installation.md
make docs-sync   # đồng bộ tài liệu sang frontend (trước build)
```

Mở http://localhost:3000/setup để tạo Owner. Chi tiết: [docs/README.md](docs/README.md).

## Giấy phép (tóm tắt)

- **Được phép:** sử dụng, sửa, phân phối lại cho mục đích **phi thương mại**, kèm ghi nhận tác giả **anhtuanlc** và giữ file [LICENSE](LICENSE).
- **Không được:** dùng cho mục đích thương mại nếu chưa có **cấp phép bằng văn bản** từ anhtuanlc (lutan2212@gmail.com).

Toàn văn: [LICENSE](LICENSE).

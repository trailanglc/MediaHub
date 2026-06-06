# Chạy lần đầu

Sau khi [cài đặt](02-installation.md) xong, tạo tài khoản Owner và thử hệ thống.

## 1. Tạo Owner

Khi chưa có user `owner`:

1. Mở http://localhost:3000 → **`/setup`**
2. Tạo email + mật khẩu (≥ 8 ký tự, có chữ và số)
3. Đăng nhập tại **`/login`**

API tương đương:

- `GET /api/setup/status`
- `POST /api/setup/owner`

Production (`APP_ENV=production`): bắt buộc `SETUP_TOKEN` và header `X-Setup-Token` (hoặc field `setup_token`).

## 2. Kiểm tra dashboard

Sau đăng nhập Owner:

1. Mở **Dashboard** — xem System Health
2. Mở **File Manager** (`/files`) — upload thử một file ảnh hoặc video
3. Nếu upload video: mở **Videos** (`/videos`) → **Convert** sang HLS (cần worker đang chạy)

## 3. Kiểm tra API

```bash
curl http://localhost:8080/health
```

Đã login Owner trên browser — cookie session hoạt động cho các request `/api/*` từ frontend.

## 4. Tích hợp CMS (tuỳ chọn)

Nếu cần kết nối website bên ngoài:

1. Owner → **Quản trị → API Keys** → tạo key với scopes phù hợp
2. Đọc [Integration API](../integration/README.md)

## Bước tiếp theo

| Mục tiêu | Tài liệu |
|----------|----------|
| Tinh chỉnh env, CDN | [Configuration](../configuration/README.md) |
| Dùng dashboard đầy đủ | [Usage](../usage/README.md) |
| Tích hợp API | [Integration](../integration/README.md) |

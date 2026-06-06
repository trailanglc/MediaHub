# Hướng dẫn sử dụng Dashboard

Tài liệu vận hành MediaHub qua giao diện web. Cài đặt: [Getting started](../getting-started/02-installation.md).

## Dev stack — dừng an toàn

**Backend:** `make gateway` — **Ctrl+C** dừng toàn bộ (API + scheduler + worker).

**Frontend:** `make fe` — **Ctrl+C** dừng Next.js. Process sót: `make dev-stop`.

**Production:** [Production tuning](../configuration/production-tuning.md).

Nếu chạy riêng `make api` / `make scheduler` / `make worker`, **Ctrl+C từng terminal**. Sau khi tắt worker giữa convert, kiểm tra `ps aux | grep ffmpeg`.

## Mục lục

| Chủ đề | Link |
|--------|------|
| Đăng nhập, members, phân quyền | [Auth & members](auth-members.md) |
| File Manager | [File Manager](file-manager.md) |
| Videos & HLS | [Videos & HLS](videos-hls.md) |
| Settings, kiểm thử | [System & maintenance](system-maintenance.md) |

## Tích hợp API

CMS bên ngoài: [Integration API](../integration/README.md) — khác với session dashboard JWT.

## Dashboard theo role

- **Owner**: tổng quan + System Health, toàn quyền quản trị
- **Member**: thư mục được chia sẻ + File Manager trong phạm vi quyền

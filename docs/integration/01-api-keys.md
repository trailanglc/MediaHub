# Tạo API Key

API key là credential cho Integration API (`/api/v1`). Chỉ **Owner** tạo và quản lý key.

## Tạo qua dashboard

1. Đăng nhập owner → **Quản trị → API Keys**
2. **Tạo key mới** — đặt tên, chọn scopes
3. **Lưu secret** — chỉ hiển thị một lần (prefix `mh_`)

## Scopes

| Scope | Mô tả |
|-------|--------|
| `media:upload` | Upload file |
| `media:read` | Đọc metadata, delivery URLs, HLS |
| `media:convert` | Chuyển mã video sang HLS |
| `media:delete` | Xóa media |
| `stream` | Phát HLS qua `/stream`, embed player |

Một key có thể gom nhiều scope. Endpoint `/api/v1` kiểm tra scope riêng từng nhóm route.

## Root folder (tuỳ chọn)

Gán **root folder** — mọi upload/read bị giới hạn trong cây thư mục đó. Hữu ích khi mỗi CMS/site dùng một key riêng.

## Allowed IPs (tuỳ chọn)

Chỉ chấp nhận request từ IP server CMS. Để trống = không giới hạn IP.

## Quản lý qua Management API

Owner JWT (Bearer token từ login):

| Method | Path |
|--------|------|
| GET | `/api/api-keys` |
| POST | `/api/api-keys` |
| PATCH | `/api/api-keys/{public_id}` |
| DELETE | `/api/api-keys/{public_id}` |

Response tạo key gồm `secret` plaintext — lưu ngay, không xem lại được.

## Tiếp theo

Gửi key trong mọi request Integration API: [Authentication](02-authentication.md)

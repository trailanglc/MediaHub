# Authentication

Mọi request Integration API (`/api/v1/*`) cần header:

```http
X-API-Key: mh_...
```

Không dùng query `api_key`. Không dùng JWT cookie dashboard cho `/api/v1`.

## Ví dụ

```bash
curl -H "X-API-Key: $KEY" "$API/api/v1/media/upload/limits"
```

## IP allowlist

Nếu key có **Allowed IPs**, request từ IP khác → `403 forbidden`.

## Scope per route

Mỗi nhóm endpoint yêu cầu scope tương ứng:

| Route prefix | Scope |
|--------------|-------|
| `/api/v1/media/upload/*` | `media:upload` |
| `GET /api/v1/media/{id}`, `delivery-urls`, `hls` | `media:read` |
| `POST .../convert` | `media:convert` |
| `DELETE /api/v1/media/{id}` | `media:delete` |
| `/stream/*`, `/embed/*` (signed URL) | `stream` (khi embed ngoài app) |

Key thiếu scope → `403 forbidden`.

## Lỗi xác thực

| HTTP | `error` | Nguyên nhân |
|------|---------|-------------|
| 401 | `unauthorized` | Thiếu hoặc sai API key |
| 403 | `forbidden` | Key revoked, sai scope, sai IP, ngoài root folder |

Chi tiết: [Errors & quota](06-errors-quota.md)

## Management API (JWT)

Quản lý key (`/api/api-keys`) dùng **Bearer token** Owner từ `POST /api/auth/login` — không dùng `X-API-Key`.

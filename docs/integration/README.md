# Integration API

MediaHub có thể dùng làm **media backend** cho website/CMS khác qua Integration API (`/api/v1`).

## Integration API vs Dashboard API

| | Integration API | Dashboard API |
|--|-----------------|---------------|
| Base path | `/api/v1/media/*` | `/api/*` |
| Auth | Header `X-API-Key` | Cookie JWT / Bearer token |
| Dùng cho | CMS, server backend | Browser dashboard |
| Tạo credential | Owner → API Keys | Đăng nhập `/login` |

## Luồng tích hợp typcial

```
Tạo API Key → Upload media → (Convert HLS) → Delivery URLs → Embed / Stream
                         ↘ Webhook events (tuỳ chọn)
```

## Scopes

| Scope | Quyền |
|-------|--------|
| `media:upload` | Init/chunk/complete upload |
| `media:read` | Metadata, delivery URLs, HLS info |
| `media:convert` | Convert / retry HLS |
| `media:delete` | Xóa media |
| `stream` | Phát HLS qua `/stream`, embed player |

## Tài liệu chi tiết

| Chủ đề | Link |
|--------|------|
| Tạo API key | [01 — API Keys](01-api-keys.md) |
| Xác thực | [02 — Authentication](02-authentication.md) |
| Upload | [03 — Upload](03-upload.md) |
| Convert & delivery | [04 — Convert & delivery](04-convert-delivery.md) |
| Webhooks | [05 — Webhooks](05-webhooks.md) |
| Lỗi & quota | [06 — Errors & quota](06-errors-quota.md) |
| Production | [07 — Production checklist](07-production-checklist.md) |

## API Reference

- Trình duyệt tương tác: [/docs/api](/docs/api)
- OpenAPI: [openapi.yaml](../api/openapi.yaml)
- Postman: [postman-collection.json](../api/postman-collection.json)

## Quick start (curl)

```bash
export API=http://localhost:8080
export KEY=mh_your_key

# Upload init
curl -X POST "$API/api/v1/media/upload/init" \
  -H "X-API-Key: $KEY" \
  -H "Content-Type: application/json" \
  -d '{"file_name":"photo.jpg","size":1024,"mime_type":"image/jpeg"}'
```

Tiếp theo: [Tạo API key](01-api-keys.md)

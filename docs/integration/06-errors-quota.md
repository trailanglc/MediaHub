# Errors & quota

Integration API trả JSON lỗi thống nhất:

```json
{
  "error": "validation_error",
  "message": "human readable detail"
}
```

## Mã lỗi

| HTTP | `error` | Nguyên nhân thường gặp |
|------|---------|------------------------|
| 400 | `validation_error` | Body sai, chunk lỗi, file quá lớn, parent_id invalid |
| 401 | `unauthorized` | Thiếu/sai API key |
| 403 | `forbidden` | Scope thiếu, IP không allowlist, ngoài root folder |
| 404 | `not_found` | Media/session không tồn tại |
| 409 | `conflict` | Convert đang chạy, retry/cancel không được phép |
| 429 | `rate_limited` | Vượt quota API key |
| 503 | `system_busy` | Server quá tải (governor) |
| 503 | `queue_full` | Hàng đợi convert đầy |
| 503 | `queue_paused` | Queue convert đang tạm dừng |
| 507 | `storage_quota_exceeded` | Vượt quota storage workspace (Settings) |
| 500 | `internal_error` | Lỗi server |

## Quota API key

Không áp dụng cho `/stream` segment bytes.

| Env | Mặc định | Áp dụng |
|-----|----------|---------|
| `API_KEY_UPLOAD_INIT_PER_MIN` | 120 | `POST .../upload/init` |
| `API_KEY_CONVERT_PER_HOUR` | 60 | `POST .../convert` |

Vượt quota → HTTP **429** `rate_limited`.

## Retry gợi ý

| Lỗi | Hành động |
|-----|-----------|
| 429 | Exponential backoff, giảm tần suất init/convert |
| 503 system_busy | Retry sau 5–30s |
| 409 conflict | Poll `hls_status`, không gọi convert trùng |
| 400 upload | Kiểm tra chunk index, size, complete đủ chunks |

## Upload lỗi thường gặp

- `ErrUploadTooLarge` — vượt `max_upload_bytes`
- `ErrUploadIncomplete` — complete trước khi gửi đủ chunk
- `ErrUploadInvalidChunk` — index/size chunk sai

Tiếp theo: [Production checklist](07-production-checklist.md)

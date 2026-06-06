# Production checklist — Integration

Checklist trước khi tích hợp CMS production.

## URL & CDN

- [ ] `API_PUBLIC_URL` trỏ domain public API (signed HLS, stream)
- [ ] `CDN_PUBLIC_URL` trỏ CDN (nếu cache `/assets`, `/embed`)
- [ ] Nginx proxy `/assets`, `/embed` về origin — giữ query `e`, `s`
- [ ] `STREAM_INTERNAL_REDIRECT_PREFIX` + nginx nếu offload segment (tuỳ chọn)

## TTL & cache

- [ ] `ASSET_DELIVERY_URL_TTL` phù hợp cache edge CDN
- [ ] `STREAM_SEGMENT_URL_TTL` cân bằng bảo mật vs cache hit
- [ ] `PRESIGNED_PUT_URL_TTL` đủ cho upload lớn (direct mode)

## Bảo mật

- [ ] HTTPS bắt buộc cho webhook endpoints receiver
- [ ] API key **Allowed IPs** = IP server CMS
- [ ] Root folder per key — cô lập tenant/site
- [ ] Không log plaintext `mh_...` secret
- [ ] Rotate key định kỳ; revoke key cũ qua dashboard

## Vận hành

- [ ] Worker convert chạy ổn định (`make worker` hoặc gateway)
- [ ] Monitor System Health — `system_busy`, queue depth
- [ ] Quota `API_KEY_*` tune theo traffic CMS

## Kiểm thử

- [ ] Upload end-to-end (multipart + direct)
- [ ] Convert → poll `hls_status` → delivery URLs
- [ ] Embed/oEmbed từ domain production
- [ ] Webhook verify HMAC + idempotent handler

## Tài liệu liên quan

- [Production tuning](../configuration/production-tuning.md)
- [Storage & CDN](../configuration/storage-cdn.md)
- [OpenAPI spec](../api/openapi.yaml)

# Tài liệu MediaHub

MediaHub là nền tảng **quản lý media tự host**: dashboard phân quyền, lưu trữ S3/MinIO, streaming HLS, và **Integration API** cho website/CMS bên ngoài.

## Ba luồng đọc chính

| Luồng | Dành cho | Bắt đầu tại |
|-------|----------|-------------|
| **Cài đặt** | Admin self-host, dev local | [Tổng quan hệ thống](getting-started/01-overview.md) |
| **Cấu hình** | Tinh chỉnh env, CDN, production | [Bản đồ cấu hình](configuration/README.md) |
| **Kết nối API** | Developer CMS / backend bên ngoài | [Integration API](integration/README.md) |

## Tôi muốn…

| Mục tiêu | Tài liệu |
|----------|----------|
| Cài MediaHub lần đầu trên máy dev | [Cài đặt](getting-started/02-installation.md) |
| Tạo Owner và thử upload | [Chạy lần đầu](getting-started/03-first-run.md) |
| Hiểu biến môi trường backend | [Backend env](configuration/backend-env.md) |
| Bật CDN cho signed URL | [Storage & CDN](configuration/storage-cdn.md) |
| Deploy VPS / systemd | [Production tuning](configuration/production-tuning.md) |
| Tích hợp CMS qua API key | [Tạo API key](integration/01-api-keys.md) |
| Upload file từ server CMS | [Upload](integration/03-upload.md) |
| Lấy URL ảnh / video / embed | [Convert & delivery](integration/04-convert-delivery.md) |
| Nhận webhook khi upload xong | [Webhooks](integration/05-webhooks.md) |
| Dùng dashboard (Owner/Member) | [Hướng dẫn sử dụng](usage/README.md) |
| Xem spec máy đọc (OpenAPI) | [API Reference](/docs/api) — tải [openapi.yaml](api/openapi.yaml) |

## Cấu trúc tài liệu

```
getting-started/   Cài đặt và chạy lần đầu
configuration/     Biến môi trường, CDN, production
integration/       Kết nối API /api/v1 cho CMS
usage/             Dashboard: File Manager, Videos, Members
reference/         Kiến trúc kỹ thuật, thuật ngữ
api/               OpenAPI YAML, Postman collection
```

## Tài nguyên tải về

- [OpenAPI spec](api/openapi.yaml) — import Postman/Insomnia hoặc codegen client
- [Postman collection](api/postman-collection.json)

## Liên kết nhanh

- [README repo](../README.md)
- [Đặc tả production](../mediahub_production_spec.md)
- [Giấy phép](../LICENSE)

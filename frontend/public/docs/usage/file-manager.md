# File Manager

Route dashboard: **`/files`**

Cần migration `000006` trở lên. Chạy `make scheduler` (hoặc `make gateway`) để xóa S3 / purge thùng rác tự động.

## Tính năng

- Upload multipart S3
- Cây thư mục, thùng rác, tìm kiếm
- Bulk rename, preview
- Chia sẻ quyền theo folder/file

## API chính

| Method | Path |
|--------|------|
| GET | `/api/objects`, `/api/objects/search` |
| GET | `/api/objects/trash` |
| POST | `/api/folders` |
| GET/PATCH/DELETE | `/api/objects/{public_id}` |
| POST | `/api/objects/{public_id}/restore` |
| DELETE | `/api/objects/{public_id}/purge` |
| POST | `/api/objects/trash/empty` |
| POST | `/api/upload/init`, chunk, complete |

Upload qua dashboard dùng session JWT — khác Integration API (`/api/v1`).

## Phân quyền

Member cần quyền `read`/`upload`/… trên folder hoặc file. Xem [Auth & members](auth-members.md).

# MediaHub Production Architecture Specification

> Self-hosted Media Asset Manager & HLS Streaming Platform  
> Backend: Go  
> Frontend: Next.js  
> Storage: S3-compatible abstraction  
> Database: PostgreSQL  
> Queue/Cache: Redis  
> Video Processing: FFmpeg  
> Streaming: Nginx / Media Gateway  
> Priority: **Security → Performance → Reliability**

---

## 1. Project Overview

MediaHub is a production-ready, self-hosted media management and HLS video streaming platform.

The system is designed to help an owner manage files, images, and videos from a central web interface, convert selected videos into HLS format, and serve HLS streams securely to external websites or applications.

MediaHub is not a general cloud drive clone. Its primary focus is:

- Centralized media asset management
- Secure file and video storage
- Manual video conversion to HLS
- Secure HLS streaming
- Owner-managed member access
- High-performance media delivery
- Operational visibility for the system owner

---

## 2. Project Goals

### 2.1. Main Goals

MediaHub must provide:

- A web-based dashboard for managing files, images, and videos
- Secure authentication for the system owner and members
- Member management with resource-level permissions
- Folder, file, image, and video organization
- Manual conversion from source video files to HLS
- HLS preview inside the dashboard
- Secure HLS URLs for external websites
- Signed URL support
- Domain allowlist support
- API key support
- System health dashboard
- Storage usage dashboard
- Convert queue monitoring
- Basic stream/access analytics
- Cleanup tools for temporary and orphaned files

### 2.2. Non-Goals

MediaHub is not initially designed for:

- Multi-tenant SaaS
- Billing
- Per-user storage quota
- Public user registration
- Complex organization hierarchy
- Enterprise-grade multi-role administration
- Automatic video conversion after upload
- Direct streaming of source video files

---

## 3. Product Model

MediaHub uses a **single workspace model**.

There is one initial **Owner** account created during the first setup.

The Owner controls the entire system and may create additional member accounts.

Members can only access resources explicitly granted to them.

```txt
Owner
 ├── Full system access
 ├── File/video management
 ├── Member management
 ├── Permission management
 ├── Storage/system dashboard
 └── Stream/security settings

Member
 ├── Access only granted files/folders/videos
 ├── Can view/manage depending on permission
 └── Cannot access system settings unless explicitly allowed in future versions
```

---

## 4. Core Principles

The implementation must follow these principles:

1. Security first.
2. Performance second.
3. Reliability third.
4. Never stream source video files directly.
5. Only HLS-ready video assets can be streamed.
6. Video conversion must be manually triggered.
7. Never trust client-side permission state.
8. Every sensitive operation must be checked server-side.
9. Never expose raw storage paths, internal numeric IDs, secrets, or API key values.
10. Use strong indexing and pagination for all large queries.
11. Use background jobs for heavy work.
12. Keep API, worker, storage, and streaming responsibilities separated.
13. Prefer deterministic, idempotent processing for background jobs.
14. Keep production deployment explicit and observable.

---

## 5. Technology Stack

### 5.1. Backend

```txt
Language: Go
Framework: Gin or Fiber
Database: PostgreSQL
Cache/Queue: Redis
Queue library: Asynq
Storage SDK: AWS SDK for Go v2 or compatible S3 client
Auth: JWT / secure session
Password hashing: Argon2id or bcrypt
Video processing: FFmpeg + FFprobe
Logging: Zap / Zerolog
Metrics: Prometheus-compatible metrics
```

Recommended Go libraries:

```txt
github.com/gin-gonic/gin
github.com/jackc/pgx/v5
github.com/redis/go-redis/v9
github.com/hibiken/asynq
github.com/aws/aws-sdk-go-v2
github.com/golang-jwt/jwt/v5
github.com/google/uuid
github.com/oklog/ulid/v2
go.uber.org/zap
```

### 5.2. Frontend

```txt
Framework: Next.js
Language: TypeScript
Styling: TailwindCSS
UI components: shadcn/ui
Data fetching: TanStack Query
Forms: react-hook-form + zod
State: Zustand
Video playback: hls.js
Upload: Uppy or react-dropzone
Charts: Recharts or ECharts
Tables: TanStack Table
```

### 5.3. Storage

```txt
Object storage: S3-compatible Object Storage-compatible
Buckets/prefixes:
- originals/
- hls/
- thumbnails/
- temp/
- documents/
```

### 5.4. Streaming Layer

```txt
Nginx or Go Media Gateway
```

The streaming layer is responsible for serving HLS manifests and segments efficiently while enforcing stream security policies.

---

## 6. High-Level Architecture

```txt
Browser
  ↓
Next.js Web UI
  ↓
Go API
  ↓
PostgreSQL
  ↓
Redis Queue
  ↓
FFmpeg Worker
  ↓
S3-compatible Object Storage
  ↓
Nginx / Media Gateway
  ↓
External Website / App
```

### 6.1. Component Responsibilities

#### Next.js Web UI

- Login
- Owner setup
- Dashboard
- File manager
- Video manager
- Member management
- Permission management
- API key management
- Stream policy management
- HLS preview
- System health display
- Storage usage display
- Queue monitoring

#### Go API

- Authentication
- Authorization
- Resource management
- Permission checks
- Upload authorization
- Metadata management
- Convert job creation
- Stream token generation
- API key management
- Dashboard data
- System health checks

#### PostgreSQL

Stores:

- Users
- Media objects
- Folder tree
- Video metadata
- Permissions
- API keys
- Stream policies
- Convert jobs
- Audit logs
- System settings

#### Redis

Used for:

- Background job queue
- Job status cache
- Rate limiting
- Session/token revocation cache
- Stream access counters
- Permission cache if needed

#### FFmpeg Worker

Handles:

- FFprobe metadata extraction
- Thumbnail generation
- HLS conversion
- Conversion logs
- Job status updates

#### S3-compatible Object Storage

Stores:

- Original uploaded files
- HLS output
- Thumbnails
- Temporary uploads
- Documents and generic files

#### Nginx / Media Gateway

Handles:

- HLS delivery
- Signed URL validation
- Domain allowlist validation
- Rate limiting
- Static file performance
- Optional cache control

---

## 7. Storage Abstraction Strategy

### 7.1. Design Goal

MediaHub must treat object storage as a replaceable infrastructure provider.

The application should not know whether the actual storage backend is MinIO, Garage, Cloudflare R2, AWS S3, Wasabi, Backblaze B2, or another S3-compatible service.

The system must depend on a storage interface, not a storage vendor.

### 7.2. Development Provider

For local development, MediaHub uses MinIO because it is easy to run with Docker and provides a convenient S3-compatible API.

MinIO is only the default local development provider.

It must not be hard-coded into business logic, database schema, API response models, or storage key naming.

### 7.3. Production Provider

Production deployments should support any S3-compatible storage provider.

Recommended production options:

```txt
Garage
Cloudflare R2
AWS S3
Wasabi
Backblaze B2 S3
MinIO / AIStor if the operator explicitly chooses it
```

### 7.4. Neutral Environment Variables

Configuration must use neutral `STORAGE_*` names, not `MINIO_*` names.

```env
STORAGE_DRIVER=s3
STORAGE_ENDPOINT=http://localhost:9000
STORAGE_REGION=us-east-1
STORAGE_BUCKET=mediahub
STORAGE_ACCESS_KEY=mediahub
STORAGE_SECRET_KEY=mediahub_password
STORAGE_USE_SSL=false
STORAGE_USE_PATH_STYLE=true
```

Provider changes should mostly require configuration changes, not business logic changes.

### 7.5. Required Storage Interface

The backend must define a neutral interface similar to:

```go
type ObjectStorage interface {
    PutObject(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
    GetObject(ctx context.Context, key string) (io.ReadCloser, error)
    DeleteObject(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    PresignGetObject(ctx context.Context, key string, ttl time.Duration) (string, error)
}
```

Recommended adapter structure:

```txt
internal/storage/
├── storage.go          # ObjectStorage interface
├── s3.go               # S3-compatible implementation
├── local.go            # Optional local filesystem implementation for tests
└── keys.go             # Storage key generation
```

### 7.6. Recommended SDK

For long-term S3-compatible flexibility, prefer:

```txt
AWS SDK for Go v2
```

The MinIO Go SDK can also be used, but AWS SDK naming is more provider-neutral and may be easier to adapt across S3-compatible services.

### 7.7. Storage Key Rules

Do not use original filenames as storage keys.

Do not include provider names in storage keys.

Good examples:

```txt
originals/{object_ulid}
hls/{video_public_id}/master.m3u8
hls/{video_public_id}/720p/segment_00001.ts
thumbnails/{object_public_id}.jpg
temp/uploads/{upload_id}/chunk_001
documents/{object_ulid}
```

Bad examples:

```txt
minio/videos/my-video.mp4
uploads/../../../unsafe.mp4
originals/user-input-name.mp4
```

Original filenames are metadata only.

### 7.8. Storage Adapter Rules

The storage adapter must handle:

- Endpoint configuration
- Region configuration
- Path-style or virtual-hosted-style addressing
- SSL on/off
- Bucket name
- Access key
- Secret key
- Presigned URL behavior
- Provider-specific compatibility quirks

Business modules such as upload, video conversion, thumbnails, and streaming must never call provider-specific SDK code directly.

### 7.9. Development Docker Compose

Development may run MinIO through Docker Compose:

```yaml
minio:
  image: minio/minio:latest
  container_name: mediahub-minio
  restart: unless-stopped
  command: server /data --console-address ":9001"
  environment:
    MINIO_ROOT_USER: mediahub
    MINIO_ROOT_PASSWORD: mediahub_password
  ports:
    - "9000:9000"
    - "9001:9001"
  volumes:
    - mediahub_minio:/data
```

Even when MinIO is used in Docker, the application must still read neutral `STORAGE_*` variables.

### 7.10. Migration Between Providers

To support future migration between storage providers:

- Keep object keys provider-neutral.
- Store only object keys in the database.
- Do not store full provider URLs as canonical file references.
- Generate access URLs dynamically.
- Keep bucket/prefix structure consistent.
- Provide a future migration script that can copy objects provider-to-provider.
- Verify checksum after copy where possible.


---

## 8. Owner and Member Model

### 7.1. Initial Owner Setup

On first deployment:

```txt
GET /api/setup/status
```

If no owner exists, setup is allowed.

```txt
POST /api/setup/owner
```

After the first owner is created:

- Setup endpoint must be locked.
- No second owner may be created through setup.
- Owner creation must be audited.
- In production, setup should require a `SETUP_TOKEN` environment variable.

### 7.2. User Roles

MediaHub supports three base roles:

```txt
owner
manager
viewer
```

#### Owner

Owner has full system access.

Capabilities:

- Manage all files/folders/images/videos
- Upload resources
- Delete resources
- Convert videos to HLS
- Manage HLS assets
- Manage members
- Grant/revoke permissions
- Manage API keys
- Manage domain allowlists
- View system health
- View storage usage
- View convert queue
- View security/access logs
- Run cleanup tasks
- Update system settings

#### Manager

Manager can manage resources when granted permission.

Possible capabilities:

- Read
- Upload
- Update
- Delete
- Convert
- Share
- Stream
- Download
- Manage selected resources

Manager cannot access global system settings unless future versions explicitly add that ability.

#### Viewer

Viewer can only read, download, or stream granted resources.

---

## 9. Permission Model

### 8.1. Resource Types

Permissions can be applied to:

```txt
folder
file
image
video
```

### 8.2. Permission Actions

Supported actions:

```txt
read
upload
update
delete
convert
share
stream
download
manage
```

### 8.3. Folder Permission Inheritance

If a member has permission on a folder, the permission applies to child folders and child files.

Example:

```txt
Member A has read permission on /videos/project-a
→ Member A can read all child files/videos inside /videos/project-a
```

The first production version should support:

```txt
allow
revoke
```

It should not initially implement:

```txt
explicit deny
complex priority rules
condition-based access
multi-tenant policy inheritance
```

### 8.4. Permission Evaluation Order

```txt
1. If user is Owner → allow.
2. If direct permission exists on target resource → allow.
3. If inherited permission exists from any parent folder → allow.
4. Otherwise → deny.
```

### 8.5. Permission Check Requirements

Every protected endpoint must call server-side permission checks.

Required checks:

```txt
read object
upload to folder
update object
delete object
convert video
share object
stream video
download file
manage permission
```

Frontend permission state is only for UI display and must never be trusted.

---

## 10. Media Object Model

MediaHub represents folders, files, images, and videos as media objects.

```txt
media_objects
 ├── folder
 ├── file
 ├── image
 └── video
```

Video-specific fields are stored separately in `video_assets`.

This keeps file management consistent while allowing video-specific processing.

---

## 11. Video Lifecycle

### 10.1. Upload

When a video is uploaded:

```txt
Upload source video
→ Store original file
→ Create media object
→ Extract optional metadata
→ hls_status = none
```

The video is stored but not streamable.

### 10.2. Manual Convert

Only users with `convert` permission can start conversion.

```txt
POST /api/videos/{id}/convert
```

Flow:

```txt
Check permission
Check video exists
Check hls_status is none/failed/deleted
Create convert job
Set hls_status = pending
Worker picks job
Set hls_status = converting
Run FFmpeg
Generate HLS output
Validate output
Set hls_status = ready
```

### 10.3. HLS Streaming

Only HLS-ready videos can be streamed.

Required conditions:

```txt
object.type = video
video_assets.hls_status = ready
stream policy is valid
signed URL is valid if required
domain is allowed if required
API key is valid if required
```

Source files must not be streamed directly.

### 10.4. HLS Status Values

```txt
none
pending
converting
ready
failed
deleted
```

### 10.5. HLS Output Structure

Recommended storage layout:

```txt
hls/{video_public_id}/master.m3u8
hls/{video_public_id}/1080p/index.m3u8
hls/{video_public_id}/1080p/segment_00001.ts
hls/{video_public_id}/720p/index.m3u8
hls/{video_public_id}/720p/segment_00001.ts
hls/{video_public_id}/480p/index.m3u8
hls/{video_public_id}/480p/segment_00001.ts
```

---

## 12. ID Strategy

### 11.1. Internal IDs

Use sequential or sortable internal IDs for database primary keys.

Recommended:

```txt
BIGINT
Snowflake ID
```

Do not use UUIDv4 random as the main primary key for high-volume tables.

### 11.2. Public IDs

Use public IDs for API routes and external references.

Recommended:

```txt
UUIDv7
```

### 11.3. Storage Keys

Use storage-safe sortable IDs.

Recommended:

```txt
ULID
UUIDv7
```

### 11.4. Secret Tokens

For API keys, signed URLs, and secrets, use cryptographically secure random values.

Never use sortable IDs as secrets.

### 11.5. Recommended Pattern

```txt
id BIGINT PRIMARY KEY
public_id UUID UNIQUE NOT NULL
```

Internal services use `id`.

External APIs use `public_id`.

---

## 13. Database Schema

### 12.1. users

```sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('owner', 'manager', 'viewer')),
    status TEXT NOT NULL CHECK (status IN ('active', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 12.2. media_objects

```sql
CREATE TABLE media_objects (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE,
    parent_id BIGINT REFERENCES media_objects(id) ON DELETE SET NULL,
    type TEXT NOT NULL CHECK (type IN ('folder', 'file', 'image', 'video')),
    name TEXT NOT NULL,
    original_name TEXT,
    mime_type TEXT,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    storage_key TEXT,
    checksum TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_by BIGINT REFERENCES users(id),
    updated_by BIGINT REFERENCES users(id),
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 12.3. object_paths

Closure table for folder inheritance and descendant queries.

```sql
CREATE TABLE object_paths (
    ancestor_id BIGINT NOT NULL REFERENCES media_objects(id) ON DELETE CASCADE,
    descendant_id BIGINT NOT NULL REFERENCES media_objects(id) ON DELETE CASCADE,
    depth INT NOT NULL,
    PRIMARY KEY (ancestor_id, descendant_id)
);
```

### 12.4. video_assets

```sql
CREATE TABLE video_assets (
    id BIGSERIAL PRIMARY KEY,
    object_id BIGINT NOT NULL UNIQUE REFERENCES media_objects(id) ON DELETE CASCADE,
    duration_seconds INT,
    width INT,
    height INT,
    codec TEXT,
    bitrate BIGINT,
    hls_status TEXT NOT NULL DEFAULT 'none',
    hls_master_key TEXT,
    hls_storage_prefix TEXT,
    thumbnail_key TEXT,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 12.5. permissions

```sql
CREATE TABLE permissions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    resource_type TEXT NOT NULL CHECK (resource_type IN ('folder', 'file', 'image', 'video')),
    resource_id BIGINT NOT NULL REFERENCES media_objects(id) ON DELETE CASCADE,
    permission TEXT NOT NULL CHECK (
        permission IN ('read', 'upload', 'update', 'delete', 'convert', 'share', 'stream', 'download', 'manage')
    ),
    granted_by BIGINT REFERENCES users(id),
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 12.6. api_keys

```sql
CREATE TABLE api_keys (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE,
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL,
    scopes JSONB NOT NULL DEFAULT '[]',
    allowed_domains JSONB NOT NULL DEFAULT '[]',
    allowed_ips JSONB NOT NULL DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'active',
    created_by BIGINT REFERENCES users(id),
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 12.7. stream_policies

```sql
CREATE TABLE stream_policies (
    id BIGSERIAL PRIMARY KEY,
    video_asset_id BIGINT NOT NULL UNIQUE REFERENCES video_assets(id) ON DELETE CASCADE,
    access_mode TEXT NOT NULL DEFAULT 'signed_url',
    allowed_domains JSONB NOT NULL DEFAULT '[]',
    token_ttl_seconds INT NOT NULL DEFAULT 3600,
    allow_download BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 12.8. convert_jobs

```sql
CREATE TABLE convert_jobs (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE,
    video_asset_id BIGINT NOT NULL REFERENCES video_assets(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled')),
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    error TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 12.9. audit_logs

```sql
CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    actor_id BIGINT REFERENCES users(id),
    action TEXT NOT NULL,
    target_type TEXT,
    target_id BIGINT,
    ip TEXT,
    user_agent TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## 14. Required Indexes

```sql
CREATE INDEX idx_media_objects_parent_id ON media_objects(parent_id);
CREATE INDEX idx_media_objects_type ON media_objects(type);
CREATE INDEX idx_media_objects_status ON media_objects(status);
CREATE INDEX idx_media_objects_created_at ON media_objects(created_at);
CREATE INDEX idx_media_objects_public_id ON media_objects(public_id);

CREATE INDEX idx_object_paths_descendant ON object_paths(descendant_id);
CREATE INDEX idx_object_paths_ancestor ON object_paths(ancestor_id);

CREATE INDEX idx_video_assets_object_id ON video_assets(object_id);
CREATE INDEX idx_video_assets_hls_status ON video_assets(hls_status);

CREATE INDEX idx_permissions_user_resource ON permissions(user_id, resource_type, resource_id);
CREATE INDEX idx_permissions_resource ON permissions(resource_type, resource_id);
CREATE INDEX idx_permissions_user_revoked ON permissions(user_id, revoked_at);

CREATE INDEX idx_convert_jobs_status_created ON convert_jobs(status, created_at);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
```

---

## 15. API Specification

### 14.1. Setup

```txt
GET  /api/setup/status
POST /api/setup/owner
```

### 14.2. Auth

```txt
POST /api/auth/login
POST /api/auth/logout
POST /api/auth/refresh
GET  /api/auth/me
```

### 14.3. Members

```txt
GET    /api/members
POST   /api/members
GET    /api/members/{public_id}
PATCH  /api/members/{public_id}
DELETE /api/members/{public_id}
```

### 14.4. Media Objects

```txt
GET    /api/objects
POST   /api/folders
POST   /api/upload
GET    /api/objects/{public_id}
PATCH  /api/objects/{public_id}
DELETE /api/objects/{public_id}
```

### 14.5. Videos

```txt
GET    /api/videos
GET    /api/videos/{public_id}
POST   /api/videos/{public_id}/convert
GET    /api/videos/{public_id}/hls
DELETE /api/videos/{public_id}/hls
```

### 14.6. Permissions

```txt
GET    /api/permissions?resource_id={public_id}
POST   /api/permissions
DELETE /api/permissions/{id}
```

### 14.7. API Keys

```txt
GET    /api/api-keys
POST   /api/api-keys
PATCH  /api/api-keys/{public_id}
DELETE /api/api-keys/{public_id}
```

### 14.8. Stream

```txt
GET /stream/{video_public_id}/master.m3u8
GET /stream/{video_public_id}/{quality}/{segment}
```

### 14.9. System

```txt
GET  /api/system/health
GET  /api/system/storage
GET  /api/system/queue
GET  /api/system/security
GET  /api/system/stream-analytics
POST /api/system/cleanup/temp
POST /api/system/cleanup/orphans
```

---

## 16. Frontend Pages

```txt
/setup
/login
/dashboard
/files
/videos
/videos/[id]
/members
/permissions
/api-keys
/settings
/system/health
/system/storage
/system/queue
/security
```

### 15.1. Dashboard

Must show:

- Storage usage
- Total files
- Total images
- Total videos
- HLS-ready videos
- Convert queue status
- Recent uploads
- Recent stream activity
- Failed jobs
- System health summary

### 15.2. File Manager

Must support:

- Folder tree
- Breadcrumb
- Grid view
- Table view
- Upload
- Rename
- Move
- Delete
- Preview
- Share/manage access
- Filter by type
- Search
- Pagination

### 15.3. Video Manager

Must support:

- Video list
- HLS status
- Manual Convert button
- Convert progress
- HLS preview
- Copy HLS URL
- Copy embed code
- Stream policy configuration
- Delete HLS only
- Delete original video

### 15.4. Members

Must support:

- Create member
- Disable member
- Change role
- View granted resources
- Revoke permissions

### 15.5. System Pages

Owner can view:

- API status
- PostgreSQL status
- Redis status
- MinIO status
- Worker status
- FFmpeg status
- Disk/storage usage
- Queue status
- Security events
- Cleanup tools

---

## 17. Security Engineering Requirements

### 16.1. Authentication

- Use Argon2id or bcrypt for password hashing.
- Enforce minimum password strength.
- Rate limit login attempts.
- Support session/token revocation.
- Rotate refresh tokens.
- Use HttpOnly, Secure, SameSite cookies if using cookie-based auth.
- Never return password hash to frontend.

### 16.2. Setup Security

- Setup route is available only when no owner exists.
- Production setup should require `SETUP_TOKEN`.
- Setup must be disabled after owner creation.
- Setup events must be audited.

### 16.3. Authorization

- All resource operations must perform permission checks.
- Owner bypass is allowed but must still be auditable for sensitive actions.
- Members must not access resources without direct or inherited permission.
- Permission logic must be centralized in one package/module.
- Do not duplicate permission rules across handlers.

### 16.4. Upload Security

- Limit max upload size.
- Support multipart upload for large files.
- Validate file extension and detected file type.
- Do not trust client-provided MIME type only.
- Sanitize original filenames.
- Prevent path traversal.
- Store files using generated storage keys.
- Store uploads in temp first, then commit metadata.
- Clean unfinished temp uploads.
- Optionally integrate antivirus scanning later.

### 16.5. FFmpeg Security

- Use `exec.Command` with separated arguments.
- Never concatenate user input into shell commands.
- Disable remote input unless explicitly needed.
- Run worker with a low-privilege OS user.
- Apply job timeout.
- Limit concurrent conversion jobs.
- Validate HLS output after conversion.
- Clean partial output on failure.
- Conversion job must be idempotent.

### 16.6. Stream Security

- Never stream source video files.
- Only stream `hls_status = ready`.
- Use HMAC signed URLs for protected streams.
- Signed token must include path, video ID, expiry, and optionally IP/domain.
- Keep token TTL short.
- Validate domain allowlist with Origin/Referer, but do not rely only on headers.
- API key must be hashed at rest.
- Rate limit stream requests.
- Do not expose MinIO public buckets directly unless using controlled signed URLs.

### 16.7. API Key Security

- API key is shown only once at creation.
- Store only hash in database.
- Support scopes.
- Support status: active/revoked.
- Support allowed domain/IP restrictions.
- Log last used timestamp.
- Never log raw API key.

### 16.8. Database Security

- Use parameterized queries.
- Do not concatenate SQL strings from user input.
- Enforce pagination on list endpoints.
- Use transactions for multi-step operations.
- Use soft delete for user-facing delete.
- Use hard delete only through cleanup jobs.
- Keep migrations versioned.

### 16.9. Frontend Security

- Do not store secrets in localStorage if avoidable.
- Do not trust frontend role checks.
- Escape unsafe output.
- Validate forms using schema validation.
- Hide internal IDs and storage keys.
- Never expose stack traces in production UI.

### 16.10. Logging Security

Do not log:

- Passwords
- Password hashes
- API keys
- JWTs
- Refresh tokens
- Signed URL tokens
- Storage secret keys

Audit important events:

- Login
- Failed login
- Upload
- Delete
- Convert
- Permission grant
- Permission revoke
- API key creation
- API key revoke
- Stream policy change
- Cleanup
- Queue pause/resume

---

## 18. Performance Engineering Requirements

### 17.1. API Performance

- Use request context timeouts.
- Stream large uploads and downloads.
- Never load full large files into memory.
- Use database connection pool.
- Use pagination.
- Avoid N+1 queries.
- Cache permission checks where safe.
- Keep handlers small and focused.

### 17.2. Database Performance

- Use BIGINT internal IDs.
- Use UUIDv7 public IDs.
- Add required indexes before production use.
- Avoid unbounded folder queries.
- Use closure table for folder hierarchy.
- Use cursor pagination for large object lists.
- Archive or rotate audit logs if needed.

### 17.3. Storage Performance

- Use object storage for production.
- Use multipart uploads for large files.
- Use generated object keys.
- Separate originals, hls, thumbnails, and temp prefixes.
- Avoid excessive synchronous storage checks inside hot paths.
- Provide background consistency checker.

### 17.4. Queue Performance

- Conversion must be asynchronous.
- Limit concurrent FFmpeg jobs.
- Retry with backoff.
- Use idempotency keys.
- Avoid duplicate conversion for same video.
- Store job progress.
- Allow cancel/retry for failed jobs.

### 17.5. Streaming Performance

- Serve HLS using Nginx or optimized gateway.
- Cache HLS segments aggressively when safe.
- Use shorter cache TTL for master playlist.
- Support gzip/brotli only where useful.
- Do not proxy every segment through heavy API logic.
- Rate limit abusive clients.
- Track bandwidth asynchronously.

---

## 19. Reliability Requirements

- API must not crash on failed FFmpeg jobs.
- Failed convert jobs must be recorded with error details.
- Partial HLS output must be cleaned or marked invalid.
- Worker restart must not corrupt job state.
- Upload commit must be transactional between DB and storage metadata.
- Background cleanup must be safe and reversible where possible.
- System health endpoint must report degraded dependencies.
- The app should fail closed for permission/security checks.
- Stream access must deny if token/policy validation fails.

---

## 20. Deployment Requirements

### 19.1. Production Services

```txt
api
web
worker
postgres
redis
minio
nginx
```

### 19.2. Environment Variables

```txt
APP_ENV=production
APP_URL=https://media.example.com

DB_DSN=postgres://...

REDIS_ADDR=redis:6379

STORAGE_DRIVER=s3
STORAGE_ENDPOINT=http://minio:9000
STORAGE_REGION=us-east-1
STORAGE_BUCKET=mediahub
STORAGE_ACCESS_KEY=...
STORAGE_SECRET_KEY=...
STORAGE_USE_SSL=false
STORAGE_USE_PATH_STYLE=true

JWT_SECRET=...
SETUP_TOKEN=...

FFMPEG_PATH=/usr/bin/ffmpeg
FFPROBE_PATH=/usr/bin/ffprobe

STREAM_SIGNING_SECRET=...
```

### 19.3. Nginx Responsibilities

- TLS termination if not handled by external proxy
- Serve HLS static files or proxy to gateway
- Apply rate limits
- Apply cache headers
- Reject oversized requests where appropriate
- Hide upstream details

---

## 21. Observability

### 20.1. Metrics

Track:

- API request count
- API latency
- Upload count
- Upload failure count
- Convert job count
- Convert duration
- Convert failure count
- Stream request count
- Stream bandwidth
- Storage usage
- Redis queue size
- DB connection pool usage
- Worker status

### 20.2. Logs

Required logs:

- API logs
- Worker logs
- Stream access logs
- Security logs
- Audit logs
- Error logs

### 20.3. Health Checks

Required health endpoints:

```txt
/api/system/health
/api/system/health/database
/api/system/health/redis
/api/system/health/storage
/api/system/health/worker
/api/system/health/ffmpeg
```

---

## 22. Cleanup and Maintenance

Owner must be able to run:

- Clean expired temp uploads
- Clean failed partial HLS output
- Clean orphaned thumbnails
- Clean orphaned HLS assets
- Verify database-to-storage consistency
- Rebuild storage index
- Retry failed jobs
- Clear old audit logs according to retention settings

Cleanup jobs must be safe:

- Dry-run mode recommended
- Log deleted object keys
- Never delete active referenced objects
- Prefer soft-delete workflow before hard-delete

---

## 23. Roadmap

### Phase 1 - Production MVP

- Owner setup
- Login/logout
- Dashboard
- File manager
- Upload file/video
- PostgreSQL
- MinIO
- Redis
- FFmpeg worker
- Manual HLS convert
- HLS preview
- Signed HLS URL
- Basic system health

### Phase 2 - Member Access

- Member creation
- Manager/viewer roles
- Resource-level permissions
- Folder permission inheritance
- Permission revoke
- Audit logs

### Phase 3 - Stream Security

- Domain allowlist
- API key support
- Stream policy UI
- Stream analytics
- Rate limits

### Phase 4 - Operations

- Cleanup tools
- Queue management
- Failed job retry
- Storage consistency checker
- Better system dashboard
- Backup/restore documentation

### Phase 5 - Advanced Features

- Public share pages
- Embed player page
- Advanced transcoding profiles
- Multi-worker scaling
- CDN integration
- OpenTelemetry integration

---

## 24. Final Architecture Summary

MediaHub is a production-focused self-hosted media platform with:

```txt
Single workspace
One initial Owner
Owner-managed members
Resource-level permissions
PostgreSQL database
Redis queue/cache
S3-compatible object storage
Go backend
Next.js frontend
FFmpeg worker
Nginx/Gateway streaming
Manual HLS conversion
HLS-only streaming
Signed URL and domain/API-key protection
System health and storage dashboard
Security-first implementation
```

The system should remain simple enough to be self-hosted, but strong enough to handle real production media workloads securely and efficiently.


---

## 25. Storage Provider Decision

MediaHub development uses MinIO for local S3-compatible testing.

Production storage must be provider-neutral and based on the S3-compatible adapter.

The system must be able to switch storage providers by changing configuration and, if needed, adapter-level compatibility settings.

Supported storage targets:

```txt
MinIO for local development
Garage for self-hosted production
Cloudflare R2
AWS S3
Wasabi
Backblaze B2 S3
MinIO / AIStor if explicitly selected
```

Storage is implemented through a provider-neutral S3-compatible adapter.

Business logic must never depend on MinIO-specific behavior.

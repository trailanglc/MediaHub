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

CREATE TABLE object_paths (
    ancestor_id BIGINT NOT NULL REFERENCES media_objects(id) ON DELETE CASCADE,
    descendant_id BIGINT NOT NULL REFERENCES media_objects(id) ON DELETE CASCADE,
    depth INT NOT NULL,
    PRIMARY KEY (ancestor_id, descendant_id)
);

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

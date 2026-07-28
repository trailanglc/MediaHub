CREATE TABLE download_jobs (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE,
    created_by BIGINT REFERENCES users(id),
    source_url TEXT NOT NULL,
    resolved_url TEXT,
    kind TEXT NOT NULL CHECK (kind IN ('direct', 'hls', 'ytdlp', 'html')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled')),
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    title TEXT,
    thumbnail_url TEXT,
    mime_hint TEXT,
    selected_format JSONB NOT NULL DEFAULT '{}',
    parent_folder_id BIGINT REFERENCES media_objects(id) ON DELETE SET NULL,
    video_public_id UUID,
    object_id BIGINT REFERENCES media_objects(id) ON DELETE SET NULL,
    progress_pct INT NOT NULL DEFAULT 0,
    bytes_done BIGINT NOT NULL DEFAULT 0,
    bytes_total BIGINT NOT NULL DEFAULT 0,
    last_error TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_download_jobs_created_by_created ON download_jobs(created_by, created_at DESC);
CREATE INDEX idx_download_jobs_status_created ON download_jobs(status, created_at);

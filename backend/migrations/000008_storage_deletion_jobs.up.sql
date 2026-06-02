CREATE TABLE storage_deletion_jobs (
    id BIGSERIAL PRIMARY KEY,
    object_id BIGINT NOT NULL REFERENCES media_objects(id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL,
    is_prefix BOOLEAN NOT NULL DEFAULT false,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    attempts INT NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_storage_deletion_jobs_pending
    ON storage_deletion_jobs (status, created_at)
    WHERE status IN ('pending', 'failed');

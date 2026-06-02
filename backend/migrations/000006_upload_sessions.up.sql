CREATE TABLE upload_sessions (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_object_id BIGINT NOT NULL REFERENCES media_objects(id) ON DELETE CASCADE,
    original_name TEXT NOT NULL,
    mime_type TEXT,
    total_size BIGINT NOT NULL,
    chunk_size INT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'aborted', 'expired')),
    storage_prefix TEXT NOT NULL,
    committed_object_id BIGINT REFERENCES media_objects(id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_upload_sessions_user_status ON upload_sessions(user_id, status);
CREATE INDEX idx_upload_sessions_expires_at ON upload_sessions(expires_at);

ALTER TABLE media_objects
    ADD COLUMN IF NOT EXISTS thumbnail_key TEXT;

CREATE INDEX idx_media_objects_thumbnail_key
    ON media_objects (thumbnail_key)
    WHERE thumbnail_key IS NOT NULL AND deleted_at IS NULL;

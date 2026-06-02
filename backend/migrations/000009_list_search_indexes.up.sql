CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- List children under a folder (filter + sort by name, id).
CREATE INDEX idx_media_objects_parent_active_list
    ON media_objects (parent_id, name, id)
    WHERE deleted_at IS NULL AND status = 'active';

CREATE INDEX idx_media_objects_name_trgm
    ON media_objects USING gin (name gin_trgm_ops);

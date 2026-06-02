-- Trash list: ORDER BY deleted_at DESC, id DESC (ListDeleted).
CREATE INDEX idx_media_objects_deleted_list
    ON media_objects (deleted_at DESC, id DESC)
    WHERE deleted_at IS NOT NULL;

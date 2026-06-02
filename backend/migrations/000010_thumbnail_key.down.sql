DROP INDEX IF EXISTS idx_media_objects_thumbnail_key;
ALTER TABLE media_objects DROP COLUMN IF EXISTS thumbnail_key;

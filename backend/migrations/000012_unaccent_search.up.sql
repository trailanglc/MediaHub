-- Accent-insensitive search: "anh" matches "Ảnh", "Anh", etc.
CREATE EXTENSION IF NOT EXISTS unaccent;

CREATE OR REPLACE FUNCTION f_unaccent(text)
RETURNS text
LANGUAGE sql
IMMUTABLE
PARALLEL SAFE
STRICT
AS $$
  SELECT public.unaccent('public.unaccent', $1)
$$;

DROP INDEX IF EXISTS idx_media_objects_name_trgm;

CREATE INDEX idx_media_objects_name_unaccent_trgm
    ON media_objects USING gin (f_unaccent(name) gin_trgm_ops);

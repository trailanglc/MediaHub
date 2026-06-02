DROP INDEX IF EXISTS idx_media_objects_name_unaccent_trgm;

CREATE INDEX idx_media_objects_name_trgm
    ON media_objects USING gin (name gin_trgm_ops);

DROP FUNCTION IF EXISTS f_unaccent(text);

-- Do not drop extension unaccent; other DBs may use it.

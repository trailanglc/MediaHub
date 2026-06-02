DROP INDEX IF EXISTS idx_media_objects_name_trgm;
DROP INDEX IF EXISTS idx_media_objects_parent_active_list;

-- Do not drop extension pg_trgm; other DBs may use it.

DELETE FROM object_paths
WHERE descendant_id IN (
    SELECT id FROM media_objects WHERE public_id = '00000000-0000-4000-8000-000000000001'::uuid
);

DELETE FROM media_objects WHERE public_id = '00000000-0000-4000-8000-000000000001'::uuid;

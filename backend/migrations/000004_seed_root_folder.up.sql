-- Dev seed: root folder for permission testing (idempotent via fixed public_id).
INSERT INTO media_objects (public_id, parent_id, type, name, status)
SELECT '00000000-0000-4000-8000-000000000001'::uuid, NULL, 'folder', 'Root', 'active'
WHERE NOT EXISTS (
    SELECT 1 FROM media_objects WHERE public_id = '00000000-0000-4000-8000-000000000001'::uuid
);

INSERT INTO object_paths (ancestor_id, descendant_id, depth)
SELECT m.id, m.id, 0
FROM media_objects m
WHERE m.public_id = '00000000-0000-4000-8000-000000000001'::uuid
  AND NOT EXISTS (
    SELECT 1 FROM object_paths op
    WHERE op.ancestor_id = m.id AND op.descendant_id = m.id
  );

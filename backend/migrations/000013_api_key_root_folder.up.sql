ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS root_folder_public_id UUID REFERENCES media_objects(public_id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_api_keys_root_folder ON api_keys(root_folder_public_id);

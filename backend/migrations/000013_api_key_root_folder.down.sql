DROP INDEX IF EXISTS idx_api_keys_root_folder;
ALTER TABLE api_keys DROP COLUMN IF EXISTS root_folder_public_id;

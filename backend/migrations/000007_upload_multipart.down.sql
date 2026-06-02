ALTER TABLE upload_sessions
    DROP COLUMN IF EXISTS parts_json,
    DROP COLUMN IF EXISTS final_storage_key,
    DROP COLUMN IF EXISTS s3_upload_id;

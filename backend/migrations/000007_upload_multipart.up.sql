ALTER TABLE upload_sessions
    ADD COLUMN IF NOT EXISTS s3_upload_id TEXT,
    ADD COLUMN IF NOT EXISTS final_storage_key TEXT,
    ADD COLUMN IF NOT EXISTS parts_json JSONB NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN upload_sessions.s3_upload_id IS 'S3 multipart upload ID';
COMMENT ON COLUMN upload_sessions.final_storage_key IS 'Destination object key assembled via multipart';
COMMENT ON COLUMN upload_sessions.parts_json IS 'Array of {part_number, etag} for completed parts';

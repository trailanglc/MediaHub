CREATE TABLE system_settings (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL,
    updated_by BIGINT REFERENCES users(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Defaults; effective values still merge with .env on read when keys are absent.
INSERT INTO system_settings (key, value) VALUES
    ('workspace.name', '"MediaHub"'::jsonb),
    ('workspace.public_url', '"http://localhost:3000"'::jsonb),
    ('media.default_root_folder_public_id', '"00000000-0000-4000-8000-000000000001"'::jsonb),
    ('media.max_upload_bytes', '5368709120'::jsonb),
    ('security.login_max_attempts', '5'::jsonb),
    ('security.login_lockout_minutes', '15'::jsonb),
    ('streaming.default_token_ttl_seconds', '3600'::jsonb),
    ('streaming.global_allowed_domains', '[]'::jsonb),
    ('storage.quota_bytes', '0'::jsonb),
    ('maintenance.audit_retention_days', '90'::jsonb)
ON CONFLICT (key) DO NOTHING;

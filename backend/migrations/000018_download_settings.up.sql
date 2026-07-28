-- Seed download settings (optional; absent keys fall back to DOWNLOAD_* env defaults).
INSERT INTO system_settings (key, value) VALUES
    ('download.max_concurrent', '2'::jsonb),
    ('download.chunk_concurrency', '2'::jsonb),
    ('download.job_timeout_seconds', '7200'::jsonb),
    ('download.analyze_timeout_seconds', '60'::jsonb),
    ('download.queue_max_depth', '50'::jsonb)
ON CONFLICT (key) DO NOTHING;

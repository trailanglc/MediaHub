DELETE FROM system_settings WHERE key IN (
    'download.max_concurrent',
    'download.chunk_concurrency',
    'download.job_timeout_seconds',
    'download.analyze_timeout_seconds',
    'download.queue_max_depth'
);

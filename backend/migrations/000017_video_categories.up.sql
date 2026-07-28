CREATE TABLE video_categories (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE,
    name TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_video_categories_name_lower ON video_categories (lower(name));

ALTER TABLE video_assets
    ADD COLUMN category_id BIGINT REFERENCES video_categories(id) ON DELETE SET NULL;

CREATE INDEX idx_video_assets_category_id ON video_assets(category_id);

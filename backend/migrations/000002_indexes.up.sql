CREATE INDEX idx_media_objects_parent_id ON media_objects(parent_id);
CREATE INDEX idx_media_objects_type ON media_objects(type);
CREATE INDEX idx_media_objects_status ON media_objects(status);
CREATE INDEX idx_media_objects_created_at ON media_objects(created_at);
CREATE INDEX idx_media_objects_public_id ON media_objects(public_id);

CREATE INDEX idx_object_paths_descendant ON object_paths(descendant_id);
CREATE INDEX idx_object_paths_ancestor ON object_paths(ancestor_id);

CREATE INDEX idx_video_assets_object_id ON video_assets(object_id);
CREATE INDEX idx_video_assets_hls_status ON video_assets(hls_status);

CREATE INDEX idx_permissions_user_resource ON permissions(user_id, resource_type, resource_id);
CREATE INDEX idx_permissions_resource ON permissions(resource_type, resource_id);
CREATE INDEX idx_permissions_user_revoked ON permissions(user_id, revoked_at);

CREATE INDEX idx_convert_jobs_status_created ON convert_jobs(status, created_at);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);

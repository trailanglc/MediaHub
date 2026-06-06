import { env } from "@/lib/api/env";
import { encryptPassword } from "@/lib/auth/password-crypto";

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
    public body?: unknown,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

type RequestOptions = RequestInit & {
  params?: Record<string, string>;
};

function buildURL(path: string, params?: Record<string, string>) {
  const url = new URL(path, env.NEXT_PUBLIC_API_URL);
  if (params) {
    Object.entries(params).forEach(([k, v]) => url.searchParams.set(k, v));
  }
  return url.toString();
}

let refreshInFlight: Promise<boolean> | null = null;

async function tryRefreshSession(): Promise<boolean> {
  if (!refreshInFlight) {
    refreshInFlight = refreshAuth()
      .then(() => true)
      .catch(() => false)
      .finally(() => {
        refreshInFlight = null;
      });
  }
  return refreshInFlight;
}

export async function apiFetch<T>(
  path: string,
  options: RequestOptions = {},
  retried = false,
): Promise<T> {
  const { params, headers, ...init } = options;
  const res = await fetch(buildURL(path, params), {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...headers,
    },
    credentials: "include",
  });

  const text = await res.text();
  let data: unknown = null;
  if (text) {
    try {
      data = JSON.parse(text) as unknown;
    } catch {
      data = text;
    }
  }

  if (
    res.status === 401 &&
    !retried &&
    path !== "/api/auth/login" &&
    path !== "/api/auth/refresh"
  ) {
    const refreshed = await tryRefreshSession();
    if (refreshed) {
      return apiFetch<T>(path, options, true);
    }
  }

  if (!res.ok) {
    throw new ApiError(
      (data as { message?: string })?.message ?? res.statusText,
      res.status,
      data,
    );
  }

  return data as T;
}

export type ComponentHealth = {
  status: string;
  error?: string;
  details?: Record<string, string>;
};

export type HealthWarning = {
  level: "warning" | "critical" | string;
  code: string;
  message: string;
};

export type HealthResponse = {
  status: string;
  timestamp: string;
  api_uptime_seconds: number;
  go_version: string;
  num_goroutine: number;
  metrics_cache_ttl_seconds?: number;
  metrics_cached_at?: string;
  host?: {
    hostname: string;
    os: string;
    platform: string;
    kernel_version: string;
    uptime_seconds: number;
    cpu_percent: number;
    cpu_count: number;
    memory: {
      total_bytes: number;
      used_bytes: number;
      available_bytes: number;
      used_percent: number;
    };
    disks: Array<{
      path: string;
      total_bytes: number;
      used_bytes: number;
      free_bytes: number;
      used_percent: number;
    }>;
  };
  warnings?: HealthWarning[];
  resource_snapshot?: {
    cpu_idle_percent: number;
    ram_idle_percent: number;
    redis_idle_percent: number;
    headroom_percent: number;
    pressure: number;
    cpu_count: number;
  };
  resource_limits?: {
    convert_slots: number;
    defer_convert: boolean;
    system_busy: boolean;
    ffmpeg_threads: number;
  };
  queue_depth?: number;
  queue_max_depth?: number;
  components: Record<string, ComponentHealth>;
};

export const SYSTEM_HEALTH_QUERY_KEY = ["system-health"] as const;

export function fetchSystemHealth() {
  return apiFetch<HealthResponse>("/api/system/health");
}

export type CleanupTempResult = {
  expired_upload_sessions: number;
  removed_temp_objects: number;
};

export type CleanupOrphansResult = {
  dry_run: boolean;
  scanned: number;
  orphan_count: number;
  deleted: number;
  sample_keys?: string[];
  truncated?: boolean;
};

export function cleanupTemp() {
  return apiFetch<CleanupTempResult>("/api/system/cleanup/temp", { method: "POST" });
}

export function cleanupOrphans(body?: { dry_run?: boolean; max_delete?: number }) {
  return apiFetch<CleanupOrphansResult>("/api/system/cleanup/orphans", {
    method: "POST",
    body: JSON.stringify(body ?? { dry_run: true }),
  });
}

export type SetupStatusResponse = {
  setup_required: boolean;
  setup_allowed: boolean;
};

export type CreateOwnerResponse = {
  public_id: string;
  email: string;
  role: string;
};

export function fetchSetupStatus() {
  return apiFetch<SetupStatusResponse>("/api/setup/status");
}

export async function createOwner(body: {
  email: string;
  password: string;
  setup_token?: string;
}) {
  const encrypted_password = await encryptPassword(body.password);
  return apiFetch<CreateOwnerResponse>("/api/setup/owner", {
    method: "POST",
    body: JSON.stringify({
      email: body.email,
      encrypted_password,
      setup_token: body.setup_token,
    }),
  });
}

export type AuthUser = {
  public_id: string;
  email: string;
  role: "owner" | "manager" | "viewer";
};

type AuthResponse = { user: AuthUser };

export async function login(body: { email: string; password: string }) {
  const encrypted_password = await encryptPassword(body.password);
  return apiFetch<AuthResponse>("/api/auth/login", {
    method: "POST",
    body: JSON.stringify({
      email: body.email,
      encrypted_password,
    }),
  });
}

export function logout() {
  return apiFetch<void>("/api/auth/logout", { method: "POST" });
}

export function fetchMe() {
  return apiFetch<AuthResponse>("/api/auth/me");
}

export function refreshAuth() {
  return apiFetch<AuthResponse>("/api/auth/refresh", { method: "POST" });
}

export type Member = AuthUser & {
  status: string;
  created_at: string;
  updated_at: string;
};

export function fetchMembers(params?: { cursor?: string; limit?: string }) {
  return apiFetch<{ items: Member[] }>("/api/members", { params });
}

export async function createMember(body: {
  email: string;
  password: string;
  role: string;
}) {
  const encrypted_password = await encryptPassword(body.password);
  return apiFetch<{ user: Member }>("/api/members", {
    method: "POST",
    body: JSON.stringify({
      email: body.email,
      role: body.role,
      encrypted_password,
    }),
  });
}

export async function updateMember(
  publicId: string,
  body: { role?: string; status?: string; password?: string },
) {
  const payload: Record<string, string> = {};
  if (body.role) payload.role = body.role;
  if (body.status) payload.status = body.status;
  if (body.password) {
    payload.encrypted_password = await encryptPassword(body.password);
  }
  return apiFetch<{ user: Member }>(`/api/members/${publicId}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  });
}

export function disableMember(publicId: string) {
  return apiFetch<void>(`/api/members/${publicId}`, { method: "DELETE" });
}

export function restoreMember(publicId: string) {
  return apiFetch<{ user: Member }>(`/api/members/${publicId}/restore`, {
    method: "POST",
  });
}

export function purgeMember(publicId: string) {
  return apiFetch<void>(`/api/members/${publicId}/purge`, { method: "DELETE" });
}

export type PermissionGrant = {
  id: number;
  user_public_id: string;
  user_email: string;
  resource_public_id: string;
  resource_name: string;
  resource_type: string;
  permission: string;
  created_at: string;
};

export const MY_PERMISSIONS_QUERY_KEY = ["permissions", "mine"] as const;

export function fetchMyPermissions() {
  return apiFetch<{ items: PermissionGrant[] }>("/api/permissions/mine");
}

export function fetchPermissions(resourceId: string) {
  return apiFetch<{ items: PermissionGrant[] }>("/api/permissions", {
    params: { resource_id: resourceId },
  });
}

export function grantPermission(body: {
  user_public_id: string;
  resource_public_id: string;
  permission: string;
}) {
  return apiFetch<{ permission: PermissionGrant }>("/api/permissions", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function revokePermission(id: number) {
  return apiFetch<void>(`/api/permissions/${id}`, { method: "DELETE" });
}

/** Fallback when settings API unavailable (migration 000004 seed). */
export const ROOT_FOLDER_PUBLIC_ID =
  "00000000-0000-4000-8000-000000000001";

export const SETTINGS_QUERY_KEY = ["settings"] as const;

export type SettingsEditable = {
  workspace: {
    name: string;
    public_url: string;
  };
  homepage: {
    meta_title: string;
    meta_description: string;
    keywords: string[];
    favicon_object_id: string;
    favicon_url: string;
    og_image_object_id: string;
    og_image_url: string;
    hero_eyebrow: string;
    hero_title: string;
    hero_description: string;
    hero_background_object_id: string;
    hero_background_url: string;
    features_title: string;
    features_description: string;
    cta_title: string;
    cta_description: string;
    schema_include_default: boolean;
    schema_custom: unknown[];
  };
  media: {
    default_root_folder_public_id: string;
    max_upload_bytes: number;
    delete_empty_folders_only: boolean;
  };
  security: {
    login_max_attempts: number;
    login_lockout_minutes: number;
  };
  streaming: {
    default_token_ttl_seconds: number;
    global_allowed_domains: string[];
  };
  storage: {
    quota_bytes: number;
  };
  maintenance: {
    audit_retention_days: number;
    trash_retention_days: number;
  };
};

export type SettingsReadonly = {
  infrastructure: {
    app_env: string;
    db_configured: boolean;
    redis_addr: string;
    storage_driver: string;
    storage_endpoint: string;
    storage_bucket: string;
    storage_region: string;
    storage_use_ssl: boolean;
    jwt_configured: boolean;
    setup_token_configured: boolean;
    ffmpeg_path: string;
    ffprobe_path: string;
  };
  runtime_locked: {
    require_encrypted_password: boolean;
    min_password_length: number;
  };
};

export type SettingsResponse = {
  editable: SettingsEditable;
  readonly: SettingsReadonly;
};

export type SettingsPatch = {
  workspace?: Partial<SettingsEditable["workspace"]>;
  homepage?: Partial<SettingsEditable["homepage"]>;
  media?: Partial<SettingsEditable["media"]>;
  security?: Partial<SettingsEditable["security"]>;
  streaming?: Partial<SettingsEditable["streaming"]>;
  storage?: Partial<SettingsEditable["storage"]>;
  maintenance?: Partial<SettingsEditable["maintenance"]>;
};

export function fetchSettings() {
  return apiFetch<SettingsResponse>("/api/settings");
}

export function updateSettings(patch: SettingsPatch) {
  return apiFetch<SettingsResponse>("/api/settings", {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
}

// --- Media objects / File Manager ---

export type ObjectCapabilities = {
  read: boolean;
  upload: boolean;
  update: boolean;
  delete: boolean;
  manage: boolean;
  download: boolean;
};

export type MediaObjectType = "folder" | "file" | "image" | "video";

export type BreadcrumbItem = {
  public_id: string;
  name: string;
  type: string;
  depth: number;
};

export type MediaObject = {
  public_id: string;
  parent_public_id: string | null;
  type: MediaObjectType;
  name: string;
  mime_type: string | null;
  size_bytes: number;
  created_at: string;
  updated_at: string;
  capabilities: ObjectCapabilities;
  preview_url?: string;
  download_url?: string;
  breadcrumbs?: BreadcrumbItem[];
};

export type ListObjectsResponse = {
  items: MediaObject[];
  next_cursor?: number;
};

export const OBJECTS_QUERY_KEY = "objects";

export function fetchObjects(params: {
  parent_id?: string;
  type?: string;
  q?: string;
  cursor?: string;
  limit?: string;
}) {
  const query: Record<string, string> = {};
  if (params.parent_id) query.parent_id = params.parent_id;
  if (params.type) query.type = params.type;
  if (params.q) query.q = params.q;
  if (params.cursor) query.cursor = params.cursor;
  if (params.limit) query.limit = params.limit;
  return apiFetch<ListObjectsResponse>("/api/objects", { params: query });
}

export function fetchObject(publicId: string) {
  return apiFetch<MediaObject>(`/api/objects/${publicId}`);
}

export type ObjectAccessURLs = {
  preview_url?: string;
  thumbnail_url?: string;
  download_url?: string;
};

export function fetchObjectPreviewURLs(ids: string[]) {
  return apiFetch<{ urls: Record<string, ObjectAccessURLs> }>(
    "/api/objects/preview-urls",
    {
      method: "POST",
      body: JSON.stringify({ ids }),
    },
  );
}

export function createFolder(body: { parent_id: string; name: string }) {
  return apiFetch<MediaObject>("/api/folders", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function patchObject(
  publicId: string,
  body: { name?: string; parent_id?: string },
) {
  return apiFetch<MediaObject>(`/api/objects/${publicId}`, {
    method: "PATCH",
    body: JSON.stringify(body),
  });
}

export type DeleteObjectResponse = {
  deleted_count: number;
};

export function deleteObject(publicId: string) {
  return apiFetch<DeleteObjectResponse>(`/api/objects/${publicId}`, {
    method: "DELETE",
  });
}

export function fetchTrashObjects(params: {
  type?: string;
  q?: string;
  cursor?: string;
  limit?: string;
}) {
  const query: Record<string, string> = {};
  if (params.type) query.type = params.type;
  if (params.q) query.q = params.q;
  if (params.cursor) query.cursor = params.cursor;
  if (params.limit) query.limit = params.limit;
  return apiFetch<ListObjectsResponse>("/api/objects/trash", { params: query });
}

export function fetchObjectsSearch(params: {
  type?: string;
  q: string;
  cursor?: string;
  limit?: string;
}) {
  const query: Record<string, string> = { q: params.q };
  if (params.type) query.type = params.type;
  if (params.cursor) query.cursor = params.cursor;
  if (params.limit) query.limit = params.limit;
  return apiFetch<ListObjectsResponse>("/api/objects/search", { params: query });
}

export type RestoreObjectResponse = {
  restored_count: number;
  object: MediaObject;
};

export function restoreObject(publicId: string) {
  return apiFetch<RestoreObjectResponse>(`/api/objects/${publicId}/restore`, {
    method: "POST",
  });
}

export type PurgeObjectResponse = {
  purged_count: number;
};

export function purgeObject(publicId: string) {
  return apiFetch<PurgeObjectResponse>(`/api/objects/${publicId}/purge`, {
    method: "DELETE",
  });
}

export type EmptyTrashResponse = {
  purged_count: number;
  roots_purged: number;
  roots_skipped: number;
};

export function emptyTrash() {
  return apiFetch<EmptyTrashResponse>("/api/objects/trash/empty", {
    method: "POST",
  });
}

export type BulkRenameResponse = {
  renamed_count: number;
  items: MediaObject[];
};

export function bulkRenameObjects(body: {
  object_ids: string[];
  mode: "prefix" | "suffix";
  value: string;
}) {
  return apiFetch<BulkRenameResponse>("/api/objects/bulk-rename", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export type UploadLimitsResponse = {
  max_upload_bytes: number;
};

export const UPLOAD_LIMITS_QUERY_KEY = ["upload-limits"] as const;

export function fetchUploadLimits() {
  return apiFetch<UploadLimitsResponse>("/api/upload/limits");
}

export type InitUploadResponse = {
  session_public_id: string;
  chunk_size: number;
  total_chunks: number;
  expires_at: string;
  max_upload_bytes: number;
};

export function initUpload(body: {
  parent_id: string;
  file_name: string;
  size: number;
  mime_type?: string;
  chunk_size?: number;
}) {
  return apiFetch<InitUploadResponse>("/api/upload/init", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export async function uploadChunk(
  sessionId: string,
  index: number,
  chunk: Blob,
): Promise<void> {
  const url = buildURL(`/api/upload/${sessionId}/chunks/${index}`);
  const res = await fetch(url, {
    method: "PUT",
    body: chunk,
    headers: { "Content-Type": "application/octet-stream" },
    credentials: "include",
  });
  if (!res.ok) {
    const text = await res.text();
    let data: unknown = null;
    try {
      data = JSON.parse(text);
    } catch {
      data = text;
    }
    throw new ApiError(
      (data as { message?: string })?.message ?? res.statusText,
      res.status,
      data,
    );
  }
}

export function completeUpload(sessionId: string) {
  return apiFetch<MediaObject>(`/api/upload/${sessionId}/complete`, {
    method: "POST",
  });
}

export function abortUpload(sessionId: string) {
  return apiFetch<void>(`/api/upload/${sessionId}`, { method: "DELETE" });
}

// --- Videos ---

export type HLSStatus =
  | "none"
  | "pending"
  | "converting"
  | "ready"
  | "failed"
  | "deleted";

export type VideoCapabilities = {
  read: boolean;
  convert: boolean;
  stream: boolean;
  delete: boolean;
  update: boolean;
};

export type ConvertJob = {
  public_id: string;
  status: string;
  attempts: number;
  max_attempts: number;
  error?: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
};

export type StreamPolicy = {
  access_mode: string;
  allowed_domains: string[];
  token_ttl_seconds: number;
  allow_download: boolean;
};

export type ConvertProgress = {
  stage: string;
  percent: number;
};

export type Video = {
  public_id: string;
  name: string;
  mime_type?: string;
  size_bytes: number;
  hls_status: HLSStatus;
  convert_progress?: ConvertProgress;
  duration_seconds?: number;
  width?: number;
  height?: number;
  codec?: string;
  /** Measured source bitrate (bits/s) from ffprobe */
  bitrate_bps?: number;
  last_error?: string;
  thumbnail_url?: string;
  created_at: string;
  updated_at: string;
  capabilities: VideoCapabilities;
  latest_job?: ConvertJob;
  stream_policy?: StreamPolicy;
};

export type ListVideosResponse = {
  items: Video[];
  next_cursor?: number;
};

export const VIDEOS_QUERY_KEY = "videos";

export function fetchVideos(params?: {
  hls_status?: string;
  q?: string;
  cursor?: string;
  limit?: string;
}) {
  const query: Record<string, string> = {};
  if (params?.hls_status) query.hls_status = params.hls_status;
  if (params?.q) query.q = params.q;
  if (params?.cursor) query.cursor = params.cursor;
  if (params?.limit) query.limit = params.limit;
  return apiFetch<ListVideosResponse>("/api/videos", { params: query });
}

export function fetchVideo(publicId: string) {
  return apiFetch<Video>(`/api/videos/${publicId}`);
}

export type ConvertVideoInput = {
  /** e.g. ["1080p","720p","480p"]; omit = all renditions that fit source height */
  variants?: string[];
};

export function convertVideo(publicId: string, input?: ConvertVideoInput) {
  return apiFetch<{ job: ConvertJob }>(`/api/videos/${publicId}/convert`, {
    method: "POST",
    body: input?.variants?.length ? JSON.stringify({ variants: input.variants }) : undefined,
  });
}

export function retryConvertVideo(publicId: string, input?: ConvertVideoInput) {
  return apiFetch<{ job: ConvertJob }>(`/api/videos/${publicId}/convert/retry`, {
    method: "POST",
    body: input?.variants?.length ? JSON.stringify({ variants: input.variants }) : undefined,
  });
}

export type HLSAccessResponse = {
  master_url: string;
  embed_html: string;
  expires_at: number;
};

export function fetchVideoHLS(publicId: string) {
  return apiFetch<HLSAccessResponse>(`/api/videos/${publicId}/hls`);
}

export function deleteVideoHLS(publicId: string) {
  return apiFetch<{ ok: boolean }>(`/api/videos/${publicId}/hls`, {
    method: "DELETE",
  });
}

export function fetchStreamPolicy(publicId: string) {
  return apiFetch<StreamPolicy>(`/api/videos/${publicId}/stream-policy`);
}

export function patchStreamPolicy(
  publicId: string,
  body: Partial<StreamPolicy>,
) {
  return apiFetch<StreamPolicy>(`/api/videos/${publicId}/stream-policy`, {
    method: "PATCH",
    body: JSON.stringify(body),
  });
}

// --- API keys (owner) ---

export type APIKey = {
  public_id: string;
  name: string;
  scopes: string[];
  allowed_ips: string[];
  root_folder_public_id?: string;
  status: string;
  last_used_at?: string;
  created_at: string;
};

export const INTEGRATION_API_KEY_SCOPES = [
  "stream",
  "media:upload",
  "media:read",
  "media:convert",
  "media:delete",
] as const;

export type IntegrationAPIKeyScope = (typeof INTEGRATION_API_KEY_SCOPES)[number];

export const API_KEY_SCOPE_LABELS: Record<IntegrationAPIKeyScope, string> = {
  stream: "Stream HLS (/stream)",
  "media:upload": "Upload media",
  "media:read": "Đọc metadata & delivery URLs",
  "media:convert": "Chuyển mã video HLS",
  "media:delete": "Xóa media",
};

export function fetchAPIKeys() {
  return apiFetch<{ items: APIKey[] }>("/api/api-keys");
}

export function createAPIKey(body: {
  name: string;
  scopes?: string[];
  allowed_ips?: string[];
  root_folder_public_id?: string;
}) {
  return apiFetch<{ key: APIKey; secret: string }>("/api/api-keys", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function patchAPIKey(
  publicId: string,
  body: {
    name?: string;
    scopes?: string[];
    allowed_ips?: string[];
    root_folder_public_id?: string;
    clear_root_folder?: boolean;
    status?: string;
  },
) {
  return apiFetch<APIKey>(`/api/api-keys/${publicId}`, {
    method: "PATCH",
    body: JSON.stringify(body),
  });
}

export function revokeAPIKey(publicId: string) {
  return apiFetch<{ ok: boolean }>(`/api/api-keys/${publicId}`, {
    method: "DELETE",
  });
}

export type QueueStatusResponse = {
  convert_jobs: Record<string, number>;
  video_hls: Record<string, number>;
  queue_depth: number;
  failed_jobs?: Array<{
    job_public_id: string;
    video_public_id: string;
    video_name: string;
    attempts: number;
    max_attempts: number;
    error?: string;
    finished_at?: string;
  }>;
  media_objects?: {
    total: number;
    folders: number;
    files: number;
  };
};

export function fetchQueueStatus() {
  return apiFetch<QueueStatusResponse>("/api/system/queue");
}

export type SystemStorageResponse = {
  status: string;
  details?: Record<string, string>;
  error?: string;
  driver?: string;
  bucket?: string;
  quota_bytes?: number;
};

export function fetchSystemStorage() {
  return apiFetch<SystemStorageResponse>("/api/system/storage");
}

export type SystemSecurityResponse = {
  login_max_attempts: number;
  login_lockout_window_sec: number;
  redis_fail_closed: boolean;
  stream_rate_limit_per_min: number;
  active_api_keys: number;
  global_stream_domains: string[];
  app_env?: string;
  password_transport: { require_encrypted: boolean };
  cookie_secure: boolean;
};

export function fetchSystemSecurity() {
  return apiFetch<SystemSecurityResponse>("/api/system/security");
}

export function fetchStreamAnalytics() {
  return apiFetch<Record<string, unknown>>("/api/system/stream-analytics");
}

export function fetchComponentHealth(component: string) {
  return apiFetch<{ component: string; health: ComponentHealth }>(
    `/api/system/health/${component}`,
  );
}

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anhtuanlc/mediahub/internal/authz"
	convertprogress "github.com/anhtuanlc/mediahub/internal/platform/convert"
	"github.com/anhtuanlc/mediahub/internal/platform/rediscache"
	"github.com/anhtuanlc/mediahub/internal/platform/resource"
	"github.com/anhtuanlc/mediahub/internal/platform/webhook"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/anhtuanlc/mediahub/internal/transcode"
	"github.com/google/uuid"
)

var (
	ErrVideoAccessDenied    = errors.New("access denied")
	ErrVideoNotFound        = errors.New("video not found")
	ErrVideoInvalidState    = errors.New("invalid hls status for operation")
	ErrVideoConvertActive   = errors.New("convert already in progress")
	ErrVideoConvertNotActive = errors.New("no active convert job")
	ErrConvertJobNotDismissible = errors.New("convert job cannot be dismissed")
	ErrVideoNotStreamable   = errors.New("video is not ready for streaming")
	ErrVideoInvalidVariants = errors.New("invalid convert variants")
	ErrVideoRetryNotAllowed = errors.New("convert retry not allowed")
	ErrSystemBusy           = errors.New("system busy")
)

// StartConvertInput is optional body for POST /videos/:id/convert.
type StartConvertInput struct {
	Variants []string `json:"variants"`
}

type VideoCapabilities struct {
	Read     bool `json:"read"`
	Convert  bool `json:"convert"`
	Stream   bool `json:"stream"`
	Delete   bool `json:"delete"`
	Update   bool `json:"update"`
}

type ConvertJobDTO struct {
	PublicID    string     `json:"public_id"`
	Status      string     `json:"status"`
	Attempts    int        `json:"attempts"`
	MaxAttempts int        `json:"max_attempts"`
	Error       *string    `json:"error,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type StreamPolicyDTO struct {
	AccessMode      string   `json:"access_mode"`
	AllowedDomains  []string `json:"allowed_domains"`
	TokenTTLSeconds int      `json:"token_ttl_seconds"`
	AllowDownload   bool     `json:"allow_download"`
}

type VideoDTO struct {
	PublicID        string            `json:"public_id"`
	Name            string            `json:"name"`
	MimeType        *string           `json:"mime_type,omitempty"`
	SizeBytes       int64             `json:"size_bytes"`
	HLSStatus       string            `json:"hls_status"`
	DurationSeconds *int              `json:"duration_seconds,omitempty"`
	Width           *int              `json:"width,omitempty"`
	Height          *int              `json:"height,omitempty"`
	Codec           *string           `json:"codec,omitempty"`
	BitrateBps      *int64            `json:"bitrate_bps,omitempty"`
	LastError       *string           `json:"last_error,omitempty"`
	ThumbnailURL    *string           `json:"thumbnail_url,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	Capabilities    VideoCapabilities `json:"capabilities"`
	LatestJob       *ConvertJobDTO       `json:"latest_job,omitempty"`
	StreamPolicy    *StreamPolicyDTO     `json:"stream_policy,omitempty"`
	ConvertProgress *ConvertProgressDTO  `json:"convert_progress,omitempty"`
}

type ConvertProgressDTO struct {
	Stage   string `json:"stage"`
	Percent int    `json:"percent"`
}

type HLSAccessDTO struct {
	MasterURL   string `json:"master_url"`
	EmbedHTML   string `json:"embed_html"`
	ExpiresAt   int64  `json:"expires_at"`
}

type VideoService struct {
	videos    *repository.VideoRepository
	objects   *repository.MediaObjectRepository
	authz     *authz.Service
	audit     *repository.AuditRepository
	settings  *SettingsService
	store     storage.ObjectStorage
	enqueue   *ConvertEnqueue
	streamTok       *StreamTokenService
	streamBaseURL   string
	convertProgress *convertprogress.ProgressStore
	cache           *rediscache.Store
	resources       *resource.Reader
	webhooks        *webhook.Dispatcher
}

func NewVideoService(
	videos *repository.VideoRepository,
	objects *repository.MediaObjectRepository,
	authzSvc *authz.Service,
	audit *repository.AuditRepository,
	settings *SettingsService,
	store storage.ObjectStorage,
	enqueue *ConvertEnqueue,
	streamTok *StreamTokenService,
	streamBaseURL string,
	convertProgress *convertprogress.ProgressStore,
	cache *rediscache.Store,
	resources *resource.Reader,
) *VideoService {
	return &VideoService{
		videos:          videos,
		objects:         objects,
		authz:           authzSvc,
		audit:           audit,
		settings:        settings,
		store:           store,
		enqueue:         enqueue,
		streamTok:       streamTok,
		streamBaseURL:   streamBaseURL,
		convertProgress: convertProgress,
		cache:           cache,
		resources:       resources,
	}
}

func (s *VideoService) SetWebhooks(d *webhook.Dispatcher) {
	s.webhooks = d
}

func (s *VideoService) cleanupHLSPrefix(ctx context.Context, videoPublicID uuid.UUID) {
	if s3, ok := s.store.(*storage.S3Storage); ok {
		_ = s3.DeletePrefix(ctx, storage.HLSPrefix(videoPublicID))
	}
}

func (s *VideoService) invalidateStreamCache(ctx context.Context, publicID uuid.UUID) {
	rediscache.InvalidateStreamVideo(ctx, s.cache, publicID)
}

func (s *VideoService) attachConvertProgress(ctx context.Context, dto *VideoDTO) {
	if s.convertProgress == nil || dto == nil {
		return
	}
	if dto.HLSStatus != "pending" && dto.HLSStatus != "converting" {
		return
	}
	p, err := s.convertProgress.Get(ctx, dto.PublicID)
	if err != nil || p == nil {
		if dto.HLSStatus == "pending" {
			dto.ConvertProgress = &ConvertProgressDTO{Stage: "queued", Percent: 5}
		}
		return
	}
	dto.ConvertProgress = &ConvertProgressDTO{Stage: p.Stage, Percent: p.Percent}
}

type ListVideosInput struct {
	HLSStatus []string
	Query     string
	Cursor    int64
	Limit     int
}

func (s *VideoService) videoCapabilities(ctx context.Context, userID int64, role string, objectID int64) VideoCapabilities {
	if authz.IsOwnerRole(role) {
		return VideoCapabilities{Read: true, Convert: true, Stream: true, Delete: true, Update: true}
	}
	cap := VideoCapabilities{}
	checks := []struct {
		action authz.Action
		dest   *bool
	}{
		{authz.ActionRead, &cap.Read},
		{authz.ActionConvert, &cap.Convert},
		{authz.ActionStream, &cap.Stream},
		{authz.ActionDelete, &cap.Delete},
		{authz.ActionUpdate, &cap.Update},
	}
	for _, c := range checks {
		ok, _ := s.authz.HasPermission(ctx, userID, role, objectID, c.action)
		*c.dest = ok
	}
	return cap
}

func videoCapsFromAuthz(c authz.Capabilities) VideoCapabilities {
	return VideoCapabilities{
		Read:    c.Read,
		Convert: c.Convert,
		Stream:  c.Stream,
		Delete:  c.Delete,
		Update:  c.Update,
	}
}

func jobToDTO(j *repository.ConvertJob) *ConvertJobDTO {
	if j == nil {
		return nil
	}
	return &ConvertJobDTO{
		PublicID:    j.PublicID.String(),
		Status:      j.Status,
		Attempts:    j.Attempts,
		MaxAttempts: j.MaxAttempts,
		Error:       j.Error,
		StartedAt:   j.StartedAt,
		FinishedAt:  j.FinishedAt,
		CreatedAt:   j.CreatedAt,
	}
}

func policyToDTO(p *repository.StreamPolicy) *StreamPolicyDTO {
	if p == nil {
		return nil
	}
	return &StreamPolicyDTO{
		AccessMode:      p.AccessMode,
		AllowedDomains:  p.AllowedDomains,
		TokenTTLSeconds: p.TokenTTLSeconds,
		AllowDownload:   p.AllowDownload,
	}
}

func (s *VideoService) rowToDTO(ctx context.Context, userID int64, role string, row *repository.VideoRow, includePolicy bool, precomputed *VideoCapabilities) (*VideoDTO, error) {
	m := &row.Media
	a := &row.Asset
	var caps VideoCapabilities
	if precomputed != nil {
		caps = *precomputed
	} else {
		caps = s.videoCapabilities(ctx, userID, role, m.ID)
	}
	dto := &VideoDTO{
		PublicID:        m.PublicID.String(),
		Name:            m.Name,
		MimeType:        m.MimeType,
		SizeBytes:       m.SizeBytes,
		HLSStatus:       a.HLSStatus,
		DurationSeconds: a.DurationSeconds,
		Width:           a.Width,
		Height:          a.Height,
		Codec:           a.Codec,
		BitrateBps:      a.Bitrate,
		LastError:       a.LastError,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
		Capabilities:    caps,
	}
	thumbKey := a.ThumbnailKey
	if thumbKey == nil || *thumbKey == "" {
		thumbKey = m.ThumbnailKey
	}
	if thumbKey != nil && *thumbKey != "" && caps.Read {
		if u, err := s.store.PresignGetObject(ctx, *thumbKey, 15*time.Minute); err == nil {
			dto.ThumbnailURL = &u
		}
	}
	job, _ := s.videos.LatestJobForAsset(ctx, a.ID)
	dto.LatestJob = jobToDTO(job)
	if includePolicy && caps.Read {
		settings, _ := s.settings.Get(ctx)
		ttl := 3600
		if settings != nil && settings.Editable.Streaming.DefaultTokenTTLSeconds > 0 {
			ttl = settings.Editable.Streaming.DefaultTokenTTLSeconds
		}
		pol, _ := s.videos.GetOrCreateStreamPolicy(ctx, a.ID, ttl)
		dto.StreamPolicy = policyToDTO(pol)
	}
	s.attachConvertProgress(ctx, dto)
	return dto, nil
}

func (s *VideoService) List(ctx context.Context, userID int64, role string, in ListVideosInput) ([]VideoDTO, *int64, error) {
	limit := in.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.videos.ListVideos(ctx, repository.VideoListFilter{
		HLSStatus: in.HLSStatus,
		Query:     in.Query,
		Cursor:    in.Cursor,
		Limit:     limit + 1,
	})
	if err != nil {
		return nil, nil, err
	}
	var next *int64
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1].Media.ID
		next = &last
	}
	if !authz.IsOwnerRole(role) {
		ids := make([]int64, len(rows))
		for i := range rows {
			ids[i] = rows[i].Media.ID
		}
		visible, err := s.authz.VisibleForListing(ctx, userID, role, ids)
		if err != nil {
			return nil, nil, err
		}
		filtered := rows[:0]
		for _, r := range rows {
			if _, ok := visible[r.Media.ID]; ok {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
		if len(rows) == 0 {
			return nil, next, nil
		}
		filteredIDs := make([]int64, len(rows))
		for i := range rows {
			filteredIDs[i] = rows[i].Media.ID
		}
		capsByID, err := s.authz.CapabilitiesMap(ctx, userID, role, filteredIDs)
		if err != nil {
			return nil, nil, err
		}
		out := make([]VideoDTO, 0, len(rows))
		for i := range rows {
			caps := videoCapsFromAuthz(capsByID[rows[i].Media.ID])
			if !caps.Read {
				continue
			}
			dto, err := s.rowToDTO(ctx, userID, role, &rows[i], false, &caps)
			if err != nil {
				return nil, nil, err
			}
			out = append(out, *dto)
		}
		return out, next, nil
	}
	out := make([]VideoDTO, 0, len(rows))
	for i := range rows {
		dto, err := s.rowToDTO(ctx, userID, role, &rows[i], false, nil)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, *dto)
	}
	return out, next, nil
}

func (s *VideoService) Get(ctx context.Context, userID int64, role string, publicID uuid.UUID) (*VideoDTO, error) {
	row, err := s.videos.GetByObjectPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, repository.ErrVideoAssetNotFound) {
			return nil, ErrVideoNotFound
		}
		return nil, err
	}
	caps := s.videoCapabilities(ctx, userID, role, row.Media.ID)
	if !caps.Read {
		return nil, ErrVideoAccessDenied
	}
	return s.rowToDTO(ctx, userID, role, row, true, nil)
}

func (s *VideoService) StartConvert(ctx context.Context, userID int64, role, ip, ua string, publicID uuid.UUID, input StartConvertInput) (*ConvertJobDTO, error) {
	row, err := s.videos.GetByObjectPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, repository.ErrVideoAssetNotFound) {
			return nil, ErrVideoNotFound
		}
		return nil, err
	}
	if !s.videoCapabilities(ctx, userID, role, row.Media.ID).Convert {
		return nil, ErrVideoAccessDenied
	}
	switch row.Asset.HLSStatus {
	case "none", "failed", "deleted":
	default:
		return nil, ErrVideoInvalidState
	}
	if row.Media.StorageKey == nil || *row.Media.StorageKey == "" {
		return nil, fmt.Errorf("%w: missing source file", ErrVideoInvalidState)
	}
	active, err := s.videos.GetActiveJobForAsset(ctx, row.Asset.ID)
	if err != nil {
		return nil, err
	}
	if active != nil {
		return nil, ErrVideoConvertActive
	}
	if s.resources != nil && s.resources.Enabled {
		if _, lim, err := s.resources.Current(ctx); err == nil {
			if lim.SystemBusy || lim.DeferConvert {
				return nil, ErrSystemBusy
			}
		}
	}
	if s.enqueue != nil && s.enqueue.MaxDepth() > 0 {
		depth, err := s.enqueue.PendingDepth(ctx)
		if err != nil {
			return nil, err
		}
		if depth >= s.enqueue.MaxDepth() {
			return nil, ErrConvertQueueFull
		}
	}
	if s.enqueue != nil {
		paused, err := s.enqueue.IsQueuePaused(ctx)
		if err != nil {
			return nil, err
		}
		if paused {
			return nil, ErrConvertQueuePaused
		}
	}
	src := transcode.SourceProfile{}
	if row.Asset.Height != nil {
		src.Height = *row.Asset.Height
	}
	if row.Asset.Bitrate != nil {
		src.Bitrate = *row.Asset.Bitrate
	}
	if _, err := transcode.ResolveVariants(input.Variants, src); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrVideoInvalidVariants, err.Error())
	}
	jobPID := uuid.New()
	prevHLS := row.Asset.HLSStatus
	tx, err := s.videos.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, `
		INSERT INTO convert_jobs (public_id, video_asset_id, status, created_by)
		VALUES ($1, $2, 'pending', $3)
	`, jobPID, row.Asset.ID, userID)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE video_assets SET hls_status = 'pending', updated_at = now() WHERE id = $1`, row.Asset.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if s.convertProgress != nil {
		_ = s.convertProgress.Set(ctx, publicID.String(), "queued", 8)
	}
	maxAttempts := defaultConvertMaxAttempts
	if err := s.enqueue.EnqueueConvert(ctx, jobPID, publicID, row.Media.ID, input.Variants, maxAttempts); err != nil {
		msg := err.Error()
		if j, jerr := s.videos.GetConvertJobByPublicID(ctx, jobPID); jerr == nil {
			_ = s.videos.UpdateJobStatus(ctx, j.ID, "failed", &msg)
		}
		_ = s.videos.UpdateHLSStatus(ctx, row.Asset.ID, prevHLS)
		if s.convertProgress != nil {
			_ = s.convertProgress.Clear(ctx, publicID.String())
		}
		return nil, fmt.Errorf("enqueue convert: %w", err)
	}
	actorID := userID
	auditMeta := map[string]string{"job_public_id": jobPID.String()}
	if len(input.Variants) > 0 {
		auditMeta["variants"] = strings.Join(input.Variants, ",")
	}
	_ = s.audit.Log(ctx, &actorID, "video.convert", "video", &row.Media.ID, ip, ua, auditMeta)
	j, err := s.videos.GetConvertJobByPublicID(ctx, jobPID)
	if err != nil {
		return jobToDTO(&repository.ConvertJob{PublicID: jobPID, Status: "pending", CreatedAt: time.Now()}), nil
	}
	if s.webhooks != nil {
		s.webhooks.Emit(ctx, webhook.EventConvertStarted, map[string]any{
			"public_id": publicID.String(),
			"job_id":    jobPID.String(),
		})
	}
	return jobToDTO(j), nil
}

// RetryConvert starts a new convert job when the latest attempt failed.
func (s *VideoService) RetryConvert(ctx context.Context, userID int64, role, ip, ua string, publicID uuid.UUID, input StartConvertInput) (*ConvertJobDTO, error) {
	row, err := s.videos.GetByObjectPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, repository.ErrVideoAssetNotFound) {
			return nil, ErrVideoNotFound
		}
		return nil, err
	}
	if !s.videoCapabilities(ctx, userID, role, row.Media.ID).Convert {
		return nil, ErrVideoAccessDenied
	}
	if row.Asset.HLSStatus != "failed" {
		return nil, ErrVideoRetryNotAllowed
	}
	active, err := s.videos.GetActiveJobForAsset(ctx, row.Asset.ID)
	if err != nil {
		return nil, err
	}
	if active != nil {
		return nil, ErrVideoConvertActive
	}
	return s.StartConvert(ctx, userID, role, ip, ua, publicID, input)
}

// FailStaleRunningJobs marks long-running convert jobs as failed and cleans partial HLS output.
func (s *VideoService) FailStaleRunningJobs(ctx context.Context, olderThan time.Duration) (int, error) {
	stale, err := s.videos.FailStaleRunningJobs(ctx, olderThan)
	if err != nil {
		return 0, err
	}
	for _, row := range stale {
		s.cleanupHLSPrefix(ctx, row.VideoPublicID)
		if s.convertProgress != nil {
			_ = s.convertProgress.Clear(ctx, row.VideoPublicID.String())
			_ = s.convertProgress.ClearCancel(ctx, row.JobPublicID.String())
		}
		s.invalidateStreamCache(ctx, row.VideoPublicID)
	}
	return len(stale), nil
}

// CancelConvert cancels the active convert job for a video.
func (s *VideoService) CancelConvert(ctx context.Context, userID int64, role, ip, ua string, publicID uuid.UUID) error {
	row, err := s.videos.GetByObjectPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, repository.ErrVideoAssetNotFound) {
			return ErrVideoNotFound
		}
		return err
	}
	if !s.videoCapabilities(ctx, userID, role, row.Media.ID).Convert {
		return ErrVideoAccessDenied
	}
	active, err := s.videos.GetActiveJobForAsset(ctx, row.Asset.ID)
	if err != nil {
		return err
	}
	if active == nil {
		return ErrVideoConvertNotActive
	}

	cancelMsg := "cancelled by user"
	if err := s.videos.UpdateJobStatus(ctx, active.ID, "cancelled", &cancelMsg); err != nil {
		return err
	}

	switch active.Status {
	case "pending":
		if s.enqueue != nil {
			_ = s.enqueue.DeletePendingTaskByJobID(ctx, active.PublicID)
		}
		_ = s.videos.UpdateHLSStatus(ctx, row.Asset.ID, "none")
	case "running":
		if s.convertProgress != nil {
			_ = s.convertProgress.RequestCancel(ctx, active.PublicID.String())
		}
		_ = s.videos.UpdateHLSStatus(ctx, row.Asset.ID, "failed")
		s.cleanupHLSPrefix(ctx, publicID)
	}

	if s.convertProgress != nil {
		_ = s.convertProgress.Clear(ctx, publicID.String())
	}
	s.invalidateStreamCache(ctx, publicID)

	actorID := userID
	meta := map[string]string{"job_public_id": active.PublicID.String()}
	_ = s.audit.Log(ctx, &actorID, "video.convert.cancel", "video", &row.Media.ID, ip, ua, meta)
	return nil
}

// DeleteFailedConvertJob removes a failed or cancelled job record from history (Owner/system).
func (s *VideoService) DeleteFailedConvertJob(ctx context.Context, userID int64, role, ip, ua string, jobPublicID uuid.UUID) error {
	if role != "owner" {
		return ErrVideoAccessDenied
	}
	job, err := s.videos.GetConvertJobByPublicID(ctx, jobPublicID)
	if err != nil {
		if errors.Is(err, repository.ErrConvertJobNotFound) {
			return ErrConvertJobNotDismissible
		}
		return err
	}
	if job.Status != "failed" && job.Status != "cancelled" {
		return ErrConvertJobNotDismissible
	}
	row, err := s.videos.GetVideoRowByAssetID(ctx, job.VideoAssetID)
	if err != nil {
		return err
	}
	if err := s.videos.DeleteTerminalJobByPublicID(ctx, jobPublicID); err != nil {
		return err
	}
	actorID := userID
	meta := map[string]string{
		"job_public_id":   jobPublicID.String(),
		"video_public_id": row.Media.PublicID.String(),
		"status":          job.Status,
	}
	_ = s.audit.Log(ctx, &actorID, "system.convert_job.delete", "convert_job", &row.Media.ID, ip, ua, meta)
	return nil
}

func (s *VideoService) GetHLSAccess(ctx context.Context, userID int64, role string, publicID uuid.UUID) (*HLSAccessDTO, error) {
	row, err := s.videos.GetByObjectPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, repository.ErrVideoAssetNotFound) {
			return nil, ErrVideoNotFound
		}
		return nil, err
	}
	if !s.videoCapabilities(ctx, userID, role, row.Media.ID).Stream {
		return nil, ErrVideoAccessDenied
	}
	if row.Asset.HLSStatus != "ready" {
		return nil, ErrVideoNotStreamable
	}
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	ttlSec := 3600
	if settings != nil && settings.Editable.Streaming.DefaultTokenTTLSeconds > 0 {
		ttlSec = settings.Editable.Streaming.DefaultTokenTTLSeconds
	}
	pol, err := s.videos.GetOrCreateStreamPolicy(ctx, row.Asset.ID, ttlSec)
	if err != nil {
		return nil, err
	}
	ttl := time.Duration(pol.TokenTTLSeconds) * time.Second
	exp := time.Now().Add(ttl)
	masterURL := s.streamTok.BuildStreamURL(s.streamBaseURL, publicID.String(), "master.m3u8", ttl)
	embedHTML := fmt.Sprintf(
		`<iframe src="%s" allow="autoplay; encrypted-media; fullscreen" allowfullscreen style="width:100%%;aspect-ratio:16/9;border:0"></iframe>`,
		s.streamTok.BuildEmbedURL(s.streamBaseURL, publicID.String(), ttl),
	)
	return &HLSAccessDTO{
		MasterURL: masterURL,
		EmbedHTML: embedHTML,
		ExpiresAt: exp.Unix(),
	}, nil
}

func (s *VideoService) DeleteHLS(ctx context.Context, userID int64, role, ip, ua string, publicID uuid.UUID, deletePrefix func(context.Context, string) error) error {
	row, err := s.videos.GetByObjectPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, repository.ErrVideoAssetNotFound) {
			return ErrVideoNotFound
		}
		return err
	}
	caps := s.videoCapabilities(ctx, userID, role, row.Media.ID)
	if !caps.Delete && !caps.Update {
		return ErrVideoAccessDenied
	}
	prefix := storage.HLSPrefix(publicID)
	if deletePrefix != nil {
		if err := deletePrefix(ctx, prefix); err != nil {
			return err
		}
	}
	if err := s.videos.ClearHLS(ctx, row.Asset.ID); err != nil {
		return err
	}
	actorID := userID
	_ = s.audit.Log(ctx, &actorID, "video.hls_delete", "video", &row.Media.ID, ip, ua, nil)
	s.invalidateStreamCache(ctx, publicID)
	return nil
}

type UpdateStreamPolicyInput struct {
	AccessMode      *string
	AllowedDomains  *[]string // nil = không đổi; trỏ tới [] = xóa allowlist
	TokenTTLSeconds *int
	AllowDownload   *bool
}

func (s *VideoService) GetStreamPolicy(ctx context.Context, userID int64, role string, publicID uuid.UUID) (*StreamPolicyDTO, error) {
	row, err := s.videos.GetByObjectPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, repository.ErrVideoAssetNotFound) {
			return nil, ErrVideoNotFound
		}
		return nil, err
	}
	if !s.videoCapabilities(ctx, userID, role, row.Media.ID).Read {
		return nil, ErrVideoAccessDenied
	}
	settings, _ := s.settings.Get(ctx)
	ttl := 3600
	if settings != nil {
		ttl = settings.Editable.Streaming.DefaultTokenTTLSeconds
	}
	pol, err := s.videos.GetOrCreateStreamPolicy(ctx, row.Asset.ID, ttl)
	if err != nil {
		return nil, err
	}
	return policyToDTO(pol), nil
}

func (s *VideoService) UpdateStreamPolicy(ctx context.Context, userID int64, role, ip, ua string, publicID uuid.UUID, in UpdateStreamPolicyInput) (*StreamPolicyDTO, error) {
	row, err := s.videos.GetByObjectPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, repository.ErrVideoAssetNotFound) {
			return nil, ErrVideoNotFound
		}
		return nil, err
	}
	if !authz.IsOwnerRole(role) && !s.videoCapabilities(ctx, userID, role, row.Media.ID).Update {
		return nil, ErrVideoAccessDenied
	}
	settings, _ := s.settings.Get(ctx)
	ttl := 3600
	if settings != nil {
		ttl = settings.Editable.Streaming.DefaultTokenTTLSeconds
	}
	_, _ = s.videos.GetOrCreateStreamPolicy(ctx, row.Asset.ID, ttl)
	pol, err := s.videos.UpdateStreamPolicy(ctx, row.Asset.ID, repository.StreamPolicyUpdate{
		AccessMode:      in.AccessMode,
		AllowedDomains:  in.AllowedDomains,
		TokenTTLSeconds: in.TokenTTLSeconds,
		AllowDownload:   in.AllowDownload,
	})
	if err != nil {
		return nil, err
	}
	actorID := userID
	_ = s.audit.Log(ctx, &actorID, "video.stream_policy_update", "video", &row.Media.ID, ip, ua, nil)
	s.invalidateStreamCache(ctx, publicID)
	return policyToDTO(pol), nil
}

func (s *VideoService) Stats(ctx context.Context) (map[string]int64, error) {
	return s.videos.CountByHLSStatus(ctx)
}

// HLSStatus returns the HLS lifecycle state for a video object.
func (s *VideoService) HLSStatus(ctx context.Context, publicID uuid.UUID) (string, error) {
	row, err := s.videos.GetByObjectPublicID(ctx, publicID)
	if err != nil {
		return "", err
	}
	return row.Asset.HLSStatus, nil
}

// AllowSourceDownload reports whether the original file may be delivered via /assets/.../file.
// Non-video objects are always allowed; videos follow stream_policies.allow_download.
func (s *VideoService) AllowSourceDownload(ctx context.Context, m *repository.MediaObject) (bool, error) {
	if m == nil || m.Type != "video" {
		return true, nil
	}
	row, err := s.videos.GetByObjectPublicID(ctx, m.PublicID)
	if err != nil {
		return false, err
	}
	pol, err := s.videos.GetOrCreateStreamPolicy(ctx, row.Asset.ID, 3600)
	if err != nil {
		return false, err
	}
	return pol.AllowDownload, nil
}

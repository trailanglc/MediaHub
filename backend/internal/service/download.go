package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/anhtuanlc/mediahub/internal/download"
	downloadprog "github.com/anhtuanlc/mediahub/internal/platform/download"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrDownloadNotFound   = errors.New("download job not found")
	ErrDownloadForbidden  = errors.New("download forbidden")
	ErrDownloadBadRequest = errors.New("download bad request")
	ErrDownloadNoMedia    = errors.New("no media candidates found")
)

type DownloadService struct {
	jobs      *repository.DownloadJobRepository
	media     *MediaObjectService
	settings  *SettingsService
	enqueue   *DownloadEnqueue
	progress  *downloadprog.ProgressStore
	ytdlpPath string
	analyzeTO time.Duration
	audit     *repository.AuditRepository
}

func NewDownloadService(
	jobs *repository.DownloadJobRepository,
	media *MediaObjectService,
	settings *SettingsService,
	enqueue *DownloadEnqueue,
	progress *downloadprog.ProgressStore,
	ytdlpPath string,
	analyzeTO time.Duration,
	audit *repository.AuditRepository,
) *DownloadService {
	return &DownloadService{
		jobs:      jobs,
		media:     media,
		settings:  settings,
		enqueue:   enqueue,
		progress:  progress,
		ytdlpPath: ytdlpPath,
		analyzeTO: analyzeTO,
		audit:     audit,
	}
}

type AnalyzeResult struct {
	Kind        string                `json:"kind"` // direct | hls | website
	FinalURL    string                `json:"final_url,omitempty"`
	Filename    string                `json:"filename,omitempty"`
	ContentType string                `json:"content_type,omitempty"`
	Title       string                `json:"title,omitempty"`
	Candidates  []download.Candidate  `json:"candidates,omitempty"`
	Source      string                `json:"source,omitempty"` // ytdlp | html | path
}

type CreateDownloadJobInput struct {
	URL            string          `json:"url"`
	ResolvedURL    string          `json:"resolved_url"`
	Kind           string          `json:"kind"`
	FormatID       string          `json:"format_id"`
	Title          string          `json:"title"`
	ThumbnailURL   string          `json:"thumbnail_url"`
	IsHLS          bool            `json:"is_hls"`
	Ext            string          `json:"ext"`
	ParentPublicID *uuid.UUID      `json:"parent_public_id"`
	Candidate      json.RawMessage `json:"candidate"`
}

type DownloadJobDTO struct {
	PublicID      string  `json:"public_id"`
	SourceURL     string  `json:"source_url"`
	ResolvedURL   *string `json:"resolved_url,omitempty"`
	Kind          string  `json:"kind"`
	Status        string  `json:"status"`
	Title         *string `json:"title,omitempty"`
	ThumbnailURL  *string `json:"thumbnail_url,omitempty"`
	ProgressPct   int     `json:"progress_pct"`
	ProgressStage string  `json:"progress_stage,omitempty"`
	BytesDone     int64   `json:"bytes_done"`
	BytesTotal    int64   `json:"bytes_total"`
	VideoPublicID *string `json:"video_public_id,omitempty"`
	LastError     *string `json:"last_error,omitempty"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

func (s *DownloadService) analyzeTimeout(ctx context.Context) time.Duration {
	if s.settings != nil {
		if lim, err := s.settings.DownloadLimits(ctx); err == nil && lim.AnalyzeTimeoutSeconds > 0 {
			return time.Duration(lim.AnalyzeTimeoutSeconds) * time.Second
		}
	}
	if s.analyzeTO > 0 {
		return s.analyzeTO
	}
	return 60 * time.Second
}

func (s *DownloadService) Analyze(ctx context.Context, rawURL string) (*AnalyzeResult, error) {
	analyzeTO := s.analyzeTimeout(ctx)
	classified, err := download.Classify(ctx, rawURL, analyzeTO)
	if err != nil {
		if errors.Is(err, download.ErrBlockedURL) {
			return nil, fmt.Errorf("%w: %v", ErrDownloadBadRequest, err)
		}
		return nil, err
	}

	out := &AnalyzeResult{
		Kind:        classified.Kind,
		FinalURL:    classified.FinalURL,
		Filename:    classified.Filename,
		ContentType: classified.ContentType,
		Source:      "path",
	}
	if classified.Kind == download.KindDirect || classified.Kind == download.KindHLS {
		return out, nil
	}

	// Website: yt-dlp then HTML fallback.
	cands, title, err := download.AnalyzeYTDLP(ctx, s.ytdlpPath, rawURL, analyzeTO)
	if err == nil && len(cands) > 0 {
		out.Kind = download.KindWebsite
		out.Candidates = cands
		out.Title = title
		out.Source = download.KindYTDLP
		return out, nil
	}

	htmlCands, htmlTitle, htmlErr := download.AnalyzeHTML(ctx, rawURL, analyzeTO)
	if htmlErr != nil && err != nil {
		return nil, fmt.Errorf("%w: yt-dlp: %v; html: %v", ErrDownloadNoMedia, err, htmlErr)
	}
	if len(htmlCands) == 0 {
		msg := "no candidates"
		if err != nil {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%w: %s", ErrDownloadNoMedia, msg)
	}
	out.Kind = download.KindWebsite
	out.Candidates = htmlCands
	out.Title = htmlTitle
	out.Source = download.KindHTML
	return out, nil
}

func (s *DownloadService) CreateJob(ctx context.Context, userID int64, role, ip, ua string, in CreateDownloadJobInput) (*DownloadJobDTO, error) {
	rawURL := in.URL
	if rawURL == "" {
		return nil, fmt.Errorf("%w: url required", ErrDownloadBadRequest)
	}
	u, err := download.ValidateURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDownloadBadRequest, err)
	}
	if err := download.ResolveAndCheck(ctx, u); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDownloadBadRequest, err)
	}

	parent, err := s.resolveParent(ctx, in.ParentPublicID)
	if err != nil {
		return nil, err
	}
	if err := s.media.CheckUploadPerm(ctx, userID, role, parent.ID); err != nil {
		if errors.Is(err, ErrMediaAccessDenied) {
			return nil, ErrDownloadForbidden
		}
		return nil, err
	}

	kind := in.Kind
	resolved := in.ResolvedURL
	title := in.Title
	thumb := in.ThumbnailURL
	sel := map[string]any{}
	if in.FormatID != "" {
		sel["format_id"] = in.FormatID
	}
	if in.IsHLS {
		sel["is_hls"] = true
	}
	if in.Ext != "" {
		sel["ext"] = in.Ext
	}
	if resolved != "" {
		sel["url"] = resolved
	}
	if len(in.Candidate) > 0 {
		var c download.Candidate
		if json.Unmarshal(in.Candidate, &c) == nil {
			if c.URL != "" {
				resolved = c.URL
				sel["url"] = c.URL
			}
			if c.ID != "" {
				sel["format_id"] = c.ID
			}
			sel["is_hls"] = c.IsHLS
			if c.Ext != "" {
				sel["ext"] = c.Ext
			}
			if title == "" {
				title = c.Title
			}
			if thumb == "" {
				thumb = c.Thumbnail
			}
			if c.IsHLS {
				kind = download.KindHLS
			} else if kind == "" || kind == download.KindWebsite {
				kind = download.KindYTDLP
			}
		}
	}

	if kind == "" || kind == download.KindWebsite {
		classified, err := download.Classify(ctx, rawURL, s.analyzeTO)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrDownloadBadRequest, err)
		}
		switch classified.Kind {
		case download.KindHLS:
			kind = download.KindHLS
			if resolved == "" {
				resolved = classified.FinalURL
			}
		case download.KindDirect:
			kind = download.KindDirect
			if resolved == "" {
				resolved = classified.FinalURL
			}
		default:
			kind = download.KindYTDLP
		}
	}

	if kind == download.KindHLS && resolved != "" {
		if _, err := download.ValidateURL(resolved); err != nil {
			return nil, fmt.Errorf("%w: resolved url", ErrDownloadBadRequest)
		}
	}

	selJSON, _ := json.Marshal(sel)
	jobPID := uuid.New()
	var resolvedPtr *string
	if resolved != "" {
		resolvedPtr = &resolved
	}
	var titlePtr, thumbPtr *string
	if title != "" {
		titlePtr = &title
	}
	if thumb != "" {
		thumbPtr = &thumb
	}
	parentID := parent.ID
	job, err := s.jobs.Create(ctx, repository.CreateDownloadJobInput{
		PublicID:       jobPID,
		CreatedBy:      userID,
		SourceURL:      rawURL,
		ResolvedURL:    resolvedPtr,
		Kind:           kind,
		Title:          titlePtr,
		ThumbnailURL:   thumbPtr,
		SelectedFormat: selJSON,
		ParentFolderID: &parentID,
		MaxAttempts:    3,
	})
	if err != nil {
		return nil, err
	}
	if s.progress != nil {
		_ = s.progress.Set(ctx, jobPID.String(), downloadprog.Progress{
			Stage:   "queued",
			Percent: 5,
		})
	}
	if err := s.enqueue.EnqueueFetch(ctx, jobPID, 3); err != nil {
		msg := err.Error()
		_ = s.jobs.MarkFailed(ctx, job.ID, msg)
		return nil, err
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, &userID, "download.create", "download_job", &job.ID, ip, ua, map[string]string{
			"kind": kind,
			"url":  download.RedactURL(rawURL),
		})
	}
	dto := s.toDTO(ctx, job)
	s.publishDTO(ctx, dto, true)
	return dto, nil
}

func (s *DownloadService) List(ctx context.Context, userID int64, cursor int64, limit int) ([]DownloadJobDTO, *int64, error) {
	items, next, err := s.jobs.ListByUser(ctx, userID, cursor, limit)
	if err != nil {
		return nil, nil, err
	}
	out := make([]DownloadJobDTO, 0, len(items))
	for i := range items {
		out = append(out, *s.toDTO(ctx, &items[i]))
	}
	return out, next, nil
}

func (s *DownloadService) Get(ctx context.Context, userID int64, role string, publicID uuid.UUID) (*DownloadJobDTO, error) {
	job, err := s.jobs.GetByPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, repository.ErrDownloadJobNotFound) {
			return nil, ErrDownloadNotFound
		}
		return nil, err
	}
	if job.CreatedBy == nil || *job.CreatedBy != userID {
		// owners can view any
		if role != "owner" {
			return nil, ErrDownloadForbidden
		}
	}
	return s.toDTO(ctx, job), nil
}

func (s *DownloadService) Cancel(ctx context.Context, userID int64, role, ip, ua string, publicID uuid.UUID) error {
	job, err := s.jobs.GetByPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, repository.ErrDownloadJobNotFound) {
			return ErrDownloadNotFound
		}
		return err
	}
	if job.CreatedBy == nil || *job.CreatedBy != userID {
		if role != "owner" {
			return ErrDownloadForbidden
		}
	}
	if job.Status == "succeeded" || job.Status == "cancelled" {
		return nil
	}
	if s.progress != nil {
		_ = s.progress.RequestCancel(ctx, publicID.String())
	}
	_ = s.enqueue.DeletePendingTaskByJobID(ctx, publicID)
	_ = s.jobs.MarkCancelled(ctx, job.ID)
	if s.audit != nil {
		_ = s.audit.Log(ctx, &userID, "download.cancel", "download_job", &job.ID, ip, ua, nil)
	}
	job.Status = "cancelled"
	s.publishDTO(ctx, s.toDTO(ctx, job), true)
	return nil
}

func (s *DownloadService) Retry(ctx context.Context, userID int64, role, ip, ua string, publicID uuid.UUID) (*DownloadJobDTO, error) {
	job, err := s.jobs.GetByPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, repository.ErrDownloadJobNotFound) {
			return nil, ErrDownloadNotFound
		}
		return nil, err
	}
	if job.CreatedBy == nil || *job.CreatedBy != userID {
		if role != "owner" {
			return nil, ErrDownloadForbidden
		}
	}
	if job.Status != "failed" && job.Status != "cancelled" {
		return nil, fmt.Errorf("%w: only failed/cancelled jobs can be retried", ErrDownloadBadRequest)
	}
	if s.progress != nil {
		_ = s.progress.ClearCancel(ctx, publicID.String())
		_ = s.progress.Clear(ctx, publicID.String())
		_ = s.progress.Set(ctx, publicID.String(), downloadprog.Progress{
			Stage:   "queued",
			Percent: 5,
		})
	}
	if err := s.jobs.ResetForRetry(ctx, job.ID); err != nil {
		return nil, err
	}
	if err := s.enqueue.EnqueueFetch(ctx, publicID, job.MaxAttempts); err != nil {
		msg := err.Error()
		_ = s.jobs.MarkFailed(ctx, job.ID, msg)
		return nil, err
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, &userID, "download.retry", "download_job", &job.ID, ip, ua, nil)
	}
	job, err = s.jobs.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	dto := s.toDTO(ctx, job)
	s.publishDTO(ctx, dto, true)
	return dto, nil
}

func (s *DownloadService) Delete(ctx context.Context, userID int64, role, ip, ua string, publicID uuid.UUID) error {
	job, err := s.jobs.GetByPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, repository.ErrDownloadJobNotFound) {
			return ErrDownloadNotFound
		}
		return err
	}
	if job.CreatedBy == nil || *job.CreatedBy != userID {
		if role != "owner" {
			return ErrDownloadForbidden
		}
	}
	if job.Status == "pending" || job.Status == "running" {
		return fmt.Errorf("%w: cancel the job before deleting", ErrDownloadBadRequest)
	}
	_ = s.enqueue.DeletePendingTaskByJobID(ctx, publicID)
	if s.progress != nil {
		_ = s.progress.Clear(ctx, publicID.String())
		_ = s.progress.ClearCancel(ctx, publicID.String())
	}
	if err := s.jobs.Delete(ctx, job.ID); err != nil {
		if errors.Is(err, repository.ErrDownloadJobNotFound) {
			return fmt.Errorf("%w: only finished jobs can be deleted", ErrDownloadBadRequest)
		}
		return err
	}
	if s.audit != nil {
		_ = s.audit.Log(ctx, &userID, "download.delete", "download_job", &job.ID, ip, ua, nil)
	}
	if s.progress != nil && job.CreatedBy != nil {
		deleted := "deleted"
		_ = s.progress.Publish(ctx, downloadprog.JobEvent{
			PublicID:  publicID.String(),
			CreatedBy: *job.CreatedBy,
			Status:    deleted,
		}, true)
	}
	return nil
}

// ProgressStore exposes the Redis progress/event bus for SSE.
func (s *DownloadService) ProgressStore() *downloadprog.ProgressStore {
	return s.progress
}

func (s *DownloadService) publishDTO(ctx context.Context, dto *DownloadJobDTO, force bool) {
	if s.progress == nil || dto == nil {
		return
	}
	// created_by is not on DTO — look up from DB for channel routing.
	pid, err := uuid.Parse(dto.PublicID)
	if err != nil {
		return
	}
	job, err := s.jobs.GetByPublicID(ctx, pid)
	if err != nil || job.CreatedBy == nil {
		return
	}
	ev := downloadprog.JobEvent{
		PublicID:      dto.PublicID,
		CreatedBy:     *job.CreatedBy,
		Status:        dto.Status,
		Kind:          dto.Kind,
		SourceURL:     dto.SourceURL,
		Title:         dto.Title,
		ThumbnailURL:  dto.ThumbnailURL,
		ProgressPct:   dto.ProgressPct,
		ProgressStage: dto.ProgressStage,
		BytesDone:     dto.BytesDone,
		BytesTotal:    dto.BytesTotal,
		VideoPublicID: dto.VideoPublicID,
		LastError:     dto.LastError,
		CreatedAt:     dto.CreatedAt,
		UpdatedAt:     dto.UpdatedAt,
	}
	_ = s.progress.Publish(ctx, ev, force)
}

func (s *DownloadService) resolveParent(ctx context.Context, parentPID *uuid.UUID) (*repository.MediaObject, error) {
	if parentPID == nil {
		return nil, fmt.Errorf("%w: parent_public_id required", ErrDownloadBadRequest)
	}
	m, err := s.media.Objects().GetByPublicID(ctx, *parentPID)
	if err != nil {
		return nil, err
	}
	if m.Type != "folder" {
		return nil, fmt.Errorf("%w: parent must be folder", ErrDownloadBadRequest)
	}
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	rootID, err := uuid.Parse(settings.Editable.Media.DefaultRootFolderPublicID)
	if err != nil {
		rootID = uuid.MustParse(repository.DefaultRootFolderPublicID)
	}
	if m.PublicID == rootID {
		return nil, fmt.Errorf("%w: choose a subfolder (not root)", ErrDownloadBadRequest)
	}
	return m, nil
}

func (s *DownloadService) toDTO(ctx context.Context, job *repository.DownloadJob) *DownloadJobDTO {
	dto := &DownloadJobDTO{
		PublicID:    job.PublicID.String(),
		SourceURL:   job.SourceURL,
		ResolvedURL: job.ResolvedURL,
		Kind:        job.Kind,
		Status:      job.Status,
		Title:       job.Title,
		ThumbnailURL: job.ThumbnailURL,
		ProgressPct: job.ProgressPct,
		BytesDone:   job.BytesDone,
		BytesTotal:  job.BytesTotal,
		LastError:   job.LastError,
		CreatedAt:   job.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   job.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if job.VideoPublicID != nil {
		v := job.VideoPublicID.String()
		dto.VideoPublicID = &v
	}
	// Live Redis snapshot wins for active jobs so reload/SSE snapshot shows
	// progress even between Postgres persist ticks (~1s).
	if s.progress != nil && (job.Status == "pending" || job.Status == "running") {
		if p, err := s.progress.Get(ctx, job.PublicID.String()); err == nil && p != nil {
			if p.Stage != "" {
				dto.ProgressStage = p.Stage
			}
			if p.Percent > dto.ProgressPct {
				dto.ProgressPct = p.Percent
			}
			if p.BytesDone > dto.BytesDone {
				dto.BytesDone = p.BytesDone
			}
			if p.BytesTotal > dto.BytesTotal {
				dto.BytesTotal = p.BytesTotal
			}
		}
	}
	return dto
}

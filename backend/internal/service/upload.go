package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/anhtuanlc/mediahub/internal/mediautil"
	"github.com/anhtuanlc/mediahub/internal/platform"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUploadTooLarge       = errors.New("upload exceeds max size")
	ErrUploadInvalidChunk   = errors.New("invalid chunk")
	ErrUploadSessionDone    = errors.New("upload session not pending")
	ErrUploadSessionExpired = errors.New("upload session expired")
	ErrUploadNotOwner       = errors.New("upload session not owned by user")
	ErrUploadIncomplete     = errors.New("upload missing parts")
	ErrUploadMultipart       = errors.New("multipart upload not configured for session")
	ErrUploadRateLimited     = errors.New("upload init rate limit exceeded")
	ErrUploadTooManyPending  = errors.New("too many pending uploads")
)

type UploadService struct {
	sessions      *repository.UploadSessionRepository
	media         *MediaObjectService
	pool          *pgxpool.Pool
	store         storage.ObjectStorage
	thumbnails    *ThumbnailService
	uploadLimiter *platform.UploadRateLimiter
	maxPending    int
}

func NewUploadService(
	sessions *repository.UploadSessionRepository,
	media *MediaObjectService,
	pool *pgxpool.Pool,
	store storage.ObjectStorage,
	thumbnails *ThumbnailService,
	uploadLimiter *platform.UploadRateLimiter,
	maxPending int,
) *UploadService {
	return &UploadService{
		sessions:      sessions,
		media:         media,
		pool:          pool,
		store:         store,
		thumbnails:    thumbnails,
		uploadLimiter: uploadLimiter,
		maxPending:    maxPending,
	}
}

type InitUploadInput struct {
	ParentPublicID uuid.UUID
	FileName       string
	Size           int64
	MimeType       string
	ChunkSize      int
}

type InitUploadResult struct {
	SessionPublicID string    `json:"session_public_id"`
	ChunkSize       int       `json:"chunk_size"`
	TotalChunks     int       `json:"total_chunks"`
	ExpiresAt       time.Time `json:"expires_at"`
	MaxUploadBytes  int64     `json:"max_upload_bytes"`
}

type UploadLimitsResult struct {
	MaxUploadBytes int64 `json:"max_upload_bytes"`
}

func (s *UploadService) Limits(ctx context.Context) (*UploadLimitsResult, error) {
	settings, err := s.media.Settings().Get(ctx)
	if err != nil {
		return nil, err
	}
	return &UploadLimitsResult{
		MaxUploadBytes: settings.Editable.Media.MaxUploadBytes,
	}, nil
}

func (s *UploadService) Init(ctx context.Context, userID int64, role string, in InitUploadInput) (*InitUploadResult, error) {
	if s.uploadLimiter != nil {
		ok, err := s.uploadLimiter.AllowInit(ctx, userID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrUploadRateLimited
		}
	}
	if s.maxPending > 0 {
		n, err := s.sessions.CountPendingByUser(ctx, userID)
		if err != nil {
			return nil, err
		}
		if n >= s.maxPending {
			return nil, ErrUploadTooManyPending
		}
	}
	if in.Size <= 0 {
		return nil, fmt.Errorf("%w: size required", ErrUploadInvalidChunk)
	}
	settings, err := s.media.Settings().Get(ctx)
	if err != nil {
		return nil, err
	}
	if in.Size > settings.Editable.Media.MaxUploadBytes {
		return nil, ErrUploadTooLarge
	}

	parent, err := s.media.Objects().GetByPublicID(ctx, in.ParentPublicID)
	if err != nil {
		return nil, err
	}
	if parent.Type != "folder" {
		return nil, ErrMediaInvalidParent
	}
	if err := s.media.CheckUploadPerm(ctx, userID, role, parent.ID); err != nil {
		return nil, err
	}

	chunkSize := mediautil.NormalizeChunkSize(in.ChunkSize)
	totalChunks := mediautil.ChunkCount(in.Size, chunkSize)
	if totalChunks > mediautil.MaxChunks {
		return nil, ErrUploadTooLarge
	}

	sessionID := uuid.New()
	var mimePtr *string
	mime := "application/octet-stream"
	if in.MimeType != "" {
		mime = in.MimeType
		mimePtr = &in.MimeType
	}

	objType := mediautil.DetectObjectType(mime, in.FileName)
	prefix := mediautil.StoragePrefixForType(objType)
	finalKey := storage.NewObjectKey(prefix)

	uploadID, err := s.store.CreateMultipartUpload(ctx, finalKey, mime)
	if err != nil {
		return nil, fmt.Errorf("create multipart upload: %w", err)
	}

	sess, err := s.sessions.Create(ctx, repository.CreateUploadSessionInput{
		PublicID:        sessionID,
		UserID:          userID,
		ParentObjectID:  parent.ID,
		OriginalName:    in.FileName,
		MimeType:        mimePtr,
		TotalSize:       in.Size,
		ChunkSize:       chunkSize,
		StoragePrefix:   finalKey,
		S3UploadID:      uploadID,
		FinalStorageKey: finalKey,
		ExpiresAt:       time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		_ = s.store.AbortMultipartUpload(ctx, finalKey, uploadID)
		return nil, err
	}

	platform.Upload.InitTotal.Add(1)
	return &InitUploadResult{
		SessionPublicID: sess.PublicID.String(),
		ChunkSize:       chunkSize,
		TotalChunks:     totalChunks,
		ExpiresAt:       sess.ExpiresAt,
		MaxUploadBytes:  settings.Editable.Media.MaxUploadBytes,
	}, nil
}

func (s *UploadService) PutChunk(ctx context.Context, userID int64, sessionPublicID uuid.UUID, index int, body io.Reader, size int64) error {
	sess, err := s.sessions.GetByPublicID(ctx, sessionPublicID)
	if err != nil {
		return err
	}
	if err := s.validateSessionActive(sess, userID); err != nil {
		return err
	}
	if sess.S3UploadID == nil || sess.FinalStorageKey == nil {
		return ErrUploadMultipart
	}

	expected := mediautil.ChunkCount(sess.TotalSize, sess.ChunkSize)
	if index < 0 || index >= expected {
		return ErrUploadInvalidChunk
	}
	maxChunk := int64(sess.ChunkSize)
	if index == expected-1 {
		remainder := sess.TotalSize - int64(index)*int64(sess.ChunkSize)
		if remainder > 0 && remainder < maxChunk {
			maxChunk = remainder
		}
	}
	if size > maxChunk {
		return ErrUploadInvalidChunk
	}

	data, err := io.ReadAll(io.LimitReader(body, size))
	if err != nil {
		return fmt.Errorf("read chunk body: %w", err)
	}
	if int64(len(data)) != size {
		return ErrUploadInvalidChunk
	}

	partNumber := int32(index + 1)
	etag, err := s.store.UploadPart(ctx, *sess.FinalStorageKey, *sess.S3UploadID, partNumber, bytes.NewReader(data), size)
	if err != nil {
		return err
	}

	platform.Upload.ChunkTotal.Add(1)
	return s.sessions.AppendPart(ctx, sess.ID, repository.UploadPartRecord{
		PartNumber: partNumber,
		ETag:       etag,
	})
}

func (s *UploadService) Complete(ctx context.Context, userID int64, role string, sessionPublicID uuid.UUID, ip, ua string) (*MediaObjectDTO, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	sess, err := s.sessions.GetByPublicIDForUpdateTx(ctx, tx, sessionPublicID)
	if err != nil {
		return nil, err
	}
	if err := s.validateSessionActive(sess, userID); err != nil {
		return nil, err
	}
	if sess.S3UploadID == nil || sess.FinalStorageKey == nil {
		return nil, ErrUploadMultipart
	}

	expectedParts := mediautil.ChunkCount(sess.TotalSize, sess.ChunkSize)
	parts, err := mergeUploadParts(sess.Parts, expectedParts)
	if err != nil {
		return nil, err
	}

	if err := s.store.CompleteMultipartUpload(ctx, *sess.FinalStorageKey, *sess.S3UploadID, parts); err != nil {
		return nil, err
	}

	var checksumPtr *string
	if sess.TotalSize > 0 {
		sum, err := s.store.HashObjectSHA256(ctx, *sess.FinalStorageKey, sess.TotalSize)
		if err != nil {
			return nil, fmt.Errorf("checksum object: %w", err)
		}
		checksumPtr = &sum
	}

	m, err := s.media.BuildCommittedObject(ctx, userID, sess.ParentObjectID, sess.OriginalName, sess.MimeType, sess.TotalSize, *sess.FinalStorageKey, checksumPtr)
	if err != nil {
		return nil, err
	}

	if err := s.sessions.MarkCompleted(ctx, tx, sess.ID, m.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	tid := m.ID
	_ = s.media.Audit().Log(ctx, &userID, "upload", m.Type, &tid, ip, ua, map[string]string{"name": sess.OriginalName})

	platform.Upload.CompleteTotal.Add(1)
	if m.Type == "image" && s.thumbnails != nil && m.StorageKey != nil {
		pid := m.PublicID
		srcKey := *m.StorageKey
		go func() {
			_ = s.thumbnails.GenerateForObject(context.Background(), pid, srcKey)
		}()
	}

	return s.media.Get(ctx, userID, role, m.PublicID)
}

func mergeUploadParts(records []repository.UploadPartRecord, expected int) ([]storage.CompletedPart, error) {
	if len(records) < expected {
		return nil, fmt.Errorf("%w: got %d of %d parts", ErrUploadIncomplete, len(records), expected)
	}
	byPart := make(map[int32]storage.CompletedPart, len(records))
	for _, r := range records {
		if r.PartNumber < 1 || r.ETag == "" {
			return nil, ErrUploadInvalidChunk
		}
		if _, dup := byPart[r.PartNumber]; dup {
			return nil, fmt.Errorf("%w: duplicate part %d", ErrUploadInvalidChunk, r.PartNumber)
		}
		byPart[r.PartNumber] = storage.CompletedPart{
			PartNumber: r.PartNumber,
			ETag:       r.ETag,
		}
	}
	if len(byPart) != expected {
		return nil, fmt.Errorf("%w: got %d unique parts, need %d", ErrUploadIncomplete, len(byPart), expected)
	}
	out := make([]storage.CompletedPart, 0, expected)
	for i := 1; i <= expected; i++ {
		p, ok := byPart[int32(i)]
		if !ok {
			return nil, fmt.Errorf("%w: missing part %d", ErrUploadIncomplete, i)
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].PartNumber < out[j].PartNumber
	})
	return out, nil
}

func (s *UploadService) Abort(ctx context.Context, userID int64, sessionPublicID uuid.UUID) error {
	sess, err := s.sessions.GetByPublicID(ctx, sessionPublicID)
	if err != nil {
		return err
	}
	if sess.UserID != userID {
		return ErrUploadNotOwner
	}
	if sess.Status != "pending" {
		return nil
	}
	s.abortMultipart(ctx, sess)
	return s.sessions.MarkAborted(ctx, sess.ID)
}

// ExpireStaleSessions aborts multipart uploads for expired pending sessions.
func (s *UploadService) ExpireStaleSessions(ctx context.Context) (int, error) {
	sessions, err := s.sessions.ListExpiredPending(ctx, 100)
	if err != nil {
		return 0, err
	}
	var n int
	for i := range sessions {
		sess := &sessions[i]
		s.abortMultipart(ctx, sess)
		if err := s.sessions.MarkExpired(ctx, sess.ID); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (s *UploadService) validateSessionActive(sess *repository.UploadSession, userID int64) error {
	if sess.UserID != userID {
		return ErrUploadNotOwner
	}
	if sess.Status != "pending" {
		return ErrUploadSessionDone
	}
	if time.Now().After(sess.ExpiresAt) {
		return ErrUploadSessionExpired
	}
	return nil
}

func (s *UploadService) abortMultipart(ctx context.Context, sess *repository.UploadSession) {
	if sess.S3UploadID == nil || sess.FinalStorageKey == nil {
		return
	}
	_ = s.store.AbortMultipartUpload(ctx, *sess.FinalStorageKey, *sess.S3UploadID)
	// Legacy chunk temp layout (pre-multipart sessions).
	if sess.StoragePrefix != "" && sess.StoragePrefix != *sess.FinalStorageKey {
		if deleter, ok := s.store.(*storage.S3Storage); ok {
			_ = deleter.DeletePrefix(ctx, sess.StoragePrefix)
		}
	}
}

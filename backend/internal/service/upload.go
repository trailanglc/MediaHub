package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/anhtuanlc/mediahub/internal/mediautil"
	"github.com/anhtuanlc/mediahub/internal/platform/upload"
	"github.com/anhtuanlc/mediahub/internal/platform/webhook"
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
	ErrUploadDirectRequired  = errors.New("direct upload requires PUT to storage before complete")
)

const (
	UploadModeMultipart = "multipart"
	UploadModeDirect    = "direct"
)

type UploadService struct {
	sessions      *repository.UploadSessionRepository
	media         *MediaObjectService
	pool          *pgxpool.Pool
	store         storage.ObjectStorage
	thumbnails    *ThumbnailService
	uploadLimiter *upload.RateLimiter
	webhooks         *webhook.Dispatcher
	maxPending       int
	presignedPutTTL  time.Duration
}

func NewUploadService(
	sessions *repository.UploadSessionRepository,
	media *MediaObjectService,
	pool *pgxpool.Pool,
	store storage.ObjectStorage,
	thumbnails *ThumbnailService,
	uploadLimiter *upload.RateLimiter,
	maxPending int,
	presignedPutTTL time.Duration,
) *UploadService {
	if presignedPutTTL <= 0 {
		presignedPutTTL = time.Hour
	}
	return &UploadService{
		sessions:        sessions,
		media:           media,
		pool:            pool,
		store:           store,
		thumbnails:      thumbnails,
		uploadLimiter:   uploadLimiter,
		maxPending:      maxPending,
		presignedPutTTL: presignedPutTTL,
	}
}

func (s *UploadService) SetWebhooks(d *webhook.Dispatcher) {
	s.webhooks = d
}

type InitUploadInput struct {
	ParentPublicID uuid.UUID
	FileName       string
	Size           int64
	MimeType       string
	ChunkSize      int
	Mode           string // multipart (default) or direct
}

type InitUploadResult struct {
	SessionPublicID string            `json:"session_public_id"`
	UploadMode      string            `json:"upload_mode"`
	ChunkSize       int               `json:"chunk_size"`
	TotalChunks     int               `json:"total_chunks"`
	PutURL          *string           `json:"put_url,omitempty"`
	PutHeaders      map[string]string `json:"put_headers,omitempty"`
	ExpiresAt       time.Time         `json:"expires_at"`
	MaxUploadBytes  int64             `json:"max_upload_bytes"`
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
	if err := s.reconcilePendingSessions(ctx, userID); err != nil {
		return nil, err
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
	expiresAt := time.Now().Add(24 * time.Hour)

	mode := strings.TrimSpace(in.Mode)
	if mode == "" {
		mode = UploadModeMultipart
	}
	if mode != UploadModeMultipart && mode != UploadModeDirect {
		return nil, fmt.Errorf("%w: invalid upload mode", ErrUploadInvalidChunk)
	}

	if mode == UploadModeDirect {
		putURL, err := s.store.PresignPutObject(ctx, finalKey, mime, in.Size, s.presignedPutTTL)
		if err != nil {
			return nil, fmt.Errorf("presign put: %w", err)
		}
		sess, err := s.sessions.Create(ctx, repository.CreateUploadSessionInput{
			PublicID:        sessionID,
			UserID:          userID,
			ParentObjectID:  parent.ID,
			OriginalName:    in.FileName,
			MimeType:        mimePtr,
			TotalSize:       in.Size,
			ChunkSize:       0,
			StoragePrefix:   finalKey,
			S3UploadID:      nil,
			FinalStorageKey: finalKey,
			ExpiresAt:       expiresAt,
		})
		if err != nil {
			return nil, err
		}
		upload.Default.InitTotal.Add(1)
		headers := map[string]string{
			"Content-Type":   mime,
			"Content-Length": strconv.FormatInt(in.Size, 10),
		}
		return &InitUploadResult{
			SessionPublicID: sess.PublicID.String(),
			UploadMode:      UploadModeDirect,
			ChunkSize:       0,
			TotalChunks:     0,
			PutURL:          &putURL,
			PutHeaders:      headers,
			ExpiresAt:       sess.ExpiresAt,
			MaxUploadBytes:  settings.Editable.Media.MaxUploadBytes,
		}, nil
	}

	chunkSize := mediautil.NormalizeChunkSize(in.ChunkSize)
	totalChunks := mediautil.ChunkCount(in.Size, chunkSize)
	if totalChunks > mediautil.MaxChunks {
		return nil, ErrUploadTooLarge
	}

	uploadID, err := s.store.CreateMultipartUpload(ctx, finalKey, mime)
	if err != nil {
		return nil, fmt.Errorf("create multipart upload: %w", err)
	}
	uploadIDPtr := uploadID

	sess, err := s.sessions.Create(ctx, repository.CreateUploadSessionInput{
		PublicID:        sessionID,
		UserID:          userID,
		ParentObjectID:  parent.ID,
		OriginalName:    in.FileName,
		MimeType:        mimePtr,
		TotalSize:       in.Size,
		ChunkSize:       chunkSize,
		StoragePrefix:   finalKey,
		S3UploadID:      &uploadIDPtr,
		FinalStorageKey: finalKey,
		ExpiresAt:       expiresAt,
	})
	if err != nil {
		_ = s.store.AbortMultipartUpload(ctx, finalKey, uploadID)
		return nil, err
	}

	upload.Default.InitTotal.Add(1)
	return &InitUploadResult{
		SessionPublicID: sess.PublicID.String(),
		UploadMode:      UploadModeMultipart,
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
	if isDirectUploadSession(sess) {
		return fmt.Errorf("%w: use presigned PUT then complete", ErrUploadInvalidChunk)
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

	upload.Default.ChunkTotal.Add(1)
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
	if sess.FinalStorageKey == nil || *sess.FinalStorageKey == "" {
		return nil, ErrUploadMultipart
	}

	if isDirectUploadSession(sess) {
		info, err := s.store.StatObject(ctx, *sess.FinalStorageKey)
		if err != nil {
			return nil, ErrUploadDirectRequired
		}
		gotSize := info.TotalSize
		if gotSize == 0 {
			gotSize = info.Size
		}
		if gotSize != sess.TotalSize {
			return nil, fmt.Errorf("%w: size mismatch", ErrUploadInvalidChunk)
		}
	} else {
		if sess.S3UploadID == nil {
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

	upload.Default.CompleteTotal.Add(1)
	if m.Type == "image" && s.thumbnails != nil && m.StorageKey != nil {
		pid := m.PublicID
		srcKey := *m.StorageKey
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			_ = s.thumbnails.GenerateForObject(ctx, pid, srcKey)
		}()
	}

	dto, err := s.media.Get(ctx, userID, role, m.PublicID)
	if err != nil {
		return nil, err
	}
	if s.webhooks != nil {
		s.webhooks.Emit(ctx, webhook.EventUploadCompleted, map[string]any{
			"public_id": m.PublicID.String(),
			"type":      m.Type,
			"name":      m.Name,
		})
	}
	return dto, nil
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

// reconcilePendingSessions expires stale sessions and aborts oldest excess pending
// so a new upload can start (orphaned inits from failed/cancelled uploads).
func (s *UploadService) reconcilePendingSessions(ctx context.Context, userID int64) error {
	if _, err := s.ExpireStaleSessions(ctx); err != nil {
		return err
	}
	if s.maxPending <= 0 {
		return nil
	}
	n, err := s.sessions.CountPendingByUser(ctx, userID)
	if err != nil {
		return err
	}
	if n < s.maxPending {
		return nil
	}
	excess := n - s.maxPending + 1
	list, err := s.sessions.ListActivePendingByUser(ctx, userID, excess)
	if err != nil {
		return err
	}
	for i := range list {
		sess := &list[i]
		s.abortMultipart(ctx, sess)
		if err := s.sessions.MarkAborted(ctx, sess.ID); err != nil {
			return err
		}
	}
	n, err = s.sessions.CountPendingByUser(ctx, userID)
	if err != nil {
		return err
	}
	if n >= s.maxPending {
		return ErrUploadTooManyPending
	}
	return nil
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

func isDirectUploadSession(sess *repository.UploadSession) bool {
	return sess.S3UploadID == nil || *sess.S3UploadID == ""
}

func (s *UploadService) abortMultipart(ctx context.Context, sess *repository.UploadSession) {
	if sess.FinalStorageKey != nil && *sess.FinalStorageKey != "" {
		if isDirectUploadSession(sess) {
			_ = s.store.DeleteObject(ctx, *sess.FinalStorageKey)
			return
		}
	}
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

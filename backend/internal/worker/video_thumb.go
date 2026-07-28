package worker

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anhtuanlc/mediahub/internal/download"
	"github.com/anhtuanlc/mediahub/internal/repository"
	"github.com/anhtuanlc/mediahub/internal/storage"
	"github.com/anhtuanlc/mediahub/internal/transcode"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// persistVideoThumbnail uploads a local JPEG and stores thumbnail_key on asset + object.
func persistVideoThumbnail(
	ctx context.Context,
	store storage.ObjectStorage,
	videos *repository.VideoRepository,
	objects *repository.MediaObjectRepository,
	videoPID uuid.UUID,
	assetID, objectID int64,
	localJPEG string,
) (string, error) {
	f, err := os.Open(localJPEG)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	if st.Size() <= 0 {
		return "", fmt.Errorf("empty thumbnail")
	}
	key := storage.ThumbnailObjectKey(videoPID)
	if err := store.PutObject(ctx, key, f, st.Size(), "image/jpeg"); err != nil {
		return "", err
	}
	_ = videos.SetThumbnailKey(ctx, assetID, key)
	_ = objects.SetThumbnailKey(ctx, objectID, key)
	return key, nil
}

func extractThumbnailFile(ctx context.Context, ffmpegPath, inputPath, outJPEG string) error {
	cfg := transcode.Config{FFmpegPath: ffmpegPath}
	if cfg.FFmpegPath == "" {
		cfg.FFmpegPath = "ffmpeg"
	}
	return transcode.ExtractThumbnail(ctx, cfg, inputPath, outJPEG)
}

func downloadRemoteThumbnail(ctx context.Context, rawURL, outPath string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return fmt.Errorf("empty url")
	}
	u, err := download.ValidateURL(rawURL)
	if err != nil {
		return err
	}
	if err := download.ResolveAndCheck(ctx, u); err != nil {
		return err
	}
	client := download.SafeHTTPClient(45 * time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "MediaHub/1.0")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("thumbnail http %d", res.StatusCode)
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return err
	}
	if n < 64 {
		return fmt.Errorf("thumbnail too small")
	}
	return nil
}

// saveVideoThumbnail tries remote poster URL, then a frame from local media (file or HLS dir).
func saveVideoThumbnail(
	ctx context.Context,
	log *zap.Logger,
	store storage.ObjectStorage,
	videos *repository.VideoRepository,
	objects *repository.MediaObjectRepository,
	ffmpegPath string,
	videoPID uuid.UUID,
	assetID, objectID int64,
	remoteURL *string,
	localMediaPath string,
) string {
	tmpDir, err := os.MkdirTemp("", "mh-thumb-*")
	if err != nil {
		return ""
	}
	defer os.RemoveAll(tmpDir)
	outJPEG := filepath.Join(tmpDir, "thumb.jpg")

	tryPersist := func() string {
		key, err := persistVideoThumbnail(ctx, store, videos, objects, videoPID, assetID, objectID, outJPEG)
		if err != nil {
			if log != nil {
				log.Debug("thumbnail.persist_failed", zap.Error(err), zap.String("video_public_id", videoPID.String()))
			}
			return ""
		}
		return key
	}

	if remoteURL != nil && strings.TrimSpace(*remoteURL) != "" {
		rawPath := filepath.Join(tmpDir, "remote.bin")
		if err := downloadRemoteThumbnail(ctx, *remoteURL, rawPath); err == nil {
			if extractThumbnailFile(ctx, ffmpegPath, rawPath, outJPEG) == nil {
				if key := tryPersist(); key != "" {
					return key
				}
			} else {
				_ = os.Rename(rawPath, outJPEG)
				if key := tryPersist(); key != "" {
					return key
				}
			}
		} else if log != nil {
			log.Debug("thumbnail.remote_failed", zap.Error(err), zap.String("video_public_id", videoPID.String()))
		}
	}

	var candidates []string
	if localMediaPath != "" {
		st, err := os.Stat(localMediaPath)
		if err == nil && !st.IsDir() {
			candidates = append(candidates, localMediaPath)
		} else if err == nil && st.IsDir() {
			var tsFiles, other []string
			_ = filepath.WalkDir(localMediaPath, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				low := strings.ToLower(path)
				switch {
				case strings.HasSuffix(low, ".ts"), strings.HasSuffix(low, ".mp4"),
					strings.HasSuffix(low, ".mkv"), strings.HasSuffix(low, ".webm"),
					strings.HasSuffix(low, ".m4s"):
					tsFiles = append(tsFiles, path)
				case strings.HasSuffix(low, ".m3u8"):
					other = append(other, path)
				}
				return nil
			})
			candidates = append(candidates, tsFiles...)
			candidates = append(candidates, other...)
		}
	}
	for _, c := range candidates {
		if extractThumbnailFile(ctx, ffmpegPath, c, outJPEG) == nil {
			if key := tryPersist(); key != "" {
				return key
			}
		}
	}
	if log != nil {
		log.Debug("thumbnail.extract_failed", zap.String("video_public_id", videoPID.String()))
	}
	return ""
}

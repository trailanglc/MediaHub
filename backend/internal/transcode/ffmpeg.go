package transcode

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ProbeResult struct {
	DurationSeconds int
	Width           int
	Height          int
	Codec           string
	Bitrate         int64
}

type Config struct {
	FFmpegPath  string
	FFprobePath string
	Timeout     time.Duration
	// Threads limits ffmpeg/ffprobe CPU use (0 = ffmpeg default).
	Threads int
}

// ProgressFunc reports convert stage and 0–100 percent.
type ProgressFunc func(stage string, percent int)

// scaleVF returns a video filter that scales to max height and forces even dimensions for libx264.
func scaleVF(maxHeight int) string {
	return fmt.Sprintf(
		"scale=-2:%d:force_original_aspect_ratio=decrease,scale=trunc(iw/2)*2:trunc(ih/2)*2",
		maxHeight,
	)
}

func Probe(ctx context.Context, cfg Config, inputPath string) (*ProbeResult, error) {
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		inputPath,
	}
	if cfg.Threads > 0 {
		args = append([]string{"-threads", strconv.Itoa(cfg.Threads)}, args...)
	}
	cmd := exec.CommandContext(ctx, cfg.FFprobePath, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}
	var parsed struct {
		Format struct {
			Duration string `json:"duration"`
			BitRate  string `json:"bit_rate"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, err
	}
	res := &ProbeResult{}
	if d, err := strconv.ParseFloat(parsed.Format.Duration, 64); err == nil {
		res.DurationSeconds = int(d + 0.5)
	}
	if b, err := strconv.ParseInt(parsed.Format.BitRate, 10, 64); err == nil {
		res.Bitrate = b
	}
	for _, s := range parsed.Streams {
		if s.CodecType == "video" {
			res.Width = s.Width
			res.Height = s.Height
			res.Codec = s.CodecName
			break
		}
	}
	return res, nil
}

// ConvertToHLS runs ffmpeg to produce multi-bitrate HLS under outDir.
// variantNames is the client-selected list; empty uses all renditions that fit the source.
func ConvertToHLS(ctx context.Context, cfg Config, inputPath, outDir string, variantNames []string, onProgress ProgressFunc) error {
	probe, _ := Probe(ctx, cfg, inputPath)
	src := SourceProfile{}
	if probe != nil {
		src.Height = probe.Height
		src.Bitrate = probe.Bitrate
	}
	variants, err := ResolveVariants(variantNames, src)
	if err != nil {
		return err
	}
	if onProgress != nil {
		onProgress("probe", 12)
	}

	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	basePct := 15
	step := 55 / len(variants)
	if step < 1 {
		step = 1
	}

	for i, v := range variants {
		if onProgress != nil {
			onProgress("encode_"+v.Name, basePct+i*step)
		}
		variantDir := filepath.Join(outDir, v.Name)
		if err := os.MkdirAll(variantDir, 0o750); err != nil {
			return err
		}
		maxH := v.Height
		if src.Height > 0 && src.Height < maxH {
			maxH = src.Height
			if maxH%2 != 0 {
				maxH--
			}
		}
		preset := "fast"
		if i == 0 {
			preset = "medium" // rendition cao nhất: chất lượng tốt hơn cho bản 1080p
		}
		args := []string{
			"-y", "-i", inputPath,
			"-vf", scaleVF(maxH),
			"-c:v", "libx264", "-preset", preset, "-b:v", EncodeBitrate(v, src),
			"-c:a", "aac", "-b:a", "128k", "-ac", "2",
			"-f", "hls",
			"-hls_time", "6",
			"-hls_playlist_type", "vod",
			"-hls_segment_filename", filepath.Join(variantDir, "segment_%05d.ts"),
			filepath.Join(variantDir, "index.m3u8"),
		}
		if cfg.Threads > 0 {
			args = append([]string{"-threads", strconv.Itoa(cfg.Threads)}, args...)
		}
		cmd := exec.CommandContext(ctx, cfg.FFmpegPath, args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("ffmpeg %s: %w: %s", v.Name, err, trimFFmpegLog(string(out)))
		}
	}

	if onProgress != nil {
		onProgress("playlist", 72)
	}
	return writeMasterPlaylist(outDir, variants)
}

func trimFFmpegLog(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 2000 {
		return s[len(s)-2000:]
	}
	return s
}

func writeMasterPlaylist(outDir string, variants []HLSVariant) error {
	var lines []string
	lines = append(lines, "#EXTM3U", "#EXT-X-VERSION:3")
	for _, v := range variants {
		bw := strings.TrimSuffix(v.Bitrate, "k") + "000"
		lines = append(lines, fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%s,RESOLUTION=1280x%d", bw, v.Height))
		lines = append(lines, v.Name+"/index.m3u8")
	}
	master := filepath.Join(outDir, "master.m3u8")
	return os.WriteFile(master, []byte(strings.Join(lines, "\n")+"\n"), 0o640)
}

// ValidateHLSOutput checks master and at least one segment exist.
func ValidateHLSOutput(outDir string) error {
	master := filepath.Join(outDir, "master.m3u8")
	if _, err := os.Stat(master); err != nil {
		return fmt.Errorf("missing master.m3u8: %w", err)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		segDir := filepath.Join(outDir, e.Name())
		files, err := os.ReadDir(segDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".ts") {
				return nil
			}
		}
	}
	return fmt.Errorf("no hls segments found")
}

// ExtractThumbnail grabs one frame as JPEG.
func ExtractThumbnail(ctx context.Context, cfg Config, inputPath, outputPath string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	args := []string{
		"-y", "-ss", "1", "-i", inputPath,
		"-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2",
		"-frames:v", "1", "-q:v", "2",
		outputPath,
	}
	if cfg.Threads > 0 {
		args = append([]string{"-threads", strconv.Itoa(cfg.Threads)}, args...)
	}
	cmd := exec.CommandContext(ctx, cfg.FFmpegPath, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("thumbnail: %w: %s", err, trimFFmpegLog(string(out)))
	}
	return nil
}

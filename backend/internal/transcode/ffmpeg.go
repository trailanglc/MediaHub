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
	FPS             float64
}

type Config struct {
	FFmpegPath  string
	FFprobePath string
	Timeout     time.Duration
	// Threads limits ffmpeg/ffprobe CPU use (0 = ffmpeg default).
	Threads int
	// HwAccel: auto | vaapi | none (default auto).
	HwAccel string
	// VAAPIDevice is the DRM render node (default /dev/dri/renderD128).
	VAAPIDevice string
}

// ProgressFunc reports convert stage and 0–100 percent.
type ProgressFunc func(stage string, percent int)

// scaleVF returns a video filter that scales to max height and forces even dimensions for libx264.
func scaleVF(maxHeight int) string {
	return scaleVFSharp(maxHeight)
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
			CodecType    string `json:"codec_type"`
			CodecName    string `json:"codec_name"`
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			RFrameRate   string `json:"r_frame_rate"`
			AvgFrameRate string `json:"avg_frame_rate"`
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
			res.FPS = parseFrameRate(s.RFrameRate)
			if res.FPS <= 0 {
				res.FPS = parseFrameRate(s.AvgFrameRate)
			}
			break
		}
	}
	return res, nil
}

// parseFrameRate converts ffprobe rational frame rate ("30000/1001") to fps.
func parseFrameRate(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "0/0" {
		return 0
	}
	if i := strings.Index(s, "/"); i >= 0 {
		num, err1 := strconv.ParseFloat(s[:i], 64)
		den, err2 := strconv.ParseFloat(s[i+1:], 64)
		if err1 == nil && err2 == nil && den != 0 {
			return num / den
		}
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

// ConvertToHLS runs ffmpeg to produce multi-bitrate HLS under outDir.
// variantNames is the client-selected list; empty uses all renditions that fit the source.
func ConvertToHLS(ctx context.Context, cfg Config, inputPath, outDir string, variantNames []string, onProgress ProgressFunc) error {
	probe, _ := Probe(ctx, cfg, inputPath)
	src := SourceProfile{}
	if probe != nil {
		src.Width = probe.Width
		src.Height = probe.Height
		src.Bitrate = probe.Bitrate
		src.FPS = probe.FPS
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

	hw := ResolveHwAccel(cfg.FFmpegPath, cfg.HwAccel, cfg.VAAPIDevice)
	device := cfg.VAAPIDevice
	if device == "" {
		device = "/dev/dri/renderD128"
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
		// CPU fallback uses slow: user prioritizes sharpness over encode time.
		preset := "slow"
		levelStr := h264LevelString(encodedWidth(maxH, src), maxH, src.FPS)
		br := EncodeBitrate(v, src)

		// High-quality sources (≥1080p or high bitrate): slightly lower VAAPI QP for sharpness.
		qp := 17
		if src.Height >= 1080 || src.Bitrate >= 6_000_000 {
			qp = 15
		}

		var args []string
		useVAAPI := hw == HwAccelVAAPI
		if useVAAPI {
			args = buildVAAPIEncodeArgs(inputPath, variantDir, device, maxH, br, levelStr, qp)
		} else {
			args = buildCPUEncodeArgs(inputPath, variantDir, maxH, br, levelStr, preset)
		}
		if cfg.Threads > 0 && !useVAAPI {
			args = append([]string{"-threads", strconv.Itoa(cfg.Threads)}, args...)
		}
		cmd := exec.CommandContext(ctx, cfg.FFmpegPath, args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			if useVAAPI {
				// Absolute quality: never leave a failed GPU encode — retry on libx264 slow.
				args = buildCPUEncodeArgs(inputPath, variantDir, maxH, br, levelStr, preset)
				if cfg.Threads > 0 {
					args = append([]string{"-threads", strconv.Itoa(cfg.Threads)}, args...)
				}
				cmd = exec.CommandContext(ctx, cfg.FFmpegPath, args...)
				if out2, err2 := cmd.CombinedOutput(); err2 != nil {
					return fmt.Errorf("ffmpeg %s: vaapi failed (%v); cpu fallback: %w: %s",
						v.Name, err, err2, trimFFmpegLog(string(out2)+" | vaapi: "+string(out)))
				}
			} else {
				return fmt.Errorf("ffmpeg %s: %w: %s", v.Name, err, trimFFmpegLog(string(out)))
			}
		}
	}

	if onProgress != nil {
		onProgress("playlist", 72)
	}
	return writeMasterPlaylist(outDir, variants, src)
}

func trimFFmpegLog(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 2000 {
		return s[len(s)-2000:]
	}
	return s
}

const audioBitrateBps = 192_000

func writeMasterPlaylist(outDir string, variants []HLSVariant, src SourceProfile) error {
	var lines []string
	lines = append(lines, "#EXTM3U", "#EXT-X-VERSION:3", "#EXT-X-INDEPENDENT-SEGMENTS")
	for _, v := range variants {
		encH := v.Height
		if src.Height > 0 && src.Height < encH {
			encH = src.Height
		}
		if encH%2 != 0 {
			encH--
		}
		encW := encodedWidth(encH, src)

		videoBps := parseBitrateString(EncodeBitrate(v, src))
		avgBps := videoBps + audioBitrateBps
		// Peak allowance over the average target for VBV headroom.
		peakBps := int64(float64(videoBps)*1.2) + audioBitrateBps

		codecs := h264AudioCodecs(encW, encH, src.FPS)
		lines = append(lines, fmt.Sprintf(
			"#EXT-X-STREAM-INF:BANDWIDTH=%d,AVERAGE-BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS=\"%s\"",
			peakBps, avgBps, encW, encH, codecs,
		))
		lines = append(lines, v.Name+"/index.m3u8")
	}
	master := filepath.Join(outDir, "master.m3u8")
	return os.WriteFile(master, []byte(strings.Join(lines, "\n")+"\n"), 0o640)
}

// encodedWidth derives the even output width for a target height, preserving the source aspect
// ratio (falls back to 16:9 when the source dimensions are unknown). Mirrors scaleVF(-2:height).
func encodedWidth(height int, src SourceProfile) int {
	num, den := src.Width, src.Height
	if num <= 0 || den <= 0 {
		num, den = 16, 9
	}
	w := int(float64(height)*float64(num)/float64(den) + 0.5)
	if w%2 != 0 {
		w--
	}
	if w < 2 {
		w = 2
	}
	return w
}

// h264LevelIDC returns the H.264 level_idc (decimal, e.g. 40 for 4.0) needed for the given
// frame size and frame rate, using the standard MaxFS / MaxMBPS limits.
func h264LevelIDC(width, height int, fps float64) int {
	if fps <= 0 {
		fps = 30
	}
	mbW := (width + 15) / 16
	mbH := (height + 15) / 16
	mbFrame := mbW * mbH
	mbps := float64(mbFrame) * fps
	levels := []struct {
		idc     int
		maxMBPS float64
		maxFS   int
	}{
		{30, 40500, 1620},
		{31, 108000, 3600},
		{32, 216000, 5120},
		{40, 245760, 8192},
		{42, 522240, 8704},
		{50, 589824, 22080},
		{51, 983040, 36864},
	}
	for _, l := range levels {
		if mbFrame <= l.maxFS && mbps <= l.maxMBPS {
			return l.idc
		}
	}
	return 51
}

func h264LevelString(width, height int, fps float64) string {
	idc := h264LevelIDC(width, height, fps)
	return fmt.Sprintf("%d.%d", idc/10, idc%10)
}

// h264AudioCodecs returns the RFC 6381 CODECS attribute for High-profile H.264 + AAC-LC,
// matching the pinned encoder profile/level.
func h264AudioCodecs(width, height int, fps float64) string {
	idc := h264LevelIDC(width, height, fps)
	return fmt.Sprintf("avc1.6400%02x,mp4a.40.2", idc)
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
		"-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2:flags=lanczos",
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

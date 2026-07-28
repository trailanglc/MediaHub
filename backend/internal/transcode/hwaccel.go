package transcode

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// HwAccel modes for ConvertToHLS.
const (
	HwAccelAuto  = "auto"
	HwAccelVAAPI = "vaapi"
	HwAccelNone  = "none"
)

// ResolveHwAccel picks an effective accelerator. "auto" prefers VAAPI when the device works.
func ResolveHwAccel(ffmpegPath, mode, device string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" || mode == HwAccelAuto {
		if vaapiDeviceReady(device) && vaapiEncoderReady(ffmpegPath, device) {
			return HwAccelVAAPI
		}
		return HwAccelNone
	}
	if mode == HwAccelVAAPI {
		if vaapiDeviceReady(device) && vaapiEncoderReady(ffmpegPath, device) {
			return HwAccelVAAPI
		}
		return HwAccelNone
	}
	return HwAccelNone
}

func vaapiDeviceReady(device string) bool {
	if device == "" {
		device = "/dev/dri/renderD128"
	}
	st, err := os.Stat(device)
	if err != nil {
		return false
	}
	return !st.IsDir()
}

// vaapiEncoderReady probes a short 720p CQP encode (min size for many AMD iGPUs is 128px).
func vaapiEncoderReady(ffmpegPath, device string) bool {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	if device == "" {
		device = "/dev/dri/renderD128"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-init_hw_device", "vaapi=va:" + device,
		"-filter_hw_device", "va",
		"-f", "lavfi", "-i", "color=c=black:s=1280x720:d=0.2",
		"-vf", "format=nv12,hwupload",
		"-c:v", "h264_vaapi", "-rc_mode", "CQP", "-qp", "18",
		"-frames:v", "2",
		"-f", "null", "-",
	}
	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	return cmd.Run() == nil
}

func scaleVFSharp(maxHeight int) string {
	// lanczos + accurate chroma for sharpness on downscales / high-quality sources.
	return fmt.Sprintf(
		"scale=-2:%d:flags=lanczos+accurate_rnd+full_chroma_int:force_original_aspect_ratio=decrease,scale=trunc(iw/2)*2:trunc(ih/2)*2",
		maxHeight,
	)
}

func scaleVFVAAPI(maxHeight int) string {
	return fmt.Sprintf(
		"scale=-2:%d:flags=lanczos+accurate_rnd+full_chroma_int:force_original_aspect_ratio=decrease,scale=trunc(iw/2)*2:trunc(ih/2)*2,format=nv12,hwupload",
		maxHeight,
	)
}

func bitrateMaxRate(br string) string {
	bps := parseBitrateString(br)
	if bps <= 0 {
		return br
	}
	return formatBitrateKbps(bps + bps/2)
}

func bitrateBufSize(br string) string {
	bps := parseBitrateString(br)
	if bps <= 0 {
		return br
	}
	return formatBitrateKbps(bps * 2)
}

func buildCPUEncodeArgs(inputPath, variantDir string, maxH int, br, levelStr, preset string) []string {
	return []string{
		"-y", "-i", inputPath,
		"-vf", scaleVFSharp(maxH),
		// CRF giữ độ sắc nét; maxrate/bufsize chỉ trần VBV (cho phép file lớn).
		"-c:v", "libx264", "-preset", preset, "-crf", "16",
		"-maxrate", bitrateMaxRate(br), "-bufsize", bitrateBufSize(br),
		"-profile:v", "high", "-level:v", levelStr,
		"-sc_threshold", "0",
		"-force_key_frames", "expr:gte(t,n_forced*6)",
		"-c:a", "aac", "-b:a", "192k", "-ac", "2",
		"-f", "hls",
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_flags", "independent_segments",
		"-hls_segment_filename", filepath.Join(variantDir, "segment_%05d.ts"),
		filepath.Join(variantDir, "index.m3u8"),
	}
}

func buildVAAPIEncodeArgs(inputPath, variantDir, device string, maxH int, br, levelStr string, qp int) []string {
	if device == "" {
		device = "/dev/dri/renderD128"
	}
	if qp <= 0 {
		qp = 17
	}
	return []string{
		"-y",
		"-init_hw_device", "vaapi=va:" + device,
		"-filter_hw_device", "va",
		"-i", inputPath,
		"-vf", scaleVFVAAPI(maxH),
		"-c:v", "h264_vaapi",
		"-rc_mode", "CQP", "-qp", fmt.Sprintf("%d", qp),
		// Bitrate hints + VBV keep size from collapsing on drivers that blend modes.
		"-b:v", br, "-maxrate", bitrateMaxRate(br), "-bufsize", bitrateBufSize(br),
		"-profile:v", "high", "-level", levelStr,
		"-force_key_frames", "expr:gte(t,n_forced*6)",
		"-c:a", "aac", "-b:a", "192k", "-ac", "2",
		"-f", "hls",
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_flags", "independent_segments",
		"-hls_segment_filename", filepath.Join(variantDir, "segment_%05d.ts"),
		filepath.Join(variantDir, "index.m3u8"),
	}
}

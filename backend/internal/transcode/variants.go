package transcode

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// HLSVariant is one renditions in a multi-bitrate HLS output.
type HLSVariant struct {
	Name      string
	Height    int
	Bitrate   string // ffmpeg -b:v value e.g. "2800k"
	TargetBps int64  // nominal ladder cap in bits/s
}

// SourceProfile describes input quality from probe / DB (0 = unknown).
type SourceProfile struct {
	Width   int
	Height  int
	Bitrate int64   // bits per second (container/video stream)
	FPS     float64 // frames per second (0 = unknown -> assume 30)
}

// KnownVariants lists renditions the platform supports (highest first).
// Home-lab quality ladder: 2K / 1080p / 720p only (no SD).
var KnownVariants = []HLSVariant{
	{Name: "1440p", Height: 1440, Bitrate: "14000k", TargetBps: 14_000_000},
	{Name: "1080p", Height: 1080, Bitrate: "10000k", TargetBps: 10_000_000},
	{Name: "720p", Height: 720, Bitrate: "6000k", TargetBps: 6_000_000},
}

const sourceHeightSlack = 16

// qualityBitrateBoost keeps encode bitrate above measured source so VAAPI/CPU
// do not crush detail (user accepts larger HLS output).
const qualityBitrateBoost = 1.45

var variantsByName map[string]HLSVariant

func init() {
	variantsByName = make(map[string]HLSVariant, len(KnownVariants))
	for _, v := range KnownVariants {
		variantsByName[v.Name] = v
	}
}

// VariantNames returns supported variant ids for API/docs.
func VariantNames() []string {
	out := make([]string, len(KnownVariants))
	for i, v := range KnownVariants {
		out[i] = v.Name
	}
	return out
}

// VariantFitsSource: chỉ chặn upscale độ phân giải (không chặn theo bitrate ladder).
func VariantFitsSource(v HLSVariant, src SourceProfile) bool {
	if src.Height > 0 && v.Height > src.Height+sourceHeightSlack {
		return false
	}
	return true
}

func variantRejectReason(v HLSVariant, src SourceProfile) string {
	if src.Height > 0 && v.Height > src.Height+sourceHeightSlack {
		return fmt.Sprintf("variant %s exceeds source height %dpx", v.Name, src.Height)
	}
	return ""
}

// ResolveVariants picks renditions to encode. requested empty means all heights that fit the source.
func ResolveVariants(requested []string, src SourceProfile) ([]HLSVariant, error) {
	if len(requested) == 0 {
		return defaultVariantsForSource(src), nil
	}
	seen := make(map[string]struct{}, len(requested))
	var out []HLSVariant
	var skipped []string
	for _, name := range requested {
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		v, ok := variantsByName[name]
		if !ok {
			return nil, fmt.Errorf("unknown variant %q (allowed: %v)", name, VariantNames())
		}
		if reason := variantRejectReason(v, src); reason != "" {
			skipped = append(skipped, reason)
			continue
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		if len(skipped) > 0 {
			return nil, errors.New(skipped[0])
		}
		return nil, errors.New("at least one variant is required")
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Height > out[j].Height })
	return out, nil
}

func defaultVariantsForSource(src SourceProfile) []HLSVariant {
	var out []HLSVariant
	for _, v := range KnownVariants {
		if VariantFitsSource(v, src) {
			out = append(out, v)
		}
	}
	if len(out) > 0 {
		return out
	}
	if src.Height <= 0 && src.Bitrate <= 0 {
		dup := make([]HLSVariant, len(KnownVariants))
		copy(dup, KnownVariants)
		return dup
	}
	h := src.Height
	if h <= 0 {
		h = 720
	}
	if h%2 != 0 {
		h--
	}
	br := EncodeBitrate(HLSVariant{Height: h, TargetBps: 6_000_000, Bitrate: "6000k"}, src)
	return []HLSVariant{{
		Name:      "source",
		Height:    h,
		Bitrate:   br,
		TargetBps: parseBitrateString(br),
	}}
}

// EncodeBitrate picks -b:v from source measured bitrate and rendition height (area ratio).
// Prefers generous bitrate (quality / sharpness) over small files.
func EncodeBitrate(v HLSVariant, src SourceProfile) string {
	renditionH := v.Height
	if src.Height > 0 && src.Height < renditionH {
		renditionH = src.Height
	}
	if renditionH%2 != 0 {
		renditionH--
	}

	if src.Bitrate <= 0 || src.Height <= 0 {
		return v.Bitrate
	}

	areaRatio := float64(renditionH*renditionH) / float64(src.Height*src.Height)
	if areaRatio > 1 {
		areaRatio = 1
	}
	target := int64(float64(src.Bitrate) * areaRatio * qualityBitrateBoost)

	floor := bitrateFloorForHeight(renditionH)
	if target < floor {
		target = floor
	}
	if v.TargetBps > 0 && target > v.TargetBps {
		target = v.TargetBps
	}
	// Không cắt theo bitrate nguồn — chấp nhận file lớn để giữ độ sắc nét.
	return formatBitrateKbps(target)
}

func bitrateFloorForHeight(h int) int64 {
	switch {
	case h >= 1440:
		return 8_000_000
	case h >= 1080:
		return 5_000_000
	case h >= 720:
		return 3_000_000
	default:
		return 2_000_000
	}
}

func formatBitrateKbps(bps int64) string {
	kbps := bps / 1000
	if kbps < 400 {
		kbps = 400
	}
	return strconv.FormatInt(kbps, 10) + "k"
}

func parseBitrateString(s string) int64 {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0
	}
	mult := int64(1)
	if strings.HasSuffix(s, "k") {
		mult = 1000
		s = strings.TrimSuffix(s, "k")
	} else if strings.HasSuffix(s, "m") {
		mult = 1_000_000
		s = strings.TrimSuffix(s, "m")
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n * mult
}

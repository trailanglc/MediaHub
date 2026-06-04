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
	Height  int
	Bitrate int64 // bits per second (container/video stream)
}

// KnownVariants lists renditions the platform supports (highest first).
var KnownVariants = []HLSVariant{
	{Name: "1080p", Height: 1080, Bitrate: "5000k", TargetBps: 5_000_000},
	{Name: "720p", Height: 720, Bitrate: "2800k", TargetBps: 2_800_000},
	{Name: "480p", Height: 480, Bitrate: "1400k", TargetBps: 1_400_000},
}

const sourceHeightSlack = 16

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
// Video YouTube 1080p thường ~1.5–2.5 Mbps — vẫn cho encode 1080p/720p với bitrate scale theo nguồn.
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
		h = 480
	}
	if h%2 != 0 {
		h--
	}
	br := EncodeBitrate(HLSVariant{Height: h, TargetBps: 1_400_000}, src)
	return []HLSVariant{{
		Name:      "source",
		Height:    h,
		Bitrate:   br,
		TargetBps: parseBitrateString(br),
	}}
}

// EncodeBitrate picks -b:v from source measured bitrate and rendition height (area ratio), not fixed ladder caps.
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
	target := int64(float64(src.Bitrate) * areaRatio * 1.1)

	floor := bitrateFloorForHeight(renditionH)
	if target < floor {
		target = floor
	}
	if v.TargetBps > 0 && target > v.TargetBps {
		target = v.TargetBps
	}
	// Cùng độ phân giải nguồn: không vượt bitrate đo được (tránh file phình không thêm chi tiết).
	if renditionH >= src.Height-sourceHeightSlack && target > src.Bitrate {
		target = src.Bitrate
	}
	return formatBitrateKbps(target)
}

func bitrateFloorForHeight(h int) int64 {
	switch {
	case h >= 1080:
		return 1_800_000
	case h >= 720:
		return 1_000_000
	default:
		return 550_000
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

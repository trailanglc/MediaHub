package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var ErrStreamTokenInvalid = errors.New("invalid stream token")

type StreamTokenService struct {
	secret []byte
}

func NewStreamTokenService(secret string) *StreamTokenService {
	return &StreamTokenService{secret: []byte(secret)}
}

// Sign creates a session token bound to videoPublicID and expiry (not per-segment).
func (s *StreamTokenService) Sign(videoPublicID string, exp time.Time) string {
	payload := fmt.Sprintf("%s|%d", videoPublicID, exp.Unix())
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Verify checks a session token for the given video and expiry.
func (s *StreamTokenService) Verify(videoPublicID, token string, expUnix int64) error {
	if len(s.secret) == 0 || token == "" || expUnix <= 0 {
		return ErrStreamTokenInvalid
	}
	if time.Now().Unix() > expUnix {
		return ErrStreamTokenInvalid
	}
	expected := s.Sign(videoPublicID, time.Unix(expUnix, 0))
	if !hmac.Equal([]byte(expected), []byte(token)) {
		return ErrStreamTokenInvalid
	}
	return nil
}

// BuildStreamURL returns the master playlist entry URL with session token query params.
func (s *StreamTokenService) BuildStreamURL(baseURL, videoPublicID, resourcePath string, ttl time.Duration) string {
	exp := time.Now().Add(ttl).Unix()
	token := s.Sign(videoPublicID, time.Unix(exp, 0))
	u, _ := url.Parse(strings.TrimRight(baseURL, "/"))
	u.Path = strings.TrimRight(u.Path, "/") + "/stream/" + videoPublicID + "/" + strings.TrimLeft(resourcePath, "/")
	q := u.Query()
	q.Set("token", token)
	q.Set("exp", strconv.FormatInt(exp, 10))
	u.RawQuery = q.Encode()
	return u.String()
}

// SegmentExpiry returns a window-aligned expiry shared by all viewers within the same
// time bucket. Aligning to window boundaries keeps the signed segment URL identical for
// every viewer in that window, so a CDN/edge caches each segment once and fans it out.
// Validity is between 1x and 2x the window so segments referenced near a boundary stay valid.
func SegmentExpiry(now time.Time, window time.Duration) int64 {
	w := int64(window / time.Second)
	if w <= 0 {
		w = 3600
	}
	return (now.Unix()/w + 2) * w
}

// SignSegment creates a shared (per-video, per-window) signature for segment URLs.
// It is namespaced with a "seg" prefix so it can never be confused with a session token.
func (s *StreamTokenService) SignSegment(videoPublicID string, expUnix int64) string {
	payload := fmt.Sprintf("seg|%s|%d", videoPublicID, expUnix)
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// VerifySegment validates a shared segment signature for the given video and expiry.
func (s *StreamTokenService) VerifySegment(videoPublicID, sig string, expUnix int64) error {
	if len(s.secret) == 0 || sig == "" || expUnix <= 0 {
		return ErrStreamTokenInvalid
	}
	if time.Now().Unix() > expUnix {
		return ErrStreamTokenInvalid
	}
	expected := s.SignSegment(videoPublicID, expUnix)
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return ErrStreamTokenInvalid
	}
	return nil
}

// SignAsset creates a shared (per-object, per-variant, per-window) delivery signature.
func (s *StreamTokenService) SignAsset(objectPublicID, variant string, expUnix int64) string {
	payload := fmt.Sprintf("asset|%s|%s|%d", objectPublicID, variant, expUnix)
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// VerifyAsset validates a shared asset delivery signature.
func (s *StreamTokenService) VerifyAsset(objectPublicID, variant string, expUnix int64, sig string) error {
	if len(s.secret) == 0 || sig == "" || expUnix <= 0 {
		return ErrStreamTokenInvalid
	}
	if time.Now().Unix() > expUnix {
		return ErrStreamTokenInvalid
	}
	expected := s.SignAsset(objectPublicID, variant, expUnix)
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return ErrStreamTokenInvalid
	}
	return nil
}

// BuildAssetURL returns a signed delivery URL for an object variant.
func (s *StreamTokenService) BuildAssetURL(baseURL, objectPublicID, variant string, window time.Duration) string {
	exp := SegmentExpiry(time.Now(), window)
	sig := s.SignAsset(objectPublicID, variant, exp)
	u, _ := url.Parse(strings.TrimRight(baseURL, "/"))
	u.Path = strings.TrimRight(u.Path, "/") + "/assets/" + objectPublicID + "/" + variant
	q := u.Query()
	q.Set("e", strconv.FormatInt(exp, 10))
	q.Set("s", sig)
	u.RawQuery = q.Encode()
	return u.String()
}

// BuildEmbedURL returns a signed embed player URL for a video object.
func (s *StreamTokenService) BuildEmbedURL(baseURL, videoPublicID string, window time.Duration) string {
	return s.BuildAssetURL(baseURL, videoPublicID, "embed", window)
}

// HashDomain returns a stable hash for optional domain binding.
func HashDomain(domain string) string {
	d := strings.ToLower(strings.TrimSpace(domain))
	h := sha256.Sum256([]byte(d))
	return hex.EncodeToString(h[:8])
}

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

// HashDomain returns a stable hash for optional domain binding.
func HashDomain(domain string) string {
	d := strings.ToLower(strings.TrimSpace(domain))
	h := sha256.Sum256([]byte(d))
	return hex.EncodeToString(h[:8])
}

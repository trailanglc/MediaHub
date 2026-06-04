package service

import (
	"strings"
	"testing"
	"time"
)

func TestStreamTokenSignVerify(t *testing.T) {
	svc := NewStreamTokenService("test-secret")
	exp := time.Now().Add(time.Hour).Unix()
	token := svc.Sign("video-uuid", time.Unix(exp, 0))
	if err := svc.Verify("video-uuid", token, exp); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := svc.Verify("video-uuid", token, exp-7200); err == nil {
		t.Fatal("expected expired token to fail")
	}
	if err := svc.Verify("other", token, exp); err == nil {
		t.Fatal("expected wrong video id to fail")
	}
}

func TestBuildStreamURL(t *testing.T) {
	svc := NewStreamTokenService("test-secret")
	u := svc.BuildStreamURL("http://localhost:8080", "vid", "master.m3u8", time.Hour)
	if u == "" {
		t.Fatal("empty url")
	}
	for _, sub := range []string{"token=", "exp=", "master.m3u8"} {
		if !stringsContains(u, sub) {
			t.Fatalf("url missing %q: %s", sub, u)
		}
	}
}

func TestSegmentSignVerify(t *testing.T) {
	svc := NewStreamTokenService("test-secret")
	exp := SegmentExpiry(time.Now(), time.Hour)
	sig := svc.SignSegment("video-uuid", exp)
	if err := svc.VerifySegment("video-uuid", sig, exp); err != nil {
		t.Fatalf("verify segment: %v", err)
	}
	if err := svc.VerifySegment("other", sig, exp); err == nil {
		t.Fatal("expected wrong video id to fail")
	}
	if err := svc.VerifySegment("video-uuid", sig, exp-100000); err == nil {
		t.Fatal("expected expired signature to fail")
	}
	// A session token must never validate as a segment signature (namespace isolation).
	sessTok := svc.Sign("video-uuid", time.Unix(exp, 0))
	if err := svc.VerifySegment("video-uuid", sessTok, exp); err == nil {
		t.Fatal("expected session token to be rejected as segment signature")
	}
}

func TestSegmentExpiryShared(t *testing.T) {
	now := time.Now()
	a := SegmentExpiry(now, time.Hour)
	b := SegmentExpiry(now.Add(30*time.Second), time.Hour)
	if a != b {
		t.Fatalf("viewers in the same window must share expiry: %d != %d", a, b)
	}
	if a <= now.Unix() {
		t.Fatalf("expiry must be in the future: %d <= %d", a, now.Unix())
	}
	if a-now.Unix() < int64(time.Hour/time.Second) {
		t.Fatalf("expiry must grant at least one window of validity")
	}
}

func TestAssetSignVerify(t *testing.T) {
	svc := NewStreamTokenService("test-secret")
	exp := SegmentExpiry(time.Now(), 24*time.Hour)
	sig := svc.SignAsset("obj-uuid", "image", exp)
	if err := svc.VerifyAsset("obj-uuid", "image", exp, sig); err != nil {
		t.Fatalf("verify asset: %v", err)
	}
	if err := svc.VerifyAsset("obj-uuid", "thumbnail", exp, sig); err == nil {
		t.Fatal("expected variant mismatch to fail")
	}
}

func stringsContains(s, sub string) bool {
	return strings.Index(s, sub) >= 0
}

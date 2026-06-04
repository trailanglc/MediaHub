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

func stringsContains(s, sub string) bool {
	return strings.Index(s, sub) >= 0
}

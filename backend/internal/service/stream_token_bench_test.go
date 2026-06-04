package service

import (
	"testing"
	"time"
)

func BenchmarkStreamTokenSign(b *testing.B) {
	svc := NewStreamTokenService("bench-secret-0123456789")
	exp := time.Now().Add(time.Hour)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = svc.Sign("550e8400-e29b-41d4-a716-446655440000", exp)
	}
}

func BenchmarkStreamTokenVerify(b *testing.B) {
	svc := NewStreamTokenService("bench-secret-0123456789")
	expUnix := time.Now().Add(time.Hour).Unix()
	tok := svc.Sign("550e8400-e29b-41d4-a716-446655440000", time.Unix(expUnix, 0))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = svc.Verify("550e8400-e29b-41d4-a716-446655440000", tok, expUnix)
	}
}

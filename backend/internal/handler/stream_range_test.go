package handler

import "testing"

func TestParseByteRange(t *testing.T) {
	size := int64(1000)
	start, end, ok := parseByteRange("bytes=0-99", size)
	if !ok || start != 0 || end != 99 {
		t.Fatalf("got %d-%d ok=%v", start, end, ok)
	}
	start, end, ok = parseByteRange("bytes=500-", size)
	if !ok || start != 500 || end != 999 {
		t.Fatalf("open range: got %d-%d", start, end)
	}
	_, _, ok = parseByteRange("bytes=2000-", size)
	if ok {
		t.Fatal("expected invalid range")
	}
}

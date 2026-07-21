package utils

import "testing"

func TestUploadPlan(t *testing.T) {
	mode, chunk, count := UploadPlan(40 << 20)
	if mode != 1 || count != 1 || chunk != 40<<20 {
		t.Fatalf("single plan: %d %d %d", mode, chunk, count)
	}
	mode, chunk, count = UploadPlan(10 << 30)
	if mode != 2 || chunk != 16<<20 || count != 640 {
		t.Fatalf("multipart plan: %d %d %d", mode, chunk, count)
	}
	_, _, count = UploadPlan(200 << 40)
	if count > int(MaxParts) {
		t.Fatalf("too many parts: %d", count)
	}
}

func TestNormalizeFolder(t *testing.T) {
	if got := NormalizeFolder(`research\\ndvi`); got != "/research/ndvi/" {
		t.Fatalf("got %q", got)
	}
}

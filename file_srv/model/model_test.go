package model

import "testing"

func TestFinalizingStatusIsDistinct(t *testing.T) {
	if UploadFinalizing == UploadComplete || UploadFinalizing == UploadCancelled {
		t.Fatal("finalizing status must not overlap terminal states")
	}
}

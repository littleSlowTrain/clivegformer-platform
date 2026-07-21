package utils

import (
	"testing"
	"time"
)

func TestDownloadTicket(t *testing.T) {
	value, err := SignDownloadTicket("secret", 4, 9, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := VerifyDownloadTicket("secret", value)
	if err != nil || ticket.UserID != 4 || ticket.FileID != 9 {
		t.Fatalf("ticket=%#v err=%v", ticket, err)
	}
}
func TestParseRange(t *testing.T) {
	start, end, partial, err := ParseRange("bytes=10-19", 100)
	if err != nil || !partial || start != 10 || end != 19 {
		t.Fatalf("%d %d %v %v", start, end, partial, err)
	}
}

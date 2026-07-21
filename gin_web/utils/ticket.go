package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type DownloadTicket struct {
	UserID  uint64 `json:"uid"`
	FileID  uint64 `json:"fid"`
	Expires int64  `json:"exp"`
}

func SignDownloadTicket(secret string, userID, fileID uint64, ttl time.Duration) (string, error) {
	payload, err := json.Marshal(DownloadTicket{UserID: userID, FileID: fileID, Expires: time.Now().Add(ttl).Unix()})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func VerifyDownloadTicket(secret, value string) (DownloadTicket, error) {
	var ticket DownloadTicket
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return ticket, errors.New("invalid ticket")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ticket, err
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return ticket, errors.New("invalid signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ticket, err
	}
	if err = json.Unmarshal(payload, &ticket); err != nil {
		return ticket, err
	}
	if ticket.Expires < time.Now().Unix() {
		return ticket, errors.New("expired ticket")
	}
	return ticket, nil
}

func ParseRange(value string, size int64) (start, end int64, partial bool, err error) {
	if value == "" {
		return 0, size - 1, false, nil
	}
	if !strings.HasPrefix(value, "bytes=") {
		return 0, 0, false, errors.New("invalid range")
	}
	spec := strings.TrimPrefix(value, "bytes=")
	if strings.Contains(spec, ",") {
		return 0, 0, false, errors.New("multiple ranges unsupported")
	}
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false, errors.New("invalid range")
	}
	if parts[0] == "" {
		suffix, e := strconv.ParseInt(parts[1], 10, 64)
		if e != nil || suffix <= 0 {
			return 0, 0, false, errors.New("invalid suffix")
		}
		if suffix > size {
			suffix = size
		}
		return size - suffix, size - 1, true, nil
	}
	start, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil || start < 0 || start >= size {
		return 0, 0, false, errors.New("range outside object")
	}
	end = size - 1
	if parts[1] != "" {
		end, e = strconv.ParseInt(parts[1], 10, 64)
		if e != nil || end < start {
			return 0, 0, false, errors.New("invalid range end")
		}
		if end >= size {
			end = size - 1
		}
	}
	return start, end, true, nil
}
func ContentRange(start, end, size int64) string {
	return fmt.Sprintf("bytes %d-%d/%d", start, end, size)
}

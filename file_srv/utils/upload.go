package utils

import (
	"fmt"
	"math"
	"path"
	"regexp"
	"strings"
)

const (
	SingleThreshold int64 = 64 << 20
	DefaultChunk    int64 = 16 << 20
	MaxParts        int64 = 9500
)

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func ValidHash(value string) bool { return sha256Pattern.MatchString(value) }

func UploadPlan(size int64) (mode int8, chunkSize int, chunkCount int) {
	if size <= SingleThreshold {
		return 1, int(size), 1
	}
	chunk := DefaultChunk
	if count := int64(math.Ceil(float64(size) / float64(chunk))); count > MaxParts {
		needed := int64(math.Ceil(float64(size) / float64(MaxParts)))
		chunk = int64(math.Ceil(float64(needed)/(1<<20))) * (1 << 20)
	}
	return 2, int(chunk), int((size + chunk - 1) / chunk)
}

func ObjectKey(hash string) string { return fmt.Sprintf("sha256/%s/%s/%s", hash[:2], hash[2:4], hash) }

func NormalizeFolder(folder string) string {
	folder = strings.TrimSpace(strings.ReplaceAll(folder, "\\", "/"))
	if folder == "" {
		return "/"
	}
	clean := path.Clean("/" + folder)
	if !strings.HasSuffix(clean, "/") && clean != "/" {
		clean += "/"
	}
	return clean
}

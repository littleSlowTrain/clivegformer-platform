package utils

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var regionCodePattern = regexp.MustCompile(`^[0-9]{1,32}$`)

type ScientificMetadata struct {
	RegionCode string
	BlockIndex uint32
	DataYear   uint32
	Classified bool
}

func ParseScientificFilename(filename string) ScientificMetadata {
	name := strings.TrimSpace(filepath.Base(filename))
	extension := filepath.Ext(name)
	if extension == "" {
		return ScientificMetadata{}
	}
	parts := strings.Split(strings.TrimSuffix(name, extension), "_")
	if len(parts) != 4 || !regionCodePattern.MatchString(parts[0]) {
		return ScientificMetadata{}
	}
	block, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return ScientificMetadata{}
	}
	start, err := time.Parse("20060102", parts[2])
	if err != nil {
		return ScientificMetadata{}
	}
	end, err := time.Parse("20060102", parts[3])
	if err != nil || end.Before(start) || start.Year() != end.Year() {
		return ScientificMetadata{}
	}
	return ScientificMetadata{RegionCode: parts[0], BlockIndex: uint32(block), DataYear: uint32(start.Year()), Classified: true}
}

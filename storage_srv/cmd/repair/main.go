package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/clivegformer/platform/storage_srv/config"
	"github.com/clivegformer/platform/storage_srv/initialize"
	"github.com/minio/minio-go/v7"
)

func main() {
	filePath := flag.String("file", "", "local source file")
	expectedHash := flag.String("expected-hash", "", "expected lowercase SHA-256")
	flag.Parse()
	if *filePath == "" || len(*expectedHash) != 64 {
		log.Fatal("-file and a 64-character -expected-hash are required")
	}
	absolutePath, err := filepath.Abs(*filePath)
	if err != nil {
		log.Fatal(err)
	}
	file, err := os.Open(absolutePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		log.Fatal("source must be a regular file")
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		log.Fatal(err)
	}
	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actualHash, *expectedHash) {
		log.Fatalf("source SHA-256 mismatch: got %s", actualHash)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		log.Fatal(err)
	}

	cfg := config.Load()
	client, _, err := initialize.Ceph(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey, cfg.Region, cfg.Secure)
	if err != nil {
		log.Fatal(err)
	}
	objectKey := fmt.Sprintf("sha256/%s/%s/%s", actualHash[:2], actualHash[2:4], actualHash)
	ctx := context.Background()
	if existing, statErr := client.StatObject(ctx, cfg.Bucket, objectKey, minio.StatObjectOptions{}); statErr == nil {
		if existing.Size != info.Size() {
			log.Fatalf("existing object has unexpected size: %d/%d", existing.Size, info.Size())
		}
		fmt.Printf("object already healthy: %s/%s (%d bytes)\n", cfg.Bucket, objectKey, existing.Size)
		return
	}
	if _, err := client.PutObject(ctx, cfg.Bucket, objectKey, file, info.Size(), minio.PutObjectOptions{ContentType: "application/x-netcdf"}); err != nil {
		log.Fatal(err)
	}
	repaired, err := client.StatObject(ctx, cfg.Bucket, objectKey, minio.StatObjectOptions{})
	if err != nil || repaired.Size != info.Size() {
		log.Fatalf("object verification failed after repair: %v", err)
	}
	fmt.Printf("repaired object: %s/%s (%d bytes)\n", cfg.Bucket, objectKey, repaired.Size)
}

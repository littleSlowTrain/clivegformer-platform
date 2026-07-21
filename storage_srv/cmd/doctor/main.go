package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/clivegformer/platform/storage_srv/config"
	"github.com/clivegformer/platform/storage_srv/initialize"
	"github.com/minio/minio-go/v7"
)

func main() {
	cfg := config.Load()
	client, _, err := initialize.Ceph(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey, cfg.Region, cfg.Secure)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		log.Fatalf("ceph authentication/readiness failed: %v", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil && !strings.Contains(strings.ToLower(err.Error()), "exist") {
			log.Fatalf("create bucket: %v", err)
		}
	}
	object := ".clivegformer-doctor"
	payload := "storage-ready"
	if _, err := client.PutObject(ctx, cfg.Bucket, object, strings.NewReader(payload), int64(len(payload)), minio.PutObjectOptions{}); err != nil {
		log.Fatalf("write probe: %v", err)
	}
	if err := client.RemoveObject(ctx, cfg.Bucket, object, minio.RemoveObjectOptions{}); err != nil {
		log.Fatalf("remove probe: %v", err)
	}
	fmt.Printf("Ceph RGW ready: %s/%s\n", cfg.Endpoint, cfg.Bucket)
}

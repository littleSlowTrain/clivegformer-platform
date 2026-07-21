package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/clivegformer/platform/storage_srv/config"
	"github.com/clivegformer/platform/storage_srv/initialize"
	"github.com/minio/minio-go/v7"
)

const (
	expectedEndpoint = "192.168.10.130:80"
	expectedBucket   = "clivegformer-data"
	confirmation     = "PURGE clivegformer-data AT 192.168.10.130:80"
)

func main() {
	confirm := flag.String("confirm", "", "required destructive-operation confirmation")
	verifyOnly := flag.Bool("verify-only", false, "only report object and incomplete multipart counts")
	flag.Parse()

	cfg := config.Load()
	if cfg.Endpoint != expectedEndpoint || cfg.Bucket != expectedBucket {
		log.Fatalf("refusing target endpoint=%q bucket=%q; expected endpoint=%q bucket=%q", cfg.Endpoint, cfg.Bucket, expectedEndpoint, expectedBucket)
	}
	if !*verifyOnly && *confirm != confirmation {
		log.Fatalf("confirmation must exactly equal %q", confirmation)
	}

	ctx := context.Background()
	client, _, err := initialize.Ceph(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey, cfg.Region, cfg.Secure)
	if err != nil {
		log.Fatal(err)
	}
	objects, incomplete, err := counts(ctx, client, cfg.Bucket)
	if err != nil {
		log.Fatal(err)
	}
	if *verifyOnly {
		fmt.Printf("endpoint=%s bucket=%s objects=%d incomplete_multipart=%d\n", cfg.Endpoint, cfg.Bucket, objects, incomplete)
		return
	}

	for upload := range client.ListIncompleteUploads(ctx, cfg.Bucket, "", true) {
		if upload.Err != nil {
			log.Fatalf("list incomplete multipart: %v", upload.Err)
		}
		if err := client.RemoveIncompleteUpload(ctx, cfg.Bucket, upload.Key); err != nil {
			log.Fatalf("abort incomplete multipart %q: %v", upload.Key, err)
		}
	}
	for object := range client.ListObjects(ctx, cfg.Bucket, minio.ListObjectsOptions{Recursive: true}) {
		if object.Err != nil {
			log.Fatalf("list object: %v", object.Err)
		}
		if err := client.RemoveObject(ctx, cfg.Bucket, object.Key, minio.RemoveObjectOptions{}); err != nil {
			log.Fatalf("remove object %q: %v", object.Key, err)
		}
	}

	objects, incomplete, err = counts(ctx, client, cfg.Bucket)
	if err != nil {
		log.Fatal(err)
	}
	if objects != 0 || incomplete != 0 {
		log.Fatalf("purge verification failed: objects=%d incomplete_multipart=%d", objects, incomplete)
	}
	fmt.Printf("purged endpoint=%s bucket=%s; objects=0 incomplete_multipart=0; bucket preserved\n", cfg.Endpoint, cfg.Bucket)
}

func counts(ctx context.Context, client *minio.Client, bucket string) (int, int, error) {
	objects := 0
	for object := range client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true}) {
		if object.Err != nil {
			return 0, 0, object.Err
		}
		objects++
	}
	incomplete := 0
	for upload := range client.ListIncompleteUploads(ctx, bucket, "", true) {
		if upload.Err != nil {
			return 0, 0, upload.Err
		}
		incomplete++
	}
	return objects, incomplete, nil
}

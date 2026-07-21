package config

import (
	"net/url"
	"os"
	"strings"
)

type Config struct {
	GRPCAddr, FileAddr, Endpoint, AccessKey, SecretKey, Bucket, Region, RocketMQ string
	Secure                                                                       bool
}

func Load() Config {
	raw := env("CEPH_ENDPOINT", "http://192.168.10.130:80")
	parsed, _ := url.Parse(raw)
	endpoint, secure := strings.TrimSuffix(raw, "/"), false
	if parsed.Host != "" {
		endpoint, secure = parsed.Host, parsed.Scheme == "https"
	}
	return Config{
		GRPCAddr: env("STORAGE_GRPC_ADDR", ":50053"), FileAddr: env("FILE_GRPC_ADDR", "127.0.0.1:50052"),
		Endpoint: endpoint, Secure: secure, AccessKey: os.Getenv("CEPH_ACCESS_KEY"), SecretKey: os.Getenv("CEPH_SECRET_KEY"),
		Bucket: env("CEPH_BUCKET", "clivegformer-data"), Region: env("CEPH_REGION", "us-east-1"), RocketMQ: env("ROCKETMQ_NAMESRV", "127.0.0.1:9876"),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

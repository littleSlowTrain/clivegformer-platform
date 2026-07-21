package config

import (
	"fmt"
	"os"
)

type Config struct {
	GRPCAddr, StorageAddr, RedisAddr, RedisPassword, RocketMQ, Bucket, DSN string
}

func Load() Config {
	host, port := env("MYSQL_HOST", "127.0.0.1"), env("MYSQL_PORT", "3307")
	db, user := env("MYSQL_DATABASE", "clivegformer"), env("MYSQL_USER", "clivegformer_app")
	pass := os.Getenv("MYSQL_PASSWORD")
	return Config{
		GRPCAddr: env("FILE_GRPC_ADDR", ":50052"), StorageAddr: env("STORAGE_GRPC_ADDR", "127.0.0.1:50053"),
		RedisAddr: env("REDIS_ADDR", "127.0.0.1:6379"), RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RocketMQ: env("ROCKETMQ_NAMESRV", "127.0.0.1:9876"), Bucket: env("CEPH_BUCKET", "clivegformer-data"),
		DSN: fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", user, pass, host, port, db),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

package config

import (
	"fmt"
	"os"
)

type Config struct {
	GRPCAddr  string
	JWTSecret string
	DSN       string
}

func Load() Config {
	host := env("MYSQL_HOST", "127.0.0.1")
	port := env("MYSQL_PORT", "3307")
	db := env("MYSQL_DATABASE", "clivegformer")
	user := env("MYSQL_USER", "clivegformer_app")
	pass := os.Getenv("MYSQL_PASSWORD")
	return Config{
		GRPCAddr:  env("USER_GRPC_ADDR", ":50051"),
		JWTSecret: env("JWT_SECRET", "development-only-change-me"),
		DSN:       fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", user, pass, host, port, db),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

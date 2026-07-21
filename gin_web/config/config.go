package config

import "os"

type Config struct{ HTTPAddr, UserAddr, FileAddr, StorageAddr, AnalysisAddr, JWTSecret, WebOrigin string }

func Load() Config {
	return Config{HTTPAddr: env("GIN_HTTP_ADDR", ":8080"), UserAddr: env("USER_GRPC_ADDR", "127.0.0.1:50051"), FileAddr: env("FILE_GRPC_ADDR", "127.0.0.1:50052"), StorageAddr: env("STORAGE_GRPC_ADDR", "127.0.0.1:50053"), AnalysisAddr: env("ANALYSIS_GRPC_ADDR", "127.0.0.1:50054"), JWTSecret: env("JWT_SECRET", "development-only-change-me"), WebOrigin: env("WEB_ORIGIN", "http://localhost:5173,http://127.0.0.1:5173")}
}
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

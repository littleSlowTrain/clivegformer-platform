package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

const (
	expectedDatabase = "clivegformer"
	confirmation     = "RESET clivegformer DATA AND PROJECT REDIS KEYS"
)

var projectPatterns = []string{
	"upload_session:*",
	"lock_hash:*",
	"upload_parts:*",
	"upload_waiters:*",
	"upload_heartbeat:*",
}

func main() {
	confirm := flag.String("confirm", "", "required destructive-operation confirmation")
	verifyOnly := flag.Bool("verify-only", false, "only report table rows and project Redis keys")
	flag.Parse()

	database := env("MYSQL_DATABASE", expectedDatabase)
	if database != expectedDatabase {
		log.Fatalf("refusing database %q; expected %q", database, expectedDatabase)
	}
	if !*verifyOnly && *confirm != confirmation {
		log.Fatalf("confirmation must exactly equal %q", confirmation)
	}

	dsn := fmt.Sprintf("root:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true", os.Getenv("MYSQL_ROOT_PASSWORD"), env("MYSQL_HOST", "127.0.0.1"), env("MYSQL_PORT", "3307"), database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}
	var selected string
	if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&selected); err != nil || selected != expectedDatabase {
		log.Fatalf("database identity check failed: selected=%q err=%v", selected, err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: env("REDIS_ADDR", "127.0.0.1:6379"), Password: os.Getenv("REDIS_PASSWORD")})
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal(err)
	}

	if *verifyOnly {
		report(ctx, db, rdb)
		return
	}
	var active int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM upload_session WHERE status IN (0,1,4,6)").Scan(&active); err != nil {
		log.Fatal(err)
	}
	if active != 0 {
		log.Fatalf("refusing reset: %d upload sessions are active, paused, or finalizing", active)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	for _, table := range []string{"analysis_result", "user_file", "upload_session", "file_object", "user"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM `"+table+"`"); err != nil {
			_ = tx.Rollback()
			log.Fatalf("clear %s: %v", table, err)
		}
	}
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
	for _, table := range []string{"analysis_result", "user_file", "upload_session", "file_object", "user"} {
		if _, err := db.ExecContext(ctx, "ALTER TABLE `"+table+"` AUTO_INCREMENT = 1"); err != nil {
			log.Fatalf("reset auto increment %s: %v", table, err)
		}
	}

	if err := rdb.Del(ctx, "all_file").Err(); err != nil {
		log.Fatal(err)
	}
	for _, pattern := range projectPatterns {
		if err := deletePattern(ctx, rdb, pattern); err != nil {
			log.Fatal(err)
		}
	}
	report(ctx, db, rdb)
}

func deletePattern(ctx context.Context, rdb *redis.Client, pattern string) error {
	var cursor uint64
	for {
		keys, next, err := rdb.Scan(ctx, cursor, pattern, 200).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := rdb.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

func report(ctx context.Context, db *sql.DB, rdb *redis.Client) {
	parts := make([]string, 0, 6)
	for _, table := range []string{"user", "file_object", "user_file", "upload_session", "analysis_result"} {
		var count int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM `"+table+"`").Scan(&count); err != nil {
			log.Fatal(err)
		}
		parts = append(parts, fmt.Sprintf("%s=%d", table, count))
	}
	redisCount := 0
	if exists, err := rdb.Exists(ctx, "all_file").Result(); err != nil {
		log.Fatal(err)
	} else {
		redisCount += int(exists)
	}
	for _, pattern := range projectPatterns {
		var cursor uint64
		for {
			keys, next, err := rdb.Scan(ctx, cursor, pattern, 200).Result()
			if err != nil {
				log.Fatal(err)
			}
			redisCount += len(keys)
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}
	fmt.Printf("database=%s %s project_redis_keys=%d\n", expectedDatabase, strings.Join(parts, " "), redisCount)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

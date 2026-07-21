package initialize

import (
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Database(dsn string) (*gorm.DB, error) { return gorm.Open(mysql.Open(dsn), &gorm.Config{}) }
func Redis(addr, password string) *redis.Client {
	return redis.NewClient(&redis.Options{Addr: addr, Password: password})
}

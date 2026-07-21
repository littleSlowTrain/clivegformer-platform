package initialize

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Database(dsn string) (*gorm.DB, error) { return gorm.Open(mysql.Open(dsn), &gorm.Config{}) }

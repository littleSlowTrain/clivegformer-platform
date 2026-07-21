package model

import "time"

type User struct {
	ID           uint64 `gorm:"primaryKey"`
	Username     string `gorm:"size:64;uniqueIndex;not null"`
	PasswordHash string `gorm:"size:255;not null"`
	Email        string `gorm:"size:255;uniqueIndex;not null"`
	Role         string `gorm:"size:32;not null;default:user"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (User) TableName() string { return "user" }

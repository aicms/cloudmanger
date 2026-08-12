package model

import "gorm.io/gorm"

// User 系统登录用户
type User struct {
	BaseModel
	Username     string `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string `gorm:"size:128;not null" json:"-"`
	DisplayName  string `gorm:"size:64" json:"display_name"`
	Email        string `gorm:"size:128" json:"email"`
	Role         string `gorm:"size:32;default:admin" json:"role"` // admin / viewer
	Status       string `gorm:"size:16;default:active" json:"status"`
}

// 用户登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
	User      User   `json:"user"`
}

// afterCreate 钩子：保证 gorm 与 gorm.DB 初始化不冲突
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.Role == "" {
		u.Role = "admin"
	}
	if u.Status == "" {
		u.Status = "active"
	}
	return nil
}

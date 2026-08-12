package auth

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrTokenInvalid = errors.New("token invalid")
	ErrTokenExpired = errors.New("token expired")
)

type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// JWTManager JWT 签发与解析
type JWTManager struct {
	secret      []byte
	expiresHours atomic.Int64
}

func NewJWTManager(secret string, expiresHours int) *JWTManager {
	m := &JWTManager{secret: []byte(secret)}
	m.expiresHours.Store(int64(expiresHours))
	return m
}

// SetSessionTimeout 动态更新令牌有效期（小时），供系统设置变更与启动初始化调用
func (m *JWTManager) SetSessionTimeout(hours int) {
	if hours < 1 {
		hours = 1
	}
	m.expiresHours.Store(int64(hours))
}

// Generate 生成 token
func (m *JWTManager) Generate(userID uint, username, role string) (string, error) {
	expires := time.Duration(m.expiresHours.Load()) * time.Hour
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expires)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// Parse 解析 token
func (m *JWTManager) Parse(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrTokenInvalid
		}
		return m.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}
	if !token.Valid {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}

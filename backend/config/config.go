package config

import (
	"encoding/json"
	"log"
	"os"
)

// Config 全局配置
type Config struct {
	Host       string
	Port       string
	DBPath     string
	JWTSecret  string
	AESKey     string // 用于加密云账号密钥
	IsDev      bool
}

// fileConfig 对应 data/config.json 中可选的覆盖项，字段留空表示不覆盖对应默认值
type fileConfig struct {
	Host      string `json:"host"`
	Port      string `json:"port"`
	DBPath    string `json:"db_path"`
	JWTSecret string `json:"jwt_secret"`
	AESKey    string `json:"aes_key"`
	GinMode   string `json:"gin_mode"` // debug / release
}

// configFilePath 系统配置文件路径：与数据库同目录，随包运行位置解析
const configFilePath = "./data/config.json"

// Load 加载配置，优先级从低到高依次覆盖：内置默认值（生产模式，端口 8002）
// → data/config.json（若存在） → 环境变量（若设置）
func Load() *Config {
	c := &Config{
		Host:      "0.0.0.0",
		Port:      "8002",
		DBPath:    "./data/app.db",
		JWTSecret: "cloudmanager-default-jwt-secret-change-me",
		AESKey:    "cloudmanager-aes-key-32bytes-ok!!!", // 必须 16/24/32 字节
		IsDev:     false,
	}

	if fc := loadFileConfig(configFilePath); fc != nil {
		if fc.Host != "" {
			c.Host = fc.Host
		}
		if fc.Port != "" {
			c.Port = fc.Port
		}
		if fc.DBPath != "" {
			c.DBPath = fc.DBPath
		}
		if fc.JWTSecret != "" {
			c.JWTSecret = fc.JWTSecret
		}
		if fc.AESKey != "" {
			c.AESKey = fc.AESKey
		}
		if fc.GinMode != "" {
			c.IsDev = fc.GinMode == "debug"
		}
	}

	c.Host = getEnv("HOST", c.Host)
	c.Port = getEnv("PORT", c.Port)
	c.DBPath = getEnv("DB_PATH", c.DBPath)
	c.JWTSecret = getEnv("JWT_SECRET", c.JWTSecret)
	c.AESKey = getEnv("AES_KEY", c.AESKey)
	if v := os.Getenv("GIN_MODE"); v != "" {
		c.IsDev = v == "debug"
	}

	return c
}

// loadFileConfig 读取 data/config.json；文件不存在时静默跳过，格式错误时告警并忽略
func loadFileConfig(path string) *fileConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		log.Printf("[warn] 解析 %s 失败，已忽略: %v", path, err)
		return nil
	}
	return &fc
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

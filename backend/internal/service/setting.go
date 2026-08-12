package service

import (
	"strconv"

	"cloudmanager/internal/repository"

	"gorm.io/gorm"
)

// 设置项键名
const (
	// SettingKeySyncConcurrency 批量同步并发上限
	SettingKeySyncConcurrency = "sync_concurrency"
	// SettingKeySyncInterval 自动同步间隔（分钟），0 表示关闭自动同步
	SettingKeySyncInterval = "sync_interval"
	// SettingKeySSHUser WebShell / 系统信息采集的默认 SSH 用户名
	SettingKeySSHUser = "ssh_user"
	// SettingKeySSHPort 默认 SSH 端口
	SettingKeySSHPort = "ssh_port"
	// SettingKeySessionTimeout JWT 会话有效期（小时）
	SettingKeySessionTimeout = "session_timeout"
	// SettingKeyPlatformName 平台展示名称（侧边栏 / 页头）
	SettingKeyPlatformName = "platform_name"
	// SettingKeyTheme 界面主题：light / dark
	SettingKeyTheme = "theme"
)

// SettingService 系统设置读写（键值对持久化）
type SettingService struct {
	repo *repository.SettingRepository
}

func NewSettingService(db *gorm.DB) *SettingService {
	return &SettingService{repo: repository.NewSettingRepository(db)}
}

// GetInt 读取整型设置，解析失败或不存在时返回默认值
func (s *SettingService) GetInt(key string, def int) int {
	v, err := s.repo.Get(key)
	if err != nil || v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// GetString 读取字符串设置，缺失或为空时返回默认值
func (s *SettingService) GetString(key, def string) string {
	v, err := s.repo.Get(key)
	if err != nil || v == "" {
		return def
	}
	return v
}

func (s *SettingService) Set(key, value string) error {
	return s.repo.Set(key, value)
}

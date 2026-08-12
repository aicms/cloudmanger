package handler

import (
	"strconv"
	"strings"

	"cloudmanager/internal/service"

	"github.com/gin-gonic/gin"
)

// GetSettings 系统设置（含全部可配置项）
func (h *H) GetSettings(c *gin.Context) {
	concurrency := h.Svc.Setting.GetInt(service.SettingKeySyncConcurrency, 3)
	platformName := h.Svc.Setting.GetString(service.SettingKeyPlatformName, "多云管理平台")
	JSON(c, gin.H{
		"system_name":          platformName,
		"platform_name":        platformName,
		"version":              "1.0.0",
		"sync_interval":        h.Svc.Setting.GetInt(service.SettingKeySyncInterval, 0),
		"sync_concurrency":     concurrency,
		"sync_concurrency_min": 1,
		"sync_concurrency_max": 20,
		"ssh_user":             h.Svc.Setting.GetString(service.SettingKeySSHUser, "root"),
		"ssh_port":             h.Svc.Setting.GetInt(service.SettingKeySSHPort, 22),
		"session_timeout":      h.Svc.Setting.GetInt(service.SettingKeySessionTimeout, 24),
		"theme":                h.Svc.Setting.GetString(service.SettingKeyTheme, "light"),
	})
}

// UpdateSettings 更新系统设置（支持全部配置项，按需校验边界）
func (h *H) UpdateSettings(c *gin.Context) {
	var req struct {
		SyncConcurrency *int    `json:"sync_concurrency"`
		SyncInterval    *int    `json:"sync_interval"`
		SSHUser         *string `json:"ssh_user"`
		SSHPort         *int    `json:"ssh_port"`
		SessionTimeout  *int    `json:"session_timeout"`
		PlatformName    *string `json:"platform_name"`
		Theme           *string `json:"theme"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		FailMsg(c, "参数错误")
		return
	}

	var changed []string

	if req.SyncConcurrency != nil {
		n := *req.SyncConcurrency
		if n < 1 || n > 20 {
			FailMsg(c, "批量同步并发数需在 1-20 之间")
			return
		}
		h.Svc.Setting.Set(service.SettingKeySyncConcurrency, strconv.Itoa(n))
		changed = append(changed, "sync_concurrency="+strconv.Itoa(n))
	}

	if req.SyncInterval != nil {
		n := *req.SyncInterval
		if n != 0 && (n < 5 || n > 1440) {
			FailMsg(c, "自动同步间隔需为 0（关闭）或 5-1440 分钟")
			return
		}
		h.Svc.Setting.Set(service.SettingKeySyncInterval, strconv.Itoa(n))
		changed = append(changed, "sync_interval="+strconv.Itoa(n))
	}

	if req.SSHUser != nil {
		u := strings.TrimSpace(*req.SSHUser)
		if u == "" {
			FailMsg(c, "SSH 默认用户名不能为空")
			return
		}
		h.Svc.Setting.Set(service.SettingKeySSHUser, u)
		changed = append(changed, "ssh_user="+u)
	}

	if req.SSHPort != nil {
		n := *req.SSHPort
		if n < 1 || n > 65535 {
			FailMsg(c, "SSH 端口需在 1-65535 之间")
			return
		}
		h.Svc.Setting.Set(service.SettingKeySSHPort, strconv.Itoa(n))
		changed = append(changed, "ssh_port="+strconv.Itoa(n))
	}

	if req.SessionTimeout != nil {
		n := *req.SessionTimeout
		if n < 1 || n > 720 {
			FailMsg(c, "会话超时需在 1-720 小时之间")
			return
		}
		h.Svc.Setting.Set(service.SettingKeySessionTimeout, strconv.Itoa(n))
		h.Svc.JWT.SetSessionTimeout(n)
		changed = append(changed, "session_timeout="+strconv.Itoa(n)+"h")
	}

	if req.PlatformName != nil {
		name := strings.TrimSpace(*req.PlatformName)
		if name == "" {
			FailMsg(c, "平台名称不能为空")
			return
		}
		h.Svc.Setting.Set(service.SettingKeyPlatformName, name)
		changed = append(changed, "platform_name="+name)
	}

	if req.Theme != nil {
		t := strings.TrimSpace(*req.Theme)
		if t != "light" && t != "dark" {
			FailMsg(c, "主题仅支持 light 或 dark")
			return
		}
		h.Svc.Setting.Set(service.SettingKeyTheme, t)
		changed = append(changed, "theme="+t)
	}

	if len(changed) > 0 {
		h.recordOp(c, "setting.update", "settings", "更新系统设置: "+strings.Join(changed, ", "), "success")
	}
	JSON(c, gin.H{"success": true})
}

package model

// 云厂商枚举
const (
	ProviderAliyun  = "aliyun"
	ProviderTencent = "tencent"
)

// CloudAccount 云账号（密钥加密存储）
type CloudAccount struct {
	BaseModel
	Name            string  `gorm:"size:64;not null" json:"name"`
	Provider        string  `gorm:"size:32;index;not null" json:"provider"` // aliyun / tencent
	AccessKeyID     string  `gorm:"size:128" json:"access_key_id"`
	AccessKeySecret string  `gorm:"size:256" json:"-"` // 加密存储，不返回
	Region          string  `gorm:"size:64" json:"region"`
	Remark          string  `gorm:"size:256" json:"remark"`
	Enabled         bool    `gorm:"default:true" json:"enabled"`
	LastSyncedAt    *string `gorm:"size:32" json:"last_synced_at"`
	SyncStatus      string  `gorm:"size:16" json:"sync_status"`   // ok / error，最近一次同步结果
	LastError       string  `gorm:"size:512" json:"last_error"`   // 最近一次同步/连接的错误信息
}

// 云账号展示用 DTO（脱敏密钥 + 资源计数）
type CloudAccountDTO struct {
	CloudAccount
	AccessKeyMasked string `json:"access_key_masked"`
	ServerCount     int64  `json:"server_count"`
	DomainCount     int64  `json:"domain_count"`
	RecordCount     int64  `json:"record_count"`
}

type CreateCloudAccountRequest struct {
	Name            string `json:"name" binding:"required"`
	Provider        string `json:"provider" binding:"required"`
	AccessKeyID     string `json:"access_key_id" binding:"required"`
	AccessKeySecret string `json:"access_key_secret"`
	Region          string `json:"region"`
	Remark          string `json:"remark"`
}

// SyncResult 一次同步的结果统计
type SyncResult struct {
	Servers int      `json:"servers"`
	Domains int      `json:"domains"`
	Records int      `json:"records"`
	Errors  []string `json:"errors"`
}

// BatchSyncAccountResult 单个账号的批量同步结果
type BatchSyncAccountResult struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Servers int    `json:"servers"`
	Domains int    `json:"domains"`
	Records int    `json:"records"`
}

// BatchSyncResult 批量同步的聚合结果
type BatchSyncResult struct {
	Total     int                      `json:"total"`
	Success   int                      `json:"success"`
	Failed    int                      `json:"failed"`
	Results   []BatchSyncAccountResult `json:"results"`
}

// ImportAccountResult CSV 导入中单个账号（行）的处理结果
type ImportAccountResult struct {
	Row      int    `json:"row"`
	Provider string `json:"provider"`
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}

// ImportCloudAccountsResult CSV 批量导入云账号的聚合结果
type ImportCloudAccountsResult struct {
	Total   int                   `json:"total"`
	Success int                   `json:"success"`
	Failed  int                   `json:"failed"`
	Results []ImportAccountResult `json:"results"`
}

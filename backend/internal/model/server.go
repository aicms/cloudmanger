package model

// Server 云服务器（同步缓存）
type Server struct {
	BaseModel
	CloudAccountID uint   `gorm:"index" json:"cloud_account_id"`
	Provider        string `gorm:"size:32;index" json:"provider"` // aliyun / tencent
	Category        string `gorm:"size:32;index" json:"category"` // cvm / lighthouse / ecs / swas
	InstanceID      string `gorm:"size:128;index" json:"instance_id"`
	Name            string `gorm:"size:128" json:"name"`
	CustomName      string `gorm:"size:128" json:"custom_name"` // 用户自定义名称，展示优先于 Name
	Status          string `gorm:"size:32;index" json:"status"` // Running / Stopped / Stopping / Starting
	Region          string `gorm:"size:64;index" json:"region"`
	Zone            string `gorm:"size:64" json:"zone"`
	CPU             int    `json:"cpu"`
	Memory          int    `json:"memory"` // MB
	OS              string `gorm:"size:128" json:"os"`
	InstanceType    string `gorm:"size:64" json:"instance_type"`
	PublicIP        string `gorm:"size:128" json:"public_ip"`
	InnerIP         string `gorm:"size:128" json:"inner_ip"`
	Bandwidth       int    `json:"bandwidth"` // Mbps
	Tags            string `gorm:"size:512" json:"tags"`
	ExpiryDate      string `gorm:"size:32;index" json:"expiry_date"` // 预付费到期日 YYYY-MM-DD
	SyncedAt        string `gorm:"size:32" json:"synced_at"`
}

type ServerQuery struct {
	PageQuery
	Provider string `form:"provider"`
	Region   string `form:"region"`
	Status   string `form:"status"`
	Tag      string `form:"tag"` // 按标签筛选（逗号分隔多个，OR 关系）
}

type ServerActionRequest struct {
	Action string `json:"action" binding:"required"` // start / stop / reboot
}

// ServerBatchActionRequest 按筛选条件批量操作（含标签）
type ServerBatchActionRequest struct {
	Action   string `json:"action" binding:"required"` // start / stop / reboot
	Provider string `json:"provider"`
	Region   string `json:"region"`
	Status   string `json:"status"`
	Keyword  string `json:"keyword"`
	Tag      string `json:"tag"`
}

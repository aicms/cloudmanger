package model

// DNSRecord 域名解析记录
type DNSRecord struct {
	BaseModel
	DomainID      uint   `gorm:"index" json:"domain_id"`
	DomainName    string `gorm:"size:128;index" json:"domain_name"`
	CloudAccountID uint  `gorm:"index" json:"cloud_account_id"`
	Provider      string `gorm:"size:32;index" json:"provider"`
	RecordID      string `gorm:"size:128" json:"record_id"`
	Type          string `gorm:"size:16;index" json:"type"` // A / AAAA / CNAME / MX / TXT / NS / SRV
	Host          string `gorm:"size:128" json:"host"`      // 主机记录，如 @ www
	Value         string `gorm:"size:512" json:"value"`      // 记录值
	TTL           int    `json:"ttl"`
	Priority      int    `json:"priority"` // MX 优先级
	Status        string `gorm:"size:16" json:"status"` // enable / disable
	SyncedAt      string `gorm:"size:32" json:"synced_at"`
}

type DNSRecordQuery struct {
	PageQuery
	DomainID uint   `form:"domain_id"`
	Domain   string `form:"domain"`
	Type     string `form:"type"`
	Provider string `form:"provider"`
}

type CreateDNSRecordRequest struct {
	DomainID  uint   `json:"domain_id" binding:"required"`
	Type      string `json:"type" binding:"required"`
	Host      string `json:"host" binding:"required"`
	Value     string `json:"value" binding:"required"`
	TTL       int    `json:"ttl"`
	Priority  int    `json:"priority"`
}

type UpdateDNSRecordRequest struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority"`
	Status   string `json:"status"`
}

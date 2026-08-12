package model

import "time"

// Domain 域名（同步缓存）
type Domain struct {
	BaseModel
	CloudAccountID    uint   `gorm:"index" json:"cloud_account_id"`
	Provider          string `gorm:"size:32;index" json:"provider"`
	DomainName        string `gorm:"size:128;index" json:"domain_name"`
	Registrar         string `gorm:"size:64" json:"registrar"`
	RegistrationDate  string `gorm:"size:32" json:"registration_date"`
	ExpiryDate        string `gorm:"size:32;index" json:"expiry_date"`
	DNSServers        string `gorm:"size:256" json:"dns_servers"`
	Status            string `gorm:"size:32" json:"status"` // ok / expired / expiring
	SyncedAt          string `gorm:"size:32" json:"synced_at"`
}

// DomainStatusExpiring 将到期阈值天数
const DomainStatusExpiring = 30

// ComputeDomainStatus 按到期日期实时计算域名状态
// expired: 已过期; expiring: DomainStatusExpiring 天内到期; ok: 正常
func ComputeDomainStatus(expiryDate string) string {
	if expiryDate == "" {
		return "ok"
	}
	t, err := time.Parse("2006-01-02", expiryDate)
	if err != nil {
		return "ok"
	}
	days := int(t.Sub(time.Now().Truncate(24*time.Hour)).Hours() / 24)
	switch {
	case days < 0:
		return "expired"
	case days <= DomainStatusExpiring:
		return "expiring"
	default:
		return "ok"
	}
}

type DomainQuery struct {
	PageQuery
	Provider string `form:"provider"`
	Status   string `form:"status"`
}

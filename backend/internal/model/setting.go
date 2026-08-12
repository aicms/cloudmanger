package model

// Setting 系统设置键值对（用于持久化可配置项，如批量同步并发数）
type Setting struct {
	Key   string `gorm:"primaryKey;size:64" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}

func (Setting) TableName() string { return "settings" }

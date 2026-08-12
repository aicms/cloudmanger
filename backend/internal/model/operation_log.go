package model

// OperationLog 操作审计日志
type OperationLog struct {
	BaseModel
	UserID uint   `gorm:"index" json:"user_id"`
	Username string `gorm:"size:64" json:"username"`
	Action  string `gorm:"size:64;index" json:"action"`   // login / server.start / dns.create ...
	Target  string `gorm:"size:128" json:"target"`        // 操作对象
	Detail  string `gorm:"size:512" json:"detail"`
	IP      string `gorm:"size:64" json:"ip"`
	Status  string `gorm:"size:16" json:"status"` // success / failed
}

type OperationLogQuery struct {
	PageQuery
	Action string `form:"action"`
}

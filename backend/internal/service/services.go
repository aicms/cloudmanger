package service

import (
	"cloudmanager/config"
	"cloudmanager/internal/auth"
	"cloudmanager/internal/repository"

	"gorm.io/gorm"
)

// Services 业务层容器，聚合所有 service 与依赖
type Services struct {
	DB        *gorm.DB
	Cfg       *config.Config
	JWT       *auth.JWTManager
	Encryptor *auth.AESEncryptor

	Auth         *AuthService
	Setting      *SettingService
	CloudAccount *CloudAccountService
	Server       *ServerService
	Domain       *DomainService
	DNSRecord    *DNSRecordService
	Dashboard    *DashboardService
	OperationLog *OperationLogService
}

// New 构建业务层并注入依赖
func New(cfg *config.Config, db *gorm.DB) *Services {
	jwtMgr := auth.NewJWTManager(cfg.JWTSecret, 24)
	enc := auth.NewAESEncryptor(cfg.AESKey)

	s := &Services{
		DB:        db,
		Cfg:       cfg,
		JWT:       jwtMgr,
		Encryptor: enc,
	}
	s.Auth = NewAuthService(db, jwtMgr)
	s.Setting = NewSettingService(db)
	s.CloudAccount = NewCloudAccountService(db, enc, s.Setting)
	s.Server = NewServerService(db, enc)
	s.Domain = NewDomainService(db)
	s.DNSRecord = NewDNSRecordService(db, enc)
	s.Dashboard = NewDashboardService(db)
	s.OperationLog = NewOperationLogService(db)

	// 用库内配置初始化会话超时（系统设置变更时也会实时更新）
	jwtMgr.SetSessionTimeout(s.Setting.GetInt(SettingKeySessionTimeout, 24))
	return s
}

// 避免未使用导入警告
var _ = repository.NewUserRepository

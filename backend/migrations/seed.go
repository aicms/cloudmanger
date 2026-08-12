package migrations

import (
	"log"

	"cloudmanager/config"
	"cloudmanager/internal/auth"
	"cloudmanager/internal/logging"
	"cloudmanager/internal/model"
	"cloudmanager/internal/service"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Seed 初始化种子数据：
//  1. 默认管理员账号 admin / admin123
//  2. 启动时同步已录入的云账号（云账号由用户在页面录入）
func Seed(db *gorm.DB, cfg *config.Config) error {
	// 1. 默认管理员
	var userCount int64
	db.Model(&model.User{}).Count(&userCount)
	if userCount == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		if err := db.Create(&model.User{
			Username:     "admin",
			PasswordHash: string(hash),
			DisplayName:  "管理员",
			Email:        "admin@cloudmanager.local",
			Role:         "admin",
			Status:       "active",
		}).Error; err != nil {
			return err
		}
		log.Println("✓ 已创建默认管理员 admin / admin123")
	}

	// 2. 云账号由用户在页面录入，不再种子示例账号
	enc := auth.NewAESEncryptor(cfg.AESKey)

	// 3. 启动时后台异步同步已录入的云账号（刷新本地资源缓存）
	// 多地域拉取会并发调用数十个地域 API，放后台执行避免阻塞 HTTP 服务启动
	go func() {
		defer logging.Recover("seed-initial-sync")
		caSvc := service.NewCloudAccountService(db, enc, service.NewSettingService(db))
		var accounts []model.CloudAccount
		db.Find(&accounts)
		for _, a := range accounts {
			if _, err := caSvc.Sync(a.ID); err != nil {
				log.Printf("同步账号 %s 时警告: %v", a.Name, err)
			}
		}
		if len(accounts) > 0 {
			log.Printf("✓ 已同步 %d 个云账号的数据", len(accounts))
		}
	}()

	return nil
}

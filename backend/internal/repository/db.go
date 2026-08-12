package repository

import (
	"cloudmanager/config"
	"cloudmanager/internal/model"
	"log"
	"os"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Init 初始化 SQLite 并自动迁移建表
func Init(cfg *config.Config) error {
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true, // 忽略 Upsert 查找不存在时的噪音日志
			Colorful:                  true,
		},
	)
	db, err := gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return err
	}
	DB = db
	return DB.AutoMigrate(
		&model.User{},
		&model.CloudAccount{},
		&model.Server{},
		&model.Domain{},
		&model.DNSRecord{},
		&model.OperationLog{},
		&model.Setting{},
	)
}

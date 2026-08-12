package repository

import (
	"cloudmanager/internal/model"

	"gorm.io/gorm"
)

type SettingRepository struct{ db *gorm.DB }

func NewSettingRepository(db *gorm.DB) *SettingRepository { return &SettingRepository{db: db} }

// Get 读取设置值，未找到时返回空字符串与 error
func (r *SettingRepository) Get(key string) (string, error) {
	var s model.Setting
	err := r.db.Where("key = ?", key).First(&s).Error
	if err != nil {
		return "", err
	}
	return s.Value, nil
}

// Set 写入设置值（不存在则创建，存在则更新）
func (r *SettingRepository) Set(key, value string) error {
	var s model.Setting
	err := r.db.Where("key = ?", key).First(&s).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(&model.Setting{Key: key, Value: value}).Error
	}
	if err != nil {
		return err
	}
	s.Value = value
	return r.db.Save(&s).Error
}

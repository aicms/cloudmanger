package repository

import (
	"cloudmanager/internal/model"

	"gorm.io/gorm"
)

type CloudAccountRepository struct{ db *gorm.DB }

func NewCloudAccountRepository(db *gorm.DB) *CloudAccountRepository {
	return &CloudAccountRepository{db: db}
}

func (r *CloudAccountRepository) List() ([]model.CloudAccount, error) {
	var list []model.CloudAccount
	err := r.db.Order("id DESC").Find(&list).Error
	return list, err
}

func (r *CloudAccountRepository) FindByID(id uint) (*model.CloudAccount, error) {
	var a model.CloudAccount
	if err := r.db.First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *CloudAccountRepository) Create(a *model.CloudAccount) error {
	return r.db.Create(a).Error
}

func (r *CloudAccountRepository) Update(a *model.CloudAccount) error {
	return r.db.Save(a).Error
}

func (r *CloudAccountRepository) Delete(id uint) error {
	return r.db.Delete(&model.CloudAccount{}, id).Error
}

func (r *CloudAccountRepository) UpdateSyncedAt(id uint, t string) error {
	return r.db.Model(&model.CloudAccount{}).Where("id = ?", id).Update("last_synced_at", t).Error
}

// UpdateSyncState 更新最近一次同步的状态/错误/时间
func (r *CloudAccountRepository) UpdateSyncState(id uint, status, lastErr, t string) error {
	return r.db.Model(&model.CloudAccount{}).Where("id = ?", id).Updates(map[string]interface{}{
		"sync_status":     status,
		"last_error":      lastErr,
		"last_synced_at":  t,
	}).Error
}

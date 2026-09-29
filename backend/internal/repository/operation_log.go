package repository

import (
	"time"

	"cloudmanager/internal/model"

	"gorm.io/gorm"
)

type OperationLogRepository struct{ db *gorm.DB }

func NewOperationLogRepository(db *gorm.DB) *OperationLogRepository {
	return &OperationLogRepository{db: db}
}

func (r *OperationLogRepository) Paginate(q model.OperationLogQuery) (*model.PageResult, error) {
	var list []model.OperationLog
	var total int64
	tx := r.db.Model(&model.OperationLog{})
	if q.Action != "" {
		tx = tx.Where("action = ?", q.Action)
	}
	if q.Keyword != "" {
		like := "%" + q.Keyword + "%"
		tx = tx.Where("target LIKE ? OR detail LIKE ? OR username LIKE ?", like, like, like)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	if err := tx.Order("id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return &model.PageResult{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

func (r *OperationLogRepository) Create(log *model.OperationLog) error {
	return r.db.Create(log).Error
}

// ListRecent 最近 N 条日志
func (r *OperationLogRepository) ListRecent(limit int) ([]model.OperationLog, error) {
	var list []model.OperationLog
	err := r.db.Order("id DESC").Limit(limit).Find(&list).Error
	return list, err
}

// DeleteBefore 删除创建时间早于 t 的日志，返回删除条数
func (r *OperationLogRepository) DeleteBefore(t time.Time) (int64, error) {
	res := r.db.Where("created_at < ?", t).Delete(&model.OperationLog{})
	return res.RowsAffected, res.Error
}

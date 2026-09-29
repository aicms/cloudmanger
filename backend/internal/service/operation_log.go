package service

import (
	"time"

	"cloudmanager/internal/model"
	"cloudmanager/internal/repository"

	"gorm.io/gorm"
)

type OperationLogService struct{ db *gorm.DB }

func NewOperationLogService(db *gorm.DB) *OperationLogService {
	return &OperationLogService{db: db}
}

func (s *OperationLogService) Paginate(q model.OperationLogQuery) (*model.PageResult, error) {
	q.Normalize()
	return repository.NewOperationLogRepository(s.db).Paginate(q)
}

func (s *OperationLogService) Record(log *model.OperationLog) error {
	return repository.NewOperationLogRepository(s.db).Create(log)
}

// Clear 清除 days 天之前的日志，返回删除条数
func (s *OperationLogService) Clear(days int) (int64, error) {
	if days < 1 {
		days = 7
	}
	return repository.NewOperationLogRepository(s.db).DeleteBefore(time.Now().AddDate(0, 0, -days))
}

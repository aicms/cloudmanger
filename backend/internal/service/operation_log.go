package service

import (
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

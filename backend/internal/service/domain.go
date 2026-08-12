package service

import (
	"cloudmanager/internal/model"
	"cloudmanager/internal/repository"

	"gorm.io/gorm"
)

type DomainService struct{ db *gorm.DB }

func NewDomainService(db *gorm.DB) *DomainService { return &DomainService{db: db} }

func (s *DomainService) Paginate(q model.DomainQuery) (*model.PageResult, error) {
	q.Normalize()
	return repository.NewDomainRepository(s.db).Paginate(q)
}

func (s *DomainService) FindByID(id uint) (*model.Domain, error) {
	return repository.NewDomainRepository(s.db).FindByID(id)
}

func (s *DomainService) ListExpiringSoon(days int) ([]model.Domain, error) {
	return repository.NewDomainRepository(s.db).ListExpiringSoon(days)
}

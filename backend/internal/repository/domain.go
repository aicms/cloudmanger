package repository

import (
	"cloudmanager/internal/model"
	"time"

	"gorm.io/gorm"
)

type DomainRepository struct{ db *gorm.DB }

func NewDomainRepository(db *gorm.DB) *DomainRepository { return &DomainRepository{db: db} }

func (r *DomainRepository) Paginate(q model.DomainQuery) (*model.PageResult, error) {
	var list []model.Domain
	var total int64
	tx := r.db.Model(&model.Domain{})
	if q.Provider != "" {
		tx = tx.Where("provider = ?", q.Provider)
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	if q.Keyword != "" {
		like := "%" + q.Keyword + "%"
		tx = tx.Where("domain_name LIKE ?", like)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	if err := tx.Order("id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return &model.PageResult{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

func (r *DomainRepository) FindByID(id uint) (*model.Domain, error) {
	var d model.Domain
	if err := r.db.First(&d, id).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DomainRepository) FindByName(name string) (*model.Domain, error) {
	var d model.Domain
	if err := r.db.Where("domain_name = ?", name).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DomainRepository) Upsert(d *model.Domain) error {
	var exist model.Domain
	err := r.db.Where("cloud_account_id = ? AND domain_name = ?", d.CloudAccountID, d.DomainName).First(&exist).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(d).Error
	}
	if err != nil {
		return err
	}
	d.ID = exist.ID
	return r.db.Save(d).Error
}

func (r *DomainRepository) ListExpiringSoon(days int) ([]model.Domain, error) {
	var list []model.Domain
	// 在 Go 中计算日期边界，避免 SQLite 函数被当成字符串字面量
	now := time.Now().Format("2006-01-02")
	future := time.Now().AddDate(0, 0, days).Format("2006-01-02")
	err := r.db.Where("expiry_date >= ? AND expiry_date <= ?", now, future).
		Order("expiry_date ASC").Find(&list).Error
	return list, err
}

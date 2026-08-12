package repository

import (
	"cloudmanager/internal/model"

	"gorm.io/gorm"
)

type DNSRecordRepository struct{ db *gorm.DB }

func NewDNSRecordRepository(db *gorm.DB) *DNSRecordRepository { return &DNSRecordRepository{db: db} }

func (r *DNSRecordRepository) Paginate(q model.DNSRecordQuery) (*model.PageResult, error) {
	var list []model.DNSRecord
	var total int64
	tx := r.db.Model(&model.DNSRecord{})
	if q.DomainID != 0 {
		tx = tx.Where("domain_id = ?", q.DomainID)
	}
	if q.Domain != "" {
		tx = tx.Where("domain_name = ?", q.Domain)
	}
	if q.Type != "" {
		tx = tx.Where("type = ?", q.Type)
	}
	if q.Provider != "" {
		tx = tx.Where("provider = ?", q.Provider)
	}
	if q.Keyword != "" {
		like := "%" + q.Keyword + "%"
		tx = tx.Where("host LIKE ? OR value LIKE ?", like, like)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	if err := tx.Order("id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return &model.PageResult{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

func (r *DNSRecordRepository) FindByID(id uint) (*model.DNSRecord, error) {
	var rec model.DNSRecord
	if err := r.db.First(&rec, id).Error; err != nil {
		return nil, err
	}
	return &rec, nil
}

func (r *DNSRecordRepository) Create(rec *model.DNSRecord) error {
	return r.db.Create(rec).Error
}

func (r *DNSRecordRepository) Update(rec *model.DNSRecord) error {
	return r.db.Save(rec).Error
}

func (r *DNSRecordRepository) Delete(id uint) error {
	return r.db.Delete(&model.DNSRecord{}, id).Error
}

func (r *DNSRecordRepository) Upsert(rec *model.DNSRecord) error {
	var exist model.DNSRecord
	err := r.db.Where("domain_id = ? AND record_id = ?", rec.DomainID, rec.RecordID).First(&exist).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(rec).Error
	}
	if err != nil {
		return err
	}
	rec.ID = exist.ID
	return r.db.Save(rec).Error
}

package repository

import (
	"cloudmanager/internal/model"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type ServerRepository struct{ db *gorm.DB }

func NewServerRepository(db *gorm.DB) *ServerRepository { return &ServerRepository{db: db} }

func (r *ServerRepository) Paginate(q model.ServerQuery) (*model.PageResult, error) {
	var list []model.Server
	var total int64
	tx := r.db.Model(&model.Server{})
	if q.Provider != "" {
		tx = tx.Where("provider = ?", q.Provider)
	}
	if q.Region != "" {
		tx = tx.Where("region = ?", q.Region)
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	if q.Keyword != "" {
		like := "%" + q.Keyword + "%"
		tx = tx.Where("name LIKE ? OR instance_id LIKE ? OR public_ip LIKE ? OR inner_ip LIKE ?", like, like, like, like)
	}
	if q.Tag != "" {
		conds := make([]string, 0)
		args := make([]interface{}, 0)
		for _, p := range strings.Split(q.Tag, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				conds = append(conds, "tags LIKE ?")
				args = append(args, "%"+p+"%")
			}
		}
		if len(conds) > 0 {
			tx = tx.Where(strings.Join(conds, " OR "), args...)
		}
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	if err := tx.Order("id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return &model.PageResult{List: list, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

func (r *ServerRepository) FindByID(id uint) (*model.Server, error) {
	var s model.Server
	if err := r.db.First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ServerRepository) FindByInstanceID(provider, instanceID string) (*model.Server, error) {
	var s model.Server
	if err := r.db.Where("provider = ? AND instance_id = ?", provider, instanceID).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ServerRepository) Upsert(s *model.Server) error {
	// 按 cloud_account_id + instance_id 唯一
	var exist model.Server
	err := r.db.Where("cloud_account_id = ? AND instance_id = ?", s.CloudAccountID, s.InstanceID).First(&exist).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(s).Error
	}
	if err != nil {
		return err
	}
	s.ID = exist.ID
	// 保留用户自定义的名称与标签，避免同步时被云厂商数据覆盖
	s.CustomName = exist.CustomName
	s.Tags = exist.Tags
	return r.db.Save(s).Error
}

// ListIDs 按筛选条件返回匹配的服务器 ID 列表（不分页，用于批量操作）
func (r *ServerRepository) ListIDs(q model.ServerQuery) ([]uint, error) {
	var ids []uint
	tx := r.db.Model(&model.Server{})
	if q.Provider != "" {
		tx = tx.Where("provider = ?", q.Provider)
	}
	if q.Region != "" {
		tx = tx.Where("region = ?", q.Region)
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	if q.Keyword != "" {
		like := "%" + q.Keyword + "%"
		tx = tx.Where("name LIKE ? OR instance_id LIKE ? OR public_ip LIKE ? OR inner_ip LIKE ?", like, like, like, like)
	}
	if q.Tag != "" {
		conds := make([]string, 0)
		args := make([]interface{}, 0)
		for _, p := range strings.Split(q.Tag, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				conds = append(conds, "tags LIKE ?")
				args = append(args, "%"+p+"%")
			}
		}
		if len(conds) > 0 {
			tx = tx.Where(strings.Join(conds, " OR "), args...)
		}
	}
	if err := tx.Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// UpdateMeta 更新用户可编辑字段：自定义名称、标签
func (r *ServerRepository) UpdateMeta(id uint, customName, tags string) error {
	return r.db.Model(&model.Server{}).Where("id = ?", id).
		Updates(map[string]interface{}{"custom_name": customName, "tags": tags}).Error
}

// ListTags 返回去重后的全部标签（用于筛选）
func (r *ServerRepository) ListTags() ([]string, error) {
	var raw []string
	if err := r.db.Model(&model.Server{}).Distinct("tags").Where("tags <> ''").Pluck("tags", &raw).Error; err != nil {
		return nil, err
	}
	set := make(map[string]struct{})
	for _, s := range raw {
		for _, t := range strings.Split(s, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				set[t] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(set))
	for t := range set {
		out = append(out, t)
	}
	sort.Strings(out)
	return out, nil
}

func (r *ServerRepository) UpdateStatus(id uint, status string) error {
	return r.db.Model(&model.Server{}).Where("id = ?", id).Update("status", status).Error
}

func (r *ServerRepository) CountByStatus() (map[string]int64, error) {
	type result struct {
		Status string
		Count  int64
	}
	var results []result
	err := r.db.Model(&model.Server{}).Select("status, count(*) as count").Group("status").Scan(&results).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]int64)
	for _, r := range results {
		m[r.Status] = r.Count
	}
	return m, nil
}

func (r *ServerRepository) CountByProvider() (map[string]int64, error) {
	type result struct {
		Provider string
		Count    int64
	}
	var results []result
	err := r.db.Model(&model.Server{}).Select("provider, count(*) as count").Group("provider").Scan(&results).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]int64)
	for _, r := range results {
		m[r.Provider] = r.Count
	}
	return m, nil
}

// CountByRegion 服务器地域分布
func (r *ServerRepository) CountByRegion() (map[string]int64, error) {
	type result struct {
		Region string
		Count  int64
	}
	var results []result
	err := r.db.Model(&model.Server{}).Select("region, count(*) as count").Group("region").Order("count DESC").Scan(&results).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]int64)
	for _, r := range results {
		m[r.Region] = r.Count
	}
	return m, nil
}

// ListRegions 所有出现过的地域（用于筛选项动态填充）
func (r *ServerRepository) ListRegions() ([]string, error) {
	var regions []string
	err := r.db.Model(&model.Server{}).Distinct("region").Where("region <> ''").Pluck("region", &regions).Error
	return regions, err
}

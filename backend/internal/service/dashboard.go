package service

import (
	"cloudmanager/internal/model"
	"cloudmanager/internal/repository"

	"gorm.io/gorm"
)

type DashboardService struct{ db *gorm.DB }

func NewDashboardService(db *gorm.DB) *DashboardService { return &DashboardService{db: db} }

// Overview 仪表盘汇总数据
type Overview struct {
	ServerTotal      int64            `json:"server_total"`
	ServerRunning   int64            `json:"server_running"`
	ServerStopped   int64            `json:"server_stopped"`
	DomainTotal     int64            `json:"domain_total"`
	DomainExpiring  int64            `json:"domain_expiring"`
	DomainExpired   int64            `json:"domain_expired"`
	ProviderDist    map[string]int64 `json:"provider_dist"`
	StatusDist      map[string]int64 `json:"status_dist"`
	RegionDist      map[string]int64 `json:"region_dist"`
	ExpiringDomains []model.Domain   `json:"expiring_domains"`
	RecentLogs      []model.OperationLog `json:"recent_logs"`
}

func (s *DashboardService) Overview() (*Overview, error) {
	o := &Overview{
		ProviderDist: map[string]int64{},
		StatusDist:   map[string]int64{},
		RegionDist:   map[string]int64{},
	}

	srvRepo := repository.NewServerRepository(s.db)
	domRepo := repository.NewDomainRepository(s.db)
	logRepo := repository.NewOperationLogRepository(s.db)

	statusDist, _ := srvRepo.CountByStatus()
	for k, v := range statusDist {
		o.StatusDist[k] = v
		o.ServerTotal += v
		if k == "Running" {
			o.ServerRunning = v
		}
		if k == "Stopped" {
			o.ServerStopped = v
		}
	}

	provDist, _ := srvRepo.CountByProvider()
	for k, v := range provDist {
		o.ProviderDist[k] = v
	}

	regionDist, _ := srvRepo.CountByRegion()
	for k, v := range regionDist {
		o.RegionDist[k] = v
	}

	// 域名统计
	var domainTotal, expired int64
	s.db.Model(&model.Domain{}).Count(&domainTotal)
	o.DomainTotal = domainTotal

	// 即将到期（30天内）
	expiringList, _ := domRepo.ListExpiringSoon(model.DomainStatusExpiring)
	o.ExpiringDomains = expiringList
	o.DomainExpiring = int64(len(expiringList))
	s.db.Model(&model.Domain{}).Where("status = ?", "expired").Count(&expired)
	o.DomainExpired = expired

	// 最近操作日志（10条）
	o.RecentLogs, _ = logRepo.ListRecent(10)

	return o, nil
}

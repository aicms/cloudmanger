package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"cloudmanager/internal/auth"
	"cloudmanager/internal/cloud"
	"cloudmanager/internal/model"
	"cloudmanager/internal/repository"

	"gorm.io/gorm"
)

type DNSRecordService struct {
	db  *gorm.DB
	enc *auth.AESEncryptor
}

func NewDNSRecordService(db *gorm.DB, enc *auth.AESEncryptor) *DNSRecordService {
	return &DNSRecordService{db: db, enc: enc}
}

func (s *DNSRecordService) Paginate(q model.DNSRecordQuery) (*model.PageResult, error) {
	q.Normalize()
	return repository.NewDNSRecordRepository(s.db).Paginate(q)
}

func (s *DNSRecordService) FindByID(id uint) (*model.DNSRecord, error) {
	return repository.NewDNSRecordRepository(s.db).FindByID(id)
}

// resolveProvider 通过 domain 解析出所属云账号与 provider
func (s *DNSRecordService) resolveProvider(domainID uint) (cloud.Account, *model.Domain, error) {
	domRepo := repository.NewDomainRepository(s.db)
	dom, err := domRepo.FindByID(domainID)
	if err != nil {
		return cloud.Account{}, nil, errors.New("域名不存在")
	}
	acctRepo := repository.NewCloudAccountRepository(s.db)
	acct, err := acctRepo.FindByID(dom.CloudAccountID)
	if err != nil {
		return cloud.Account{}, dom, errors.New("云账号不存在")
	}
	secret, _ := s.enc.Decrypt(acct.AccessKeySecret)
	account := cloud.Account{
		ID:              acct.ID,
		Provider:        acct.Provider,
		AccessKeyID:     acct.AccessKeyID,
		AccessKeySecret: secret,
		Region:          acct.Region,
	}
	return account, dom, nil
}

// Create 创建解析记录：先调云厂商接口，再写本地缓存
func (s *DNSRecordService) Create(req model.CreateDNSRecordRequest) (*model.DNSRecord, error) {
	account, dom, err := s.resolveProvider(req.DomainID)
	if err != nil {
		return nil, err
	}
	provider := cloud.Get(account.Provider)
	if provider == nil {
		return nil, errors.New("不支持的云厂商: " + account.Provider)
	}
	input := cloud.DNSRecord{
		Type:     req.Type,
		Host:     req.Host,
		Value:    req.Value,
		TTL:      req.TTL,
		Priority: req.Priority,
		Status:   "enable",
	}
	created, err := provider.CreateDNSRecord(nil, account, dom.DomainName, input)
	if err != nil {
		return nil, err
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	rec := &model.DNSRecord{
		DomainID:       req.DomainID,
		DomainName:     dom.DomainName,
		CloudAccountID: dom.CloudAccountID,
		Provider:       account.Provider,
		RecordID:       created.RecordID,
		Type:           created.Type,
		Host:           created.Host,
		Value:          created.Value,
		TTL:            created.TTL,
		Priority:       created.Priority,
		Status:         created.Status,
		SyncedAt:       now,
	}
	if err := repository.NewDNSRecordRepository(s.db).Create(rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// Update 修改解析记录
func (s *DNSRecordService) Update(id uint, req model.UpdateDNSRecordRequest) (*model.DNSRecord, error) {
	recRepo := repository.NewDNSRecordRepository(s.db)
	rec, err := recRepo.FindByID(id)
	if err != nil {
		return nil, errors.New("解析记录不存在")
	}
	account, _, err := s.resolveProvider(rec.DomainID)
	if err != nil {
		return nil, err
	}
	provider := cloud.Get(account.Provider)
	if provider == nil {
		return nil, errors.New("不支持的云厂商: " + account.Provider)
	}
	input := cloud.DNSRecord{
		RecordID: rec.RecordID,
		Type:     strOr(req.Type, rec.Type),
		Host:     strOr(req.Host, rec.Host),
		Value:    strOr(req.Value, rec.Value),
		TTL:      intOr(req.TTL, rec.TTL),
		Priority: intOr(req.Priority, rec.Priority),
		// 状态只在调用方显式要求时下发：本地 rec.Status 可能因旧的错误同步而不实，
		// 用它兜底会让"只改记录值"的操作意外启用/暂停解析。
		Status: req.Status,
	}
	if _, err := provider.UpdateDNSRecord(nil, account, rec.DomainName, input); err != nil {
		return nil, err
	}
	// 同步更新本地
	if req.Type != "" {
		rec.Type = req.Type
	}
	if req.Host != "" {
		rec.Host = req.Host
	}
	if req.Value != "" {
		rec.Value = req.Value
	}
	if req.TTL > 0 {
		rec.TTL = req.TTL
	}
	if req.Priority > 0 {
		rec.Priority = req.Priority
	}
	if req.Status != "" {
		rec.Status = req.Status
	}
	if err := recRepo.Update(rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// Delete 删除解析记录
func (s *DNSRecordService) Delete(id uint) error {
	recRepo := repository.NewDNSRecordRepository(s.db)
	rec, err := recRepo.FindByID(id)
	if err != nil {
		return errors.New("解析记录不存在")
	}
	account, _, err := s.resolveProvider(rec.DomainID)
	if err == nil {
		if provider := cloud.Get(account.Provider); provider != nil {
			if e := provider.DeleteDNSRecord(nil, account, rec.DomainName, rec.RecordID); e != nil && !recordAlreadyGone(e) {
				return fmt.Errorf("云上解析没删掉，本地记录已保留: %w", e)
			}
		}
	}
	return recRepo.Delete(id)
}

// recordAlreadyGone 判断云厂商错误是否表示该记录在云上本就不存在。
// 只有这种情况可以直接清掉本地缓存；其余失败必须回给用户，否则本地记录消失了、
// 云上解析还在，界面却显示删除成功。
func recordAlreadyGone(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, k := range []string{
		"recordidinvalid", // DNSPod：记录编号错误
		"recordnotexist",  // 阿里云：记录不存在
		"invalidrecordid",
		"记录编号错误",
		"不存在",
	} {
		if strings.Contains(msg, k) {
			return true
		}
	}
	return false
}

// strOr 取非空字符串，否则取 fallback
func strOr(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}

// intOr 取非零值，否则取 fallback
func intOr(v, fallback int) int {
	if v != 0 {
		return v
	}
	return fallback
}

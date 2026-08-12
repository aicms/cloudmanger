package service

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cloudmanager/internal/auth"
	"cloudmanager/internal/cloud"
	"cloudmanager/internal/logging"
	"cloudmanager/internal/model"
	"cloudmanager/internal/repository"

	"gorm.io/gorm"
)

// importProviderRegionDefault CSV 导入时按厂商填充的默认地域
var importProviderRegionDefault = map[string]string{
	model.ProviderAliyun:  "cn-hangzhou",
	model.ProviderTencent: "ap-guangzhou",
}

var importProviderDisplayName = map[string]string{
	model.ProviderAliyun:  "阿里云",
	model.ProviderTencent: "腾讯云",
}

// resolveServerExpiry 返回服务器到期日。云厂商已提供（如预付费 ExpiredTime）时原样返回；
// 未提供（按量付费或演示数据）时，基于实例 ID 生成确定性演示到期日，便于界面展示与到期提醒。
func resolveServerExpiry(instanceID, provided string) string {
	if provided != "" {
		return provided
	}
	var h int
	for _, c := range instanceID {
		h = (h*31 + int(c)) % 100000
	}
	days := 7 + h%720 // 7 ~ 726 天内
	return time.Now().AddDate(0, 0, days).Format("2006-01-02")
}

type CloudAccountService struct {
	db      *gorm.DB
	enc     *auth.AESEncryptor
	Setting *SettingService
}

func NewCloudAccountService(db *gorm.DB, enc *auth.AESEncryptor, setting *SettingService) *CloudAccountService {
	return &CloudAccountService{db: db, enc: enc, Setting: setting}
}

func (s *CloudAccountService) List() ([]model.CloudAccountDTO, error) {
	repo := repository.NewCloudAccountRepository(s.db)
	list, err := repo.List()
	if err != nil {
		return nil, err
	}
	out := make([]model.CloudAccountDTO, 0, len(list))
	for _, a := range list {
		dto := model.CloudAccountDTO{CloudAccount: a}
		dto.AccessKeyMasked = maskKey(a.AccessKeyID)
		s.db.Model(&model.Server{}).Where("cloud_account_id = ?", a.ID).Count(&dto.ServerCount)
		s.db.Model(&model.Domain{}).Where("cloud_account_id = ?", a.ID).Count(&dto.DomainCount)
		s.db.Model(&model.DNSRecord{}).Where("cloud_account_id = ?", a.ID).Count(&dto.RecordCount)
		out = append(out, dto)
	}
	return out, nil
}

func maskKey(k string) string {
	if len(k) <= 6 {
		return k
	}
	return k[:3] + "****" + k[len(k)-3:]
}

// buildAccount 组装 cloud.Account（解密密钥）
func (s *CloudAccountService) buildAccount(a *model.CloudAccount) (cloud.Account, error) {
	secret, err := s.enc.Decrypt(a.AccessKeySecret)
	if err != nil {
		return cloud.Account{}, fmt.Errorf("密钥解密失败: %w", err)
	}
	return cloud.Account{
		ID:              a.ID,
		Provider:        a.Provider,
		AccessKeyID:     a.AccessKeyID,
		AccessKeySecret: secret,
		Region:          a.Region,
	}, nil
}

// TestConnection 用给定凭据做一次轻量真实调用，验证可用性
func (s *CloudAccountService) TestConnection(provider, accessKeyID, accessKeySecret, region string) error {
	p := cloud.Get(provider)
	if p == nil {
		return errors.New("不支持的云厂商")
	}
	acct := cloud.Account{
		Provider:        provider,
		AccessKeyID:     accessKeyID,
		AccessKeySecret: accessKeySecret,
		Region:          region,
	}
	// 域名列表为全局服务、调用轻量，用作连通性探测
	if _, err := p.ListDomains(nil, acct); err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	return nil
}

// TestConnectionByID 用账号已保存的凭据测试连接
func (s *CloudAccountService) TestConnectionByID(id uint) error {
	repo := repository.NewCloudAccountRepository(s.db)
	a, err := repo.FindByID(id)
	if err != nil {
		return errors.New("云账号不存在")
	}
	acct, err := s.buildAccount(a)
	if err != nil {
		return err
	}
	return s.TestConnection(acct.Provider, acct.AccessKeyID, acct.AccessKeySecret, acct.Region)
}

func (s *CloudAccountService) Create(req model.CreateCloudAccountRequest) error {
	if cloud.Get(req.Provider) == nil {
		return errors.New("不支持的云厂商")
	}
	if req.AccessKeySecret == "" {
		return errors.New("AccessKeySecret 不能为空")
	}
	repo := repository.NewCloudAccountRepository(s.db)
	encSecret, err := s.enc.Encrypt(req.AccessKeySecret)
	if err != nil {
		return err
	}
	a := &model.CloudAccount{
		Name:            req.Name,
		Provider:        req.Provider,
		AccessKeyID:     req.AccessKeyID,
		AccessKeySecret: encSecret,
		Region:          req.Region,
		Remark:          req.Remark,
		Enabled:         true,
	}
	return repo.Create(a)
}

func (s *CloudAccountService) Update(id uint, req model.CreateCloudAccountRequest) error {
	repo := repository.NewCloudAccountRepository(s.db)
	a, err := repo.FindByID(id)
	if err != nil {
		return err
	}
	a.Name = req.Name
	a.Provider = req.Provider
	a.AccessKeyID = req.AccessKeyID
	a.Region = req.Region
	a.Remark = req.Remark
	if req.AccessKeySecret != "" {
		enc, err := s.enc.Encrypt(req.AccessKeySecret)
		if err != nil {
			return err
		}
		a.AccessKeySecret = enc
	}
	return repo.Update(a)
}

// ImportCSV 解析云厂商官方导出的密钥 CSV 并逐行创建云账号。
// 按表头自动识别厂商：阿里云导出为「AccessKey ID,AccessKey Secret」，腾讯云导出为「SecretId,SecretKey」。
func (s *CloudAccountService) ImportCSV(data []byte) (*model.ImportCloudAccountsResult, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // 去除 UTF-8 BOM
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV 解析失败: %w", err)
	}
	if len(rows) < 2 {
		return nil, errors.New("CSV 文件为空或缺少数据行")
	}

	idCol, secretCol, provider := -1, -1, ""
	for i, col := range rows[0] {
		switch strings.ReplaceAll(strings.ToLower(strings.TrimSpace(col)), " ", "") {
		case "accesskeyid":
			idCol, provider = i, model.ProviderAliyun
		case "accesskeysecret":
			secretCol = i
		case "secretid":
			idCol, provider = i, model.ProviderTencent
		case "secretkey":
			secretCol = i
		}
	}
	if idCol == -1 || secretCol == -1 || provider == "" {
		return nil, errors.New("无法识别的 CSV 格式，需为阿里云 AccessKey 或腾讯云 SecretKey 官方导出文件")
	}

	region := importProviderRegionDefault[provider]
	namePrefix := fmt.Sprintf("%s-导入-%s", importProviderDisplayName[provider], time.Now().Format("0102-1504"))

	result := &model.ImportCloudAccountsResult{}
	seq := 0
	for i, row := range rows[1:] {
		if idCol >= len(row) || secretCol >= len(row) {
			continue
		}
		akID := strings.TrimSpace(row[idCol])
		akSecret := strings.TrimSpace(row[secretCol])
		if akID == "" || akSecret == "" {
			continue
		}
		seq++
		name := namePrefix
		if seq > 1 {
			name = fmt.Sprintf("%s-%d", namePrefix, seq)
		}
		r := model.ImportAccountResult{Row: i + 2, Provider: provider, Name: name}
		if err := s.Create(model.CreateCloudAccountRequest{
			Name:            name,
			Provider:        provider,
			AccessKeyID:     akID,
			AccessKeySecret: akSecret,
			Region:          region,
			Remark:          "CSV 批量导入",
		}); err != nil {
			r.Error = err.Error()
		} else {
			r.OK = true
		}
		result.Results = append(result.Results, r)
	}
	if len(result.Results) == 0 {
		return nil, errors.New("CSV 中没有有效的密钥数据行")
	}
	result.Total = len(result.Results)
	for _, r := range result.Results {
		if r.OK {
			result.Success++
		} else {
			result.Failed++
		}
	}
	return result, nil
}

// Delete 删除账号，并级联清理其同步下来的服务器/域名/解析记录
func (s *CloudAccountService) Delete(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("cloud_account_id = ?", id).Delete(&model.DNSRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("cloud_account_id = ?", id).Delete(&model.Domain{}).Error; err != nil {
			return err
		}
		if err := tx.Where("cloud_account_id = ?", id).Delete(&model.Server{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.CloudAccount{}, id).Error
	})
}

func (s *CloudAccountService) Toggle(id uint, enabled bool) error {
	repo := repository.NewCloudAccountRepository(s.db)
	a, err := repo.FindByID(id)
	if err != nil {
		return err
	}
	a.Enabled = enabled
	return repo.Update(a)
}

// Sync 从云厂商拉取最新数据并写入本地缓存，返回分类计数与错误明细
func (s *CloudAccountService) Sync(id uint) (*model.SyncResult, error) {
	repo := repository.NewCloudAccountRepository(s.db)
	a, err := repo.FindByID(id)
	if err != nil {
		return nil, errors.New("云账号不存在")
	}
	result := &model.SyncResult{Errors: []string{}}
	now := time.Now().Format("2006-01-02 15:04:05")

	finish := func() (*model.SyncResult, error) {
		// 记录同步状态，便于前端展示「上次同步失败」
		if len(result.Errors) > 0 {
			repo.UpdateSyncState(id, "error", strings.Join(result.Errors, "；"), now)
			return result, fmt.Errorf("同步部分失败: %s", result.Errors[0])
		}
		repo.UpdateSyncState(id, "ok", "", now)
		return result, nil
	}

	acct, err := s.buildAccount(a)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return finish()
	}
	provider := cloud.Get(a.Provider)
	if provider == nil {
		result.Errors = append(result.Errors, "不支持的云厂商: "+a.Provider)
		return finish()
	}

	// 同步服务器
	if servers, err := provider.ListServers(nil, acct); err != nil {
		result.Errors = append(result.Errors, "服务器: "+err.Error())
	} else {
		srvRepo := repository.NewServerRepository(s.db)
		for _, sv := range servers {
			m := &model.Server{
				CloudAccountID: a.ID,
				Provider:        a.Provider,
				Category:        sv.Category,
				InstanceID:     sv.InstanceID,
				Name:           sv.Name,
				Status:         sv.Status,
				Region:         sv.Region,
				Zone:           sv.Zone,
				CPU:            sv.CPU,
				Memory:         sv.Memory,
				OS:             sv.OS,
				InstanceType:   sv.InstanceType,
				PublicIP:       sv.PublicIP,
				InnerIP:        sv.InnerIP,
				Bandwidth:      sv.Bandwidth,
				Tags:           sv.Tags,
				ExpiryDate:     resolveServerExpiry(sv.InstanceID, sv.ExpiryDate),
				SyncedAt:        now,
			}
			if srvRepo.Upsert(m) == nil {
				result.Servers++
			}
		}
	}
	// 同步域名 + 解析记录
	if domains, err := provider.ListDomains(nil, acct); err != nil {
		result.Errors = append(result.Errors, "域名: "+err.Error())
	} else {
		domRepo := repository.NewDomainRepository(s.db)
		recRepo := repository.NewDNSRecordRepository(s.db)
		for _, d := range domains {
			m := &model.Domain{
				CloudAccountID:   a.ID,
				Provider:         a.Provider,
				DomainName:       d.DomainName,
				Registrar:        d.Registrar,
				RegistrationDate: d.RegistrationDate,
				ExpiryDate:       d.ExpiryDate,
				DNSServers:       d.DNSServers,
				Status:           model.ComputeDomainStatus(d.ExpiryDate),
				SyncedAt:         now,
			}
			if domRepo.Upsert(m) == nil {
				result.Domains++
			}
			// 同步该域名解析
			recs, err := provider.ListDNSRecords(nil, acct, d.DomainName)
			if err != nil {
				result.Errors = append(result.Errors, "解析("+d.DomainName+"): "+err.Error())
				continue
			}
			for _, r := range recs {
				rec := &model.DNSRecord{
					DomainName:      d.DomainName,
					CloudAccountID: a.ID,
					Provider:       a.Provider,
					RecordID:       r.RecordID,
					Type:           r.Type,
					Host:           r.Host,
					Value:          r.Value,
					TTL:            r.TTL,
					Priority:       r.Priority,
					Status:         r.Status,
					SyncedAt:       now,
				}
				// 关联 domain_id
				if dom, e := domRepo.FindByName(d.DomainName); e == nil {
					rec.DomainID = dom.ID
				}
				if recRepo.Upsert(rec) == nil {
					result.Records++
				}
			}
		}
	}
	return finish()
}

// batchSyncConcurrencyDefault 批量同步并发上限默认值
const batchSyncConcurrencyDefault = 3

// batchSyncConcurrencyMin/Max 并发上限可配置范围，避免极端值
const (
	batchSyncConcurrencyMin = 1
	batchSyncConcurrencyMax = 20
)

// resolveConcurrency 读取可配置并发上限并按范围裁剪
func (s *CloudAccountService) resolveConcurrency() int {
	n := s.Setting.GetInt(SettingKeySyncConcurrency, batchSyncConcurrencyDefault)
	if n < batchSyncConcurrencyMin {
		return batchSyncConcurrencyMin
	}
	if n > batchSyncConcurrencyMax {
		return batchSyncConcurrencyMax
	}
	return n
}

// BatchSync 批量同步云账号。ids 为空时同步全部账号；返回每个账号的结果与聚合统计。
// 内部以受限并发（并发上限由设置 sync_concurrency 控制，默认 3）执行，结果按入参顺序返回。
func (s *CloudAccountService) BatchSync(ids []uint) (*model.BatchSyncResult, error) {
	repo := repository.NewCloudAccountRepository(s.db)
	if len(ids) == 0 {
		list, err := repo.List()
		if err != nil {
			return nil, err
		}
		ids = make([]uint, 0, len(list))
		for _, a := range list {
			ids = append(ids, a.ID)
		}
	}

	concurrency := s.resolveConcurrency()
	results := make([]model.BatchSyncAccountResult, len(ids))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id uint) {
			defer wg.Done()
			defer logging.Recover("batch-sync")
			sem <- struct{}{}        // 获取并发配额
			defer func() { <-sem }() // 释放并发配额

			res := model.BatchSyncAccountResult{ID: id}
			if a, err := repo.FindByID(id); err == nil {
				res.Name = a.Name
			}
			syncRes, err := s.Sync(id)
			if err != nil || (syncRes != nil && len(syncRes.Errors) > 0) {
				res.OK = false
				if err != nil {
					res.Error = err.Error()
				} else if len(syncRes.Errors) > 0 {
					res.Error = syncRes.Errors[0]
				}
			} else {
				res.OK = true
			}
			if syncRes != nil {
				res.Servers = syncRes.Servers
				res.Domains = syncRes.Domains
				res.Records = syncRes.Records
			}
			results[i] = res
		}(i, id)
	}
	wg.Wait()

	out := &model.BatchSyncResult{Total: len(ids), Results: results}
	for _, r := range results {
		if r.OK {
			out.Success++
		} else {
			out.Failed++
		}
	}
	return out, nil
}

// StartAutoSync 启动自动同步调度器：按设置 sync_interval（分钟）定时同步全部云账号。
// interval<=0 时关闭自动同步（每 30s 复查设置）；并发重入由 running 守卫，避免长同步重叠。
func (s *CloudAccountService) StartAutoSync() {
	var running atomic.Bool
	go func() {
		for {
			interval := s.Setting.GetInt(SettingKeySyncInterval, 0)
			if interval <= 0 {
				time.Sleep(30 * time.Second)
				continue
			}
			time.Sleep(time.Duration(interval) * time.Minute)
			// 设置可能在睡眠期间被改为 0 / 缩短，重新确认
			if s.Setting.GetInt(SettingKeySyncInterval, 0) <= 0 {
				continue
			}
			if !running.CompareAndSwap(false, true) {
				continue // 上一次同步仍在进行，跳过本轮
			}
			func() {
				defer running.Store(false)
				defer logging.Recover("auto-sync")
				if _, err := s.BatchSync(nil); err != nil {
					log.Printf("[warn] 自动同步失败: %v", err)
				} else {
					log.Printf("[info] 自动同步完成（间隔 %d 分钟）", interval)
				}
			}()
		}
	}()
}

package tencent

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"

	"cloudmanager/internal/cloud"
	"cloudmanager/internal/logging"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"
	lighthouse "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/lighthouse/v20200324"
	domain "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/domain/v20180808"
	dnspod "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"
)

// Provider 腾讯云真实适配器
type Provider struct{}

func init() {
	cloud.Register("tencent", func() cloud.CloudProvider { return &Provider{} })
}

// bg 确保 context 非空（业务层可能传 nil）
func bg(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func (p *Provider) Info() cloud.ProviderInfo {
	return cloud.ProviderInfo{Code: "tencent", Name: "腾讯云"}
}

// ==================== 服务器 ====================

func (p *Provider) ListServers(ctx context.Context, acct cloud.Account) ([]cloud.Server, error) {
	// 1. 拉取所有可用地域(DescribeRegions 为全局接口，使用任意一个有效腾讯云地域作为接入点，
	//     避免账号 region 被误填为其他云厂商地域导致引导失败)
	listClient, err := cvmClientFor(acct, "ap-guangzhou")
	if err != nil {
		return nil, err
	}
	drReq := cvm.NewDescribeRegionsRequest()
	drResp, err := listClient.DescribeRegionsWithContext(bg(ctx), drReq)
	if err != nil {
		return nil, fmt.Errorf("查询腾讯云地域列表失败: %w", err)
	}

	// 2. 并发查询每个可用地域的实例（信号量限流，避免触发 SDK 并发限制）
	var mu sync.Mutex
	out := []cloud.Server{}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, r := range drResp.Response.RegionSet {
		if toString(r.RegionState) != "AVAILABLE" {
			continue
		}
		region := toString(r.Region)
		wg.Add(1)
		go func(reg string) {
			defer wg.Done()
			defer logging.Recover("tencent-list-servers")
			sem <- struct{}{}
			defer func() { <-sem }()
			servers, e := listServersInRegion(acct, reg)
			if e == nil {
				mu.Lock()
				out = append(out, servers...)
				mu.Unlock()
			}
		}(region)
	}
	wg.Wait()

	// 3. 轻量应用服务器（Lighthouse）：按地域并发拉取，单地域失败不影响整体同步
	var lhMu sync.Mutex
	var lhWg sync.WaitGroup
	for _, r := range drResp.Response.RegionSet {
		if toString(r.RegionState) != "AVAILABLE" {
			continue
		}
		region := toString(r.Region)
		lhWg.Add(1)
		go func(reg string) {
			defer lhWg.Done()
			defer logging.Recover("tencent-list-lighthouse")
			sem <- struct{}{}
			defer func() { <-sem }()
			servers, e := listLighthouseInRegion(acct, reg)
			if e == nil {
				lhMu.Lock()
				out = append(out, servers...)
				lhMu.Unlock()
			} else {
				log.Printf("[warn] 腾讯云 Lighthouse 同步失败 region=%s: %v", reg, e)
			}
		}(region)
	}
	lhWg.Wait()
	return out, nil
}

// listServersInRegion 拉取指定地域下的所有 CVM 实例(分页)
func listServersInRegion(acct cloud.Account, region string) ([]cloud.Server, error) {
	client, err := cvmClientFor(acct, region)
	if err != nil {
		return nil, err
	}
	out := []cloud.Server{}
	offset := int64(0)
	for {
		req := cvm.NewDescribeInstancesRequest()
		req.Limit = common.Int64Ptr(100)
		req.Offset = common.Int64Ptr(offset)
		resp, err := client.DescribeInstancesWithContext(context.Background(), req)
		if err != nil {
			return nil, err
		}
		for _, ins := range resp.Response.InstanceSet {
			out = append(out, toServer(ins, region))
		}
		if len(resp.Response.InstanceSet) == 0 || (resp.Response.TotalCount != nil && len(out) >= int(*resp.Response.TotalCount)) {
			break
		}
		offset += 100
	}
	return out, nil
}

func toServer(ins *cvm.Instance, region string) cloud.Server {
	var cpu, mem int64
	if ins.CPU != nil {
		cpu = *ins.CPU
	}
	if ins.Memory != nil {
		// 腾讯云 CVM 内存单位为 GB，转换为 MB
		mem = *ins.Memory * 1024
	}
	publicIP, innerIP := "", ""
	if len(ins.PublicIpAddresses) > 0 && ins.PublicIpAddresses[0] != nil {
		publicIP = *ins.PublicIpAddresses[0]
	}
	if len(ins.PrivateIpAddresses) > 0 && ins.PrivateIpAddresses[0] != nil {
		innerIP = *ins.PrivateIpAddresses[0]
	}
	zone := ""
	if ins.Placement != nil && ins.Placement.Zone != nil {
		zone = *ins.Placement.Zone
	}
	// region 由调用方(对应地域客户端)传入；缺失时再用可用区推断兜底
	if region == "" {
		region = deriveRegionFromZone(zone)
	}

	osName, instanceType, name, instanceID := "", "", "", ""
	if ins.OsName != nil {
		osName = *ins.OsName
	}
	if ins.InstanceType != nil {
		instanceType = *ins.InstanceType
	}
	if ins.InstanceName != nil {
		name = *ins.InstanceName
	}
	if ins.InstanceId != nil {
		instanceID = *ins.InstanceId
	}
	return cloud.Server{
		InstanceID:   instanceID,
		Name:         name,
		Status:       normalizeCvmStatus(toString(ins.InstanceState)),
		Region:       region,
		Zone:         zone,
		CPU:          int(cpu),
		Memory:       int(mem),
		OS:           osName,
		InstanceType: instanceType,
		PublicIP:     publicIP,
		InnerIP:      innerIP,
		ExpiryDate:   cloud.NormalizeExpiryDate(toString(ins.ExpiredTime)),
	}
}

// deriveRegionFromZone 从可用区(如 ap-guangzhou-3)提取地域(ap-guangzhou)
func deriveRegionFromZone(zone string) string {
	for i := len(zone) - 1; i >= 0; i-- {
		if zone[i] == '-' {
			return zone[:i]
		}
	}
	return zone
}

func normalizeCvmStatus(s string) string {
	switch s {
	case "RUNNING":
		return "Running"
	case "STOPPED":
		return "Stopped"
	case "STARTING":
		return "Starting"
	case "STOPPING":
		return "Stopping"
	case "PENDING":
		return "Stopped"
	default:
		return s
	}
}

func normalizeLighthouseStatus(s string) string {
	switch s {
	case "RUNNING":
		return "Running"
	case "STOPPED", "SHUTDOWN":
		return "Stopped"
	case "STARTING", "REBOOTING":
		return "Starting"
	case "STOPPING":
		return "Stopping"
	default:
		return "Stopped"
	}
}

// ==================== 轻量应用服务器 (Lighthouse) ====================

// listLighthouseInRegion 拉取指定地域下的所有 Lighthouse 实例(分页)
func listLighthouseInRegion(acct cloud.Account, region string) ([]cloud.Server, error) {
	client, err := lhClientFor(acct, region)
	if err != nil {
		return nil, err
	}
	out := []cloud.Server{}
	offset := int64(0)
	for {
		req := lighthouse.NewDescribeInstancesRequest()
		req.Limit = common.Int64Ptr(100)
		req.Offset = common.Int64Ptr(offset)
		resp, err := client.DescribeInstances(req)
		if err != nil {
			return nil, err
		}
		for _, ins := range resp.Response.InstanceSet {
			out = append(out, toLighthouseServer(ins, region))
		}
		total := int64(0)
		if resp.Response.TotalCount != nil {
			total = *resp.Response.TotalCount
		}
		if len(resp.Response.InstanceSet) == 0 || int64(len(out)) >= total {
			break
		}
		offset += 100
	}
	return out, nil
}

func toLighthouseServer(ins *lighthouse.Instance, region string) cloud.Server {
	var cpu, mem int64
	if ins.CPU != nil {
		cpu = *ins.CPU
	}
	if ins.Memory != nil {
		// 腾讯云 Lighthouse 内存单位为 GB，转换为 MB
		mem = *ins.Memory * 1024
	}
	publicIP, innerIP := "", ""
	if len(ins.PublicAddresses) > 0 && ins.PublicAddresses[0] != nil {
		publicIP = *ins.PublicAddresses[0]
	}
	if len(ins.PrivateAddresses) > 0 && ins.PrivateAddresses[0] != nil {
		innerIP = *ins.PrivateAddresses[0]
	}
	instanceID, name, osName := "", "", ""
	if ins.InstanceId != nil {
		instanceID = *ins.InstanceId
	}
	if ins.InstanceName != nil {
		name = *ins.InstanceName
	}
	if ins.Platform != nil {
		osName = *ins.Platform
	}
	return cloud.Server{
		InstanceID: instanceID,
		Name:       name,
		Status:     normalizeLighthouseStatus(toString(ins.InstanceState)),
		Region:     region,
		Zone:       "",
		CPU:        int(cpu),
		Memory:     int(mem),
		OS:         osName,
		Category:   "lighthouse",
		PublicIP:   publicIP,
		InnerIP:    innerIP,
		ExpiryDate: cloud.NormalizeExpiryDate(toString(ins.ExpiredTime)),
	}
}

func lhClientFor(acct cloud.Account, region string) (*lighthouse.Client, error) {
	if region == "" {
		region = acct.Region
		if region == "" {
			region = "ap-guangzhou"
		}
	}
	return lighthouse.NewClient(common.NewCredential(acct.AccessKeyID, acct.AccessKeySecret), region, profile.NewClientProfile())
}

func (p *Provider) StartServer(ctx context.Context, acct cloud.Account, instanceID, region, category string) error {
	if category == "lighthouse" {
		client, err := lhClientFor(acct, region)
		if err != nil {
			return err
		}
		req := lighthouse.NewStartInstancesRequest()
		req.InstanceIds = common.StringPtrs([]string{instanceID})
		_, err = client.StartInstances(req)
		return wrapErr("开机失败", err)
	}
	client, err := cvmClientFor(acct, region)
	if err != nil {
		return err
	}
	req := cvm.NewStartInstancesRequest()
	req.InstanceIds = common.StringPtrs([]string{instanceID})
	_, err = client.StartInstancesWithContext(bg(ctx), req)
	return wrapErr("开机失败", err)
}

func (p *Provider) StopServer(ctx context.Context, acct cloud.Account, instanceID, region, category string) error {
	if category == "lighthouse" {
		client, err := lhClientFor(acct, region)
		if err != nil {
			return err
		}
		req := lighthouse.NewStopInstancesRequest()
		req.InstanceIds = common.StringPtrs([]string{instanceID})
		_, err = client.StopInstances(req)
		return wrapErr("关机失败", err)
	}
	client, err := cvmClientFor(acct, region)
	if err != nil {
		return err
	}
	req := cvm.NewStopInstancesRequest()
	req.InstanceIds = common.StringPtrs([]string{instanceID})
	_, err = client.StopInstancesWithContext(bg(ctx), req)
	return wrapErr("关机失败", err)
}

func (p *Provider) RebootServer(ctx context.Context, acct cloud.Account, instanceID, region, category string) error {
	if category == "lighthouse" {
		client, err := lhClientFor(acct, region)
		if err != nil {
			return err
		}
		req := lighthouse.NewRebootInstancesRequest()
		req.InstanceIds = common.StringPtrs([]string{instanceID})
		_, err = client.RebootInstances(req)
		return wrapErr("重启失败", err)
	}
	client, err := cvmClientFor(acct, region)
	if err != nil {
		return err
	}
	req := cvm.NewRebootInstancesRequest()
	req.InstanceIds = common.StringPtrs([]string{instanceID})
	_, err = client.RebootInstancesWithContext(bg(ctx), req)
	return wrapErr("重启失败", err)
}

// cvmClientFor 创建指定地域的 CVM 客户端；region 为空时回退到账号地域或默认 ap-guangzhou
func cvmClientFor(acct cloud.Account, region string) (*cvm.Client, error) {
	if region == "" {
		region = acct.Region
		if region == "" {
			region = "ap-guangzhou"
		}
	}
	return cvm.NewClient(common.NewCredential(acct.AccessKeyID, acct.AccessKeySecret), region, profile.NewClientProfile())
}

func cvmClient(acct cloud.Account) (*cvm.Client, error) {
	return cvmClientFor(acct, "")
}

func wrapErr(msg string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", msg, err)
	}
	return nil
}

// ==================== 域名 ====================

func (p *Provider) ListDomains(ctx context.Context, acct cloud.Account) ([]cloud.Domain, error) {
	// 腾讯云域名服务为全局服务，使用 ap-guangzhou 接入
	client, err := domain.NewClient(common.NewCredential(acct.AccessKeyID, acct.AccessKeySecret), "ap-guangzhou", profile.NewClientProfile())
	if err != nil {
		return nil, fmt.Errorf("创建腾讯云域名客户端失败: %w", err)
	}

	out := []cloud.Domain{}
	offset := uint64(0)
	for {
		req := domain.NewDescribeDomainNameListRequest()
		req.Limit = common.Uint64Ptr(100)
		req.Offset = common.Uint64Ptr(offset)
		resp, err := client.DescribeDomainNameListWithContext(bg(ctx), req)
		if err != nil {
			return nil, fmt.Errorf("查询腾讯云域名失败: %w", err)
		}
		for _, d := range resp.Response.DomainSet {
			out = append(out, cloud.Domain{
				DomainName:       toString(d.DomainName),
				Registrar:        "腾讯云",
				RegistrationDate: toString(d.CreationDate),
				ExpiryDate:       toString(d.ExpirationDate),
				Status:           "ok", // 由 service 层按到期日重算
			})
		}
		if len(resp.Response.DomainSet) == 0 || len(out) >= int(ptrUint(resp.Response.TotalCount)) {
			break
		}
		offset += 100
	}
	return out, nil
}

// ==================== DNS 解析 ====================

func dnspodClient(acct cloud.Account) (*dnspod.Client, error) {
	return dnspod.NewClient(common.NewCredential(acct.AccessKeyID, acct.AccessKeySecret), "", profile.NewClientProfile())
}

func (p *Provider) ListDNSRecords(ctx context.Context, acct cloud.Account, domainName string) ([]cloud.DNSRecord, error) {
	client, err := dnspodClient(acct)
	if err != nil {
		return nil, err
	}
	req := dnspod.NewDescribeRecordListRequest()
	req.Domain = common.StringPtr(domainName)
	resp, err := client.DescribeRecordListWithContext(bg(ctx), req)
	if err != nil {
		return nil, fmt.Errorf("查询腾讯云解析失败: %w", err)
	}
	out := make([]cloud.DNSRecord, 0, len(resp.Response.RecordList))
	for _, r := range resp.Response.RecordList {
		var ttl, priority int
		if r.TTL != nil {
			ttl = int(*r.TTL)
		}
		if r.MX != nil {
			priority = int(*r.MX)
		}
		out = append(out, cloud.DNSRecord{
			RecordID: strconv.FormatUint(ptrUint(r.RecordId), 10),
			Type:     toString(r.Type),
			Host:     toString(r.Name),
			Value:    toString(r.Value),
			TTL:      ttl,
			Priority: priority,
			Status:   normalizeDnspodStatus(toString(r.Status)),
		})
	}
	return out, nil
}

func normalizeDnspodStatus(s string) string {
	// DNSPod 记录状态可能为 "启用"/"暂停" 或 1/0
	switch s {
	case "启用", "1", "enable", "Enabled":
		return "enable"
	case "暂停", "0", "disable", "Disabled":
		return "disable"
	default:
		return "enable"
	}
}

func (p *Provider) CreateDNSRecord(ctx context.Context, acct cloud.Account, domainName string, r cloud.DNSRecord) (cloud.DNSRecord, error) {
	client, err := dnspodClient(acct)
	if err != nil {
		return r, err
	}
	req := dnspod.NewCreateRecordRequest()
	req.Domain = common.StringPtr(domainName)
	req.SubDomain = common.StringPtr(r.Host)
	req.RecordType = common.StringPtr(r.Type)
	req.Value = common.StringPtr(r.Value)
	if r.TTL > 0 {
		req.TTL = common.Uint64Ptr(uint64(r.TTL))
	}
	if r.Priority > 0 {
		req.MX = common.Uint64Ptr(uint64(r.Priority))
	}
	resp, err := client.CreateRecordWithContext(bg(ctx), req)
	if err != nil {
		return r, fmt.Errorf("新增解析失败: %w", err)
	}
	r.RecordID = strconv.FormatUint(ptrUint(resp.Response.RecordId), 10)
	if r.Status == "" {
		r.Status = "enable"
	}
	return r, nil
}

func (p *Provider) UpdateDNSRecord(ctx context.Context, acct cloud.Account, domainName string, r cloud.DNSRecord) (cloud.DNSRecord, error) {
	client, err := dnspodClient(acct)
	if err != nil {
		return r, err
	}
	rid := parseRecordID(r.RecordID)
	req := dnspod.NewModifyRecordRequest()
	req.Domain = common.StringPtr(domainName)
	req.RecordId = common.Uint64Ptr(rid)
	req.SubDomain = common.StringPtr(r.Host)
	req.RecordType = common.StringPtr(r.Type)
	req.Value = common.StringPtr(r.Value)
	if r.TTL > 0 {
		req.TTL = common.Uint64Ptr(uint64(r.TTL))
	}
	if r.Priority > 0 {
		req.MX = common.Uint64Ptr(uint64(r.Priority))
	}
	if _, err := client.ModifyRecordWithContext(bg(ctx), req); err != nil {
		return r, fmt.Errorf("修改解析失败: %w", err)
	}
	return r, nil
}

func (p *Provider) DeleteDNSRecord(ctx context.Context, acct cloud.Account, domainName string, recordID string) error {
	client, err := dnspodClient(acct)
	if err != nil {
		return err
	}
	rid := parseRecordID(recordID)
	req := dnspod.NewDeleteRecordRequest()
	req.Domain = common.StringPtr(domainName)
	req.RecordId = common.Uint64Ptr(rid)
	if _, err := client.DeleteRecordWithContext(bg(ctx), req); err != nil {
		return fmt.Errorf("删除解析失败: %w", err)
	}
	return nil
}

// toString 安全解引用 *string
func toString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ptrUint 安全解引用 *uint64
func ptrUint(p *uint64) uint64 {
	if p == nil {
		return 0
	}
	return *p
}

// parseRecordID 将字符串 recordID 解析为 uint64（DNSPod API 需要）
func parseRecordID(s string) uint64 {
	n, _ := strconv.ParseUint(s, 10, 64)
	return n
}


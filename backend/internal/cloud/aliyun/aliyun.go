package aliyun

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"cloudmanager/internal/cloud"
	"cloudmanager/internal/logging"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/alidns"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/domain"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	swas_open "github.com/aliyun/alibaba-cloud-sdk-go/services/swas-open"
)

// 阿里云域名/DNS 服务为全局服务，固定用 cn-hangzhou 接入
const globalRegion = "cn-hangzhou"

// Provider 阿里云真实适配器
type Provider struct{}

func init() {
	cloud.Register("aliyun", func() cloud.CloudProvider { return &Provider{} })
}

func (p *Provider) Info() cloud.ProviderInfo {
	return cloud.ProviderInfo{Code: "aliyun", Name: "阿里云"}
}

// ecsClient 创建 ECS 客户端（按账号地域，或默认 cn-hangzhou）
func ecsClient(acct cloud.Account) *ecs.Client {
	return ecsClientFor(acct, "")
}

// ecsClientFor 创建指定地域的 ECS 客户端
func ecsClientFor(acct cloud.Account, region string) *ecs.Client {
	if region == "" {
		region = acct.Region
		if region == "" {
			region = globalRegion
		}
	}
	c, _ := ecs.NewClientWithAccessKey(region, acct.AccessKeyID, acct.AccessKeySecret)
	return c
}

// dnsClient 创建域名/DNS 客户端（全局服务）
func dnsClient(acct cloud.Account) (*domain.Client, *alidns.Client) {
	dc, _ := domain.NewClientWithAccessKey(globalRegion, acct.AccessKeyID, acct.AccessKeySecret)
	ac, _ := alidns.NewClientWithAccessKey(globalRegion, acct.AccessKeyID, acct.AccessKeySecret)
	return dc, ac
}

// ==================== 服务器 ====================

func (p *Provider) ListServers(ctx context.Context, acct cloud.Account) ([]cloud.Server, error) {
	// 1. 拉取账号可见的所有地域（DescribeRegions 为全局接口，用任意地域客户端即可）
	regionClient := ecsClient(acct)
	drReq := ecs.CreateDescribeRegionsRequest()
	drReq.Scheme = "https"
	drResp, err := regionClient.DescribeRegions(drReq)
	if err != nil {
		return nil, fmt.Errorf("查询阿里云地域列表失败: %w", err)
	}

	// 2. 并发查询每个地域的实例（限制并发，避免触发限流）
	var mu sync.Mutex
	out := []cloud.Server{}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, r := range drResp.Regions.Region {
		region := r.RegionId
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer logging.Recover("aliyun-list-servers")
			sem <- struct{}{}
			defer func() { <-sem }()
			servers, e := listServersInRegion(acct, region)
			if e == nil {
				mu.Lock()
				out = append(out, servers...)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// 轻量应用服务器（SWAS）：地域级接口，需按地域遍历（单地域失败不影响整体同步）
	var swasMu sync.Mutex
	var swasWg sync.WaitGroup
	for _, r := range drResp.Regions.Region {
		region := r.RegionId
		swasWg.Add(1)
		go func(reg string) {
			defer swasWg.Done()
			defer logging.Recover("aliyun-list-swas")
			sem <- struct{}{}
			defer func() { <-sem }()
			servers, e := listSwasInRegion(acct, reg)
			if e == nil {
				swasMu.Lock()
				out = append(out, servers...)
				swasMu.Unlock()
			} else {
				// 部分地域不提供轻量应用服务器属正常情况（接口不支持 / 接入点域名不存在），
				// 避免刷屏；仅记录真正异常
				msg := e.Error()
				if !strings.Contains(msg, "Region") &&
					!strings.Contains(msg, "Unsupported") &&
					!strings.Contains(msg, "CanNotResolveEndpoint") &&
					!strings.Contains(msg, "not support") &&
					!strings.Contains(msg, "no such host") {
					log.Printf("[warn] 阿里云 SWAS 同步失败 region=%s: %v", reg, e)
				}
			}
		}(region)
	}
	swasWg.Wait()

	return out, nil
}

// listSwasInRegion 拉取指定地域下的轻量应用服务器（地域级接口，按分页）
func listSwasInRegion(acct cloud.Account, region string) ([]cloud.Server, error) {
	client := swasClientFor(acct, region)
	out := []cloud.Server{}
	pageNum := 1
	for {
		req := swas_open.CreateListInstancesRequest()
		req.Scheme = "https"
		req.Domain = swasEndpoint(region)
		req.PageSize = requests.NewInteger(100)
		req.PageNumber = requests.NewInteger(pageNum)
		resp, err := client.ListInstances(req)
		if err != nil {
			return nil, err
		}
		for _, ins := range resp.Instances {
			out = append(out, toSwasServer(ins))
		}
		if len(resp.Instances) == 0 || len(out) >= resp.TotalCount {
			break
		}
		pageNum++
	}
	return out, nil
}

// toSwasServer 将轻量应用服务器实例映射为统一 Server
func toSwasServer(ins swas_open.Instance) cloud.Server {
	cpu := ins.ResourceSpec.Cpu
	mem := 0
	if m, err := strconv.ParseFloat(ins.ResourceSpec.Memory, 64); err == nil {
		// 阿里云轻量应用服务器 Memory 单位为 GB，转换为 MB
		mem = int(m * 1024)
	}
	osName := ins.Image.ImageName
	return cloud.Server{
		InstanceID: ins.InstanceId,
		Name:       ins.InstanceName,
		Status:     normalizeEcsStatus(ins.Status),
		Region:     ins.RegionId,
		Zone:       "",
		CPU:        cpu,
		Memory:     mem,
		OS:         osName,
		PublicIP:   ins.PublicIpAddress,
		InnerIP:    ins.InnerIpAddress,
		Bandwidth:  ins.ResourceSpec.Bandwidth,
		Category:   "swas",
		ExpiryDate: cloud.NormalizeExpiryDate(ins.ExpiredTime),
	}
}

// swasClientFor 创建指定地域的轻量应用服务器客户端
func swasClientFor(acct cloud.Account, region string) *swas_open.Client {
	if region == "" {
		region = acct.Region
		if region == "" {
			region = globalRegion
		}
	}
	c, _ := swas_open.NewClientWithAccessKey(region, acct.AccessKeyID, acct.AccessKeySecret)
	return c
}

// swasEndpoint 轻量应用服务器(SWAS-OPEN)接入点需显式指定，SDK 无法自动解析
func swasEndpoint(region string) string {
	if region == "" {
		region = globalRegion
	}
	return "swas." + region + ".aliyuncs.com"
}

// listServersInRegion 分页查询单个地域的 ECS 实例
func listServersInRegion(acct cloud.Account, region string) ([]cloud.Server, error) {
	client := ecsClientFor(acct, region)
	req := ecs.CreateDescribeInstancesRequest()
	req.Scheme = "https"
	req.PageSize = requests.NewInteger(100)
	req.PageNumber = requests.NewInteger(1)

	out := []cloud.Server{}
	pageNum := 1
	for {
		req.PageNumber = requests.NewInteger(pageNum)
		resp, err := client.DescribeInstances(req)
		if err != nil {
			return nil, err
		}
		for _, ins := range resp.Instances.Instance {
			out = append(out, toServer(ins))
		}
		if len(out) >= resp.TotalCount || len(resp.Instances.Instance) == 0 {
			break
		}
		pageNum++
	}
	return out, nil
}

func toServer(ins ecs.Instance) cloud.Server {
	cpu := ins.Cpu
	mem := ins.Memory // MB
	publicIP := ""
	if len(ins.PublicIpAddress.IpAddress) > 0 {
		publicIP = ins.PublicIpAddress.IpAddress[0]
	}
	if publicIP == "" && ins.EipAddress.IpAddress != "" {
		publicIP = ins.EipAddress.IpAddress
	}
	innerIP := ""
	if len(ins.InnerIpAddress.IpAddress) > 0 {
		innerIP = ins.InnerIpAddress.IpAddress[0]
	}
	if innerIP == "" && len(ins.VpcAttributes.PrivateIpAddress.IpAddress) > 0 {
		innerIP = ins.VpcAttributes.PrivateIpAddress.IpAddress[0]
	}
	// 标签
	tags := make([]string, 0, len(ins.Tags.Tag))
	for _, t := range ins.Tags.Tag {
		tags = append(tags, fmt.Sprintf("%s:%s", t.TagKey, t.TagValue))
	}
	return cloud.Server{
		InstanceID:   ins.InstanceId,
		Name:         ins.InstanceName,
		Status:       normalizeEcsStatus(ins.Status),
		Region:       ins.RegionId,
		Zone:         ins.ZoneId,
		CPU:          cpu,
		Memory:       mem,
		OS:           ins.OSName,
		InstanceType: ins.InstanceType,
		PublicIP:     publicIP,
		InnerIP:      innerIP,
		// 公网带宽上限（Mbps）；按量付费/VPC 实例可能为 0（EIP 带宽不计入此字段）
		Bandwidth:    ins.InternetMaxBandwidthOut,
		Tags:         strings.Join(tags, ","),
		Category:     "ecs",
		ExpiryDate:   cloud.NormalizeExpiryDate(ins.ExpiredTime),
	}
}

func normalizeEcsStatus(s string) string {
	switch s {
	case "Running", "Stopped", "Starting", "Stopping":
		return s
	case "Pending":
		return "Stopped"
	default:
		return s
	}
}

func (p *Provider) StartServer(ctx context.Context, acct cloud.Account, instanceID, region, category string) error {
	if category == "swas" {
		client := swasClientFor(acct, region)
		req := swas_open.CreateStartInstanceRequest()
		req.Scheme = "https"
		req.Domain = swasEndpoint(region)
		req.InstanceId = instanceID
		if _, err := client.StartInstance(req); err != nil {
			return fmt.Errorf("开机失败: %w", err)
		}
		return nil
	}
	client := ecsClientFor(acct, region)
	req := ecs.CreateStartInstanceRequest()
	req.Scheme = "https"
	req.InstanceId = instanceID
	if _, err := client.StartInstance(req); err != nil {
		return fmt.Errorf("开机失败: %w", err)
	}
	return nil
}

func (p *Provider) StopServer(ctx context.Context, acct cloud.Account, instanceID, region, category string) error {
	if category == "swas" {
		client := swasClientFor(acct, region)
		req := swas_open.CreateStopInstanceRequest()
		req.Scheme = "https"
		req.Domain = swasEndpoint(region)
		req.InstanceId = instanceID
		if _, err := client.StopInstance(req); err != nil {
			return fmt.Errorf("关机失败: %w", err)
		}
		return nil
	}
	client := ecsClientFor(acct, region)
	req := ecs.CreateStopInstanceRequest()
	req.Scheme = "https"
	req.InstanceId = instanceID
	if _, err := client.StopInstance(req); err != nil {
		return fmt.Errorf("关机失败: %w", err)
	}
	return nil
}

func (p *Provider) RebootServer(ctx context.Context, acct cloud.Account, instanceID, region, category string) error {
	if category == "swas" {
		client := swasClientFor(acct, region)
		req := swas_open.CreateRebootInstanceRequest()
		req.Scheme = "https"
		req.Domain = swasEndpoint(region)
		req.InstanceId = instanceID
		if _, err := client.RebootInstance(req); err != nil {
			return fmt.Errorf("重启失败: %w", err)
		}
		return nil
	}
	client := ecsClientFor(acct, region)
	req := ecs.CreateRebootInstanceRequest()
	req.Scheme = "https"
	req.InstanceId = instanceID
	if _, err := client.RebootInstance(req); err != nil {
		return fmt.Errorf("重启失败: %w", err)
	}
	return nil
}

// ==================== 域名 ====================

func (p *Provider) ListDomains(ctx context.Context, acct cloud.Account) ([]cloud.Domain, error) {
	dc, _ := dnsClient(acct)
	req := domain.CreateQueryDomainListRequest()
	req.Scheme = "https"
	req.PageNum = requests.NewInteger(1)
	req.PageSize = requests.NewInteger(100)

	out := []cloud.Domain{}
	pageNum := 1
	for {
		req.PageNum = requests.NewInteger(pageNum)
		resp, err := dc.QueryDomainList(req)
		if err != nil {
			return nil, fmt.Errorf("查询阿里云域名失败: %w", err)
		}
		for _, d := range resp.Data.Domain {
			out = append(out, cloud.Domain{
				DomainName:       d.DomainName,
				Registrar:        "阿里云",
				RegistrationDate: d.RegistrationDate,
				ExpiryDate:       d.ExpirationDate,
				DNSServers:       "",
				Status:           "ok", // 由 service 层按到期日重算
			})
		}
		if len(resp.Data.Domain) == 0 || len(out) >= resp.TotalItemNum {
			break
		}
		pageNum++
	}
	return out, nil
}

// ==================== DNS 解析 ====================

func (p *Provider) ListDNSRecords(ctx context.Context, acct cloud.Account, domainName string) ([]cloud.DNSRecord, error) {
	_, ac := dnsClient(acct)
	req := alidns.CreateDescribeDomainRecordsRequest()
	req.Scheme = "https"
	req.DomainName = domainName
	resp, err := ac.DescribeDomainRecords(req)
	if err != nil {
		return nil, fmt.Errorf("查询阿里云解析失败: %w", err)
	}
	out := make([]cloud.DNSRecord, 0, len(resp.DomainRecords.Record))
	for _, r := range resp.DomainRecords.Record {
		out = append(out, cloud.DNSRecord{
			RecordID: r.RecordId,
			Type:     r.Type,
			Host:     r.RR,
			Value:    r.Value,
			TTL:      int(r.TTL),
			Priority: int(r.Priority),
			Status:   normalizeRecordStatus(r.Status),
		})
	}
	return out, nil
}

func normalizeRecordStatus(s string) string {
	switch strings.ToLower(s) {
	case "enable", "enabled":
		return "enable"
	case "disable", "disabled":
		return "disable"
	default:
		return "enable"
	}
}

// aliyunStatus 把本地的 enable/disable 映射为 API 需要的 ENABLE/DISABLE。
// 无法识别时返回空串，表示不改动记录状态。
func aliyunStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "disable", "disabled", "暂停", "0":
		return "DISABLE"
	case "enable", "enabled", "启用", "1":
		return "ENABLE"
	}
	return ""
}

func (p *Provider) CreateDNSRecord(ctx context.Context, acct cloud.Account, domainName string, r cloud.DNSRecord) (cloud.DNSRecord, error) {
	_, ac := dnsClient(acct)
	req := alidns.CreateAddDomainRecordRequest()
	req.Scheme = "https"
	req.DomainName = domainName
	req.RR = r.Host
	req.Type = r.Type
	req.Value = r.Value
	if r.TTL > 0 {
		req.TTL = requests.NewInteger(r.TTL)
	}
	if r.Priority > 0 {
		req.Priority = requests.NewInteger(r.Priority)
	}
	resp, err := ac.AddDomainRecord(req)
	if err != nil {
		return r, fmt.Errorf("新增解析失败: %w", err)
	}
	r.RecordID = resp.RecordId
	if r.Status == "" {
		r.Status = "enable"
	}
	return r, nil
}

func (p *Provider) UpdateDNSRecord(ctx context.Context, acct cloud.Account, domainName string, r cloud.DNSRecord) (cloud.DNSRecord, error) {
	_, ac := dnsClient(acct)
	req := alidns.CreateUpdateDomainRecordRequest()
	req.Scheme = "https"
	req.RecordId = r.RecordID
	req.RR = r.Host
	req.Type = r.Type
	req.Value = r.Value
	if r.TTL > 0 {
		req.TTL = requests.NewInteger(r.TTL)
	}
	if r.Priority > 0 {
		req.Priority = requests.NewInteger(r.Priority)
	}
	if _, err := ac.UpdateDomainRecord(req); err != nil {
		return r, fmt.Errorf("修改解析失败: %w", err)
	}
	// UpdateDomainRecord 不带状态字段，启用/暂停必须走 SetDomainRecordStatus
	if err := setRecordStatus(acct, r); err != nil {
		return r, err
	}
	return r, nil
}

// UpdateDNSRecordStatus 实现 cloud.DNSRecordStatusUpdater：
// 只改启用/暂停，不重发记录内容。
func (p *Provider) UpdateDNSRecordStatus(ctx context.Context, acct cloud.Account, domainName string, r cloud.DNSRecord) error {
	return setRecordStatus(acct, r)
}

func setRecordStatus(acct cloud.Account, r cloud.DNSRecord) error {
	st := aliyunStatus(r.Status)
	if st == "" {
		return nil
	}
	_, ac := dnsClient(acct)
	req := alidns.CreateSetDomainRecordStatusRequest()
	req.Scheme = "https"
	req.RecordId = r.RecordID
	req.Status = st
	if _, err := ac.SetDomainRecordStatus(req); err != nil {
		return fmt.Errorf("修改解析状态失败: %w", err)
	}
	return nil
}

func (p *Provider) DeleteDNSRecord(ctx context.Context, acct cloud.Account, domainName string, recordID string) error {
	_, ac := dnsClient(acct)
	req := alidns.CreateDeleteDomainRecordRequest()
	req.Scheme = "https"
	req.RecordId = recordID
	if _, err := ac.DeleteDomainRecord(req); err != nil {
		return fmt.Errorf("删除解析失败: %w", err)
	}
	return nil
}

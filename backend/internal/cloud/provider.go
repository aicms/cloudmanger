package cloud

import (
	"context"
	"strings"
	"time"
)

// NormalizeExpiryDate 将云厂商返回的到期时间（多为 RFC3339，如 2026-08-22T16:00:00Z）
// 统一规整为 YYYY-MM-DD，便于前端按天计算剩余时间。解析失败时回退取前 10 位。
func NormalizeExpiryDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Format("2006-01-02")
	}
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// ProviderInfo 云厂商信息
type ProviderInfo struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Account 云账号上下文（从 DB 读取后传入）
type Account struct {
	ID              uint
	Provider        string
	AccessKeyID     string
	AccessKeySecret string // 已解密
	Region          string
}

// Server 云服务器
type Server struct {
	InstanceID   string `json:"instance_id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Region       string `json:"region"`
	Zone         string `json:"zone"`
	CPU          int    `json:"cpu"`
	Memory       int    `json:"memory"`
	OS           string `json:"os"`
	InstanceType string `json:"instance_type"`
	PublicIP     string `json:"public_ip"`
	InnerIP      string `json:"inner_ip"`
	Bandwidth    int    `json:"bandwidth"`
	Tags         string `json:"tags"`
	Category     string `json:"category"` // cvm / lighthouse / ecs / swas
	ExpiryDate   string `json:"expiry_date"` // 预付费到期日 YYYY-MM-DD
}

// Domain 域名
type Domain struct {
	DomainName       string `json:"domain_name"`
	Registrar        string `json:"registrar"`
	RegistrationDate string `json:"registration_date"`
	ExpiryDate       string `json:"expiry_date"`
	DNSServers       string `json:"dns_servers"`
	Status           string `json:"status"`
}

// DNSRecord 解析记录
type DNSRecord struct {
	RecordID string `json:"record_id"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Value    string `json:"value"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority"`
	Status   string `json:"status"`
}

// CloudProvider 统一云厂商接口
// 业务层只面向该接口编程，各真实云厂商均实现它。
type CloudProvider interface {
	Info() ProviderInfo

	// 服务器
	ListServers(ctx context.Context, acct Account) ([]Server, error)
	// region 为实例所在地域（来自本地缓存快照），为空时回退到账号地域；category 用于区分云服务器/轻量应用服务器以路由到对应 API
	StartServer(ctx context.Context, acct Account, instanceID, region, category string) error
	StopServer(ctx context.Context, acct Account, instanceID, region, category string) error
	RebootServer(ctx context.Context, acct Account, instanceID, region, category string) error

	// 域名
	ListDomains(ctx context.Context, acct Account) ([]Domain, error)

	// DNS 解析
	ListDNSRecords(ctx context.Context, acct Account, domainName string) ([]DNSRecord, error)
	CreateDNSRecord(ctx context.Context, acct Account, domainName string, r DNSRecord) (DNSRecord, error)
	UpdateDNSRecord(ctx context.Context, acct Account, domainName string, r DNSRecord) (DNSRecord, error)
	DeleteDNSRecord(ctx context.Context, acct Account, domainName string, recordID string) error
}

// DNSRecordStatusUpdater 可选能力：只切换解析记录的启用/暂停状态，不重发记录内容。
// 阿里云 UpdateDomainRecord 会按 (RR,Type,Value,Line) 判重，把原值重发回去会直接报
// DomainRecordDuplicate，所以纯状态切换必须走它自己的 SetDomainRecordStatus。
// 未实现该接口的厂商，业务层退回到 UpdateDNSRecord。
type DNSRecordStatusUpdater interface {
	UpdateDNSRecordStatus(ctx context.Context, acct Account, domainName string, r DNSRecord) error
}

// ProviderRegistry 按云厂商获取 Provider 实现
var registry = map[string]func() CloudProvider{}

// Register 注册一个云厂商 provider 工厂
func Register(provider string, factory func() CloudProvider) {
	registry[provider] = factory
}

// Get 获取指定云厂商的 provider 实例
func Get(provider string) CloudProvider {
	if f, ok := registry[provider]; ok {
		return f()
	}
	return nil
}

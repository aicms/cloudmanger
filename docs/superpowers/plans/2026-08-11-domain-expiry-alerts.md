# Domain Expiry Alerts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Daily-scheduled scan of synced domains that pushes expiry alerts through WeCom/DingTalk/Feishu bot webhook, SMTP email, and/or a generic webhook, each threshold firing at most once until renewal.

**Architecture:** A new `internal/notify` package holds three independent `Sender` implementations behind one interface. A new `AlertService` owns threshold-bucket dedup state (its own table, decoupled from the sync-cache-lifecycle `Domain` table), builds senders from persisted settings, and runs the daily scan. Settings API and a new "到期告警" Settings tab expose configuration and per-channel "发送测试" buttons.

**Tech Stack:** Go stdlib (`net/smtp`, `net/http`, `encoding/json`), existing GORM/SQLite stack, existing AES encryptor for the SMTP password, React/AntD on the frontend (existing `Settings.tsx` tab pattern).

## Global Constraints

- Backend code/comments follow existing style: Chinese comments where the codebase already uses them, `gofmt`-clean (new files only — do not reformat unrelated pre-existing files).
- No new third-party Go dependencies. Everything here is buildable with the stdlib plus what's already in `go.mod`.
- This repo has no configured test framework/CI (see `CLAUDE.md`) — but `go test ./...` works out of the box with the stdlib `testing` package and the already-verified `:memory:` SQLite DSN (`github.com/glebarez/sqlite`). Use that for backend unit tests. There is no frontend test setup; frontend changes are verified via `pnpm build` + manual browser check, matching how prior work in this session was verified.
- Frontend: keep all user-facing strings Chinese, matching the rest of the app.
- Setting keys are stored via the existing `settings` key-value table (`internal/repository/setting.go`) — follow the `SettingKeyXxx = "xxx"` constant pattern already in `internal/service/setting.go`.

---

### Task 1: Domain alert dedup state - model, migration, repository

**Files:**
- Create: `backend/internal/model/domain_alert.go`
- Modify: `backend/internal/model/domain.go` (extract `DaysUntilExpiry`, reuse it from `ComputeDomainStatus`)
- Create: `backend/internal/repository/domain_alert.go`
- Create: `backend/internal/repository/domain_alert_test.go`
- Modify: `backend/internal/repository/domain.go` (add `ListAll`)
- Modify: `backend/internal/repository/db.go` (register migration)

**Interfaces:**
- Produces: `model.DomainAlertState{DomainID uint, LastThreshold int, AlertedAt string}` - `LastThreshold` sentinel `-1` = never alerted, `0` = alerted at "expired" bucket, positive = alerted at that many days.
- Produces: `model.DaysUntilExpiry(expiryDate string) (days int, ok bool)`
- Produces: `repository.NewDomainAlertRepository(db *gorm.DB) *DomainAlertRepository` with `Get(domainID uint) (*model.DomainAlertState, error)` (never returns `gorm.ErrRecordNotFound` - returns the `-1` sentinel instead) and `Save(s *model.DomainAlertState) error` (upsert by `DomainID`).
- Produces: `(*DomainRepository) ListAll() ([]model.Domain, error)` - all synced domains, no pagination.

- [ ] **Step 1: Create the `DomainAlertState` model**

Create `backend/internal/model/domain_alert.go`:

```go
package model

// DomainAlertState 域名到期告警的去重状态，与 Domain 表分离存储。
// Domain 每次同步都会被整行覆盖（见 DomainRepository.Upsert 的 gorm.Save），
// 告警状态若放在 Domain 里会在下一次同步时被静默清零，因此单独建表。
type DomainAlertState struct {
	DomainID      uint   `gorm:"primaryKey" json:"domain_id"`
	LastThreshold int    `json:"last_threshold"` // 已告警过的最紧阈值天数；0=已过期档位；-1=未告警过
	AlertedAt     string `gorm:"size:32" json:"alerted_at"`
}

func (DomainAlertState) TableName() string { return "domain_alert_states" }
```

- [ ] **Step 2: Extract `DaysUntilExpiry` in `domain.go`**

In `backend/internal/model/domain.go`, replace the existing `ComputeDomainStatus` function with:

```go
// DaysUntilExpiry 解析到期日期，返回距今剩余天数（可为负，表示已过期）；
// expiryDate 为空或格式不是 2006-01-02 时 ok=false。
func DaysUntilExpiry(expiryDate string) (days int, ok bool) {
	if expiryDate == "" {
		return 0, false
	}
	t, err := time.Parse("2006-01-02", expiryDate)
	if err != nil {
		return 0, false
	}
	return int(t.Sub(time.Now().Truncate(24 * time.Hour)).Hours() / 24), true
}

// ComputeDomainStatus 按到期日期实时计算域名状态
// expired: 已过期; expiring: DomainStatusExpiring 天内到期; ok: 正常
func ComputeDomainStatus(expiryDate string) string {
	days, ok := DaysUntilExpiry(expiryDate)
	if !ok {
		return "ok"
	}
	switch {
	case days < 0:
		return "expired"
	case days <= DomainStatusExpiring:
		return "expiring"
	default:
		return "ok"
	}
}
```

This is a pure refactor (same behavior, no test needed on its own - `ComputeDomainStatus`'s existing callers are unaffected) that gives the alert scan a reusable, already-correct day-counting function instead of duplicating date parsing.

- [ ] **Step 3: Add `ListAll` to `DomainRepository`**

In `backend/internal/repository/domain.go`, add this method (anywhere after `NewDomainRepository`):

```go
// ListAll 返回全部已同步域名，用于到期告警扫描（不分页）
func (r *DomainRepository) ListAll() ([]model.Domain, error) {
	var list []model.Domain
	err := r.db.Find(&list).Error
	return list, err
}
```

- [ ] **Step 4: Create `DomainAlertRepository`**

Create `backend/internal/repository/domain_alert.go`:

```go
package repository

import (
	"cloudmanager/internal/model"

	"gorm.io/gorm"
)

type DomainAlertRepository struct{ db *gorm.DB }

func NewDomainAlertRepository(db *gorm.DB) *DomainAlertRepository {
	return &DomainAlertRepository{db: db}
}

// Get 读取某域名的告警去重状态；从未告警过时返回 LastThreshold=-1（不是错误）
func (r *DomainAlertRepository) Get(domainID uint) (*model.DomainAlertState, error) {
	var s model.DomainAlertState
	err := r.db.Where("domain_id = ?", domainID).First(&s).Error
	if err == gorm.ErrRecordNotFound {
		return &model.DomainAlertState{DomainID: domainID, LastThreshold: -1}, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Save 写入告警去重状态（不存在则创建，存在则更新）
func (r *DomainAlertRepository) Save(s *model.DomainAlertState) error {
	var exist model.DomainAlertState
	err := r.db.Where("domain_id = ?", s.DomainID).First(&exist).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(s).Error
	}
	if err != nil {
		return err
	}
	return r.db.Model(&model.DomainAlertState{}).
		Where("domain_id = ?", s.DomainID).
		Updates(map[string]interface{}{
			"last_threshold": s.LastThreshold,
			"alerted_at":     s.AlertedAt,
		}).Error
}
```

- [ ] **Step 5: Register the migration**

In `backend/internal/repository/db.go`, add `&model.DomainAlertState{}` to the `AutoMigrate(...)` call (after `&model.Setting{}`):

```go
	return DB.AutoMigrate(
		&model.User{},
		&model.CloudAccount{},
		&model.Server{},
		&model.Domain{},
		&model.DNSRecord{},
		&model.OperationLog{},
		&model.Setting{},
		&model.DomainAlertState{},
	)
```

- [ ] **Step 6: Write the repository test**

Create `backend/internal/repository/domain_alert_test.go`:

```go
package repository

import (
	"testing"

	"cloudmanager/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newAlertTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.DomainAlertState{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestDomainAlertRepository_GetDefaultsToUnalerted(t *testing.T) {
	repo := NewDomainAlertRepository(newAlertTestDB(t))

	state, err := repo.Get(42)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if state.DomainID != 42 || state.LastThreshold != -1 {
		t.Fatalf("got %+v, want DomainID=42 LastThreshold=-1", state)
	}
}

func TestDomainAlertRepository_SaveThenGet(t *testing.T) {
	repo := NewDomainAlertRepository(newAlertTestDB(t))

	if err := repo.Save(&model.DomainAlertState{DomainID: 7, LastThreshold: 15, AlertedAt: "2026-08-11 09:00:00"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	state, err := repo.Get(7)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if state.LastThreshold != 15 {
		t.Fatalf("got LastThreshold=%d, want 15", state.LastThreshold)
	}

	// 更新已存在的状态
	if err := repo.Save(&model.DomainAlertState{DomainID: 7, LastThreshold: 7, AlertedAt: "2026-08-11 10:00:00"}); err != nil {
		t.Fatalf("Save (update) returned error: %v", err)
	}
	state, err = repo.Get(7)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if state.LastThreshold != 7 {
		t.Fatalf("got LastThreshold=%d after update, want 7", state.LastThreshold)
	}
}
```

- [ ] **Step 7: Run the tests**

Run: `cd backend && go test ./internal/repository/... -run TestDomainAlertRepository -v`
Expected: both tests `PASS`.

- [ ] **Step 8: Build the whole backend to catch any wiring mistakes**

Run: `cd backend && go build ./...`
Expected: no output (success).

- [ ] **Step 9: Commit**

```bash
git add backend/internal/model/domain_alert.go backend/internal/model/domain.go \
  backend/internal/repository/domain_alert.go backend/internal/repository/domain_alert_test.go \
  backend/internal/repository/domain.go backend/internal/repository/db.go
git commit -m "Add domain alert dedup state model, repository, and migration"
```

---

### Task 2: Pure threshold-bucket logic (TDD)

**Files:**
- Create: `backend/internal/service/alert_bucket.go`
- Create: `backend/internal/service/alert_bucket_test.go`

**Interfaces:**
- Consumes: nothing (pure functions, no DB).
- Produces: `NextAlertState(daysLeft int, thresholds []int, current int) (nextState int, shouldAlert bool)`
- Produces: `ParseThresholds(raw string) []int` - parses a comma-separated setting string like `"30,15,7,3,1"` into a sorted-ascending, deduped `[]int`, silently skipping invalid/non-positive entries; returns the default `[30, 15, 7, 3, 1]` if `raw` is empty or nothing valid parses.

- [ ] **Step 1: Write the failing tests**

Create `backend/internal/service/alert_bucket_test.go`:

```go
package service

import (
	"reflect"
	"testing"
)

func TestNextAlertState(t *testing.T) {
	thresholds := []int{30, 15, 7, 3, 1}

	cases := []struct {
		name      string
		daysLeft  int
		current   int
		wantNext  int
		wantAlert bool
	}{
		{"safe zone, never alerted", 45, -1, -1, false},
		{"safe zone, previously alerted, resets", 45, 15, -1, false},
		{"first crossing into 15-day bucket", 10, -1, 15, true},
		{"same bucket, no repeat", 10, 15, 15, false},
		{"crosses into tighter 7-day bucket", 5, 15, 7, true},
		{"expired, first time", -3, 1, 0, true},
		{"expired, already alerted at expired bucket", -3, 0, 0, false},
		{"exact boundary: daysLeft == threshold", 7, -1, 7, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			next, alert := NextAlertState(tc.daysLeft, thresholds, tc.current)
			if next != tc.wantNext || alert != tc.wantAlert {
				t.Fatalf("NextAlertState(%d, thresholds, %d) = (%d, %v), want (%d, %v)",
					tc.daysLeft, tc.current, next, alert, tc.wantNext, tc.wantAlert)
			}
		})
	}
}

func TestParseThresholds(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []int
	}{
		{"empty uses default", "", []int{1, 3, 7, 15, 30}},
		{"normal list gets sorted ascending", "30,15,7,3,1", []int{1, 3, 7, 15, 30}},
		{"dedupes and skips invalid entries", "7,7,abc,-1,0,3", []int{3, 7}},
		{"whitespace tolerated", " 30 , 7 ", []int{7, 30}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseThresholds(tc.raw)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseThresholds(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd backend && go test ./internal/service/... -run 'TestNextAlertState|TestParseThresholds' -v`
Expected: `FAIL` - `NextAlertState` and `ParseThresholds` are undefined.

- [ ] **Step 3: Implement**

Create `backend/internal/service/alert_bucket.go`:

```go
package service

import (
	"sort"
	"strconv"
	"strings"
)

// DefaultAlertThresholds 未配置时使用的默认到期告警阈值（天）
var DefaultAlertThresholds = []int{1, 3, 7, 15, 30}

// NextAlertState 计算一次扫描后某域名的告警去重状态应该变成什么，以及这一轮要不要发告警。
//
// thresholds 不要求预先排序；current 是该域名当前存的 LastThreshold（-1 = 从未告警过）。
//
// 规则：
//   - daysLeft < 0（已过期）算作最紧的 bucket 0；否则 bucket 是阈值列表中满足
//     daysLeft <= T 的最小 T；如果比最大阈值还宽松（安全区），bucket = -1
//   - bucket == -1：处于安全区。如果之前告警过（current != -1），清空状态（不发送，只重置，
//     为下一次到期周期做准备）；否则什么都不做
//   - bucket 比 current 更紧（current == -1 或 bucket < current）：触发告警，状态更新为 bucket
//   - 否则（同一档位或更宽松但仍在到期区间内）：不重复发送，状态不变
func NextAlertState(daysLeft int, thresholds []int, current int) (nextState int, shouldAlert bool) {
	sorted := make([]int, len(thresholds))
	copy(sorted, thresholds)
	sort.Ints(sorted)

	bucket := -1
	if daysLeft < 0 {
		bucket = 0
	} else {
		for _, t := range sorted {
			if daysLeft <= t {
				bucket = t
				break
			}
		}
	}

	if bucket == -1 {
		if current != -1 {
			return -1, false
		}
		return current, false
	}

	if current == -1 || bucket < current {
		return bucket, true
	}

	return current, false
}

// ParseThresholds 解析形如 "30,15,7,3,1" 的阈值设置字符串，返回排序去重后的正整数列表。
// 空字符串或解析后没有任何合法值时，返回 DefaultAlertThresholds。
func ParseThresholds(raw string) []int {
	seen := map[int]bool{}
	var out []int
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n <= 0 || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	if len(out) == 0 {
		out = append([]int{}, DefaultAlertThresholds...)
		return out
	}
	sort.Ints(out)
	return out
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/service/... -run 'TestNextAlertState|TestParseThresholds' -v`
Expected: all subtests `PASS`.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/alert_bucket.go backend/internal/service/alert_bucket_test.go
git commit -m "Add pure threshold-bucket dedup logic for domain expiry alerts"
```

---

### Task 3: Notification channels (`internal/notify` package)

**Files:**
- Create: `backend/internal/notify/event.go`
- Create: `backend/internal/notify/wecom.go`
- Create: `backend/internal/notify/wecom_test.go`
- Create: `backend/internal/notify/webhook.go`
- Create: `backend/internal/notify/webhook_test.go`
- Create: `backend/internal/notify/smtp.go`
- Create: `backend/internal/notify/smtp_test.go`

**Interfaces:**
- Produces: `notify.AlertEvent{DomainName, ExpiryDate string, DaysLeft int, Threshold int}`
- Produces: `notify.Sender` interface: `Send(events []AlertEvent) error`, `Test() error`
- Produces: `notify.NewWeComSender(webhookURL string) *WeComSender` (covers WeCom/DingTalk/Feishu bot webhooks - same JSON shape)
- Produces: `notify.NewGenericWebhookSender(url string) *GenericWebhookSender`
- Produces: `notify.NewSMTPSender(host string, port int, username, password, from, to string) *SMTPSender`

**Scope note:** `WeComSender` and `GenericWebhookSender` are tested end-to-end against an `httptest.Server` (real HTTP round-trip, no network dependency). `SMTPSender`'s message-building is tested as a pure function; actually dialing an SMTP server is exercised manually via the Settings "发送测试" button in Task 8's verification, not by an automated test - spinning up a fake SMTP server needs machinery beyond what this feature justifies (YAGNI).

- [ ] **Step 1: Define the shared event type and interface**

Create `backend/internal/notify/event.go`:

```go
package notify

// AlertEvent 一条域名到期告警事件
type AlertEvent struct {
	DomainName string
	ExpiryDate string
	DaysLeft   int
	Threshold  int // 0 表示"已过期"档位
}

// Sender 一种告警推送渠道。Send 把一批事件合并成一条消息发出去，避免刷屏。
// Test 发送一条示例消息，用于 Settings 页的"发送测试"按钮验证凭据。
type Sender interface {
	Send(events []AlertEvent) error
	Test() error
}

// testEvent 用于 Test()：一条清楚标注"测试"的示例事件
var testEvent = AlertEvent{
	DomainName: "example.com",
	ExpiryDate: "2026-12-31",
	DaysLeft:   7,
	Threshold:  7,
}
```

- [ ] **Step 2: Write the failing test for `WeComSender`**

Create `backend/internal/notify/wecom_test.go`:

```go
package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWeComSender_SendPostsTextMessage(t *testing.T) {
	var gotBody map[string]interface{}
	var gotMethod, gotContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewWeComSender(server.URL)
	events := []AlertEvent{
		{DomainName: "example.com", ExpiryDate: "2026-08-20", DaysLeft: 7, Threshold: 7},
		{DomainName: "other.com", ExpiryDate: "2026-08-14", DaysLeft: 1, Threshold: 1},
	}

	if err := sender.Send(events); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotBody["msgtype"] != "text" {
		t.Fatalf("msgtype = %v, want text", gotBody["msgtype"])
	}
	text, ok := gotBody["text"].(map[string]interface{})
	if !ok {
		t.Fatalf("text field missing or wrong type: %v", gotBody["text"])
	}
	content, _ := text["content"].(string)
	if !strings.Contains(content, "example.com") || !strings.Contains(content, "other.com") {
		t.Fatalf("content missing domain names: %q", content)
	}
}

func TestWeComSender_TestSendsMarkedTestMessage(t *testing.T) {
	var gotContent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		text, _ := body["text"].(map[string]interface{})
		gotContent, _ = text["content"].(string)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewWeComSender(server.URL)
	if err := sender.Test(); err != nil {
		t.Fatalf("Test returned error: %v", err)
	}
	if !strings.Contains(gotContent, "测试") {
		t.Fatalf("test message should be marked as a test, got: %q", gotContent)
	}
}

func TestWeComSender_SendReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	sender := NewWeComSender(server.URL)
	if err := sender.Send([]AlertEvent{testEvent}); err == nil {
		t.Fatal("expected error on non-2xx response, got nil")
	}
}
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd backend && go test ./internal/notify/... -run TestWeComSender -v`
Expected: `FAIL` - `NewWeComSender` undefined.

- [ ] **Step 4: Implement `WeComSender`**

Create `backend/internal/notify/wecom.go`:

```go
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// WeComSender 企业微信/钉钉/飞书自定义机器人 Webhook（三者的免签名模式 payload 一致：
// {"msgtype":"text","text":{"content":"..."}}），Settings 页的"类型"选项仅影响提示文案。
type WeComSender struct {
	WebhookURL string
	httpClient *http.Client
}

func NewWeComSender(webhookURL string) *WeComSender {
	return &WeComSender{WebhookURL: webhookURL, httpClient: &http.Client{Timeout: 10 * time.Second}}
}

func (s *WeComSender) Send(events []AlertEvent) error {
	return s.post(formatEventsAsText(events))
}

func (s *WeComSender) Test() error {
	return s.post(formatEventsAsText([]AlertEvent{testEvent}) + "\n（这是一条测试消息）")
}

func (s *WeComSender) post(content string) error {
	body, err := json.Marshal(map[string]interface{}{
		"msgtype": "text",
		"text":    map[string]string{"content": content},
	})
	if err != nil {
		return err
	}
	resp, err := s.httpClient.Post(s.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook 返回状态码 %d", resp.StatusCode)
	}
	return nil
}

// formatEventsAsText 把一批告警事件合并成一条易读的文本消息
func formatEventsAsText(events []AlertEvent) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("【域名到期提醒】共 %d 个域名需要关注：\n", len(events)))
	for _, e := range events {
		label := fmt.Sprintf("%d 天后到期", e.DaysLeft)
		if e.Threshold == 0 {
			label = "已过期"
		}
		b.WriteString(fmt.Sprintf("- %s：%s（到期日 %s）\n", e.DomainName, label, e.ExpiryDate))
	}
	return strings.TrimRight(b.String(), "\n")
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `cd backend && go test ./internal/notify/... -run TestWeComSender -v`
Expected: all `PASS`.

- [ ] **Step 6: Write the failing test for `GenericWebhookSender`**

Create `backend/internal/notify/webhook_test.go`:

```go
package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenericWebhookSender_SendPostsEventArray(t *testing.T) {
	var got []AlertEvent
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Events []AlertEvent `json:"events"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		got = body.Events
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewGenericWebhookSender(server.URL)
	events := []AlertEvent{{DomainName: "example.com", ExpiryDate: "2026-08-20", DaysLeft: 7, Threshold: 7}}

	if err := sender.Send(events); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if len(got) != 1 || got[0].DomainName != "example.com" {
		t.Fatalf("got events = %+v, want one event for example.com", got)
	}
}

func TestGenericWebhookSender_SendReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	sender := NewGenericWebhookSender(server.URL)
	if err := sender.Send([]AlertEvent{testEvent}); err == nil {
		t.Fatal("expected error on non-2xx response, got nil")
	}
}
```

- [ ] **Step 7: Run to verify it fails**

Run: `cd backend && go test ./internal/notify/... -run TestGenericWebhookSender -v`
Expected: `FAIL` - `NewGenericWebhookSender` undefined.

- [ ] **Step 8: Implement `GenericWebhookSender`**

Create `backend/internal/notify/webhook.go`:

```go
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GenericWebhookSender 通用 Webhook：POST 一个固定结构的 JSON body，方便对接自建系统
type GenericWebhookSender struct {
	URL        string
	httpClient *http.Client
}

func NewGenericWebhookSender(url string) *GenericWebhookSender {
	return &GenericWebhookSender{URL: url, httpClient: &http.Client{Timeout: 10 * time.Second}}
}

func (s *GenericWebhookSender) Send(events []AlertEvent) error {
	return s.post(events)
}

func (s *GenericWebhookSender) Test() error {
	return s.post([]AlertEvent{testEvent})
}

func (s *GenericWebhookSender) post(events []AlertEvent) error {
	body, err := json.Marshal(map[string]interface{}{"events": events})
	if err != nil {
		return err
	}
	resp, err := s.httpClient.Post(s.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook 返回状态码 %d", resp.StatusCode)
	}
	return nil
}
```

- [ ] **Step 9: Run to verify it passes**

Run: `cd backend && go test ./internal/notify/... -run TestGenericWebhookSender -v`
Expected: all `PASS`.

- [ ] **Step 10: Write the failing test for SMTP message building**

Create `backend/internal/notify/smtp_test.go`:

```go
package notify

import (
	"strings"
	"testing"
)

func TestBuildSMTPMessage_ContainsHeadersAndBody(t *testing.T) {
	s := &SMTPSender{From: "alerts@example.com", To: "admin@example.com"}
	msg := string(s.buildMessage("主题测试", "正文内容"))

	if !strings.Contains(msg, "From: alerts@example.com") {
		t.Fatalf("message missing From header: %q", msg)
	}
	if !strings.Contains(msg, "To: admin@example.com") {
		t.Fatalf("message missing To header: %q", msg)
	}
	if !strings.Contains(msg, "Subject: =?UTF-8?B?") {
		t.Fatalf("message missing base64-encoded UTF-8 Subject header: %q", msg)
	}
	if !strings.Contains(msg, "正文内容") {
		t.Fatalf("message missing body: %q", msg)
	}
}

func TestSMTPSender_TestEventFormatsAsExpectedSubjectAndBody(t *testing.T) {
	s := &SMTPSender{From: "alerts@example.com", To: "admin@example.com"}
	subject, body := s.formatEvents([]AlertEvent{testEvent})

	if !strings.Contains(subject, "域名到期") {
		t.Fatalf("subject = %q, want it to mention 域名到期", subject)
	}
	if !strings.Contains(body, testEvent.DomainName) {
		t.Fatalf("body = %q, want it to mention %q", body, testEvent.DomainName)
	}
}
```

- [ ] **Step 11: Run to verify it fails**

Run: `cd backend && go test ./internal/notify/... -run TestBuildSMTPMessage -v`
Expected: `FAIL` - `SMTPSender` undefined.

- [ ] **Step 12: Implement `SMTPSender`**

Create `backend/internal/notify/smtp.go`:

```go
package notify

import (
	"encoding/base64"
	"fmt"
	"net/smtp"
	"strings"
)

// SMTPSender 通过 SMTP + STARTTLS 发送告警邮件（标准库 net/smtp，不引入新依赖）
type SMTPSender struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       string // 支持逗号分隔多个收件人
}

func NewSMTPSender(host string, port int, username, password, from, to string) *SMTPSender {
	return &SMTPSender{Host: host, Port: port, Username: username, Password: password, From: from, To: to}
}

func (s *SMTPSender) Send(events []AlertEvent) error {
	subject, body := s.formatEvents(events)
	return s.send(subject, body)
}

func (s *SMTPSender) Test() error {
	subject, body := s.formatEvents([]AlertEvent{testEvent})
	return s.send("【测试】"+subject, "这是一条测试消息，用于验证 SMTP 配置是否可用。\n\n"+body)
}

func (s *SMTPSender) send(subject, body string) error {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	auth := smtp.PlainAuth("", s.Username, s.Password, s.Host)
	msg := s.buildMessage(subject, body)
	recipients := strings.Split(s.To, ",")
	for i := range recipients {
		recipients[i] = strings.TrimSpace(recipients[i])
	}
	return smtp.SendMail(addr, auth, s.From, recipients, msg)
}

// formatEvents 把事件列表格式化成邮件主题+正文
func (s *SMTPSender) formatEvents(events []AlertEvent) (subject, body string) {
	subject = fmt.Sprintf("域名到期提醒：%d 个域名需要关注", len(events))
	var b strings.Builder
	for _, e := range events {
		label := fmt.Sprintf("%d 天后到期", e.DaysLeft)
		if e.Threshold == 0 {
			label = "已过期"
		}
		b.WriteString(fmt.Sprintf("%s：%s（到期日 %s）\n", e.DomainName, label, e.ExpiryDate))
	}
	return subject, b.String()
}

// buildMessage 组装带 UTF-8 Subject 编码的最简 SMTP 报文
func (s *SMTPSender) buildMessage(subject, body string) []byte {
	encodedSubject := "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(subject)) + "?="
	var b strings.Builder
	b.WriteString("From: " + s.From + "\r\n")
	b.WriteString("To: " + s.To + "\r\n")
	b.WriteString("Subject: " + encodedSubject + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	return []byte(b.String())
}
```

- [ ] **Step 13: Run to verify it passes**

Run: `cd backend && go test ./internal/notify/... -v`
Expected: every test in the package `PASS`.

- [ ] **Step 14: Build the whole backend**

Run: `cd backend && go build ./...`
Expected: no output (success).

- [ ] **Step 15: Commit**

```bash
git add backend/internal/notify/
git commit -m "Add notify package: WeCom/DingTalk/Feishu, SMTP, and generic webhook senders"
```

---

### Task 4: `AlertService` - scan, dedup, dispatch, and channel testing

**Files:**
- Modify: `backend/internal/service/setting.go` (add setting key constants)
- Create: `backend/internal/service/alert.go`
- Create: `backend/internal/service/alert_test.go`

**Interfaces:**
- Consumes: `NextAlertState`, `ParseThresholds` (Task 2); `notify.Sender`, `notify.AlertEvent`, `notify.NewWeComSender`, `notify.NewGenericWebhookSender`, `notify.NewSMTPSender` (Task 3); `repository.NewDomainAlertRepository`, `(*DomainRepository).ListAll` (Task 1); `model.DaysUntilExpiry` (Task 1); existing `auth.AESEncryptor` (`Encrypt`/`Decrypt`), existing `OperationLogService.Record`.
- Produces: `service.NewAlertService(db *gorm.DB, enc *auth.AESEncryptor, setting *SettingService, opLog *OperationLogService) *AlertService`
- Produces: `(*AlertService) RunCheck(senders []notify.Sender) (*AlertCheckResult, error)` - pure-ish core (DB read/write, but senders are injected so it's mockable)
- Produces: `(*AlertService) BuildSenders() []notify.Sender` - reads settings, returns only the enabled+configured channels
- Produces: `(*AlertService) TestChannel(channel string) error` - `channel` one of `"wecom"`, `"email"`, `"webhook"`; builds that one sender from current settings regardless of its enabled flag (so admins can verify credentials before turning a channel on) and calls `.Test()`
- Produces: `AlertCheckResult{DomainsAlerted int, ChannelResults []ChannelResult}`, `ChannelResult{Channel string, OK bool, Error string}`

**Setting keys added** (follow the existing `SettingKeyXxx = "xxx"` pattern in `internal/service/setting.go`):

```go
SettingKeyAlertEnabled       = "alert_enabled"        // "0"/"1"
SettingKeyAlertThresholds    = "alert_thresholds"      // "30,15,7,3,1"
SettingKeyAlertCheckTime     = "alert_check_time"      // "09:00"
SettingKeyAlertWeComEnabled  = "alert_wecom_enabled"   // "0"/"1"
SettingKeyAlertWeComType     = "alert_wecom_type"      // "wecom" / "dingtalk" / "feishu" (仅影响前端提示文案)
SettingKeyAlertWeComURL      = "alert_wecom_url"
SettingKeyAlertEmailEnabled  = "alert_email_enabled"   // "0"/"1"
SettingKeyAlertEmailHost     = "alert_email_host"
SettingKeyAlertEmailPort     = "alert_email_port"      // 默认 "587"
SettingKeyAlertEmailUsername = "alert_email_username"
SettingKeyAlertEmailPassword = "alert_email_password"  // AES 加密存储
SettingKeyAlertEmailFrom     = "alert_email_from"
SettingKeyAlertEmailTo       = "alert_email_to"
SettingKeyAlertWebhookEnabled = "alert_webhook_enabled" // "0"/"1"
SettingKeyAlertWebhookURL     = "alert_webhook_url"
```

**Scope note:** per the design spec, only the SMTP password is encrypted-at-rest and masked on API read (Task 6). The WeCom/webhook URLs are stored and returned in plaintext, consistent with how every other Settings field in this codebase already behaves (e.g. `ssh_user`) - this is a single-admin settings page, not a multi-tenant secret store like cloud account credentials.

- [ ] **Step 1: Add the setting key constants**

In `backend/internal/service/setting.go`, add to the existing `const (...)` block (after `SettingKeyTheme`):

```go
	// SettingKeyAlertEnabled 是否启用域名到期告警
	SettingKeyAlertEnabled = "alert_enabled"
	// SettingKeyAlertThresholds 告警阈值天数，逗号分隔，如 "30,15,7,3,1"
	SettingKeyAlertThresholds = "alert_thresholds"
	// SettingKeyAlertCheckTime 每日检查时间，HH:MM 格式
	SettingKeyAlertCheckTime = "alert_check_time"
	// SettingKeyAlertWeComEnabled 是否启用企业微信/钉钉/飞书机器人告警
	SettingKeyAlertWeComEnabled = "alert_wecom_enabled"
	// SettingKeyAlertWeComType 机器人类型：wecom / dingtalk / feishu（仅影响前端提示文案，三者 payload 一致）
	SettingKeyAlertWeComType = "alert_wecom_type"
	// SettingKeyAlertWeComURL 机器人 Webhook 地址
	SettingKeyAlertWeComURL = "alert_wecom_url"
	// SettingKeyAlertEmailEnabled 是否启用邮件告警
	SettingKeyAlertEmailEnabled = "alert_email_enabled"
	SettingKeyAlertEmailHost     = "alert_email_host"
	SettingKeyAlertEmailPort     = "alert_email_port"
	SettingKeyAlertEmailUsername = "alert_email_username"
	// SettingKeyAlertEmailPassword SMTP 密码，AES 加密存储
	SettingKeyAlertEmailPassword = "alert_email_password"
	SettingKeyAlertEmailFrom     = "alert_email_from"
	SettingKeyAlertEmailTo       = "alert_email_to"
	// SettingKeyAlertWebhookEnabled 是否启用通用 Webhook 告警
	SettingKeyAlertWebhookEnabled = "alert_webhook_enabled"
	SettingKeyAlertWebhookURL     = "alert_webhook_url"
```

- [ ] **Step 2: Write the failing test for `RunCheck`**

Create `backend/internal/service/alert_test.go`:

```go
package service

import (
	"errors"
	"testing"
	"time"

	"cloudmanager/internal/auth"
	"cloudmanager/internal/model"
	"cloudmanager/internal/notify"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newAlertServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Domain{}, &model.DomainAlertState{}, &model.Setting{}, &model.OperationLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// fakeSender 记录收到的事件，可配置返回错误
type fakeSender struct {
	name       string
	failWith   error
	gotEvents  []notify.AlertEvent
	sendCalled int
}

func (f *fakeSender) Send(events []notify.AlertEvent) error {
	f.sendCalled++
	f.gotEvents = events
	return f.failWith
}
func (f *fakeSender) Test() error { return f.failWith }

func TestAlertService_RunCheck_AlertsOnceAndDedupsNextRun(t *testing.T) {
	db := newAlertServiceTestDB(t)
	enc := auth.NewAESEncryptor("test-key-32-bytes-long-enough!!!")
	setting := NewSettingService(db)
	opLog := NewOperationLogService(db)
	svc := NewAlertService(db, enc, setting, opLog)

	expiringSoon := time.Now().AddDate(0, 0, 5).Format("2006-01-02")
	if err := db.Create(&model.Domain{DomainName: "example.com", ExpiryDate: expiringSoon}).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}

	sender := &fakeSender{name: "fake"}

	result, err := svc.RunCheck([]notify.Sender{sender})
	if err != nil {
		t.Fatalf("RunCheck returned error: %v", err)
	}
	if result.DomainsAlerted != 1 {
		t.Fatalf("DomainsAlerted = %d, want 1", result.DomainsAlerted)
	}
	if sender.sendCalled != 1 {
		t.Fatalf("sender.Send called %d times, want 1", sender.sendCalled)
	}
	if len(sender.gotEvents) != 1 || sender.gotEvents[0].DomainName != "example.com" {
		t.Fatalf("got events = %+v, want one event for example.com", sender.gotEvents)
	}

	// 第二次运行：同一域名同一档位，不应重复告警
	sender2 := &fakeSender{name: "fake2"}
	result2, err := svc.RunCheck([]notify.Sender{sender2})
	if err != nil {
		t.Fatalf("second RunCheck returned error: %v", err)
	}
	if result2.DomainsAlerted != 0 {
		t.Fatalf("second run DomainsAlerted = %d, want 0 (dedup)", result2.DomainsAlerted)
	}
	if sender2.sendCalled != 0 {
		t.Fatalf("second run sender.Send called %d times, want 0", sender2.sendCalled)
	}
}

func TestAlertService_RunCheck_OneChannelFailureDoesNotBlockOthers(t *testing.T) {
	db := newAlertServiceTestDB(t)
	enc := auth.NewAESEncryptor("test-key-32-bytes-long-enough!!!")
	svc := NewAlertService(db, enc, NewSettingService(db), NewOperationLogService(db))

	expiringSoon := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	if err := db.Create(&model.Domain{DomainName: "fails-to-notify.com", ExpiryDate: expiringSoon}).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}

	failing := &fakeSender{name: "failing", failWith: errors.New("boom")}
	ok := &fakeSender{name: "ok"}

	result, err := svc.RunCheck([]notify.Sender{failing, ok})
	if err != nil {
		t.Fatalf("RunCheck returned error: %v", err)
	}
	if ok.sendCalled != 1 {
		t.Fatalf("ok sender should still be called, got sendCalled=%d", ok.sendCalled)
	}
	if len(result.ChannelResults) != 2 {
		t.Fatalf("ChannelResults = %+v, want 2 entries", result.ChannelResults)
	}
	var sawFailure, sawSuccess bool
	for _, r := range result.ChannelResults {
		if !r.OK && r.Error != "" {
			sawFailure = true
		}
		if r.OK {
			sawSuccess = true
		}
	}
	if !sawFailure || !sawSuccess {
		t.Fatalf("ChannelResults = %+v, want one failure and one success", result.ChannelResults)
	}
}

func TestAlertService_RunCheck_SkipsDomainsNotYetInAnyThreshold(t *testing.T) {
	db := newAlertServiceTestDB(t)
	enc := auth.NewAESEncryptor("test-key-32-bytes-long-enough!!!")
	svc := NewAlertService(db, enc, NewSettingService(db), NewOperationLogService(db))

	farFuture := time.Now().AddDate(1, 0, 0).Format("2006-01-02")
	if err := db.Create(&model.Domain{DomainName: "safe.com", ExpiryDate: farFuture}).Error; err != nil {
		t.Fatalf("seed domain: %v", err)
	}

	sender := &fakeSender{}
	result, err := svc.RunCheck([]notify.Sender{sender})
	if err != nil {
		t.Fatalf("RunCheck returned error: %v", err)
	}
	if result.DomainsAlerted != 0 || sender.sendCalled != 0 {
		t.Fatalf("expected no alert for a domain a year out, got DomainsAlerted=%d sendCalled=%d",
			result.DomainsAlerted, sender.sendCalled)
	}
}
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd backend && go test ./internal/service/... -run TestAlertService -v`
Expected: `FAIL` - `NewAlertService` undefined.

- [ ] **Step 4: Implement `AlertService`**

Create `backend/internal/service/alert.go`:

```go
package service

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"cloudmanager/internal/auth"
	"cloudmanager/internal/model"
	"cloudmanager/internal/notify"
	"cloudmanager/internal/repository"

	"gorm.io/gorm"
)

// errUnknownChannelOrNotConfigured TestChannel 收到未知渠道名，或该渠道必要字段未填写时返回
var errUnknownChannelOrNotConfigured = errors.New("未知的告警渠道，或该渠道尚未填写必要配置")

type AlertService struct {
	db      *gorm.DB
	enc     *auth.AESEncryptor
	setting *SettingService
	opLog   *OperationLogService
}

func NewAlertService(db *gorm.DB, enc *auth.AESEncryptor, setting *SettingService, opLog *OperationLogService) *AlertService {
	return &AlertService{db: db, enc: enc, setting: setting, opLog: opLog}
}

type ChannelResult struct {
	Channel string
	OK      bool
	Error   string
}

type AlertCheckResult struct {
	DomainsAlerted int
	ChannelResults []ChannelResult
}

// RunCheck 扫描全部已同步域名，按阈值分级去重后把命中的事件合并推送给每个传入的 sender。
// senders 由调用方构造（见 BuildSenders），便于单测注入假实现。
func (s *AlertService) RunCheck(senders []notify.Sender) (*AlertCheckResult, error) {
	thresholds := ParseThresholds(s.setting.GetString(SettingKeyAlertThresholds, ""))

	domainRepo := repository.NewDomainRepository(s.db)
	alertRepo := repository.NewDomainAlertRepository(s.db)

	domains, err := domainRepo.ListAll()
	if err != nil {
		return nil, err
	}

	var events []notify.AlertEvent
	now := time.Now().Format("2006-01-02 15:04:05")

	for _, d := range domains {
		days, ok := model.DaysUntilExpiry(d.ExpiryDate)
		if !ok {
			continue
		}
		state, err := alertRepo.Get(d.ID)
		if err != nil {
			continue // 单个域名查询失败不影响整体扫描
		}
		next, shouldAlert := NextAlertState(days, thresholds, state.LastThreshold)
		if next != state.LastThreshold {
			alertedAt := state.AlertedAt
			if shouldAlert {
				alertedAt = now
			}
			_ = alertRepo.Save(&model.DomainAlertState{DomainID: d.ID, LastThreshold: next, AlertedAt: alertedAt})
		}
		if shouldAlert {
			events = append(events, notify.AlertEvent{
				DomainName: d.DomainName,
				ExpiryDate: d.ExpiryDate,
				DaysLeft:   days,
				Threshold:  next,
			})
		}
	}

	result := &AlertCheckResult{DomainsAlerted: len(events)}

	if len(events) > 0 {
		for _, sender := range senders {
			cr := ChannelResult{Channel: senderName(sender), OK: true}
			if err := sender.Send(events); err != nil {
				cr.OK = false
				cr.Error = err.Error()
			}
			result.ChannelResults = append(result.ChannelResults, cr)
		}
	}

	s.recordCheckLog(result)
	return result, nil
}

// senderName 用于日志/结果展示；按具体类型区分（顺序判断即可，三种类型互斥）
func senderName(s notify.Sender) string {
	switch s.(type) {
	case *notify.WeComSender:
		return "wecom"
	case *notify.SMTPSender:
		return "email"
	case *notify.GenericWebhookSender:
		return "webhook"
	default:
		return "unknown"
	}
}

func (s *AlertService) recordCheckLog(result *AlertCheckResult) {
	status := "success"
	var parts []string
	parts = append(parts, "命中 "+strconv.Itoa(result.DomainsAlerted)+" 个域名")
	for _, cr := range result.ChannelResults {
		if cr.OK {
			parts = append(parts, cr.Channel+": 成功")
		} else {
			status = "failed"
			parts = append(parts, cr.Channel+": 失败("+cr.Error+")")
		}
	}
	_ = s.opLog.Record(&model.OperationLog{
		Action: "domain_alert.check",
		Target: "domain_alert",
		Detail: strings.Join(parts, "; "),
		Status: status,
	})
}

// BuildSenders 按当前设置构造已启用的告警渠道
func (s *AlertService) BuildSenders() []notify.Sender {
	var senders []notify.Sender
	if s.setting.GetInt(SettingKeyAlertWeComEnabled, 0) == 1 {
		if url := s.setting.GetString(SettingKeyAlertWeComURL, ""); url != "" {
			senders = append(senders, notify.NewWeComSender(url))
		}
	}
	if s.setting.GetInt(SettingKeyAlertEmailEnabled, 0) == 1 {
		if sender := s.buildEmailSender(); sender != nil {
			senders = append(senders, sender)
		}
	}
	if s.setting.GetInt(SettingKeyAlertWebhookEnabled, 0) == 1 {
		if url := s.setting.GetString(SettingKeyAlertWebhookURL, ""); url != "" {
			senders = append(senders, notify.NewGenericWebhookSender(url))
		}
	}
	return senders
}

func (s *AlertService) buildEmailSender() *notify.SMTPSender {
	host := s.setting.GetString(SettingKeyAlertEmailHost, "")
	if host == "" {
		return nil
	}
	encPassword := s.setting.GetString(SettingKeyAlertEmailPassword, "")
	password, err := s.enc.Decrypt(encPassword)
	if err != nil {
		password = ""
	}
	return notify.NewSMTPSender(
		host,
		s.setting.GetInt(SettingKeyAlertEmailPort, 587),
		s.setting.GetString(SettingKeyAlertEmailUsername, ""),
		password,
		s.setting.GetString(SettingKeyAlertEmailFrom, ""),
		s.setting.GetString(SettingKeyAlertEmailTo, ""),
	)
}

// TestChannel 用当前设置里的凭据（不管是否启用）测试某一个渠道，供 Settings 页"发送测试"按钮使用
func (s *AlertService) TestChannel(channel string) error {
	var sender notify.Sender
	switch channel {
	case "wecom":
		sender = notify.NewWeComSender(s.setting.GetString(SettingKeyAlertWeComURL, ""))
	case "email":
		sender = s.buildEmailSender()
	case "webhook":
		sender = notify.NewGenericWebhookSender(s.setting.GetString(SettingKeyAlertWebhookURL, ""))
	}
	if sender == nil {
		return errUnknownChannelOrNotConfigured
	}
	return sender.Test()
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `cd backend && go test ./internal/service/... -run TestAlertService -v`
Expected: all `PASS`.

- [ ] **Step 6: Run the full service package test suite**

Run: `cd backend && go test ./internal/service/... -v`
Expected: everything from Task 2 and Task 4 passes (`TestNextAlertState`, `TestParseThresholds`, `TestAlertService_*`).

- [ ] **Step 7: Build the whole backend**

Run: `cd backend && go build ./...`
Expected: no output (success).

- [ ] **Step 8: Commit**

```bash
git add backend/internal/service/setting.go backend/internal/service/alert.go backend/internal/service/alert_test.go
git commit -m "Add AlertService: scan, per-channel dispatch, and channel testing"
```

---

### Task 5: Daily scheduler + wiring into `Services`/`main.go`

**Files:**
- Create: `backend/internal/service/alert_schedule.go`
- Create: `backend/internal/service/alert_schedule_test.go`
- Modify: `backend/internal/service/services.go`
- Modify: `backend/cmd/server/main.go`

**Interfaces:**
- Consumes: `(*AlertService) RunCheck`, `(*AlertService) BuildSenders` (Task 4); `logging.Recover` (already exists in `internal/logging`, used the same way as `CloudAccount.StartAutoSync`).
- Produces: `nextRunTime(now time.Time, hhmm string) time.Time` (pure, testable)
- Produces: `(*AlertService) StartDailyCheck()` - starts the background goroutine (mirrors `CloudAccountService.StartAutoSync`'s pattern in `internal/service/cloud_account.go`)
- Produces: `Services.Alert *AlertService` field, constructed in `service.New`

- [ ] **Step 1: Write the failing test for `nextRunTime`**

Create `backend/internal/service/alert_schedule_test.go`:

```go
package service

import (
	"testing"
	"time"
)

func TestNextRunTime(t *testing.T) {
	loc := time.Local

	cases := []struct {
		name string
		now  time.Time
		hhmm string
		want time.Time
	}{
		{
			name: "later today",
			now:  time.Date(2026, 8, 11, 7, 0, 0, 0, loc),
			hhmm: "09:00",
			want: time.Date(2026, 8, 11, 9, 0, 0, 0, loc),
		},
		{
			name: "already passed today, rolls to tomorrow",
			now:  time.Date(2026, 8, 11, 10, 0, 0, 0, loc),
			hhmm: "09:00",
			want: time.Date(2026, 8, 12, 9, 0, 0, 0, loc),
		},
		{
			name: "exactly now rolls to tomorrow",
			now:  time.Date(2026, 8, 11, 9, 0, 0, 0, loc),
			hhmm: "09:00",
			want: time.Date(2026, 8, 12, 9, 0, 0, 0, loc),
		},
		{
			name: "invalid format falls back to 09:00 later today",
			now:  time.Date(2026, 8, 11, 7, 0, 0, 0, loc),
			hhmm: "not-a-time",
			want: time.Date(2026, 8, 11, 9, 0, 0, 0, loc),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nextRunTime(tc.now, tc.hhmm)
			if !got.Equal(tc.want) {
				t.Fatalf("nextRunTime(%v, %q) = %v, want %v", tc.now, tc.hhmm, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/service/... -run TestNextRunTime -v`
Expected: `FAIL` - `nextRunTime` undefined.

- [ ] **Step 3: Implement `nextRunTime` and `StartDailyCheck`**

Create `backend/internal/service/alert_schedule.go`:

```go
package service

import (
	"log"
	"strconv"
	"strings"
	"time"

	"cloudmanager/internal/logging"
)

// nextRunTime 计算下一次检查时间：今天该时刻还没到就是今天，否则是明天。
// hhmm 解析失败时回退到 "09:00"。
func nextRunTime(now time.Time, hhmm string) time.Time {
	hour, minute := 9, 0
	parts := strings.SplitN(hhmm, ":", 2)
	if len(parts) == 2 {
		if h, err := strconv.Atoi(parts[0]); err == nil {
			if m, err := strconv.Atoi(parts[1]); err == nil {
				hour, minute = h, m
			}
		}
	}

	candidate := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !candidate.After(now) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate
}

// StartDailyCheck 启动域名到期告警的每日调度：每天固定时间（设置项 alert_check_time，
// 默认 09:00）扫描一次全部域名。alert_enabled=0 时跳过本轮但调度继续运行，
// 修改设置后无需重启，下一轮循环会重新读取。
func (s *AlertService) StartDailyCheck() {
	go func() {
		defer logging.Recover("domain-alert-check")
		for {
			checkTime := s.setting.GetString(SettingKeyAlertCheckTime, "09:00")
			sleepDuration := time.Until(nextRunTime(time.Now(), checkTime))
			time.Sleep(sleepDuration)

			if s.setting.GetInt(SettingKeyAlertEnabled, 0) != 1 {
				continue
			}
			func() {
				defer logging.Recover("domain-alert-check-run")
				senders := s.BuildSenders()
				if len(senders) == 0 {
					return
				}
				if _, err := s.RunCheck(senders); err != nil {
					log.Printf("[warn] 域名到期检查失败: %v", err)
				}
			}()
		}
	}()
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go test ./internal/service/... -run TestNextRunTime -v`
Expected: all subtests `PASS`.

- [ ] **Step 5: Wire `AlertService` into `Services`**

In `backend/internal/service/services.go`, add the field and constructor call:

```go
type Services struct {
	DB        *gorm.DB
	Cfg       *config.Config
	JWT       *auth.JWTManager
	Encryptor *auth.AESEncryptor

	Auth         *AuthService
	Setting      *SettingService
	CloudAccount *CloudAccountService
	Server       *ServerService
	Domain       *DomainService
	DNSRecord    *DNSRecordService
	Dashboard    *DashboardService
	OperationLog *OperationLogService
	Alert        *AlertService
}
```

and in `New`, after `s.OperationLog = NewOperationLogService(db)`:

```go
	s.OperationLog = NewOperationLogService(db)
	s.Alert = NewAlertService(db, enc, s.Setting, s.OperationLog)
```

- [ ] **Step 6: Start the scheduler in `main.go`**

In `backend/cmd/server/main.go`, right after `svc.CloudAccount.StartAutoSync()`:

```go
	// 启动自动同步调度器（按设置中的 sync_interval 定时同步全部云账号）
	svc.CloudAccount.StartAutoSync()

	// 启动域名到期告警调度器（按设置中的 alert_check_time 每天检查一次）
	svc.Alert.StartDailyCheck()
```

- [ ] **Step 7: Build and run the full backend test suite**

Run: `cd backend && go build ./... && go test ./... -v`
Expected: build succeeds with no output; every test across `internal/repository`, `internal/service`, `internal/notify` passes.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/service/alert_schedule.go backend/internal/service/alert_schedule_test.go \
  backend/internal/service/services.go backend/cmd/server/main.go
git commit -m "Wire domain expiry alert daily scheduler into service container and main"
```

---

### Task 6: Backend Settings API - Get/Update fields + test-channel endpoint

**Files:**
- Modify: `backend/internal/api/handler/setting.go`
- Modify: `backend/internal/api/router.go`

**Interfaces:**
- Consumes: `service.SettingKeyAlert*` constants (Task 4), `Services.Alert.TestChannel` (Task 4).
- Produces: `GET /api/settings` response gains alert fields; `PUT /api/settings` accepts them; new `POST /api/settings/alert/test` handler.

**Manual verification only** (documented at the end) - this task modifies gin HTTP handlers directly; the codebase has no existing handler-test scaffolding (see `CLAUDE.md`: no test suite configured) and adding `httptest`-based gin route testing here would be new infrastructure disproportionate to this feature. Verified instead via `curl` against a running dev server, consistent with how every other backend change in this project has been checked so far.

- [ ] **Step 1: Extend `GetSettings`**

In `backend/internal/api/handler/setting.go`, add to the `gin.H{...}` returned by `GetSettings` (after `"theme": ...`):

```go
		"theme":                h.Svc.Setting.GetString(service.SettingKeyTheme, "light"),

		"alert_enabled":          h.Svc.Setting.GetInt(service.SettingKeyAlertEnabled, 0) == 1,
		"alert_thresholds":       h.Svc.Setting.GetString(service.SettingKeyAlertThresholds, "30,15,7,3,1"),
		"alert_check_time":       h.Svc.Setting.GetString(service.SettingKeyAlertCheckTime, "09:00"),
		"alert_wecom_enabled":    h.Svc.Setting.GetInt(service.SettingKeyAlertWeComEnabled, 0) == 1,
		"alert_wecom_type":       h.Svc.Setting.GetString(service.SettingKeyAlertWeComType, "wecom"),
		"alert_wecom_url":        h.Svc.Setting.GetString(service.SettingKeyAlertWeComURL, ""),
		"alert_email_enabled":    h.Svc.Setting.GetInt(service.SettingKeyAlertEmailEnabled, 0) == 1,
		"alert_email_host":       h.Svc.Setting.GetString(service.SettingKeyAlertEmailHost, ""),
		"alert_email_port":       h.Svc.Setting.GetInt(service.SettingKeyAlertEmailPort, 587),
		"alert_email_username":   h.Svc.Setting.GetString(service.SettingKeyAlertEmailUsername, ""),
		"alert_email_password_set": h.Svc.Setting.GetString(service.SettingKeyAlertEmailPassword, "") != "",
		"alert_email_from":       h.Svc.Setting.GetString(service.SettingKeyAlertEmailFrom, ""),
		"alert_email_to":         h.Svc.Setting.GetString(service.SettingKeyAlertEmailTo, ""),
		"alert_webhook_enabled":  h.Svc.Setting.GetInt(service.SettingKeyAlertWebhookEnabled, 0) == 1,
		"alert_webhook_url":      h.Svc.Setting.GetString(service.SettingKeyAlertWebhookURL, ""),
```

Note `alert_email_password_set` (bool), not the password itself - mirrors the design spec's "接口返回时脱敏" requirement without inventing a masked-string convention that doesn't exist elsewhere in this codebase.

- [ ] **Step 2: Extend `UpdateSettings`**

In the same file, add fields to the request struct:

```go
	var req struct {
		SyncConcurrency *int    `json:"sync_concurrency"`
		SyncInterval    *int    `json:"sync_interval"`
		SSHUser         *string `json:"ssh_user"`
		SSHPort         *int    `json:"ssh_port"`
		SessionTimeout  *int    `json:"session_timeout"`
		PlatformName    *string `json:"platform_name"`
		Theme           *string `json:"theme"`

		AlertEnabled       *bool   `json:"alert_enabled"`
		AlertThresholds    *string `json:"alert_thresholds"`
		AlertCheckTime     *string `json:"alert_check_time"`
		AlertWeComEnabled  *bool   `json:"alert_wecom_enabled"`
		AlertWeComType     *string `json:"alert_wecom_type"`
		AlertWeComURL      *string `json:"alert_wecom_url"`
		AlertEmailEnabled  *bool   `json:"alert_email_enabled"`
		AlertEmailHost     *string `json:"alert_email_host"`
		AlertEmailPort     *int    `json:"alert_email_port"`
		AlertEmailUsername *string `json:"alert_email_username"`
		AlertEmailPassword *string `json:"alert_email_password"` // 非空才更新，留空表示不修改
		AlertEmailFrom     *string `json:"alert_email_from"`
		AlertEmailTo       *string `json:"alert_email_to"`
		AlertWebhookEnabled *bool  `json:"alert_webhook_enabled"`
		AlertWebhookURL     *string `json:"alert_webhook_url"`
	}
```

and, before the final `if len(changed) > 0 {`, add:

```go
	boolToFlag := func(b bool) string {
		if b {
			return "1"
		}
		return "0"
	}

	if req.AlertEnabled != nil {
		h.Svc.Setting.Set(service.SettingKeyAlertEnabled, boolToFlag(*req.AlertEnabled))
		changed = append(changed, "alert_enabled="+boolToFlag(*req.AlertEnabled))
	}
	if req.AlertThresholds != nil {
		h.Svc.Setting.Set(service.SettingKeyAlertThresholds, strings.TrimSpace(*req.AlertThresholds))
		changed = append(changed, "alert_thresholds="+*req.AlertThresholds)
	}
	if req.AlertCheckTime != nil {
		t := strings.TrimSpace(*req.AlertCheckTime)
		if _, err := time.Parse("15:04", t); err != nil {
			FailMsg(c, "检查时间格式需为 HH:MM")
			return
		}
		h.Svc.Setting.Set(service.SettingKeyAlertCheckTime, t)
		changed = append(changed, "alert_check_time="+t)
	}
	if req.AlertWeComEnabled != nil {
		h.Svc.Setting.Set(service.SettingKeyAlertWeComEnabled, boolToFlag(*req.AlertWeComEnabled))
		changed = append(changed, "alert_wecom_enabled="+boolToFlag(*req.AlertWeComEnabled))
	}
	if req.AlertWeComType != nil {
		h.Svc.Setting.Set(service.SettingKeyAlertWeComType, strings.TrimSpace(*req.AlertWeComType))
	}
	if req.AlertWeComURL != nil {
		h.Svc.Setting.Set(service.SettingKeyAlertWeComURL, strings.TrimSpace(*req.AlertWeComURL))
		changed = append(changed, "alert_wecom_url updated")
	}
	if req.AlertEmailEnabled != nil {
		h.Svc.Setting.Set(service.SettingKeyAlertEmailEnabled, boolToFlag(*req.AlertEmailEnabled))
		changed = append(changed, "alert_email_enabled="+boolToFlag(*req.AlertEmailEnabled))
	}
	if req.AlertEmailHost != nil {
		h.Svc.Setting.Set(service.SettingKeyAlertEmailHost, strings.TrimSpace(*req.AlertEmailHost))
	}
	if req.AlertEmailPort != nil {
		h.Svc.Setting.Set(service.SettingKeyAlertEmailPort, strconv.Itoa(*req.AlertEmailPort))
	}
	if req.AlertEmailUsername != nil {
		h.Svc.Setting.Set(service.SettingKeyAlertEmailUsername, strings.TrimSpace(*req.AlertEmailUsername))
	}
	if req.AlertEmailPassword != nil && *req.AlertEmailPassword != "" {
		enc, err := h.Svc.Encryptor.Encrypt(*req.AlertEmailPassword)
		if err != nil {
			FailMsg(c, "邮箱密码加密失败")
			return
		}
		h.Svc.Setting.Set(service.SettingKeyAlertEmailPassword, enc)
		changed = append(changed, "alert_email_password updated")
	}
	if req.AlertEmailFrom != nil {
		h.Svc.Setting.Set(service.SettingKeyAlertEmailFrom, strings.TrimSpace(*req.AlertEmailFrom))
	}
	if req.AlertEmailTo != nil {
		h.Svc.Setting.Set(service.SettingKeyAlertEmailTo, strings.TrimSpace(*req.AlertEmailTo))
	}
	if req.AlertWebhookEnabled != nil {
		h.Svc.Setting.Set(service.SettingKeyAlertWebhookEnabled, boolToFlag(*req.AlertWebhookEnabled))
		changed = append(changed, "alert_webhook_enabled="+boolToFlag(*req.AlertWebhookEnabled))
	}
	if req.AlertWebhookURL != nil {
		h.Svc.Setting.Set(service.SettingKeyAlertWebhookURL, strings.TrimSpace(*req.AlertWebhookURL))
		changed = append(changed, "alert_webhook_url updated")
	}
```

Add `"strings"`, `"strconv"`, and `"time"` to the file's imports if not already present (`strconv` and `strings` are already imported; add `"time"`).

- [ ] **Step 3: Add the test-channel handler**

At the end of `backend/internal/api/handler/setting.go`, add:

```go
// TestAlertChannel 测试某个到期告警渠道的当前配置是否可用（Settings 页"发送测试"按钮）
func (h *H) TestAlertChannel(c *gin.Context) {
	var req struct {
		Channel string `json:"channel" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		FailMsg(c, "参数错误")
		return
	}
	if err := h.Svc.Alert.TestChannel(req.Channel); err != nil {
		JSON(c, gin.H{"success": false, "error": err.Error()})
		return
	}
	JSON(c, gin.H{"success": true})
}
```

- [ ] **Step 4: Register the route**

In `backend/internal/api/router.go`, add next to the other `/settings` routes:

```go
			auth.GET("/settings", h.GetSettings)
			auth.PUT("/settings", h.UpdateSettings)
			auth.POST("/settings/alert/test", h.TestAlertChannel)
```

- [ ] **Step 5: Build**

Run: `cd backend && go build ./...`
Expected: no output (success).

- [ ] **Step 6: Manual verification with curl**

Run the server against a scratch DB (same pattern used throughout this project's manual testing):

```bash
cd backend
go build -o bin/cloud-manager.exe ./cmd/server
DB_PATH=./scratch-test.db GIN_MODE=debug PORT=8098 ./bin/cloud-manager.exe &
sleep 2
TOKEN=$(curl -s -X POST http://localhost:8098/api/auth/login -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | grep -oE '"token":"[^"]+"' | cut -d'"' -f4)

# Update alert settings
curl -s -X PUT http://localhost:8098/api/settings -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"alert_enabled":true,"alert_thresholds":"30,15,7,3,1","alert_check_time":"09:00","alert_webhook_enabled":true,"alert_webhook_url":"http://localhost:9999/hook"}'

# Confirm it round-trips
curl -s http://localhost:8098/api/settings -H "Authorization: Bearer $TOKEN"

# Test the webhook channel (expect success:false since nothing is listening on :9999 - verifies the
# error path reaches the API without crashing the server)
curl -s -X POST http://localhost:8098/api/settings/alert/test -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" -d '{"channel":"webhook"}'
```

Expected: `PUT` returns `{"code":0,...}`; `GET` echoes back `alert_enabled: true`, `alert_thresholds: "30,15,7,3,1"`, `alert_webhook_url` set; the test-channel call returns `{"code":0,"message":"ok","data":{"success":false,"error":"..."}}` (connection refused, since nothing listens on :9999 - this confirms the endpoint round-trips through `AlertService.TestChannel` correctly).

Then stop the server and remove the scratch DB:

```bash
kill %1
rm -f backend/scratch-test.db
```

- [ ] **Step 7: Commit**

```bash
git add backend/internal/api/handler/setting.go backend/internal/api/router.go
git commit -m "Add domain expiry alert fields to Settings API and a channel-test endpoint"
```

---

### Task 7: Frontend - types and API client

**Files:**
- Modify: `frontend/src/api/index.ts`

**Interfaces:**
- Produces: `SettingsData` gains the alert fields; `settingsApi.testAlertChannel(channel: 'wecom' | 'email' | 'webhook')`.

- [ ] **Step 1: Extend `SettingsData`**

In `frontend/src/api/index.ts`, extend the interface:

```ts
export interface SettingsData {
  system_name: string
  platform_name: string
  version: string
  sync_interval: number
  sync_concurrency: number
  sync_concurrency_min: number
  sync_concurrency_max: number
  ssh_user: string
  ssh_port: number
  session_timeout: number
  theme: 'light' | 'dark'

  alert_enabled: boolean
  alert_thresholds: string
  alert_check_time: string
  alert_wecom_enabled: boolean
  alert_wecom_type: 'wecom' | 'dingtalk' | 'feishu'
  alert_wecom_url: string
  alert_email_enabled: boolean
  alert_email_host: string
  alert_email_port: number
  alert_email_username: string
  alert_email_password_set: boolean
  alert_email_from: string
  alert_email_to: string
  alert_webhook_enabled: boolean
  alert_webhook_url: string
}
```

- [ ] **Step 2: Add the test-channel API call**

In the same file, extend `settingsApi`:

```ts
export const settingsApi = {
  get: () => client.get<unknown, SettingsData>('/settings'),
  update: (data: Partial<SettingsData>) => client.put('/settings', data),
  testAlertChannel: (channel: 'wecom' | 'email' | 'webhook') =>
    client.post<unknown, { success: boolean; error?: string }>('/settings/alert/test', { channel }),
}
```

Note: the request payload sent from the frontend uses the same field names as `SettingsData` except `alert_email_password` (write-only, not part of the read model) - that field is added directly in the request payload from `Settings.tsx` in Task 8 rather than in this shared type, since it never appears in a `GET` response.

- [ ] **Step 3: Typecheck**

Run: `cd frontend && pnpm build`
Expected: succeeds (this also runs `tsc --noEmit`). No new type errors - `Settings.tsx` doesn't reference the new fields yet, so nothing should break.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/api/index.ts
git commit -m "Add domain expiry alert fields and test-channel call to Settings API client"
```

---

### Task 8: Frontend - "到期告警" Settings tab

**Files:**
- Modify: `frontend/src/pages/Settings.tsx`

**Interfaces:**
- Consumes: `settingsApi.get/update/testAlertChannel` (Task 7), existing `Settings.tsx` local-state-per-tab + `saving` map pattern (see `handleSaveGeneral`/`handleSaveSync` etc. already in the file).

- [ ] **Step 1: Add imports and local state**

In `frontend/src/pages/Settings.tsx`, add to the antd import list: `Switch`, `Select`, `TimePicker`. Add `dayjs` import:

```ts
import {
  Card,
  Form,
  Input,
  Button,
  Alert,
  InputNumber,
  Tabs,
  Segmented,
  App,
  Space,
  Typography,
  Switch,
  Select,
  TimePicker,
} from 'antd'
import dayjs from 'dayjs'
```

After the existing `useState` block (after `const [sessionTimeout, setSessionTimeout] = useState(24)`), add:

```ts
  // 到期告警
  const [alertEnabled, setAlertEnabled] = useState(false)
  const [alertThresholds, setAlertThresholds] = useState('30,15,7,3,1')
  const [alertCheckTime, setAlertCheckTime] = useState('09:00')
  const [wecomEnabled, setWecomEnabled] = useState(false)
  const [wecomType, setWecomType] = useState<'wecom' | 'dingtalk' | 'feishu'>('wecom')
  const [wecomUrl, setWecomUrl] = useState('')
  const [emailEnabled, setEmailEnabled] = useState(false)
  const [emailHost, setEmailHost] = useState('')
  const [emailPort, setEmailPort] = useState(587)
  const [emailUsername, setEmailUsername] = useState('')
  const [emailPassword, setEmailPassword] = useState('') // 留空 = 不修改已保存的密码
  const [emailPasswordSet, setEmailPasswordSet] = useState(false)
  const [emailFrom, setEmailFrom] = useState('')
  const [emailTo, setEmailTo] = useState('')
  const [webhookEnabled, setWebhookEnabled] = useState(false)
  const [webhookUrl, setWebhookUrl] = useState('')
  const [testing, setTesting] = useState<Record<string, boolean>>({})
```

- [ ] **Step 2: Populate state on load**

In the `useEffect` that calls `settingsApi.get()`, add after `setSessionTimeout(s.session_timeout)`:

```ts
      setAlertEnabled(s.alert_enabled)
      setAlertThresholds(s.alert_thresholds)
      setAlertCheckTime(s.alert_check_time)
      setWecomEnabled(s.alert_wecom_enabled)
      setWecomType(s.alert_wecom_type)
      setWecomUrl(s.alert_wecom_url)
      setEmailEnabled(s.alert_email_enabled)
      setEmailHost(s.alert_email_host)
      setEmailPort(s.alert_email_port)
      setEmailUsername(s.alert_email_username)
      setEmailPasswordSet(s.alert_email_password_set)
      setEmailFrom(s.alert_email_from)
      setEmailTo(s.alert_email_to)
      setWebhookEnabled(s.alert_webhook_enabled)
      setWebhookUrl(s.alert_webhook_url)
```

- [ ] **Step 3: Add the save handler**

After `handleSaveSecurity`, add:

```ts
  // 到期告警
  const handleSaveAlert = async () => {
    if (!data) return
    setSavingKey('alert', true)
    try {
      const payload: Record<string, unknown> = {
        alert_enabled: alertEnabled,
        alert_thresholds: alertThresholds,
        alert_check_time: alertCheckTime,
        alert_wecom_enabled: wecomEnabled,
        alert_wecom_type: wecomType,
        alert_wecom_url: wecomUrl,
        alert_email_enabled: emailEnabled,
        alert_email_host: emailHost,
        alert_email_port: emailPort,
        alert_email_username: emailUsername,
        alert_email_from: emailFrom,
        alert_email_to: emailTo,
        alert_webhook_enabled: webhookEnabled,
        alert_webhook_url: webhookUrl,
      }
      if (emailPassword) {
        payload.alert_email_password = emailPassword
      }
      await settingsApi.update(payload)
      if (emailPassword) {
        setEmailPasswordSet(true)
        setEmailPassword('')
      }
      message.success('到期告警设置已保存')
    } finally {
      setSavingKey('alert', false)
    }
  }

  const handleTestChannel = async (channel: 'wecom' | 'email' | 'webhook') => {
    setTesting((t) => ({ ...t, [channel]: true }))
    try {
      const res = await settingsApi.testAlertChannel(channel)
      if (res.success) {
        message.success('测试消息已发送，请查看是否收到')
      } else {
        message.error(`发送失败：${res.error || '未知错误'}`)
      }
    } finally {
      setTesting((t) => ({ ...t, [channel]: false }))
    }
  }
```

- [ ] **Step 4: Add the tab**

In the `items` array, after the `security` tab object (before the closing `]`), add:

```ts
    {
      key: 'alert',
      label: '到期告警',
      children: (
        <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <Card>
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 16 }}
              message="每天固定时间扫描全部已同步域名，按到期天数分级推送；同一域名同一告警等级只推送一次，续费后自动重置。"
            />
            <Space direction="vertical" size={20} style={{ width: '100%' }}>
              <div>
                <Space align="center">
                  <Switch checked={alertEnabled} onChange={setAlertEnabled} />
                  <Text strong>启用到期告警</Text>
                </Space>
              </div>
              <div>
                <Text strong>告警阈值（天，逗号分隔）</Text>
                <div style={{ marginTop: 8 }}>
                  <Input
                    value={alertThresholds}
                    onChange={(e) => setAlertThresholds(e.target.value)}
                    style={{ maxWidth: 280 }}
                    placeholder="30,15,7,3,1"
                  />
                </div>
              </div>
              <div>
                <Text strong>每日检查时间</Text>
                <div style={{ marginTop: 8 }}>
                  <TimePicker
                    value={dayjs(alertCheckTime, 'HH:mm')}
                    format="HH:mm"
                    onChange={(v) => setAlertCheckTime(v ? v.format('HH:mm') : '09:00')}
                    allowClear={false}
                  />
                </div>
              </div>
              <Button type="primary" loading={saving.alert} onClick={handleSaveAlert}>
                保存
              </Button>
            </Space>
          </Card>

          <Card title="企业微信 / 钉钉 / 飞书 机器人">
            <Space direction="vertical" size={20} style={{ width: '100%' }}>
              <Space align="center">
                <Switch checked={wecomEnabled} onChange={setWecomEnabled} />
                <Text strong>启用</Text>
              </Space>
              <div>
                <Text strong>机器人类型</Text>
                <div style={{ marginTop: 8 }}>
                  <Select
                    value={wecomType}
                    onChange={setWecomType}
                    style={{ width: 200 }}
                    options={[
                      { label: '企业微信', value: 'wecom' },
                      { label: '钉钉', value: 'dingtalk' },
                      { label: '飞书', value: 'feishu' },
                    ]}
                  />
                </div>
              </div>
              <div>
                <Text strong>Webhook 地址</Text>
                <div style={{ marginTop: 8 }}>
                  <Input
                    value={wecomUrl}
                    onChange={(e) => setWecomUrl(e.target.value)}
                    placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=..."
                  />
                </div>
              </div>
              <Space>
                <Button type="primary" loading={saving.alert} onClick={handleSaveAlert}>
                  保存
                </Button>
                <Button loading={testing.wecom} onClick={() => handleTestChannel('wecom')}>
                  发送测试
                </Button>
              </Space>
            </Space>
          </Card>

          <Card title="邮件（SMTP）">
            <Space direction="vertical" size={20} style={{ width: '100%' }}>
              <Space align="center">
                <Switch checked={emailEnabled} onChange={setEmailEnabled} />
                <Text strong>启用</Text>
              </Space>
              <Space wrap size={20}>
                <div>
                  <Text strong>SMTP 服务器</Text>
                  <div style={{ marginTop: 8 }}>
                    <Input value={emailHost} onChange={(e) => setEmailHost(e.target.value)} style={{ width: 220 }} placeholder="smtp.example.com" />
                  </div>
                </div>
                <div>
                  <Text strong>端口</Text>
                  <div style={{ marginTop: 8 }}>
                    <InputNumber min={1} max={65535} value={emailPort} onChange={(v) => setEmailPort(v ?? 587)} style={{ width: 120 }} />
                  </div>
                </div>
              </Space>
              <div>
                <Text strong>用户名</Text>
                <div style={{ marginTop: 8 }}>
                  <Input value={emailUsername} onChange={(e) => setEmailUsername(e.target.value)} style={{ maxWidth: 280 }} />
                </div>
              </div>
              <div>
                <Text strong>密码</Text>
                <div style={{ marginTop: 8 }}>
                  <Input.Password
                    value={emailPassword}
                    onChange={(e) => setEmailPassword(e.target.value)}
                    style={{ maxWidth: 280 }}
                    placeholder={emailPasswordSet ? '已设置，留空表示不修改' : '请输入 SMTP 密码'}
                  />
                </div>
              </div>
              <Space wrap size={20}>
                <div>
                  <Text strong>发件人</Text>
                  <div style={{ marginTop: 8 }}>
                    <Input value={emailFrom} onChange={(e) => setEmailFrom(e.target.value)} style={{ width: 220 }} placeholder="alerts@example.com" />
                  </div>
                </div>
                <div>
                  <Text strong>收件人（逗号分隔）</Text>
                  <div style={{ marginTop: 8 }}>
                    <Input value={emailTo} onChange={(e) => setEmailTo(e.target.value)} style={{ width: 260 }} placeholder="admin@example.com" />
                  </div>
                </div>
              </Space>
              <Space>
                <Button type="primary" loading={saving.alert} onClick={handleSaveAlert}>
                  保存
                </Button>
                <Button loading={testing.email} onClick={() => handleTestChannel('email')}>
                  发送测试
                </Button>
              </Space>
            </Space>
          </Card>

          <Card title="通用 Webhook">
            <Space direction="vertical" size={20} style={{ width: '100%' }}>
              <Space align="center">
                <Switch checked={webhookEnabled} onChange={setWebhookEnabled} />
                <Text strong>启用</Text>
              </Space>
              <div>
                <Text strong>Webhook 地址</Text>
                <div style={{ marginTop: 8 }}>
                  <Input value={webhookUrl} onChange={(e) => setWebhookUrl(e.target.value)} placeholder="https://your-system.example.com/hooks/domain-alert" />
                </div>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  会 POST 一个 JSON body：{'{ "events": [{ "DomainName", "ExpiryDate", "DaysLeft", "Threshold" }] }'}
                </Text>
              </div>
              <Space>
                <Button type="primary" loading={saving.alert} onClick={handleSaveAlert}>
                  保存
                </Button>
                <Button loading={testing.webhook} onClick={() => handleTestChannel('webhook')}>
                  发送测试
                </Button>
              </Space>
            </Space>
          </Card>
        </Space>
      ),
    },
```

- [ ] **Step 5: Build**

Run: `cd frontend && pnpm build`
Expected: succeeds, no type errors.

- [ ] **Step 6: Manual browser verification**

Start the backend (scratch DB, as in Task 6) and `pnpm dev`, log in, navigate to `/settings`, open the "到期告警" tab, and confirm:
- All fields render and are editable
- Toggling "启用" switches and clicking "保存" round-trips (reload the page, values persist)
- Clicking "发送测试" on the webhook channel with a bogus URL shows an error message (`message.error`); with `https://httpbin.org/post` (or any reachable URL) shows success

Take a screenshot of the new tab to confirm layout is not broken (reuse the Playwright pattern already used earlier in this project for UI verification).

- [ ] **Step 7: Commit**

```bash
git add frontend/src/pages/Settings.tsx
git commit -m "Add domain expiry alert configuration tab to Settings page"
```

---

### Task 9: End-to-end verification

**Files:** none (verification only, per the design spec's testing plan).

- [ ] **Step 1: Build the full release**

Run: `cd backend && go build ./... && go test ./...` then `cd ../frontend && pnpm build`
Expected: everything passes.

- [ ] **Step 2: Live check with a near-future check time**

Start the backend against a scratch DB (`DB_PATH=./scratch-e2e.db`), configure alert settings via the Settings UI (or `curl`) with `alert_check_time` set to a couple of minutes from now and `alert_enabled: true`, with the generic webhook pointed at a throwaway local listener (e.g. `python3 -m http.server` or a one-off `nc -l`) or a real reachable webhook.

Manually set an existing synced domain's `expiry_date` to within the configured threshold range directly in the DB:

```bash
sqlite3 backend/scratch-e2e.db "UPDATE domains SET expiry_date = date('now', '+5 days') WHERE id = 1;"
```

Wait for the configured check time to pass. Confirm:
- The webhook receives exactly one POST containing that domain
- `operation_logs` has a new `domain_alert.check` row (check via `GET /api/operation-logs` or the 操作日志 page)
- Waiting for a second check cycle (change `alert_check_time` again to a few minutes out) does **not** re-send the same domain (dedup working)

- [ ] **Step 3: Clean up scratch artifacts**

```bash
rm -f backend/scratch-e2e.db backend/scratch-test.db
```

- [ ] **Step 4: Final full-repo sanity build**

Run: `cd backend && go build ./... && go vet ./...` and `cd ../frontend && pnpm build`
Expected: both succeed with no errors.

This task has no commit of its own - it's verification of the already-committed Tasks 1-8.

---

## Self-Review Notes

- **Spec coverage:** dedup state (Task 1, 2), three channels (Task 3), scheduler + Settings API + UI (Tasks 4-8), error isolation between channels (Task 4 test `TestAlertService_RunCheck_OneChannelFailureDoesNotBlockOthers`), operation log summary per run (Task 4 `recordCheckLog`), manual test buttons (Task 3 `Test()`, Task 6 endpoint, Task 8 UI). All spec sections are covered.
- **Fixed during self-review:** an earlier draft of Task 4 Step 4 referenced a non-existent `fmtErrorf` helper as an intermediate step before correcting to `errors.New` - cleaned up so the task now shows only the correct, final code with `errors` in the import block from the start.
- **Type consistency:** `notify.Sender`, `notify.AlertEvent`, `AlertService.RunCheck/BuildSenders/TestChannel`, and the `SettingKeyAlert*` constants are named identically everywhere they're used across Tasks 3-6.

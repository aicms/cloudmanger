# 域名到期告警 — 设计文档

日期：2026-08-11

## 背景 / 问题

仪表盘目前只是**被动展示**即将到期的域名（30 天内），管理员必须主动打开页面才能看到，容易漏掉续费导致域名过期。需要一个**主动推送**的到期告警机制。

服务器（预付费实例）也有到期日概念，但本次不做，作为后续迭代——先把域名这个最紧迫的场景做完整。

## 目标

- 每天固定时间自动扫描全部已同步域名，按到期天数分级告警
- 支持三种通知渠道：企业微信/钉钉/飞书 Webhook、SMTP 邮件、通用 Webhook，三者可同时启用
- 同一域名同一告警等级只推送一次，续费后自动重置
- 全部通过 Settings 页配置，不需要改代码/环境变量

## 非目标

- 服务器到期告警（后续迭代）
- 告警发送历史/审计面板（用操作日志代替）
- 钉钉签名密钥模式（先支持免签的自定义机器人 Webhook）

## 数据模型

### 新表 `domain_alert_state`

```go
type DomainAlertState struct {
	DomainID      uint   `gorm:"primaryKey"`
	LastThreshold int    // 已告警过的最紧阈值（天数），0 表示"已过期"档位，-1 表示未告警过
	AlertedAt     string // 最近一次告警时间
}
```

**为什么单独建表，而不是加字段到 `Domain`**：`DomainRepository.Upsert` 每次同步都会对已存在的行执行 `gorm.Save`（整行覆盖式更新，见 `internal/repository/domain.go:63`），同步逻辑构造的 `Domain` struct 里没有告警相关字段，如果把状态放在 `Domain` 表里，会在下一次同步时被静默清零。独立表由告警逻辑单独读写，不受同步生命周期影响；`DomainID` 外键关联但不做级联约束（域名被删除时告警状态成为孤儿行，下次扫描前清理即可，不影响正确性）。

### 阈值分级逻辑

阈值列表默认 `[30, 15, 7, 3, 1]`（可在 Settings 修改，逗号分隔的正整数，从大到小排序后使用）。过期（剩余天数 < 0）视为额外的最紧档位，用 `LastThreshold = 0` 表示；未告警过用 `-1` 表示（区别于合法阈值都 ≥1）。

扫描每个域名时：

1. 计算 `daysLeft`（复用 `model.ComputeDomainStatus` 的日期解析逻辑）
2. 若 `daysLeft < 0`，目标档位 `bucket = 0`；否则在阈值列表中找到 **满足 `daysLeft <= T` 的最小 T** 作为 `bucket`；若都不满足（daysLeft 比最大阈值还大，说明在安全区），`bucket = -1`
3. 若 `bucket == -1`：说明当前处于安全区。若 `state.LastThreshold != -1`（此前告警过，现在已续费到安全区），把 `LastThreshold` 重置为 `-1`（不发送，只清状态，让未来重新进入告警周期）；否则什么都不做，跳过该域名
4. 若 `bucket != -1` 且比当前 `LastThreshold` 更紧（即 `state.LastThreshold == -1` 或 `bucket < state.LastThreshold`），触发一次告警，更新 `LastThreshold = bucket`
5. 若 `bucket != -1` 但不比当前 `LastThreshold` 更紧（`bucket >= state.LastThreshold`，例如天数在同一档位内小幅波动），不重复发送，也不改状态

这样保证：每个阈值档位只推送一次；完全续费到安全区会清空状态；下次再次进入到期区间会重新走一遍完整的告警周期。

## 通知渠道

新增 Setting 项（复用现有 `settings` 表的 key-value 存储方式，与 `sync_interval` 等一致），新建 `internal/notify` 包封装发送逻辑：

```go
package notify

type AlertEvent struct {
	DomainName string
	ExpiryDate string
	DaysLeft   int
	Threshold  int // 0 表示已过期
}

type Sender interface {
	Send(events []AlertEvent) error
	Test() error // Settings 页"发送测试"按钮用
}
```

三个实现：

- **`notify.WeCom`**（同时覆盖企业微信/飞书自定义机器人，二者 payload 一致：`{"msgtype":"text","text":{"content":"..."}}`；钉钉自定义机器人格式相同但字段名 `msgtype`/`text` 一致，三者可共用一个实现，只是 URL 不同）——用户在 Settings 选择"类型"仅用于展示提示文案，实际发送逻辑一致
- **`notify.SMTP`**（`net/smtp` + STARTTLS，标准库即可，不引入新依赖）
- **`notify.Webhook`**（POST 固定 JSON body：`{"domain":..., "expiry_date":..., "days_left":..., "threshold":...}`）

一次扫描周期内命中的所有告警事件合并成一条消息（而不是每个域名单独发一条),避免刷屏；三个渠道各自独立发送，互不影响。

## 调度

`internal/service` 新增 `AlertService`，`StartDailyCheck()` 在 `main.go` 里与 `CloudAccount.StartAutoSync()` 同级启动，goroutine 内 `defer logging.Recover("domain-alert-check")`。

调度方式：不使用 cron 库（YAGNI，标准库够用）——每次唤醒后计算"下一个配置的检查时间点"（今天该时间点已过则算明天），`time.Sleep` 到该时刻，执行一次扫描，循环。检查时间存储为 `HH:MM` 格式的字符串 setting（默认 `09:00`），修改后**不需要重启**，下一轮循环会重新读取。

## Settings 页

新增"到期告警"tab（`frontend/src/pages/Settings.tsx`），字段：

- 启用开关
- 阈值天数（逗号分隔输入框，如 `30,15,7,3,1`）
- 检查时间（TimePicker，`HH:MM`）
- 企业微信/钉钉/飞书：启用开关 + Webhook URL + 类型选择（仅影响提示文案）+ 发送测试按钮
- 邮件：启用开关 + host/port/username/password/from/to + 发送测试按钮
- 通用 Webhook：启用开关 + URL + 发送测试按钮

后端 `GetSettings`/`UpdateSettings` 按现有模式扩展对应字段；密码类字段（SMTP 密码）复用现有 AES 加密工具（`internal/auth.AESEncryptor`，云账号密钥同款）加密存储，接口返回时脱敏。

## 错误处理

- 单个渠道发送失败：记录 `log.Printf` 警告，不影响其他渠道，不影响 `LastThreshold` 更新（即：只要触发条件满足就更新状态，避免配置错误导致每天都在无效重试计算）——**取舍**：如果 Webhook 配置错误，用户会错过这一次告警，但下一个更紧的阈值到达时还会再触发一次，不会完全静默
- 扫描本身出错（数据库读取失败等）：整轮跳过，记录警告，下一天重试
- 每轮结束后，无论成功与否，写一条 `operation_log`（`domain_alert.check`），汇总本轮触发了几个域名、几个渠道成功/失败，方便在"操作日志"页排查

## 测试计划

- 后端：`ComputeAlertBucket`（阈值分级纯函数）单元测试覆盖边界（临界值、过期、续费回退）
- 手动验证：Settings 页三个"发送测试"按钮分别验证凭据可用
- 端到端：调低检查时间到几分钟后 + 手动改一条已同步域名的 `expiry_date` 到阈值内，观察是否收到告警且不重复

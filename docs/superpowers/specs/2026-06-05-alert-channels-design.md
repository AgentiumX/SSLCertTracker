# Plan 3.3a：告警渠道管理 — 设计规格

- **日期**：2026-06-05
- **作者**：chenjiefeng（与 Claude 头脑风暴）
- **版本**：v1.0
- **状态**：草案，待评审
- **关联**：
  - 拆分自 [SSL 证书监控系统 — 设计规格](2026-06-02-ssl-cert-tracker-design.md) 第 3.6 节 + 第 7.3 节
  - 依赖 [Plan 3.1 登录认证系统](2026-06-03-auth-system-design.md)
  - 依赖 [Plan 3.2 管理后台](2026-06-03-admin-backend-design.md)
  - 后续：Plan 3.3b（告警引擎）、Plan 3.3c（历史查询 + 清理）

---

## 1. 背景与目标

实现告警渠道的 CRUD 管理和测试发送功能，为 Plan 3.3b 的告警引擎提供发送基础设施。

**本期范围（Plan 3.3a）：**

- 后端：`alert_channels` 表 CRUD API + 5 种渠道适配器（webhook / dingtalk / feishu / wecom / email）+ 测试发送
- 前端：`/admin/channels` 渠道管理页（CRUD + 行内测试按钮）

**非目标（推迟到 Plan 3.3b / 3.3c）：**

- 告警引擎（状态机 + 状态变化触发 + 每日提醒）
- 历史查询页 + 历史清理
- 渠道按域名分组配置
- 发送结果入库

---

## 2. 架构

```
┌──────────────────────────────────────────────────────────────┐
│  Browser                                                     │
│  /admin/channels  渠道管理页（CRUD + textarea config + 测试） │
└──────────────────────────────────────────────────────────────┘
                          ↓
┌──────────────────────────────────────────────────────────────┐
│  Server                                                      │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  AlertChannelHandler (CRUD + test)                     │  │
│  └────────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  alert package                                         │  │
│  │  ├ Channel interface { Send / Test / ValidateConfig }  │  │
│  │  ├ webhook.go                                          │  │
│  │  ├ dingtalk.go                                         │  │
│  │  ├ feishu.go                                           │  │
│  │  ├ wecom.go                                            │  │
│  │  ├ email.go                                            │  │
│  │  └ factory.go  (NewChannel factory)                    │  │
│  └────────────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  Store (alert_channels CRUD)                           │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

**职责切分：**

| 组件 | 职责 |
|------|------|
| `internal/alert/channel.go` | `Channel` 接口 + `Message` 结构体 + `NewChannel` 工厂函数 |
| `internal/alert/webhook.go` | Webhook 渠道适配器 |
| `internal/alert/dingtalk.go` | 钉钉渠道适配器（HMAC-SHA256 签名） |
| `internal/alert/feishu.go` | 飞书渠道适配器 |
| `internal/alert/wecom.go` | 企业微信渠道适配器 |
| `internal/alert/email.go` | Email 渠道适配器（SMTP） |
| `internal/api/alert_channel_handler.go` | CRUD + test handler |
| `internal/store/store.go` | AlertChannel CRUD 方法 |
| `web/src/views/AdminChannels.vue` | 渠道管理页 |

---

## 3. 数据模型

### 3.1 `alert_channels` 表（复用设计文档第 3.6 节）

```go
type AlertChannel struct {
    ID        uint      `gorm:"primaryKey"`
    Name      string    `gorm:"not null"`
    Type      string    `gorm:"not null"`     // webhook | dingtalk | feishu | wecom | email
    Config    string    `gorm:"not null;type:text"`  // JSON
    Enabled   bool      `gorm:"not null;default:true"`
    CreatedAt time.Time
}
```

AutoMigrate 在 `main.go` 中追加 `&store.AlertChannel{}`。

---

## 4. 渠道配置 schema

每种渠道的 config JSON 结构：

### 4.1 webhook

```json
{ "url": "https://example.com/hook" }
```

字段：
- `url`（必填）：Webhook URL

### 4.2 dingtalk

```json
{ "url": "https://oapi.dingtalk.com/robot/send?access_token=xxx", "secret": "SEC..." }
```

字段：
- `url`（必填）：钉钉机器人 Webhook URL
- `secret`（可选）：加签密钥（部分钉钉机器人不需要签名）

### 4.3 feishu

```json
{ "url": "https://open.feishu.cn/open-apis/bot/v2/hook/xxx" }
```

字段：
- `url`（必填）：飞书机器人 Webhook URL

### 4.4 wecom

```json
{ "url": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx" }
```

字段：
- `url`（必填）：企业微信机器人 Webhook URL

### 4.5 email

```json
{
  "smtp_host": "smtp.example.com",
  "smtp_port": 587,
  "username": "alert@example.com",
  "password": "xxx",
  "from": "alert@example.com",
  "to": ["admin@example.com"]
}
```

字段：
- `smtp_host`（必填）：SMTP 服务器地址
- `smtp_port`（必填）：SMTP 端口（常见：587 TLS、465 SSL、25 明文）
- `username`（必填）：SMTP 认证用户名
- `password`（必填）：SMTP 认证密码
- `from`（必填）：发件人地址
- `to`（必填）：收件人地址列表（至少 1 个）

---

## 5. Channel 接口

```go
package alert

// Message is the unified message payload sent through all channels.
type Message struct {
    Title string
    Body  string
}

// Channel is the interface all alert channel adapters must implement.
type Channel interface {
    // Send sends a message through the channel.
    Send(msg Message) error
    // Test sends a fixed test message to verify the channel configuration.
    Test() error
    // ValidateConfig validates the channel's JSON config string.
    ValidateConfig() error
}

// NewChannel creates a Channel adapter based on the channel type and config JSON.
func NewChannel(channelType string, configJSON string) (Channel, error)
```

**Test 消息模板：**
```
Title: "[SSL Tracker] 告警渠道测试"
Body:  "这是一条测试消息。如果您收到此消息，说明告警渠道配置正确。\n\n时间: {当前时间 RFC3339}"
```

**各渠道发送格式（遵循设计文档第 7.4 节）：**

| 渠道 | 格式 |
|------|------|
| webhook | JSON `{ "title": "...", "body": "..." }` POST 到 URL |
| dingtalk | Markdown 消息体 `{ "msgtype": "markdown", "markdown": { "title": "...", "text": "..." } }`，有 secret 时附加 HMAC-SHA256 签名 |
| feishu | Markdown 消息体 `{ "msg_type": "interactive", "card": { "header": { "title": { "tag": "plain_text", "content": "..." } }, "elements": [{ "tag": "markdown", "content": "..." }] } }` |
| wecom | Markdown 消息体 `{ "msgtype": "markdown", "markdown": { "content": "**{title}**\n{body}" } }` |
| email | HTML 邮件（`<h2>{title}</h2><pre>{body}</pre>`），使用 SMTP + STARTTLS（端口 587）或直接 TLS（端口 465） |

---

## 6. API 契约

> 错误响应统一格式：`{ "error": { "code": "string", "message": "string" } }`。所有端点在 `adminGroup` 下，受 `AuthMiddleware` 保护。

### 6.1 `GET /api/admin/alert-channels`

**响应：** `200`
```json
{
  "channels": [
    { "id": 1, "name": "钉钉运维群", "type": "dingtalk", "enabled": true, "created_at": "2026-06-05T10:00:00Z" }
  ]
}
```

**注意：** 列表**不返回 config 字段**（含敏感信息如密码/secret）。编辑时通过 `GET /:id` 获取。

### 6.2 `GET /api/admin/alert-channels/:id`

**响应：** `200`
```json
{ "id": 1, "name": "钉钉运维群", "type": "dingtalk", "config": "{\"url\":\"...\",\"secret\":\"...\"}", "enabled": true, "created_at": "..." }
```

返回含 config（编辑时需要）。

**错误：** `404 not_found`

### 6.3 `POST /api/admin/alert-channels`

**请求：**
```json
{ "name": "钉钉运维群", "type": "dingtalk", "config": "{\"url\":\"...\",\"secret\":\"...\"}", "enabled": true }
```

**字段约束：**
- `name`、`type`、`config` 必填
- `type` 必须是 `webhook` / `dingtalk` / `feishu` / `wecom` / `email` 之一，否则 `400 invalid_request`
- `config` 必须是合法 JSON 且通过对应渠道的 `ValidateConfig`，否则 `400 invalid_request`
- `enabled` 默认 `true`（如果未提供）

**成功响应：** `201 { "id": 1 }`

### 6.4 `PUT /api/admin/alert-channels/:id`

**请求：** 同 POST

**成功响应：** `200 { "ok": true }`

**错误：** `404 not_found`（ID 不存在）、`400 invalid_request`（验证失败）

### 6.5 `DELETE /api/admin/alert-channels/:id`

**成功响应：** `200 { "ok": true }`

### 6.6 `POST /api/admin/alert-channels/:id/test`

**流程：** 从 DB 加载渠道 → `NewChannel(type, config)` → `channel.Test()`

**成功响应：** `200 { "ok": true }`

**失败响应：** `400`
```json
{ "error": { "code": "send_failed", "message": "具体错误信息（如连接超时、认证失败等）" } }
```

---

## 7. 前端

### 7.1 路由

```ts
{ path: '/admin/channels', component: AdminChannels, meta: { requiresAuth: true } },
```

### 7.2 Header 导航

在"Agent 管理"后追加"告警渠道"链接：

```vue
<RouterLink to="/admin/channels" class="...">告警渠道</RouterLink>
```

### 7.3 `/admin/channels` 渠道管理页

**布局：**
- **顶部**：标题"告警渠道" + "新增渠道"按钮
- **新建/编辑表单**（展开式，复用 AdminDomains 的行内编辑模式）：
  - name input
  - type select（webhook / dingtalk / feishu / wecom / email）
  - config textarea（JSON 输入，monospace 字体）
  - enabled checkbox（默认 true）
  - 保存 / 取消按钮
- **列表**：表格显示所有渠道
  - 列：名称 / 类型 / 启用 / 操作
  - 类型列用中文标签：webhook → "Webhook"、dingtalk → "钉钉"、feishu → "飞书"、wecom → "企业微信"、email → "邮件"
  - 启用列：绿色"已启用" / 灰色"已禁用"
  - 操作列：编辑 / 测试 / 删除
- **测试按钮**：点击后该按钮 loading（disabled + "发送中..."），成功 → 短暂绿色提示"发送成功"，失败 → 顶部红色 banner 显示错误
- **删除**：浏览器 `confirm()`

**风格：** 与 AdminDomains / AdminAgents 页面一致（Tailwind 配色、错误 banner、loading 状态、catch 风格）。

### 7.4 类型与 API 客户端

新增 `web/src/types.ts`：

```ts
export interface AlertChannel {
  id: number
  name: string
  type: 'webhook' | 'dingtalk' | 'feishu' | 'wecom' | 'email'
  config?: string    // 仅 GET 单个渠道时返回
  enabled: boolean
  created_at: string
}
```

新增 `web/src/api.ts`：

```ts
export const channelApi = {
  list: () =>
    request<{ channels: AlertChannel[] }>('/api/admin/alert-channels'),
  get: (id: number) =>
    request<AlertChannel>(`/api/admin/alert-channels/${id}`),
  create: (req: { name: string; type: string; config: string; enabled: boolean }) =>
    request<{ id: number }>('/api/admin/alert-channels', { method: 'POST', body: JSON.stringify(req) }),
  update: (id: number, req: { name: string; type: string; config: string; enabled: boolean }) =>
    request<{ ok: boolean }>(`/api/admin/alert-channels/${id}`, { method: 'PUT', body: JSON.stringify(req) }),
  delete: (id: number) =>
    request<{ ok: boolean }>(`/api/admin/alert-channels/${id}`, { method: 'DELETE' }),
  test: (id: number) =>
    request<{ ok: boolean }>(`/api/admin/alert-channels/${id}/test`, { method: 'POST' }),
}
```

---

## 8. 测试策略

### 8.1 Server 单元测试

**`server/internal/alert/channel_test.go`**（各渠道测试）：
- `TestValidateConfig_Webhook_Valid` / `_Invalid_MissingURL` / `_Invalid_JSON`
- `TestValidateConfig_Dingtalk_Valid` / `_Valid_NoSecret` / `_Invalid_MissingURL`
- `TestValidateConfig_Feishu_Valid` / `_Invalid_MissingURL`
- `TestValidateConfig_Wecom_Valid` / `_Invalid_MissingURL`
- `TestValidateConfig_Email_Valid` / `_Invalid_MissingHost` / `_Invalid_EmptyTo`
- `TestNewChannel_AllTypes` / `_InvalidType`
- `TestWebhook_Send` — httptest.NewServer 接收 POST，验证 JSON 格式
- `TestDingtalk_Send_NoSecret` / `_WithSecret` — httptest.NewServer 验证签名参数
- `TestFeishu_Send` — httptest.NewServer 验证消息格式
- `TestWecom_Send` — httptest.NewServer 验证消息格式
- `TestEmail_Send` — 可选，依赖 SMTP mock 或跳过（CI 环境无 SMTP 服务器）

**`server/internal/store/alert_channel_test.go`**（Store 层测试）：
- `TestCreateAlertChannel` — 创建成功
- `TestListAlertChannels` — 列出全部
- `TestGetAlertChannel` — 获取单个（含 config）
- `TestGetAlertChannel_NotFound` — 404
- `TestUpdateAlertChannel` — 更新成功
- `TestDeleteAlertChannel` — 删除成功

**`server/internal/api/alert_channel_test.go`**（API 测试）：
- `TestCreateChannel_Success` / `_InvalidType` / `_InvalidConfig_JSON` / `_InvalidConfig_MissingField`
- `TestListChannels_NoConfig` — 列表不返回 config
- `TestGetChannel_WithConfig` — 单个返回含 config
- `TestUpdateChannel_Success` / `_NotFound`
- `TestDeleteChannel_Success`
- `TestTestChannel_Success` / `_SendFailed` / `_NotFound`

### 8.2 前端测试

延续 Plan 3.1 / 3.2 的策略：不写前端单元测试，依赖手动 + curl 端到端验证。

### 8.3 端到端冒烟（最后任务）

启动 server + 浏览器登录后，逐项验证：

1. Header 出现"告警渠道"链接
2. `/admin/channels` 新建一个 webhook 渠道（url 指向 httpbin.org 或类似服务）
3. 点击"测试" → 收到成功提示（或验证 httpbin 收到请求）
4. 新建一个钉钉渠道 → 点击"测试" → 预期失败（无真实钉钉机器人）→ 错误信息显示
5. 编辑渠道 → 修改 config → 保存 → 验证列表更新
6. 删除渠道 → confirm → 验证列表移除
7. 列表页不显示 config 列（敏感信息保护）

---

## 9. 文件清单

### 9.1 Server 新增

| 文件 | 职责 |
|------|------|
| `server/internal/alert/channel.go` | Channel 接口 + Message + NewChannel 工厂 |
| `server/internal/alert/webhook.go` | Webhook 适配器 |
| `server/internal/alert/dingtalk.go` | 钉钉适配器 |
| `server/internal/alert/feishu.go` | 飞书适配器 |
| `server/internal/alert/wecom.go` | 企业微信适配器 |
| `server/internal/alert/email.go` | Email 适配器（SMTP） |
| `server/internal/alert/channel_test.go` | 渠道适配器单元测试 |
| `server/internal/api/alert_channel_handler.go` | CRUD + test handler |
| `server/internal/api/alert_channel_test.go` | API 测试 |
| `server/internal/store/alert_channel_test.go` | Store 测试 |

### 9.2 Server 修改

| 文件 | 修改内容 |
|------|---------|
| `server/internal/store/models.go` | 新增 AlertChannel struct |
| `server/internal/store/store.go` | 新增 CRUD 方法（CreateAlertChannel / ListAlertChannels / GetAlertChannel / UpdateAlertChannel / DeleteAlertChannel） |
| `server/internal/api/router.go` | 注册 6 条新路由 |
| `server/cmd/server/main.go` | AutoMigrate 加 `&store.AlertChannel{}` |

### 9.3 前端新增

| 文件 | 职责 |
|------|------|
| `web/src/views/AdminChannels.vue` | 渠道管理页 |

### 9.4 前端修改

| 文件 | 修改内容 |
|------|---------|
| `web/src/types.ts` | 新增 AlertChannel 接口 |
| `web/src/api.ts` | 新增 channelApi |
| `web/src/router.ts` | 新增 /admin/channels 路由 |
| `web/src/components/Header.vue` | 添加"告警渠道"导航链接 |

---

## 10. 关键设计决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 渠道数量 | 全部 5 种 | 设计文档已规划，一次到位 |
| 代码组织 | 统一接口 + 独立文件 | 职责清晰、易扩展、测试隔离 |
| 配置验证 | 后端强验证（ValidateConfig） | 防无效配置入库 |
| 测试流程 | 保存后测试（列表页行内按钮） | API 设计简单 |
| 前端输入 | textarea JSON | 单管理员技术背景，实现快 |
| 列表返回 config | 不返回 | 含敏感信息，编辑时调 Get 单个 |
| Email 实现 | net/smtp + crypto/tls | Go 标准库足够，无需第三方 SMTP 库 |

---

## 11. 后续计划（不属于本期）

- **Plan 3.3b — 告警引擎**：状态机（`alert_states` 表）+ 状态变化告警 + 每日提醒 + 在 `PostResults` 中触发
- **Plan 3.3c — 历史查询 + 清理**：历史查询 API + 前端页面 + 每日 02:00 清理任务
- 如需动态表单替代 textarea JSON、渠道按域名分组、发送结果入库，可在后续阶段追加

# SSL 证书监控系统 — 设计规格

- **日期**：2026-06-02
- **作者**：chenjiefeng（与 Claude 头脑风暴）
- **版本**：v1.0
- **状态**：草案，待评审

---

## 1. 背景与目标

构建一套自部署的 SSL 证书监控系统，用于持续检测一组域名的：

- 证书剩余有效期（即将过期 / 已过期）
- 证书与域名是否匹配
- 证书是否可达（连接失败、握手失败）

系统采用 **Server + Agent** 架构，支持从多个地理位置（10–20 台 Agent 机器）对同一域名进行并发检测，发现地域差异（如 CDN 路由错误、DNS 污染）。Server 提供配置管理、Dashboard 展示和多渠道告警；Agent 无对外端口，定时主动拉取检测任务并回调结果。

**非目标**：

- 不做 HTTP/Web 内容监控（不验证返回内容、状态码）
- 不做证书自动续签
- 不做 Agent 自动升级 / 集中下发配置（运维通过 Dockerfile / 二进制 + 配置文件部署）

---

## 2. 系统架构

```
┌───────────────────────────────────┐         ┌──────────────────┐
│  Server (单二进制)                 │         │  Agent #1 (北京)  │
│  ┌────────────────────────────┐   │  pull   │  Go binary       │
│  │  Vue 前端 (embed)            │   │◄────────┤  goroutine 检测  │
│  └────────────────────────────┘   │  push   │                  │
│  ┌────────────────────────────┐   │◄────────┤                  │
│  │  Gin API                  │   │         └──────────────────┘
│  │  ├─ Public  (看板)         │   │         ┌──────────────────┐
│  │  ├─ Auth    (配置)         │   │         │  Agent #2 (上海)  │
│  │  └─ Agent   (Token)        │   │◄────────┤                  │
│  └────────────────────────────┘   │         └──────────────────┘
│  ┌────────────────────────────┐   │         ┌──────────────────┐
│  │  Scheduler / Alert Engine │   │         │  Agent #N (...)  │
│  └────────────────────────────┘   │◄────────┤                  │
│  ┌────────────────────────────┐   │         └──────────────────┘
│  │  SQLite / MySQL           │   │
│  └────────────────────────────┘   │
└───────────────────────────────────┘
```

**职责切分**

| 组件 | 职责 |
|------|------|
| Server 后端 | API、Agent 注册与心跳、域名调度计算（global + 额外 − 排除）、检测结果持久化、告警引擎、历史清理 |
| Agent | 拉取自己应检测的域名 → 并发 TLS 检测 → 批量回调结果，无监听端口 |
| Web 前端 | Dashboard（免登录）、配置中心（登录） |

**部署形态**

- **Server**：Go 后端通过 `embed` 把 Vue 构建产物打进单二进制；提供 Dockerfile 与裸二进制两种发行方式
- **Agent**：单二进制；提供 Dockerfile 与裸二进制两种发行方式

---

## 3. 数据模型

> 使用 GORM 抽象 SQLite / MySQL；以下用 SQL DDL 风格描述。MySQL 与 SQLite 的差异由 ORM 处理（自增主键、TIMESTAMP 精度等）。

### 3.1 `agents`

```
agents:
  agent_id        TEXT PK         -- sha256(hostname + ip + unix_nano)[:16]，首次启动生成并落盘
  display_name    TEXT NOT NULL   -- 配置文件中的"地域+机器名"，可变更
  hostname        TEXT
  ip              TEXT
  remark          TEXT            -- Server 端管理员备注，与 display_name 独立
  registered_at   TIMESTAMP
  last_seen_at    TIMESTAMP       -- 拉取域名列表时更新（充当心跳）
```

### 3.2 `domains`

```
domains:
  id              INTEGER PK
  host            TEXT NOT NULL    -- 例：example.com
  port            INTEGER NOT NULL -- 默认根据协议补 443
  protocol        TEXT NOT NULL    -- https | wss
  is_global       BOOLEAN NOT NULL -- true = 全局检测，false = 仅指定 agent 检测
  remark          TEXT
  created_at      TIMESTAMP
  UNIQUE (host, port, protocol)
```

### 3.3 `agent_domain_overrides`

Agent 维度的差异化覆盖。

```
agent_domain_overrides:
  agent_id        TEXT NOT NULL
  domain_id       INTEGER NOT NULL
  action          TEXT NOT NULL    -- include (额外加入) | exclude (从全局剔除)
  PRIMARY KEY (agent_id, domain_id)
```

调度计算规则（在 Server 内存中执行）：

```
agent_domains(agent_id) =
  ( {d | d.is_global = true} ∪ {d | (agent_id, d, include) ∈ overrides} )
  \ {d | (agent_id, d, exclude) ∈ overrides}
```

> 注：`exclude` 仅对全局域名有意义；对非全局且未 include 的域名设置 exclude 等效于无操作。

### 3.4 `check_results`

```
check_results:
  id              INTEGER PK
  agent_id        TEXT NOT NULL
  domain_id       INTEGER NOT NULL
  checked_at      TIMESTAMP NOT NULL
  status          TEXT NOT NULL    -- ok | expiring | expired | mismatch | unreachable
  not_after       TIMESTAMP        -- 证书过期时间，unreachable 时为 NULL
  issuer          TEXT
  subject         TEXT
  sans            TEXT             -- JSON 数组，存所有 SAN
  error_message   TEXT             -- 失败原因 / 链不受信详情
  INDEX (domain_id, agent_id, checked_at DESC)
  INDEX (checked_at)               -- 历史清理用
```

### 3.5 `alert_states`

告警状态机，每个 (域名, agent) 维度一行。

```
alert_states:
  domain_id              INTEGER NOT NULL
  agent_id               TEXT NOT NULL
  current_status         TEXT NOT NULL
  status_since           TIMESTAMP NOT NULL
  last_alert_at          TIMESTAMP        -- 最近一次"状态变化告警"
  last_daily_reminder    TIMESTAMP        -- 最近一次"每日提醒"
  PRIMARY KEY (domain_id, agent_id)
```

### 3.6 `alert_channels`

```
alert_channels:
  id          INTEGER PK
  name        TEXT NOT NULL
  type        TEXT NOT NULL     -- email | webhook | dingtalk | feishu | wecom
  config      TEXT NOT NULL     -- JSON，渠道特定字段（webhook URL、SMTP host 等）
  enabled     BOOLEAN NOT NULL DEFAULT true
  created_at  TIMESTAMP
```

### 3.7 `users`

```
users:
  id              INTEGER PK
  username        TEXT UNIQUE NOT NULL
  password_hash   TEXT NOT NULL    -- bcrypt
  created_at      TIMESTAMP
```

启动时若 `users` 表为空，使用 `auth.admin_username` / `auth.admin_password` 创建初始管理员。

### 3.8 历史清理

每天 02:00（本地时区）执行，删除 `check_results` 中 `checked_at < now() - retention.history_days` 的记录。

---

## 4. API 契约

> 所有 JSON。错误响应统一：`{ "error": { "code": "string", "message": "string" } }`，HTTP 状态码语义化。

### 4.1 Agent 接口

认证：`Authorization: Bearer <agent_token>`，Token 在 Server `config.yaml` 中配置，所有 Agent 共用。

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/agent/register` | 首次注册或 `display_name` / `hostname` / `ip` 变更时调用 |
| GET  | `/api/agent/domains`  | 拉取该 Agent 应检测的域名列表，同时更新 `last_seen_at`（心跳） |
| POST | `/api/agent/results`  | 批量上报检测结果 |

**`POST /api/agent/register`**

请求：
```json
{
  "agent_id": "a1b2c3d4e5f60718",
  "display_name": "北京-prod-01",
  "hostname": "host-bj-01",
  "ip": "10.0.0.5"
}
```
响应：`200 { "ok": true }`

**`GET /api/agent/domains?agent_id=...`**

响应：
```json
{
  "domains": [
    { "id": 1, "host": "example.com", "port": 443, "protocol": "https" },
    { "id": 7, "host": "ws.example.com", "port": 443, "protocol": "wss" }
  ]
}
```

**`POST /api/agent/results`**

请求：
```json
{
  "agent_id": "a1b2c3d4e5f60718",
  "results": [
    {
      "domain_id": 1,
      "checked_at": "2026-06-02T10:00:00Z",
      "status": "ok",
      "not_after": "2026-06-14T23:59:00Z",
      "issuer": "Let's Encrypt R3",
      "subject": "CN=example.com",
      "sans": ["example.com", "www.example.com"],
      "error_message": ""
    },
    {
      "domain_id": 7,
      "checked_at": "2026-06-02T10:00:01Z",
      "status": "unreachable",
      "error_message": "dial tcp: i/o timeout"
    }
  ]
}
```
响应：`200 { "accepted": 2 }`

### 4.2 公开接口（看板，无需登录）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/dashboard/overview`     | 总览卡数据（域名总数 / 健康 / 异常 / Agent 在线） |
| GET | `/api/dashboard/domains`      | 域名聚合列表，每域名展示 X/Y 健康 |
| GET | `/api/dashboard/domains/:id`  | 域名详情：各 Agent 最新结果对比 |

### 4.3 管理接口（需登录，Cookie/Session 或 JWT）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/login`        | 登录 |
| POST | `/api/auth/logout`       | 登出 |
| GET  | `/api/auth/me`           | 当前用户 |
| GET/POST/PUT/DELETE | `/api/admin/domains[/:id]` | 全局域名 CRUD |
| GET  | `/api/admin/agents`      | Agent 列表（含 last_seen_at、在线状态） |
| PUT  | `/api/admin/agents/:id`  | 更新 Agent 备注（display_name 由 Agent 配置文件主导，此处仅备注） |
| GET/POST/DELETE | `/api/admin/agents/:id/overrides` | Agent 域名覆盖（额外/排除） |
| GET/POST/PUT/DELETE | `/api/admin/alert-channels[/:id]` | 告警渠道 CRUD |
| POST | `/api/admin/alert-channels/:id/test` | 测试发送 |
| GET  | `/api/admin/history`     | 历史查询，支持 `?domain_id=&agent_id=&from=&to=&page=&size=` |

### 4.4 在线状态判定

Agent 在线 = `now - last_seen_at < 3 * agent.check.interval`（按 Agent 注册时上报的 interval；若未上报则用 Server 默认值 1h）。

> 简化处理：Server 不存储每个 Agent 的 interval；统一用 `3 * 1h = 3h` 作为离线阈值。需要时再扩展。

---

## 5. Agent 工作流

```
启动
  ├─ 读取 config.yaml (server_url, token, display_name, interval, timeout, concurrency)
  ├─ 加载或生成 agent_id
  │   ├─ id_file 不存在：sha256(hostname + ip + unix_nano)[:16] → 写入 id_file
  │   └─ id_file 存在：从文件读取
  ├─ 与 Server 注册（register API），上报当前 hostname/ip/display_name
  └─ 启动 ticker（默认 1h）

每个 tick：
  ├─ GET /api/agent/domains → 域名列表
  ├─ errgroup + semaphore (concurrency 限流)
  │   每域名一个 goroutine：
  │   ├─ ctx, cancel := context.WithTimeout(parentCtx, timeout)
  │   ├─ result := checkDomain(ctx, domain)  // 见第 6 节
  │   └─ 写入 results channel
  ├─ 收集所有结果（等所有 goroutine 完成）
  └─ POST /api/agent/results（批量，单次请求）
```

**容错与边界**：

- 拉取域名失败：本轮跳过，等下一个 tick；不本地缓存上一轮的列表。
- 回调失败：本轮结果丢弃；不重试不堆积（避免雪崩）。下一轮重新检测重新上报。
- 单个域名检测异常（panic）：用 `recover` 兜底，结果记为 `unreachable` + error_message。
- `display_name` 变更检测：每次启动时读取配置；如与 `agent_id` 文件中记录的上次值不同，调一次 register；运行期间不监听配置变更。

**Agent 配置文件**

```yaml
server_url: "https://ssl-tracker.example.com"
auth_token: "your-shared-token-here"

agent:
  display_name: "北京-prod-01"
  id_file: "./agent_id"

check:
  interval: "1h"
  timeout: "10s"
  concurrency: 50
```

---

## 6. SSL 检测逻辑

**实现思路**：使用 `crypto/tls` 直接发起 TLS 握手获取证书，自行做判定，避免标准 `tls.Config` 验证失败时拿不到证书信息。

```go
dialer := &net.Dialer{Timeout: timeout}
conn, err := tls.DialWithDialer(dialer, "tcp", host+":"+port, &tls.Config{
    InsecureSkipVerify: true,  // 自己验证，便于拿到链信息
    ServerName:         host,
})
if err != nil {
    return result{Status: "unreachable", ErrorMessage: err.Error()}
}
defer conn.Close()

certs := conn.ConnectionState().PeerCertificates
leaf := certs[0]
```

**判定顺序**（任何一项命中即返回该状态，但 issuer/subject/not_after 字段始终填充以便前端展示）：

1. **域名匹配**：`leaf.VerifyHostname(host)` 返回错误 → `mismatch`（通配符自动处理）
2. **过期判断**（Agent 端只判断 `expired`，不判断 `expiring`）：
   - `now > leaf.NotAfter` → `expired`
3. **链信任**（可选告警维度，记录到 `error_message`，状态不变）：
   - 用 `x509.NewCertPool()` 构建 Intermediates
   - 用 `x509.SystemCertPool()` 作为 Roots 调 `leaf.Verify(...)`
   - 失败时附加描述到 `error_message`
4. 否则 → `ok`（即使 `not_after` 已临近阈值，Agent 仍上报 ok）

**Agent 与 Server 的状态语义切分**：

- **Agent 上报的 status**：`ok | expired | mismatch | unreachable`，**不含 `expiring`**。Agent 仅做"是否能连、是否匹配、是否已过期"的客观判定，不掌握"即将过期"的阈值。
- **Server 写入 check_results 前的最终 status**：在 Agent 上报基础上，若 `agent.status == ok` 且 `not_after - now < expire_threshold_days`，改写为 `expiring`；其余原样保留。
- 收益：管理员调整 `expire_threshold_days` 后，下一个 tick 即按新阈值重新分类，无需 Agent 改配置或重算历史。

**协议处理**：

- `https` 与 `wss` 在 TLS 检测层面无差异（都是 TCP + TLS 握手）。Agent 不发起任何 HTTP / WebSocket Upgrade 请求。
- 协议字段保留用于前端区分和未来扩展（如 wss 检测 Sec-WebSocket-Protocol 时的差异）。

**端口**：

- 域名记录中的 `port` 字段在 Server 端创建时若未提供，根据 `protocol` 自动填 443。
- Agent 不再处理默认端口逻辑。

---

## 7. 告警引擎

### 7.1 状态变化告警

每次 `/api/agent/results` 写入 `check_results` 后，对每条结果执行：

```
prev = alert_states[(domain_id, agent_id)]   // 可能不存在
curr_status = result.status

if prev == nil:
    insert alert_states (curr_status, status_since=now, last_alert_at=NULL)
    若 curr_status != ok → 立即发送"告警"
elif prev.current_status != curr_status:
    更新 alert_states (current_status=curr_status, status_since=now, last_alert_at=now)
    if prev.current_status == ok and curr_status != ok:
        发送"告警"
    elif prev.current_status != ok and curr_status == ok:
        发送"恢复"
    else:
        发送"状态变化"  (异常 → 异常，如 expiring → expired)
else:
    不告警（状态未变）
```

### 7.2 每日提醒

定时任务每天 `alert.daily_reminder_time`（默认 09:00，时区 `alert.daily_reminder_timezone`）扫描：

```
for each row in alert_states where current_status != ok:
    if last_daily_reminder is NULL or date(last_daily_reminder) < today:
        发送"持续异常提醒"
        update last_daily_reminder = now
```

### 7.3 渠道与发送

- 启用的所有渠道并发发送（每渠道一个 goroutine）。
- 单渠道发送失败仅记日志，不影响其他渠道，不重试（避免循环放大）。
- 发送结果不入库（避免再起一张表）；运维通过日志排查。
- 测试发送接口（`POST /api/admin/alert-channels/:id/test`）发送固定模板的测试消息。

### 7.4 消息模板（统一文本，各渠道适配富文本）

```
[SSL Alert] {host}:{port} ({protocol}) - Agent: {display_name}
状态: {状态中文}（{补充信息：剩余 N 天 / 错误原因 / "已恢复"}）
过期时间: {not_after 或 "—"}
颁发者: {issuer 或 "—"}
检测时间: {checked_at}
```

**渠道差异**：

- **email**：HTML 表格 + 上述字段
- **webhook**：JSON `{type, host, port, protocol, agent_display_name, status, not_after, issuer, error_message, checked_at, alert_kind}`，`alert_kind ∈ {alert, recovery, status_change, daily_reminder}`
- **dingtalk / feishu / wecom**：Markdown 文本，沿用统一模板

---

## 8. 前端设计

### 8.1 风格定位（苹果风）

- **字体**：`-apple-system, BlinkMacSystemFont, "SF Pro Display", "PingFang SC", sans-serif`
- **字号层级**：H1 32px / 600，H2 24px / 500，正文 14–15px / 400，辅助 12–13px / 400 灰
- **配色**：
  - 主背景 `#FFFFFF`，次背景 `#F5F5F7`
  - 文本主 `#1D1D1F`，文本辅 `#86868B`
  - 强调 `#0071E3`（克制使用）
  - 状态色：成功 `#34C759`、警告 `#FF9F0A`、危险 `#FF3B30`，仅用于小圆点 / 细边
- **空间**：卡片间距 24–32px，内边距 24px，弱边框（1px `#D2D2D7` 或无边框靠背景区分）
- **数据展示**：大数字 + 细字重 + 小灰色标签，避免密集表格；列表行内距 ≥ 56px
- **动效**：仅在状态切换时使用 200–300ms ease-out 过渡

### 8.2 页面结构

| 路由 | 登录 | 内容 |
|------|------|------|
| `/`                       | 免 | 顶部 4 张概览卡（域名总数 / 健康 / 异常 / Agent 在线）+ 域名聚合列表 |
| `/domains/:id`            | 免 | 各 Agent 检测结果对比表 + 时间轴 |
| `/login`                  | -  | 简单登录页 |
| `/admin/domains`          | 需 | 全局域名 CRUD |
| `/admin/agents`           | 需 | Agent 列表 + 单 Agent 的额外/排除域名配置 |
| `/admin/channels`         | 需 | 告警渠道 CRUD + 测试 |
| `/admin/history`          | 需 | 按域名 / Agent / 时间范围查询 |

> 域名聚合列表：一行一个域名，右侧显示 `X/Y 健康` 或 `Y - X 异常`，点击跳转详情。
>
> 域名详情：以 Agent 为列、检测项（状态、过期时间、颁发者、SAN、错误）为行的对比矩阵。

### 8.3 技术栈

- Vue 3 + Vite + TypeScript
- Tailwind CSS（主题色用 CSS 变量，便于全局微调）
- shadcn-vue（组件源码本地化，便于调整视觉细节）
- Pinia（状态管理）
- Vue Router
- 图标：Lucide

> 实现阶段调用 `frontend-design` 与 `ui-ux-pro-max` 技能细化首页与详情页的视觉。

---

## 9. Server 配置文件

```yaml
server:
  listen: ":8080"

auth:
  agent_token: "your-shared-token-here"
  admin_username: "admin"
  admin_password: "change-me"            # 启动时若 users 表为空，hash 后写入

database:
  type: sqlite                            # sqlite | mysql
  sqlite:
    path: "./data/ssl-tracker.db"
  mysql:
    dsn: "user:pass@tcp(host:3306)/ssl_tracker?parseTime=true&charset=utf8mb4"

retention:
  history_days: 7

alert:
  expire_threshold_days: 15
  daily_reminder_time: "09:00"
  daily_reminder_timezone: "Asia/Shanghai"

session:
  secret: "random-secret-change-me"      # 用于签发管理端 JWT/Session
  ttl: "24h"
```

---

## 10. 项目结构（monorepo）

```
SSLCertTracker/
├── server/
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── api/           # Gin handler
│   │   │   ├── agent.go
│   │   │   ├── dashboard.go
│   │   │   ├── admin.go
│   │   │   └── auth.go
│   │   ├── auth/          # token 校验、session
│   │   ├── store/         # GORM models + repository
│   │   ├── scheduler/     # 调度计算（global+include-exclude）
│   │   ├── alert/         # 渠道适配器 + 状态机
│   │   ├── retention/     # 历史清理任务
│   │   └── config/
│   ├── web/               # //go:embed 指向构建产物
│   │   └── dist/          # 来自 web/ 的 build 输出
│   └── config.yaml.example
├── agent/
│   ├── cmd/agent/main.go
│   ├── internal/
│   │   ├── checker/       # TLS 检测核心
│   │   ├── client/        # 调用 server API
│   │   ├── runner/        # ticker + 并发控制
│   │   └── config/
│   └── config.yaml.example
├── web/                   # Vue 3 源码
│   ├── src/
│   ├── public/
│   ├── index.html
│   ├── package.json
│   ├── vite.config.ts
│   └── tailwind.config.ts
├── docs/
│   └── superpowers/specs/
├── docker/
│   ├── server.Dockerfile
│   └── agent.Dockerfile
├── Makefile               # build-web / build-server / build-agent / build-all
├── go.work                # 多 module 工作区（server + agent）
└── README.md
```

**构建链**：

1. `make build-web` → 在 `web/` 内 `pnpm build`，产物到 `server/web/dist/`
2. `make build-server` → `go build` 把 `server/web/dist/` embed 进二进制
3. `make build-agent` → `go build` agent
4. Docker 镜像基于 `scratch` 或 `alpine`

---

## 11. 安全考虑

- **Agent Token**：仅在 Server 配置文件，绝不下发到前端 JS。前端无任何入口管理 Agent Token。Token 泄露 = 任意机器可注册为 Agent，需运维妥善保管。
- **管理员密码**：启动时一次性从配置文件 hash 后写入 DB；后续通过前端修改密码。
- **Session**：HttpOnly + SameSite=Lax Cookie；登出清除 server 端 session（如使用 JWT 则用短 TTL + 黑名单或简单短期 token）。
- **密码哈希**：bcrypt（cost ≥ 10）。
- **HTTPS**：Server 部署时由前置代理（Nginx / Caddy）做 TLS 终止；二进制本身只监听 HTTP。
- **TLS 检测目标**：连接到外部域名仅做握手，不发送应用数据；不可作为攻击代理。
- **防 SSRF**：Agent 只检测 Server 下发的域名列表；运维须确保前端配置只录入合法资产。

---

## 12. 性能容量预估

- 域名 5000 × Agent 20 × 1 次/小时 = 100k 次检测/小时 = 28 次/秒。Agent 单机一次 tick 最多 5000 域名 × 1 次，按 concurrency=50、每检测 1s 计 ≈ 100s 完成，远低于 1h 间隔。
- 检测结果写入：单次 tick Agent 上报 ≤ 5000 行 → 总 ≤ 100k 行/小时 → 2.4M 行/天 → 7 天 ≈ 17M 行。SQLite 单表 1700 万行可承载但查询性能下滑明显，需要 MySQL。
- **结论**：默认 SQLite，文档明示 **>5000 域名或 >10 Agent 高频写入推荐 MySQL**。

---

## 13. 验收准则

- [ ] Server 单二进制可启动，前端可访问 `/`
- [ ] 配置 `agent_token` 后，Agent 可注册并周期性拉取域名
- [ ] 在前端配置全局域名后，所有 Agent 在下一个 tick 看到该域名
- [ ] 给 A Agent 设置 include 一个非全局域名 → 仅 A Agent 检测
- [ ] 给 B Agent 设置 exclude 一个全局域名 → B Agent 不检测、其他 Agent 仍检测
- [ ] 域名详情页可看到不同 Agent 的检测结果差异
- [ ] 制造一个即将过期场景（手动改阈值或证书）→ 收到 5 种告警渠道之一
- [ ] 状态从 expiring 恢复为 ok → 收到恢复通知
- [ ] 异常状态持续 → 次日 09:00 收到每日提醒
- [ ] Agent 离线超过 3h → Dashboard 显示离线
- [ ] SQLite / MySQL 切换通过配置文件即可生效
- [ ] 历史记录超过 7 天自动清理

---

## 14. 待后续展开

以下不在 v1 范围内，后续单独头脑风暴：

- 多用户 + 角色权限
- 证书内容签名变化检测（防中间人 / 替换）
- Prometheus metrics 暴露
- Agent 自升级
- 域名分组 / 标签
- 告警渠道按域名分组

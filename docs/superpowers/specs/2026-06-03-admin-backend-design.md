# Plan 3.2：管理后台 — 设计规格

- **日期**：2026-06-03
- **作者**：chenjiefeng（与 Claude 头脑风暴）
- **版本**：v1.0
- **状态**：草案，待评审
- **关联**：
  - 拆分自 [SSL 证书监控系统 — 设计规格](2026-06-02-ssl-cert-tracker-design.md) 第 4.3 节
  - 依赖 [Plan 3.1 登录认证系统](2026-06-03-auth-system-design.md)

---

## 1. 背景与目标

在 Plan 3.1 已建好的登录认证基础上，构建管理员可视化操作的**域名管理**和 **Agent 管理**界面，让管理员脱离 curl，通过浏览器完成日常运维。

**本期范围（Plan 3.2）：**

- 后端补：`PUT /api/admin/domains/:id`、`GET /api/admin/agents`、`PUT /api/admin/agents/:id`、`GET/POST/DELETE /api/admin/agents/:id/overrides`
- 前端：`/admin/domains` 域名管理页（增删改查）、`/admin/agents` Agent 列表页（含备注编辑）、`/admin/agents/:id` 单 Agent 的 override 矩阵
- Header 顶级导航：已登录显示 `域名管理` / `Agent 管理` 链接
- Agent 在线状态阈值常量化（3 小时），dashboard 与 admin 共用

**非目标（推迟到 Plan 3.3 或后续）：**

- 告警渠道 CRUD + 测试发送
- 历史查询页
- 告警引擎本身
- Agent 删除（本期不实现，未来如需可加 DELETE）
- 域名 host/port/protocol 编辑（删除 + 重建即可）
- 用户管理 CRUD（修改密码、增删用户）
- CSRF Token

---

## 2. 架构

```
┌────────────────────────────────────────────────────────────┐
│  Browser (已登录管理员)                                       │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Header: [SSL Tracker]  域名管理 Agent管理  [admin▼] │  │
│  └──────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  /admin/domains      域名 CRUD（行内编辑、新建表单）   │  │
│  │  /admin/agents       Agent 列表（行内编辑 remark）    │  │
│  │  /admin/agents/:id   该 Agent 的 override 矩阵        │  │
│  └──────────────────────────────────────────────────────┘  │
│              ↓ Cookie: session_id (Plan 3.1)               │
└────────────────────────────────────────────────────────────┘
                              ↓
┌────────────────────────────────────────────────────────────┐
│  Server (受 AuthMiddleware 保护)                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  AdminHandler                                         │  │
│  │  ├ Domains: List / Get / Create / Update / Delete    │  │
│  │  ├ Agents:  List / Update                            │  │
│  │  └ Overrides: List / Set / Delete (per-agent)        │  │
│  └──────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Store (复用现有 + 新增 UpdateDomain/Agent + override) │  │
│  └──────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────┘
```

**职责切分：**

| 组件 | 职责 |
|------|------|
| `internal/store/store.go` | 新增 `UpdateDomainMeta` / `UpdateAgentRemark` / `ListOverrides` / `UpsertOverride`；导出常量 `AgentOnlineWindow` |
| `internal/api/admin.go` | 新增 6 个 handler 方法 |
| `internal/api/router.go` | 注册 6 条新路由 |
| `web/src/views/AdminDomains.vue` | 域名 CRUD 页 |
| `web/src/views/AdminAgents.vue` | Agent 列表 + 备注编辑 |
| `web/src/views/AdminAgentOverrides.vue` | 单 Agent 域名 override 矩阵 |
| `web/src/components/Header.vue` | 已登录时增加管理导航链接 |
| `web/src/api.ts` | 新增 `adminApi` |

---

## 3. 数据模型 / Store 改动

### 3.1 复用现有结构（无 schema 变更）

`Domain`、`Agent`、`AgentDomainOverride` 在 Plan 1 已定义，无需迁移。

### 3.2 Store 层新增方法

```go
// 仅更新 is_global 和 remark；host/port/protocol 不可改
func (s *Store) UpdateDomainMeta(id uint, isGlobal bool, remark string) error

// 更新 Agent 备注（display_name 由 Agent 配置文件主导）
func (s *Store) UpdateAgentRemark(agentID, remark string) error

// 列出某 Agent 的所有 overrides（include + exclude 都返回）
func (s *Store) ListOverrides(agentID string) ([]AgentDomainOverride, error)

// Upsert：如果 (agent_id, domain_id) 已存在则更新 action，否则插入
func (s *Store) UpsertOverride(agentID string, domainID uint, action string) error

// DeleteOverride 复用 Plan 1 已有方法，签名不变
func (s *Store) DeleteOverride(agentID string, domainID uint) error
```

`UpdateDomainMeta` 内部使用 GORM `Model(&Domain{}).Where("id = ?", id).Updates(map[string]interface{}{"is_global": ..., "remark": ...})`，确保只更新这两列。如果影响行数为 0，返回 `gorm.ErrRecordNotFound`。

`UpdateAgentRemark` 同上，按 `agent_id` 定位。

### 3.3 公共常量

```go
// store/store.go
const AgentOnlineWindow = 3 * time.Hour
```

`internal/api/dashboard.go` 中 `NewDashboardHandler(s, 3*time.Hour)` 改为 `NewDashboardHandler(s, store.AgentOnlineWindow)`，`cmd/server/main.go` 同步替换。

`admin.go` 计算 `is_online` 时直接 `time.Since(agent.LastSeenAt) < store.AgentOnlineWindow`。

---

## 4. API 契约

> 错误响应统一格式：`{ "error": { "code": "string", "message": "string" } }`。所有 admin 接口已被 Plan 3.1 的 `AuthMiddleware` 保护——未登录访问返回 `401 unauthenticated`。

### 4.1 `PUT /api/admin/domains/:id`

**请求：**
```json
{ "is_global": true, "remark": "marketing site" }
```

**字段约束：**
- 仅 `is_global` 和 `remark` 可改
- 服务端忽略请求体中的其他字段（host / port / protocol 即使存在也不影响）
- `is_global` 必填（bool 没有"未提供"语义，强制要求）
- `remark` 必填，可为空字符串

**实现注意：** Go 的 `binding:"required"` 对 bool 类型不可靠（`false` 与"未提供"无法区分）。实现时使用 `*bool` 指针字段并显式检查 `nil`，或自行做 JSON raw 检查。计划阶段会给出具体写法。

**成功响应：** `200`
```json
{ "ok": true }
```

**错误响应：**
- `404 not_found` — 域名不存在
- `400 invalid_request` — 字段缺失或类型错误

### 4.2 `GET /api/admin/agents`

**响应：** `200`
```json
{
  "agents": [
    {
      "agent_id": "a1b2c3d4e5f60718",
      "display_name": "Beijing-prod-01",
      "hostname": "host-bj-01",
      "ip": "10.0.0.5",
      "remark": "marketing 业务专用",
      "registered_at": "2026-05-10T08:00:00Z",
      "last_seen_at": "2026-06-03T11:55:00Z",
      "is_online": true
    }
  ]
}
```

`is_online` 由 server 计算：`time.Since(LastSeenAt) < store.AgentOnlineWindow`。

### 4.3 `PUT /api/admin/agents/:id`

**请求：**
```json
{ "remark": "迁移中，预计 2026-07-01 退役" }
```

**字段约束：**
- 只能改 `remark`（display_name 由 Agent 配置文件主导）
- `remark` 必填，可为空字符串（清除备注）

**成功响应：** `200 { "ok": true }`

**错误响应：**
- `404 not_found` — Agent 不存在
- `400 invalid_request` — 字段缺失

### 4.4 `GET /api/admin/agents/:id/overrides`

**响应：** `200`
```json
{
  "overrides": [
    { "domain_id": 5, "action": "include" },
    { "domain_id": 9, "action": "exclude" }
  ]
}
```

返回该 Agent 当前所有 override 记录。前端结合 `GET /api/admin/domains` 的全量域名列表，渲染三态矩阵。

### 4.5 `POST /api/admin/agents/:id/overrides`

**请求：**
```json
{ "domain_id": 5, "action": "include" }
```

**语义：** Upsert——如果 `(agent_id, domain_id)` 已存在则更新 action；否则插入。

**字段约束：**
- `action` 必须是 `"include"` 或 `"exclude"`（其他值 400）
- `domain_id` 必须存在（不存在 404）
- 路径中的 `:id`（agent_id）必须存在（不存在 404）

**成功响应：** `200 { "ok": true }`

### 4.6 `DELETE /api/admin/agents/:id/overrides/:domain_id`

删除该 (agent_id, domain_id) 的 override，让该域名回到默认状态（global → 监控；非 global → 不监控）。

**成功响应：** `200 { "ok": true }`

**幂等：** 删除不存在的 override 也返回 200（与 logout 一致），方便前端无需先查再删。

---

## 5. 前端

### 5.1 路由（router.ts 新增）

```ts
{ path: '/admin/domains', component: AdminDomains, meta: { requiresAuth: true } },
{ path: '/admin/agents', component: AdminAgents, meta: { requiresAuth: true } },
{ path: '/admin/agents/:id', component: AdminAgentOverrides, props: true, meta: { requiresAuth: true } },
```

`meta.requiresAuth: true` 触发 Plan 3.1 已建好的 `beforeEach` 守卫——未登录跳 `/login?redirect=/admin/...`，登录后跳回原路径。

### 5.2 Header 调整

已登录时在 logo 和用户菜单之间显示导航链接，未登录时不显示。

```vue
<RouterLink to="/" class="logo">SSL Tracker</RouterLink>

<nav v-if="!loading && user" class="flex gap-1 ml-4">
  <RouterLink to="/admin/domains" class="...">域名管理</RouterLink>
  <RouterLink to="/admin/agents" class="...">Agent 管理</RouterLink>
</nav>

<div class="flex-1" />

<!-- 现有的登录按钮 / 用户下拉（Plan 3.1 实现） -->
```

激活路由用 RouterLink 默认的 `router-link-active` class 高亮（Tailwind class 切换：active → text-ink，inactive → text-ink-soft hover:text-ink）。

### 5.3 `/admin/domains` 域名管理页

**布局：**

- **顶部**：标题"域名管理" + "新增域名"按钮
- **新建表单**：点击"新增域名"展开表单（host input / port number input / protocol select 含 `https` `wss` / is_global checkbox / remark input / 保存 / 取消）
- **列表**：表格显示所有域名
  - 列：host / port / protocol / 全局 / 备注 / 操作
  - 每行操作：`编辑` / `删除`
  - 编辑展开：行内显示 is_global checkbox + remark input + 保存 / 取消按钮（host/port/protocol 显示为只读文本，明确不可改）
  - 删除：浏览器 `confirm()` 提示 `确定删除域名 example.com:443/https 吗？`，确认后调 DELETE

**错误处理：** API 失败显示顶部红色 banner，3 秒后自动消失或手动 ×。

**数据加载：** `onMounted` 调 `adminApi.listDomains()`；新建/编辑/删除成功后重新拉取。

### 5.4 `/admin/agents` Agent 列表页

**布局：**

- **顶部**：标题"Agent 管理"
- **列表**：表格显示所有 Agent
  - 列：display_name / hostname / ip / 备注 / 状态 / 最后心跳 / 操作
  - 状态：在线 → 绿色"在线"；离线 → 灰色"离线 X 天前"（用 `time.Since` 格式化，比如 "离线 2 天前"、"离线 5 小时前"）
  - 备注列：行内编辑（点击"编辑"展开 input + 保存 / 取消）
  - 操作列：`编辑` / `管理监控`（后者跳到 `/admin/agents/:id`）
- 不提供"删除"按钮（本期不实现 Agent 删除）

### 5.5 `/admin/agents/:id` Override 矩阵页

**布局：**

- **顶部**：Agent 信息卡（display_name / hostname / ip / 在线状态 / 备注）
- **域名列表**：列出**所有域名**（global 和非 global 都列）
  - 列：host:port (protocol) / 默认状态 / 当前状态 / 切换按钮
  - 默认状态：global → "默认监控"；非 global → "默认不监控"
  - 当前状态（结合 override）：
    - global + 无 override → `[监控]`
    - global + exclude → `[已排除]`（红色）
    - 非 global + 无 override → `[不监控]`
    - 非 global + include → `[已加入]`（绿色）
  - 切换按钮：
    - `[监控]` → 点击 "排除"：`POST {action: "exclude"}`
    - `[已排除]` → 点击 "恢复"：`DELETE`
    - `[不监控]` → 点击 "加入"：`POST {action: "include"}`
    - `[已加入]` → 点击 "移除"：`DELETE`
  - 每行独立 loading 状态：点击后该按钮短暂 disabled + 转圈，请求完成后刷新该行；失败显示该行错误并回滚
- **数据加载：** `Promise.all([adminApi.listDomains(), adminApi.listOverrides(agentId)])` 合并展示

**离线 Agent：** 仍可编辑（override 是配置数据，与在线状态无关）。仅在顶部 Agent 信息卡用灰色 + 离线提示。

### 5.6 类型与 API 客户端

新增 `web/src/types.ts`：

```ts
export interface DomainAdmin {
  id: number
  host: string
  port: number
  protocol: string
  is_global: boolean
  remark: string
  created_at: string
}

export interface AgentAdmin {
  agent_id: string
  display_name: string
  hostname: string
  ip: string
  remark: string
  registered_at: string
  last_seen_at: string
  is_online: boolean
}

export interface Override {
  domain_id: number
  action: 'include' | 'exclude'
}
```

新增 `web/src/api.ts`（沿用 Plan 3.1 已建的 `request<T>` 函数）：

```ts
export const adminApi = {
  listDomains: () =>
    request<{ domains: DomainAdmin[] }>('/api/admin/domains'),
  createDomain: (req: { host: string; port: number; protocol: string; is_global: boolean; remark: string }) =>
    request<{ id: number }>('/api/admin/domains', { method: 'POST', body: JSON.stringify(req) }),
  updateDomain: (id: number, req: { is_global: boolean; remark: string }) =>
    request<{ ok: boolean }>(`/api/admin/domains/${id}`, { method: 'PUT', body: JSON.stringify(req) }),
  deleteDomain: (id: number) =>
    request<{ ok: boolean }>(`/api/admin/domains/${id}`, { method: 'DELETE' }),

  listAgents: () =>
    request<{ agents: AgentAdmin[] }>('/api/admin/agents'),
  updateAgentRemark: (id: string, remark: string) =>
    request<{ ok: boolean }>(`/api/admin/agents/${id}`, { method: 'PUT', body: JSON.stringify({ remark }) }),

  listOverrides: (agentId: string) =>
    request<{ overrides: Override[] }>(`/api/admin/agents/${agentId}/overrides`),
  setOverride: (agentId: string, domainId: number, action: 'include' | 'exclude') =>
    request<{ ok: boolean }>(`/api/admin/agents/${agentId}/overrides`, {
      method: 'POST',
      body: JSON.stringify({ domain_id: domainId, action }),
    }),
  deleteOverride: (agentId: string, domainId: number) =>
    request<{ ok: boolean }>(`/api/admin/agents/${agentId}/overrides/${domainId}`, { method: 'DELETE' }),
}
```

不引入 Pinia / 状态管理库——admin 页是相对独立的视图，每页 onMounted 拉数据即可。

---

## 6. 测试策略

### 6.1 Server 单元测试

**`server/internal/store/admin_test.go`**（新文件，5 个测试）：
- `TestUpdateDomainMeta` — 改 is_global + remark 成功，host/port 不变
- `TestUpdateDomainMeta_NotFound` — 不存在的 ID 返回 ErrRecordNotFound
- `TestUpdateAgentRemark` — 改 remark 成功
- `TestListOverrides` — 返回 include + exclude 两条
- `TestUpsertOverride` — 第一次插入，第二次更新 action

**`server/internal/api/admin_test.go`**（扩展现有文件，新增测试）：
- `TestUpdateDomain_Success` — PUT 200 + DB 状态正确
- `TestUpdateDomain_NotFound` — 404
- `TestUpdateDomain_MissingFields` — 缺字段 400
- `TestUpdateDomain_IgnoresHostInBody` — 即使请求体含 host 也不改 host
- `TestListAgents_IncludesOnlineFlag` — 含 is_online 字段，最近心跳 → true，远古心跳 → false
- `TestUpdateAgentRemark_Success` — PUT 200
- `TestUpdateAgentRemark_NotFound` — 404
- `TestListOverrides_Empty` — 无 override 返回空数组
- `TestListOverrides_IncludeAndExclude` — 两类都返回
- `TestSetOverride_New` — 首次插入
- `TestSetOverride_UpdateAction` — 同 (agent, domain) 改 action
- `TestSetOverride_InvalidAction` — 非 include/exclude → 400
- `TestSetOverride_DomainNotFound` — domain_id 不存在 → 404
- `TestSetOverride_AgentNotFound` — agent_id 不存在 → 404
- `TestDeleteOverride_Success` — 200 + DB 删除
- `TestDeleteOverride_Idempotent` — 不存在也 200

### 6.2 前端测试

延续 Plan 2 / 3.1 的策略：不写前端单元测试，依赖手动 + curl 端到端验证。

### 6.3 端到端冒烟（最后任务）

启动 server + 浏览器登录后，逐项验证：

1. Header 出现"域名管理" / "Agent 管理"链接
2. 未登录直接访问 `/admin/domains` → 跳 `/login?redirect=/admin/domains`，登录后跳回
3. `/admin/domains` 新建一个域名 → 列表出现 → 编辑 remark → 保存 → 删除
4. `/admin/agents` 列表显示已注册的 Agent → 编辑 remark → 保存
5. 进入某 Agent 的 override 页：切换若干域名状态 → 刷新页面验证持久 → 验证 Agent 实际拉取的域名列表（GET /api/agent/domains）反映 override
6. 离线 Agent（人为改 last_seen_at 为 5 小时前）显示灰色 + "离线 5 小时前"
7. Plan 2 dashboard 仍正常（验证 AgentOnlineWindow 常量替换没有破坏现有功能）

---

## 7. 文件清单

### 7.1 Server 新增

| 文件 | 职责 |
|------|------|
| `server/internal/store/admin_test.go` | Store 层 admin 操作测试 |

（admin handler 测试加在已有 `admin_test.go`，扩展不新增文件）

### 7.2 Server 修改

| 文件 | 修改内容 |
|------|---------|
| `server/internal/store/store.go` | 新增 `UpdateDomainMeta` / `UpdateAgentRemark` / `ListOverrides` / `UpsertOverride` 方法；导出常量 `AgentOnlineWindow` |
| `server/internal/api/admin.go` | 新增 6 个 handler 方法（UpdateDomain / ListAgents / UpdateAgent / ListOverrides / SetOverride / DeleteOverride） |
| `server/internal/api/router.go` | 注册 6 条新路由 |
| `server/internal/api/dashboard.go` | `NewDashboardHandler` 内部用 `store.AgentOnlineWindow` 替代硬编码 3h |
| `server/cmd/server/main.go` | 替换 `3*time.Hour` 为 `store.AgentOnlineWindow` |
| `server/internal/api/admin_test.go` | 扩展测试（新增上文列出的测试用例） |

### 7.3 前端新增

| 文件 | 职责 |
|------|------|
| `web/src/views/AdminDomains.vue` | 域名 CRUD 页 |
| `web/src/views/AdminAgents.vue` | Agent 列表 + 备注编辑 |
| `web/src/views/AdminAgentOverrides.vue` | 单 Agent 域名 override 矩阵 |

### 7.4 前端修改

| 文件 | 修改内容 |
|------|---------|
| `web/src/router.ts` | 新增 3 个路由（带 meta.requiresAuth） |
| `web/src/components/Header.vue` | 已登录时显示导航链接 |
| `web/src/types.ts` | 新增 `DomainAdmin` / `AgentAdmin` / `Override` 接口 |
| `web/src/api.ts` | 新增 `adminApi` |

---

## 8. 关键设计决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 域名编辑范围 | 仅 is_global / remark | 域名身份不可变，避免历史数据语义飘移 |
| Agent 列表字段 | 完整字段（hostname/ip/registered_at 全返回） | 排查友好，10-20 台部署不需分页 |
| Agent 删除 | 本期不实现 | 单管理员低频场景，YAGNI |
| Override UI | 域名清单 × 三态切换 | 一目了然 Agent 实际监控范围 |
| Header 导航 | 顶级链接 + 用户菜单 | 4-5 个管理项扁平够用 |
| 删除确认 | 浏览器原生 confirm() | 零样式负担 |
| 编辑模式 | 行内展开 | 字段少（1-2 个），不打断列表 |
| 在线阈值 | 抽常量 AgentOnlineWindow=3h，全站共用 | 一处修改全站生效 |
| Override 切换并发 | 行内独立 loading | 不同行可并行操作 |
| 离线 Agent override | 照常可编辑 | override 是配置数据，与在线状态无关 |
| 状态管理 | 每页 onMounted 自取 | 不引入 Pinia，YAGNI |
| Override 写入语义 | Upsert（POST 重复 = 更新 action） | 前端不需要先查再删再插 |

---

## 9. 后续计划（不属于本期）

- **Plan 3.3 — 告警引擎**：状态机 + 5 渠道发送（email/webhook/dingtalk/feishu/wecom）+ 告警渠道 CRUD + 测试发送 + 历史查询页 + 历史清理
- 如需 Agent 删除、用户管理 CRUD、CSRF Token、Session 持久化，可在后续阶段视需要追加

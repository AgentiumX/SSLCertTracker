# Plan 3.1：登录认证系统 — 设计规格

- **日期**：2026-06-03
- **作者**：chenjiefeng（与 Claude 头脑风暴）
- **版本**：v1.0
- **状态**：草案，待评审
- **关联**：拆分自 [SSL 证书监控系统 — 设计规格](2026-06-02-ssl-cert-tracker-design.md) 第 4.3 节

---

## 1. 背景与目标

为 SSL 证书监控系统增加管理员登录功能，作为后续 Plan 3.2（管理后台）和 Plan 3.3（告警引擎）的基础设施。

**本期范围（Plan 3.1）：**

- 登录、登出、当前用户查询 API
- Cookie + 内存 Session 管理
- 前端登录页 (`/login`) + Header 登录按钮
- `/api/admin/*` 路由加上认证中间件
- 前端路由守卫框架（`meta.requiresAuth`），本期暂无受保护路由

**非目标（推迟到 Plan 3.2 / 3.3）：**

- 用户管理 CRUD（增删用户、修改密码）
- Session 持久化（DB 存储）
- 登录限流（依赖反代层）
- `/admin/*` 前端管理页面
- CSRF Token

---

## 2. 架构

```
┌──────────────────────────────────────────┐
│  Browser                                 │
│  ┌────────────────────────────────────┐  │
│  │  Vue 前端                          │  │
│  │  Header: [登录] / [用户名 ▼ 登出]  │  │
│  │  /login 页面                       │  │
│  │  useAuth() composable              │  │
│  │  router beforeEach 守卫(基础设施)  │  │
│  └────────────────────────────────────┘  │
│              ↓ Cookie: session_id        │
└──────────────────────────────────────────┘
           ↓ HTTPS
┌──────────────────────────────────────────┐
│  Server                                  │
│  ┌────────────────────────────────────┐  │
│  │  Auth Middleware                   │  │
│  │  └─ /api/auth/me, /api/admin/*     │  │
│  └────────────────────────────────────┘  │
│  ┌────────────────────────────────────┐  │
│  │  Auth API Handler                  │  │
│  │  ├─ POST /api/auth/login           │  │
│  │  ├─ POST /api/auth/logout          │  │
│  │  └─ GET  /api/auth/me              │  │
│  └────────────────────────────────────┘  │
│  ┌────────────────────────────────────┐  │
│  │  Session Store (内存 sync.Map)     │  │
│  │  session_id → {user_id, expires}   │  │
│  └────────────────────────────────────┘  │
│  ┌────────────────────────────────────┐  │
│  │  User Store (GORM users 表)        │  │
│  │  └─ bcrypt password_hash           │  │
│  └────────────────────────────────────┘  │
└──────────────────────────────────────────┘
```

**职责切分：**

| 组件 | 职责 |
|------|------|
| `internal/auth/session.go` | SessionStore 数据结构、生命周期管理、清理 goroutine |
| `internal/auth/password.go` | bcrypt 密码 hash/verify 包装 |
| `internal/auth/middleware.go` | AuthMiddleware（与现有 AgentTokenMiddleware 同包） |
| `internal/api/auth_handler.go` | login/logout/me 的 HTTP handler |
| `internal/store/store.go` | User 表 CRUD（CreateUser、GetUserByUsername） |
| `cmd/server/main.go` | 启动时初始化 SessionStore、创建初始管理员 |
| `web/src/composables/useAuth.ts` | 前端登录状态管理（user ref + login/logout/fetchMe） |
| `web/src/views/Login.vue` | 登录表单页 |
| `web/src/components/Header.vue` | 顶部导航 + 登录按钮/用户菜单 |

---

## 3. 数据模型

### 3.1 `users` 表（复用第 3.7 节）

```go
type User struct {
    ID           uint      `gorm:"primaryKey"`
    Username     string    `gorm:"uniqueIndex;not null"`
    PasswordHash string    `gorm:"not null"`           // bcrypt cost=10
    CreatedAt    time.Time
}
```

**AutoMigrate**：在 `main.go` 现有迁移列表中追加 `&store.User{}`。

### 3.2 初始管理员创建逻辑

启动时（AutoMigrate 之后），执行：

```
count := store.CountUsers()
if count == 0:
    if cfg.Auth.AdminUsername == "" || cfg.Auth.AdminPassword == "":
        log.Fatal("users table empty and auth.admin_username/password not configured")
    hash := bcrypt.GenerateFromPassword(cfg.Auth.AdminPassword, cost=10)
    store.CreateUser(cfg.Auth.AdminUsername, hash)
    log.Printf("created initial admin user: %s", cfg.Auth.AdminUsername)
else:
    // 不更新已存在的用户密码，避免重启重置
    log.Printf("users table has %d users, skipping initial admin creation", count)
```

**说明：** 一旦初始用户创建成功，配置中的 `admin_password` 字段可注释或留空，重启时不会再被使用。

### 3.3 Session（不入库）

存内存：

```go
type Session struct {
    UserID    uint
    Username  string
    ExpiresAt time.Time
}
```

Server 重启后所有 Session 失效，用户重新登录。

---

## 4. API 契约

> 错误响应统一格式：`{ "error": { "code": "string", "message": "string" } }`

### 4.1 `POST /api/auth/login`

**请求：**
```json
{ "username": "admin", "password": "secret" }
```

**成功响应：** `200 OK`
```json
{ "user": { "id": 1, "username": "admin" } }
```
- 响应头：`Set-Cookie: session_id=<64_hex_chars>; Path=/; HttpOnly; SameSite=Lax; Secure=<configured>; Max-Age=<ttl_seconds>`

**失败响应：** `401 Unauthorized`
```json
{ "error": { "code": "invalid_credentials", "message": "invalid credentials" } }
```
- 不区分用户名错和密码错（防用户名枚举）
- 即使用户不存在也运行一次 bcrypt 比较（用预存的 dummy hash），防时序攻击

**输入校验：** `username` 和 `password` 必填，否则 `400 Bad Request`。

### 4.2 `POST /api/auth/logout`

**请求：** 无 body，自动从 Cookie 取 session_id

**响应：** `200 OK`
```json
{ "ok": true }
```
- 响应头：`Set-Cookie: session_id=; Path=/; Max-Age=0`（清除 Cookie）
- 从 SessionStore 删除该 session_id
- 未登录调用也返回 200（幂等，不报错）

### 4.3 `GET /api/auth/me`

**已登录响应：** `200 OK`
```json
{ "user": { "id": 1, "username": "admin" } }
```
- **副作用：** 滑动续期：
  - 将该 Session 的 `ExpiresAt` 重置为 `now + ttl`
  - 重发 `Set-Cookie` 头，更新浏览器侧 Cookie 的 `Max-Age=<ttl_seconds>`，与 Server 侧 Session 保持同步
- 同步刷新的目的：避免"Server Session 还有效但浏览器 Cookie 已过期"导致用户被异常踢出

**未登录响应：** `401 Unauthorized`
```json
{ "error": { "code": "unauthenticated", "message": "not logged in" } }
```

### 4.4 受保护路由

本期生效：
- `GET /api/auth/me` → AuthMiddleware
- `POST/GET/PUT/DELETE /api/admin/*` → AuthMiddleware（现有路由本期补上保护）

不受保护：
- `/api/dashboard/*`（公开 Dashboard）
- `/api/agent/*`（已有 AgentTokenMiddleware，使用 Bearer Token，不走 Cookie）
- `/api/auth/login`、`/api/auth/logout`（登录/登出入口本身）
- 静态文件 + SPA fallback

---

## 5. Session 管理

### 5.1 数据结构

```go
type Session struct {
    UserID    uint
    Username  string
    ExpiresAt time.Time
}

type SessionStore struct {
    mu       sync.RWMutex
    sessions map[string]*Session
    ttl      time.Duration
}
```

### 5.2 关键方法

```go
// 生成新 Session，返回 session_id（64 字符 hex）
func (s *SessionStore) Create(userID uint, username string) string

// 查询 Session：
//   - 不存在或已过期：返回 (nil, false)，过期时同步删除
//   - 存在且未过期：滑动续期 ExpiresAt = now + ttl，返回 (sess, true)
func (s *SessionStore) Get(sessionID string) (*Session, bool)

// 删除 Session（登出用）
func (s *SessionStore) Delete(sessionID string)

// 清理所有过期 Session（清理 goroutine 调用）
func (s *SessionStore) Cleanup()
```

### 5.3 SessionID 生成

```go
b := make([]byte, 32)
crypto/rand.Read(b)
sessionID := hex.EncodeToString(b)  // 64 字符
```

碰撞概率可忽略（2^256），不可预测。

### 5.4 清理策略

启动时启动 goroutine：

```go
go func() {
    ticker := time.NewTicker(10 * time.Minute)
    defer ticker.Stop()
    for range ticker.C {
        store.Cleanup()
    }
}()
```

每 10 分钟扫描一次，删除 `ExpiresAt < now` 的项。Server 重启 map 直接清空，无需持久化清理。

### 5.5 TTL 配置

从 `cfg.Session.TTL` 读取，使用 `time.ParseDuration`，默认 `"24h"`。配置缺失时使用默认值。

### 5.6 Auth 中间件

```go
func AuthMiddleware(store *SessionStore) gin.HandlerFunc {
    return func(c *gin.Context) {
        sid, err := c.Cookie("session_id")
        if err != nil {
            c.AbortWithStatusJSON(401, gin.H{
                "error": gin.H{"code": "unauthenticated", "message": "not logged in"},
            })
            return
        }
        sess, ok := store.Get(sid)
        if !ok {
            c.AbortWithStatusJSON(401, gin.H{
                "error": gin.H{"code": "unauthenticated", "message": "not logged in"},
            })
            return
        }
        c.Set("user_id", sess.UserID)
        c.Set("username", sess.Username)
        c.Next()
    }
}
```

---

## 6. Cookie 安全配置

| 属性 | 值 | 理由 |
|------|------|------|
| `HttpOnly` | `true` | 防 XSS 通过 `document.cookie` 窃取 |
| `SameSite` | `Lax` | 防 CSRF，同时允许导航场景下的 Cookie 发送 |
| `Secure` | 配置可控 | HTTPS 部署设 `true`，本地 HTTP 开发设 `false` |
| `Path` | `/` | 全站可用 |
| `Max-Age` | `ttl_seconds` | 浏览器侧过期时间，每次 `/api/auth/me` 调用时与 Server 侧 Session 同步刷新 |

**`Secure` 来源：** 新增 `cfg.Session.Secure bool` 配置项，默认 `false`（兼容本地开发）。生产环境配 `true`。

---

## 7. 前端

### 7.1 `useAuth.ts` (composable)

```ts
import { ref } from 'vue'
import { authApi } from '@/api'

export interface User { id: number; username: string }

const user = ref<User | null>(null)
const loading = ref(true)
const initialized = ref(false)

async function fetchMe(): Promise<void> {
    try {
        const res = await authApi.me()
        user.value = res.user
    } catch {
        user.value = null
    } finally {
        loading.value = false
        initialized.value = true
    }
}

async function login(username: string, password: string): Promise<void> {
    const res = await authApi.login(username, password)
    user.value = res.user
}

async function logout(): Promise<void> {
    await authApi.logout()
    user.value = null
}

export function useAuth() {
    return { user, loading, initialized, fetchMe, login, logout }
}
```

**模块级单例**：`user`、`loading`、`initialized` 是模块级 ref，所有调用 `useAuth()` 的组件共享同一份状态。

### 7.2 `App.vue` 改造

- 启动时调一次 `fetchMe()` 初始化 user 状态
- 挂载 `<Header />` 组件替换原来内联的 header

### 7.3 `Header.vue`

```
<header>
  <Logo + SSL Tracker (RouterLink to /)>
  <Spacer />
  <Slot:>
    if loading: nothing (避免闪烁)
    if !user:   <RouterLink to="/login">登录</RouterLink>
    if user:    <UserMenu username + 登出按钮>
  </Slot>
</header>
```

UserMenu 简化版：用 `<details><summary>` 实现下拉，无需引入额外 UI 库。

### 7.4 `Login.vue`

- 用户名 + 密码输入框
- 登录按钮（loading 时禁用）
- 失败时显示错误（红色提示）
- `onMounted`：等待 `initialized.value === true`（必要时 await `fetchMe()`）后检查 `user.value`，已登录则立即跳转 `route.query.redirect || '/'`
- 登录成功 → 调用 `useAuth().login()` → 跳转 `route.query.redirect || '/'`

### 7.5 路由守卫

```ts
router.beforeEach(async (to) => {
    if (!to.meta.requiresAuth) return
    const { user, initialized, fetchMe } = useAuth()
    if (!initialized.value) await fetchMe()
    if (!user.value) {
        return { path: '/login', query: { redirect: to.fullPath } }
    }
})
```

**本期路由配置：**
```ts
routes: [
    { path: '/', component: Overview },
    { path: '/domains/:id', component: DomainDetail, props: true },
    { path: '/login', component: Login },
]
```

本期没有 `meta: { requiresAuth: true }` 的路由——守卫框架建好备用。

### 7.6 `api.ts` 新增

```ts
export const authApi = {
    login: (username: string, password: string) =>
        request<{ user: User }>('/api/auth/login', {
            method: 'POST',
            body: JSON.stringify({ username, password }),
        }),
    logout: () =>
        request<{ ok: boolean }>('/api/auth/logout', { method: 'POST' }),
    me: () =>
        request<{ user: User }>('/api/auth/me'),
}
```

`request` 函数需要确保 `credentials: 'include'`（已在现有 `api.ts` 中处理则无需改动；如未处理需补上）。

---

## 8. 配置

`server/config.yaml.example` 现有字段已足够，仅需补充注释和添加 `session.secure` 字段：

```yaml
auth:
  agent_token: "your-shared-token-here"
  # 首次启动时，若 users 表为空，使用以下账号创建初始管理员
  # 创建后可清空或注释 admin_password（重启不会再被使用）
  admin_username: "admin"
  admin_password: "change-me-on-first-run"

session:
  ttl: "24h"        # Session 有效期；每次访问自动续期
  secure: false     # 生产环境 HTTPS 部署改为 true，强制 Cookie 仅 HTTPS 传输
```

`session.secret` 字段保留但本期不使用（无签名 Session ID 需求）。

---

## 9. 测试策略

### 9.1 Server 单元测试

**`server/internal/auth/session_test.go`**
- `TestSessionStore_CreateAndGet` — 创建后能取出
- `TestSessionStore_Expired` — 过期 Session 取不到
- `TestSessionStore_SlidingExpiry` — Get 后 ExpiresAt 被刷新
- `TestSessionStore_Delete` — 删除后取不到
- `TestSessionStore_Cleanup` — Cleanup 后过期项被清除
- `TestSessionStore_Concurrent` — `go test -race` 验证并发安全（多 goroutine 同时 Create/Get/Delete）

**`server/internal/auth/password_test.go`**
- `TestHashAndVerify` — hash + verify 正确
- `TestVerifyWrongPassword` — 错误密码 verify 失败

**`server/internal/api/auth_handler_test.go`**
- `TestLogin_Success` — 正确凭证 → 200 + 设置 Cookie + Session 入库
- `TestLogin_WrongPassword` — 错误密码 → 401 + 不设 Cookie
- `TestLogin_UnknownUser` — 用户不存在 → 401 + 不设 Cookie + 验证仍执行 bcrypt（响应时间 ≥ 50ms 作为下限断言，防时序攻击退化）
- `TestLogin_MissingFields` — 缺字段 → 400
- `TestLogout_LoggedIn` — 已登录登出 → 200 + Cookie 清除 + Session 删除
- `TestLogout_NotLoggedIn` — 未登录登出 → 200（幂等）
- `TestMe_LoggedIn` — 携带有效 Cookie → 200 + 返回 user + ExpiresAt 被刷新 + 响应包含新 Set-Cookie 头（Max-Age=ttl）
- `TestMe_NotLoggedIn` — 无 Cookie → 401
- `TestMe_ExpiredSession` — Cookie 在但 Session 已过期 → 401

**`server/internal/api/admin_auth_test.go`**
- `TestAdminRoutes_Unauthenticated` — 未登录访问 `/api/admin/domains` → 401
- `TestAdminRoutes_Authenticated` — 登录后访问 `/api/admin/domains` → 200

**`server/internal/store/user_test.go`**
- `TestCreateUser` — 创建用户成功
- `TestCreateUser_Duplicate` — 重复 username → 错误
- `TestGetUserByUsername_Found` / `_NotFound`
- `TestCountUsers` — 空表返回 0，插入后返回正确数

### 9.2 集成测试（手动 + 端到端冒烟）

启动 server，执行：

1. 首次启动 → 日志显示 `created initial admin user: admin`
2. `POST /api/auth/login` 用错误密码 → 401
3. `POST /api/auth/login` 用正确密码 → 200 + 收到 Cookie
4. `GET /api/auth/me` 携带 Cookie → 200 + 返回 user
5. `GET /api/admin/domains` 携带 Cookie → 200
6. `GET /api/admin/domains` 不携带 Cookie → 401
7. `POST /api/auth/logout` → 200，Cookie 清除
8. `GET /api/auth/me` → 401
9. 浏览器访问 `/` → 看到 Header 右侧"登录"按钮
10. 点击"登录" → 跳转 `/login`
11. 填表登录 → 跳回 `/`，Header 显示用户名和登出
12. 重启 Server → 浏览器刷新 → 登录状态丢失（Session 内存清空，符合预期）

### 9.3 前端测试

Plan 2 没有写前端单元测试，本期保持一致，依赖手动验证。

---

## 10. 文件清单

### 10.1 Server 新增

| 文件 | 职责 |
|------|------|
| `server/internal/auth/session.go` | SessionStore 实现 |
| `server/internal/auth/session_test.go` | Session 单元测试 |
| `server/internal/auth/password.go` | bcrypt 包装 |
| `server/internal/auth/password_test.go` | 密码 hash 测试 |
| `server/internal/auth/middleware.go` | AuthMiddleware（与现有 agent.go 同包） |
| `server/internal/api/auth_handler.go` | login/logout/me handler |
| `server/internal/api/auth_handler_test.go` | Auth handler 测试 |
| `server/internal/api/admin_auth_test.go` | 管理路由保护测试 |
| `server/internal/store/user_test.go` | User CRUD 测试 |

### 10.2 Server 修改

| 文件 | 修改内容 |
|------|---------|
| `server/internal/store/store.go` | 新增 `User` struct、`CreateUser`、`GetUserByUsername`、`CountUsers` |
| `server/internal/api/router.go` | 注册 `/api/auth/*`；`/api/admin/*` 加 AuthMiddleware；签名变更：增加 `*SessionStore` 参数 |
| `server/cmd/server/main.go` | AutoMigrate 加 `&store.User{}`；创建 SessionStore；初始管理员创建；启动清理 goroutine；传 SessionStore 给 SetupRouter |
| `server/config.yaml.example` | 注释提示初始密码用法；新增 `session.secure` 字段 |
| `server/internal/config/config.go` | `SessionConfig` 增加 `Secure bool` |
| `server/go.mod` | 新增 `golang.org/x/crypto/bcrypt` 依赖 |

### 10.3 前端新增

| 文件 | 职责 |
|------|------|
| `web/src/views/Login.vue` | 登录表单页 |
| `web/src/composables/useAuth.ts` | 认证状态管理 |
| `web/src/components/Header.vue` | 顶部导航 + 登录按钮/用户菜单 |

### 10.4 前端修改

| 文件 | 修改内容 |
|------|---------|
| `web/src/App.vue` | 抽 header 到 Header 组件；启动时 fetchMe |
| `web/src/router.ts` | 加 `/login` 路由；beforeEach 守卫 |
| `web/src/types.ts` | 新增 `User` 接口 |
| `web/src/api.ts` | 新增 `authApi`；确保 `credentials: 'include'` |

---

## 11. 关键设计决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| Session 存储 | 内存 sync.Map | 单管理员，重启重登录可接受，避免 DB 表和清理任务复杂度 |
| Cookie 安全 | HttpOnly + SameSite=Lax + Secure 配置可控 | 防 XSS/CSRF，本地开发不受阻 |
| 初始管理员 | config.yaml 启动初始化 | 设计文档已有字段，简单可靠 |
| 限流 | 不做 | 单管理员场景；bcrypt 天然慢；爆破防护应在反代层 |
| 前端守卫 | beforeEach + meta.requiresAuth | 提前建好框架，Plan 3.2 加 admin 路由直接用 |
| 错误信息 | 统一 "invalid credentials" | 防用户名枚举 |
| 时序攻击 | 用户不存在也跑 bcrypt | 防时序枚举 |
| Session 续期 | 滑动续期（每次 Get 刷新 ExpiresAt + 同步重发 Cookie 刷新 Max-Age） | 活跃用户不会被踢；保证浏览器 Cookie 与 Server Session 过期时间一致 |
| 状态管理 | composables + 模块级 ref | 不引入 Pinia，单一全局状态够用 |

---

## 12. 后续计划（不属于本期）

- **Plan 3.2 — 管理后台**：基于本期的 Auth 框架，实现 `/api/admin/*` 完整 CRUD（agents、overrides、alert-channels、history）和对应的前端管理页面
- **Plan 3.3 — 告警引擎**：状态机 + 5 渠道发送 + 每日提醒 + 历史清理
- 如需持久化 Session（重启不掉线）、登录限流、用户管理 CRUD、CSRF Token，可在 3.2/3.3 阶段视需要追加

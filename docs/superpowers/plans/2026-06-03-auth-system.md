# Plan 3.1：登录认证系统 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 为 SSL 证书监控系统增加管理员登录功能（POST /api/auth/login + bcrypt + Cookie/内存 Session），并保护 /api/admin/* 路由；前端增加 /login 页面、Header 登录按钮、useAuth composable 和路由守卫框架。

**架构：** 后端用内存 sync.Map 存 Session（Server 重启失效），bcrypt 验证密码（防时序攻击），AuthMiddleware 保护管理路由，启动时从配置创建初始管理员。前端用模块级 ref 管理登录状态，App.vue 启动时调一次 /api/auth/me 初始化，路由守卫通过 meta.requiresAuth 标记保护页面（本期暂无）。

**技术栈：** Go + Gin + GORM + golang.org/x/crypto/bcrypt；Vue 3 + vue-router + lucide-vue-next。

**关联规格：** [docs/superpowers/specs/2026-06-03-auth-system-design.md](../specs/2026-06-03-auth-system-design.md)

---

## 文件结构

### Server 端

**store 层**
- 修改 `server/internal/store/models.go` — 新增 `User` 结构体
- 修改 `server/internal/store/store.go` — 新增 `CreateUser` / `GetUserByUsername` / `CountUsers`
- 测试 `server/internal/store/user_test.go`

**auth 包（新增 3 个文件，与现有 token.go 同包）**
- 创建 `server/internal/auth/password.go` — bcrypt 包装
- 创建 `server/internal/auth/password_test.go`
- 创建 `server/internal/auth/session.go` — SessionStore
- 创建 `server/internal/auth/session_test.go`
- 创建 `server/internal/auth/middleware.go` — AuthMiddleware

**API 层**
- 创建 `server/internal/api/auth_handler.go` — login/logout/me handler
- 创建 `server/internal/api/auth_handler_test.go`
- 创建 `server/internal/api/admin_auth_test.go` — 管理路由保护测试
- 修改 `server/internal/api/router.go` — 注册 auth 路由 + admin 加中间件 + 签名增加 `*SessionStore`

**配置 + 入口**
- 修改 `server/internal/config/config.go` — `SessionConfig` 增加 `Secure bool`
- 修改 `server/config.yaml.example` — 注释 + 加 `session.secure`
- 修改 `server/cmd/server/main.go` — AutoMigrate User、初始管理员、SessionStore、清理 goroutine

**依赖**
- 修改 `server/go.mod` — `golang.org/x/crypto/bcrypt`（已传递依赖，需显式提到 require）

### Web 前端

**新增**
- `web/src/composables/useAuth.ts`
- `web/src/views/Login.vue`
- `web/src/components/Header.vue`

**修改**
- `web/src/App.vue` — 抽 header 到 Header 组件、启动时 fetchMe
- `web/src/router.ts` — 加 /login 路由 + beforeEach 守卫
- `web/src/types.ts` — 新增 User 接口
- `web/src/api.ts` — 新增 authApi、确保 credentials: 'include'

---

## 任务清单

1. **任务 1：User 数据模型 + Store CRUD**（后端，独立可测）
2. **任务 2：bcrypt 密码包装**（后端，独立可测）
3. **任务 3：内存 SessionStore**（后端，独立可测）
4. **任务 4：AuthMiddleware**（后端，依赖任务 3）
5. **任务 5：Auth API Handler（login/logout/me）**（后端，依赖任务 1-4）
6. **任务 6：Router 接线 + Admin 路由保护**（后端，依赖任务 5）
7. **任务 7：配置更新（Secure 字段 + yaml example）**（后端独立）
8. **任务 8：Server 启动初始化（main.go）**（后端，依赖 1-7）
9. **任务 9：前端 useAuth composable + types + api**（前端基础设施）
10. **任务 10：Login 页面 + Header 组件 + 路由集成**（前端 UI，依赖任务 9）
11. **任务 11：端到端冒烟验证**（全栈）

---

## 任务 1：User 数据模型 + Store CRUD

**文件：**
- 修改：`server/internal/store/models.go`（追加 User 结构体）
- 修改：`server/internal/store/store.go`（追加 3 个方法）
- 创建：`server/internal/store/user_test.go`

- [ ] **步骤 1：编写失败的测试**

创建 `server/internal/store/user_test.go`：

```go
package store

import (
	"testing"

	"gorm.io/gorm"
)

func setupUserTestDB(t *testing.T) *Store {
	s := setupTestDB(t)
	if err := s.db.AutoMigrate(&User{}); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCreateUser(t *testing.T) {
	s := setupUserTestDB(t)
	u := &User{Username: "admin", PasswordHash: "$2a$10$hash"}
	if err := s.CreateUser(u); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if u.ID == 0 {
		t.Errorf("expected ID > 0 after create")
	}
}

func TestCreateUser_Duplicate(t *testing.T) {
	s := setupUserTestDB(t)
	if err := s.CreateUser(&User{Username: "admin", PasswordHash: "h"}); err != nil {
		t.Fatal(err)
	}
	err := s.CreateUser(&User{Username: "admin", PasswordHash: "h2"})
	if err == nil {
		t.Errorf("expected duplicate username error, got nil")
	}
}

func TestGetUserByUsername_Found(t *testing.T) {
	s := setupUserTestDB(t)
	if err := s.CreateUser(&User{Username: "admin", PasswordHash: "h"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("GetUserByUsername failed: %v", err)
	}
	if got.Username != "admin" {
		t.Errorf("expected admin, got %s", got.Username)
	}
}

func TestGetUserByUsername_NotFound(t *testing.T) {
	s := setupUserTestDB(t)
	_, err := s.GetUserByUsername("nope")
	if err != gorm.ErrRecordNotFound {
		t.Errorf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestCountUsers(t *testing.T) {
	s := setupUserTestDB(t)
	n, err := s.CountUsers()
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
	if err := s.CreateUser(&User{Username: "a", PasswordHash: "h"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateUser(&User{Username: "b", PasswordHash: "h"}); err != nil {
		t.Fatal(err)
	}
	n, err = s.CountUsers()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd server && go test ./internal/store/ -run "TestCreateUser|TestGetUserByUsername|TestCountUsers" -v`
预期：编译失败，提示 `undefined: User` 和未定义的方法。

- [ ] **步骤 3：在 models.go 追加 User 结构体**

修改 `server/internal/store/models.go`，在文件末尾追加：

```go
type User struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	CreatedAt    time.Time
}
```

- [ ] **步骤 4：在 store.go 追加 3 个方法**

修改 `server/internal/store/store.go`，在文件末尾追加：

```go
// User operations

func (s *Store) CreateUser(u *User) error {
	return s.db.Create(u).Error
}

func (s *Store) GetUserByUsername(username string) (*User, error) {
	var u User
	err := s.db.Where("username = ?", username).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) CountUsers() (int64, error) {
	var n int64
	err := s.db.Model(&User{}).Count(&n).Error
	return n, err
}
```

- [ ] **步骤 5：运行测试验证通过**

运行：`cd server && go test ./internal/store/ -run "TestCreateUser|TestGetUserByUsername|TestCountUsers" -v`
预期：4 个测试全部 PASS。

- [ ] **步骤 6：Commit**

```bash
git add server/internal/store/models.go server/internal/store/store.go server/internal/store/user_test.go
git commit -m "feat(store): add User model and CRUD"
```

---

## 任务 2：bcrypt 密码包装

**文件：**
- 创建：`server/internal/auth/password.go`
- 创建：`server/internal/auth/password_test.go`

- [ ] **步骤 1：编写失败的测试**

创建 `server/internal/auth/password_test.go`：

```go
package auth

import "testing"

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Errorf("expected non-empty hash")
	}
	if hash == "hunter2" {
		t.Errorf("hash must not equal plaintext")
	}
	if !VerifyPassword(hash, "hunter2") {
		t.Errorf("VerifyPassword should accept correct password")
	}
}

func TestVerifyWrongPassword(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if VerifyPassword(hash, "wrong") {
		t.Errorf("VerifyPassword should reject wrong password")
	}
}

func TestVerifyPassword_InvalidHash(t *testing.T) {
	if VerifyPassword("not-a-valid-bcrypt-hash", "anything") {
		t.Errorf("VerifyPassword should reject invalid hash format")
	}
}

func TestDummyHash_Verifies(t *testing.T) {
	// DummyHash is used in login flow when user not found, to defend against
	// timing attacks. It must be a valid bcrypt hash so VerifyPassword runs
	// the full computation, but should never match any real password.
	if VerifyPassword(DummyHash, "") {
		t.Errorf("DummyHash should not match empty password")
	}
	if VerifyPassword(DummyHash, "any-password") {
		t.Errorf("DummyHash should not match any password")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd server && go test ./internal/auth/ -run "TestHashAndVerify|TestVerifyWrongPassword|TestVerifyPassword_InvalidHash|TestDummyHash" -v`
预期：编译失败，`undefined: HashPassword`。

- [ ] **步骤 3：实现 password.go**

创建 `server/internal/auth/password.go`：

```go
package auth

import "golang.org/x/crypto/bcrypt"

const bcryptCost = 10

// DummyHash is a pre-computed bcrypt hash used to defend against timing
// attacks during login. When a username doesn't exist, the handler verifies
// the submitted password against this hash so the response time matches the
// "user found" branch. The plaintext is intentionally unknown / random.
var DummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd server && go test ./internal/auth/ -run "TestHashAndVerify|TestVerifyWrongPassword|TestVerifyPassword_InvalidHash|TestDummyHash" -v`
预期：4 个测试全部 PASS。

- [ ] **步骤 5：Commit**

```bash
git add server/internal/auth/password.go server/internal/auth/password_test.go
git commit -m "feat(auth): add bcrypt password hash/verify helpers"
```

---

## 任务 3：内存 SessionStore

**文件：**
- 创建：`server/internal/auth/session.go`
- 创建：`server/internal/auth/session_test.go`

- [ ] **步骤 1：编写失败的测试**

创建 `server/internal/auth/session_test.go`：

```go
package auth

import (
	"sync"
	"testing"
	"time"
)

func TestSessionStore_CreateAndGet(t *testing.T) {
	store := NewSessionStore(time.Hour)
	sid := store.Create(42, "admin")
	if len(sid) != 64 {
		t.Errorf("expected 64-char session id, got %d", len(sid))
	}
	sess, ok := store.Get(sid)
	if !ok {
		t.Fatalf("expected to find created session")
	}
	if sess.UserID != 42 || sess.Username != "admin" {
		t.Errorf("unexpected session payload: %+v", sess)
	}
}

func TestSessionStore_GetUnknown(t *testing.T) {
	store := NewSessionStore(time.Hour)
	if _, ok := store.Get("does-not-exist"); ok {
		t.Errorf("expected ok=false for unknown session id")
	}
}

func TestSessionStore_Expired(t *testing.T) {
	store := NewSessionStore(10 * time.Millisecond)
	sid := store.Create(1, "u")
	time.Sleep(20 * time.Millisecond)
	if _, ok := store.Get(sid); ok {
		t.Errorf("expired session should not be returned")
	}
}

func TestSessionStore_SlidingExpiry(t *testing.T) {
	store := NewSessionStore(100 * time.Millisecond)
	sid := store.Create(1, "u")
	time.Sleep(60 * time.Millisecond)
	// Get should refresh ExpiresAt
	if _, ok := store.Get(sid); !ok {
		t.Fatalf("session should still be valid")
	}
	time.Sleep(60 * time.Millisecond) // total 120ms since create, but only 60ms since last Get
	if _, ok := store.Get(sid); !ok {
		t.Errorf("sliding expiry should have kept session alive")
	}
}

func TestSessionStore_Delete(t *testing.T) {
	store := NewSessionStore(time.Hour)
	sid := store.Create(1, "u")
	store.Delete(sid)
	if _, ok := store.Get(sid); ok {
		t.Errorf("deleted session should not be retrievable")
	}
}

func TestSessionStore_Delete_Unknown(t *testing.T) {
	store := NewSessionStore(time.Hour)
	store.Delete("does-not-exist") // must not panic
}

func TestSessionStore_Cleanup(t *testing.T) {
	store := NewSessionStore(10 * time.Millisecond)
	sid1 := store.Create(1, "u1")
	time.Sleep(20 * time.Millisecond)
	sid2 := store.Create(2, "u2")
	store.Cleanup()
	// sid1 expired, should be gone after Cleanup
	if _, ok := store.peekRaw(sid1); ok {
		t.Errorf("expected sid1 to be cleaned up")
	}
	// sid2 still fresh
	if _, ok := store.peekRaw(sid2); !ok {
		t.Errorf("expected sid2 to remain")
	}
}

func TestSessionStore_Concurrent(t *testing.T) {
	store := NewSessionStore(time.Hour)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sid := store.Create(uint(i), "u")
			_, _ = store.Get(sid)
			store.Delete(sid)
		}(i)
	}
	wg.Wait()
}

func TestSessionStore_TTL(t *testing.T) {
	store := NewSessionStore(time.Hour)
	if store.TTL() != time.Hour {
		t.Errorf("expected TTL=1h, got %v", store.TTL())
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd server && go test ./internal/auth/ -run "TestSessionStore" -v`
预期：编译失败，`undefined: NewSessionStore`。

- [ ] **步骤 3：实现 session.go**

创建 `server/internal/auth/session.go`：

```go
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

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

func NewSessionStore(ttl time.Duration) *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*Session),
		ttl:      ttl,
	}
}

func (s *SessionStore) TTL() time.Duration {
	return s.ttl
}

// Create generates a new session and returns the session id (64 hex chars).
func (s *SessionStore) Create(userID uint, username string) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read on a healthy system never fails; if it does we
		// cannot generate a session safely.
		panic("crypto/rand failed: " + err.Error())
	}
	sid := hex.EncodeToString(b)
	s.mu.Lock()
	s.sessions[sid] = &Session{
		UserID:    userID,
		Username:  username,
		ExpiresAt: time.Now().Add(s.ttl),
	}
	s.mu.Unlock()
	return sid
}

// Get returns the session if it exists and has not expired. On a hit it also
// performs sliding expiry: ExpiresAt is reset to now+ttl.
func (s *SessionStore) Get(sid string) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sid]
	if !ok {
		return nil, false
	}
	if time.Now().After(sess.ExpiresAt) {
		delete(s.sessions, sid)
		return nil, false
	}
	sess.ExpiresAt = time.Now().Add(s.ttl)
	// Return a copy so callers can't mutate internal state.
	cp := *sess
	return &cp, true
}

// peekRaw returns whether a session exists in the map, without expiry checks
// or sliding renewal. Test-only helper.
func (s *SessionStore) peekRaw(sid string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[sid]
	return sess, ok
}

func (s *SessionStore) Delete(sid string) {
	s.mu.Lock()
	delete(s.sessions, sid)
	s.mu.Unlock()
}

// Cleanup removes all expired sessions.
func (s *SessionStore) Cleanup() {
	now := time.Now()
	s.mu.Lock()
	for sid, sess := range s.sessions {
		if now.After(sess.ExpiresAt) {
			delete(s.sessions, sid)
		}
	}
	s.mu.Unlock()
}
```

- [ ] **步骤 4：运行测试验证通过（含 race 检测）**

运行：`cd server && go test -race ./internal/auth/ -run "TestSessionStore" -v`
预期：所有测试 PASS，无数据竞争。

- [ ] **步骤 5：Commit**

```bash
git add server/internal/auth/session.go server/internal/auth/session_test.go
git commit -m "feat(auth): add in-memory SessionStore with sliding expiry"
```

---

## 任务 4：AuthMiddleware

**文件：**
- 创建：`server/internal/auth/middleware.go`

注：本任务的中间件行为由任务 6 的 `admin_auth_test.go` 通过完整 router 端到端验证。这里不另写单元测试——因为 gin handler 测试需要构造完整请求上下文，与端到端测试高度重复。

- [ ] **步骤 1：实现 middleware.go**

创建 `server/internal/auth/middleware.go`：

```go
package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const SessionCookieName = "session_id"

// AuthMiddleware ensures the request carries a valid session cookie.
// On success it sets c.Keys["user_id"] and c.Keys["username"].
// On failure it aborts with 401.
func AuthMiddleware(store *SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, err := c.Cookie(SessionCookieName)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": "unauthenticated", "message": "not logged in"},
			})
			return
		}
		sess, ok := store.Get(sid)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
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

- [ ] **步骤 2：构建验证编译**

运行：`cd server && go build ./...`
预期：编译成功，无错误。

- [ ] **步骤 3：Commit**

```bash
git add server/internal/auth/middleware.go
git commit -m "feat(auth): add AuthMiddleware for cookie-based session auth"
```

---

## 任务 5：Auth API Handler（login / logout / me）

**文件：**
- 创建：`server/internal/api/auth_handler.go`
- 创建：`server/internal/api/auth_handler_test.go`

本任务在 router 中注册临时 routes 仅用于单元测试，正式接线在任务 6。

- [ ] **步骤 1：编写失败的测试**

创建 `server/internal/api/auth_handler_test.go`：

```go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"ssl-tracker/server/internal/auth"
	"ssl-tracker/server/internal/store"
)

func setupAuthAPI(t *testing.T) (*gin.Engine, *store.Store, *auth.SessionStore) {
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "auth.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&store.User{}); err != nil {
		t.Fatal(err)
	}
	s := store.NewStore(db)
	sessions := auth.NewSessionStore(time.Hour)

	hash, err := auth.HashPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateUser(&store.User{Username: "admin", PasswordHash: hash}); err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	h := NewAuthHandler(s, sessions, false)
	r.POST("/api/auth/login", h.Login)
	r.POST("/api/auth/logout", h.Logout)
	r.GET("/api/auth/me", auth.AuthMiddleware(sessions), h.Me)
	return r, s, sessions
}

func postJSON(r *gin.Engine, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func getReq(r *gin.Engine, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func extractSessionCookie(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	res := http.Response{Header: w.Header()}
	for _, c := range res.Cookies() {
		if c.Name == auth.SessionCookieName {
			return c
		}
	}
	t.Fatalf("no session cookie set; headers=%v", w.Header())
	return nil
}

func TestLogin_Success(t *testing.T) {
	r, _, _ := setupAuthAPI(t)
	w := postJSON(r, "/api/auth/login", map[string]string{
		"username": "admin", "password": "secret",
	}, nil)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	c := extractSessionCookie(t, w)
	if c.Value == "" || len(c.Value) != 64 {
		t.Errorf("expected 64-char cookie value, got %q", c.Value)
	}
	if !c.HttpOnly {
		t.Errorf("cookie must be HttpOnly")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie must be SameSite=Lax, got %v", c.SameSite)
	}
	if c.Secure {
		t.Errorf("cookie should not be Secure when configured insecure")
	}
	if c.Path != "/" {
		t.Errorf("cookie path must be /, got %q", c.Path)
	}
	if c.MaxAge <= 0 {
		t.Errorf("cookie MaxAge must be > 0, got %d", c.MaxAge)
	}

	var resp struct{ User struct{ ID int; Username string } }
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.User.Username != "admin" {
		t.Errorf("expected admin in response, got %+v", resp)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	r, _, _ := setupAuthAPI(t)
	w := postJSON(r, "/api/auth/login", map[string]string{
		"username": "admin", "password": "WRONG",
	}, nil)
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid_credentials") {
		t.Errorf("expected error code invalid_credentials, body=%s", w.Body.String())
	}
	if c := w.Header().Get("Set-Cookie"); c != "" {
		t.Errorf("must not set cookie on failure, got %q", c)
	}
}

func TestLogin_UnknownUser(t *testing.T) {
	r, _, _ := setupAuthAPI(t)
	start := time.Now()
	w := postJSON(r, "/api/auth/login", map[string]string{
		"username": "ghost", "password": "anything",
	}, nil)
	elapsed := time.Since(start)
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	// Lower bound to ensure bcrypt was actually run (defense against timing attacks).
	// bcrypt cost=10 takes ~50-100ms even on fast hardware.
	if elapsed < 30*time.Millisecond {
		t.Errorf("login for unknown user returned in %v; bcrypt was not run -> timing attack risk", elapsed)
	}
	if !strings.Contains(w.Body.String(), "invalid_credentials") {
		t.Errorf("expected error code invalid_credentials, body=%s", w.Body.String())
	}
}

func TestLogin_MissingFields(t *testing.T) {
	r, _, _ := setupAuthAPI(t)
	for _, body := range []map[string]string{
		{"username": "admin"},
		{"password": "secret"},
		{},
	} {
		w := postJSON(r, "/api/auth/login", body, nil)
		if w.Code != 400 {
			t.Errorf("body=%v expected 400, got %d resp=%s", body, w.Code, w.Body.String())
		}
	}
}

func TestLogout_LoggedIn(t *testing.T) {
	r, _, sessions := setupAuthAPI(t)
	loginW := postJSON(r, "/api/auth/login", map[string]string{
		"username": "admin", "password": "secret",
	}, nil)
	cookie := extractSessionCookie(t, loginW)
	if _, ok := sessions.Get(cookie.Value); !ok {
		t.Fatal("session should exist after login")
	}

	w := postJSON(r, "/api/auth/logout", nil, cookie)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	// Session removed
	if _, ok := sessions.Get(cookie.Value); ok {
		t.Errorf("session should be deleted on logout")
	}
	// Cookie cleared (Max-Age=0 or negative)
	res := http.Response{Header: w.Header()}
	cleared := false
	for _, c := range res.Cookies() {
		if c.Name == auth.SessionCookieName && c.MaxAge <= 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Errorf("expected logout to clear cookie, headers=%v", w.Header())
	}
}

func TestLogout_NotLoggedIn(t *testing.T) {
	r, _, _ := setupAuthAPI(t)
	w := postJSON(r, "/api/auth/logout", nil, nil)
	if w.Code != 200 {
		t.Errorf("logout without cookie should be idempotent, got %d", w.Code)
	}
}

func TestMe_LoggedIn(t *testing.T) {
	r, _, sessions := setupAuthAPI(t)
	loginW := postJSON(r, "/api/auth/login", map[string]string{
		"username": "admin", "password": "secret",
	}, nil)
	cookie := extractSessionCookie(t, loginW)

	// Snapshot expiry before me call.
	before, _ := sessions.Get(cookie.Value)
	beforeExp := before.ExpiresAt

	time.Sleep(10 * time.Millisecond)

	w := getReq(r, "/api/auth/me", cookie)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	// New Set-Cookie must be present (sliding refresh).
	res := http.Response{Header: w.Header()}
	hasCookie := false
	for _, c := range res.Cookies() {
		if c.Name == auth.SessionCookieName && c.Value == cookie.Value && c.MaxAge > 0 {
			hasCookie = true
		}
	}
	if !hasCookie {
		t.Errorf("expected /me to refresh Set-Cookie, headers=%v", w.Header())
	}

	// ExpiresAt should have advanced.
	after, _ := sessions.Get(cookie.Value)
	if !after.ExpiresAt.After(beforeExp) {
		t.Errorf("ExpiresAt should advance after /me, before=%v after=%v", beforeExp, after.ExpiresAt)
	}
}

func TestMe_NotLoggedIn(t *testing.T) {
	r, _, _ := setupAuthAPI(t)
	w := getReq(r, "/api/auth/me", nil)
	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestMe_ExpiredSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "auth-exp.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&store.User{}); err != nil {
		t.Fatal(err)
	}
	s := store.NewStore(db)
	sessions := auth.NewSessionStore(10 * time.Millisecond)
	hash, _ := auth.HashPassword("secret")
	if err := s.CreateUser(&store.User{Username: "admin", PasswordHash: hash}); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	h := NewAuthHandler(s, sessions, false)
	r.POST("/api/auth/login", h.Login)
	r.GET("/api/auth/me", auth.AuthMiddleware(sessions), h.Me)

	loginW := postJSON(r, "/api/auth/login", map[string]string{
		"username": "admin", "password": "secret",
	}, nil)
	cookie := extractSessionCookie(t, loginW)

	time.Sleep(20 * time.Millisecond) // session expires
	w := getReq(r, "/api/auth/me", cookie)
	if w.Code != 401 {
		t.Errorf("expected 401 after expiry, got %d", w.Code)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd server && go test ./internal/api/ -run "TestLogin|TestLogout|TestMe_" -v`
预期：编译失败，`undefined: NewAuthHandler`。

- [ ] **步骤 3：实现 auth_handler.go**

创建 `server/internal/api/auth_handler.go`：

```go
package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"ssl-tracker/server/internal/auth"
	"ssl-tracker/server/internal/store"
)

type AuthHandler struct {
	store        *store.Store
	sessions     *auth.SessionStore
	cookieSecure bool
}

func NewAuthHandler(s *store.Store, sessions *auth.SessionStore, cookieSecure bool) *AuthHandler {
	return &AuthHandler{store: s, sessions: sessions, cookieSecure: cookieSecure}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func unauthenticated(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error": gin.H{"code": "unauthenticated", "message": "not logged in"},
	})
}

func invalidCredentials(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error": gin.H{"code": "invalid_credentials", "message": "invalid credentials"},
	})
}

func (h *AuthHandler) setSessionCookie(c *gin.Context, sid string) {
	maxAge := int(h.sessions.TTL().Seconds())
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(auth.SessionCookieName, sid, maxAge, "/", "", h.cookieSecure, true)
}

func (h *AuthHandler) clearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(auth.SessionCookieName, "", -1, "/", "", h.cookieSecure, true)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"code": "bad_request", "message": err.Error()},
		})
		return
	}

	user, err := h.store.GetUserByUsername(req.Username)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Run bcrypt against dummy hash so unknown-user response time matches
		// the wrong-password branch (defense against timing-based username enumeration).
		_ = auth.VerifyPassword(auth.DummyHash, req.Password)
		invalidCredentials(c)
		return
	}
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"code": "server_error", "message": err.Error()},
		})
		return
	}

	if !auth.VerifyPassword(user.PasswordHash, req.Password) {
		invalidCredentials(c)
		return
	}

	sid := h.sessions.Create(user.ID, user.Username)
	h.setSessionCookie(c, sid)
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{"id": user.ID, "username": user.Username},
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if sid, err := c.Cookie(auth.SessionCookieName); err == nil {
		h.sessions.Delete(sid)
	}
	h.clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthHandler) Me(c *gin.Context) {
	// AuthMiddleware already validated and refreshed the session; we just need
	// to re-issue Set-Cookie so browser Max-Age stays in sync with server TTL.
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")
	if sid, err := c.Cookie(auth.SessionCookieName); err == nil {
		h.setSessionCookie(c, sid)
	}
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{"id": userID, "username": username},
	})
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd server && go test ./internal/api/ -run "TestLogin|TestLogout|TestMe_" -v`
预期：所有测试 PASS。

- [ ] **步骤 5：Commit**

```bash
git add server/internal/api/auth_handler.go server/internal/api/auth_handler_test.go
git commit -m "feat(api): add login/logout/me handlers with timing-safe auth"
```

---

## 任务 6：Router 接线 + Admin 路由保护

**文件：**
- 修改：`server/internal/api/router.go`
- 创建：`server/internal/api/admin_auth_test.go`

- [ ] **步骤 1：编写失败的测试**

创建 `server/internal/api/admin_auth_test.go`：

```go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"ssl-tracker/server/internal/auth"
	"ssl-tracker/server/internal/store"
)

func setupFullRouter(t *testing.T) (*gin.Engine, *http.Cookie) {
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "router.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&store.Agent{}, &store.Domain{}, &store.AgentDomainOverride{},
		&store.CheckResult{}, &store.User{}); err != nil {
		t.Fatal(err)
	}
	s := store.NewStore(db)
	sessions := auth.NewSessionStore(time.Hour)
	hash, _ := auth.HashPassword("secret")
	if err := s.CreateUser(&store.User{Username: "admin", PasswordHash: hash}); err != nil {
		t.Fatal(err)
	}

	r := SetupRouter(s, "agent-token-xyz", 15, sessions, false, nil)

	// Login to obtain cookie
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "secret"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	res := http.Response{Header: w.Header()}
	var cookie *http.Cookie
	for _, c := range res.Cookies() {
		if c.Name == auth.SessionCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no session cookie after login")
	}
	return r, cookie
}

func TestAdminRoutes_Unauthenticated(t *testing.T) {
	r, _ := setupFullRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/domains", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("unauthenticated /api/admin/domains expected 401, got %d", w.Code)
	}
}

func TestAdminRoutes_Authenticated(t *testing.T) {
	r, cookie := setupFullRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/domains", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("authenticated /api/admin/domains expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestDashboardRoutes_StillPublic(t *testing.T) {
	r, _ := setupFullRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/overview", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("public /api/dashboard/overview expected 200, got %d", w.Code)
	}
}

func TestAuthMe_RequiresAuth(t *testing.T) {
	r, _ := setupFullRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("unauthenticated /api/auth/me expected 401, got %d", w.Code)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd server && go test ./internal/api/ -run "TestAdminRoutes|TestDashboardRoutes_StillPublic|TestAuthMe_RequiresAuth" -v`
预期：编译失败，`SetupRouter` 签名不匹配（少 2 个参数）。

- [ ] **步骤 3：修改 router.go**

将 `server/internal/api/router.go` 整体替换为：

```go
package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"ssl-tracker/server/internal/auth"
	"ssl-tracker/server/internal/store"
)

func SetupRouter(s *store.Store, agentToken string, expireThresholdDays int,
	sessions *auth.SessionStore, cookieSecure bool, webHandler http.Handler) *gin.Engine {
	r := gin.Default()

	agentGroup := r.Group("/api/agent")
	agentGroup.Use(auth.AgentTokenMiddleware(agentToken))
	{
		h := NewAgentHandler(s, expireThresholdDays)
		agentGroup.POST("/register", h.Register)
		agentGroup.GET("/domains", h.GetDomains)
		agentGroup.POST("/results", h.PostResults)
	}

	dash := r.Group("/api/dashboard")
	{
		h := NewDashboardHandler(s, 3*time.Hour)
		dash.GET("/overview", h.Overview)
		dash.GET("/domains", h.Domains)
		dash.GET("/domains/:id", h.DomainDetail)
	}

	authH := NewAuthHandler(s, sessions, cookieSecure)
	r.POST("/api/auth/login", authH.Login)
	r.POST("/api/auth/logout", authH.Logout)
	r.GET("/api/auth/me", auth.AuthMiddleware(sessions), authH.Me)

	adminGroup := r.Group("/api/admin")
	adminGroup.Use(auth.AuthMiddleware(sessions))
	{
		h := NewAdminHandler(s)
		adminGroup.POST("/domains", h.CreateDomain)
		adminGroup.GET("/domains", h.ListDomains)
		adminGroup.GET("/domains/:id", h.GetDomain)
		adminGroup.DELETE("/domains/:id", h.DeleteDomain)
	}

	if webHandler != nil {
		r.NoRoute(gin.WrapH(webHandler))
	}
	return r
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd server && go test ./internal/api/ -v`
预期：所有 api 测试通过（包括之前的 dashboard / agent / admin 测试，以及新加的 auth 测试）。

- [ ] **步骤 5：Commit**

```bash
git add server/internal/api/router.go server/internal/api/admin_auth_test.go
git commit -m "feat(api): wire auth routes and protect /api/admin/* with middleware"
```

---

## 任务 7：配置更新（Secure 字段 + yaml example）

**文件：**
- 修改：`server/internal/config/config.go`
- 修改：`server/config.yaml.example`

- [ ] **步骤 1：在 SessionConfig 加 Secure 字段**

修改 `server/internal/config/config.go`，把 `SessionConfig` 整体替换为：

```go
type SessionConfig struct {
	Secret string `yaml:"secret"`
	TTL    string `yaml:"ttl"`
	Secure bool   `yaml:"secure"`
}
```

- [ ] **步骤 2：更新 yaml example**

将 `server/config.yaml.example` 的 `auth` 和 `session` 段替换为：

```yaml
auth:
  agent_token: "your-shared-token-here"
  # 首次启动时，若 users 表为空，使用以下账号创建初始管理员
  # 创建后可清空或注释 admin_password（重启不会再被使用）
  admin_username: "admin"
  admin_password: "change-me-on-first-run"
```

```yaml
session:
  secret: ""        # 本期不使用，保留供未来 CSRF / 签名 Session 使用
  ttl: "24h"        # Session 有效期；每次访问自动续期
  secure: false     # 生产环境 HTTPS 部署改为 true，强制 Cookie 仅 HTTPS 传输
```

完整文件内容应为：

```yaml
server:
  listen: ":8080"

auth:
  agent_token: "your-shared-token-here"
  # 首次启动时，若 users 表为空，使用以下账号创建初始管理员
  # 创建后可清空或注释 admin_password（重启不会再被使用）
  admin_username: "admin"
  admin_password: "change-me-on-first-run"

database:
  type: sqlite  # sqlite | mysql
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
  secret: ""        # 本期不使用，保留供未来 CSRF / 签名 Session 使用
  ttl: "24h"        # Session 有效期；每次访问自动续期
  secure: false     # 生产环境 HTTPS 部署改为 true，强制 Cookie 仅 HTTPS 传输
```

- [ ] **步骤 3：构建验证**

运行：`cd server && go build ./...`
预期：编译成功。

- [ ] **步骤 4：Commit**

```bash
git add server/internal/config/config.go server/config.yaml.example
git commit -m "feat(config): add session.secure flag and document admin bootstrap"
```

---

## 任务 8：Server 启动初始化（main.go）

**文件：**
- 修改：`server/cmd/server/main.go`

- [ ] **步骤 1：替换 main.go**

将 `server/cmd/server/main.go` 整体替换为：

```go
package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"ssl-tracker/server/internal/api"
	"ssl-tracker/server/internal/auth"
	"ssl-tracker/server/internal/config"
	"ssl-tracker/server/internal/store"
	"ssl-tracker/server/internal/web"
)

const defaultSessionTTL = 24 * time.Hour

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	var db *gorm.DB
	switch cfg.Database.Type {
	case "sqlite":
		if err := os.MkdirAll("data", 0755); err != nil {
			log.Fatalf("Failed to create data dir: %v", err)
		}
		db, err = gorm.Open(sqlite.Open(cfg.Database.SQLite.Path), &gorm.Config{})
	case "mysql":
		db, err = gorm.Open(mysql.Open(cfg.Database.MySQL.DSN), &gorm.Config{})
	default:
		log.Fatalf("Unsupported database type: %s", cfg.Database.Type)
	}
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}

	if err := db.AutoMigrate(
		&store.Agent{},
		&store.Domain{},
		&store.AgentDomainOverride{},
		&store.CheckResult{},
		&store.User{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	s := store.NewStore(db)

	if err := bootstrapAdminUser(s, cfg); err != nil {
		log.Fatalf("Failed to bootstrap admin user: %v", err)
	}

	ttl := defaultSessionTTL
	if cfg.Session.TTL != "" {
		parsed, err := time.ParseDuration(cfg.Session.TTL)
		if err != nil {
			log.Fatalf("Invalid session.ttl %q: %v", cfg.Session.TTL, err)
		}
		ttl = parsed
	}
	sessions := auth.NewSessionStore(ttl)
	go runSessionCleanup(sessions)

	r := api.SetupRouter(s, cfg.Auth.AgentToken, cfg.Alert.ExpireThresholdDays,
		sessions, cfg.Session.Secure, web.Handler())

	log.Printf("Server starting on %s", cfg.Server.Listen)
	if err := r.Run(cfg.Server.Listen); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func bootstrapAdminUser(s *store.Store, cfg *config.Config) error {
	count, err := s.CountUsers()
	if err != nil {
		return err
	}
	if count > 0 {
		log.Printf("users table has %d users, skipping initial admin creation", count)
		return nil
	}
	if cfg.Auth.AdminUsername == "" || cfg.Auth.AdminPassword == "" {
		log.Fatal("users table is empty and auth.admin_username/admin_password is not configured")
	}
	hash, err := auth.HashPassword(cfg.Auth.AdminPassword)
	if err != nil {
		return err
	}
	if err := s.CreateUser(&store.User{Username: cfg.Auth.AdminUsername, PasswordHash: hash}); err != nil {
		return err
	}
	log.Printf("created initial admin user: %s", cfg.Auth.AdminUsername)
	return nil
}

func runSessionCleanup(sessions *auth.SessionStore) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		sessions.Cleanup()
	}
}
```

- [ ] **步骤 2：构建验证**

运行：`cd server && go build ./cmd/server`
预期：编译成功。

- [ ] **步骤 3：完整测试**

运行：`cd server && go test ./...`
预期：所有 server 测试 PASS。

- [ ] **步骤 4：Commit**

```bash
git add server/cmd/server/main.go
git commit -m "feat(server): bootstrap admin user, init SessionStore and cleanup goroutine"
```

---

## 任务 9：前端 useAuth composable + types + api

**文件：**
- 修改：`web/src/types.ts`
- 修改：`web/src/api.ts`
- 创建：`web/src/composables/useAuth.ts`

- [ ] **步骤 1：在 types.ts 追加 User 接口**

修改 `web/src/types.ts`，在文件末尾追加：

```ts
export interface User {
  id: number
  username: string
}
```

- [ ] **步骤 2：重写 api.ts，加入凭证和 authApi**

将 `web/src/api.ts` 整体替换为：

```ts
import type { Overview, DomainsResponse, DomainDetail, User } from './types'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
    ...init,
  })
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`
    try {
      const body = await res.json()
      if (body?.error?.message) message = body.error.message
    } catch {
      // ignore parse errors, fall back to status text
    }
    throw new Error(message)
  }
  return res.json()
}

export const api = {
  overview: () => request<Overview>('/api/dashboard/overview'),
  domains: () => request<DomainsResponse>('/api/dashboard/domains'),
  domainDetail: (id: number | string) =>
    request<DomainDetail>(`/api/dashboard/domains/${id}`),
}

export const authApi = {
  login: (username: string, password: string) =>
    request<{ user: User }>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  logout: () =>
    request<{ ok: boolean }>('/api/auth/logout', { method: 'POST' }),
  me: () => request<{ user: User }>('/api/auth/me'),
}
```

- [ ] **步骤 3：创建 useAuth composable**

创建 `web/src/composables/useAuth.ts`：

```ts
import { ref } from 'vue'
import { authApi } from '../api'
import type { User } from '../types'

const user = ref<User | null>(null)
const loading = ref(true)
const initialized = ref(false)

async function fetchMe(): Promise<void> {
  loading.value = true
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

- [ ] **步骤 4：构建验证**

运行：`cd web && npm run build`
预期：构建成功，无类型错误。

- [ ] **步骤 5：Commit**

```bash
git add web/src/types.ts web/src/api.ts web/src/composables/useAuth.ts
git commit -m "feat(web): add useAuth composable, authApi, credentials handling"
```

---

## 任务 10：Login 页面 + Header 组件 + 路由集成

**文件：**
- 创建：`web/src/components/Header.vue`
- 创建：`web/src/views/Login.vue`
- 修改：`web/src/App.vue`
- 修改：`web/src/router.ts`

- [ ] **步骤 1：创建 Header.vue**

创建 `web/src/components/Header.vue`：

```vue
<script setup lang="ts">
import { RouterLink, useRouter } from 'vue-router'
import { ShieldCheck, ChevronDown } from 'lucide-vue-next'
import { useAuth } from '../composables/useAuth'

const { user, loading, logout } = useAuth()
const router = useRouter()

async function handleLogout() {
  try {
    await logout()
  } finally {
    router.push('/')
  }
}
</script>

<template>
  <header class="bg-bg border-b border-border-soft">
    <div class="max-w-6xl mx-auto px-6 py-4 flex items-center gap-3">
      <RouterLink to="/" class="flex items-center gap-2 text-ink font-semibold text-lg">
        <ShieldCheck :size="22" class="text-accent" />
        SSL Tracker
      </RouterLink>
      <div class="flex-1" />
      <template v-if="!loading">
        <RouterLink
          v-if="!user"
          to="/login"
          class="text-ink-soft hover:text-ink text-sm font-medium px-3 py-1.5 rounded-md hover:bg-bg-subtle transition"
        >
          登录
        </RouterLink>
        <details v-else class="relative">
          <summary class="list-none cursor-pointer flex items-center gap-1 text-ink text-sm font-medium px-3 py-1.5 rounded-md hover:bg-bg-subtle transition">
            {{ user.username }}
            <ChevronDown :size="14" />
          </summary>
          <div class="absolute right-0 mt-1 w-32 bg-bg border border-border-soft rounded-md shadow-md py-1 z-10">
            <button
              type="button"
              class="w-full text-left px-3 py-1.5 text-sm text-ink hover:bg-bg-subtle"
              @click="handleLogout"
            >
              登出
            </button>
          </div>
        </details>
      </template>
    </div>
  </header>
</template>
```

- [ ] **步骤 2：创建 Login.vue**

创建 `web/src/views/Login.vue`：

```vue
<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '../composables/useAuth'

const username = ref('')
const password = ref('')
const error = ref('')
const submitting = ref(false)

const { user, initialized, fetchMe, login } = useAuth()
const route = useRoute()
const router = useRouter()

function redirectTarget(): string {
  const r = route.query.redirect
  return typeof r === 'string' && r.startsWith('/') ? r : '/'
}

async function ensureInitialized() {
  if (!initialized.value) await fetchMe()
}

async function maybeRedirect() {
  await ensureInitialized()
  if (user.value) {
    router.replace(redirectTarget())
  }
}

onMounted(maybeRedirect)
watch(user, (v) => {
  if (v) router.replace(redirectTarget())
})

async function submit() {
  if (submitting.value) return
  error.value = ''
  submitting.value = true
  try {
    await login(username.value, password.value)
    // watcher above will redirect once user is set
  } catch (e: any) {
    error.value = e?.message || '登录失败'
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="max-w-sm mx-auto mt-16">
    <h1 class="text-2xl font-semibold text-ink mb-6">管理员登录</h1>
    <form class="space-y-4" @submit.prevent="submit">
      <div>
        <label class="block text-sm text-ink-soft mb-1">用户名</label>
        <input
          v-model="username"
          type="text"
          autocomplete="username"
          required
          class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink focus:outline-none focus:border-accent"
        />
      </div>
      <div>
        <label class="block text-sm text-ink-soft mb-1">密码</label>
        <input
          v-model="password"
          type="password"
          autocomplete="current-password"
          required
          class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink focus:outline-none focus:border-accent"
        />
      </div>
      <p v-if="error" class="text-sm text-bad">{{ error }}</p>
      <button
        type="submit"
        :disabled="submitting"
        class="w-full bg-accent text-white py-2 rounded-md font-medium hover:opacity-90 disabled:opacity-50 transition"
      >
        {{ submitting ? '登录中…' : '登录' }}
      </button>
    </form>
  </div>
</template>
```

- [ ] **步骤 3：改造 App.vue**

将 `web/src/App.vue` 整体替换为：

```vue
<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterView } from 'vue-router'
import Header from './components/Header.vue'
import { useAuth } from './composables/useAuth'

const { fetchMe } = useAuth()

onMounted(() => {
  // Initialize auth state at app startup; tolerates failure (treated as logged-out).
  fetchMe()
})
</script>

<template>
  <div class="min-h-screen">
    <Header />
    <main class="max-w-6xl mx-auto px-6 py-8">
      <RouterView />
    </main>
  </div>
</template>
```

- [ ] **步骤 4：更新 router.ts（加 /login + beforeEach 守卫）**

将 `web/src/router.ts` 整体替换为：

```ts
import { createRouter, createWebHistory } from 'vue-router'
import Overview from './views/Overview.vue'
import DomainDetail from './views/DomainDetail.vue'
import Login from './views/Login.vue'
import { useAuth } from './composables/useAuth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: Overview },
    { path: '/domains/:id', component: DomainDetail, props: true },
    { path: '/login', component: Login },
  ],
})

router.beforeEach(async (to) => {
  if (!to.meta.requiresAuth) return
  const { user, initialized, fetchMe } = useAuth()
  if (!initialized.value) await fetchMe()
  if (!user.value) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
})

export default router
```

- [ ] **步骤 5：构建验证**

运行：`cd web && npm run build`
预期：构建成功，无类型错误。

- [ ] **步骤 6：Commit**

```bash
git add web/src/components/Header.vue web/src/views/Login.vue web/src/App.vue web/src/router.ts
git commit -m "feat(web): add Login page, Header with login menu, route guards"
```

---

## 任务 11：端到端冒烟验证

**文件：** 无新增（仅运行+清理）

本任务用真实 Server + 浏览器确认所有部件协同工作。

- [ ] **步骤 1：构建产物**

运行：
```bash
cd web && npm run build
```

```bash
cd server && go build -o server ./cmd/server
```

预期：均成功。

- [ ] **步骤 2：写测试配置**

创建 `server/config.local.yaml`（已被 .gitignore 忽略 `*.local.yaml`）：

```yaml
server:
  listen: ":8080"

auth:
  agent_token: "plan31-token"
  admin_username: "admin"
  admin_password: "plan31-pass"

database:
  type: sqlite
  sqlite:
    path: "./data/plan31.db"

retention:
  history_days: 7

alert:
  expire_threshold_days: 15
  daily_reminder_time: "09:00"
  daily_reminder_timezone: "Asia/Shanghai"

session:
  secret: ""
  ttl: "24h"
  secure: false
```

- [ ] **步骤 3：清理旧 DB 并启动 Server**

清理旧 DB（如果存在）：

```bash
# Linux/macOS
rm -f server/data/plan31.db

# Windows PowerShell
Remove-Item -Force server/data/plan31.db -ErrorAction SilentlyContinue
```

启动 Server：

```bash
cd server
./server -config config.local.yaml
```

预期日志包含：
- `created initial admin user: admin`
- `Server starting on :8080`

- [ ] **步骤 4：用 curl 跑后端冒烟**

在另一个终端运行（cookies 文件用相对路径，跨平台）：

```bash
# 1. 错误密码 → 401
curl -i -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"WRONG"}'

# 2. 正确密码 → 200 + Set-Cookie
curl -i -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"plan31-pass"}' \
  -c cookies.txt

# 3. 用 cookie 调 me → 200 + 返回 user
curl -i -b cookies.txt http://localhost:8080/api/auth/me

# 4. 用 cookie 调 admin → 200
curl -i -b cookies.txt http://localhost:8080/api/admin/domains

# 5. 不带 cookie → 401
curl -i http://localhost:8080/api/admin/domains

# 6. 登出 → 200 + Cookie 清除
curl -i -X POST -b cookies.txt http://localhost:8080/api/auth/logout

# 7. 登出后 me → 401
curl -i -b cookies.txt http://localhost:8080/api/auth/me
```

预期：所有响应符合上面注释中的状态码。Windows PowerShell 用户用 `Invoke-WebRequest -SessionVariable s` 或安装 curl.exe 等价方式。

- [ ] **步骤 5：浏览器冒烟**

打开 `http://localhost:8080/`，依次确认：

1. Header 右侧出现"登录"按钮
2. 点击"登录" → 跳转 `/login`，看到登录表单
3. 用错误密码提交 → 看到红色错误提示，不跳转
4. 用正确密码（admin / plan31-pass）提交 → 跳回 `/`，Header 右侧变为 `admin ▼`
5. 点击 `admin ▼` 展开 → 看到"登出"按钮
6. 点击"登出" → 退回未登录状态，Header 显示"登录"
7. 直接访问 `/login` 时已登录 → 自动跳回 `/`
8. 访问 `/login?redirect=/domains/1` → 登录后跳到 `/domains/1`（如果库里没该域名会显示空详情页，仍可验证跳转）

- [ ] **步骤 6：重启 Server，验证 Session 失效**

停掉 server (Ctrl-C) → 重新启动：

```bash
./server -config config.local.yaml
```

预期日志：
- `users table has 1 users, skipping initial admin creation`（不再创建管理员）

刷新浏览器 → 之前的登录失效，Header 重新显示"登录"按钮（Session 在内存清空，符合设计）。

- [ ] **步骤 7：清理**

```bash
# 停掉 server (Ctrl-C)
rm -f cookies.txt
rm -f server/data/plan31.db
```

`config.local.yaml` 由 .gitignore 忽略，可保留也可删除，不影响仓库。

- [ ] **步骤 8：标记 Plan 3.1 完成（无新文件 commit）**

本任务无代码变更。如需占位 commit：

```bash
git commit --allow-empty -m "chore: Plan 3.1 end-to-end smoke test passed"
```

或直接跳过这一步。

---

## 自检备忘

- 规格 §3.2 的初始管理员创建：任务 8 `bootstrapAdminUser`
- 规格 §4.1 / §4.2 / §4.3 三个端点：任务 5
- 规格 §4.4 受保护路由：任务 6
- 规格 §5 SessionStore 全部 API + 清理 goroutine：任务 3 + 任务 8
- 规格 §6 Cookie 安全配置：任务 5 `setSessionCookie` + 任务 7 `Secure` 字段
- 规格 §7.1 useAuth：任务 9
- 规格 §7.2 App.vue：任务 10
- 规格 §7.3 Header：任务 10
- 规格 §7.4 Login.vue：任务 10
- 规格 §7.5 路由守卫：任务 10
- 规格 §7.6 api.ts：任务 9
- 规格 §8 配置：任务 7
- 规格 §9.1 测试覆盖：每个对应任务
- 规格 §9.2 端到端冒烟：任务 11

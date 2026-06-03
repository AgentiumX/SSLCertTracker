# SSL 证书监控系统 - Frontend Dashboard 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 实现公开 Dashboard（首页 + 域名详情，免登录），Vue 前端 embed 进 Server 二进制，访问 `/` 即可看到所有域名的最新 SSL 检测状态。

**架构：** Server 新增 3 个 `/api/dashboard/*` 端点（无认证），向 Store 加最新结果聚合查询。Vue 3 SPA 通过 Vite 构建到 `server/web/dist/`，由 `embed.FS` 打进 server 二进制；HTML5 history 路由回退到 `index.html`。

**技术栈：** Vue 3 + Vite + TypeScript + Tailwind CSS（v3）+ vue-router + lucide-vue-next。**不引入 shadcn-vue / Pinia**——本期仅 2 页只读视图，状态由 Vue `ref` 直接管理，引入这两个库属于过度工程，留到 Plan 3 admin 后台再加。

---

## 文件结构

### Server 端新增

**Store 聚合查询**
- 修改 `server/internal/store/store.go` - 新增 `LatestResults`/`LatestResultsForDomain`/`CountAgentsOnline` 等聚合方法
- 测试 `server/internal/store/store_aggregate_test.go`

**Dashboard API**
- 创建 `server/internal/api/dashboard.go` - 3 个 handler：overview / domains / domain detail
- 测试 `server/internal/api/dashboard_test.go`

**前端 embed**
- 创建 `server/internal/web/web.go` - `//go:embed dist` 包装为 `http.FileSystem`，处理 SPA 回退
- 修改 `server/internal/api/router.go` - 注册 `/api/dashboard/*` 和静态文件路由（fallback 到 `index.html`）
- 修改 `server/cmd/server/main.go` - 仅注入 `web.Handler`，无逻辑变更

### Vue 项目（新增 monorepo 子目录）

**项目根**
- 创建 `web/package.json`
- 创建 `web/vite.config.ts`
- 创建 `web/tsconfig.json`
- 创建 `web/tsconfig.node.json`
- 创建 `web/tailwind.config.ts`
- 创建 `web/postcss.config.js`
- 创建 `web/index.html`
- 创建 `web/.gitignore`
- 创建 `web/env.d.ts`

**源码**
- 创建 `web/src/main.ts` - Vue app 入口
- 创建 `web/src/App.vue` - 顶部导航 + `<router-view>`
- 创建 `web/src/router.ts` - 2 路由：`/` 和 `/domains/:id`
- 创建 `web/src/style.css` - Tailwind directives + 苹果风 CSS 变量
- 创建 `web/src/api.ts` - fetch 包装 + 响应类型
- 创建 `web/src/types.ts` - 后端响应的 TypeScript 类型
- 创建 `web/src/components/StatCard.vue` - 概览页大数字卡片
- 创建 `web/src/components/StatusDot.vue` - 状态小圆点
- 创建 `web/src/views/Overview.vue` - 首页：4 卡 + 域名列表
- 创建 `web/src/views/DomainDetail.vue` - 详情页：Agent × 检测项矩阵

### 构建集成

**项目根**
- 修改 `Makefile` - 新增 `build-web` 目标，串联到 `build-server`
- 修改 `.gitignore` - 忽略 `web/node_modules`、`web/dist`、`server/internal/web/dist`
- 创建 `server/internal/web/dist/.gitkeep` - 占位，确保 `embed` 在前端没构建时也能编译（包含说明文件）

---

## 任务 1：Store 聚合查询

**文件：**
- 修改：`server/internal/store/store.go` - 追加方法
- 测试：`server/internal/store/store_aggregate_test.go`

- [ ] **步骤 1：编写聚合查询测试**

```go
// server/internal/store/store_aggregate_test.go
package store

import (
	"testing"
	"time"
)

func TestLatestResults_PerAgentDomain(t *testing.T) {
	s := setupTestDB(t)
	now := time.Now()
	s.SaveCheckResults([]CheckResult{
		{AgentID: "a1", DomainID: 1, CheckedAt: now.Add(-2 * time.Hour), Status: "ok"},
		{AgentID: "a1", DomainID: 1, CheckedAt: now.Add(-1 * time.Hour), Status: "expiring"}, // newer
		{AgentID: "a1", DomainID: 2, CheckedAt: now, Status: "ok"},
		{AgentID: "a2", DomainID: 1, CheckedAt: now, Status: "expired"},
	})

	results, err := s.LatestResults()
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 latest rows (a1/1, a1/2, a2/1), got %d", len(results))
	}
	// a1/1 should be the "expiring" one (newer)
	for _, r := range results {
		if r.AgentID == "a1" && r.DomainID == 1 && r.Status != "expiring" {
			t.Errorf("a1/1 expected expiring, got %s", r.Status)
		}
	}
}

func TestLatestResultsForDomain(t *testing.T) {
	s := setupTestDB(t)
	now := time.Now()
	s.SaveCheckResults([]CheckResult{
		{AgentID: "a1", DomainID: 5, CheckedAt: now.Add(-1 * time.Hour), Status: "ok"},
		{AgentID: "a1", DomainID: 5, CheckedAt: now, Status: "expiring"},
		{AgentID: "a2", DomainID: 5, CheckedAt: now, Status: "ok"},
		{AgentID: "a1", DomainID: 6, CheckedAt: now, Status: "ok"}, // different domain, ignored
	})

	results, err := s.LatestResultsForDomain(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 rows (one per agent), got %d", len(results))
	}
}

func TestCountAgentsOnline(t *testing.T) {
	s := setupTestDB(t)
	now := time.Now()
	s.CreateAgent(&Agent{AgentID: "online", DisplayName: "x", RegisteredAt: now, LastSeenAt: now.Add(-1 * time.Hour)})
	s.CreateAgent(&Agent{AgentID: "offline", DisplayName: "y", RegisteredAt: now, LastSeenAt: now.Add(-5 * time.Hour)})

	online, total, err := s.CountAgents(3 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("expected 2 total, got %d", total)
	}
	if online != 1 {
		t.Errorf("expected 1 online, got %d", online)
	}
}

func TestListAgents(t *testing.T) {
	s := setupTestDB(t)
	now := time.Now()
	s.CreateAgent(&Agent{AgentID: "a1", DisplayName: "Beijing", Hostname: "h1", IP: "1.1.1.1", RegisteredAt: now, LastSeenAt: now})
	s.CreateAgent(&Agent{AgentID: "a2", DisplayName: "Shanghai", Hostname: "h2", IP: "2.2.2.2", RegisteredAt: now, LastSeenAt: now})

	agents, err := s.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

```bash
cd server && go test ./internal/store -run "Latest|CountAgents|ListAgents" -v
```

预期：FAIL，`undefined: LatestResults`, `undefined: LatestResultsForDomain`, `undefined: CountAgents`, `undefined: ListAgents`

- [ ] **步骤 3：实现聚合查询方法**

追加到 `server/internal/store/store.go` 末尾：

```go
// LatestResults returns the most recent CheckResult per (agent_id, domain_id).
// Implemented in Go to avoid SQL dialect differences between SQLite and MySQL.
func (s *Store) LatestResults() ([]CheckResult, error) {
	var all []CheckResult
	if err := s.db.Order("checked_at DESC").Find(&all).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	out := make([]CheckResult, 0, len(all))
	for _, r := range all {
		key := r.AgentID + "|" + fmt.Sprint(r.DomainID)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, r)
	}
	return out, nil
}

// LatestResultsForDomain returns the most recent CheckResult per agent for a single domain.
func (s *Store) LatestResultsForDomain(domainID uint) ([]CheckResult, error) {
	var all []CheckResult
	if err := s.db.Where("domain_id = ?", domainID).Order("checked_at DESC").Find(&all).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	out := make([]CheckResult, 0, len(all))
	for _, r := range all {
		if seen[r.AgentID] {
			continue
		}
		seen[r.AgentID] = true
		out = append(out, r)
	}
	return out, nil
}

// CountAgents returns (online, total). Online means LastSeenAt within onlineWindow.
func (s *Store) CountAgents(onlineWindow time.Duration) (online, total int64, err error) {
	if err = s.db.Model(&Agent{}).Count(&total).Error; err != nil {
		return 0, 0, err
	}
	threshold := time.Now().Add(-onlineWindow)
	if err = s.db.Model(&Agent{}).Where("last_seen_at >= ?", threshold).Count(&online).Error; err != nil {
		return 0, 0, err
	}
	return online, total, nil
}

// ListAgents returns all agents.
func (s *Store) ListAgents() ([]Agent, error) {
	var agents []Agent
	err := s.db.Find(&agents).Error
	return agents, err
}
```

新增 import 在 `store.go` 顶部：`"fmt"`（如尚未导入）。

- [ ] **步骤 4：运行测试验证通过**

```bash
cd server && go test ./internal/store -v
```

预期：PASS（包含原有测试 + 4 个新测试）

- [ ] **步骤 5：Commit**

```bash
git add server/internal/store/
git commit -m "feat(server): add aggregate queries for dashboard (latest results, agent counts)"
```

---

## 任务 2：Dashboard API handlers

**文件：**
- 创建：`server/internal/api/dashboard.go`
- 测试：`server/internal/api/dashboard_test.go`

- [ ] **步骤 1：编写 Dashboard API 测试**

```go
// server/internal/api/dashboard_test.go
package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"ssl-tracker/server/internal/store"
)

func setupDashboardAPI(t *testing.T) (*gin.Engine, *store.Store) {
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "dash.db")
	db, _ := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	t.Cleanup(func() { sqlDB, _ := db.DB(); sqlDB.Close() })
	db.AutoMigrate(&store.Agent{}, &store.Domain{}, &store.AgentDomainOverride{}, &store.CheckResult{})
	s := store.NewStore(db)

	r := gin.New()
	h := NewDashboardHandler(s, 3*time.Hour)
	r.GET("/api/dashboard/overview", h.Overview)
	r.GET("/api/dashboard/domains", h.Domains)
	r.GET("/api/dashboard/domains/:id", h.DomainDetail)
	return r, s
}

func TestDashboardOverview(t *testing.T) {
	r, s := setupDashboardAPI(t)
	now := time.Now()
	s.CreateAgent(&store.Agent{AgentID: "a1", DisplayName: "A1", RegisteredAt: now, LastSeenAt: now})
	s.CreateAgent(&store.Agent{AgentID: "a2", DisplayName: "A2", RegisteredAt: now, LastSeenAt: now.Add(-5 * time.Hour)})
	s.CreateDomain(&store.Domain{Host: "ok.com", Port: 443, Protocol: "https", IsGlobal: true})
	s.CreateDomain(&store.Domain{Host: "bad.com", Port: 443, Protocol: "https", IsGlobal: true})
	s.SaveCheckResults([]store.CheckResult{
		{AgentID: "a1", DomainID: 1, CheckedAt: now, Status: "ok"},
		{AgentID: "a1", DomainID: 2, CheckedAt: now, Status: "expired"},
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/dashboard/overview", nil))
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	var resp struct {
		TotalDomains   int `json:"total_domains"`
		HealthyDomains int `json:"healthy_domains"`
		AlertDomains   int `json:"alert_domains"`
		AgentsOnline   int `json:"agents_online"`
		AgentsTotal    int `json:"agents_total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.TotalDomains != 2 {
		t.Errorf("TotalDomains: want 2, got %d", resp.TotalDomains)
	}
	if resp.HealthyDomains != 1 {
		t.Errorf("HealthyDomains: want 1, got %d", resp.HealthyDomains)
	}
	if resp.AlertDomains != 1 {
		t.Errorf("AlertDomains: want 1, got %d", resp.AlertDomains)
	}
	if resp.AgentsOnline != 1 || resp.AgentsTotal != 2 {
		t.Errorf("agents: want 1/2, got %d/%d", resp.AgentsOnline, resp.AgentsTotal)
	}
}

func TestDashboardDomainsList(t *testing.T) {
	r, s := setupDashboardAPI(t)
	now := time.Now()
	s.CreateAgent(&store.Agent{AgentID: "a1", DisplayName: "A1", RegisteredAt: now, LastSeenAt: now})
	s.CreateAgent(&store.Agent{AgentID: "a2", DisplayName: "A2", RegisteredAt: now, LastSeenAt: now})
	s.CreateDomain(&store.Domain{Host: "x.com", Port: 443, Protocol: "https", IsGlobal: true})
	s.SaveCheckResults([]store.CheckResult{
		{AgentID: "a1", DomainID: 1, CheckedAt: now, Status: "ok"},
		{AgentID: "a2", DomainID: 1, CheckedAt: now, Status: "expired"},
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/dashboard/domains", nil))
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	var resp struct {
		Domains []struct {
			ID            uint   `json:"id"`
			Host          string `json:"host"`
			Port          int    `json:"port"`
			HealthyCount  int    `json:"healthy_count"`
			TotalChecks   int    `json:"total_checks"`
			WorstStatus   string `json:"worst_status"`
		} `json:"domains"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(resp.Domains))
	}
	d := resp.Domains[0]
	if d.HealthyCount != 1 || d.TotalChecks != 2 {
		t.Errorf("counts: want 1/2, got %d/%d", d.HealthyCount, d.TotalChecks)
	}
	if d.WorstStatus != "expired" {
		t.Errorf("WorstStatus: want expired, got %s", d.WorstStatus)
	}
}

func TestDashboardDomainDetail(t *testing.T) {
	r, s := setupDashboardAPI(t)
	now := time.Now()
	notAfter := now.Add(30 * 24 * time.Hour)
	s.CreateAgent(&store.Agent{AgentID: "a1", DisplayName: "Beijing", RegisteredAt: now, LastSeenAt: now})
	s.CreateDomain(&store.Domain{Host: "d.com", Port: 443, Protocol: "https", IsGlobal: true})
	s.SaveCheckResults([]store.CheckResult{
		{AgentID: "a1", DomainID: 1, CheckedAt: now, Status: "ok", NotAfter: &notAfter, Issuer: "LE"},
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/dashboard/domains/1", nil))
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	var resp struct {
		Domain  map[string]interface{}   `json:"domain"`
		Results []map[string]interface{} `json:"results"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Domain["host"] != "d.com" {
		t.Errorf("host mismatch: %v", resp.Domain["host"])
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.Results[0]["agent_display_name"] != "Beijing" {
		t.Errorf("agent_display_name not joined: %v", resp.Results[0])
	}
}

func TestDashboardDomainDetail_NotFound(t *testing.T) {
	r, _ := setupDashboardAPI(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/dashboard/domains/9999", nil))
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

```bash
cd server && go test ./internal/api -run Dashboard -v
```

预期：FAIL，`undefined: NewDashboardHandler`

- [ ] **步骤 3：实现 Dashboard handlers**

```go
// server/internal/api/dashboard.go
package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"ssl-tracker/server/internal/store"
)

type DashboardHandler struct {
	store         *store.Store
	onlineWindow  time.Duration
}

func NewDashboardHandler(s *store.Store, onlineWindow time.Duration) *DashboardHandler {
	return &DashboardHandler{store: s, onlineWindow: onlineWindow}
}

// statusRank: lower is healthier. Used to pick the worst status across agents.
var statusRank = map[string]int{
	"ok":          0,
	"expiring":    1,
	"mismatch":    2,
	"expired":     3,
	"unreachable": 4,
}

func worstStatus(a, b string) string {
	if statusRank[b] > statusRank[a] {
		return b
	}
	return a
}

func (h *DashboardHandler) Overview(c *gin.Context) {
	domains, err := h.store.ListAllDomains()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	results, err := h.store.LatestResults()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	online, total, err := h.store.CountAgents(h.onlineWindow)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}

	// Domain is "alert" if ANY latest result is non-ok. "Healthy" if at least one
	// result and all are ok. Domains with zero results count as neither.
	domainStatus := make(map[uint]string)
	for _, r := range results {
		domainStatus[r.DomainID] = worstStatus(domainStatus[r.DomainID], r.Status)
	}
	healthy, alert := 0, 0
	for _, d := range domains {
		s, ok := domainStatus[d.ID]
		if !ok {
			continue
		}
		if s == "ok" {
			healthy++
		} else {
			alert++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_domains":   len(domains),
		"healthy_domains": healthy,
		"alert_domains":   alert,
		"agents_online":   online,
		"agents_total":    total,
	})
}

func (h *DashboardHandler) Domains(c *gin.Context) {
	domains, err := h.store.ListAllDomains()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	results, err := h.store.LatestResults()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	type stats struct {
		healthy int
		total   int
		worst   string
	}
	per := make(map[uint]*stats)
	for _, r := range results {
		st := per[r.DomainID]
		if st == nil {
			st = &stats{}
			per[r.DomainID] = st
		}
		st.total++
		if r.Status == "ok" {
			st.healthy++
		}
		st.worst = worstStatus(st.worst, r.Status)
	}

	out := make([]gin.H, 0, len(domains))
	for _, d := range domains {
		st := per[d.ID]
		row := gin.H{
			"id":             d.ID,
			"host":           d.Host,
			"port":           d.Port,
			"protocol":       d.Protocol,
			"is_global":      d.IsGlobal,
			"remark":         d.Remark,
			"healthy_count":  0,
			"total_checks":   0,
			"worst_status":   "",
		}
		if st != nil {
			row["healthy_count"] = st.healthy
			row["total_checks"] = st.total
			row["worst_status"] = st.worst
		}
		out = append(out, row)
	}
	c.JSON(http.StatusOK, gin.H{"domains": out})
}

func (h *DashboardHandler) DomainDetail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_id", "message": "invalid domain ID"}})
		return
	}
	domain, err := h.store.GetDomain(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "domain not found"}})
		return
	}
	results, err := h.store.LatestResultsForDomain(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	// Join agent display names. Build agent map once.
	agents, err := h.store.ListAgents()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	agentMap := make(map[string]store.Agent, len(agents))
	for _, a := range agents {
		agentMap[a.AgentID] = a
	}

	threshold := time.Now().Add(-h.onlineWindow)
	out := make([]gin.H, 0, len(results))
	for _, r := range results {
		a := agentMap[r.AgentID]
		out = append(out, gin.H{
			"agent_id":           r.AgentID,
			"agent_display_name": a.DisplayName,
			"agent_online":       !a.LastSeenAt.IsZero() && a.LastSeenAt.After(threshold),
			"checked_at":         r.CheckedAt,
			"status":             r.Status,
			"not_after":          r.NotAfter,
			"issuer":             r.Issuer,
			"subject":            r.Subject,
			"sans":               r.SANs,
			"error_message":      r.ErrorMessage,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"domain": gin.H{
			"id":       domain.ID,
			"host":     domain.Host,
			"port":     domain.Port,
			"protocol": domain.Protocol,
			"remark":   domain.Remark,
		},
		"results": out,
	})
}
```

- [ ] **步骤 4：运行测试验证通过**

```bash
cd server && go test ./internal/api -run Dashboard -v
```

预期：PASS（4 个新测试）

- [ ] **步骤 5：Commit**

```bash
git add server/internal/api/dashboard.go server/internal/api/dashboard_test.go
git commit -m "feat(server): add Dashboard API (overview, domains list, domain detail)"
```

---

## 任务 3：Embed 前端 + SPA 路由回退

**文件：**
- 创建：`server/internal/web/web.go`
- 创建：`server/internal/web/dist/.gitkeep`
- 创建：`server/internal/web/dist/index.html`（占位，真实文件由 vite build 覆盖）
- 修改：`server/internal/api/router.go`
- 修改：`server/cmd/server/main.go`

> **说明：** `embed` 要求 embed 路径在编译时存在且非空。我们提交一个最小占位 `index.html`，前端真正构建产物会覆盖它。`dist` 目录由 Makefile 在 `build-server` 前清空+生成。

- [ ] **步骤 1：创建占位 index.html**

```html
<!-- server/internal/web/dist/index.html -->
<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>SSL Tracker</title></head>
<body><p>Frontend not built. Run <code>make build-web</code> first.</p></body></html>
```

- [ ] **步骤 2：创建 .gitkeep**

```bash
touch server/internal/web/dist/.gitkeep
```

- [ ] **步骤 3：实现 web.go**

```go
// server/internal/web/web.go
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist
var distFS embed.FS

// Handler returns an http.Handler that serves the embedded SPA.
// Unknown paths (excluding /api/*) fall back to index.html so vue-router
// HTML5 mode works on direct URL access / refresh.
func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			r.URL.Path = "/index.html"
			fileServer.ServeHTTP(w, r)
			return
		}
		if _, err := fs.Stat(sub, path); err != nil {
			// Not a real file → SPA fallback to index.html
			r.URL.Path = "/index.html"
			fileServer.ServeHTTP(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
```

- [ ] **步骤 4：注册到 Router**

修改 `server/internal/api/router.go`，新增 `webHandler` 参数并挂载非 API 路径：

```go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"ssl-tracker/server/internal/auth"
	"ssl-tracker/server/internal/store"
	"time"
)

func SetupRouter(s *store.Store, agentToken string, expireThresholdDays int, webHandler http.Handler) *gin.Engine {
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

	adminGroup := r.Group("/api/admin")
	{
		h := NewAdminHandler(s)
		adminGroup.POST("/domains", h.CreateDomain)
		adminGroup.GET("/domains", h.ListDomains)
		adminGroup.GET("/domains/:id", h.GetDomain)
		adminGroup.DELETE("/domains/:id", h.DeleteDomain)
	}

	// Static + SPA fallback for all non-API paths.
	if webHandler != nil {
		r.NoRoute(gin.WrapH(webHandler))
	}
	return r
}
```

- [ ] **步骤 5：更新 main.go 注入 webHandler**

修改 `server/cmd/server/main.go` 的最后几行：

```go
	s := store.NewStore(db)
	r := api.SetupRouter(s, cfg.Auth.AgentToken, cfg.Alert.ExpireThresholdDays, web.Handler())
```

并新增 import：`"ssl-tracker/server/internal/web"`。

- [ ] **步骤 6：修改现有 api_test.go 适配新签名**

`server/internal/api/api_test.go` 里 `setupTestAPI` 中调用 `SetupRouter` 的地方（如果有）也要传 `nil` 作为 webHandler。当前 `api_test.go` 直接构造 handler，没用 SetupRouter，所以这步无操作；只需检查整个 server 包能编译：

```bash
cd server && go build ./...
```

预期：编译通过。

- [ ] **步骤 7：验证启动后访问 / 返回占位页**

```bash
cd server && go build -o server.exe ./cmd/server
cp config.yaml.example config.yaml.test
sed -i 's|./data/ssl-tracker.db|./data/test-embed.db|' config.yaml.test
mkdir -p data && rm -f data/test-embed.db
./server.exe -config config.yaml.test &
SERVER_PID=$!
sleep 2
curl -s http://localhost:8080/ | head -3
curl -s http://localhost:8080/some/spa/route | head -3
kill $SERVER_PID
rm -f server.exe config.yaml.test
rm -rf data/test-embed.db
```

预期：两次 curl 都返回占位 HTML 中的 `Frontend not built` 文本（验证 SPA 回退）。

- [ ] **步骤 8：Commit**

```bash
git add server/internal/web/ server/internal/api/router.go server/cmd/server/main.go
git commit -m "feat(server): embed Vue dist directory with SPA route fallback"
```

---

## 任务 4：Vue 项目脚手架

**文件：**
- 创建：`web/package.json` 等所有配置文件
- 创建：`web/src/main.ts`、`web/src/App.vue`、`web/src/router.ts`、`web/src/style.css`、`web/index.html`
- 创建：`web/.gitignore`
- 修改：根 `.gitignore`

- [ ] **步骤 1：初始化 web/ 目录与 package.json**

```bash
mkdir -p web/src/views web/src/components
```

```json
// web/package.json
{
  "name": "ssl-tracker-web",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc --noEmit && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "lucide-vue-next": "^0.460.0",
    "vue": "^3.5.13",
    "vue-router": "^4.5.0"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^5.2.1",
    "@types/node": "^22.10.0",
    "autoprefixer": "^10.4.20",
    "postcss": "^8.5.0",
    "tailwindcss": "^3.4.17",
    "typescript": "^5.7.0",
    "vite": "^6.0.5",
    "vue-tsc": "^2.2.0"
  }
}
```

- [ ] **步骤 2：vite.config.ts**

构建产物直接输出到 `server/internal/web/dist/`。

```ts
// web/vite.config.ts
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'node:path'

export default defineConfig({
  plugins: [vue()],
  resolve: { alias: { '@': path.resolve(__dirname, 'src') } },
  build: {
    outDir: path.resolve(__dirname, '../server/internal/web/dist'),
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
```

- [ ] **步骤 3：tsconfig**

```json
// web/tsconfig.json
{
  "compilerOptions": {
    "target": "ES2022",
    "useDefineForClassFields": true,
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "jsx": "preserve",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "esModuleInterop": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "skipLibCheck": true,
    "noEmit": true,
    "baseUrl": ".",
    "paths": { "@/*": ["src/*"] },
    "types": ["vite/client"]
  },
  "include": ["src/**/*", "src/**/*.vue", "env.d.ts"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

```json
// web/tsconfig.node.json
{
  "compilerOptions": {
    "composite": true,
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "skipLibCheck": true,
    "allowSyntheticDefaultImports": true,
    "strict": true,
    "noEmit": true,
    "types": ["node"]
  },
  "include": ["vite.config.ts"]
}
```

- [ ] **步骤 4：env.d.ts**

```ts
// web/env.d.ts
/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}
```

- [ ] **步骤 5：Tailwind 配置**

```ts
// web/tailwind.config.ts
import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{vue,ts}'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['-apple-system', 'BlinkMacSystemFont', '"SF Pro Display"', '"PingFang SC"', 'sans-serif'],
      },
      colors: {
        bg: '#FFFFFF',
        'bg-subtle': '#F5F5F7',
        ink: '#1D1D1F',
        'ink-soft': '#86868B',
        accent: '#0071E3',
        ok: '#34C759',
        warn: '#FF9F0A',
        bad: '#FF3B30',
        'border-soft': '#D2D2D7',
      },
    },
  },
  plugins: [],
} satisfies Config
```

```js
// web/postcss.config.js
export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
}
```

- [ ] **步骤 6：style.css**

```css
/* web/src/style.css */
@tailwind base;
@tailwind components;
@tailwind utilities;

html, body, #app {
  background: theme('colors.bg-subtle');
  color: theme('colors.ink');
  font-family: theme('fontFamily.sans');
  -webkit-font-smoothing: antialiased;
}

body {
  margin: 0;
}

a {
  color: inherit;
  text-decoration: none;
}
```

- [ ] **步骤 7：index.html**

```html
<!-- web/index.html -->
<!DOCTYPE html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>SSL Tracker</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

- [ ] **步骤 8：main.ts + App.vue + router.ts**

```ts
// web/src/main.ts
import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './style.css'

createApp(App).use(router).mount('#app')
```

```vue
<!-- web/src/App.vue -->
<script setup lang="ts">
import { RouterLink, RouterView } from 'vue-router'
import { ShieldCheck } from 'lucide-vue-next'
</script>

<template>
  <div class="min-h-screen">
    <header class="bg-bg border-b border-border-soft">
      <div class="max-w-6xl mx-auto px-6 py-4 flex items-center gap-3">
        <RouterLink to="/" class="flex items-center gap-2 text-ink font-semibold text-lg">
          <ShieldCheck :size="22" class="text-accent" />
          SSL Tracker
        </RouterLink>
      </div>
    </header>
    <main class="max-w-6xl mx-auto px-6 py-8">
      <RouterView />
    </main>
  </div>
</template>
```

```ts
// web/src/router.ts
import { createRouter, createWebHistory } from 'vue-router'
import Overview from './views/Overview.vue'
import DomainDetail from './views/DomainDetail.vue'

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: Overview },
    { path: '/domains/:id', component: DomainDetail, props: true },
  ],
})
```

- [ ] **步骤 9：web/.gitignore + 根 .gitignore**

```gitignore
# web/.gitignore
node_modules
dist
.vite
*.local
```

修改根 `.gitignore` 追加：

```gitignore
# Frontend artifacts
web/node_modules
web/dist
server/internal/web/dist/assets
server/internal/web/dist/index.html
!server/internal/web/dist/.gitkeep
```

> **注意：** placeholder `index.html` 已在任务 3 commit。后续 vite build 会覆盖它，因此 ignore 它+保留 .gitkeep 即可。

- [ ] **步骤 10：安装依赖（人工或 CI）**

```bash
cd web && npm install
```

> 如果 npm 不可用，跳过此步骤；任务 6 build 时再装。

- [ ] **步骤 11：Commit 脚手架**

```bash
git add web/ .gitignore
git commit -m "feat(web): scaffold Vue 3 + Vite + Tailwind project"
```

---

## 任务 5：API 客户端 + 类型定义

**文件：**
- 创建：`web/src/types.ts`
- 创建：`web/src/api.ts`

- [ ] **步骤 1：定义后端响应类型**

```ts
// web/src/types.ts
export type Status = 'ok' | 'expiring' | 'expired' | 'mismatch' | 'unreachable' | ''

export interface Overview {
  total_domains: number
  healthy_domains: number
  alert_domains: number
  agents_online: number
  agents_total: number
}

export interface DomainSummary {
  id: number
  host: string
  port: number
  protocol: string
  is_global: boolean
  remark: string
  healthy_count: number
  total_checks: number
  worst_status: Status
}

export interface DomainsResponse {
  domains: DomainSummary[]
}

export interface AgentResultRow {
  agent_id: string
  agent_display_name: string
  agent_online: boolean
  checked_at: string
  status: Status
  not_after: string | null
  issuer: string
  subject: string
  sans: string
  error_message: string
}

export interface DomainDetail {
  domain: {
    id: number
    host: string
    port: number
    protocol: string
    remark: string
  }
  results: AgentResultRow[]
}
```

- [ ] **步骤 2：实现 fetch 包装**

```ts
// web/src/api.ts
import type { Overview, DomainsResponse, DomainDetail } from './types'

async function get<T>(path: string): Promise<T> {
  const res = await fetch(path)
  if (!res.ok) {
    throw new Error(`${res.status} ${res.statusText}`)
  }
  return res.json()
}

export const api = {
  overview: () => get<Overview>('/api/dashboard/overview'),
  domains: () => get<DomainsResponse>('/api/dashboard/domains'),
  domainDetail: (id: number | string) => get<DomainDetail>(`/api/dashboard/domains/${id}`),
}
```

- [ ] **步骤 3：Commit**

```bash
git add web/src/types.ts web/src/api.ts
git commit -m "feat(web): add typed API client for dashboard endpoints"
```

---

## 任务 6：状态展示组件

**文件：**
- 创建：`web/src/components/StatusDot.vue`
- 创建：`web/src/components/StatCard.vue`

- [ ] **步骤 1：StatusDot.vue**

```vue
<!-- web/src/components/StatusDot.vue -->
<script setup lang="ts">
import type { Status } from '../types'
import { computed } from 'vue'

const props = defineProps<{ status: Status; label?: boolean }>()

const colorClass = computed(() => {
  switch (props.status) {
    case 'ok':
      return 'bg-ok'
    case 'expiring':
      return 'bg-warn'
    case 'expired':
    case 'mismatch':
    case 'unreachable':
      return 'bg-bad'
    default:
      return 'bg-ink-soft'
  }
})

const text = computed(() => {
  switch (props.status) {
    case 'ok': return '正常'
    case 'expiring': return '即将过期'
    case 'expired': return '已过期'
    case 'mismatch': return '域名不匹配'
    case 'unreachable': return '无法连接'
    default: return '未检测'
  }
})
</script>

<template>
  <span class="inline-flex items-center gap-2">
    <span class="w-2 h-2 rounded-full" :class="colorClass" />
    <span v-if="label" class="text-sm">{{ text }}</span>
  </span>
</template>
```

- [ ] **步骤 2：StatCard.vue**

```vue
<!-- web/src/components/StatCard.vue -->
<script setup lang="ts">
defineProps<{
  label: string
  value: number | string
  hint?: string
  tone?: 'default' | 'ok' | 'warn' | 'bad'
}>()
</script>

<template>
  <div class="bg-bg rounded-2xl p-6 border border-border-soft">
    <div class="text-xs text-ink-soft uppercase tracking-wider">{{ label }}</div>
    <div
      class="mt-3 text-4xl font-light tabular-nums"
      :class="{
        'text-ok': tone === 'ok',
        'text-warn': tone === 'warn',
        'text-bad': tone === 'bad',
      }"
    >
      {{ value }}
    </div>
    <div v-if="hint" class="mt-1 text-xs text-ink-soft">{{ hint }}</div>
  </div>
</template>
```

- [ ] **步骤 3：Commit**

```bash
git add web/src/components/
git commit -m "feat(web): add StatusDot and StatCard components"
```

---

## 任务 7：Overview 页（首页）

**文件：**
- 创建：`web/src/views/Overview.vue`

- [ ] **步骤 1：实现页面**

```vue
<!-- web/src/views/Overview.vue -->
<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { ChevronRight } from 'lucide-vue-next'
import StatCard from '../components/StatCard.vue'
import StatusDot from '../components/StatusDot.vue'
import { api } from '../api'
import type { Overview, DomainSummary } from '../types'

const overview = ref<Overview | null>(null)
const domains = ref<DomainSummary[]>([])
const error = ref('')
const loading = ref(true)

let timer: number | undefined

async function refresh() {
  try {
    const [o, d] = await Promise.all([api.overview(), api.domains()])
    overview.value = o
    domains.value = d.domains
    error.value = ''
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  refresh()
  timer = window.setInterval(refresh, 30_000)
})
onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div>
    <h1 class="text-3xl font-semibold mb-8">概览</h1>

    <div v-if="error" class="mb-6 p-4 bg-bad/10 text-bad rounded-xl text-sm">
      加载失败：{{ error }}
    </div>

    <div v-if="overview" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6 mb-10">
      <StatCard label="域名总数" :value="overview.total_domains" />
      <StatCard label="健康" :value="overview.healthy_domains" tone="ok" />
      <StatCard label="异常" :value="overview.alert_domains" :tone="overview.alert_domains > 0 ? 'bad' : 'default'" />
      <StatCard
        label="Agent 在线"
        :value="`${overview.agents_online} / ${overview.agents_total}`"
        :tone="overview.agents_online < overview.agents_total ? 'warn' : 'default'"
      />
    </div>

    <h2 class="text-xl font-medium mb-4">域名</h2>
    <div v-if="loading && domains.length === 0" class="text-ink-soft">加载中...</div>
    <div v-else-if="domains.length === 0" class="text-ink-soft text-sm">还没有任何域名。</div>
    <div v-else class="bg-bg rounded-2xl border border-border-soft divide-y divide-border-soft">
      <RouterLink
        v-for="d in domains"
        :key="d.id"
        :to="`/domains/${d.id}`"
        class="flex items-center justify-between px-6 py-4 hover:bg-bg-subtle transition-colors"
      >
        <div class="flex items-center gap-3 min-w-0">
          <StatusDot :status="d.worst_status" />
          <div class="truncate">
            <div class="font-medium">{{ d.host }}<span class="text-ink-soft text-sm">:{{ d.port }}</span></div>
            <div v-if="d.remark" class="text-xs text-ink-soft truncate">{{ d.remark }}</div>
          </div>
        </div>
        <div class="flex items-center gap-4 text-sm shrink-0">
          <span v-if="d.total_checks > 0" class="tabular-nums" :class="d.healthy_count === d.total_checks ? 'text-ok' : 'text-ink-soft'">
            {{ d.healthy_count }} / {{ d.total_checks }} 健康
          </span>
          <span v-else class="text-ink-soft text-xs">未检测</span>
          <ChevronRight :size="18" class="text-ink-soft" />
        </div>
      </RouterLink>
    </div>
  </div>
</template>
```

- [ ] **步骤 2：Commit**

```bash
git add web/src/views/Overview.vue
git commit -m "feat(web): add Overview page with stat cards and domain list"
```

---

## 任务 8：DomainDetail 页

**文件：**
- 创建：`web/src/views/DomainDetail.vue`

- [ ] **步骤 1：实现详情页**

```vue
<!-- web/src/views/DomainDetail.vue -->
<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import { ArrowLeft } from 'lucide-vue-next'
import StatusDot from '../components/StatusDot.vue'
import { api } from '../api'
import type { DomainDetail } from '../types'

const route = useRoute()
const data = ref<DomainDetail | null>(null)
const error = ref('')
let timer: number | undefined

async function refresh() {
  try {
    data.value = await api.domainDetail(route.params.id as string)
    error.value = ''
  } catch (e) {
    error.value = (e as Error).message
  }
}

function formatDate(iso: string | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  return d.toLocaleString('zh-CN', { hour12: false })
}

function daysRemaining(iso: string | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  const days = Math.floor((d.getTime() - Date.now()) / (24 * 3600 * 1000))
  if (days < 0) return `已过期 ${-days} 天`
  return `${days} 天后过期`
}

function parseSANs(s: string): string[] {
  if (!s) return []
  try { return JSON.parse(s) } catch { return [] }
}

const domain = computed(() => data.value?.domain)
const results = computed(() => data.value?.results || [])

onMounted(() => {
  refresh()
  timer = window.setInterval(refresh, 30_000)
})
onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div>
    <RouterLink to="/" class="inline-flex items-center gap-1 text-sm text-ink-soft hover:text-ink mb-6">
      <ArrowLeft :size="16" /> 返回
    </RouterLink>

    <div v-if="error" class="mb-6 p-4 bg-bad/10 text-bad rounded-xl text-sm">
      加载失败：{{ error }}
    </div>

    <template v-if="domain">
      <h1 class="text-3xl font-semibold">{{ domain.host }}<span class="text-ink-soft">:{{ domain.port }}</span></h1>
      <div class="text-ink-soft text-sm mt-1">{{ domain.protocol.toUpperCase() }} · {{ domain.remark || '无备注' }}</div>

      <h2 class="text-xl font-medium mt-10 mb-4">各 Agent 检测结果</h2>
      <div v-if="results.length === 0" class="text-ink-soft text-sm">尚无任何 Agent 上报结果。</div>
      <div v-else class="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <div
          v-for="r in results"
          :key="r.agent_id"
          class="bg-bg rounded-2xl border border-border-soft p-6"
        >
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-3">
              <StatusDot :status="r.status" label />
              <span class="font-medium">{{ r.agent_display_name || r.agent_id }}</span>
              <span v-if="!r.agent_online" class="text-xs text-ink-soft">(离线)</span>
            </div>
            <span class="text-xs text-ink-soft">{{ formatDate(r.checked_at) }}</span>
          </div>

          <dl class="mt-5 space-y-3 text-sm">
            <div class="flex justify-between gap-4">
              <dt class="text-ink-soft shrink-0">过期时间</dt>
              <dd class="text-right">
                {{ formatDate(r.not_after) }}
                <span v-if="r.not_after" class="block text-xs text-ink-soft">{{ daysRemaining(r.not_after) }}</span>
              </dd>
            </div>
            <div class="flex justify-between gap-4">
              <dt class="text-ink-soft shrink-0">颁发者</dt>
              <dd class="text-right truncate">{{ r.issuer || '—' }}</dd>
            </div>
            <div v-if="parseSANs(r.sans).length > 0">
              <dt class="text-ink-soft mb-1">SAN</dt>
              <dd class="flex flex-wrap gap-1.5">
                <span
                  v-for="san in parseSANs(r.sans)"
                  :key="san"
                  class="px-2 py-0.5 rounded-md bg-bg-subtle text-xs"
                >{{ san }}</span>
              </dd>
            </div>
            <div v-if="r.error_message" class="pt-2 border-t border-border-soft">
              <dt class="text-ink-soft mb-1">错误</dt>
              <dd class="text-bad text-xs break-all">{{ r.error_message }}</dd>
            </div>
          </dl>
        </div>
      </div>
    </template>
  </div>
</template>
```

- [ ] **步骤 2：Commit**

```bash
git add web/src/views/DomainDetail.vue
git commit -m "feat(web): add DomainDetail page with per-Agent result cards"
```

---

## 任务 9：构建集成 + Makefile

**文件：**
- 修改：`Makefile`

- [ ] **步骤 1：扩展 Makefile**

```makefile
.PHONY: help install-web build-web build-server build-agent build-all dev-web dev-server test clean

help:
	@echo "Available targets:"
	@echo "  install-web     Install frontend npm dependencies"
	@echo "  build-web       Build frontend → server/internal/web/dist/"
	@echo "  build-server    Build server (depends on build-web)"
	@echo "  build-agent     Build agent"
	@echo "  build-all       Build everything"
	@echo "  dev-web         Run Vite dev server (proxies /api to :8080)"
	@echo "  dev-server      Run server with default config.yaml"
	@echo "  test            Run all Go tests"
	@echo "  clean           Remove binaries and build outputs"

install-web:
	cd web && npm install

build-web:
	cd web && npm run build

build-server: build-web
	cd server && go build -o server cmd/server/main.go

build-agent:
	cd agent && go build -o agent cmd/agent/main.go

build-all: build-server build-agent

dev-web:
	cd web && npm run dev

dev-server:
	cd server && go run ./cmd/server -config config.yaml

test:
	cd server && go test ./...
	cd agent && go test ./...

clean:
	rm -f server/server server/server.exe
	rm -f agent/agent agent/agent.exe
	rm -rf data/
	rm -rf web/dist server/internal/web/dist/assets server/internal/web/dist/index.html
```

> **注意：** `clean` 不删 `.gitkeep`，下次 `build-web` 后 dist 又会被 vite 重建。

- [ ] **步骤 2：Commit**

```bash
git add Makefile
git commit -m "build: add Makefile targets for frontend build and dev server"
```

---

## 任务 10：端到端验证

**目标：** 在真实浏览器中看到首页和详情页，并验证 SPA 路由直接访问也能 fallback。

- [ ] **步骤 1：构建前端**

```bash
cd web && npm install && npm run build
```

预期：`server/internal/web/dist/` 下生成 `index.html` + `assets/*.js,*.css`。

- [ ] **步骤 2：构建并启动 Server**

```bash
cd ../server
go build -o server.exe ./cmd/server
cp config.yaml.example config.yaml.e2e
# Edit config.yaml.e2e: agent_token=plan2-token, sqlite path=./data/plan2.db
mkdir -p data && rm -f data/plan2.db
./server.exe -config config.yaml.e2e
```

- [ ] **步骤 3：注入测试数据**

新终端：

```bash
# 创建一个全局域名
curl -X POST http://localhost:8080/api/admin/domains \
  -H "Content-Type: application/json" \
  -d '{"host":"example.com","port":443,"protocol":"https","is_global":true,"remark":"plan2 test"}'

# 注册一个假 Agent + 写一条 ok 结果（用 Agent API）
curl -X POST http://localhost:8080/api/agent/register \
  -H "Authorization: Bearer plan2-token" \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"abcdef0123456789","display_name":"Beijing","hostname":"h1","ip":"127.0.0.1"}'

# 让 Agent 拉一次域名（更新 last_seen）
curl -H "Authorization: Bearer plan2-token" \
  "http://localhost:8080/api/agent/domains?agent_id=abcdef0123456789"

# 上报一条结果
NOT_AFTER=$(date -d '+90 days' -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -v+90d -u +%Y-%m-%dT%H:%M:%SZ)
NOW=$(date -u +%Y-%m-%dT%H:%M:%SZ)
curl -X POST http://localhost:8080/api/agent/results \
  -H "Authorization: Bearer plan2-token" \
  -H "Content-Type: application/json" \
  -d "{\"agent_id\":\"abcdef0123456789\",\"results\":[{\"domain_id\":1,\"checked_at\":\"$NOW\",\"status\":\"ok\",\"not_after\":\"$NOT_AFTER\",\"issuer\":\"LE\",\"subject\":\"CN=example.com\",\"sans\":\"[\\\"example.com\\\"]\"}]}"
```

- [ ] **步骤 4：浏览器验证**

打开 http://localhost:8080/

预期：
- 4 张概览卡：1 / 1 / 0 / 1 / 1
- 域名列表显示 `example.com:443`，状态为绿点，右侧 `1 / 1 健康`
- 点击进入详情页 URL 变为 `/domains/1`，看到 Beijing Agent 的卡片，过期时间 `90 天后过期`

- [ ] **步骤 5：SPA 路由刷新验证**

在 `/domains/1` 页面按 F5 刷新；浏览器地址不变，页面仍正常加载（验证任务 3 的 `NoRoute` fallback）。

- [ ] **步骤 6：清理**

```bash
# 停止 server (Ctrl+C)
rm -f config.yaml.e2e server.exe
rm -rf data/plan2.db
```

- [ ] **步骤 7：Commit 完成标记**

如有任何脚手架修复、Makefile 微调，统一 commit：

```bash
git status
# 若有遗留改动：
git add -u
git commit -m "chore: tidy up after Plan 2 manual verification"
```

---

## 完成标准

✅ Server 新增 3 个 `/api/dashboard/*` 端点，无认证可访问
✅ Vue 3 SPA 通过 `embed.FS` 打进 server 二进制
✅ 浏览器访问 `/` 看到概览卡 + 域名列表
✅ 点击域名进入详情页，看到各 Agent 的检测对比
✅ 直接访问 `/domains/:id` 或刷新页面，路由 fallback 工作正常
✅ 30 秒轮询自动刷新数据
✅ Server / Store / Dashboard API 单元测试全部通过
✅ Makefile 一条命令 `make build-all` 产出可分发的 server + agent 二进制

---

## 下一步

Plan 3：Alert Engine + Production Features
- 管理员登录（`POST /api/auth/login` + bcrypt + Cookie session）
- `/api/admin/*` 全套（domains 完整 CRUD、agents 列表 + override、alert-channels CRUD、history 查询）
- 告警引擎（状态变化告警 + 每日提醒 + 5 种渠道）
- 历史清理定时任务
- Admin 前端页面（/login, /admin/*）

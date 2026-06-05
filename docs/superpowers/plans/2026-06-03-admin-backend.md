# Plan 3.2：管理后台 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 在 Plan 3.1 登录认证基础上，构建管理员可视化操作的域名管理和 Agent 管理界面

**架构：** 后端新增 6 个 admin API 端点（域名编辑、Agent 列表/编辑、Override CRUD），前端新增 3 个管理页面（域名管理、Agent 列表、Override 矩阵），Header 增加顶级导航链接

**技术栈：** Go + Gin + GORM（后端）、Vue 3 + TypeScript + Tailwind CSS（前端）

---

## 任务 1：Store 层 - AgentOnlineWindow 常量 + 4 个新方法

**文件：**
- 修改：`server/internal/store/store.go`（新增常量 + 4 方法）
- 测试：`server/internal/store/admin_test.go`（新建）

### 步骤 1：编写失败的测试

创建 `server/internal/store/admin_test.go`：

```go
package store

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestUpdateDomainMeta(t *testing.T) {
	s := setupTestDB(t)
	d := &Domain{Host: "example.com", Port: 443, Protocol: "https", IsGlobal: false, Remark: "old"}
	if err := s.CreateDomain(d); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateDomainMeta(d.ID, true, "new remark"); err != nil {
		t.Fatalf("UpdateDomainMeta: %v", err)
	}
	got, err := s.GetDomain(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsGlobal {
		t.Errorf("expected IsGlobal=true, got false")
	}
	if got.Remark != "new remark" {
		t.Errorf("expected remark='new remark', got %q", got.Remark)
	}
	if got.Host != "example.com" || got.Port != 443 || got.Protocol != "https" {
		t.Errorf("host/port/protocol should not change, got %+v", got)
	}
}

func TestUpdateDomainMeta_NotFound(t *testing.T) {
	s := setupTestDB(t)
	err := s.UpdateDomainMeta(9999, true, "x")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestUpdateAgentRemark(t *testing.T) {
	s := setupTestDB(t)
	a := &Agent{AgentID: "a1", DisplayName: "A1", Remark: "old", RegisteredAt: time.Now(), LastSeenAt: time.Now()}
	if err := s.CreateAgent(a); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateAgentRemark("a1", "new remark"); err != nil {
		t.Fatalf("UpdateAgentRemark: %v", err)
	}
	got, err := s.GetAgent("a1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Remark != "new remark" {
		t.Errorf("expected remark='new remark', got %q", got.Remark)
	}
}

func TestListOverrides(t *testing.T) {
	s := setupTestDB(t)
	d1 := &Domain{Host: "d1.com", Port: 443, Protocol: "https"}
	d2 := &Domain{Host: "d2.com", Port: 443, Protocol: "https"}
	s.CreateDomain(d1)
	s.CreateDomain(d2)
	s.CreateOverride(&AgentDomainOverride{AgentID: "a1", DomainID: d1.ID, Action: "include"})
	s.CreateOverride(&AgentDomainOverride{AgentID: "a1", DomainID: d2.ID, Action: "exclude"})
	overrides, err := s.ListOverrides("a1")
	if err != nil {
		t.Fatal(err)
	}
	if len(overrides) != 2 {
		t.Fatalf("expected 2 overrides, got %d", len(overrides))
	}
}

func TestUpsertOverride(t *testing.T) {
	s := setupTestDB(t)
	d := &Domain{Host: "d.com", Port: 443, Protocol: "https"}
	s.CreateDomain(d)
	// Insert
	if err := s.UpsertOverride("a1", d.ID, "include"); err != nil {
		t.Fatalf("UpsertOverride insert: %v", err)
	}
	// Update action
	if err := s.UpsertOverride("a1", d.ID, "exclude"); err != nil {
		t.Fatalf("UpsertOverride update: %v", err)
	}
	overrides, _ := s.ListOverrides("a1")
	if len(overrides) != 1 || overrides[0].Action != "exclude" {
		t.Errorf("expected 1 override with action=exclude, got %+v", overrides)
	}
}
```

### 步骤 2：运行测试验证失败

运行：`cd server && go test ./internal/store -run "TestUpdateDomainMeta|TestUpdateAgentRemark|TestListOverrides|TestUpsertOverride" -v`

预期：编译失败，`UpdateDomainMeta not defined`

### 步骤 3：实现 Store 层方法

在 `server/internal/store/store.go` 顶部（`import` 之后、`type Store struct` 之前）添加常量：

```go
const AgentOnlineWindow = 3 * time.Hour
```

在文件末尾追加 4 个方法：

```go
// UpdateDomainMeta updates only is_global and remark; host/port/protocol are immutable.
func (s *Store) UpdateDomainMeta(id uint, isGlobal bool, remark string) error {
	res := s.db.Model(&Domain{}).Where("id = ?", id).Updates(map[string]interface{}{
		"is_global": isGlobal,
		"remark":    remark,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateAgentRemark updates agent remark field only.
func (s *Store) UpdateAgentRemark(agentID, remark string) error {
	res := s.db.Model(&Agent{}).Where("agent_id = ?", agentID).Update("remark", remark)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListOverrides returns all overrides for an agent.
func (s *Store) ListOverrides(agentID string) ([]AgentDomainOverride, error) {
	var overrides []AgentDomainOverride
	err := s.db.Where("agent_id = ?", agentID).Find(&overrides).Error
	return overrides, err
}

// UpsertOverride inserts or updates an override.
func (s *Store) UpsertOverride(agentID string, domainID uint, action string) error {
	override := &AgentDomainOverride{AgentID: agentID, DomainID: domainID, Action: action}
	return s.db.Where(AgentDomainOverride{AgentID: agentID, DomainID: domainID}).
		Assign(AgentDomainOverride{Action: action}).FirstOrCreate(override).Error
}
```

### 步骤 4：运行测试验证通过

运行：`cd server && go test ./internal/store -run "TestUpdateDomainMeta|TestUpdateAgentRemark|TestListOverrides|TestUpsertOverride" -v`

预期：5 个测试全部 PASS

### 步骤 5：Commit

```bash
git add server/internal/store/store.go server/internal/store/admin_test.go
git commit -m "feat(store): add admin operations (UpdateDomainMeta, UpdateAgentRemark, ListOverrides, UpsertOverride)"
```

---

## 任务 2：后端 API - UpdateDomain 端点

**文件：**
- 修改：`server/internal/api/admin.go`（新增 UpdateDomain handler）
- 修改：`server/internal/api/admin_test.go`（扩展测试）

### 步骤 1：编写失败的测试

在 `server/internal/api/admin_test.go` 末尾追加：

```go
func TestUpdateDomain_Success(t *testing.T) {
	r, s := setupRouter(t)
	d := &store.Domain{Host: "old.com", Port: 443, Protocol: "https", IsGlobal: false, Remark: "old"}
	s.CreateDomain(d)
	body := `{"is_global": true, "remark": "new"}`
	req := httptest.NewRequest("PUT", "/api/admin/domains/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	got, _ := s.GetDomain(d.ID)
	if !got.IsGlobal || got.Remark != "new" {
		t.Errorf("expected is_global=true remark=new, got %+v", got)
	}
}

func TestUpdateDomain_NotFound(t *testing.T) {
	r, _ := setupRouter(t)
	body := `{"is_global": true, "remark": "x"}`
	req := httptest.NewRequest("PUT", "/api/admin/domains/999", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdateDomain_MissingFields(t *testing.T) {
	r, _ := setupRouter(t)
	d := &store.Domain{Host: "x.com", Port: 443, Protocol: "https"}
	s, _ := r.Get("/api/admin/domains")
	s.Handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/", nil))
	// Missing is_global
	body := `{"remark": "x"}`
	req := httptest.NewRequest("PUT", "/api/admin/domains/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpdateDomain_IgnoresHostInBody(t *testing.T) {
	r, s := setupRouter(t)
	d := &store.Domain{Host: "old.com", Port: 443, Protocol: "https"}
	s.CreateDomain(d)
	body := `{"is_global": true, "remark": "new", "host": "hacked.com"}`
	req := httptest.NewRequest("PUT", "/api/admin/domains/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	got, _ := s.GetDomain(d.ID)
	if got.Host != "old.com" {
		t.Errorf("host should not change, got %q", got.Host)
	}
}
```

### 步骤 2：运行测试验证失败

运行：`cd server && go test ./internal/api -run "TestUpdateDomain" -v`

预期：编译失败，`UpdateDomain not defined`

### 步骤 3：实现 UpdateDomain handler

在 `server/internal/api/admin.go` 末尾追加：

```go
func (h *AdminHandler) UpdateDomain(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_id", "message": "invalid domain ID"}})
		return
	}
	var req struct {
		IsGlobal *bool  `json:"is_global"`
		Remark   string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": err.Error()}})
		return
	}
	if req.IsGlobal == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "is_global is required"}})
		return
	}
	if err := h.store.UpdateDomainMeta(uint(id), *req.IsGlobal, req.Remark); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "domain not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
```

在文件顶部 `import` 中添加：
```go
import (
	"errors"
	// ... existing imports
	"gorm.io/gorm"
)
```

### 步骤 4：运行测试验证通过

运行：`cd server && go test ./internal/api -run "TestUpdateDomain" -v`

预期：4 个测试全部 PASS

### 步骤 5：Commit

```bash
git add server/internal/api/admin.go server/internal/api/admin_test.go
git commit -m "feat(api): add PUT /api/admin/domains/:id endpoint"
```

---

## 任务 3：后端 API - ListAgents + UpdateAgent 端点

**文件：**
- 修改：`server/internal/api/admin.go`（新增 2 handlers）
- 修改：`server/internal/api/admin_test.go`（扩展测试）

### 步骤 1：编写失败的测试

在 `server/internal/api/admin_test.go` 末尾追加：

```go
func TestListAgents_IncludesOnlineFlag(t *testing.T) {
	r, s := setupRouter(t)
	a1 := &store.Agent{AgentID: "a1", DisplayName: "A1", LastSeenAt: time.Now()}
	a2 := &store.Agent{AgentID: "a2", DisplayName: "A2", LastSeenAt: time.Now().Add(-5 * time.Hour)}
	s.CreateAgent(a1)
	s.CreateAgent(a2)
	req := httptest.NewRequest("GET", "/api/admin/agents", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Agents []struct {
			AgentID  string `json:"agent_id"`
			IsOnline bool   `json:"is_online"`
		} `json:"agents"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(resp.Agents))
	}
	if !resp.Agents[0].IsOnline {
		t.Errorf("agent a1 should be online")
	}
	if resp.Agents[1].IsOnline {
		t.Errorf("agent a2 should be offline")
	}
}

func TestUpdateAgentRemark_Success(t *testing.T) {
	r, s := setupRouter(t)
	a := &store.Agent{AgentID: "a1", DisplayName: "A1", Remark: "old", LastSeenAt: time.Now()}
	s.CreateAgent(a)
	body := `{"remark": "new"}`
	req := httptest.NewRequest("PUT", "/api/admin/agents/a1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	got, _ := s.GetAgent("a1")
	if got.Remark != "new" {
		t.Errorf("expected remark=new, got %q", got.Remark)
	}
}

func TestUpdateAgentRemark_NotFound(t *testing.T) {
	r, _ := setupRouter(t)
	body := `{"remark": "x"}`
	req := httptest.NewRequest("PUT", "/api/admin/agents/999", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
```

### 步骤 2：运行测试验证失败

运行：`cd server && go test ./internal/api -run "TestListAgents|TestUpdateAgentRemark" -v`

预期：编译失败，`ListAgents not defined`

### 步骤 3：实现 handlers

在 `server/internal/api/admin.go` 末尾追加：

```go
func (h *AdminHandler) ListAgents(c *gin.Context) {
	agents, err := h.store.ListAgents()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	threshold := time.Now().Add(-store.AgentOnlineWindow)
	out := make([]gin.H, 0, len(agents))
	for _, a := range agents {
		out = append(out, gin.H{
			"agent_id":      a.AgentID,
			"display_name":  a.DisplayName,
			"hostname":      a.Hostname,
			"ip":            a.IP,
			"remark":        a.Remark,
			"registered_at": a.RegisteredAt,
			"last_seen_at":  a.LastSeenAt,
			"is_online":     !a.LastSeenAt.IsZero() && a.LastSeenAt.After(threshold),
		})
	}
	c.JSON(http.StatusOK, gin.H{"agents": out})
}

func (h *AdminHandler) UpdateAgent(c *gin.Context) {
	agentID := c.Param("id")
	var req struct {
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": err.Error()}})
		return
	}
	if err := h.store.UpdateAgentRemark(agentID, req.Remark); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "agent not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
```

### 步骤 4：运行测试验证通过

运行：`cd server && go test ./internal/api -run "TestListAgents|TestUpdateAgentRemark" -v`

预期：3 个测试全部 PASS

### 步骤 5：Commit

```bash
git add server/internal/api/admin.go server/internal/api/admin_test.go
git commit -m "feat(api): add GET /api/admin/agents and PUT /api/admin/agents/:id endpoints"
```

---

## 任务 4：后端 API - Override CRUD 端点

**文件：**
- 修改：`server/internal/api/admin.go`（新增 3 handlers）
- 修改：`server/internal/api/admin_test.go`（扩展测试）

### 步骤 1：编写失败的测试

在 `server/internal/api/admin_test.go` 末尾追加：

```go
func TestListOverrides_Empty(t *testing.T) {
	r, _ := setupRouter(t)
	req := httptest.NewRequest("GET", "/api/admin/agents/a1/overrides", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Overrides []interface{} `json:"overrides"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Overrides) != 0 {
		t.Errorf("expected 0 overrides, got %d", len(resp.Overrides))
	}
}

func TestListOverrides_IncludeAndExclude(t *testing.T) {
	r, s := setupRouter(t)
	a := &store.Agent{AgentID: "a1", DisplayName: "A1", LastSeenAt: time.Now()}
	d1 := &store.Domain{Host: "d1.com", Port: 443, Protocol: "https"}
	d2 := &store.Domain{Host: "d2.com", Port: 443, Protocol: "https"}
	s.CreateAgent(a)
	s.CreateDomain(d1)
	s.CreateDomain(d2)
	s.CreateOverride(&store.AgentDomainOverride{AgentID: "a1", DomainID: d1.ID, Action: "include"})
	s.CreateOverride(&store.AgentDomainOverride{AgentID: "a1", DomainID: d2.ID, Action: "exclude"})
	req := httptest.NewRequest("GET", "/api/admin/agents/a1/overrides", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Overrides []struct {
			DomainID uint   `json:"domain_id"`
			Action   string `json:"action"`
		} `json:"overrides"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Overrides) != 2 {
		t.Fatalf("expected 2 overrides, got %d", len(resp.Overrides))
	}
}

func TestSetOverride_New(t *testing.T) {
	r, s := setupRouter(t)
	a := &store.Agent{AgentID: "a1", DisplayName: "A1", LastSeenAt: time.Now()}
	d := &store.Domain{Host: "d.com", Port: 443, Protocol: "https"}
	s.CreateAgent(a)
	s.CreateDomain(d)
	body := `{"domain_id": 1, "action": "include"}`
	req := httptest.NewRequest("POST", "/api/admin/agents/a1/overrides", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestSetOverride_UpdateAction(t *testing.T) {
	r, s := setupRouter(t)
	a := &store.Agent{AgentID: "a1", DisplayName: "A1", LastSeenAt: time.Now()}
	d := &store.Domain{Host: "d.com", Port: 443, Protocol: "https"}
	s.CreateAgent(a)
	s.CreateDomain(d)
	s.CreateOverride(&store.AgentDomainOverride{AgentID: "a1", DomainID: d.ID, Action: "include"})
	body := `{"domain_id": 1, "action": "exclude"}`
	req := httptest.NewRequest("POST", "/api/admin/agents/a1/overrides", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	overrides, _ := s.ListOverrides("a1")
	if len(overrides) != 1 || overrides[0].Action != "exclude" {
		t.Errorf("expected 1 override with action=exclude, got %+v", overrides)
	}
}

func TestSetOverride_InvalidAction(t *testing.T) {
	r, s := setupRouter(t)
	a := &store.Agent{AgentID: "a1", DisplayName: "A1", LastSeenAt: time.Now()}
	d := &store.Domain{Host: "d.com", Port: 443, Protocol: "https"}
	s.CreateAgent(a)
	s.CreateDomain(d)
	body := `{"domain_id": 1, "action": "invalid"}`
	req := httptest.NewRequest("POST", "/api/admin/agents/a1/overrides", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSetOverride_DomainNotFound(t *testing.T) {
	r, s := setupRouter(t)
	a := &store.Agent{AgentID: "a1", DisplayName: "A1", LastSeenAt: time.Now()}
	s.CreateAgent(a)
	body := `{"domain_id": 999, "action": "include"}`
	req := httptest.NewRequest("POST", "/api/admin/agents/a1/overrides", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestSetOverride_AgentNotFound(t *testing.T) {
	r, s := setupRouter(t)
	d := &store.Domain{Host: "d.com", Port: 443, Protocol: "https"}
	s.CreateDomain(d)
	body := `{"domain_id": 1, "action": "include"}`
	req := httptest.NewRequest("POST", "/api/admin/agents/999/overrides", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteOverride_Success(t *testing.T) {
	r, s := setupRouter(t)
	a := &store.Agent{AgentID: "a1", DisplayName: "A1", LastSeenAt: time.Now()}
	d := &store.Domain{Host: "d.com", Port: 443, Protocol: "https"}
	s.CreateAgent(a)
	s.CreateDomain(d)
	s.CreateOverride(&store.AgentDomainOverride{AgentID: "a1", DomainID: d.ID, Action: "include"})
	req := httptest.NewRequest("DELETE", "/api/admin/agents/a1/overrides/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	overrides, _ := s.ListOverrides("a1")
	if len(overrides) != 0 {
		t.Errorf("expected 0 overrides after delete, got %d", len(overrides))
	}
}

func TestDeleteOverride_Idempotent(t *testing.T) {
	r, _ := setupRouter(t)
	req := httptest.NewRequest("DELETE", "/api/admin/agents/a1/overrides/999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200 for idempotent delete, got %d", w.Code)
	}
}
```

### 步骤 2：运行测试验证失败

运行：`cd server && go test ./internal/api -run "TestListOverrides|TestSetOverride|TestDeleteOverride" -v`

预期：编译失败，`ListOverrides not defined`

### 步骤 3：实现 handlers

在 `server/internal/api/admin.go` 末尾追加：

```go
func (h *AdminHandler) ListOverrides(c *gin.Context) {
	agentID := c.Param("id")
	overrides, err := h.store.ListOverrides(agentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"overrides": overrides})
}

func (h *AdminHandler) SetOverride(c *gin.Context) {
	agentID := c.Param("id")
	var req struct {
		DomainID uint   `json:"domain_id"`
		Action   string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": err.Error()}})
		return
	}
	if req.Action != "include" && req.Action != "exclude" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "action must be include or exclude"}})
		return
	}
	// Verify domain exists
	if _, err := h.store.GetDomain(req.DomainID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "domain not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	// Verify agent exists
	if _, err := h.store.GetAgent(agentID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "agent not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	if err := h.store.UpsertOverride(agentID, req.DomainID, req.Action); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AdminHandler) DeleteOverride(c *gin.Context) {
	agentID := c.Param("id")
	domainID, err := strconv.ParseUint(c.Param("domain_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_id", "message": "invalid domain ID"}})
		return
	}
	if err := h.store.DeleteOverride(agentID, uint(domainID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
```

### 步骤 4：运行测试验证通过

运行：`cd server && go test ./internal/api -run "TestListOverrides|TestSetOverride|TestDeleteOverride" -v`

预期：9 个测试全部 PASS

### 步骤 5：Commit

```bash
git add server/internal/api/admin.go server/internal/api/admin_test.go
git commit -m "feat(api): add override CRUD endpoints (GET/POST/DELETE /api/admin/agents/:id/overrides)"
```

---

## 任务 5：后端 - Router 集成 + AgentOnlineWindow 常量化

**文件：**
- 修改：`server/internal/api/router.go`（注册 6 新路由）
- 修改：`server/internal/api/dashboard.go`（使用常量替代硬编码 3h）

### 步骤 1：修改 router.go

在 `server/internal/api/router.go` 的 `adminGroup` 块内追加 6 条路由：

```go
adminGroup := r.Group("/api/admin")
adminGroup.Use(auth.AuthMiddleware(sessions))
{
	h := NewAdminHandler(s)
	adminGroup.POST("/domains", h.CreateDomain)
	adminGroup.GET("/domains", h.ListDomains)
	adminGroup.GET("/domains/:id", h.GetDomain)
	adminGroup.PUT("/domains/:id", h.UpdateDomain)
	adminGroup.DELETE("/domains/:id", h.DeleteDomain)
	
	adminGroup.GET("/agents", h.ListAgents)
	adminGroup.PUT("/agents/:id", h.UpdateAgent)
	
	adminGroup.GET("/agents/:id/overrides", h.ListOverrides)
	adminGroup.POST("/agents/:id/overrides", h.SetOverride)
	adminGroup.DELETE("/agents/:id/overrides/:domain_id", h.DeleteOverride)
}
```

### 步骤 2：修改 dashboard.go

在 `server/internal/api/dashboard.go` 第 27 行，将 `3*time.Hour` 替换为 `store.AgentOnlineWindow`：

```go
h := NewDashboardHandler(s, store.AgentOnlineWindow)
```

### 步骤 3：运行全部测试验证不破坏

运行：`cd server && go test ./...`

预期：所有测试 PASS

### 步骤 4：Commit

```bash
git add server/internal/api/router.go server/internal/api/dashboard.go
git commit -m "feat(api): wire 6 new admin routes, use AgentOnlineWindow constant"
```

---

## 任务 6：前端基础设施 - types.ts + api.ts

**文件：**
- 修改：`web/src/types.ts`（新增 3 接口）
- 修改：`web/src/api.ts`（新增 adminApi 对象）

### 步骤 1：扩展 types.ts

在 `web/src/types.ts` 末尾追加：

```typescript
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

### 步骤 2：扩展 api.ts

在 `web/src/api.ts` 末尾追加：

```typescript
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

在文件顶部 import 添加：
```typescript
import type { Overview, DomainsResponse, DomainDetail, User, DomainAdmin, AgentAdmin, Override } from './types'
```

### 步骤 3：运行 TypeScript 编译验证

运行：`cd web && npm run build`

预期：编译通过

### 步骤 4：Commit

```bash
git add web/src/types.ts web/src/api.ts
git commit -m "feat(web): add admin types and API client"
```

---

## 任务 7：前端 - Header 导航集成

**文件：**
- 修改：`web/src/components/Header.vue`（添加导航链接）

### 步骤 1：修改 Header.vue

将 `web/src/components/Header.vue` 完整替换为：

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
      
      <nav v-if="!loading && user" class="flex gap-1 ml-4">
        <RouterLink
          to="/admin/domains"
          class="text-sm font-medium px-3 py-1.5 rounded-md transition"
          active-class="text-ink bg-bg-subtle"
          inactive-class="text-ink-soft hover:text-ink hover:bg-bg-subtle"
        >
          域名管理
        </RouterLink>
        <RouterLink
          to="/admin/agents"
          class="text-sm font-medium px-3 py-1.5 rounded-md transition"
          active-class="text-ink bg-bg-subtle"
          inactive-class="text-ink-soft hover:text-ink hover:bg-bg-subtle"
        >
          Agent 管理
        </RouterLink>
      </nav>
      
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

### 步骤 2：运行编译验证

运行：`cd web && npm run build`

预期：编译通过

### 步骤 3：Commit

```bash
git add web/src/components/Header.vue
git commit -m "feat(web): add admin navigation links to Header"
```

---

## 任务 8：前端 - Router 集成

**文件：**
- 修改：`web/src/router.ts`（新增 3 路由）

### 步骤 1：修改 router.ts

将 `web/src/router.ts` 完整替换为：

```typescript
import { createRouter, createWebHistory } from 'vue-router'
import Overview from './views/Overview.vue'
import DomainDetail from './views/DomainDetail.vue'
import Login from './views/Login.vue'
import AdminDomains from './views/AdminDomains.vue'
import AdminAgents from './views/AdminAgents.vue'
import AdminAgentOverrides from './views/AdminAgentOverrides.vue'
import { useAuth } from './composables/useAuth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: Overview },
    { path: '/domains/:id', component: DomainDetail, props: true },
    { path: '/login', component: Login },
    { path: '/admin/domains', component: AdminDomains, meta: { requiresAuth: true } },
    { path: '/admin/agents', component: AdminAgents, meta: { requiresAuth: true } },
    { path: '/admin/agents/:id', component: AdminAgentOverrides, props: true, meta: { requiresAuth: true } },
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

### 步骤 2：创建空占位文件

创建 `web/src/views/AdminDomains.vue`：

```vue
<template>
  <div>AdminDomains - TODO</div>
</template>
```

创建 `web/src/views/AdminAgents.vue`：

```vue
<template>
  <div>AdminAgents - TODO</div>
</template>
```

创建 `web/src/views/AdminAgentOverrides.vue`：

```vue
<script setup lang="ts">
defineProps<{ id: string }>()
</script>

<template>
  <div>AdminAgentOverrides for {{ id }} - TODO</div>
</template>
```

### 步骤 3：运行编译验证

运行：`cd web && npm run build`

预期：编译通过

### 步骤 4：Commit

```bash
git add web/src/router.ts web/src/views/AdminDomains.vue web/src/views/AdminAgents.vue web/src/views/AdminAgentOverrides.vue
git commit -m "feat(web): add admin routes and placeholder views"
```

---

## 任务 9：前端 - AdminDomains 页面

**文件：**
- 替换：`web/src/views/AdminDomains.vue`（完整实现）

### 步骤 1：实现 AdminDomains.vue

将 `web/src/views/AdminDomains.vue` 完整替换为：

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { adminApi } from '../api'
import type { DomainAdmin } from '../types'

const domains = ref<DomainAdmin[]>([])
const loading = ref(false)
const error = ref('')
const showCreateForm = ref(false)
const editingId = ref<number | null>(null)

const newDomain = ref({ host: '', port: 443, protocol: 'https', is_global: false, remark: '' })
const editForm = ref({ is_global: false, remark: '' })

async function loadDomains() {
  loading.value = true
  try {
    const res = await adminApi.listDomains()
    domains.value = res.domains
  } catch (e: any) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function createDomain() {
  try {
    await adminApi.createDomain(newDomain.value)
    showCreateForm.value = false
    newDomain.value = { host: '', port: 443, protocol: 'https', is_global: false, remark: '' }
    await loadDomains()
  } catch (e: any) {
    error.value = e.message
  }
}

function startEdit(d: DomainAdmin) {
  editingId.value = d.id
  editForm.value = { is_global: d.is_global, remark: d.remark }
}

function cancelEdit() {
  editingId.value = null
}

async function saveEdit(id: number) {
  try {
    await adminApi.updateDomain(id, editForm.value)
    editingId.value = null
    await loadDomains()
  } catch (e: any) {
    error.value = e.message
  }
}

async function deleteDomain(d: DomainAdmin) {
  if (!confirm(`确定删除域名 ${d.host}:${d.port}/${d.protocol} 吗？`)) return
  try {
    await adminApi.deleteDomain(d.id)
    await loadDomains()
  } catch (e: any) {
    error.value = e.message
  }
}

onMounted(loadDomains)
</script>

<template>
  <div class="max-w-6xl mx-auto px-6 py-8">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-semibold text-ink">域名管理</h1>
      <button
        type="button"
        @click="showCreateForm = !showCreateForm"
        class="px-4 py-2 bg-accent text-white rounded-md text-sm font-medium hover:opacity-90"
      >
        {{ showCreateForm ? '取消' : '新增域名' }}
      </button>
    </div>

    <div v-if="error" class="mb-4 p-3 bg-red-50 border border-red-200 rounded-md">
      <p class="text-sm text-red-600">{{ error }}</p>
      <button @click="error = ''" class="text-xs text-red-500 hover:text-red-700">×</button>
    </div>

    <div v-if="showCreateForm" class="mb-6 p-4 bg-bg-subtle rounded-md border border-border-soft">
      <h2 class="text-lg font-medium mb-3">新增域名</h2>
      <form @submit.prevent="createDomain" class="space-y-3">
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="block text-sm text-ink-soft mb-1">Host</label>
            <input v-model="newDomain.host" type="text" required class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink" />
          </div>
          <div>
            <label class="block text-sm text-ink-soft mb-1">Port</label>
            <input v-model.number="newDomain.port" type="number" required class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink" />
          </div>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="block text-sm text-ink-soft mb-1">Protocol</label>
            <select v-model="newDomain.protocol" class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink">
              <option value="https">https</option>
              <option value="wss">wss</option>
            </select>
          </div>
          <div>
            <label class="flex items-center gap-2 text-sm text-ink-soft mb-1">
              <input v-model="newDomain.is_global" type="checkbox" />
              全局监控
            </label>
          </div>
        </div>
        <div>
          <label class="block text-sm text-ink-soft mb-1">备注</label>
          <input v-model="newDomain.remark" type="text" class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink" />
        </div>
        <button type="submit" class="px-4 py-2 bg-accent text-white rounded-md text-sm font-medium hover:opacity-90">
          保存
        </button>
      </form>
    </div>

    <div v-if="loading" class="text-center py-8 text-ink-soft">加载中...</div>
    <div v-else-if="domains.length === 0" class="text-center py-8 text-ink-soft">暂无域名</div>
    
    <table v-else class="w-full">
      <thead>
        <tr class="border-b border-border-soft text-left text-sm text-ink-soft">
          <th class="pb-2 font-medium">Host</th>
          <th class="pb-2 font-medium">Port</th>
          <th class="pb-2 font-medium">Protocol</th>
          <th class="pb-2 font-medium">全局</th>
          <th class="pb-2 font-medium">备注</th>
          <th class="pb-2 font-medium">操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="d in domains" :key="d.id" class="border-b border-border-soft">
          <template v-if="editingId !== d.id">
            <td class="py-3 text-sm text-ink">{{ d.host }}</td>
            <td class="py-3 text-sm text-ink">{{ d.port }}</td>
            <td class="py-3 text-sm text-ink">{{ d.protocol }}</td>
            <td class="py-3 text-sm text-ink">{{ d.is_global ? '是' : '否' }}</td>
            <td class="py-3 text-sm text-ink-soft">{{ d.remark || '-' }}</td>
            <td class="py-3 text-sm">
              <button @click="startEdit(d)" class="text-accent hover:underline mr-3">编辑</button>
              <button @click="deleteDomain(d)" class="text-bad hover:underline">删除</button>
            </td>
          </template>
          <template v-else>
            <td class="py-3 text-sm text-ink-soft">{{ d.host }}</td>
            <td class="py-3 text-sm text-ink-soft">{{ d.port }}</td>
            <td class="py-3 text-sm text-ink-soft">{{ d.protocol }}</td>
            <td class="py-3">
              <input v-model="editForm.is_global" type="checkbox" />
            </td>
            <td class="py-3">
              <input v-model="editForm.remark" type="text" class="px-2 py-1 border border-border-soft rounded text-sm" />
            </td>
            <td class="py-3 text-sm">
              <button @click="saveEdit(d.id)" class="text-accent hover:underline mr-3">保存</button>
              <button @click="cancelEdit" class="text-ink-soft hover:underline">取消</button>
            </td>
          </template>
        </tr>
      </tbody>
    </table>
  </div>
</template>
```

### 步骤 2：运行编译验证

运行：`cd web && npm run build`

预期：编译通过

### 步骤 3：Commit

```bash
git add web/src/views/AdminDomains.vue
git commit -m "feat(web): implement AdminDomains page (CRUD with inline edit)"
```

---

## 任务 10：前端 - AdminAgents 页面

**文件：**
- 替换：`web/src/views/AdminAgents.vue`（完整实现）

### 步骤 1：实现 AdminAgents.vue

将 `web/src/views/AdminAgents.vue` 完整替换为：

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { adminApi } from '../api'
import type { AgentAdmin } from '../types'

const router = useRouter()
const agents = ref<AgentAdmin[]>([])
const loading = ref(false)
const error = ref('')
const editingId = ref<string | null>(null)
const editRemark = ref('')

async function loadAgents() {
  loading.value = true
  try {
    const res = await adminApi.listAgents()
    agents.value = res.agents
  } catch (e: any) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function startEdit(a: AgentAdmin) {
  editingId.value = a.agent_id
  editRemark.value = a.remark
}

function cancelEdit() {
  editingId.value = null
}

async function saveEdit(agentId: string) {
  try {
    await adminApi.updateAgentRemark(agentId, editRemark.value)
    editingId.value = null
    await loadAgents()
  } catch (e: any) {
    error.value = e.message
  }
}

function goToOverrides(agentId: string) {
  router.push(`/admin/agents/${agentId}`)
}

function formatOffline(lastSeen: string): string {
  const diff = Date.now() - new Date(lastSeen).getTime()
  const hours = Math.floor(diff / 3600000)
  const days = Math.floor(hours / 24)
  if (days > 0) return `离线 ${days} 天前`
  if (hours > 0) return `离线 ${hours} 小时前`
  return '刚刚离线'
}

onMounted(loadAgents)
</script>

<template>
  <div class="max-w-6xl mx-auto px-6 py-8">
    <h1 class="text-2xl font-semibold text-ink mb-6">Agent 管理</h1>

    <div v-if="error" class="mb-4 p-3 bg-red-50 border border-red-200 rounded-md">
      <p class="text-sm text-red-600">{{ error }}</p>
      <button @click="error = ''" class="text-xs text-red-500 hover:text-red-700">×</button>
    </div>

    <div v-if="loading" class="text-center py-8 text-ink-soft">加载中...</div>
    <div v-else-if="agents.length === 0" class="text-center py-8 text-ink-soft">暂无 Agent</div>
    
    <table v-else class="w-full">
      <thead>
        <tr class="border-b border-border-soft text-left text-sm text-ink-soft">
          <th class="pb-2 font-medium">名称</th>
          <th class="pb-2 font-medium">主机名</th>
          <th class="pb-2 font-medium">IP</th>
          <th class="pb-2 font-medium">备注</th>
          <th class="pb-2 font-medium">状态</th>
          <th class="pb-2 font-medium">最后心跳</th>
          <th class="pb-2 font-medium">操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="a in agents" :key="a.agent_id" class="border-b border-border-soft">
          <td class="py-3 text-sm text-ink">{{ a.display_name }}</td>
          <td class="py-3 text-sm text-ink">{{ a.hostname }}</td>
          <td class="py-3 text-sm text-ink">{{ a.ip }}</td>
          <template v-if="editingId !== a.agent_id">
            <td class="py-3 text-sm text-ink-soft">{{ a.remark || '-' }}</td>
          </template>
          <template v-else>
            <td class="py-3">
              <input v-model="editRemark" type="text" class="px-2 py-1 border border-border-soft rounded text-sm w-full" />
            </td>
          </template>
          <td class="py-3 text-sm">
            <span v-if="a.is_online" class="text-ok font-medium">在线</span>
            <span v-else class="text-ink-soft">{{ formatOffline(a.last_seen_at) }}</span>
          </td>
          <td class="py-3 text-sm text-ink-soft">{{ new Date(a.last_seen_at).toLocaleString('zh-CN') }}</td>
          <td class="py-3 text-sm">
            <template v-if="editingId !== a.agent_id">
              <button @click="startEdit(a)" class="text-accent hover:underline mr-3">编辑</button>
              <button @click="goToOverrides(a.agent_id)" class="text-accent hover:underline">管理监控</button>
            </template>
            <template v-else>
              <button @click="saveEdit(a.agent_id)" class="text-accent hover:underline mr-3">保存</button>
              <button @click="cancelEdit" class="text-ink-soft hover:underline">取消</button>
            </template>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
```

### 步骤 2：运行编译验证

运行：`cd web && npm run build`

预期：编译通过

### 步骤 3：Commit

```bash
git add web/src/views/AdminAgents.vue
git commit -m "feat(web): implement AdminAgents page (list + inline remark edit)"
```

---

## 任务 11：前端 - AdminAgentOverrides 页面

**文件：**
- 替换：`web/src/views/AdminAgentOverrides.vue`（完整实现）

### 步骤 1：实现 AdminAgentOverrides.vue

将 `web/src/views/AdminAgentOverrides.vue` 完整替换为：

```vue
<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { adminApi } from '../api'
import type { DomainAdmin, AgentAdmin, Override } from '../types'

const props = defineProps<{ id: string }>()
const router = useRouter()

const agent = ref<AgentAdmin | null>(null)
const domains = ref<DomainAdmin[]>([])
const overrides = ref<Override[]>([])
const loading = ref(false)
const error = ref('')
const toggling = ref<Record<string, boolean>>({})

const overrideMap = computed(() => {
  const map = new Map<number, string>()
  for (const o of overrides.value) {
    map.set(o.domain_id, o.action)
  }
  return map
})

function getStatus(d: DomainAdmin): string {
  const action = overrideMap.value.get(d.id)
  if (d.is_global) {
    return action === 'exclude' ? '已排除' : '监控'
  } else {
    return action === 'include' ? '已加入' : '不监控'
  }
}

function getDefaultStatus(d: DomainAdmin): string {
  return d.is_global ? '默认监控' : '默认不监控'
}

async function loadData() {
  loading.value = true
  try {
    const [agentsRes, domainsRes, overridesRes] = await Promise.all([
      adminApi.listAgents(),
      adminApi.listDomains(),
      adminApi.listOverrides(props.id),
    ])
    agent.value = agentsRes.agents.find(a => a.agent_id === props.id) || null
    domains.value = domainsRes.domains
    overrides.value = overridesRes.overrides
  } catch (e: any) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function toggleOverride(d: DomainAdmin) {
  const key = `${d.id}`
  if (toggling.value[key]) return
  toggling.value[key] = true
  try {
    const status = getStatus(d)
    if (status === '监控' || status === '不监控') {
      // Add override
      const action = d.is_global ? 'exclude' : 'include'
      await adminApi.setOverride(props.id, d.id, action)
    } else {
      // Remove override
      await adminApi.deleteOverride(props.id, d.id)
    }
    // Reload overrides
    const res = await adminApi.listOverrides(props.id)
    overrides.value = res.overrides
  } catch (e: any) {
    error.value = e.message
  } finally {
    toggling.value[key] = false
  }
}

function goBack() {
  router.push('/admin/agents')
}

onMounted(loadData)
</script>

<template>
  <div class="max-w-6xl mx-auto px-6 py-8">
    <button @click="goBack" class="text-sm text-ink-soft hover:text-ink mb-4">← 返回 Agent 列表</button>

    <div v-if="loading" class="text-center py-8 text-ink-soft">加载中...</div>
    <div v-else-if="!agent" class="text-center py-8 text-bad">Agent 不存在</div>
    
    <template v-else>
      <div class="mb-6 p-4 bg-bg-subtle rounded-md border border-border-soft">
        <h1 class="text-xl font-semibold text-ink mb-2">{{ agent.display_name }}</h1>
        <div class="grid grid-cols-2 gap-4 text-sm">
          <div><span class="text-ink-soft">主机名：</span><span class="text-ink">{{ agent.hostname }}</span></div>
          <div><span class="text-ink-soft">IP：</span><span class="text-ink">{{ agent.ip }}</span></div>
          <div><span class="text-ink-soft">备注：</span><span class="text-ink">{{ agent.remark || '-' }}</span></div>
          <div>
            <span class="text-ink-soft">状态：</span>
            <span v-if="agent.is_online" class="text-ok font-medium">在线</span>
            <span v-else class="text-ink-soft">离线</span>
          </div>
        </div>
      </div>

      <div v-if="error" class="mb-4 p-3 bg-red-50 border border-red-200 rounded-md">
        <p class="text-sm text-red-600">{{ error }}</p>
        <button @click="error = ''" class="text-xs text-red-500 hover:text-red-700">×</button>
      </div>

      <h2 class="text-lg font-semibold text-ink mb-4">域名监控配置</h2>

      <div v-if="domains.length === 0" class="text-center py-8 text-ink-soft">暂无域名</div>
      
      <table v-else class="w-full">
        <thead>
          <tr class="border-b border-border-soft text-left text-sm text-ink-soft">
            <th class="pb-2 font-medium">域名</th>
            <th class="pb-2 font-medium">默认状态</th>
            <th class="pb-2 font-medium">当前状态</th>
            <th class="pb-2 font-medium">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="d in domains" :key="d.id" class="border-b border-border-soft">
            <td class="py-3 text-sm text-ink">{{ d.host }}:{{ d.port }}/{{ d.protocol }}</td>
            <td class="py-3 text-sm text-ink-soft">{{ getDefaultStatus(d) }}</td>
            <td class="py-3 text-sm">
              <span v-if="getStatus(d) === '监控'" class="text-ok font-medium">监控</span>
              <span v-else-if="getStatus(d) === '已排除'" class="text-bad font-medium">已排除</span>
              <span v-else-if="getStatus(d) === '已加入'" class="text-ok font-medium">已加入</span>
              <span v-else class="text-ink-soft">不监控</span>
            </td>
            <td class="py-3 text-sm">
              <button
                @click="toggleOverride(d)"
                :disabled="toggling[d.id]"
                class="text-accent hover:underline disabled:opacity-50"
              >
                <template v-if="toggling[d.id]">处理中...</template>
                <template v-else-if="getStatus(d) === '监控'">排除</template>
                <template v-else-if="getStatus(d) === '已排除'">恢复</template>
                <template v-else-if="getStatus(d) === '不监控'">加入</template>
                <template v-else>移除</template>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </template>
  </div>
</template>
```

### 步骤 2：运行编译验证

运行：`cd web && npm run build`

预期：编译通过

### 步骤 3：Commit

```bash
git add web/src/views/AdminAgentOverrides.vue
git commit -m "feat(web): implement AdminAgentOverrides page (override matrix with toggle)"
```

---

## 任务 12：端到端验证

**验证清单：**

1. 启动 server：`cd server && go run cmd/server/main.go -config config.yaml`
2. 浏览器访问 `http://localhost:8080/login`，登录 admin/change-me
3. 验证 Header 显示"域名管理"和"Agent 管理"链接
4. 访问 `/admin/domains`：
   - 新建一个域名（host=test.com, port=443, protocol=https）
   - 编辑 remark 为"测试域名"
   - 删除该域名
5. 访问 `/admin/agents`：
   - 确认显示所有已注册 Agent
   - 编辑某个 Agent 的 remark
   - 验证在线/离线状态显示正确
6. 点击某 Agent 的"管理监控"：
   - 切换若干域名的监控状态
   - 刷新页面验证持久化
   - 使用 curl 调 `/api/agent/domains?agent_id=xxx` 验证 override 生效
7. 手动修改某 Agent 的 last_seen_at 为 5 小时前（用 sqlite3 或 mysql），验证显示"离线 5 小时前"
8. 验证未登录访问 `/admin/domains` 跳转 `/login?redirect=/admin/domains`
9. 验证 Plan 2 dashboard 仍正常（`/` 和 `/domains/:id`）

---

## 自检备忘

- 规格 §3.2 Store 方法：任务 1 ✓
- 规格 §3.3 AgentOnlineWindow 常量：任务 1 + 任务 5 ✓
- 规格 §4.1 PUT /api/admin/domains/:id：任务 2 ✓
- 规格 §4.2 GET /api/admin/agents：任务 3 ✓
- 规格 §4.3 PUT /api/admin/agents/:id：任务 3 ✓
- 规格 §4.4 GET /api/admin/agents/:id/overrides：任务 4 ✓
- 规格 §4.5 POST /api/admin/agents/:id/overrides：任务 4 ✓
- 规格 §4.6 DELETE /api/admin/agents/:id/overrides/:domain_id：任务 4 ✓
- 规格 §5.1 路由：任务 8 ✓
- 规格 §5.2 Header 导航：任务 7 ✓
- 规格 §5.3 AdminDomains 页：任务 9 ✓
- 规格 §5.4 AdminAgents 页：任务 10 ✓
- 规格 §5.5 AdminAgentOverrides 页：任务 11 ✓
- 规格 §6 端到端验证：任务 12 ✓

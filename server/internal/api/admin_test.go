package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"ssl-tracker/server/internal/store"
)

func setupRouter(t *testing.T) (*gin.Engine, *store.Store) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "admin.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&store.Agent{}, &store.Domain{}, &store.AgentDomainOverride{}, &store.CheckResult{}); err != nil {
		t.Fatal(err)
	}
	s := store.NewStore(db)

	r := gin.New()
	adm := NewAdminHandler(s)
	r.POST("/api/admin/domains", adm.CreateDomain)
	r.GET("/api/admin/domains", adm.ListDomains)
	r.GET("/api/admin/domains/:id", adm.GetDomain)
	r.PUT("/api/admin/domains/:id", adm.UpdateDomain)
	r.DELETE("/api/admin/domains/:id", adm.DeleteDomain)
	r.GET("/api/admin/agents", adm.ListAgents)
	r.PUT("/api/admin/agents/:id", adm.UpdateAgent)
	r.GET("/api/admin/agents/:id/overrides", adm.ListOverrides)
	r.POST("/api/admin/agents/:id/overrides", adm.SetOverride)
	r.DELETE("/api/admin/agents/:id/overrides/:domain_id", adm.DeleteOverride)
	return r, s
}

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

func TestUpdateDomain_MissingRemark(t *testing.T) {
	r, _ := setupRouter(t)
	body := `{"is_global": true}`
	req := httptest.NewRequest("PUT", "/api/admin/domains/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400 for missing remark, got %d", w.Code)
	}
}

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
	byID := make(map[string]bool)
	for _, a := range resp.Agents {
		byID[a.AgentID] = a.IsOnline
	}
	if !byID["a1"] {
		t.Errorf("a1 should be online")
	}
	if byID["a2"] {
		t.Errorf("a2 should be offline")
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
	// Verify snake_case field names are correctly populated
	if resp.Overrides[0].DomainID == 0 {
		t.Errorf("domain_id field not populated (expected non-zero)")
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

func TestSetOverride_ZeroDomainID(t *testing.T) {
	r, s := setupRouter(t)
	a := &store.Agent{AgentID: "a1", DisplayName: "A1", LastSeenAt: time.Now()}
	s.CreateAgent(a)
	body := `{"domain_id": 0, "action": "include"}`
	req := httptest.NewRequest("POST", "/api/admin/agents/a1/overrides", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400 for domain_id=0, got %d", w.Code)
	}
}

func TestDeleteOverride_InvalidDomainID(t *testing.T) {
	r, _ := setupRouter(t)
	req := httptest.NewRequest("DELETE", "/api/admin/agents/a1/overrides/abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400 for non-numeric domain_id, got %d", w.Code)
	}
}

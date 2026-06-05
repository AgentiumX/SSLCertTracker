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

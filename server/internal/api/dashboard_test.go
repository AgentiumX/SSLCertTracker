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

func mustOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func setupDashboardAPI(t *testing.T) (*gin.Engine, *store.Store) {
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "dash.db")
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
	h := NewDashboardHandler(s, 3*time.Hour)
	r.GET("/api/dashboard/overview", h.Overview)
	r.GET("/api/dashboard/domains", h.Domains)
	r.GET("/api/dashboard/domains/:id", h.DomainDetail)
	return r, s
}

func TestDashboardOverview(t *testing.T) {
	r, s := setupDashboardAPI(t)
	now := time.Now()
	mustOK(t, s.CreateAgent(&store.Agent{AgentID: "a1", DisplayName: "A1", RegisteredAt: now, LastSeenAt: now}))
	mustOK(t, s.CreateAgent(&store.Agent{AgentID: "a2", DisplayName: "A2", RegisteredAt: now, LastSeenAt: now.Add(-5 * time.Hour)}))
	mustOK(t, s.CreateDomain(&store.Domain{Host: "ok.com", Port: 443, Protocol: "https", IsGlobal: true}))
	mustOK(t, s.CreateDomain(&store.Domain{Host: "bad.com", Port: 443, Protocol: "https", IsGlobal: true}))
	mustOK(t, s.SaveCheckResults([]store.CheckResult{
		{AgentID: "a1", DomainID: 1, CheckedAt: now, Status: "ok"},
		{AgentID: "a1", DomainID: 2, CheckedAt: now, Status: "expired"},
	}))

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
	mustOK(t, s.CreateAgent(&store.Agent{AgentID: "a1", DisplayName: "A1", RegisteredAt: now, LastSeenAt: now}))
	mustOK(t, s.CreateAgent(&store.Agent{AgentID: "a2", DisplayName: "A2", RegisteredAt: now, LastSeenAt: now}))
	mustOK(t, s.CreateDomain(&store.Domain{Host: "x.com", Port: 443, Protocol: "https", IsGlobal: true}))
	mustOK(t, s.SaveCheckResults([]store.CheckResult{
		{AgentID: "a1", DomainID: 1, CheckedAt: now, Status: "ok"},
		{AgentID: "a2", DomainID: 1, CheckedAt: now, Status: "expired"},
	}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/dashboard/domains", nil))
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	var resp struct {
		Domains []struct {
			ID           uint   `json:"id"`
			Host         string `json:"host"`
			Port         int    `json:"port"`
			HealthyCount int    `json:"healthy_count"`
			TotalChecks  int    `json:"total_checks"`
			WorstStatus  string `json:"worst_status"`
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
	mustOK(t, s.CreateAgent(&store.Agent{AgentID: "a1", DisplayName: "Beijing", RegisteredAt: now, LastSeenAt: now}))
	mustOK(t, s.CreateDomain(&store.Domain{Host: "d.com", Port: 443, Protocol: "https", IsGlobal: true}))
	mustOK(t, s.SaveCheckResults([]store.CheckResult{
		{AgentID: "a1", DomainID: 1, CheckedAt: now, Status: "ok", NotAfter: &notAfter, Issuer: "LE"},
	}))

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

package api

import (
	"bytes"
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

func setupTestAPI(t *testing.T) (*gin.Engine, *store.Store) {
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB, _ := db.DB(); sqlDB.Close() })
	db.AutoMigrate(&store.Agent{}, &store.Domain{}, &store.AgentDomainOverride{}, &store.CheckResult{})
	s := store.NewStore(db)

	r := gin.New()
	ah := NewAgentHandler(s, 15)
	r.POST("/api/agent/register", ah.Register)
	r.GET("/api/agent/domains", ah.GetDomains)
	r.POST("/api/agent/results", ah.PostResults)
	adm := NewAdminHandler(s)
	r.POST("/api/admin/domains", adm.CreateDomain)
	r.GET("/api/admin/domains", adm.ListDomains)
	r.GET("/api/admin/domains/:id", adm.GetDomain)
	r.DELETE("/api/admin/domains/:id", adm.DeleteDomain)
	return r, s
}

func TestRegister(t *testing.T) {
	r, s := setupTestAPI(t)
	body, _ := json.Marshal(map[string]string{
		"agent_id": "a001", "display_name": "Beijing-01", "hostname": "h1", "ip": "10.0.0.1",
	})
	req := httptest.NewRequest("POST", "/api/agent/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	agent, err := s.GetAgent("a001")
	if err != nil || agent.DisplayName != "Beijing-01" {
		t.Errorf("agent not saved correctly: %v", err)
	}
}

func TestGetDomains(t *testing.T) {
	r, s := setupTestAPI(t)
	s.CreateAgent(&store.Agent{AgentID: "a1", DisplayName: "A1", Hostname: "h1", IP: "1.1.1.1", RegisteredAt: time.Now(), LastSeenAt: time.Now()})
	s.CreateDomain(&store.Domain{Host: "global1.com", Port: 443, Protocol: "https", IsGlobal: true})
	s.CreateDomain(&store.Domain{Host: "global2.com", Port: 443, Protocol: "https", IsGlobal: true})
	d3 := &store.Domain{Host: "extra.com", Port: 443, Protocol: "https", IsGlobal: false}
	s.CreateDomain(d3)
	s.CreateOverride(&store.AgentDomainOverride{AgentID: "a1", DomainID: d3.ID, Action: "include"})

	req := httptest.NewRequest("GET", "/api/agent/domains?agent_id=a1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	var resp struct {
		Domains []map[string]interface{} `json:"domains"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Domains) != 3 {
		t.Errorf("expected 3 domains, got %d", len(resp.Domains))
	}
}

func TestGetDomains_UpdatesLastSeen(t *testing.T) {
	r, s := setupTestAPI(t)
	oldTime := time.Now().Add(-1 * time.Hour)
	s.CreateAgent(&store.Agent{AgentID: "a2", DisplayName: "A2", Hostname: "h2", IP: "2.2.2.2", RegisteredAt: oldTime, LastSeenAt: oldTime})

	req := httptest.NewRequest("GET", "/api/agent/domains?agent_id=a2", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	agent, _ := s.GetAgent("a2")
	if !agent.LastSeenAt.After(oldTime) {
		t.Errorf("LastSeenAt not updated")
	}
}

func TestPostResults_ReclassifiesExpiring(t *testing.T) {
	r, _ := setupTestAPI(t)
	now := time.Now()
	notAfter := now.Add(10 * 24 * time.Hour) // 10 days < threshold 15 → expiring
	payload := map[string]interface{}{
		"agent_id": "a1",
		"results": []map[string]interface{}{{
			"domain_id": 1, "checked_at": now.Format(time.RFC3339),
			"status": "ok", "not_after": notAfter.Format(time.RFC3339),
		}},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/agent/results", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	var resp struct{ Accepted int `json:"accepted"` }
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Accepted != 1 {
		t.Errorf("expected accepted=1, got %d", resp.Accepted)
	}
}

func TestAdminCreateListDomain(t *testing.T) {
	r, _ := setupTestAPI(t)
	body, _ := json.Marshal(map[string]interface{}{
		"host": "example.com", "port": 443, "protocol": "https", "is_global": true,
	})
	req := httptest.NewRequest("POST", "/api/admin/domains", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body)
	}
	var created struct{ ID uint `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &created)
	if created.ID == 0 {
		t.Errorf("expected ID > 0")
	}

	req2 := httptest.NewRequest("GET", "/api/admin/domains", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	var list struct{ Domains []interface{} `json:"domains"` }
	json.Unmarshal(w2.Body.Bytes(), &list)
	if len(list.Domains) != 1 {
		t.Errorf("expected 1 domain, got %d", len(list.Domains))
	}
}

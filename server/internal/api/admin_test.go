package api

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

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

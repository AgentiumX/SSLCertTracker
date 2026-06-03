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

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

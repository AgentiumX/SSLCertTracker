package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"ssl-tracker/server/internal/store"
)

func setupAlertChannelRouter(t *testing.T) (*gin.Engine, *store.Store) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "alert.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&store.AlertChannel{}); err != nil {
		t.Fatal(err)
	}
	s := store.NewStore(db)

	r := gin.New()
	h := NewAlertChannelHandler(s)
	r.POST("/api/admin/alert-channels", h.CreateChannel)
	r.GET("/api/admin/alert-channels", h.ListChannels)
	r.GET("/api/admin/alert-channels/:id", h.GetChannel)
	r.PUT("/api/admin/alert-channels/:id", h.UpdateChannel)
	r.DELETE("/api/admin/alert-channels/:id", h.DeleteChannel)
	r.POST("/api/admin/alert-channels/:id/test", h.TestChannel)
	return r, s
}

func TestCreateChannel_Success(t *testing.T) {
	r, _ := setupAlertChannelRouter(t)
	body := `{"name":"Test","type":"webhook","config":"{\"url\":\"https://test.com\"}","enabled":true}`
	req := httptest.NewRequest("POST", "/api/admin/alert-channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Errorf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestCreateChannel_InvalidType(t *testing.T) {
	r, _ := setupAlertChannelRouter(t)
	body := `{"name":"Test","type":"invalid","config":"{}","enabled":true}`
	req := httptest.NewRequest("POST", "/api/admin/alert-channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateChannel_InvalidConfig_JSON(t *testing.T) {
	r, _ := setupAlertChannelRouter(t)
	body := `{"name":"Test","type":"webhook","config":"not json","enabled":true}`
	req := httptest.NewRequest("POST", "/api/admin/alert-channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateChannel_InvalidConfig_MissingField(t *testing.T) {
	r, _ := setupAlertChannelRouter(t)
	body := `{"name":"Test","type":"webhook","config":"{}","enabled":true}`
	req := httptest.NewRequest("POST", "/api/admin/alert-channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListChannels_NoConfig(t *testing.T) {
	r, s := setupAlertChannelRouter(t)
	s.CreateAlertChannel(&store.AlertChannel{Name: "Ch1", Type: "webhook", Config: `{"url":"https://secret.com"}`, Enabled: true})
	req := httptest.NewRequest("GET", "/api/admin/alert-channels", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "secret.com") {
		t.Errorf("list should not contain config field")
	}
}

func TestGetChannel_WithConfig(t *testing.T) {
	r, s := setupAlertChannelRouter(t)
	ch := &store.AlertChannel{Name: "Ch1", Type: "webhook", Config: `{"url":"https://test.com"}`, Enabled: true}
	s.CreateAlertChannel(ch)
	req := httptest.NewRequest("GET", "/api/admin/alert-channels/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "test.com") {
		t.Errorf("get should contain config field")
	}
}

func TestUpdateChannel_Success(t *testing.T) {
	r, s := setupAlertChannelRouter(t)
	ch := &store.AlertChannel{Name: "Old", Type: "webhook", Config: `{"url":"https://old.com"}`, Enabled: true}
	s.CreateAlertChannel(ch)
	body := `{"name":"New","type":"dingtalk","config":"{\"url\":\"https://new.com\"}","enabled":false}`
	req := httptest.NewRequest("PUT", "/api/admin/alert-channels/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUpdateChannel_NotFound(t *testing.T) {
	r, _ := setupAlertChannelRouter(t)
	body := `{"name":"X","type":"webhook","config":"{\"url\":\"https://x.com\"}","enabled":true}`
	req := httptest.NewRequest("PUT", "/api/admin/alert-channels/999", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteChannel_Success(t *testing.T) {
	r, s := setupAlertChannelRouter(t)
	ch := &store.AlertChannel{Name: "ToDelete", Type: "webhook", Config: `{"url":"https://example.com"}`, Enabled: true}
	s.CreateAlertChannel(ch)
	req := httptest.NewRequest("DELETE", "/api/admin/alert-channels/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestTestChannel_Success(t *testing.T) {
	// httptest server to receive webhook
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	r, s := setupAlertChannelRouter(t)
	ch := &store.AlertChannel{Name: "Test", Type: "webhook", Config: `{"url":"` + ts.URL + `"}`, Enabled: true}
	s.CreateAlertChannel(ch)
	req := httptest.NewRequest("POST", "/api/admin/alert-channels/1/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTestChannel_SendFailed(t *testing.T) {
	r, s := setupAlertChannelRouter(t)
	ch := &store.AlertChannel{Name: "Test", Type: "webhook", Config: `{"url":"https://invalid.domain.that.does.not.exist"}`, Enabled: true}
	s.CreateAlertChannel(ch)
	req := httptest.NewRequest("POST", "/api/admin/alert-channels/1/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestTestChannel_NotFound(t *testing.T) {
	r, _ := setupAlertChannelRouter(t)
	req := httptest.NewRequest("POST", "/api/admin/alert-channels/999/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetChannel_NotFound(t *testing.T) {
	r, _ := setupAlertChannelRouter(t)
	req := httptest.NewRequest("GET", "/api/admin/alert-channels/999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteChannel_NotFound(t *testing.T) {
	r, _ := setupAlertChannelRouter(t)
	req := httptest.NewRequest("DELETE", "/api/admin/alert-channels/999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestCreateChannel_MissingRequiredField(t *testing.T) {
	r, _ := setupAlertChannelRouter(t)
	// Missing "name" field (required)
	body := `{"type":"webhook","config":"{\"url\":\"https://test.com\"}","enabled":true}`
	req := httptest.NewRequest("POST", "/api/admin/alert-channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

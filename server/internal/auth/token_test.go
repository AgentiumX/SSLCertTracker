package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAgentTokenMiddleware_Valid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AgentTokenMiddleware("test-token"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAgentTokenMiddleware_Invalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AgentTokenMiddleware("correct"))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	for _, tc := range []struct{ header string }{
		{"Bearer wrong"},
		{""},
		{"Token correct"},
	} {
		req := httptest.NewRequest("GET", "/test", nil)
		if tc.header != "" {
			req.Header.Set("Authorization", tc.header)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Errorf("header=%q: expected 401, got %d", tc.header, w.Code)
		}
	}
}

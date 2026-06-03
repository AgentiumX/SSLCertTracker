package web_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"ssl-tracker/server/internal/web"
)

func TestHandler_RootReturnsIndex(t *testing.T) {
	h := web.Handler()
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `<div id="app">`) {
		t.Fatalf("body does not contain SPA mount point: %s", rr.Body.String())
	}
}

func TestHandler_SPAFallback(t *testing.T) {
	h := web.Handler()
	req := httptest.NewRequest("GET", "/some/spa/route", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `<div id="app">`) {
		t.Fatalf("body does not contain SPA mount point: %s", rr.Body.String())
	}
}

func TestHandler_IndexDirect(t *testing.T) {
	h := web.Handler()
	req := httptest.NewRequest("GET", "/index.html", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d; body=%s", rr.Code, rr.Body.String())
	}
}

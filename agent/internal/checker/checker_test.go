package checker

import (
	"context"
	"testing"
	"time"
)

func TestCheckDomain_InvalidHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := CheckDomain(ctx, "invalid-host-that-does-not-exist-12345.com", 443, "https")
	if result.Status != "unreachable" {
		t.Errorf("expected unreachable, got %s", result.Status)
	}
	if result.ErrorMessage == "" {
		t.Errorf("expected error message")
	}
}

func TestCheckDomain_ValidCert(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result := CheckDomain(ctx, "example.com", 443, "https")
	if result.Status == "unreachable" {
		t.Skipf("network issue: %s", result.ErrorMessage)
	}
	if result.Status != "ok" && result.Status != "expiring" && result.Status != "expired" {
		t.Errorf("unexpected status: %s", result.Status)
	}
	if result.NotAfter == nil {
		t.Errorf("expected NotAfter to be set")
	}
}

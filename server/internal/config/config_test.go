package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	yml := `
server:
  listen: ":9090"
auth:
  agent_token: "test-token-123"
  admin_username: "admin"
  admin_password: "pass123"
database:
  type: sqlite
  sqlite:
    path: "./test.db"
retention:
  history_days: 7
alert:
  expire_threshold_days: 15
  daily_reminder_time: "09:00"
  daily_reminder_timezone: "Asia/Shanghai"
session:
  secret: "test-secret"
  ttl: "24h"
`
	f, _ := os.CreateTemp("", "config*.yaml")
	defer os.Remove(f.Name())
	f.WriteString(yml)
	f.Close()

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Server.Listen != ":9090" {
		t.Errorf("expected :9090, got %s", cfg.Server.Listen)
	}
	if cfg.Auth.AgentToken != "test-token-123" {
		t.Errorf("token mismatch")
	}
	if cfg.Database.Type != "sqlite" {
		t.Errorf("expected sqlite, got %s", cfg.Database.Type)
	}
}

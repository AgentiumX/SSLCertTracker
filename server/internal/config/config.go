package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Auth      AuthConfig      `yaml:"auth"`
	Database  DatabaseConfig  `yaml:"database"`
	Retention RetentionConfig `yaml:"retention"`
	Alert     AlertConfig     `yaml:"alert"`
	Session   SessionConfig   `yaml:"session"`
}

type ServerConfig struct {
	Listen string `yaml:"listen"`
}

type AuthConfig struct {
	AgentToken    string `yaml:"agent_token"`
	AdminUsername string `yaml:"admin_username"`
	AdminPassword string `yaml:"admin_password"`
}

type DatabaseConfig struct {
	Type   string       `yaml:"type"`
	SQLite SQLiteConfig `yaml:"sqlite"`
	MySQL  MySQLConfig  `yaml:"mysql"`
}

type SQLiteConfig struct {
	Path string `yaml:"path"`
}

type MySQLConfig struct {
	DSN string `yaml:"dsn"`
}

type RetentionConfig struct {
	HistoryDays int `yaml:"history_days"`
}

type AlertConfig struct {
	ExpireThresholdDays   int    `yaml:"expire_threshold_days"`
	DailyReminderTime     string `yaml:"daily_reminder_time"`
	DailyReminderTimezone string `yaml:"daily_reminder_timezone"`
}

type SessionConfig struct {
	Secret string `yaml:"secret"`
	TTL    string `yaml:"ttl"`
	Secure bool   `yaml:"secure"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

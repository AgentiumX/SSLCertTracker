package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ServerURL string      `yaml:"server_url"`
	AuthToken string      `yaml:"auth_token"`
	Agent     AgentConfig `yaml:"agent"`
	Check     CheckConfig `yaml:"check"`
}

type AgentConfig struct {
	DisplayName string `yaml:"display_name"`
	IDFile      string `yaml:"id_file"`
}

type CheckConfig struct {
	Interval    string `yaml:"interval"`
	Timeout     string `yaml:"timeout"`
	Concurrency int    `yaml:"concurrency"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	return &cfg, yaml.Unmarshal(data, &cfg)
}

func (c *CheckConfig) IntervalDuration() time.Duration {
	d, _ := time.ParseDuration(c.Interval)
	return d
}

func (c *CheckConfig) TimeoutDuration() time.Duration {
	d, _ := time.ParseDuration(c.Timeout)
	return d
}

package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type WebhookChannel struct {
	config string
}

type webhookConfig struct {
	URL string `json:"url"`
}

func (c *WebhookChannel) Send(ctx context.Context, msg Message) error {
	var cfg webhookConfig
	if err := json.Unmarshal([]byte(c.config), &cfg); err != nil {
		return err
	}
	payload := map[string]string{"title": msg.Title, "body": msg.Body}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", cfg.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func (c *WebhookChannel) Test() error {
	return c.Send(context.Background(), buildTestMessage())
}

func (c *WebhookChannel) ValidateConfig() error {
	var cfg webhookConfig
	if err := json.Unmarshal([]byte(c.config), &cfg); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("url is required")
	}
	return nil
}

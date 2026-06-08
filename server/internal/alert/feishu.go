package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type FeishuChannel struct {
	config string
}

type feishuConfig struct {
	URL string `json:"url"`
}

func (c *FeishuChannel) Send(ctx context.Context, msg Message) error {
	var cfg feishuConfig
	if err := json.Unmarshal([]byte(c.config), &cfg); err != nil {
		return err
	}
	payload := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]string{
					"tag":     "plain_text",
					"content": msg.Title,
				},
			},
			"elements": []map[string]string{
				{
					"tag":     "markdown",
					"content": msg.Body,
				},
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("feishu returned status %d", resp.StatusCode)
	}
	return nil
}

func (c *FeishuChannel) Test() error {
	return c.Send(context.Background(), buildTestMessage())
}

func (c *FeishuChannel) ValidateConfig() error {
	var cfg feishuConfig
	if err := json.Unmarshal([]byte(c.config), &cfg); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("url is required")
	}
	return nil
}

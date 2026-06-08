package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type WecomChannel struct {
	config string
}

type wecomConfig struct {
	URL string `json:"url"`
}

func (c *WecomChannel) Send(ctx context.Context, msg Message) error {
	var cfg wecomConfig
	if err := json.Unmarshal([]byte(c.config), &cfg); err != nil {
		return err
	}
	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": fmt.Sprintf("**%s**\n%s", msg.Title, msg.Body),
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
		return fmt.Errorf("wecom returned status %d", resp.StatusCode)
	}
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("wecom: failed to decode response: %w", err)
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("wecom error %d: %s", result.ErrCode, result.ErrMsg)
	}
	return nil
}

func (c *WecomChannel) Test() error {
	return c.Send(context.Background(), buildTestMessage())
}

func (c *WecomChannel) ValidateConfig() error {
	var cfg wecomConfig
	if err := json.Unmarshal([]byte(c.config), &cfg); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("url is required")
	}
	return nil
}

package alert

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DingtalkChannel struct {
	config string
}

type dingtalkConfig struct {
	URL    string `json:"url"`
	Secret string `json:"secret"`
}

func (c *DingtalkChannel) Send(ctx context.Context, msg Message) error {
	var cfg dingtalkConfig
	if err := json.Unmarshal([]byte(c.config), &cfg); err != nil {
		return err
	}

	reqURL := cfg.URL
	if cfg.Secret != "" {
		timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
		stringToSign := timestamp + "\n" + cfg.Secret
		h := hmac.New(sha256.New, []byte(cfg.Secret))
		h.Write([]byte(stringToSign))
		sign := url.QueryEscape(base64.StdEncoding.EncodeToString(h.Sum(nil)))
		sep := "?"
		if strings.Contains(reqURL, "?") {
			sep = "&"
		}
		reqURL = fmt.Sprintf("%s%stimestamp=%s&sign=%s", reqURL, sep, timestamp, sign)
	}

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": msg.Title,
			"text":  msg.Body,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(body))
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
		return fmt.Errorf("dingtalk returned status %d", resp.StatusCode)
	}
	return nil
}

func (c *DingtalkChannel) Test() error {
	return c.Send(context.Background(), buildTestMessage())
}

func (c *DingtalkChannel) ValidateConfig() error {
	var cfg dingtalkConfig
	if err := json.Unmarshal([]byte(c.config), &cfg); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("url is required")
	}
	return nil
}

# Plan 3.3a：告警渠道管理 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 实现告警渠道的 CRUD 管理和测试发送功能，支持 5 种渠道（webhook / dingtalk / feishu / wecom / email）

**架构：** 后端使用统一 Channel 接口 + 独立适配器文件，Store 层提供 CRUD，API 层暴露 REST 端点；前端 `/admin/channels` 页面提供管理界面

**技术栈：** Go + Gin + GORM（后端），Vue 3 + TypeScript + Tailwind CSS（前端）

---

## 任务 1：Store 层 - AlertChannel 模型 + CRUD 方法

**文件：**
- 修改：`server/internal/store/models.go`（新增 AlertChannel struct）
- 修改：`server/internal/store/store.go`（新增 5 个 CRUD 方法）
- 测试：`server/internal/store/alert_channel_test.go`（新建）

- [ ] **步骤 1：编写失败的测试**

创建 `server/internal/store/alert_channel_test.go`：

```go
package store

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestCreateAlertChannel(t *testing.T) {
	s := setupTestDB(t)
	ch := &AlertChannel{
		Name:    "Test Webhook",
		Type:    "webhook",
		Config:  `{"url":"https://example.com/hook"}`,
		Enabled: true,
	}
	if err := s.CreateAlertChannel(ch); err != nil {
		t.Fatalf("CreateAlertChannel: %v", err)
	}
	if ch.ID == 0 {
		t.Errorf("expected ID > 0, got %d", ch.ID)
	}
}

func TestListAlertChannels(t *testing.T) {
	s := setupTestDB(t)
	s.CreateAlertChannel(&AlertChannel{Name: "Ch1", Type: "webhook", Config: "{}", Enabled: true})
	s.CreateAlertChannel(&AlertChannel{Name: "Ch2", Type: "dingtalk", Config: "{}", Enabled: false})
	channels, err := s.ListAlertChannels()
	if err != nil {
		t.Fatalf("ListAlertChannels: %v", err)
	}
	if len(channels) != 2 {
		t.Errorf("expected 2 channels, got %d", len(channels))
	}
}

func TestGetAlertChannel(t *testing.T) {
	s := setupTestDB(t)
	ch := &AlertChannel{Name: "Test", Type: "webhook", Config: `{"url":"https://test.com"}`, Enabled: true}
	s.CreateAlertChannel(ch)
	got, err := s.GetAlertChannel(ch.ID)
	if err != nil {
		t.Fatalf("GetAlertChannel: %v", err)
	}
	if got.Name != "Test" || got.Config != `{"url":"https://test.com"}` {
		t.Errorf("unexpected channel: %+v", got)
	}
}

func TestGetAlertChannel_NotFound(t *testing.T) {
	s := setupTestDB(t)
	_, err := s.GetAlertChannel(9999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestUpdateAlertChannel(t *testing.T) {
	s := setupTestDB(t)
	ch := &AlertChannel{Name: "Old", Type: "webhook", Config: "{}", Enabled: true}
	s.CreateAlertChannel(ch)
	if err := s.UpdateAlertChannel(ch.ID, "New", "dingtalk", `{"url":"https://new.com"}`, false); err != nil {
		t.Fatalf("UpdateAlertChannel: %v", err)
	}
	got, _ := s.GetAlertChannel(ch.ID)
	if got.Name != "New" || got.Type != "dingtalk" || got.Config != `{"url":"https://new.com"}` || got.Enabled != false {
		t.Errorf("update failed: %+v", got)
	}
}

func TestUpdateAlertChannel_NotFound(t *testing.T) {
	s := setupTestDB(t)
	err := s.UpdateAlertChannel(9999, "X", "webhook", "{}", true)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestDeleteAlertChannel(t *testing.T) {
	s := setupTestDB(t)
	ch := &AlertChannel{Name: "ToDelete", Type: "webhook", Config: "{}", Enabled: true}
	s.CreateAlertChannel(ch)
	if err := s.DeleteAlertChannel(ch.ID); err != nil {
		t.Fatalf("DeleteAlertChannel: %v", err)
	}
	_, err := s.GetAlertChannel(ch.ID)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected ErrRecordNotFound after delete, got %v", err)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd server && go test ./internal/store -run "TestCreateAlertChannel|TestListAlertChannels|TestGetAlertChannel|TestUpdateAlertChannel|TestDeleteAlertChannel" -v`

预期：编译失败，`AlertChannel not defined`

- [ ] **步骤 3：实现 AlertChannel 模型和 CRUD 方法**

在 `server/internal/store/models.go` 末尾追加：

```go
type AlertChannel struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"not null"`
	Type      string `gorm:"not null"`
	Config    string `gorm:"not null;type:text"`
	Enabled   bool   `gorm:"not null;default:true"`
	CreatedAt time.Time
}
```

在 `server/internal/store/store.go` 末尾追加：

```go
// AlertChannel operations

func (s *Store) CreateAlertChannel(ch *AlertChannel) error {
	return s.db.Create(ch).Error
}

func (s *Store) ListAlertChannels() ([]AlertChannel, error) {
	var channels []AlertChannel
	err := s.db.Find(&channels).Error
	return channels, err
}

func (s *Store) GetAlertChannel(id uint) (*AlertChannel, error) {
	var ch AlertChannel
	err := s.db.First(&ch, id).Error
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

func (s *Store) UpdateAlertChannel(id uint, name, channelType, config string, enabled bool) error {
	res := s.db.Model(&AlertChannel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"name":    name,
		"type":    channelType,
		"config":  config,
		"enabled": enabled,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) DeleteAlertChannel(id uint) error {
	res := s.db.Delete(&AlertChannel{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd server && go test ./internal/store -run "TestCreateAlertChannel|TestListAlertChannels|TestGetAlertChannel|TestUpdateAlertChannel|TestDeleteAlertChannel" -v`

预期：7 个测试全部 PASS

- [ ] **步骤 5：Commit**

```bash
git add server/internal/store/models.go server/internal/store/store.go server/internal/store/alert_channel_test.go
git commit -m "feat(store): add AlertChannel model and CRUD methods"
```

---

## 任务 2：Alert 包 - Channel 接口 + Message + 工厂函数

**文件：**
- 创建：`server/internal/alert/channel.go`（接口 + Message + NewChannel 工厂）
- 测试：`server/internal/alert/channel_test.go`（新建）

- [ ] **步骤 1：编写失败的测试**

创建 `server/internal/alert/channel_test.go`：

```go
package alert

import "testing"

func TestNewChannel_Webhook(t *testing.T) {
	ch, err := NewChannel("webhook", `{"url":"https://example.com"}`)
	if err != nil {
		t.Fatalf("NewChannel webhook: %v", err)
	}
	if ch == nil {
		t.Error("expected non-nil channel")
	}
}

func TestNewChannel_Dingtalk(t *testing.T) {
	ch, err := NewChannel("dingtalk", `{"url":"https://oapi.dingtalk.com/robot/send?access_token=xxx"}`)
	if err != nil {
		t.Fatalf("NewChannel dingtalk: %v", err)
	}
	if ch == nil {
		t.Error("expected non-nil channel")
	}
}

func TestNewChannel_Feishu(t *testing.T) {
	ch, err := NewChannel("feishu", `{"url":"https://open.feishu.cn/open-apis/bot/v2/hook/xxx"}`)
	if err != nil {
		t.Fatalf("NewChannel feishu: %v", err)
	}
	if ch == nil {
		t.Error("expected non-nil channel")
	}
}

func TestNewChannel_Wecom(t *testing.T) {
	ch, err := NewChannel("wecom", `{"url":"https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"}`)
	if err != nil {
		t.Fatalf("NewChannel wecom: %v", err)
	}
	if ch == nil {
		t.Error("expected non-nil channel")
	}
}

func TestNewChannel_Email(t *testing.T) {
	ch, err := NewChannel("email", `{"smtp_host":"smtp.example.com","smtp_port":587,"username":"user","password":"pass","from":"a@b.com","to":["c@d.com"]}`)
	if err != nil {
		t.Fatalf("NewChannel email: %v", err)
	}
	if ch == nil {
		t.Error("expected non-nil channel")
	}
}

func TestNewChannel_InvalidType(t *testing.T) {
	_, err := NewChannel("invalid", `{}`)
	if err == nil {
		t.Error("expected error for invalid type")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd server && go test ./internal/alert -run "TestNewChannel" -v`

预期：编译失败，`NewChannel not defined`

- [ ] **步骤 3：实现 Channel 接口和工厂函数**

创建 `server/internal/alert/channel.go`：

```go
package alert

import (
	"fmt"
	"time"
)

// Message is the unified message payload sent through all channels.
type Message struct {
	Title string
	Body  string
}

// Channel is the interface all alert channel adapters must implement.
type Channel interface {
	// Send sends a message through the channel.
	Send(msg Message) error
	// Test sends a fixed test message to verify the channel configuration.
	Test() error
	// ValidateConfig validates the channel's JSON config string.
	ValidateConfig() error
}

// NewChannel creates a Channel adapter based on the channel type and config JSON.
func NewChannel(channelType string, configJSON string) (Channel, error) {
	switch channelType {
	case "webhook":
		return &WebhookChannel{config: configJSON}, nil
	case "dingtalk":
		return &DingtalkChannel{config: configJSON}, nil
	case "feishu":
		return &FeishuChannel{config: configJSON}, nil
	case "wecom":
		return &WecomChannel{config: configJSON}, nil
	case "email":
		return &EmailChannel{config: configJSON}, nil
	default:
		return nil, fmt.Errorf("unsupported channel type: %s", channelType)
	}
}

// buildTestMessage creates a standard test message for channel verification.
func buildTestMessage() Message {
	return Message{
		Title: "[SSL Tracker] 告警渠道测试",
		Body:  fmt.Sprintf("这是一条测试消息。如果您收到此消息，说明告警渠道配置正确。\n\n时间: %s", time.Now().Format(time.RFC3339)),
	}
}
```

- [ ] **步骤 4：创建占位适配器（后续任务实现）**

创建 `server/internal/alert/webhook.go`：

```go
package alert

type WebhookChannel struct {
	config string
}

func (c *WebhookChannel) Send(msg Message) error {
	return nil // TODO: implement in task 3
}

func (c *WebhookChannel) Test() error {
	return c.Send(buildTestMessage())
}

func (c *WebhookChannel) ValidateConfig() error {
	return nil // TODO: implement in task 3
}
```

类似创建 `dingtalk.go`、`feishu.go`、`wecom.go`、`email.go`（结构相同，类型名不同）。

- [ ] **步骤 5：运行测试验证通过**

运行：`cd server && go test ./internal/alert -run "TestNewChannel" -v`

预期：6 个测试全部 PASS

- [ ] **步骤 6：Commit**

```bash
git add server/internal/alert/
git commit -m "feat(alert): add Channel interface, Message, and factory function"
```

---

## 任务 3：Alert 包 - Webhook 适配器

**文件：**
- 修改：`server/internal/alert/webhook.go`（完整实现）
- 测试：`server/internal/alert/channel_test.go`（追加测试）

- [ ] **步骤 1：追加失败的测试**

在 `server/internal/alert/channel_test.go` 末尾追加：

```go
func TestValidateConfig_Webhook_Valid(t *testing.T) {
	ch := &WebhookChannel{config: `{"url":"https://example.com/hook"}`}
	if err := ch.ValidateConfig(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestValidateConfig_Webhook_Invalid_MissingURL(t *testing.T) {
	ch := &WebhookChannel{config: `{}`}
	if err := ch.ValidateConfig(); err == nil {
		t.Error("expected error for missing url")
	}
}

func TestValidateConfig_Webhook_Invalid_JSON(t *testing.T) {
	ch := &WebhookChannel{config: `not json`}
	if err := ch.ValidateConfig(); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestWebhook_Send(t *testing.T) {
	// httptest server to receive webhook
	var received map[string]string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json")
		}
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer ts.Close()

	ch := &WebhookChannel{config: fmt.Sprintf(`{"url":"%s"}`, ts.URL)}
	err := ch.Send(Message{Title: "Test", Body: "Body"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if received["title"] != "Test" || received["body"] != "Body" {
		t.Errorf("unexpected payload: %+v", received)
	}
}
```

在文件顶部添加 imports：`"encoding/json"`, `"fmt"`, `"net/http"`, `"net/http/httptest"`

- [ ] **步骤 2：运行测试验证失败**

运行：`cd server && go test ./internal/alert -run "TestValidateConfig_Webhook|TestWebhook_Send" -v`

预期：测试失败（ValidateConfig 和 Send 未实现）

- [ ] **步骤 3：实现 WebhookChannel**

替换 `server/internal/alert/webhook.go`：

```go
package alert

import (
	"bytes"
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

func (c *WebhookChannel) Send(msg Message) error {
	var cfg webhookConfig
	if err := json.Unmarshal([]byte(c.config), &cfg); err != nil {
		return err
	}
	payload := map[string]string{"title": msg.Title, "body": msg.Body}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(cfg.URL, "application/json", bytes.NewReader(body))
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
	return c.Send(buildTestMessage())
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
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd server && go test ./internal/alert -run "TestValidateConfig_Webhook|TestWebhook_Send" -v`

预期：4 个测试全部 PASS

- [ ] **步骤 5：Commit**

```bash
git add server/internal/alert/webhook.go server/internal/alert/channel_test.go
git commit -m "feat(alert): implement webhook channel adapter"
```

---

## 任务 4：Alert 包 - 钉钉适配器

**文件：**
- 修改：`server/internal/alert/dingtalk.go`（完整实现）
- 测试：`server/internal/alert/channel_test.go`（追加测试）

- [ ] **步骤 1：追加失败的测试**

在 `server/internal/alert/channel_test.go` 末尾追加：

```go
func TestValidateConfig_Dingtalk_Valid(t *testing.T) {
	ch := &DingtalkChannel{config: `{"url":"https://oapi.dingtalk.com/robot/send?access_token=xxx","secret":"SEC..."}`}
	if err := ch.ValidateConfig(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestValidateConfig_Dingtalk_Valid_NoSecret(t *testing.T) {
	ch := &DingtalkChannel{config: `{"url":"https://oapi.dingtalk.com/robot/send?access_token=xxx"}`}
	if err := ch.ValidateConfig(); err != nil {
		t.Errorf("expected valid config without secret, got error: %v", err)
	}
}

func TestValidateConfig_Dingtalk_Invalid_MissingURL(t *testing.T) {
	ch := &DingtalkChannel{config: `{"secret":"SEC..."}`}
	if err := ch.ValidateConfig(); err == nil {
		t.Error("expected error for missing url")
	}
}

func TestDingtalk_Send_NoSecret(t *testing.T) {
	var received map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer ts.Close()

	ch := &DingtalkChannel{config: fmt.Sprintf(`{"url":"%s"}`, ts.URL)}
	err := ch.Send(Message{Title: "Alert", Body: "Test body"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if received["msgtype"] != "markdown" {
		t.Errorf("expected msgtype=markdown, got %+v", received)
	}
}

func TestDingtalk_Send_WithSecret(t *testing.T) {
	var receivedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		w.WriteHeader(200)
	}))
	defer ts.Close()

	ch := &DingtalkChannel{config: fmt.Sprintf(`{"url":"%s","secret":"SEC123"}`, ts.URL)}
	err := ch.Send(Message{Title: "Test", Body: "Body"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	// Verify timestamp and sign parameters are present
	if !contains(receivedURL, "timestamp=") || !contains(receivedURL, "sign=") {
		t.Errorf("expected timestamp and sign in URL, got %s", receivedURL)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd server && go test ./internal/alert -run "TestValidateConfig_Dingtalk|TestDingtalk_Send" -v`

预期：测试失败

- [ ] **步骤 3：实现 DingtalkChannel**

替换 `server/internal/alert/dingtalk.go`：

```go
package alert

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type DingtalkChannel struct {
	config string
}

type dingtalkConfig struct {
	URL    string `json:"url"`
	Secret string `json:"secret"`
}

func (c *DingtalkChannel) Send(msg Message) error {
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
		if contains(reqURL, "?") {
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
	body, _ := json.Marshal(payload)
	resp, err := http.Post(reqURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("dingtalk returned status %d", resp.StatusCode)
	}
	return nil
}

func (c *DingtalkChannel) Test() error {
	return c.Send(buildTestMessage())
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
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd server && go test ./internal/alert -run "TestValidateConfig_Dingtalk|TestDingtalk_Send" -v`

预期：5 个测试全部 PASS

- [ ] **步骤 5：Commit**

```bash
git add server/internal/alert/dingtalk.go server/internal/alert/channel_test.go
git commit -m "feat(alert): implement dingtalk channel adapter with HMAC-SHA256 signing"
```

---

## 任务 5：Alert 包 - 飞书适配器

**文件：**
- 修改：`server/internal/alert/feishu.go`（完整实现）
- 测试：`server/internal/alert/channel_test.go`（追加测试）

- [ ] **步骤 1：追加失败的测试**

在 `server/internal/alert/channel_test.go` 末尾追加：

```go
func TestValidateConfig_Feishu_Valid(t *testing.T) {
	ch := &FeishuChannel{config: `{"url":"https://open.feishu.cn/open-apis/bot/v2/hook/xxx"}`}
	if err := ch.ValidateConfig(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestValidateConfig_Feishu_Invalid_MissingURL(t *testing.T) {
	ch := &FeishuChannel{config: `{}`}
	if err := ch.ValidateConfig(); err == nil {
		t.Error("expected error for missing url")
	}
}

func TestFeishu_Send(t *testing.T) {
	var received map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer ts.Close()

	ch := &FeishuChannel{config: fmt.Sprintf(`{"url":"%s"}`, ts.URL)}
	err := ch.Send(Message{Title: "Alert", Body: "Test"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if received["msg_type"] != "interactive" {
		t.Errorf("expected msg_type=interactive, got %+v", received)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd server && go test ./internal/alert -run "TestValidateConfig_Feishu|TestFeishu_Send" -v`

预期：测试失败

- [ ] **步骤 3：实现 FeishuChannel**

替换 `server/internal/alert/feishu.go`：

```go
package alert

import (
	"bytes"
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

func (c *FeishuChannel) Send(msg Message) error {
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
	body, _ := json.Marshal(payload)
	resp, err := http.Post(cfg.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("feishu returned status %d", resp.StatusCode)
	}
	return nil
}

func (c *FeishuChannel) Test() error {
	return c.Send(buildTestMessage())
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
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd server && go test ./internal/alert -run "TestValidateConfig_Feishu|TestFeishu_Send" -v`

预期：3 个测试全部 PASS

- [ ] **步骤 5：Commit**

```bash
git add server/internal/alert/feishu.go server/internal/alert/channel_test.go
git commit -m "feat(alert): implement feishu channel adapter"
```

---

## 任务 6：Alert 包 - 企业微信适配器

**文件：**
- 修改：`server/internal/alert/wecom.go`（完整实现）
- 测试：`server/internal/alert/channel_test.go`（追加测试）

- [ ] **步骤 1：追加失败的测试**

在 `server/internal/alert/channel_test.go` 末尾追加：

```go
func TestValidateConfig_Wecom_Valid(t *testing.T) {
	ch := &WecomChannel{config: `{"url":"https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"}`}
	if err := ch.ValidateConfig(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestValidateConfig_Wecom_Invalid_MissingURL(t *testing.T) {
	ch := &WecomChannel{config: `{}`}
	if err := ch.ValidateConfig(); err == nil {
		t.Error("expected error for missing url")
	}
}

func TestWecom_Send(t *testing.T) {
	var received map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer ts.Close()

	ch := &WecomChannel{config: fmt.Sprintf(`{"url":"%s"}`, ts.URL)}
	err := ch.Send(Message{Title: "Alert", Body: "Test"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if received["msgtype"] != "markdown" {
		t.Errorf("expected msgtype=markdown, got %+v", received)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd server && go test ./internal/alert -run "TestValidateConfig_Wecom|TestWecom_Send" -v`

预期：测试失败

- [ ] **步骤 3：实现 WecomChannel**

替换 `server/internal/alert/wecom.go`：

```go
package alert

import (
	"bytes"
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

func (c *WecomChannel) Send(msg Message) error {
	var cfg wecomConfig
	if err := json.Unmarshal([]byte(c.config), &cfg); err != nil {
		return err
	}
	content := fmt.Sprintf("**%s**\n%s", msg.Title, msg.Body)
	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": content,
		},
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(cfg.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("wecom returned status %d", resp.StatusCode)
	}
	return nil
}

func (c *WecomChannel) Test() error {
	return c.Send(buildTestMessage())
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
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd server && go test ./internal/alert -run "TestValidateConfig_Wecom|TestWecom_Send" -v`

预期：3 个测试全部 PASS

- [ ] **步骤 5：Commit**

```bash
git add server/internal/alert/wecom.go server/internal/alert/channel_test.go
git commit -m "feat(alert): implement wecom channel adapter"
```

---

## 任务 7：Alert 包 - Email 适配器

**文件：**
- 修改：`server/internal/alert/email.go`（完整实现）
- 测试：`server/internal/alert/channel_test.go`（追加测试）

- [ ] **步骤 1：追加失败的测试**

在 `server/internal/alert/channel_test.go` 末尾追加：

```go
func TestValidateConfig_Email_Valid(t *testing.T) {
	ch := &EmailChannel{config: `{"smtp_host":"smtp.example.com","smtp_port":587,"username":"user","password":"pass","from":"a@b.com","to":["c@d.com"]}`}
	if err := ch.ValidateConfig(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestValidateConfig_Email_Invalid_MissingHost(t *testing.T) {
	ch := &EmailChannel{config: `{"smtp_port":587,"username":"user","password":"pass","from":"a@b.com","to":["c@d.com"]}`}
	if err := ch.ValidateConfig(); err == nil {
		t.Error("expected error for missing smtp_host")
	}
}

func TestValidateConfig_Email_Invalid_EmptyTo(t *testing.T) {
	ch := &EmailChannel{config: `{"smtp_host":"smtp.example.com","smtp_port":587,"username":"user","password":"pass","from":"a@b.com","to":[]}`}
	if err := ch.ValidateConfig(); err == nil {
		t.Error("expected error for empty to list")
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd server && go test ./internal/alert -run "TestValidateConfig_Email" -v`

预期：测试失败

- [ ] **步骤 3：实现 EmailChannel**

替换 `server/internal/alert/email.go`：

```go
package alert

import (
	"encoding/json"
	"fmt"
	"net/smtp"
	"strings"
)

type EmailChannel struct {
	config string
}

type emailConfig struct {
	SMTPHost string   `json:"smtp_host"`
	SMTPPort int      `json:"smtp_port"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	From     string   `json:"from"`
	To       []string `json:"to"`
}

func (c *EmailChannel) Send(msg Message) error {
	var cfg emailConfig
	if err := json.Unmarshal([]byte(c.config), &cfg); err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)

	html := fmt.Sprintf("<h2>%s</h2><pre>%s</pre>", msg.Title, msg.Body)
	body := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=\"UTF-8\"\r\n\r\n%s",
		cfg.From, strings.Join(cfg.To, ","), msg.Title, html)

	return smtp.SendMail(addr, auth, cfg.From, cfg.To, []byte(body))
}

func (c *EmailChannel) Test() error {
	return c.Send(buildTestMessage())
}

func (c *EmailChannel) ValidateConfig() error {
	var cfg emailConfig
	if err := json.Unmarshal([]byte(c.config), &cfg); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if cfg.SMTPHost == "" {
		return fmt.Errorf("smtp_host is required")
	}
	if cfg.SMTPPort == 0 {
		return fmt.Errorf("smtp_port is required")
	}
	if cfg.Username == "" {
		return fmt.Errorf("username is required")
	}
	if cfg.Password == "" {
		return fmt.Errorf("password is required")
	}
	if cfg.From == "" {
		return fmt.Errorf("from is required")
	}
	if len(cfg.To) == 0 {
		return fmt.Errorf("to must contain at least one recipient")
	}
	return nil
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`cd server && go test ./internal/alert -run "TestValidateConfig_Email" -v`

预期：3 个测试全部 PASS

- [ ] **步骤 5：Commit**

```bash
git add server/internal/alert/email.go server/internal/alert/channel_test.go
git commit -m "feat(alert): implement email channel adapter with SMTP"
```

---

## 任务 8：API Handler - AlertChannel CRUD + Test 端点

**文件：**
- 创建：`server/internal/api/alert_channel_handler.go`（6 个 handler）
- 测试：`server/internal/api/alert_channel_test.go`（新建）

- [ ] **步骤 1：编写失败的测试**

创建 `server/internal/api/alert_channel_test.go`：

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ssl-tracker/server/internal/store"
)

func setupAlertChannelRouter(t *testing.T) (*http.ServeMux, *store.Store) {
	s := setupTestStore(t)
	h := NewAlertChannelHandler(s)
	r := http.NewServeMux()
	r.HandleFunc("POST /api/admin/alert-channels", h.CreateChannel)
	r.HandleFunc("GET /api/admin/alert-channels", h.ListChannels)
	r.HandleFunc("GET /api/admin/alert-channels/{id}", h.GetChannel)
	r.HandleFunc("PUT /api/admin/alert-channels/{id}", h.UpdateChannel)
	r.HandleFunc("DELETE /api/admin/alert-channels/{id}", h.DeleteChannel)
	r.HandleFunc("POST /api/admin/alert-channels/{id}/test", h.TestChannel)
	return r, s
}

func TestCreateChannel_Success(t *testing.T) {
	r, _ := setupAlertChannelRouter(t)
	body := `{"name":"Test","type":"webhook","config":"{\"url\":\"https://test.com\"}","enabled":true}`
	req := httptest.NewRequest("POST", "/api/admin/alert-channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Errorf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestCreateChannel_InvalidType(t *testing.T) {
	r, _ := setupAlertChannelRouter(t)
	body := `{"name":"Test","type":"invalid","config":"{}","enabled":true}`
	req := httptest.NewRequest("POST", "/api/admin/alert-channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateChannel_InvalidConfig_JSON(t *testing.T) {
	r, _ := setupAlertChannelRouter(t)
	body := `{"name":"Test","type":"webhook","config":"not json","enabled":true}`
	req := httptest.NewRequest("POST", "/api/admin/alert-channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateChannel_InvalidConfig_MissingField(t *testing.T) {
	r, _ := setupAlertChannelRouter(t)
	body := `{"name":"Test","type":"webhook","config":"{}","enabled":true}`
	req := httptest.NewRequest("POST", "/api/admin/alert-channels", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListChannels_NoConfig(t *testing.T) {
	r, s := setupAlertChannelRouter(t)
	s.CreateAlertChannel(&store.AlertChannel{Name: "Ch1", Type: "webhook", Config: `{"url":"https://secret.com"}`, Enabled: true})
	req := httptest.NewRequest("GET", "/api/admin/alert-channels", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "secret.com") {
		t.Errorf("list should not contain config field")
	}
}

func TestGetChannel_WithConfig(t *testing.T) {
	r, s := setupAlertChannelRouter(t)
	ch := &store.AlertChannel{Name: "Ch1", Type: "webhook", Config: `{"url":"https://test.com"}`, Enabled: true}
	s.CreateAlertChannel(ch)
	req := httptest.NewRequest("GET", "/api/admin/alert-channels/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "test.com") {
		t.Errorf("get should contain config field")
	}
}

func TestUpdateChannel_Success(t *testing.T) {
	r, s := setupAlertChannelRouter(t)
	ch := &store.AlertChannel{Name: "Old", Type: "webhook", Config: `{"url":"https://old.com"}`, Enabled: true}
	s.CreateAlertChannel(ch)
	body := `{"name":"New","type":"dingtalk","config":"{\"url\":\"https://new.com\"}","enabled":false}`
	req := httptest.NewRequest("PUT", "/api/admin/alert-channels/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUpdateChannel_NotFound(t *testing.T) {
	r, _ := setupAlertChannelRouter(t)
	body := `{"name":"X","type":"webhook","config":"{\"url\":\"https://x.com\"}","enabled":true}`
	req := httptest.NewRequest("PUT", "/api/admin/alert-channels/999", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteChannel_Success(t *testing.T) {
	r, s := setupAlertChannelRouter(t)
	ch := &store.AlertChannel{Name: "ToDelete", Type: "webhook", Config: "{}", Enabled: true}
	s.CreateAlertChannel(ch)
	req := httptest.NewRequest("DELETE", "/api/admin/alert-channels/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestTestChannel_Success(t *testing.T) {
	// httptest server to receive webhook
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer ts.Close()

	r, s := setupAlertChannelRouter(t)
	ch := &store.AlertChannel{Name: "Test", Type: "webhook", Config: `{"url":"` + ts.URL + `"}`, Enabled: true}
	s.CreateAlertChannel(ch)
	req := httptest.NewRequest("POST", "/api/admin/alert-channels/1/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestTestChannel_SendFailed(t *testing.T) {
	r, s := setupAlertChannelRouter(t)
	ch := &store.AlertChannel{Name: "Test", Type: "webhook", Config: `{"url":"https://invalid.domain.that.does.not.exist"}`, Enabled: true}
	s.CreateAlertChannel(ch)
	req := httptest.NewRequest("POST", "/api/admin/alert-channels/1/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestTestChannel_NotFound(t *testing.T) {
	r, _ := setupAlertChannelRouter(t)
	req := httptest.NewRequest("POST", "/api/admin/alert-channels/999/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`cd server && go test ./internal/api -run "TestCreateChannel|TestListChannels|TestGetChannel|TestUpdateChannel|TestDeleteChannel|TestTestChannel" -v`

预期：编译失败，`NewAlertChannelHandler not defined`

- [ ] **步骤 3：实现 AlertChannelHandler**

创建 `server/internal/api/alert_channel_handler.go`：

```go
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"ssl-tracker/server/internal/alert"
	"ssl-tracker/server/internal/store"
)

type AlertChannelHandler struct {
	store *store.Store
}

func NewAlertChannelHandler(s *store.Store) *AlertChannelHandler {
	return &AlertChannelHandler{store: s}
}

func (h *AlertChannelHandler) CreateChannel(c *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		Type    string `json:"type" binding:"required"`
		Config  string `json:"config" binding:"required"`
		Enabled *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": err.Error()}})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	// Validate config
	ch, err := alert.NewChannel(req.Type, req.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": err.Error()}})
		return
	}
	if err := ch.ValidateConfig(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": err.Error()}})
		return
	}
	channel := &store.AlertChannel{
		Name:    req.Name,
		Type:    req.Type,
		Config:  req.Config,
		Enabled: enabled,
	}
	if err := h.store.CreateAlertChannel(channel); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": channel.ID})
}

func (h *AlertChannelHandler) ListChannels(c *gin.Context) {
	channels, err := h.store.ListAlertChannels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	out := make([]gin.H, 0, len(channels))
	for _, ch := range channels {
		out = append(out, gin.H{
			"id":         ch.ID,
			"name":       ch.Name,
			"type":       ch.Type,
			"enabled":    ch.Enabled,
			"created_at": ch.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"channels": out})
}

func (h *AlertChannelHandler) GetChannel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_id", "message": "invalid channel ID"}})
		return
	}
	channel, err := h.store.GetAlertChannel(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "channel not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, channel)
}

func (h *AlertChannelHandler) UpdateChannel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_id", "message": "invalid channel ID"}})
		return
	}
	var req struct {
		Name    string `json:"name" binding:"required"`
		Type    string `json:"type" binding:"required"`
		Config  string `json:"config" binding:"required"`
		Enabled *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": err.Error()}})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	// Validate config
	ch, err := alert.NewChannel(req.Type, req.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": err.Error()}})
		return
	}
	if err := ch.ValidateConfig(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": err.Error()}})
		return
	}
	if err := h.store.UpdateAlertChannel(uint(id), req.Name, req.Type, req.Config, enabled); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "channel not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AlertChannelHandler) DeleteChannel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_id", "message": "invalid channel ID"}})
		return
	}
	if err := h.store.DeleteAlertChannel(uint(id)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "channel not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AlertChannelHandler) TestChannel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_id", "message": "invalid channel ID"}})
		return
	}
	channel, err := h.store.GetAlertChannel(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "channel not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	ch, err := alert.NewChannel(channel.Type, channel.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_config", "message": err.Error()}})
		return
	}
	if err := ch.Test(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "send_failed", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
```

注意：测试文件使用了 `setupTestStore` helper，需要确保它已存在（从 admin_test.go 复用）。如果不存在，需要创建或修改测试使用 `setupRouter`。

- [ ] **步骤 4：运行测试验证通过**

运行：`cd server && go test ./internal/api -run "TestCreateChannel|TestListChannels|TestGetChannel|TestUpdateChannel|TestDeleteChannel|TestTestChannel" -v`

预期：13 个测试全部 PASS

- [ ] **步骤 5：Commit**

```bash
git add server/internal/api/alert_channel_handler.go server/internal/api/alert_channel_test.go
git commit -m "feat(api): add AlertChannel CRUD and test endpoints"
```

---

## 任务 9：Router 集成 + AutoMigrate

**文件：**
- 修改：`server/internal/api/router.go`（注册 6 条新路由）
- 修改：`server/cmd/server/main.go`（AutoMigrate 加 AlertChannel）

- [ ] **步骤 1：修改 router.go**

在 `server/internal/api/router.go` 的 `adminGroup` 块内追加：

```go
		alertH := NewAlertChannelHandler(s)
		adminGroup.POST("/alert-channels", alertH.CreateChannel)
		adminGroup.GET("/alert-channels", alertH.ListChannels)
		adminGroup.GET("/alert-channels/:id", alertH.GetChannel)
		adminGroup.PUT("/alert-channels/:id", alertH.UpdateChannel)
		adminGroup.DELETE("/alert-channels/:id", alertH.DeleteChannel)
		adminGroup.POST("/alert-channels/:id/test", alertH.TestChannel)
```

- [ ] **步骤 2：修改 main.go**

在 `server/cmd/server/main.go` 的 `AutoMigrate` 调用中追加 `&store.AlertChannel{}`：

```go
	if err := db.AutoMigrate(
		&store.Agent{},
		&store.Domain{},
		&store.AgentDomainOverride{},
		&store.CheckResult{},
		&store.User{},
		&store.AlertChannel{},
	); err != nil {
```

- [ ] **步骤 3：运行全部测试验证不破坏**

运行：`cd server && go test ./...`

预期：所有测试 PASS

- [ ] **步骤 4：Commit**

```bash
git add server/internal/api/router.go server/cmd/server/main.go
git commit -m "feat(api): wire alert channel routes and AutoMigrate"
```

---

## 任务 10：前端基础设施 - types.ts + api.ts

**文件：**
- 修改：`web/src/types.ts`（新增 AlertChannel 接口）
- 修改：`web/src/api.ts`（新增 channelApi 对象）

- [ ] **步骤 1：扩展 types.ts**

在 `web/src/types.ts` 末尾追加：

```typescript
export interface AlertChannel {
  id: number
  name: string
  type: 'webhook' | 'dingtalk' | 'feishu' | 'wecom' | 'email'
  config?: string
  enabled: boolean
  created_at: string
}
```

- [ ] **步骤 2：扩展 api.ts**

在 `web/src/api.ts` 末尾追加：

```typescript
export const channelApi = {
  list: () =>
    request<{ channels: AlertChannel[] }>('/api/admin/alert-channels'),
  get: (id: number) =>
    request<AlertChannel>(`/api/admin/alert-channels/${id}`),
  create: (req: { name: string; type: string; config: string; enabled: boolean }) =>
    request<{ id: number }>('/api/admin/alert-channels', { method: 'POST', body: JSON.stringify(req) }),
  update: (id: number, req: { name: string; type: string; config: string; enabled: boolean }) =>
    request<{ ok: boolean }>(`/api/admin/alert-channels/${id}`, { method: 'PUT', body: JSON.stringify(req) }),
  delete: (id: number) =>
    request<{ ok: boolean }>(`/api/admin/alert-channels/${id}`, { method: 'DELETE' }),
  test: (id: number) =>
    request<{ ok: boolean }>(`/api/admin/alert-channels/${id}/test`, { method: 'POST' }),
}
```

在文件顶部 import 添加 `AlertChannel`：

```typescript
import type { Overview, DomainsResponse, DomainDetail, User, DomainAdmin, AgentAdmin, Override, AlertChannel } from './types'
```

- [ ] **步骤 3：运行 TypeScript 编译验证**

运行：`cd web && npm run build`

预期：编译通过

- [ ] **步骤 4：Commit**

```bash
git add web/src/types.ts web/src/api.ts
git commit -m "feat(web): add AlertChannel type and channelApi client"
```

---

## 任务 11：前端 Header 导航集成

**文件：**
- 修改：`web/src/components/Header.vue`（添加"告警渠道"链接）

- [ ] **步骤 1：修改 Header.vue**

在 `web/src/components/Header.vue` 的 `<nav>` 块内，在"Agent 管理"链接后追加：

```vue
        <RouterLink
          to="/admin/channels"
          class="text-sm font-medium px-3 py-1.5 rounded-md transition"
          active-class="text-ink bg-bg-subtle"
          inactive-class="text-ink-soft hover:text-ink hover:bg-bg-subtle"
        >
          告警渠道
        </RouterLink>
```

- [ ] **步骤 2：运行编译验证**

运行：`cd web && npm run build`

预期：编译通过

- [ ] **步骤 3：Commit**

```bash
git add web/src/components/Header.vue
git commit -m "feat(web): add alert channels navigation link to Header"
```

---

## 任务 12：前端 Router 集成 + 占位组件

**文件：**
- 修改：`web/src/router.ts`（新增路由）
- 创建：`web/src/views/AdminChannels.vue`（占位）

- [ ] **步骤 1：修改 router.ts**

在 `web/src/router.ts` 的 `routes` 数组中追加：

```typescript
    { path: '/admin/channels', component: AdminChannels, meta: { requiresAuth: true } },
```

在文件顶部 import 添加：

```typescript
import AdminChannels from './views/AdminChannels.vue'
```

- [ ] **步骤 2：创建占位组件**

创建 `web/src/views/AdminChannels.vue`：

```vue
<template>
  <div>AdminChannels - TODO</div>
</template>
```

- [ ] **步骤 3：运行编译验证**

运行：`cd web && npm run build`

预期：编译通过

- [ ] **步骤 4：Commit**

```bash
git add web/src/router.ts web/src/views/AdminChannels.vue
git commit -m "feat(web): add /admin/channels route and placeholder view"
```

---

## 任务 13：前端 AdminChannels 页面完整实现

**文件：**
- 替换：`web/src/views/AdminChannels.vue`（完整实现）

- [ ] **步骤 1：实现 AdminChannels.vue**

替换 `web/src/views/AdminChannels.vue` 为完整实现（约 200 行，参考 AdminDomains.vue 的模式）：

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { channelApi } from '../api'
import type { AlertChannel } from '../types'

const channels = ref<AlertChannel[]>([])
const loading = ref(true)
const error = ref('')
const showCreateForm = ref(false)
const editingId = ref<number | null>(null)

const newChannel = ref({ name: '', type: 'webhook', config: '{}', enabled: true })
const editForm = ref({ name: '', type: '', config: '', enabled: true })
const testingId = ref<number | null>(null)
const testSuccessId = ref<number | null>(null)

const typeLabels: Record<string, string> = {
  webhook: 'Webhook',
  dingtalk: '钉钉',
  feishu: '飞书',
  wecom: '企业微信',
  email: '邮件',
}

async function loadChannels() {
  loading.value = true
  try {
    const res = await channelApi.list()
    channels.value = res.channels
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

async function createChannel() {
  error.value = ''
  try {
    await channelApi.create(newChannel.value)
    showCreateForm.value = false
    newChannel.value = { name: '', type: 'webhook', config: '{}', enabled: true }
    await loadChannels()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function startEdit(id: number) {
  error.value = ''
  try {
    const ch = await channelApi.get(id)
    editingId.value = id
    editForm.value = { name: ch.name, type: ch.type, config: ch.config || '{}', enabled: ch.enabled }
  } catch (e) {
    error.value = (e as Error).message
  }
}

function cancelEdit() {
  editingId.value = null
}

async function saveEdit(id: number) {
  error.value = ''
  try {
    await channelApi.update(id, editForm.value)
    editingId.value = null
    await loadChannels()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function deleteChannel(id: number) {
  if (!confirm('确定删除此告警渠道吗？')) return
  error.value = ''
  try {
    await channelApi.delete(id)
    await loadChannels()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function testChannel(id: number) {
  testingId.value = id
  testSuccessId.value = null
  error.value = ''
  try {
    await channelApi.test(id)
    testSuccessId.value = id
    setTimeout(() => { testSuccessId.value = null }, 3000)
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    testingId.value = null
  }
}

onMounted(loadChannels)
</script>

<template>
  <div class="max-w-6xl mx-auto px-6 py-8">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-semibold text-ink">告警渠道</h1>
      <button
        type="button"
        @click="showCreateForm = !showCreateForm"
        class="px-4 py-2 bg-accent text-white rounded-md text-sm font-medium hover:opacity-90"
      >
        {{ showCreateForm ? '取消' : '新增渠道' }}
      </button>
    </div>

    <div v-if="error" class="mb-4 p-3 bg-red-50 border border-red-200 rounded-md">
      <p class="text-sm text-red-600">{{ error }}</p>
      <button @click="error = ''" class="text-xs text-red-500 hover:text-red-700">×</button>
    </div>

    <div v-if="showCreateForm" class="mb-6 p-4 bg-bg-subtle rounded-md border border-border-soft">
      <h2 class="text-lg font-medium mb-3">新增告警渠道</h2>
      <form @submit.prevent="createChannel" class="space-y-3">
        <div>
          <label class="block text-sm text-ink-soft mb-1">名称</label>
          <input v-model="newChannel.name" type="text" required class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink" />
        </div>
        <div>
          <label class="block text-sm text-ink-soft mb-1">类型</label>
          <select v-model="newChannel.type" class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink">
            <option value="webhook">Webhook</option>
            <option value="dingtalk">钉钉</option>
            <option value="feishu">飞书</option>
            <option value="wecom">企业微信</option>
            <option value="email">邮件</option>
          </select>
        </div>
        <div>
          <label class="block text-sm text-ink-soft mb-1">配置 (JSON)</label>
          <textarea v-model="newChannel.config" required rows="5" class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink font-mono text-sm"></textarea>
        </div>
        <div>
          <label class="flex items-center gap-2 text-sm text-ink-soft">
            <input v-model="newChannel.enabled" type="checkbox" />
            启用
          </label>
        </div>
        <button type="submit" class="px-4 py-2 bg-accent text-white rounded-md text-sm font-medium hover:opacity-90">
          保存
        </button>
      </form>
    </div>

    <div v-if="loading" class="text-center py-8 text-ink-soft">加载中...</div>
    <div v-else-if="channels.length === 0" class="text-center py-8 text-ink-soft">暂无告警渠道</div>

    <table v-else class="w-full">
      <thead>
        <tr class="border-b border-border-soft text-left text-sm text-ink-soft">
          <th class="pb-2 font-medium">名称</th>
          <th class="pb-2 font-medium">类型</th>
          <th class="pb-2 font-medium">启用</th>
          <th class="pb-2 font-medium">操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="ch in channels" :key="ch.id" class="border-b border-border-soft">
          <template v-if="editingId !== ch.id">
            <td class="py-3 text-sm text-ink">{{ ch.name }}</td>
            <td class="py-3 text-sm text-ink">{{ typeLabels[ch.type] || ch.type }}</td>
            <td class="py-3 text-sm">
              <span v-if="ch.enabled" class="text-ok font-medium">已启用</span>
              <span v-else class="text-ink-soft">已禁用</span>
            </td>
            <td class="py-3 text-sm space-x-2">
              <button @click="startEdit(ch.id)" class="text-accent hover:underline">编辑</button>
              <button
                @click="testChannel(ch.id)"
                :disabled="testingId === ch.id"
                class="text-accent hover:underline disabled:opacity-50"
              >
                {{ testingId === ch.id ? '发送中...' : '测试' }}
              </button>
              <span v-if="testSuccessId === ch.id" class="text-ok text-xs">发送成功</span>
              <button @click="deleteChannel(ch.id)" class="text-bad hover:underline">删除</button>
            </td>
          </template>
          <template v-else>
            <td class="py-3">
              <input v-model="editForm.name" type="text" class="px-2 py-1 border border-border-soft rounded text-sm w-full" />
            </td>
            <td class="py-3">
              <select v-model="editForm.type" class="px-2 py-1 border border-border-soft rounded text-sm">
                <option value="webhook">Webhook</option>
                <option value="dingtalk">钉钉</option>
                <option value="feishu">飞书</option>
                <option value="wecom">企业微信</option>
                <option value="email">邮件</option>
              </select>
            </td>
            <td class="py-3">
              <input v-model="editForm.enabled" type="checkbox" />
            </td>
            <td class="py-3 text-sm space-x-2">
              <button @click="saveEdit(ch.id)" class="text-accent hover:underline">保存</button>
              <button @click="cancelEdit" class="text-ink-soft hover:underline">取消</button>
            </td>
          </template>
        </tr>
      </tbody>
    </table>

    <div v-if="editingId !== null" class="mt-6 p-4 bg-bg-subtle rounded-md border border-border-soft">
      <h3 class="text-md font-medium mb-2">编辑配置</h3>
      <textarea v-model="editForm.config" rows="8" class="w-full px-3 py-2 border border-border-soft rounded-md bg-bg text-ink font-mono text-sm"></textarea>
    </div>
  </div>
</template>
```

- [ ] **步骤 2：运行编译验证**

运行：`cd web && npm run build`

预期：编译通过

- [ ] **步骤 3：Commit**

```bash
git add web/src/views/AdminChannels.vue
git commit -m "feat(web): implement AdminChannels page (CRUD + test button)"
```

---

## 任务 14：端到端验证

- [ ] **步骤 1：启动 server**

```bash
cd server
./server -config config.local.yaml
```

- [ ] **步骤 2：浏览器登录并验证 Header**

访问 `http://localhost:8080/login`，登录后验证 Header 出现"告警渠道"链接。

- [ ] **步骤 3：新建 webhook 渠道**

点击"新增渠道"，填入：
- 名称：`Test Webhook`
- 类型：`Webhook`
- 配置：`{"url":"https://httpbin.org/post"}`
- 启用：✓

保存后验证列表出现。

- [ ] **步骤 4：测试发送**

点击"测试"按钮，验证：
- 按钮变为"发送中..."
- 成功后显示"发送成功"（httpbin.org 会返回 200）

- [ ] **步骤 5：新建钉钉渠道（预期失败）**

新建钉钉渠道，配置随意，点击"测试"，验证：
- 失败后顶部显示红色 banner 错误信息

- [ ] **步骤 6：编辑渠道**

点击"编辑"，修改名称，保存，验证列表更新。

- [ ] **步骤 7：删除渠道**

点击"删除"，confirm，验证列表移除。

- [ ] **步骤 8：验证敏感信息保护**

检查列表页 HTML，确认不包含 config 字段。

- [ ] **步骤 9：Commit（如有修复）**

```bash
git add -A
git commit -m "fix: e2e verification fixes (if any)"
```

package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
	err := ch.Send(context.Background(), Message{Title: "Test", Body: "Body"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if received["title"] != "Test" || received["body"] != "Body" {
		t.Errorf("unexpected payload: %+v", received)
	}
}

func TestWebhook_Send_NonOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer ts.Close()

	ch := &WebhookChannel{config: fmt.Sprintf(`{"url":"%s"}`, ts.URL)}
	err := ch.Send(context.Background(), Message{Title: "T", Body: "B"})
	if err == nil {
		t.Error("expected error for non-2xx status")
	}
}

package alert

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"errcode":0,"errmsg":"ok"}`)
	}))
	defer ts.Close()

	ch := &DingtalkChannel{config: fmt.Sprintf(`{"url":"%s"}`, ts.URL)}
	err := ch.Send(context.Background(), Message{Title: "Alert", Body: "Test body"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if received["msgtype"] != "markdown" {
		t.Errorf("expected msgtype=markdown, got %+v", received)
	}
}

func TestDingtalk_Send_WithSecret(t *testing.T) {
	secret := "SEC123"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		tsVal := q.Get("timestamp")
		signVal := q.Get("sign")
		// Independently compute expected signature
		stringToSign := tsVal + "\n" + secret
		h := hmac.New(sha256.New, []byte(secret))
		h.Write([]byte(stringToSign))
		expectedSign := base64.StdEncoding.EncodeToString(h.Sum(nil))
		if signVal != expectedSign {
			t.Errorf("sign mismatch: got %s, want %s", signVal, expectedSign)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"errcode":0,"errmsg":"ok"}`)
	}))
	defer ts.Close()

	ch := &DingtalkChannel{config: fmt.Sprintf(`{"url":"%s","secret":"%s"}`, ts.URL, secret)}
	err := ch.Send(context.Background(), Message{Title: "Test", Body: "Body"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestDingtalk_Send_NonOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer ts.Close()
	ch := &DingtalkChannel{config: fmt.Sprintf(`{"url":"%s"}`, ts.URL)}
	if err := ch.Send(context.Background(), Message{Title: "T", Body: "B"}); err == nil {
		t.Error("expected error for non-2xx status")
	}
}

func TestDingtalk_Send_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"errcode":310000,"errmsg":"bad access_token"}`)
	}))
	defer ts.Close()
	ch := &DingtalkChannel{config: fmt.Sprintf(`{"url":"%s"}`, ts.URL)}
	err := ch.Send(context.Background(), Message{Title: "T", Body: "B"})
	if err == nil {
		t.Fatal("expected error for bad access_token")
	}
	if got := err.Error(); got != "dingtalk error 310000: bad access_token" {
		t.Errorf("unexpected error: %s", got)
	}
}

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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"code":0,"msg":"success"}`)
	}))
	defer ts.Close()

	ch := &FeishuChannel{config: fmt.Sprintf(`{"url":"%s"}`, ts.URL)}
	err := ch.Send(context.Background(), Message{Title: "Alert", Body: "Test"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if received["msg_type"] != "interactive" {
		t.Errorf("expected msg_type=interactive, got %+v", received)
	}
}

func TestFeishu_Send_NonOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer ts.Close()
	ch := &FeishuChannel{config: fmt.Sprintf(`{"url":"%s"}`, ts.URL)}
	if err := ch.Send(context.Background(), Message{Title: "T", Body: "B"}); err == nil {
		t.Error("expected error for non-2xx status")
	}
}

func TestFeishu_Send_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"code":19021,"msg":"sign match fail or timestamp is not within one hour from current time"}`)
	}))
	defer ts.Close()
	ch := &FeishuChannel{config: fmt.Sprintf(`{"url":"%s"}`, ts.URL)}
	err := ch.Send(context.Background(), Message{Title: "T", Body: "B"})
	if err == nil {
		t.Fatal("expected error for API error code")
	}
	if got := err.Error(); got != "feishu error 19021: sign match fail or timestamp is not within one hour from current time" {
		t.Errorf("unexpected error: %s", got)
	}
}

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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"errcode":0,"errmsg":"ok"}`)
	}))
	defer ts.Close()

	ch := &WecomChannel{config: fmt.Sprintf(`{"url":"%s"}`, ts.URL)}
	err := ch.Send(context.Background(), Message{Title: "Alert", Body: "Test"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if received["msgtype"] != "markdown" {
		t.Errorf("expected msgtype=markdown, got %+v", received)
	}
}

func TestWecom_Send_NonOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer ts.Close()
	ch := &WecomChannel{config: fmt.Sprintf(`{"url":"%s"}`, ts.URL)}
	if err := ch.Send(context.Background(), Message{Title: "T", Body: "B"}); err == nil {
		t.Error("expected error for non-2xx status")
	}
}

func TestWecom_Send_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{"errcode":93000,"errmsg":"invalid webhook url"}`)
	}))
	defer ts.Close()
	ch := &WecomChannel{config: fmt.Sprintf(`{"url":"%s"}`, ts.URL)}
	err := ch.Send(context.Background(), Message{Title: "T", Body: "B"})
	if err == nil {
		t.Fatal("expected error for API error code")
	}
	if got := err.Error(); got != "wecom error 93000: invalid webhook url" {
		t.Errorf("unexpected error: %s", got)
	}
}

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

func TestValidateConfig_Email_Invalid_JSON(t *testing.T) {
	ch := &EmailChannel{config: `not json`}
	if err := ch.ValidateConfig(); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

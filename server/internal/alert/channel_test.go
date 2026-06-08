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

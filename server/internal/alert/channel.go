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

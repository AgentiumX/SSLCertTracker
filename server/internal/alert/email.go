package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
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

func (c *EmailChannel) Send(ctx context.Context, msg Message) error {
	var cfg emailConfig
	if err := json.Unmarshal([]byte(c.config), &cfg); err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)

	// Build email headers and body
	var email strings.Builder
	email.WriteString(fmt.Sprintf("From: %s\r\n", cfg.From))
	email.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(cfg.To, ",")))
	email.WriteString(fmt.Sprintf("Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Title)))
	email.WriteString("MIME-Version: 1.0\r\n")
	email.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	email.WriteString("\r\n")
	email.WriteString(msg.Body)

	return smtp.SendMail(addr, auth, cfg.From, cfg.To, []byte(email.String()))
}

func (c *EmailChannel) Test() error {
	return c.Send(context.Background(), buildTestMessage())
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

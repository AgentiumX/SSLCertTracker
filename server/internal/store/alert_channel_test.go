package store

import (
	"errors"
	"testing"

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

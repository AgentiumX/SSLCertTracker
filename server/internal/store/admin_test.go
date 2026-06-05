package store

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestUpdateDomainMeta(t *testing.T) {
	s := setupTestDB(t)
	d := &Domain{Host: "example.com", Port: 443, Protocol: "https", IsGlobal: false, Remark: "old"}
	if err := s.CreateDomain(d); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateDomainMeta(d.ID, true, "new remark"); err != nil {
		t.Fatalf("UpdateDomainMeta: %v", err)
	}
	got, err := s.GetDomain(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsGlobal {
		t.Errorf("expected IsGlobal=true, got false")
	}
	if got.Remark != "new remark" {
		t.Errorf("expected remark='new remark', got %q", got.Remark)
	}
	if got.Host != "example.com" || got.Port != 443 || got.Protocol != "https" {
		t.Errorf("host/port/protocol should not change, got %+v", got)
	}
}

func TestUpdateDomainMeta_NotFound(t *testing.T) {
	s := setupTestDB(t)
	err := s.UpdateDomainMeta(9999, true, "x")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestUpdateAgentRemark(t *testing.T) {
	s := setupTestDB(t)
	a := &Agent{AgentID: "a1", DisplayName: "A1", Remark: "old", RegisteredAt: time.Now(), LastSeenAt: time.Now()}
	if err := s.CreateAgent(a); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateAgentRemark("a1", "new remark"); err != nil {
		t.Fatalf("UpdateAgentRemark: %v", err)
	}
	got, err := s.GetAgent("a1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Remark != "new remark" {
		t.Errorf("expected remark='new remark', got %q", got.Remark)
	}
}

func TestUpdateAgentRemark_NotFound(t *testing.T) {
	s := setupTestDB(t)
	err := s.UpdateAgentRemark("nonexistent", "x")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestListOverrides(t *testing.T) {
	s := setupTestDB(t)
	d1 := &Domain{Host: "d1.com", Port: 443, Protocol: "https"}
	d2 := &Domain{Host: "d2.com", Port: 443, Protocol: "https"}
	if err := s.CreateDomain(d1); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateDomain(d2); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateOverride(&AgentDomainOverride{AgentID: "a1", DomainID: d1.ID, Action: "include"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateOverride(&AgentDomainOverride{AgentID: "a1", DomainID: d2.ID, Action: "exclude"}); err != nil {
		t.Fatal(err)
	}
	overrides, err := s.ListOverrides("a1")
	if err != nil {
		t.Fatal(err)
	}
	if len(overrides) != 2 {
		t.Fatalf("expected 2 overrides, got %d", len(overrides))
	}
}

func TestUpsertOverride(t *testing.T) {
	s := setupTestDB(t)
	d := &Domain{Host: "d.com", Port: 443, Protocol: "https"}
	if err := s.CreateDomain(d); err != nil {
		t.Fatal(err)
	}
	// Insert
	if err := s.UpsertOverride("a1", d.ID, "include"); err != nil {
		t.Fatalf("UpsertOverride insert: %v", err)
	}
	// Update action
	if err := s.UpsertOverride("a1", d.ID, "exclude"); err != nil {
		t.Fatalf("UpsertOverride update: %v", err)
	}
	overrides, err := s.ListOverrides("a1")
	if err != nil {
		t.Fatal(err)
	}
	if len(overrides) != 1 || overrides[0].Action != "exclude" {
		t.Errorf("expected 1 override with action=exclude, got %+v", overrides)
	}
}

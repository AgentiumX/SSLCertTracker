package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *Store {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})
	if err := db.AutoMigrate(&Agent{}, &Domain{}, &AgentDomainOverride{}, &CheckResult{}, &User{}); err != nil {
		t.Fatal(err)
	}
	return &Store{db: db}
}

func TestCreateAgent(t *testing.T) {
	s := setupTestDB(t)
	agent := &Agent{
		AgentID:      "test-agent-001",
		DisplayName:  "Test Agent",
		Hostname:     "host1",
		IP:           "10.0.0.1",
		RegisteredAt: time.Now(),
		LastSeenAt:   time.Now(),
	}
	if err := s.CreateAgent(agent); err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}

	found, err := s.GetAgent("test-agent-001")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if found.DisplayName != "Test Agent" {
		t.Errorf("expected 'Test Agent', got %s", found.DisplayName)
	}
}

func TestUpdateAgentLastSeen(t *testing.T) {
	s := setupTestDB(t)
	agent := &Agent{AgentID: "a1", DisplayName: "A1", Hostname: "h1", IP: "1.1.1.1", RegisteredAt: time.Now(), LastSeenAt: time.Now()}
	s.CreateAgent(agent)

	time.Sleep(10 * time.Millisecond)
	newTime := time.Now()
	if err := s.UpdateAgentLastSeen("a1", newTime); err != nil {
		t.Fatalf("UpdateAgentLastSeen failed: %v", err)
	}

	found, _ := s.GetAgent("a1")
	if found.LastSeenAt.Unix() != newTime.Unix() {
		t.Errorf("LastSeenAt not updated")
	}
}

func TestUpsertAgent(t *testing.T) {
	s := setupTestDB(t)
	agent := &Agent{AgentID: "a1", DisplayName: "A1", Hostname: "h1", IP: "1.1.1.1", RegisteredAt: time.Now(), LastSeenAt: time.Now()}
	if err := s.UpsertAgent(agent); err != nil {
		t.Fatalf("UpsertAgent insert failed: %v", err)
	}

	updated := &Agent{AgentID: "a1", DisplayName: "A1-renamed", Hostname: "h1-new", IP: "2.2.2.2", LastSeenAt: time.Now()}
	if err := s.UpsertAgent(updated); err != nil {
		t.Fatalf("UpsertAgent update failed: %v", err)
	}

	found, _ := s.GetAgent("a1")
	if found.DisplayName != "A1-renamed" {
		t.Errorf("expected A1-renamed, got %s", found.DisplayName)
	}
	if found.IP != "2.2.2.2" {
		t.Errorf("expected 2.2.2.2, got %s", found.IP)
	}
}

func TestCreateDomain(t *testing.T) {
	s := setupTestDB(t)
	domain := &Domain{
		Host:     "example.com",
		Port:     443,
		Protocol: "https",
		IsGlobal: true,
		Remark:   "test domain",
	}
	if err := s.CreateDomain(domain); err != nil {
		t.Fatalf("CreateDomain failed: %v", err)
	}
	if domain.ID == 0 {
		t.Errorf("expected ID > 0")
	}

	found, err := s.GetDomain(domain.ID)
	if err != nil {
		t.Fatalf("GetDomain failed: %v", err)
	}
	if found.Host != "example.com" {
		t.Errorf("expected example.com, got %s", found.Host)
	}
}

func TestListGlobalDomains(t *testing.T) {
	s := setupTestDB(t)
	s.CreateDomain(&Domain{Host: "global1.com", Port: 443, Protocol: "https", IsGlobal: true})
	s.CreateDomain(&Domain{Host: "local1.com", Port: 443, Protocol: "https", IsGlobal: false})
	s.CreateDomain(&Domain{Host: "global2.com", Port: 443, Protocol: "https", IsGlobal: true})

	globals, err := s.ListGlobalDomains()
	if err != nil {
		t.Fatalf("ListGlobalDomains failed: %v", err)
	}
	if len(globals) != 2 {
		t.Errorf("expected 2 global domains, got %d", len(globals))
	}
}

func TestListAllDomains(t *testing.T) {
	s := setupTestDB(t)
	s.CreateDomain(&Domain{Host: "global1.com", Port: 443, Protocol: "https", IsGlobal: true})
	s.CreateDomain(&Domain{Host: "local1.com", Port: 443, Protocol: "https", IsGlobal: false})

	all, err := s.ListAllDomains()
	if err != nil {
		t.Fatalf("ListAllDomains failed: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 domains, got %d", len(all))
	}
}

func TestDeleteDomain(t *testing.T) {
	s := setupTestDB(t)
	d := &Domain{Host: "del.com", Port: 443, Protocol: "https", IsGlobal: true}
	s.CreateDomain(d)

	if err := s.DeleteDomain(d.ID); err != nil {
		t.Fatalf("DeleteDomain failed: %v", err)
	}
	if _, err := s.GetDomain(d.ID); err == nil {
		t.Errorf("expected error after delete")
	}
}

func TestAgentDomainOverrides(t *testing.T) {
	s := setupTestDB(t)
	d1 := &Domain{Host: "d1.com", Port: 443, Protocol: "https", IsGlobal: false}
	s.CreateDomain(d1)

	override := &AgentDomainOverride{
		AgentID:  "agent1",
		DomainID: d1.ID,
		Action:   "include",
	}
	if err := s.CreateOverride(override); err != nil {
		t.Fatalf("CreateOverride failed: %v", err)
	}

	includes, excludes, err := s.GetAgentOverrides("agent1")
	if err != nil {
		t.Fatalf("GetAgentOverrides failed: %v", err)
	}
	if len(includes) != 1 || includes[0] != d1.ID {
		t.Errorf("expected 1 include with ID %d, got %v", d1.ID, includes)
	}
	if len(excludes) != 0 {
		t.Errorf("expected 0 excludes")
	}

	if err := s.DeleteOverride("agent1", d1.ID); err != nil {
		t.Fatalf("DeleteOverride failed: %v", err)
	}
	includes, _, _ = s.GetAgentOverrides("agent1")
	if len(includes) != 0 {
		t.Errorf("expected 0 includes after delete")
	}
}

func TestSaveCheckResults(t *testing.T) {
	s := setupTestDB(t)
	now := time.Now()
	notAfter := now.Add(30 * 24 * time.Hour)
	results := []CheckResult{
		{
			AgentID:   "agent1",
			DomainID:  1,
			CheckedAt: now,
			Status:    "ok",
			NotAfter:  &notAfter,
			Issuer:    "Let's Encrypt",
			Subject:   "CN=example.com",
		},
	}
	if err := s.SaveCheckResults(results); err != nil {
		t.Fatalf("SaveCheckResults failed: %v", err)
	}

	var saved []CheckResult
	s.db.Find(&saved)
	if len(saved) != 1 {
		t.Errorf("expected 1 result, got %d", len(saved))
	}
}

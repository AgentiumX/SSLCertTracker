package store

import (
	"testing"
	"time"
)

func TestLatestResults_PerAgentDomain(t *testing.T) {
	s := setupTestDB(t)
	now := time.Now()
	s.SaveCheckResults([]CheckResult{
		{AgentID: "a1", DomainID: 1, CheckedAt: now.Add(-2 * time.Hour), Status: "ok"},
		{AgentID: "a1", DomainID: 1, CheckedAt: now.Add(-1 * time.Hour), Status: "expiring"}, // newer
		{AgentID: "a1", DomainID: 2, CheckedAt: now, Status: "ok"},
		{AgentID: "a2", DomainID: 1, CheckedAt: now, Status: "expired"},
	})

	results, err := s.LatestResults()
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 latest rows (a1/1, a1/2, a2/1), got %d", len(results))
	}
	// a1/1 should be the "expiring" one (newer)
	for _, r := range results {
		if r.AgentID == "a1" && r.DomainID == 1 && r.Status != "expiring" {
			t.Errorf("a1/1 expected expiring, got %s", r.Status)
		}
	}
}

func TestLatestResultsForDomain(t *testing.T) {
	s := setupTestDB(t)
	now := time.Now()
	s.SaveCheckResults([]CheckResult{
		{AgentID: "a1", DomainID: 5, CheckedAt: now.Add(-1 * time.Hour), Status: "ok"},
		{AgentID: "a1", DomainID: 5, CheckedAt: now, Status: "expiring"},
		{AgentID: "a2", DomainID: 5, CheckedAt: now, Status: "ok"},
		{AgentID: "a1", DomainID: 6, CheckedAt: now, Status: "ok"}, // different domain, ignored
	})

	results, err := s.LatestResultsForDomain(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 rows (one per agent), got %d", len(results))
	}
	// Verify a1 took the newer "expiring" record, not the older "ok"
	for _, r := range results {
		if r.AgentID == "a1" && r.Status != "expiring" {
			t.Errorf("a1 expected expiring (newest), got %s", r.Status)
		}
	}
}

func TestCountAgentsOnline(t *testing.T) {
	s := setupTestDB(t)
	now := time.Now()
	s.CreateAgent(&Agent{AgentID: "online", DisplayName: "x", RegisteredAt: now, LastSeenAt: now.Add(-1 * time.Hour)})
	s.CreateAgent(&Agent{AgentID: "offline", DisplayName: "y", RegisteredAt: now, LastSeenAt: now.Add(-5 * time.Hour)})

	online, total, err := s.CountAgents(3 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("expected 2 total, got %d", total)
	}
	if online != 1 {
		t.Errorf("expected 1 online, got %d", online)
	}
}

func TestListAgents(t *testing.T) {
	s := setupTestDB(t)
	now := time.Now()
	s.CreateAgent(&Agent{AgentID: "a1", DisplayName: "Beijing", Hostname: "h1", IP: "1.1.1.1", RegisteredAt: now, LastSeenAt: now})
	s.CreateAgent(&Agent{AgentID: "a2", DisplayName: "Shanghai", Hostname: "h2", IP: "2.2.2.2", RegisteredAt: now, LastSeenAt: now})

	agents, err := s.ListAgents()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}
}

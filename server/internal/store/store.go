package store

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

const AgentOnlineWindow = 3 * time.Hour

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) DB() *gorm.DB {
	return s.db
}

// Agent operations

func (s *Store) CreateAgent(agent *Agent) error {
	return s.db.Create(agent).Error
}

func (s *Store) GetAgent(agentID string) (*Agent, error) {
	var agent Agent
	err := s.db.Where("agent_id = ?", agentID).First(&agent).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

func (s *Store) UpdateAgentLastSeen(agentID string, t time.Time) error {
	return s.db.Model(&Agent{}).Where("agent_id = ?", agentID).Update("last_seen_at", t).Error
}

func (s *Store) UpsertAgent(agent *Agent) error {
	var existing Agent
	err := s.db.Where("agent_id = ?", agent.AgentID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return s.db.Create(agent).Error
	}
	if err != nil {
		return err
	}
	return s.db.Model(&existing).Updates(map[string]interface{}{
		"display_name": agent.DisplayName,
		"hostname":     agent.Hostname,
		"ip":           agent.IP,
		"last_seen_at": agent.LastSeenAt,
	}).Error
}

// Domain operations

func (s *Store) CreateDomain(domain *Domain) error {
	return s.db.Select("Host", "Port", "Protocol", "IsGlobal", "Remark", "CreatedAt").Create(domain).Error
}

func (s *Store) GetDomain(id uint) (*Domain, error) {
	var domain Domain
	err := s.db.First(&domain, id).Error
	if err != nil {
		return nil, err
	}
	return &domain, nil
}

func (s *Store) ListGlobalDomains() ([]Domain, error) {
	var domains []Domain
	err := s.db.Where("is_global = ?", true).Find(&domains).Error
	return domains, err
}

func (s *Store) ListAllDomains() ([]Domain, error) {
	var domains []Domain
	err := s.db.Find(&domains).Error
	return domains, err
}

func (s *Store) DeleteDomain(id uint) error {
	return s.db.Delete(&Domain{}, id).Error
}

// Override operations

func (s *Store) CreateOverride(override *AgentDomainOverride) error {
	return s.db.Create(override).Error
}

func (s *Store) DeleteOverride(agentID string, domainID uint) error {
	return s.db.Where("agent_id = ? AND domain_id = ?", agentID, domainID).Delete(&AgentDomainOverride{}).Error
}

func (s *Store) GetAgentOverrides(agentID string) (includes []uint, excludes []uint, err error) {
	var overrides []AgentDomainOverride
	if err = s.db.Where("agent_id = ?", agentID).Find(&overrides).Error; err != nil {
		return nil, nil, err
	}
	for _, o := range overrides {
		if o.Action == "include" {
			includes = append(includes, o.DomainID)
		} else if o.Action == "exclude" {
			excludes = append(excludes, o.DomainID)
		}
	}
	return includes, excludes, nil
}

// CheckResult operations

func (s *Store) SaveCheckResults(results []CheckResult) error {
	if len(results) == 0 {
		return nil
	}
	return s.db.Create(&results).Error
}

// LatestResults returns the most recent CheckResult per (agent_id, domain_id).
// Implemented in Go to avoid SQL dialect differences between SQLite and MySQL.
func (s *Store) LatestResults() ([]CheckResult, error) {
	// Bound the scan to 7 days. Records older than this are stale (retention
	// default cleans them up), and "latest" results never live that far back
	// for an active agent.
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	var all []CheckResult
	if err := s.db.Where("checked_at >= ?", cutoff).Order("checked_at DESC").Find(&all).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	out := make([]CheckResult, 0, len(all))
	for _, r := range all {
		key := r.AgentID + "|" + fmt.Sprint(r.DomainID)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, r)
	}
	return out, nil
}

// LatestResultsForDomain returns the most recent CheckResult per agent for a single domain.
func (s *Store) LatestResultsForDomain(domainID uint) ([]CheckResult, error) {
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	var all []CheckResult
	if err := s.db.Where("domain_id = ? AND checked_at >= ?", domainID, cutoff).Order("checked_at DESC").Find(&all).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	out := make([]CheckResult, 0, len(all))
	for _, r := range all {
		if seen[r.AgentID] {
			continue
		}
		seen[r.AgentID] = true
		out = append(out, r)
	}
	return out, nil
}

// CountAgents returns (online, total). Online means LastSeenAt within onlineWindow.
func (s *Store) CountAgents(onlineWindow time.Duration) (online, total int64, err error) {
	if err = s.db.Model(&Agent{}).Count(&total).Error; err != nil {
		return 0, 0, err
	}
	threshold := time.Now().Add(-onlineWindow)
	if err = s.db.Model(&Agent{}).Where("last_seen_at >= ?", threshold).Count(&online).Error; err != nil {
		return 0, 0, err
	}
	return online, total, nil
}

// ListAgents returns all agents.
func (s *Store) ListAgents() ([]Agent, error) {
	var agents []Agent
	err := s.db.Find(&agents).Error
	return agents, err
}

// User operations

func (s *Store) CreateUser(u *User) error {
	return s.db.Create(u).Error
}

func (s *Store) GetUserByUsername(username string) (*User, error) {
	var u User
	err := s.db.Where("username = ?", username).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) CountUsers() (int64, error) {
	var n int64
	err := s.db.Model(&User{}).Count(&n).Error
	return n, err
}

// UpdateDomainMeta updates only is_global and remark; host/port/protocol are immutable.
func (s *Store) UpdateDomainMeta(id uint, isGlobal bool, remark string) error {
	res := s.db.Model(&Domain{}).Where("id = ?", id).Updates(map[string]interface{}{
		"is_global": isGlobal,
		"remark":    remark,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateAgentRemark updates agent remark field only.
func (s *Store) UpdateAgentRemark(agentID, remark string) error {
	res := s.db.Model(&Agent{}).Where("agent_id = ?", agentID).Update("remark", remark)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListOverrides returns all overrides for an agent.
func (s *Store) ListOverrides(agentID string) ([]AgentDomainOverride, error) {
	var overrides []AgentDomainOverride
	err := s.db.Where("agent_id = ?", agentID).Find(&overrides).Error
	return overrides, err
}

// UpsertOverride inserts or updates an override.
func (s *Store) UpsertOverride(agentID string, domainID uint, action string) error {
	override := &AgentDomainOverride{AgentID: agentID, DomainID: domainID, Action: action}
	return s.db.Where(AgentDomainOverride{AgentID: agentID, DomainID: domainID}).
		Assign(AgentDomainOverride{Action: action}).FirstOrCreate(override).Error
}

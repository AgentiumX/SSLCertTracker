package store

import "time"

type Agent struct {
	AgentID      string    `gorm:"primaryKey;size:16"`
	DisplayName  string    `gorm:"not null"`
	Hostname     string
	IP           string
	Remark       string
	RegisteredAt time.Time `gorm:"not null"`
	LastSeenAt   time.Time `gorm:"not null"`
}

type Domain struct {
	ID        uint   `gorm:"primaryKey"`
	Host      string `gorm:"not null;index:idx_domain_unique,unique"`
	Port      int    `gorm:"not null;index:idx_domain_unique,unique"`
	Protocol  string `gorm:"not null;index:idx_domain_unique,unique"`
	IsGlobal  bool   `gorm:"not null"`
	Remark    string
	CreatedAt time.Time
}

type AgentDomainOverride struct {
	AgentID  string `gorm:"primaryKey;size:16"`
	DomainID uint   `gorm:"primaryKey"`
	Action   string `gorm:"not null"` // include | exclude
}

type CheckResult struct {
	ID           uint      `gorm:"primaryKey"`
	AgentID      string    `gorm:"not null;index:idx_check_lookup"`
	DomainID     uint      `gorm:"not null;index:idx_check_lookup"`
	CheckedAt    time.Time `gorm:"not null;index:idx_check_lookup;index:idx_cleanup"`
	Status       string    `gorm:"not null"` // ok | expiring | expired | mismatch | unreachable
	NotAfter     *time.Time
	Issuer       string
	Subject      string
	SANs         string // JSON array
	ErrorMessage string
}

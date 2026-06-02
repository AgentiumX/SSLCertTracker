package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"ssl-tracker/server/internal/processor"
	"ssl-tracker/server/internal/scheduler"
	"ssl-tracker/server/internal/store"
)

type AgentHandler struct {
	store               *store.Store
	expireThresholdDays int
}

func NewAgentHandler(s *store.Store, expireThresholdDays int) *AgentHandler {
	return &AgentHandler{store: s, expireThresholdDays: expireThresholdDays}
}

func (h *AgentHandler) Register(c *gin.Context) {
	var req struct {
		AgentID     string `json:"agent_id" binding:"required"`
		DisplayName string `json:"display_name" binding:"required"`
		Hostname    string `json:"hostname"`
		IP          string `json:"ip"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": err.Error()}})
		return
	}
	agent := &store.Agent{
		AgentID:      req.AgentID,
		DisplayName:  req.DisplayName,
		Hostname:     req.Hostname,
		IP:           req.IP,
		RegisteredAt: time.Now(),
		LastSeenAt:   time.Now(),
	}
	if err := h.store.UpsertAgent(agent); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AgentHandler) GetDomains(c *gin.Context) {
	agentID := c.Query("agent_id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "missing_agent_id", "message": "agent_id required"}})
		return
	}
	if err := h.store.UpdateAgentLastSeen(agentID, time.Now()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	globals, err := h.store.ListGlobalDomains()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	globalIDs := make([]uint, len(globals))
	for i, d := range globals {
		globalIDs[i] = d.ID
	}
	includes, excludes, err := h.store.GetAgentOverrides(agentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	domainIDs := scheduler.ComputeAgentDomains(globalIDs, includes, excludes)
	domains := make([]gin.H, 0, len(domainIDs))
	for _, id := range domainIDs {
		d, err := h.store.GetDomain(id)
		if err != nil {
			continue
		}
		domains = append(domains, gin.H{"id": d.ID, "host": d.Host, "port": d.Port, "protocol": d.Protocol})
	}
	c.JSON(http.StatusOK, gin.H{"domains": domains})
}

func (h *AgentHandler) PostResults(c *gin.Context) {
	var req struct {
		AgentID string `json:"agent_id" binding:"required"`
		Results []struct {
			DomainID     uint       `json:"domain_id" binding:"required"`
			CheckedAt    time.Time  `json:"checked_at" binding:"required"`
			Status       string     `json:"status" binding:"required"`
			NotAfter     *time.Time `json:"not_after"`
			Issuer       string     `json:"issuer"`
			Subject      string     `json:"subject"`
			SANs         string     `json:"sans"`
			ErrorMessage string     `json:"error_message"`
		} `json:"results" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": err.Error()}})
		return
	}
	results := make([]store.CheckResult, 0, len(req.Results))
	for _, r := range req.Results {
		status := r.Status
		if r.NotAfter != nil {
			status = processor.ReclassifyStatus(r.Status, *r.NotAfter, h.expireThresholdDays)
		}
		results = append(results, store.CheckResult{
			AgentID:      req.AgentID,
			DomainID:     r.DomainID,
			CheckedAt:    r.CheckedAt,
			Status:       status,
			NotAfter:     r.NotAfter,
			Issuer:       r.Issuer,
			Subject:      r.Subject,
			SANs:         r.SANs,
			ErrorMessage: r.ErrorMessage,
		})
	}
	if err := h.store.SaveCheckResults(results); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"accepted": len(results)})
}

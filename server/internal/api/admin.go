package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"ssl-tracker/server/internal/store"
)

type AdminHandler struct {
	store *store.Store
}

func NewAdminHandler(s *store.Store) *AdminHandler {
	return &AdminHandler{store: s}
}

func (h *AdminHandler) CreateDomain(c *gin.Context) {
	var req struct {
		Host     string `json:"host" binding:"required"`
		Port     int    `json:"port" binding:"required"`
		Protocol string `json:"protocol" binding:"required"`
		IsGlobal bool   `json:"is_global"`
		Remark   string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": err.Error()}})
		return
	}
	domain := &store.Domain{Host: req.Host, Port: req.Port, Protocol: req.Protocol, IsGlobal: req.IsGlobal, Remark: req.Remark}
	if err := h.store.CreateDomain(domain); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": domain.ID})
}

func (h *AdminHandler) ListDomains(c *gin.Context) {
	domains, err := h.store.ListAllDomains()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"domains": domains})
}

func (h *AdminHandler) GetDomain(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_id", "message": "invalid domain ID"}})
		return
	}
	domain, err := h.store.GetDomain(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "domain not found"}})
		return
	}
	c.JSON(http.StatusOK, domain)
}

func (h *AdminHandler) DeleteDomain(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_id", "message": "invalid domain ID"}})
		return
	}
	if err := h.store.DeleteDomain(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AdminHandler) UpdateDomain(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_id", "message": "invalid domain ID"}})
		return
	}
	var req struct {
		IsGlobal *bool   `json:"is_global"`
		Remark   *string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": err.Error()}})
		return
	}
	if req.IsGlobal == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "is_global is required"}})
		return
	}
	if req.Remark == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "remark is required"}})
		return
	}
	if err := h.store.UpdateDomainMeta(uint(id), *req.IsGlobal, *req.Remark); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "domain not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AdminHandler) ListAgents(c *gin.Context) {
	agents, err := h.store.ListAgents()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	threshold := time.Now().Add(-store.AgentOnlineWindow)
	out := make([]gin.H, 0, len(agents))
	for _, a := range agents {
		out = append(out, gin.H{
			"agent_id":      a.AgentID,
			"display_name":  a.DisplayName,
			"hostname":      a.Hostname,
			"ip":            a.IP,
			"remark":        a.Remark,
			"registered_at": a.RegisteredAt,
			"last_seen_at":  a.LastSeenAt,
			"is_online":     !a.LastSeenAt.IsZero() && a.LastSeenAt.After(threshold),
		})
	}
	c.JSON(http.StatusOK, gin.H{"agents": out})
}

func (h *AdminHandler) UpdateAgent(c *gin.Context) {
	agentID := c.Param("id")
	var req struct {
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": err.Error()}})
		return
	}
	if err := h.store.UpdateAgentRemark(agentID, req.Remark); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "agent not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AdminHandler) ListOverrides(c *gin.Context) {
	agentID := c.Param("id")
	overrides, err := h.store.ListOverrides(agentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	out := make([]gin.H, 0, len(overrides))
	for _, o := range overrides {
		out = append(out, gin.H{
			"domain_id": o.DomainID,
			"action":    o.Action,
		})
	}
	c.JSON(http.StatusOK, gin.H{"overrides": out})
}

func (h *AdminHandler) SetOverride(c *gin.Context) {
	agentID := c.Param("id")
	var req struct {
		DomainID uint   `json:"domain_id"`
		Action   string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": err.Error()}})
		return
	}
	if req.Action != "include" && req.Action != "exclude" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "action must be include or exclude"}})
		return
	}
	if req.DomainID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "domain_id must be > 0"}})
		return
	}
	// Verify domain exists
	if _, err := h.store.GetDomain(req.DomainID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "domain not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	// Verify agent exists
	if _, err := h.store.GetAgent(agentID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "agent not found"}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	if err := h.store.UpsertOverride(agentID, req.DomainID, req.Action); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AdminHandler) DeleteOverride(c *gin.Context) {
	agentID := c.Param("id")
	domainID, err := strconv.ParseUint(c.Param("domain_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_id", "message": "invalid domain ID"}})
		return
	}
	if err := h.store.DeleteOverride(agentID, uint(domainID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

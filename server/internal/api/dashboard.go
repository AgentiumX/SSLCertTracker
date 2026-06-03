package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"ssl-tracker/server/internal/store"
)

type DashboardHandler struct {
	store        *store.Store
	onlineWindow time.Duration
}

func NewDashboardHandler(s *store.Store, onlineWindow time.Duration) *DashboardHandler {
	return &DashboardHandler{store: s, onlineWindow: onlineWindow}
}

// statusRank: lower is healthier. Used to pick the worst status across agents.
var statusRank = map[string]int{
	"ok":          0,
	"expiring":    1,
	"mismatch":    2,
	"expired":     3,
	"unreachable": 4,
}

func worstStatus(a, b string) string {
	if a == "" {
		return b
	}
	if statusRank[b] > statusRank[a] {
		return b
	}
	return a
}

func (h *DashboardHandler) Overview(c *gin.Context) {
	domains, err := h.store.ListAllDomains()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	results, err := h.store.LatestResults()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	online, total, err := h.store.CountAgents(h.onlineWindow)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}

	// Domain is "alert" if ANY latest result is non-ok. "Healthy" if at least one
	// result and all are ok. Domains with zero results count as neither.
	domainStatus := make(map[uint]string)
	for _, r := range results {
		domainStatus[r.DomainID] = worstStatus(domainStatus[r.DomainID], r.Status)
	}
	healthy, alert := 0, 0
	for _, d := range domains {
		s, ok := domainStatus[d.ID]
		if !ok {
			continue
		}
		if s == "ok" {
			healthy++
		} else {
			alert++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_domains":   len(domains),
		"healthy_domains": healthy,
		"alert_domains":   alert,
		"agents_online":   online,
		"agents_total":    total,
	})
}

func (h *DashboardHandler) Domains(c *gin.Context) {
	domains, err := h.store.ListAllDomains()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	results, err := h.store.LatestResults()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	type stats struct {
		healthy int
		total   int
		worst   string
	}
	per := make(map[uint]*stats)
	for _, r := range results {
		st := per[r.DomainID]
		if st == nil {
			st = &stats{}
			per[r.DomainID] = st
		}
		st.total++
		if r.Status == "ok" {
			st.healthy++
		}
		st.worst = worstStatus(st.worst, r.Status)
	}

	out := make([]gin.H, 0, len(domains))
	for _, d := range domains {
		st := per[d.ID]
		row := gin.H{
			"id":            d.ID,
			"host":          d.Host,
			"port":          d.Port,
			"protocol":      d.Protocol,
			"is_global":     d.IsGlobal,
			"remark":        d.Remark,
			"healthy_count": 0,
			"total_checks":  0,
			"worst_status":  "",
		}
		if st != nil {
			row["healthy_count"] = st.healthy
			row["total_checks"] = st.total
			row["worst_status"] = st.worst
		}
		out = append(out, row)
	}
	c.JSON(http.StatusOK, gin.H{"domains": out})
}

func (h *DashboardHandler) DomainDetail(c *gin.Context) {
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
	results, err := h.store.LatestResultsForDomain(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	// Join agent display names. Build agent map once.
	agents, err := h.store.ListAgents()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "db_error", "message": err.Error()}})
		return
	}
	agentMap := make(map[string]store.Agent, len(agents))
	for _, a := range agents {
		agentMap[a.AgentID] = a
	}

	threshold := time.Now().Add(-h.onlineWindow)
	out := make([]gin.H, 0, len(results))
	for _, r := range results {
		a := agentMap[r.AgentID]
		out = append(out, gin.H{
			"agent_id":           r.AgentID,
			"agent_display_name": a.DisplayName,
			"agent_online":       !a.LastSeenAt.IsZero() && a.LastSeenAt.After(threshold),
			"checked_at":         r.CheckedAt,
			"status":             r.Status,
			"not_after":          r.NotAfter,
			"issuer":             r.Issuer,
			"subject":            r.Subject,
			"sans":               r.SANs,
			"error_message":      r.ErrorMessage,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"domain": gin.H{
			"id":       domain.ID,
			"host":     domain.Host,
			"port":     domain.Port,
			"protocol": domain.Protocol,
			"remark":   domain.Remark,
		},
		"results": out,
	})
}

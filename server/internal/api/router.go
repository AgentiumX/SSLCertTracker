package api

import (
	"github.com/gin-gonic/gin"
	"ssl-tracker/server/internal/auth"
	"ssl-tracker/server/internal/store"
)

func SetupRouter(s *store.Store, agentToken string, expireThresholdDays int) *gin.Engine {
	r := gin.Default()

	agentGroup := r.Group("/api/agent")
	agentGroup.Use(auth.AgentTokenMiddleware(agentToken))
	{
		h := NewAgentHandler(s, expireThresholdDays)
		agentGroup.POST("/register", h.Register)
		agentGroup.GET("/domains", h.GetDomains)
		agentGroup.POST("/results", h.PostResults)
	}

	adminGroup := r.Group("/api/admin")
	{
		h := NewAdminHandler(s)
		adminGroup.POST("/domains", h.CreateDomain)
		adminGroup.GET("/domains", h.ListDomains)
		adminGroup.GET("/domains/:id", h.GetDomain)
		adminGroup.DELETE("/domains/:id", h.DeleteDomain)
	}

	return r
}

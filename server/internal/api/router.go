package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"ssl-tracker/server/internal/auth"
	"ssl-tracker/server/internal/store"
)

func SetupRouter(s *store.Store, agentToken string, expireThresholdDays int,
	sessions *auth.SessionStore, cookieSecure bool, webHandler http.Handler) *gin.Engine {
	r := gin.Default()

	agentGroup := r.Group("/api/agent")
	agentGroup.Use(auth.AgentTokenMiddleware(agentToken))
	{
		h := NewAgentHandler(s, expireThresholdDays)
		agentGroup.POST("/register", h.Register)
		agentGroup.GET("/domains", h.GetDomains)
		agentGroup.POST("/results", h.PostResults)
	}

	dash := r.Group("/api/dashboard")
	{
		h := NewDashboardHandler(s, 3*time.Hour)
		dash.GET("/overview", h.Overview)
		dash.GET("/domains", h.Domains)
		dash.GET("/domains/:id", h.DomainDetail)
	}

	authH := NewAuthHandler(s, sessions, cookieSecure)
	r.POST("/api/auth/login", authH.Login)
	r.POST("/api/auth/logout", authH.Logout)
	r.GET("/api/auth/me", auth.AuthMiddleware(sessions), authH.Me)

	adminGroup := r.Group("/api/admin")
	adminGroup.Use(auth.AuthMiddleware(sessions))
	{
		h := NewAdminHandler(s)
		adminGroup.POST("/domains", h.CreateDomain)
		adminGroup.GET("/domains", h.ListDomains)
		adminGroup.GET("/domains/:id", h.GetDomain)
		adminGroup.DELETE("/domains/:id", h.DeleteDomain)
	}

	if webHandler != nil {
		r.NoRoute(gin.WrapH(webHandler))
	}
	return r
}

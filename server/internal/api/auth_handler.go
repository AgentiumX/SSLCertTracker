package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"ssl-tracker/server/internal/auth"
	"ssl-tracker/server/internal/store"
)

type AuthHandler struct {
	store        *store.Store
	sessions     *auth.SessionStore
	cookieSecure bool
}

func NewAuthHandler(s *store.Store, sessions *auth.SessionStore, cookieSecure bool) *AuthHandler {
	return &AuthHandler{store: s, sessions: sessions, cookieSecure: cookieSecure}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func unauthenticated(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error": gin.H{"code": "unauthenticated", "message": "not logged in"},
	})
}

func invalidCredentials(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error": gin.H{"code": "invalid_credentials", "message": "invalid credentials"},
	})
}

func (h *AuthHandler) setSessionCookie(c *gin.Context, sid string) {
	maxAge := int(h.sessions.TTL().Seconds())
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(auth.SessionCookieName, sid, maxAge, "/", "", h.cookieSecure, true)
}

func (h *AuthHandler) clearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(auth.SessionCookieName, "", -1, "/", "", h.cookieSecure, true)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"code": "bad_request", "message": err.Error()},
		})
		return
	}

	user, err := h.store.GetUserByUsername(req.Username)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Run bcrypt against dummy hash so unknown-user response time matches
		// the wrong-password branch (defense against timing-based username enumeration).
		_ = auth.VerifyPassword(auth.DummyHash, req.Password)
		invalidCredentials(c)
		return
	}
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"code": "server_error", "message": err.Error()},
		})
		return
	}

	if !auth.VerifyPassword(user.PasswordHash, req.Password) {
		invalidCredentials(c)
		return
	}

	sid := h.sessions.Create(user.ID, user.Username)
	h.setSessionCookie(c, sid)
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{"id": user.ID, "username": user.Username},
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if sid, err := c.Cookie(auth.SessionCookieName); err == nil {
		h.sessions.Delete(sid)
	}
	h.clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthHandler) Me(c *gin.Context) {
	// AuthMiddleware already validated and refreshed the session; we just need
	// to re-issue Set-Cookie so browser Max-Age stays in sync with server TTL.
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")
	if sid, err := c.Cookie(auth.SessionCookieName); err == nil {
		h.setSessionCookie(c, sid)
	}
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{"id": userID, "username": username},
	})
}

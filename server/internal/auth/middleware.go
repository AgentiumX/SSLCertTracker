package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const SessionCookieName = "session_id"

// AuthMiddleware ensures the request carries a valid session cookie.
// On success it sets c.Keys["user_id"] and c.Keys["username"].
// On failure it aborts with 401.
func AuthMiddleware(store *SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, err := c.Cookie(SessionCookieName)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": "unauthenticated", "message": "not logged in"},
			})
			return
		}
		sess, ok := store.Get(sid)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{"code": "unauthenticated", "message": "not logged in"},
			})
			return
		}
		c.Set("user_id", sess.UserID)
		c.Set("username", sess.Username)
		c.Next()
	}
}

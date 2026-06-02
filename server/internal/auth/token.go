package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func AgentTokenMiddleware(expectedToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "missing_token", "message": "Authorization header required"}})
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" || parts[1] != expectedToken {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "invalid_token", "message": "Invalid agent token"}})
			return
		}
		c.Next()
	}
}

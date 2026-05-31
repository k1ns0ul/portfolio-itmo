package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	CtxUserID = "auth.user_id"
	CtxPhone  = "auth.phone"
)

func Middleware(t *Tokenizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(strings.ToLower(raw), "bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		tok := strings.TrimSpace(raw[len("bearer "):])
		claims, err := t.Parse(tok)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		if claims.Kind != TokenAccess {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "wrong token kind"})
			return
		}
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxPhone, claims.Phone)
		c.Next()
	}
}

func UserID(c *gin.Context) int64 {
	v, _ := c.Get(CtxUserID)
	if id, ok := v.(int64); ok {
		return id
	}
	return 0
}

func MustUserID(c *gin.Context) (int64, bool) {
	id := UserID(c)
	if id == 0 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return 0, false
	}
	return id, true
}

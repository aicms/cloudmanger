package middleware

import (
	"net/http"
	"strings"

	"cloudmanager/internal/auth"
	"cloudmanager/internal/model"

	"github.com/gin-gonic/gin"
)

// Context keys
const (
	CtxUserID   = "user_id"
	CtxUsername = "username"
	CtxRole     = "role"
)

// JWT 校验中间件
func JWT(jwtMgr *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.JSON(http.StatusUnauthorized, model.Fail("未登录"))
			c.Abort()
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := jwtMgr.Parse(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, model.Fail("登录已过期或令牌无效"))
			c.Abort()
			return
		}
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxUsername, claims.Username)
		c.Set(CtxRole, claims.Role)
		c.Next()
	}
}

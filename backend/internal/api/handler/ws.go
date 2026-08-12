package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloudmanager/internal/middleware"
	"cloudmanager/internal/ws"

	"github.com/gin-gonic/gin"
)

// WebShell 建立到目标服务器公网 IP 的 SSH 远程终端（WebSocket）。
// 鉴权令牌通过 query 参数 token 传递：浏览器无法在 WS 握手中自定义 Authorization 头，
// 故采用与 JWT 中间件一致的令牌，在 handler 内手动校验。
func (h *H) WebShell(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		token = strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	}
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}
	claims, err := h.Svc.JWT.Parse(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "令牌无效或已过期"})
		return
	}
	// 回填上下文，便于复用 recordOp 记录审计日志
	c.Set(middleware.CtxUserID, claims.UserID)
	c.Set(middleware.CtxUsername, claims.Username)

	id, _ := strconv.ParseUint(c.Query("id"), 10, 64)
	if id == 0 {
		// 兼容 /ws/webshell/:id 形式的旧路由
		id, _ = strconv.ParseUint(c.Param("id"), 10, 64)
	}
	s, err := h.Svc.Server.FindByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "服务器不存在"})
		return
	}
	if s.PublicIP == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "该服务器无公网 IP，无法建立远程终端"})
		return
	}

	connected := ws.HandleWebShell(c.Writer, c.Request, ws.SSHOptions{
		Host:    s.PublicIP,
		Port:    22,
		Timeout: 15 * time.Second,
	})

	if connected {
		h.recordOp(c, "server.webshell", strconv.FormatUint(id, 10), fmt.Sprintf("远程终端 %s", s.PublicIP), "success")
	} else {
		h.recordOp(c, "server.webshell", strconv.FormatUint(id, 10), fmt.Sprintf("远程终端连接失败 %s", s.PublicIP), "failed")
	}
}

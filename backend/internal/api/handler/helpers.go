package handler

import (
	"net/http"

	"cloudmanager/internal/middleware"
	"cloudmanager/internal/model"
	"cloudmanager/internal/service"

	"github.com/gin-gonic/gin"
)

// H handler 基类，持有 service 容器
type H struct {
	Svc *service.Services
}

func New(svc *service.Services) *H { return &H{Svc: svc} }

// JSON 成功响应
func JSON(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, model.Ok(data))
}

// FailErr 失败响应
func FailErr(c *gin.Context, code int, err error) {
	c.JSON(http.StatusOK, model.FailWith(code, err.Error()))
}

// FailMsg 失败响应（自定义消息）
func FailMsg(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, model.Fail(msg))
}

// currentUsername 从上下文取当前用户名
func currentUsername(c *gin.Context) string {
	if v, ok := c.Get(middleware.CtxUsername); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func currentUserID(c *gin.Context) uint {
	if v, ok := c.Get(middleware.CtxUserID); ok {
		if id, ok := v.(uint); ok {
			return id
		}
	}
	return 0
}

// recordOp 记录操作日志（best-effort，不阻塞主流程）
func (h *H) recordOp(c *gin.Context, action, target, detail, status string) {
	_ = h.Svc.OperationLog.Record(&model.OperationLog{
		UserID:   currentUserID(c),
		Username: currentUsername(c),
		Action:   action,
		Target:   target,
		Detail:   detail,
		IP:       c.ClientIP(),
		Status:   status,
	})
}

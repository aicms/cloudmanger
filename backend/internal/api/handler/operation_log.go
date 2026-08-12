package handler

import (
	"cloudmanager/internal/model"

	"github.com/gin-gonic/gin"
)

// ListOperationLogs 操作日志列表
func (h *H) ListOperationLogs(c *gin.Context) {
	var q model.OperationLogQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		FailMsg(c, "参数错误")
		return
	}
	result, err := h.Svc.OperationLog.Paginate(q)
	if err != nil {
		FailMsg(c, err.Error())
		return
	}
	JSON(c, result)
}

package handler

import (
	"fmt"
	"strconv"

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

// ClearOperationLogs 清除 N 天前的操作日志（days 缺省为 7）
func (h *H) ClearOperationLogs(c *gin.Context) {
	days := 7
	if v := c.Query("days"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			FailMsg(c, "参数错误")
			return
		}
		days = n
	}
	n, err := h.Svc.OperationLog.Clear(days)
	if err != nil {
		FailMsg(c, err.Error())
		return
	}
	h.recordOp(c, "log.clear", "操作日志", fmt.Sprintf("清除 %d 天前的日志，共 %d 条", days, n), "success")
	JSON(c, gin.H{"deleted": n})
}

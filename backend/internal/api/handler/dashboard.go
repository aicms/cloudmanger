package handler

import "github.com/gin-gonic/gin"

// Overview 仪表盘汇总
func (h *H) DashboardOverview(c *gin.Context) {
	o, err := h.Svc.Dashboard.Overview()
	if err != nil {
		FailMsg(c, err.Error())
		return
	}
	JSON(c, o)
}

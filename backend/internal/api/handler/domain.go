package handler

import (
	"strconv"

	"cloudmanager/internal/model"

	"github.com/gin-gonic/gin"
)

// ListDomains 域名列表
func (h *H) ListDomains(c *gin.Context) {
	var q model.DomainQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		FailMsg(c, "参数错误")
		return
	}
	result, err := h.Svc.Domain.Paginate(q)
	if err != nil {
		FailMsg(c, err.Error())
		return
	}
	JSON(c, result)
}

// GetDomain 域名详情
func (h *H) GetDomain(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	d, err := h.Svc.Domain.FindByID(uint(id))
	if err != nil {
		FailMsg(c, "域名不存在")
		return
	}
	JSON(c, d)
}

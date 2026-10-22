package handler

import (
	"strconv"

	"cloudmanager/internal/model"

	"github.com/gin-gonic/gin"
)

// ListDNSRecords 解析记录列表
func (h *H) ListDNSRecords(c *gin.Context) {
	var q model.DNSRecordQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		FailMsg(c, "参数错误")
		return
	}
	result, err := h.Svc.DNSRecord.Paginate(q)
	if err != nil {
		FailMsg(c, err.Error())
		return
	}
	JSON(c, result)
}

// CreateDNSRecord 新增解析记录
func (h *H) CreateDNSRecord(c *gin.Context) {
	var req model.CreateDNSRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailMsg(c, "参数错误")
		return
	}
	rec, err := h.Svc.DNSRecord.Create(req)
	if err != nil {
		h.recordOp(c, "dns.create", req.Host, err.Error(), "failed")
		FailMsg(c, err.Error())
		return
	}
	h.recordOp(c, "dns.create", req.Host, "新增解析记录", "success")
	JSON(c, rec)
}

// UpdateDNSRecord 修改解析记录
func (h *H) UpdateDNSRecord(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.UpdateDNSRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailMsg(c, "参数错误")
		return
	}
	rec, err := h.Svc.DNSRecord.Update(uint(id), req)
	if err != nil {
		h.recordOp(c, "dns.update", c.Param("id"), err.Error(), "failed")
		FailMsg(c, err.Error())
		return
	}
	h.recordOp(c, "dns.update", c.Param("id"), "修改解析记录", "success")
	JSON(c, rec)
}

// DeleteDNSRecord 删除解析记录
func (h *H) DeleteDNSRecord(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.Svc.DNSRecord.Delete(uint(id)); err != nil {
		h.recordOp(c, "dns.delete", c.Param("id"), err.Error(), "failed")
		FailMsg(c, err.Error())
		return
	}
	h.recordOp(c, "dns.delete", c.Param("id"), "删除解析记录", "success")
	JSON(c, gin.H{"success": true})
}

package handler

import (
	"fmt"
	"strconv"

	"cloudmanager/internal/model"

	"github.com/gin-gonic/gin"
)

// ListServers 云服务器列表
func (h *H) ListServers(c *gin.Context) {
	var q model.ServerQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		FailMsg(c, "参数错误")
		return
	}
	result, err := h.Svc.Server.Paginate(q)
	if err != nil {
		FailMsg(c, err.Error())
		return
	}
	JSON(c, result)
}

// ServerMeta 返回筛选项元数据（地域、标签列表）
func (h *H) ServerMeta(c *gin.Context) {
	regions, _ := h.Svc.Server.ListRegions()
	tags, _ := h.Svc.Server.ListTags()
	JSON(c, gin.H{"regions": regions, "tags": tags})
}

// UpdateServer 更新用户可编辑字段：自定义名称、标签
func (h *H) UpdateServer(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var body struct {
		CustomName string `json:"custom_name"`
		Tags       string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		FailMsg(c, "参数错误")
		return
	}
	if err := h.Svc.Server.UpdateMeta(uint(id), body.CustomName, body.Tags); err != nil {
		FailMsg(c, err.Error())
		return
	}
	h.recordOp(c, "server.edit", c.Param("id"), "更新名称/标签", "success")
	JSON(c, gin.H{"success": true})
}

// BatchServerAction 按筛选条件（含标签）批量操作
func (h *H) BatchServerAction(c *gin.Context) {
	var req model.ServerBatchActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailMsg(c, "参数错误")
		return
	}
	n, err := h.Svc.Server.BatchAction(req)
	if err != nil {
		FailMsg(c, err.Error())
		return
	}
	h.recordOp(c, "server."+req.Action, "batch", fmt.Sprintf("批量操作 %d 台服务器", n), "success")
	JSON(c, gin.H{"success": true, "count": n})
}

// GetServer 服务器详情
func (h *H) GetServer(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	s, err := h.Svc.Server.FindByID(uint(id))
	if err != nil {
		FailMsg(c, "服务器不存在")
		return
	}
	JSON(c, s)
}

// ServerAction 开机/关机/重启
func (h *H) ServerAction(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.ServerActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailMsg(c, "参数错误")
		return
	}
	if err := h.Svc.Server.Action(uint(id), req.Action); err != nil {
		h.recordOp(c, "server."+req.Action, c.Param("id"), err.Error(), "failed")
		FailMsg(c, err.Error())
		return
	}
	h.recordOp(c, "server."+req.Action, c.Param("id"), "操作成功", "success")
	JSON(c, gin.H{"success": true})
}

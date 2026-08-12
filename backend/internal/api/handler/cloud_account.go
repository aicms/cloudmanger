package handler

import (
	"fmt"
	"io"
	"strconv"

	"cloudmanager/internal/model"

	"github.com/gin-gonic/gin"
)

// List 云账号列表
func (h *H) ListCloudAccounts(c *gin.Context) {
	list, err := h.Svc.CloudAccount.List()
	if err != nil {
		FailMsg(c, err.Error())
		return
	}
	JSON(c, list)
}

// Create 新增云账号
func (h *H) CreateCloudAccount(c *gin.Context) {
	var req model.CreateCloudAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailMsg(c, "参数错误")
		return
	}
	if err := h.Svc.CloudAccount.Create(req); err != nil {
		FailMsg(c, err.Error())
		return
	}
	h.recordOp(c, "cloud_account.create", req.Name, "新增云账号", "success")
	JSON(c, gin.H{"success": true})
}

// Update 编辑云账号
func (h *H) UpdateCloudAccount(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.CreateCloudAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailMsg(c, "参数错误")
		return
	}
	if err := h.Svc.CloudAccount.Update(uint(id), req); err != nil {
		FailMsg(c, err.Error())
		return
	}
	h.recordOp(c, "cloud_account.update", req.Name, "编辑云账号", "success")
	JSON(c, gin.H{"success": true})
}

// ImportCloudAccounts 从 CSV 文件批量导入云账号（阿里云 AccessKey / 腾讯云 SecretKey 官方导出格式）
func (h *H) ImportCloudAccounts(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		FailMsg(c, "请选择要导入的 CSV 文件")
		return
	}
	f, err := file.Open()
	if err != nil {
		FailMsg(c, "文件读取失败")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		FailMsg(c, "文件读取失败")
		return
	}
	result, err := h.Svc.CloudAccount.ImportCSV(data)
	if err != nil {
		h.recordOp(c, "cloud_account.import", file.Filename, err.Error(), "failed")
		FailMsg(c, err.Error())
		return
	}
	if result.Failed > 0 {
		h.recordOp(c, "cloud_account.import", file.Filename,
			fmt.Sprintf("导入 %d 条，成功 %d，失败 %d", result.Total, result.Success, result.Failed), "failed")
	} else {
		h.recordOp(c, "cloud_account.import", file.Filename,
			fmt.Sprintf("导入 %d 条，全部成功", result.Total), "success")
	}
	JSON(c, result)
}

// Delete 删除云账号
func (h *H) DeleteCloudAccount(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.Svc.CloudAccount.Delete(uint(id)); err != nil {
		FailMsg(c, err.Error())
		return
	}
	h.recordOp(c, "cloud_account.delete", c.Param("id"), "删除云账号", "success")
	JSON(c, gin.H{"success": true})
}

// Toggle 启用/禁用
func (h *H) ToggleCloudAccount(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		FailMsg(c, "参数错误")
		return
	}
	if err := h.Svc.CloudAccount.Toggle(uint(id), req.Enabled); err != nil {
		FailMsg(c, err.Error())
		return
	}
	JSON(c, gin.H{"success": true})
}

// Sync 触发同步，返回分类计数与错误明细
func (h *H) SyncCloudAccount(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	result, err := h.Svc.CloudAccount.Sync(uint(id))
	if result == nil {
		h.recordOp(c, "cloud_account.sync", c.Param("id"), "同步失败", "failed")
		FailMsg(c, err.Error())
		return
	}
	if err != nil {
		h.recordOp(c, "cloud_account.sync", c.Param("id"), err.Error(), "failed")
	} else {
		h.recordOp(c, "cloud_account.sync", c.Param("id"), "同步成功", "success")
	}
	// 即使有部分错误也返回结果明细，由前端展示
	JSON(c, result)
}

// BatchSync 批量同步云账号（ids 为空时同步全部）
func (h *H) BatchSyncCloudAccounts(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		FailMsg(c, "参数错误")
		return
	}
	res, err := h.Svc.CloudAccount.BatchSync(req.IDs)
	if err != nil {
		FailMsg(c, err.Error())
		return
	}
	if res.Failed > 0 {
		h.recordOp(c, "cloud_account.batch_sync", "",
			fmt.Sprintf("批量同步 %d 个账号，成功 %d，失败 %d", res.Total, res.Success, res.Failed), "failed")
	} else {
		h.recordOp(c, "cloud_account.batch_sync", "",
			fmt.Sprintf("批量同步 %d 个账号，全部成功", res.Total), "success")
	}
	JSON(c, res)
}
func (h *H) TestCloudAccountConnection(c *gin.Context) {
	var req struct {
		Provider        string `json:"provider" binding:"required"`
		AccessKeyID     string `json:"access_key_id" binding:"required"`
		AccessKeySecret string `json:"access_key_secret" binding:"required"`
		Region          string `json:"region"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		FailMsg(c, "参数错误")
		return
	}
	if err := h.Svc.CloudAccount.TestConnection(req.Provider, req.AccessKeyID, req.AccessKeySecret, req.Region); err != nil {
		JSON(c, gin.H{"success": false, "error": err.Error()})
		return
	}
	JSON(c, gin.H{"success": true})
}

// TestCloudAccountConnectionByID 用已保存账号的凭据测试连接
func (h *H) TestCloudAccountConnectionByID(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.Svc.CloudAccount.TestConnectionByID(uint(id)); err != nil {
		JSON(c, gin.H{"success": false, "error": err.Error()})
		return
	}
	JSON(c, gin.H{"success": true})
}

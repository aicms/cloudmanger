package handler

import (
	"net/http"

	"cloudmanager/internal/model"

	"github.com/gin-gonic/gin"
)

// Login 登录
func (h *H) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailMsg(c, "参数错误")
		return
	}
	resp, err := h.Svc.Auth.Login(req)
	if err != nil {
		h.recordOp(c, "login", req.Username, err.Error(), "failed")
		FailMsg(c, err.Error())
		return
	}
	h.recordOp(c, "login", req.Username, "登录成功", "success")
	JSON(c, resp)
}

// Profile 当前用户信息
func (h *H) Profile(c *gin.Context) {
	u, err := h.Svc.Auth.GetProfile(currentUserID(c))
	if err != nil {
		c.JSON(http.StatusOK, model.Fail("用户不存在"))
		return
	}
	JSON(c, u)
}

// ChangePassword 修改密码
func (h *H) ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password" binding:"min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		FailMsg(c, "新密码至少 6 位")
		return
	}
	if err := h.Svc.Auth.ChangePassword(currentUserID(c), req.OldPassword, req.NewPassword); err != nil {
		h.recordOp(c, "password.change", currentUsername(c), err.Error(), "failed")
		FailMsg(c, err.Error())
		return
	}
	h.recordOp(c, "password.change", currentUsername(c), "修改成功", "success")
	JSON(c, gin.H{"success": true})
}

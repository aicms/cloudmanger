package api

import (
	"net/http"
	"strings"

	"cloudmanager/internal/api/handler"
	"cloudmanager/internal/middleware"
	"cloudmanager/internal/service"
	"cloudmanager/internal/webui"

	_ "cloudmanager/internal/cloud/aliyun"  // 注册阿里云 provider
	_ "cloudmanager/internal/cloud/tencent" // 注册腾讯云 provider

	"github.com/gin-gonic/gin"
)

// NewRouter 构建路由
func NewRouter(svc *service.Services) *gin.Engine {
	if svc.Cfg.IsDev {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())

	h := handler.New(svc)

	api := r.Group("/api")
	{
		// 认证（无需 JWT）
		api.POST("/auth/login", h.Login)

		// 受保护接口
		auth := api.Group("")
		auth.Use(middleware.JWT(svc.JWT))
		{
			auth.GET("/auth/profile", h.Profile)
			auth.PUT("/auth/password", h.ChangePassword)

			auth.GET("/dashboard/overview", h.DashboardOverview)

			auth.GET("/cloud-accounts", h.ListCloudAccounts)
			auth.POST("/cloud-accounts", h.CreateCloudAccount)
			auth.POST("/cloud-accounts/import", h.ImportCloudAccounts)
			auth.PUT("/cloud-accounts/:id", h.UpdateCloudAccount)
			auth.DELETE("/cloud-accounts/:id", h.DeleteCloudAccount)
			auth.PUT("/cloud-accounts/:id/toggle", h.ToggleCloudAccount)
			auth.POST("/cloud-accounts/:id/sync", h.SyncCloudAccount)
		auth.POST("/cloud-accounts/batch-sync", h.BatchSyncCloudAccounts)
			auth.POST("/cloud-accounts/test-connection", h.TestCloudAccountConnection)
			auth.POST("/cloud-accounts/:id/test-connection", h.TestCloudAccountConnectionByID)

			auth.GET("/servers", h.ListServers)
			auth.GET("/servers/meta", h.ServerMeta)
			auth.GET("/servers/:id", h.GetServer)
			auth.PUT("/servers/:id", h.UpdateServer)
			auth.POST("/servers/:id/action", h.ServerAction)
			auth.POST("/servers/batch-action", h.BatchServerAction)

			auth.GET("/domains", h.ListDomains)
			auth.GET("/domains/:id", h.GetDomain)

			auth.GET("/dns-records", h.ListDNSRecords)
			auth.POST("/dns-records", h.CreateDNSRecord)
			auth.PUT("/dns-records/:id", h.UpdateDNSRecord)
			auth.DELETE("/dns-records/:id", h.DeleteDNSRecord)

			auth.GET("/operation-logs", h.ListOperationLogs)

			auth.GET("/settings", h.GetSettings)
			auth.PUT("/settings", h.UpdateSettings)
		}
	}

	// WebSocket 远程终端（浏览器无法在 WS 握手时携带 Authorization 头，令牌经 ?token= 传递，故置于 JWT 中间件之外）
	r.GET("/ws/webshell", h.WebShell)

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 前端静态资源（内嵌于二进制）：命中静态文件则直出，否则回退 index.html 交由前端路由处理
	webFS := webui.FS()
	fileServer := http.FileServer(http.FS(webFS))
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/ws") {
			c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": "接口不存在"})
			return
		}
		if trimmed := strings.TrimPrefix(p, "/"); trimmed != "" {
			if f, err := webFS.Open(trimmed); err == nil {
				f.Close()
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})

	return r
}

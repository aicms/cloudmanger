package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"cloudmanager/config"
	"cloudmanager/internal/api"
	"cloudmanager/internal/logging"
	"cloudmanager/internal/repository"
	"cloudmanager/internal/service"
	"cloudmanager/migrations"
)

func main() {
	cfg := config.Load()

	// 日志：标准输出 + logs/ 目录滚动落盘，尽早初始化以便后续所有日志（含启动失败）都能落盘
	if err := logging.Init("./logs"); err != nil {
		log.Fatalf("日志初始化失败: %v", err)
	}
	defer logging.Close()
	defer logging.Recover("main")

	// 确保数据目录存在
	if err := os.MkdirAll("./data", 0755); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}

	// 初始化数据库
	if err := repository.Init(cfg); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 种子数据（默认管理员 + 示例云账号 + 首次同步）
	if err := migrations.Seed(repository.DB, cfg); err != nil {
		log.Printf("种子数据警告: %v", err)
	}

	// 构建业务层
	svc := service.New(cfg, repository.DB)

	// 启动自动同步调度器（按设置中的 sync_interval 定时同步全部云账号）
	svc.CloudAccount.StartAutoSync()

	// 启动 HTTP 服务
	r := api.NewRouter(svc)
	addr := cfg.Host + ":" + cfg.Port
	fmt.Printf("\n🚀 多云管理平台后端已启动\n")
	fmt.Printf("   监听: %s (http://localhost:%s)\n", addr, cfg.Port)
	fmt.Printf("   数据库: %s\n", cfg.DBPath)
	fmt.Printf("   日志: ./logs/app.log\n")
	fmt.Printf("   模式: %s\n\n", map[bool]string{true: "开发", false: "生产"}[cfg.IsDev])

	go func() {
		defer logging.Recover("http-server")
		if err := r.Run(addr); err != nil {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("\n服务关闭中...")
}

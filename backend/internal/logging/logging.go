// Package logging 提供全局文件日志与后台协程的异常兜底。
// 目标：
//  1. 所有日志（标准 log 包 / gin 请求日志 / gin.Recovery）统一落盘到 logs/ 目录，滚动切割，避免单文件无限增长；
//  2. 后台协程（多地域并发同步、自动同步调度、WebSocket 终端桥接等）默认没有 panic 保护——
//     一旦某个协程 panic 会直接终止整个进程。Recover 用于在这些协程入口处兜底，
//     记录堆栈后让协程退出，而不是拖垮整个服务。
package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"
)

var fileWriter *lumberjack.Logger

// Init 初始化日志：标准输出 + logs/app.log（10MB 滚动，保留 10 个备份、最长 30 天，自动压缩旧文件）。
// 同时接管 gin 的默认输出/错误输出，使请求日志与 panic 堆栈也写入同一份日志文件。
func Init(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}
	fileWriter = &lumberjack.Logger{
		Filename:   filepath.Join(dir, "app.log"),
		MaxSize:    10, // MB
		MaxBackups: 10,
		MaxAge:     30, // 天
		Compress:   true,
	}
	out := io.MultiWriter(os.Stdout, fileWriter)
	log.SetOutput(out)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	gin.DefaultWriter = out
	gin.DefaultErrorWriter = out
	return nil
}

// Close 关闭日志文件句柄，在进程退出前调用即可（非必需，但便于优雅关闭时落盘）。
func Close() {
	if fileWriter != nil {
		_ = fileWriter.Close()
	}
}

// Recover 用于后台协程入口处 defer，捕获 panic 并记录堆栈，避免单个协程崩溃拖垮整个进程。
// 用法：go func() { defer logging.Recover("tag"); ... }()
func Recover(tag string) {
	if r := recover(); r != nil {
		log.Printf("[panic][%s] %v\n%s", tag, r, debug.Stack())
	}
}

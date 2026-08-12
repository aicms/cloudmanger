// Package webui 内嵌前端构建产物（frontend/dist），使后端可编译为单文件可执行程序。
// 打包发布前需先执行 `pnpm build`，并将 frontend/dist/* 拷贝到本目录下的 dist/ 中再 go build。
package webui

import (
	"embed"
	"io/fs"
	"mime"
)

//go:embed all:dist
var distFS embed.FS

// 部分 Windows 主机的注册表缺失/篡改了 MIME 映射（如把 .svg 识别为 image/svg 而非
// image/svg+xml），导致浏览器拒绝渲染内嵌资源（如 logo）。这里显式固定常见静态资源类型，
// 不依赖宿主系统的 mime 注册表。
func init() {
	for ext, typ := range map[string]string{
		".svg":   "image/svg+xml",
		".js":    "application/javascript; charset=utf-8",
		".css":   "text/css; charset=utf-8",
		".json":  "application/json; charset=utf-8",
		".png":   "image/png",
		".ico":   "image/x-icon",
		".woff":  "font/woff",
		".woff2": "font/woff2",
	} {
		_ = mime.AddExtensionType(ext, typ)
	}
}

// FS 返回前端静态资源的根文件系统（即 dist/ 下的内容）。
func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

# 多云管理平台 - 一键开发启动脚本
# 同时启动后端(:8002)与前端(:5173)

$ErrorActionPreference = "SilentlyContinue"
$ROOT = Split-Path -Parent $MyInvocation.MyCommand.Path

Write-Host "==> 启动后端 (Go :8002，开发模式) ..." -ForegroundColor Cyan
$env:GIN_MODE = "debug"
$be = Start-Process -FilePath "go" -ArgumentList "run","./cmd/server" -WorkingDirectory "$ROOT/backend" -PassThru -WindowStyle Minimized

Start-Sleep -Seconds 3

Write-Host "==> 启动前端 (Vite :5173) ..." -ForegroundColor Cyan
$fe = Start-Process -FilePath "pnpm" -ArgumentList "dev" -WorkingDirectory "$ROOT/frontend" -PassThru -WindowStyle Minimized

Start-Sleep -Seconds 3
Write-Host ""
Write-Host "==========================================" -ForegroundColor Green
Write-Host "  平台已启动" -ForegroundColor Green
Write-Host "  前端: http://localhost:5173" -ForegroundColor Yellow
Write-Host "  后端: http://localhost:8002" -ForegroundColor Yellow
Write-Host "  账号: admin / admin123" -ForegroundColor Yellow
Write-Host "==========================================" -ForegroundColor Green
Write-Host ""
Write-Host "按任意键停止所有服务..." -ForegroundColor Gray
[Console]::ReadKey($true) | Out-Null

Write-Host "正在停止服务..." -ForegroundColor Cyan
Stop-Process -Id $be.Id -Force
Stop-Process -Id $fe.Id -Force
Write-Host "已停止" -ForegroundColor Green

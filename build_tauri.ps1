# 多云管理平台 - Tauri 桌面客户端完整打包
# 流程: 构建前端 -> 构建 Go 后端并内嵌前端(sidecar) -> 编译 Rust 桌面壳 -> 收集产物 -> 清理构建缓存
# 产物: release\tauri\
#       cloud-manager-desktop.exe            桌面客户端(与 sidecar 同目录即可直接运行)
#       cloud-manager-<triple>.exe           Go 后端 sidecar(内嵌前端)
#       *-setup.exe                          NSIS 安装包(开箱即用)
# 用法: build_tauri.ps1            默认打包后清理 Rust 构建缓存(释放数 GB 磁盘)
#       build_tauri.ps1 -Keep      保留构建缓存,下次打包更快

param([switch]$Keep)

$ROOT = Split-Path -Parent $MyInvocation.MyCommand.Path
$OUT_DIR = "$ROOT/release/tauri"

function Fail($msg) {
    Write-Host "[错误] $msg" -ForegroundColor Red
    Pop-Location 2>$null
    exit 1
}

Write-Host "==> [1/5] 构建前端 ..." -ForegroundColor Cyan
Push-Location "$ROOT/frontend"
pnpm build
if ($LASTEXITCODE -ne 0) { Fail "前端构建失败" }
Pop-Location

Write-Host "==> [2/5] 构建 Go 后端 sidecar(内嵌前端) ..." -ForegroundColor Cyan
& powershell -NoProfile -ExecutionPolicy Bypass -File "$ROOT/tauri/build-sidecar.ps1"
if ($LASTEXITCODE -ne 0) { Fail "Go sidecar 构建失败(检查 go/rustc 环境)" }

Write-Host "==> [3/5] 编译 Tauri 桌面端并生成安装包(首次较慢,请耐心等待) ..." -ForegroundColor Cyan
Push-Location "$ROOT/tauri"
pnpm tauri build
if ($LASTEXITCODE -ne 0) { Fail "Tauri 编译失败" }
Pop-Location

Write-Host "==> [4/5] 收集产物到 release/tauri/ ..." -ForegroundColor Cyan
$triple = (rustc --print host-tuple).Trim()
New-Item -ItemType Directory -Force -Path $OUT_DIR | Out-Null
try {
    Copy-Item -Force "$ROOT/tauri/src-tauri/target/release/cloud-manager-desktop.exe" $OUT_DIR
    # 运行时 sidecar 按裸名解析(exe 同目录 cloud-manager.exe),triple 后缀只是构建期产物名
    Copy-Item -Force "$ROOT/tauri/src-tauri/binaries/cloud-manager-$triple.exe" "$OUT_DIR/cloud-manager.exe"
    Copy-Item -Force "$ROOT/tauri/src-tauri/target/release/bundle/nsis/*-setup.exe" $OUT_DIR
} catch {
    Fail "收集产物失败: $_"
}

Write-Host "==> [5/5] 清理构建缓存 ..." -ForegroundColor Cyan
if ($Keep) {
    Write-Host "    已跳过 (-Keep 模式,缓存保留)"
} else {
    $cache = "$ROOT/tauri/src-tauri/target"
    if (Test-Path $cache) {
        $gb = [math]::Round((Get-ChildItem -LiteralPath $cache -Recurse -Force -ErrorAction SilentlyContinue |
            Measure-Object Length -Sum).Sum / 1GB, 1)
        Push-Location "$ROOT/tauri/src-tauri"
        cargo clean
        Pop-Location
        Write-Host "    已清理 Rust 构建缓存 $gb GB"
    } else {
        Write-Host "    无缓存可清理"
    }
}

Write-Host ""
Write-Host "==========================================" -ForegroundColor Green
Write-Host "  桌面客户端打包完成,产物位于 release/tauri/" -ForegroundColor Green
Write-Host "  - cloud-manager-desktop.exe      直接运行即可" -ForegroundColor Yellow
Write-Host "    (启动自动拉起后端 :8002,退出自动结束)" -ForegroundColor Yellow
Write-Host "  - *-setup.exe                    NSIS 安装包" -ForegroundColor Yellow
Write-Host "==========================================" -ForegroundColor Green

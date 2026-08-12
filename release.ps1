# 多云管理平台 - 打包发布脚本
# 构建前端 -> 内嵌进 Go 二进制 -> 交叉编译多平台 -> 直接产出到 release/ 目录（不建日期子文件夹）
# 每次代码更新后重新执行本脚本即可得到最新发布包。
#
# 目标平台：
#   - windows/amd64 -> cloud-manager-windows-amd64.exe
#   - linux/arm64   -> cloud-manager-linux-arm64

$ErrorActionPreference = "Stop"
$ROOT = Split-Path -Parent $MyInvocation.MyCommand.Path
$DATE = Get-Date -Format "yyyyMMdd"
$RELEASE_DIR = "$ROOT/release"
$WEBUI_DIST = "$ROOT/backend/internal/webui/dist"

$TARGETS = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; Bin = "cloud-manager-windows-amd64.exe"; Ext = "windows-amd64" },
    @{ GOOS = "linux";   GOARCH = "arm64"; Bin = "cloud-manager-linux-arm64";       Ext = "linux-arm64" }
)

New-Item -ItemType Directory -Force -Path $RELEASE_DIR | Out-Null

Write-Host "==> 构建前端 ..." -ForegroundColor Cyan
Push-Location "$ROOT/frontend"
pnpm build
if ($LASTEXITCODE -ne 0) { Pop-Location; throw "前端构建失败" }
Pop-Location

Write-Host "==> 内嵌前端产物到 backend/internal/webui/dist ..." -ForegroundColor Cyan
if (Test-Path $WEBUI_DIST) { Remove-Item -Recurse -Force $WEBUI_DIST }
Copy-Item -Recurse "$ROOT/frontend/dist" $WEBUI_DIST

# ===== 准备 UPX（便携版，缓存在 .tools/，不装系统环境） =====
$UPX_DIR = "$ROOT/.tools"
$UPX_EXE = Get-ChildItem -Path $UPX_DIR -Filter "upx.exe" -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty FullName
if (-not $UPX_EXE) {
    try {
        Write-Host "==> 未找到本地 upx，正在下载便携版 ..." -ForegroundColor Cyan
        New-Item -ItemType Directory -Force -Path $UPX_DIR | Out-Null
        $upxZip = "$UPX_DIR/upx.zip"
        Invoke-WebRequest -Uri "https://github.com/upx/upx/releases/download/v5.0.1/upx-5.0.1-win64.zip" -OutFile $upxZip
        Expand-Archive -Path $upxZip -DestinationPath $UPX_DIR -Force
        Remove-Item -Force $upxZip
        $UPX_EXE = Get-ChildItem -Path $UPX_DIR -Filter "upx.exe" -Recurse -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty FullName
    } catch {
        Write-Host "    下载 upx 失败，跳过压缩：$_" -ForegroundColor Yellow
    }
}

Copy-Item -Force "$ROOT/scripts/release-template/.env.example" "$RELEASE_DIR/.env.example"
Copy-Item -Force "$ROOT/scripts/release-template/README.txt" "$RELEASE_DIR/README.txt"

foreach ($t in $TARGETS) {
    $binPath = "$RELEASE_DIR/$($t.Bin)"
    Write-Host "==> 编译 $($t.GOOS)/$($t.GOARCH) -> $($t.Bin) ..." -ForegroundColor Cyan
    Push-Location "$ROOT/backend"
    $env:GOOS = $t.GOOS
    $env:GOARCH = $t.GOARCH
    $env:CGO_ENABLED = "0"
    go build -trimpath -ldflags="-s -w" -o $binPath ./cmd/server
    $buildExit = $LASTEXITCODE
    Remove-Item Env:\GOOS, Env:\GOARCH, Env:\CGO_ENABLED
    Pop-Location
    if ($buildExit -ne 0) { throw "$($t.GOOS)/$($t.GOARCH) 构建失败" }

    if ($UPX_EXE) {
        & $UPX_EXE --best --lzma -q $binPath
        if ($LASTEXITCODE -ne 0) {
            Write-Host "    upx 压缩 $($t.Bin) 失败，保留未压缩版本" -ForegroundColor Yellow
        }
    }

    Write-Host "==> 打包 $($t.Ext) zip ..." -ForegroundColor Cyan
    $zipPath = "$RELEASE_DIR/cloudmanager-$DATE-$($t.Ext).zip"
    if (Test-Path $zipPath) { Remove-Item -Force $zipPath }
    Compress-Archive -Path $binPath, "$RELEASE_DIR/.env.example", "$RELEASE_DIR/README.txt" -DestinationPath $zipPath
}

Write-Host ""
Write-Host "==========================================" -ForegroundColor Green
Write-Host "  发布完成，产物位于 $RELEASE_DIR" -ForegroundColor Green
foreach ($t in $TARGETS) {
    Write-Host "  - $($t.Bin)" -ForegroundColor Yellow
    Write-Host "  - cloudmanager-$DATE-$($t.Ext).zip" -ForegroundColor Yellow
}
Write-Host "==========================================" -ForegroundColor Green

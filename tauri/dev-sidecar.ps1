# Used by `tauri dev` (beforeDevCommand) to bring up the Go backend on :8002
# BEFORE the Rust app launches. The Rust app detects the already-listening
# backend and skips spawning its own sidecar, avoiding a port conflict.
$ErrorActionPreference = "Stop"

$triple = (rustc --print host-tuple).Trim()
$exe = Join-Path $PSScriptRoot "src-tauri/binaries/cloud-manager-$triple.exe"
if (-not (Test-Path $exe)) {
    Write-Error "sidecar not found: $exe  (run 'pnpm build:sidecar' first)"
    exit 1
}

$name = [System.IO.Path]::GetFileNameWithoutExtension($exe)
Write-Host "Starting sidecar for tauri dev: $exe"
Start-Process -FilePath $exe

# Keep this script alive so Tauri tracks it as the dev command and can shut
# down the sidecar when the dev session ends.
Wait-Process -Name $name

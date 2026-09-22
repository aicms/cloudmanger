@echo off
rem CloudManager: build Tauri desktop client (frontend + Go sidecar + Rust + NSIS)
rem Logic lives in build_tauri.ps1 (this thin wrapper is ASCII-only on purpose)
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0build_tauri.ps1" %*
pause

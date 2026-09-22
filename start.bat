@echo off
rem CloudManager dev launcher: backend :8002 + frontend :5173 (logic in start.ps1)
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0start.ps1"
pause

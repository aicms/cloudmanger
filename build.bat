@echo off
setlocal
rem CloudManager local build: frontend -> embed -> backend\bin\cloud-manager.exe
rem (multi-platform release packaging: release.ps1)

set ROOT=%~dp0

echo ==^> [1/3] Building frontend ...
pushd "%ROOT%frontend"
call pnpm build
if errorlevel 1 goto :fail_frontend
popd

echo ==^> [2/3] Embedding frontend into backend\internal\webui\dist ...
if exist "%ROOT%backend\internal\webui\dist" rmdir /s /q "%ROOT%backend\internal\webui\dist"
xcopy /e /i /y /q "%ROOT%frontend\dist" "%ROOT%backend\internal\webui\dist" >nul
if errorlevel 1 goto :fail_embed

echo ==^> [3/3] Building backend ...
pushd "%ROOT%backend"
if not exist bin mkdir bin
go build -trimpath -ldflags="-s -w" -o bin\cloud-manager.exe .\cmd\server
if errorlevel 1 goto :fail_backend
popd

echo.
echo ==========================================
echo   Build OK: backend\bin\cloud-manager.exe
echo   Run and open http://localhost:8002
echo   Login: admin / admin123
echo ==========================================
pause
exit /b 0

:fail_frontend
popd 2>nul
echo [ERROR] Frontend build failed
pause
exit /b 1

:fail_embed
echo [ERROR] Embed frontend failed
pause
exit /b 1

:fail_backend
popd 2>nul
echo [ERROR] Backend build failed
pause
exit /b 1

@echo off
setlocal

cd /d "%~dp0"
title Show Me The Story - Wails Dev

where npm >nul 2>nul || goto missing_npm
where go >nul 2>nul || goto missing_go

for /f "delims=" %%G in ('go env GOPATH') do set "WAILS_EXE=%%G\bin\wails.exe"
if exist "%WAILS_EXE%" goto run_wails_exe

where wails >nul 2>nul
if not errorlevel 1 goto run_wails_path

echo [dev] Wails CLI was not found.
echo [dev] Install it once with:
echo        go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
echo [dev] Then run dev.bat again.
goto failed

:run_wails_exe
echo [dev] Starting Wails GUI with "%WAILS_EXE%"...
call "%WAILS_EXE%" dev
goto finished

:run_wails_path
echo [dev] Starting Wails GUI from PATH...
call wails dev
goto finished

:missing_npm
echo [dev] npm was not found. Install Node.js and try again.
goto failed

:missing_go
echo [dev] go was not found. Install Go or add it to PATH.
goto failed

:failed
echo.
echo [dev] Startup failed.
pause
exit /b 1

:finished
set "DEV_EXIT_CODE=%ERRORLEVEL%"
if not "%DEV_EXIT_CODE%"=="0" (
  echo.
  echo [dev] Wails development mode exited with code %DEV_EXIT_CODE%.
  pause
)
exit /b %DEV_EXIT_CODE%

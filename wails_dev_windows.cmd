@echo off
setlocal EnableExtensions EnableDelayedExpansion

for %%I in ("%~dp0.") do set "PROJECT_ROOT=%%~fI"
set "MSYS64=%MSYS64_ROOT%"
if not defined MSYS64 set "MSYS64=C:\DevDisk\DevTools\msys64"
set "MINGW_BIN=%MSYS64%\mingw64\bin"
set "FREERDP_INSTALL=%PROJECT_ROOT%\install"

where wails >nul 2>nul
if errorlevel 1 (
    echo ERROR: wails not found in PATH
    exit /b 1
)
where go >nul 2>nul
if errorlevel 1 (
    echo ERROR: go not found in PATH
    exit /b 1
)
where node >nul 2>nul
if errorlevel 1 (
    echo ERROR: node not found in PATH
    exit /b 1
)

if not exist "%FREERDP_INSTALL%\bin\libfreerdp3.dll" (
    echo ERROR: missing %FREERDP_INSTALL%\bin\libfreerdp3.dll
    echo Please run: lib_build_windows.cmd
    exit /b 1
)

set "PATH=%MINGW_BIN%;%FREERDP_INSTALL%\bin;%PATH%"
set "OPENSSL_CONF=%PROJECT_ROOT%\openssl.cnf"
set "OPENSSL_MODULES=%FREERDP_INSTALL%\bin\ossl-modules"

cd /d "%PROJECT_ROOT%"
echo Starting Wails dev ^(Windows^)...
wails dev %*

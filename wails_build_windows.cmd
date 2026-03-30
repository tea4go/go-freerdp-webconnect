@echo off
setlocal EnableExtensions EnableDelayedExpansion

for %%I in ("%~dp0.") do set "PROJECT_ROOT=%%~fI"
set "MSYS64=%MSYS64_ROOT%"
if not defined MSYS64 set "MSYS64=C:\DevDisk\DevTools\msys64"
set "MINGW_BIN=%MSYS64%\mingw64\bin"
set "FREERDP_INSTALL=%PROJECT_ROOT%\install"
set "FREERDP_BIN=%FREERDP_INSTALL%\bin"
set "BUILD_BIN=%PROJECT_ROOT%\build\bin"
set "MINGW_RUNTIME_DLLS=libssl-3-x64.dll libcrypto-3-x64.dll zlib1.dll libgcc_s_seh-1.dll"

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

if not exist "%FREERDP_BIN%\libfreerdp3.dll" (
    echo ERROR: missing %FREERDP_BIN%\libfreerdp3.dll
    echo Please run: build_windows.cmd
    exit /b 1
)

if not exist "%FREERDP_BIN%" mkdir "%FREERDP_BIN%"
for %%F in (%MINGW_RUNTIME_DLLS%) do (
    if not exist "%MINGW_BIN%\%%F" (
        echo ERROR: missing %MINGW_BIN%\%%F
        echo Please install required packages in MSYS2 MinGW64.
        exit /b 1
    )
    copy /Y "%MINGW_BIN%\%%F" "%FREERDP_BIN%\" >nul
)

set "PATH=%MINGW_BIN%;%FREERDP_BIN%;%PATH%"
set "OPENSSL_CONF=%PROJECT_ROOT%\openssl.cnf"
set "OPENSSL_MODULES=%FREERDP_BIN%\ossl-modules"

cd /d "%PROJECT_ROOT%"
echo Building Wails package ^(Windows^)...
wails build -clean %*
if errorlevel 1 (
    echo ERROR: wails build failed
    exit /b 1
)

if not exist "%BUILD_BIN%" mkdir "%BUILD_BIN%"
for %%F in ("%FREERDP_BIN%\*.dll") do copy /Y "%%~fF" "%BUILD_BIN%\" >nul
if exist "%FREERDP_BIN%\ossl-modules" (
    xcopy /E /I /Y "%FREERDP_BIN%\ossl-modules" "%BUILD_BIN%\ossl-modules" >nul
)

echo Done: %BUILD_BIN%

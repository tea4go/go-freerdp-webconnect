@echo off
REM Go-FreeRDP-WebConnect Windows 运行脚本
setlocal enabledelayedexpansion

set "PROJECT_DIR=%~dp0"
REM 去掉末尾反斜杠
if "%PROJECT_DIR:~-1%"=="\" set "PROJECT_DIR=%PROJECT_DIR:~0,-1%"

set "MSYS64=C:\DevDisk\DevTools\msys64"
set "MINGW_BIN=%MSYS64%\mingw64\bin"
set "FREERDP_INSTALL=%PROJECT_DIR%\install"

REM 设置 DLL 搜索路径
set "PATH=%MINGW_BIN%;%FREERDP_INSTALL%\bin;%PATH%"

REM 启用 OpenSSL legacy provider（NLA/NTLM 认证依赖 MD4/RC4，OpenSSL 3.x 默认禁用）
set "OPENSSL_CONF=%PROJECT_DIR%\openssl.cnf"
set "OPENSSL_MODULES=%FREERDP_INSTALL%\bin\ossl-modules"

REM 默认参数
set "HOST="
set "PORT=53389"
set "USER="
set "PASS="
set "LISTEN=54455"

REM 解析命令行参数
:parse_args
if "%~1"=="" goto done_args
if /i "%~1"=="-h"       ( set "HOST=%~2"   & shift & shift & goto parse_args )
if /i "%~1"=="--host"   ( set "HOST=%~2"   & shift & shift & goto parse_args )
if /i "%~1"=="-P"       ( set "PORT=%~2"   & shift & shift & goto parse_args )
if /i "%~1"=="--port"   ( set "PORT=%~2"   & shift & shift & goto parse_args )
if /i "%~1"=="-u"       ( set "USER=%~2"   & shift & shift & goto parse_args )
if /i "%~1"=="--user"   ( set "USER=%~2"   & shift & shift & goto parse_args )
if /i "%~1"=="-p"       ( set "PASS=%~2"   & shift & shift & goto parse_args )
if /i "%~1"=="--pass"   ( set "PASS=%~2"   & shift & shift & goto parse_args )
if /i "%~1"=="-l"       ( set "LISTEN=%~2" & shift & shift & goto parse_args )
if /i "%~1"=="--listen" ( set "LISTEN=%~2" & shift & shift & goto parse_args )
if /i "%~1"=="--help"   goto show_help
echo 未知选项: %~1
exit /b 1

:show_help
echo 用法: %~n0 [选项]
echo   -h, --host     RDP 服务器地址
echo   -P, --port     RDP 服务器端口 (默认: 53389)
echo   -u, --user     用户名
echo   -p, --pass     密码
echo   -l, --listen   HTTP 监听端口 (默认: 54455)
exit /b 0

:done_args
set "EXE=%PROJECT_DIR%\gofreerdp-windows.exe"
if not exist "%EXE%" (
    echo 错误: 找不到 gofreerdp-windows.exe，请先运行 build_windows.cmd
    exit /b 1
)

REM 拼接命令
set "CMD=%EXE% --listen=%LISTEN%"
if defined HOST set "CMD=%CMD% --host=%HOST%"
if defined PORT if not "%PORT%"=="53389" set "CMD=%CMD% --port=%PORT%"
if defined USER set "CMD=%CMD% --user=%USER%"
if defined PASS set "CMD=%CMD% --pass=%PASS%"

echo 启动: %CMD%
%CMD%

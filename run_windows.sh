#!/bin/bash
# Go-FreeRDP-WebConnect Windows 运行脚本

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
MSYS64="/c/DevDisk/DevTools/msys64"
MINGW_BIN="$MSYS64/mingw64/bin"
FREERDP_INSTALL="$PROJECT_DIR/install"

# 设置 DLL 搜索路径
export PATH="$MINGW_BIN:$FREERDP_INSTALL/bin:$PATH"

# 启用 OpenSSL legacy provider（NLA/NTLM 认证依赖 MD4/RC4，OpenSSL 3.x 默认禁用）
export OPENSSL_CONF="$PROJECT_DIR/openssl.cnf"
export OPENSSL_MODULES="$FREERDP_INSTALL/bin/ossl-modules"

# 解析参数
HOST=""
PORT="53389"
USER=""
PASS=""
LISTEN="54455"

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--host)    HOST="$2";   shift 2 ;;
        -P|--port)    PORT="$2";   shift 2 ;;
        -u|--user)    USER="$2";   shift 2 ;;
        -p|--pass)    PASS="$2";   shift 2 ;;
        -l|--listen)  LISTEN="$2"; shift 2 ;;
        --help)
            echo "用法: $0 [选项]"
            echo "  -h, --host     RDP 服务器地址"
            echo "  -P, --port     RDP 服务器端口 (默认: 53389)"
            echo "  -u, --user     用户名"
            echo "  -p, --pass     密码"
            echo "  -l, --listen   HTTP 监听端口 (默认: 54455)"
            exit 0 ;;
        *) echo "未知选项: $1"; exit 1 ;;
    esac
done

EXE="$PROJECT_DIR/gofreerdp-windows.exe"
if [ ! -f "$EXE" ]; then
    echo "错误: 找不到 gofreerdp-windows.exe，请先运行 build_windows.sh"
    exit 1
fi

CMD="$EXE --listen=$LISTEN"
[ -n "$HOST" ] && CMD="$CMD --host=$HOST"
[ -n "$PORT" ] && CMD="$CMD --port=$PORT"
[ -n "$USER" ] && CMD="$CMD --user=$USER"
[ -n "$PASS" ] && CMD="$CMD --pass=$PASS"

echo "启动: $CMD"
exec $CMD

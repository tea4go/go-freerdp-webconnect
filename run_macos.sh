#!/bin/bash

# Go-FreeRDP-WebConnect macOS 运行脚本

# 设置库路径
export DYLD_LIBRARY_PATH=/usr/local/opt/freerdp/lib:$DYLD_LIBRARY_PATH

# 项目目录
PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$PROJECT_DIR"

# 解析参数
HOST=""
PORT="53389"
USER=""
PASS=""
LISTEN="54455"

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--host)
            HOST="$2"
            shift 2
            ;;
        -P|--port)
            PORT="$2"
            shift 2
            ;;
        -u|--user)
            USER="$2"
            shift 2
            ;;
        -p|--pass)
            PASS="$2"
            shift 2
            ;;
        -l|--listen)
            LISTEN="$2"
            shift 2
            ;;
        --help)
            echo "用法: $0 [选项]"
            echo ""
            echo "选项:"
            echo "  -h, --host     RDP 服务器地址"
            echo "  -P, --port     RDP 服务器端口 (默认: 53389)"
            echo "  -u, --user     用户名"
            echo "  -p, --pass     密码"
            echo "  -l, --listen   HTTP 监听端口 (默认: 54455)"
            echo ""
            exit 0
            ;;
        *)
            echo "未知选项: $1"
            exit 1
            ;;
    esac
done

# 检查可执行文件
if [ ! -f "./go-freerdp-webconnect" ]; then
    echo "错误: 找不到可执行文件 go-freerdp-webconnect"
    echo "请先运行: go build -o go-freerdp-webconnect"
    exit 1
fi

# 构建命令
CMD="./go-freerdp-webconnect"

if [ -n "$HOST" ]; then
    CMD="$CMD --host=$HOST"
fi

if [ -n "$PORT" ]; then
    CMD="$CMD --port=$PORT"
fi

if [ -n "$USER" ]; then
    CMD="$CMD --user=$USER"
fi

if [ -n "$PASS" ]; then
    CMD="$CMD --pass=$PASS"
fi

if [ -n "$LISTEN" ]; then
    CMD="$CMD --listen=$LISTEN"
fi

echo "启动 Go-FreeRDP-WebConnect..."
echo "命令: $CMD"
echo ""

# 运行
exec $CMD

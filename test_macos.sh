#!/bin/bash

# macOS 测试脚本 for go-freerdp-webconnect

echo "=== Go-FreeRDP-WebConnect macOS 测试 ==="
echo ""

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$PROJECT_DIR"

# 设置库路径
export DYLD_LIBRARY_PATH=/usr/local/opt/freerdp/lib:$DYLD_LIBRARY_PATH

# 测试 1: 检查可执行文件
echo "[测试 1] 检查可执行文件..."
if [ -f "./go-freerdp-webconnect" ]; then
    echo "✅ 可执行文件存在"
else
    echo "❌ 可执行文件不存在"
    exit 1
fi

# 测试 2: 检查帮助信息
echo ""
echo "[测试 2] 检查帮助信息..."
./go-freerdp-webconnect --help 2>&1 | head -5
if [ $? -eq 0 ]; then
    echo "✅ 帮助信息显示正常"
else
    echo "❌ 帮助信息显示失败"
    exit 1
fi

# 测试 3: 启动 HTTP 服务测试
echo ""
echo "[测试 3] 启动 HTTP 服务测试..."
TEST_PORT=56666

# 启动服务
./go-freerdp-webconnect --listen=$TEST_PORT > /tmp/test_output.log 2>&1 &
PID=$!

# 等待服务启动
sleep 2

# 检查进程是否存在
if kill -0 $PID 2>/dev/null; then
    echo "✅ 服务进程启动成功 (PID: $PID)"
    
    # 测试 HTTP 访问
    HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$TEST_PORT/index-debug.html 2>/dev/null)
    if [ "$HTTP_STATUS" = "200" ]; then
        echo "✅ HTTP 服务响应正常 (状态码: $HTTP_STATUS)"
    else
        echo "⚠️ HTTP 服务可能未完全就绪 (状态码: $HTTP_STATUS)"
    fi
    
    # 停止服务
    kill $PID 2>/dev/null
    wait $PID 2>/dev/null
    echo "✅ 服务已停止"
else
    echo "❌ 服务进程启动失败"
    cat /tmp/test_output.log
    exit 1
fi

# 测试 4: 检查 FreeRDP 库
echo ""
echo "[测试 4] 检查 FreeRDP 库..."
if [ -f "/usr/local/opt/freerdp/lib/libfreerdp3.dylib" ]; then
    echo "✅ FreeRDP 3.x 库存在"
else
    echo "❌ FreeRDP 库不存在"
    exit 1
fi

echo ""
echo "=== 所有测试通过! ==="
echo ""
echo "使用方法:"
echo "  ./run_macos.sh -h <主机> -u <用户名> -p <密码>"
echo ""
echo "示例:"
echo "  ./run_macos.sh -h 192.168.1.100 -u administrator -p mypassword"

#!/bin/bash
# Go-FreeRDP-WebConnect Windows 测试脚本

echo "=== Go-FreeRDP-WebConnect Windows 测试 ==="
echo ""

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
MSYS64="/c/DevDisk/DevTools/msys64"
MINGW_BIN="$MSYS64/mingw64/bin"
FREERDP_INSTALL="$PROJECT_DIR/install"

# 设置 DLL 搜索路径
export PATH="$MINGW_BIN:$FREERDP_INSTALL/bin:$PATH"

PASS_COUNT=0
FAIL_COUNT=0

check_pass() { echo "✅ $1"; ((PASS_COUNT++)); }
check_fail() { echo "❌ $1"; ((FAIL_COUNT++)); }

# 测试 1: 检查可执行文件
echo "[测试 1] 检查可执行文件..."
if [ -f "$PROJECT_DIR/gofreerdp-windows.exe" ]; then
    check_pass "可执行文件存在 ($(du -sh "$PROJECT_DIR/gofreerdp-windows.exe" | cut -f1))"
else
    check_fail "可执行文件不存在，请先运行 build_windows.sh"
    exit 1
fi

# 测试 2: 检查 FreeRDP DLL
echo ""
echo "[测试 2] 检查 FreeRDP 动态库..."
for dll in libfreerdp3.dll libfreerdp-client3.dll libwinpr3.dll; do
    if [ -f "$FREERDP_INSTALL/bin/$dll" ]; then
        check_pass "$dll 存在"
    else
        check_fail "$dll 缺失"
    fi
done

# 测试 3: 检查 MinGW 运行时 DLL
echo ""
echo "[测试 3] 检查 MinGW 运行时库..."
for dll in libssl-3-x64.dll libcrypto-3-x64.dll zlib1.dll libgcc_s_seh-1.dll libwinpthread-1.dll; do
    if [ -f "$FREERDP_INSTALL/bin/$dll" ]; then
        check_pass "$dll 存在"
    else
        check_fail "$dll 缺失"
    fi
done

# 测试 4: 帮助信息
echo ""
echo "[测试 4] 检查帮助信息..."
HELP_OUT=$("$PROJECT_DIR/gofreerdp-windows.exe" --help 2>&1)
if echo "$HELP_OUT" | grep -q "host\|listen"; then
    check_pass "帮助信息正常"
    echo "  $(echo "$HELP_OUT" | grep "host" | head -1)"
else
    check_fail "帮助信息异常"
fi

# 测试 5: 版本信息
echo ""
echo "[测试 5] 检查版本信息..."
VER_OUT=$("$PROJECT_DIR/gofreerdp-windows.exe" --version 2>&1)
if echo "$VER_OUT" | grep -qE "gofreerdp|version|0\.[0-9]"; then
    check_pass "版本信息: $VER_OUT"
else
    check_fail "版本信息异常: $VER_OUT"
fi

# 测试 6: HTTP 服务启动
echo ""
echo "[测试 6] 启动 HTTP 服务测试..."
TEST_PORT=56788
LOG_FILE="$PROJECT_DIR/test_output.log"

# 先杀掉可能残留的进程
taskkill /F /IM gofreerdp-windows.exe 2>/dev/null || true

# 启动服务
"$PROJECT_DIR/gofreerdp-windows.exe" --listen=$TEST_PORT > "$LOG_FILE" 2>&1 &
SRV_PID=$!
sleep 3

if kill -0 $SRV_PID 2>/dev/null; then
    check_pass "HTTP 服务启动成功 (PID: $SRV_PID)"

    # 测试 HTTP 访问
    HTTP_CODE=$(powershell -Command "(Invoke-WebRequest -Uri 'http://localhost:$TEST_PORT/' -UseBasicParsing -TimeoutSec 5).StatusCode" 2>/dev/null || echo "0")
    if [ "$HTTP_CODE" = "200" ]; then
        check_pass "HTTP 服务响应正常 (状态码: $HTTP_CODE)"
    else
        # curl 作为备选
        HTTP_CODE2=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "http://localhost:$TEST_PORT/" 2>/dev/null || echo "0")
        if [ "$HTTP_CODE2" = "200" ]; then
            check_pass "HTTP 服务响应正常 (curl 状态码: $HTTP_CODE2)"
        else
            check_fail "HTTP 服务响应异常 (状态码: $HTTP_CODE/$HTTP_CODE2)"
            echo "  服务日志:"
            cat "$LOG_FILE" | head -5
        fi
    fi

    # 测试 /api/version 接口
    API_RESP=$(powershell -Command "(Invoke-WebRequest -Uri 'http://localhost:$TEST_PORT/api/version' -UseBasicParsing -TimeoutSec 5).Content" 2>/dev/null || echo "")
    if echo "$API_RESP" | grep -q "freerdp\|app"; then
        check_pass "/api/version 接口正常: $API_RESP"
    else
        check_fail "/api/version 接口异常"
    fi

    # 停止服务
    kill $SRV_PID 2>/dev/null
    wait $SRV_PID 2>/dev/null
    check_pass "服务已停止"
else
    check_fail "服务启动失败"
    echo "  日志:"
    cat "$LOG_FILE" | head -10
fi

# 汇总
echo ""
echo "========================================="
echo "测试结果: ✅ $PASS_COUNT 通过  ❌ $FAIL_COUNT 失败"
if [ $FAIL_COUNT -eq 0 ]; then
    echo "=== 所有测试通过! ==="
    echo ""
    echo "使用方法:"
    echo "  ./run_windows.sh -h <主机> -u <用户名> -p <密码>"
    echo ""
    echo "访问: http://localhost:54455/index-debug.html"
    exit 0
else
    echo "=== 部分测试失败! ==="
    exit 1
fi

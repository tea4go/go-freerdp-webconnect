# 测试说明

## 目录结构

```
test/
├── readme_test.md          # 本文件：测试总览
└── rdp/
    ├── rdp_test.go         # 公共测试逻辑（跨平台共用）
    ├── platform_darwin.go  # macOS 平台适配
    ├── platform_linux.go   # Linux 平台适配
    └── platform_windows.go # Windows 平台适配
```

---

## 测试用例

### TestRDPConnect — RDP 连接测试

**目的：** 验证通过 WebSocket 能成功建立 RDP 远程桌面连接。

**流程：**
1. 启动 `go-freerdp-webconnect` 服务进程（端口 54466）
2. 轮询 `/api/version` 等待 HTTP 服务就绪
3. 发起 WebSocket 握手（`/ws?dtsize=800x600`）
4. 等待后端输出 `Connected.`（最长 15 秒）
5. 检查全程无 `[ERROR]` 日志

**判定标准：**
- ✅ 后端输出 `Connected.` → PASS
- ❌ 超时未连接 / 出现 `[ERROR]` → FAIL

---

### TestRDPDisconnect — RDP 断开测试

**目的：** 验证主动断开 WebSocket 后，后端能正常处理断开流程且无报错。

**流程：**
1. 启动服务，建立 WebSocket 连接并等待 `Connected.`
2. 发送 WebSocket Close 帧（opcode=0x8，状态码 1000）模拟点击"断开连接"
3. 等待后端输出含 `Disconnecting` 的日志（最长 8 秒）
4. 检查全程无 `[ERROR]` 日志

**判定标准：**
- ✅ 后端输出 `Disconnecting ...` 且无 `[ERROR]` → PASS
- ❌ 超时未断开 / 出现 `[ERROR]` → FAIL

---

## 各平台运行方式

### macOS

```bash
# 前提：已执行 lib_build_macos.sh，FreeRDP 通过 Homebrew 安装
export DYLD_LIBRARY_PATH=/usr/local/opt/freerdp/lib:$DYLD_LIBRARY_PATH
go test ./test/rdp/ -v -timeout 60s
```

### Linux

```bash
# 前提：已执行 lib_build_linux.sh，FreeRDP 编译到 install/ 目录
export LD_LIBRARY_PATH=./install/lib:./install/lib/x86_64-linux-gnu:$LD_LIBRARY_PATH
go test ./test/rdp/ -v -timeout 60s
```

### Windows（Git Bash / MSYS2）

```bash
# 前提：已执行 build_windows.sh，FreeRDP 编译到 install/ 目录
go test ./test/rdp/ -v -timeout 60s

# 若 MSYS64 不在默认路径 C:\DevDisk\DevTools\msys64，需指定：
MSYS64_PATH=C:/tools/msys64 go test ./test/rdp/ -v -timeout 60s
```

---

## 环境变量

| 变量          | 平台    | 说明                                     | 默认值                          |
|---------------|---------|------------------------------------------|---------------------------------|
| `RDP_PASS`    | 全平台  | RDP 连接密码                             | ``                        |
| `MSYS64_PATH` | Windows | MSYS64 安装目录（用于定位 MinGW DLL）    | `C:\DevDisk\DevTools\msys64`    |

示例：
```bash
RDP_PASS=mypassword go test ./test/rdp/ -v -timeout 60s
```

---

## 平台适配说明

各平台的差异通过 Go 构建约束（`//go:build`）隔离到独立文件：

| 文件                   | 构建约束      | 适配内容                                              |
|------------------------|---------------|-------------------------------------------------------|
| `platform_darwin.go`   | `darwin`      | 可执行文件名（无扩展名），`DYLD_LIBRARY_PATH` 注入    |
| `platform_linux.go`    | `linux`       | 可执行文件名（无扩展名），`LD_LIBRARY_PATH` 注入      |
| `platform_windows.go`  | `windows`     | 可执行文件名（`.exe`），`PATH` 注入（install/bin + MinGW）|

---

## 测试端口

测试使用端口 **54466**，与生产端口 **54455** 隔离，避免冲突。

若端口被占用，可修改 `rdp_test.go` 中的常量 `testListenPort`。

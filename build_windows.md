# Windows 11 编译指南

本文档描述在 Windows 11 上编译 go-freerdp-webconnect 的完整步骤。

## 环境要求

| 组件 | 版本 | 说明 |
|------|------|------|
| Windows 11 | 22H2+ | 含 IoT LTSC 2024 |
| MSYS2 + MinGW-w64 | GCC 15.x | C 编译器 + CMake + make |
| Go | 1.24.1 | CGO 构建，需 GOTOOLCHAIN=go1.24.1 |
| Git | 任意 | 用于拉取源码 |

---

## 第一步：安装 MSYS2

### 推荐方式（清华镜像 sfx 包）

1. 从清华镜像下载 MSYS2 sfx 安装包：
   ```
   https://mirrors.tuna.tsinghua.edu.cn/msys2/distrib/x86_64/
   ```
   下载最新的 `msys2-x86_64-*.exe` 并安装到 `C:\DevDisk\DevTools\msys64`。

2. 打开 **MSYS2 MinGW64** 终端，安装所需工具链：
   ```bash
   pacman -S mingw-w64-x86_64-gcc \
             mingw-w64-x86_64-cmake \
             mingw-w64-x86_64-make \
             git
   ```

3. 验证安装：
   ```bash
   gcc --version    # 应显示 GCC 15.x
   cmake --version  # 应显示 CMake 4.x
   ```

> **注意**：后续所有构建操作均在 **MSYS2 MinGW64** 终端中执行。

---

## 第二步：安装 Go 工具链

1. 从官网下载 Go 1.24.1 Windows 安装包并安装（默认路径 `C:\Go`）。
2. 确保 `go.exe` 在 PATH 中可用：
   ```bash
   go version   # 应显示 go1.24.1
   ```

---

## 第三步：获取项目源码

```bash
git clone https://github.com/yourname/go-freerdp-webconnect.git
cd go-freerdp-webconnect

# 初始化 FreeRDP 子模块
git submodule update --init --recursive
```

FreeRDP 源码位于 `src/FreeRDP/`。

---

## 第四步：一键构建

在项目根目录执行构建脚本（首次运行会自动编译 FreeRDP，约需 5~15 分钟）：

```bash
./build_windows.sh
```

脚本完成后输出：
```
=== 构建成功! ===
```

### 构建产物

| 文件 | 说明 |
|------|------|
| `gofreerdp-windows.exe` | 主程序（约 20 MB，含静态 Go 运行时） |
| `install/bin/libfreerdp3.dll` | FreeRDP 核心库 |
| `install/bin/libfreerdp-client3.dll` | FreeRDP 客户端库 |
| `install/bin/libwinpr3.dll` | WinPR 运行时库 |

> **注意**：运行时三个 DLL 必须与 `.exe` 在同一目录，或其所在目录已加入 PATH。

---

## 第五步：验证构建结果

```bash
./test_windows.sh
```

全部通过时输出：
```
测试结果: ✅ 10 通过  ❌ 0 失败
=== 所有测试通过! ===
```

测试项目包括：
- 可执行文件存在性及大小
- 三个 FreeRDP DLL 存在性
- `--help` 输出正常
- `--version` 输出正常
- HTTP 服务在 56788 端口启动成功
- `GET /` 返回 HTTP 200
- `GET /api/version` 返回正确 JSON

---

## 第六步：运行服务

```bash
./run_windows.sh -h <RDP服务器地址> -u <用户名> -p <密码>
```

完整参数：

```
-h, --host     RDP 服务器地址（必填）
-P, --port     RDP 服务器端口（默认: 53389）
-u, --user     用户名（必填）
-p, --pass     密码（必填）
-l, --listen   HTTP 监听端口（默认: 54455）
```

服务启动后，浏览器访问：
```
http://localhost:54455/index-debug.html
```

---

## 构建脚本详解

### build_windows.sh 工作流程

```
[1/2] 编译 FreeRDP（仅首次执行）
  ├── cmake 配置（MinGW Makefiles）
  ├── mingw32-make -j<CPU核数>
  └── mingw32-make install → install/

[2/2] 编译 Go 项目
  ├── CGO_ENABLED=1
  ├── CC=<MSYS2>/mingw64/bin/gcc.exe
  ├── GOTOOLCHAIN=go1.24.1
  └── go build -o gofreerdp-windows.exe .
```

### CGO 编译参数（rdp.go）

```c
#cgo windows CFLAGS: -I${SRCDIR}/install/include/freerdp3 \
                     -I${SRCDIR}/install/include/winpr3 \
                     -D__STDC_NO_THREADS__=1
#cgo windows LDFLAGS: -L${SRCDIR}/install/bin \
                      -lfreerdp3 -lfreerdp-client3 -lwinpr3
```

### FreeRDP CMake 关键参数说明

| 参数 | 原因 |
|------|------|
| `-D__STDC_NO_THREADS__=1` | MinGW 缺少 C11 `threads.h`，绕过线程头文件检测 |
| `-Wno-incompatible-pointer-types` | 忽略 SSPI 回调函数指针类型不匹配警告 |
| `-DWITH_SSE2=OFF -DWITH_SIMD=OFF` | 避免 AVX intrinsics 内联汇编编译错误 |
| `-DUSE_UNWIND=OFF` | 避免依赖 Linux-only 的 `dlfcn.h` |
| `-DWITH_OPENSSL=OFF` | 避免 `libfreerdp3.dll` 静态内嵌 OpenSSL（与 `libssl-3-x64.dll` 双重链接导致崩溃） |
| `TEMP=/c/Temp TMP=/c/Temp` | 修复 GCC 在某些 Windows 路径下的临时文件权限问题 |
| `-G "MinGW Makefiles"` | 使用 MinGW 的 `mingw32-make`，而非 Visual Studio |
| `-DWITH_CLIENT=OFF -DWITH_SERVER=OFF` | 不编译 FreeRDP 命令行客户端和服务端，只编译库 |

---

## 常见问题

### Q: `gcc.exe` 找不到

确保 MSYS2 已安装 `mingw-w64-x86_64-gcc`，且脚本中路径与实际安装路径一致：
```bash
# 默认路径
MSYS64="/c/DevDisk/DevTools/msys64"
```
如果安装到其他路径，修改 `build_windows.sh` 第 15 行的 `MSYS64` 变量。

### Q: FreeRDP 编译出现 `threads.h: No such file`

已通过 `-D__STDC_NO_THREADS__=1` 解决。如遇此错误，确认 cmake 命令中包含该参数。

### Q: `go build` 失败，提示找不到 FreeRDP 库

确认 `install/bin/` 目录下存在三个 DLL 文件。如不存在，删除 `install/` 目录后重新运行 `build_windows.sh`。

### Q: 运行时提示 DLL 缺失

`run_windows.sh` 脚本已自动将 `install/bin/` 和 MinGW 的 `bin/` 加入 PATH。若直接运行 `.exe`，需手动将上述路径加入系统或用户 PATH，或将 DLL 复制到 `.exe` 同目录。

### Q: Go 模块下载失败

设置代理后重试（`GOPROXY=off` 时需确保 `go.sum` 已存在）：
```bash
export GOPROXY=https://goproxy.io
go mod download
```

---

## 故障排查记录

本节记录实际调试过程中遇到的深层问题及根因分析，供后续维护参考。

---

### 问题一：运行时 `Exception 0xc0000005` ACCESS_VIOLATION 崩溃

**现象**

执行 `run_windows.sh --host=<IP> --port=3389 ...` 后，通过浏览器发起 WebSocket 连接，程序立即崩溃并输出：

```
Exception 0xc0000005 0x1 0x24 0x7ff8fd3f982d
PC=0x7ff8fd3f982d
signal arrived during external code execution

main._Cfunc_safeFreerdpConnect(...)
    rdp.go:471
```

崩溃地址 `0x7ff8fd3f982d` 经 `dbghelp.dll` 符号解析定位为 `ntdll.dll!RtlInitializeResource+0x94d`，对应指令：

```asm
cmp rax, -1
je  skip
inc DWORD PTR [rax+0x24]   ; rax=0x0 → ACCESS_VIOLATION
```

**根因分析**

调用链为：

```
freerdp_connect()
  └─ freerdp_connect_begin()
       └─ freerdp_add_signal_cleanup_handler()
            └─ fsig_lock()
                 └─ EnterCriticalSection(&signal_lock)  ← 崩溃点
```

`signal_lock` 是 `libfreerdp/utils/signal_win32.c` 中的 `static CRITICAL_SECTION`，**必须通过 `freerdp_handle_signals()` 显式初始化**。在 CGO 嵌入式调用场景下，`freerdp_handle_signals()` 从未被调用，`signal_lock` 保持零值（未初始化状态）。对未初始化的 `CRITICAL_SECTION` 调用 `EnterCriticalSection`，Windows 内部的 `RtlInitializeResource` 会尝试访问 `NULL+0x24`，触发 ACCESS_VIOLATION。

注：Go runtime 内置的 VEH（向量异常处理器）优先级高于用户注册的 VEH，无法通过 `AddVectoredExceptionHandler` 拦截该异常。

**解决方案**

在 [rdp.go](rdp.go) 的 CGO 代码段添加初始化函数，并在每次调用 `freerdp_new()` 之前执行：

```c
// rdp.go CGO preamble 中
#include <freerdp/utils/signal.h>

static void initFreeRDPSignalLock(void) {
    freerdp_handle_signals();  // 初始化 signal_lock CRITICAL_SECTION
}
```

```go
// rdpconnect() 函数开头
C.initFreeRDPSignalLock()   // 必须在 freerdp_new() 之前调用
instance := C.freerdp_new()
```

`freerdp_handle_signals()` 会调用 `InitializeCriticalSection(&signal_lock)` 并注册信号处理器。该调用不会干扰 Go 的信号处理机制（Go 通过独立的 SEH/VEH 机制管理信号）。

---

### 问题二：`libfreerdp3.dll` 内嵌 OpenSSL 导致双重链接

**现象**

未添加 `-DWITH_OPENSSL=OFF` 时，用 `nm` 检查编译产物：

```bash
nm install/bin/libfreerdp3.dll | grep "SSL_CTX_new"
# 输出：
000000030074cad8 T SSL_CTX_new       ← 静态编译进 DLL 的实现
00000003008452e8 I __imp_SSL_CTX_new ← 同时动态导入 libssl-3-x64.dll
```

同一个 `SSL_CTX_new` 函数存在两份实现：`libfreerdp3.dll` 自带一份静态实现，同时又从 `libssl-3-x64.dll` 动态导入。两份 OpenSSL 实例共存，内部状态相互污染，导致 NULL 指针崩溃。

**解决方案**

在 [build_windows.sh](build_windows.sh) 的 cmake 命令中添加：

```bash
-DWITH_OPENSSL=OFF
```

该参数使 `libfreerdp3.dll` 不直接依赖 OpenSSL。`libwinpr3.dll` 仍然依赖 OpenSSL（通过跳转桩调用 `libssl-3-x64.dll`），但此时只有一份 OpenSSL 实例，双重链接问题消除。

重新编译后验证：

```bash
nm install/bin/libfreerdp3.dll | grep "SSL_CTX"
# 无输出 ✅
```

> **注意**：`libwinpr3.dll` 中的 `T SSL_CTX_new` 是跳转桩（thunk），反汇编可见 `jmp *__imp_SSL_CTX_new`，并非静态实现，属于正常现象。

---

### 问题三：运行时需要额外的 MinGW DLL

**现象**

直接运行 `gofreerdp-windows.exe` 时提示缺少 DLL，或程序无法启动。

**原因**

`libwinpr3.dll` 依赖以下 MinGW 运行时库，这些文件不在 `install/bin/` 中：

```
libssl-3-x64.dll
libcrypto-3-x64.dll
zlib1.dll
libgcc_s_seh-1.dll
```

**解决方案**

`build_windows.cmd` 和 `wails_build_windows.cmd` 已自动从 MSYS2 的 MinGW 目录复制以下 DLL 到 `install/bin/`。如需手动修复，可执行：

```bash
MINGW_BIN="/c/DevDisk/DevTools/msys64/mingw64/bin"
cp $MINGW_BIN/libssl-3-x64.dll   install/bin/
cp $MINGW_BIN/libcrypto-3-x64.dll install/bin/
cp $MINGW_BIN/zlib1.dll           install/bin/
cp $MINGW_BIN/libgcc_s_seh-1.dll  install/bin/
```

`run_windows.sh` 脚本已通过 `PATH` 设置自动覆盖此需求，手动运行时需确保上述文件可被系统找到。

---

### 问题四：FreeRDP 安全协议协商（服务器拒绝经典 RDP）

**现象**

将安全协议设置为仅允许经典 RDP（`NlaSecurity=FALSE, TlsSecurity=FALSE`）时，连接失败：

```
[ERROR][transport_read_layer]: ERRCONNECT_CONNECT_TRANSPORT_FAILED [0x0002000D]
```

日志显示协商阶段服务器断开连接。

**原因**

Windows Server 2016/2019/2022 默认要求 NLA（网络层认证）。服务器在协商阶段拒绝了仅支持经典 RDP 安全层的客户端。

**解决方案**

启用 NLA/TLS/RDP 三路自动协商（FreeRDP 按优先级自动选择）：

```go
C.freerdp_settings_set_bool(settings, C.FreeRDP_NlaSecurity, C.TRUE)
C.freerdp_settings_set_bool(settings, C.FreeRDP_TlsSecurity, C.TRUE)
C.freerdp_settings_set_bool(settings, C.FreeRDP_RdpSecurity, C.TRUE)
```

服务器会选择 `HYBRID`（NLA）协议，认证流程：TLS 握手 → NTLM 凭据交换 → RDP 会话建立。

## 目录结构

```
go-freerdp-webconnect/
├── src/FreeRDP/          # FreeRDP 源码（git submodule）
├── build/freerdp-windows/ # CMake 构建中间文件（gitignore）
├── install/              # FreeRDP 编译安装目录
│   ├── bin/              # DLL 文件
│   ├── include/          # 头文件
│   └── lib/              # 静态库（可选）
├── rdp.go                # CGO 核心代码
├── build_windows.sh      # 构建脚本
├── run_windows.sh        # 运行脚本
├── test_windows.sh       # 测试脚本
└── gofreerdp-windows.exe # 构建产物
```

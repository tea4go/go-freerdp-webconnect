# rdp.go 功能设计文档

> 文件路径：`rdp.go`
> 语言：Go（含 CGo 内嵌 C 代码）
> 依赖库：FreeRDP 3.x（libfreerdp3、libfreerdp-client3、libwinpr3）

---

## 目录

1. [概述](#1-概述)
2. [架构总览](#2-架构总览)
3. [CGo 桥接层（C 代码）](#3-cgo-桥接层c-代码)
4. [Go 数据结构](#4-go-数据结构)
5. [Context 注册表机制](#5-context-注册表机制)
6. [RDP 连接主流程](#6-rdp-连接主流程)
7. [绘图回调函数](#7-绘图回调函数)
8. [WebSocket 二进制协议（服务端→客户端）](#8-websocket-二进制协议服务端客户端)
9. [输入事件协议（客户端→服务端）](#9-输入事件协议客户端服务端)
10. [二次开发指南](#10-二次开发指南)
11. [已知限制](#11-已知限制)

---

## 1. 概述

`rdp.go` 是 go-freerdp-webconnect 项目的 **RDP 后端核心模块**，通过 CGo 调用 FreeRDP 3.x 库，将 RDP 服务器下发的图形指令转换为 WebSocket 二进制消息，推送给浏览器前端渲染。同时接收前端的鼠标、键盘、分辨率调整事件，通过 FreeRDP 转发给远程桌面服务器。

**核心职责：**

- 通过 CGo 调用 FreeRDP C 库，管理 RDP 连接生命周期
- 将 RDP 服务器的图形更新（位图、矩形填充、屏幕复制等）编码为二进制帧，发送到 WebSocket
- 将来自前端的输入事件（鼠标/键盘/分辨率）注入 RDP 连接
- 支持通过 RDPEDISP 虚拟通道动态调整远程桌面分辨率

**关键设计约束：**

- CGo 规范要求：C 内存中不能存储 Go 指针，通过全局 `map[uintptr]*rdpContextData` 绑定 C 上下文与 Go 数据
- FreeRDP 的传输层使用线程本地状态，所有 FreeRDP API 调用必须在同一 OS 线程执行（`runtime.LockOSThread`）

---

## 2. 架构总览

### 2.1 模块层次

```
┌─────────────────────────────────────────────────────┐
│                    main.go                          │
│  HTTP / WebSocket 网关                               │
│  initSocket() → rdpconnect() / processSendQ()       │
└──────────────────────┬──────────────────────────────┘
                       │  Go channel (sendq / recvq / inputq)
┌──────────────────────▼──────────────────────────────┐
│                    rdp.go                           │
│  ┌──────────────────────────────────────────────┐   │
│  │  Go 层                                        │   │
│  │  rdpconnect()      主事件循环                  │   │
│  │  primaryPatBlt()   图案填充回调                │   │
│  │  primaryScrBlt()   屏幕复制回调                │   │
│  │  primaryOpaqueRect() 矩形填充回调              │   │
│  │  primaryMultiOpaqueRect() 多矩形填充回调       │   │
│  │  beginPaint()      帧开始回调                  │   │
│  │  endPaint()        帧结束回调                  │   │
│  │  setBounds()       裁剪区回调                  │   │
│  │  bitmapUpdate()    位图更新回调                │   │
│  │  preConnect()      连接前配置回调              │   │
│  │  postConnect()     连接成功回调                │   │
│  └──────────────┬───────────────────────────────┘   │
│                 │  CGo 调用                          │
│  ┌──────────────▼───────────────────────────────┐   │
│  │  C 桥接层（CGo 内嵌 C）                        │   │
│  │  cbPreConnect / cbPostConnect                 │   │
│  │  cbPrimaryPatBlt / cbPrimaryScrBlt 等         │   │
│  │  sendMouseInput / sendKupdownInput 等         │   │
│  │  checkEventHandles                            │   │
│  └──────────────┬───────────────────────────────┘   │
└─────────────────┼───────────────────────────────────┘
                  │  FreeRDP C API
┌─────────────────▼───────────────────────────────────┐
│  FreeRDP 3.x 库（libfreerdp3）                       │
│  RDP 协议 / TLS / 图形解码 / 虚拟通道               │
└─────────────────────────────────────────────────────┘
```

### 2.2 数据流总览

```
RDP 服务器
    │  RDP 协议（TCP）
    ▼
FreeRDP C 库
    │  图形回调（cbPrimaryPatBlt 等）
    ▼
CGo 导出函数（//export）
    │  lookupCtx() 查找 Go 数据
    ▼
编码为二进制消息 → sendq channel
    │
    ▼
processSendQ() → WebSocket.Send() → 浏览器 Canvas 渲染

浏览器用户输入
    │  WebSocket 二进制消息（16字节）
    ▼
initSocket() → inputq channel
    │
    ▼
rdpconnect() 事件循环 → sendMouseInput / sendKupdownInput 等 → FreeRDP → RDP 服务器
```

---

## 3. CGo 桥接层（C 代码）

本节描述 `rdp.go` 文件顶部的内嵌 C 代码，这些函数不可直接从 Go 调用，而是作为 FreeRDP 的回调适配层使用。

### 3.1 全局状态

```c
static DispClientContext* g_dispCtx = NULL;
```

全局 DISP 虚拟通道上下文指针。当 RDPEDISP 通道连接后，`onChannelConnected` 回调将其赋值；断开后清空。`sendResizeInput` 通过此指针发送动态分辨率调整请求。

> ⚠️ **注意（二次开发）**：此全局变量为单会话设计。如需支持多并发 RDP 连接，需将其改为 per-instance 存储（例如存入扩展 rdpContext）。

### 3.2 通道事件回调

| C 函数 | 说明 |
|---|---|
| `onChannelConnected` | 捕获 DISP 虚拟通道上下文，赋值 `g_dispCtx` |
| `onChannelDisconnected` | 清除 `g_dispCtx` |
| `registerChannelEvents` | 向 FreeRDP pubSub 注册上述两个回调 |

### 3.3 输入发送函数

| C 函数 | 参数 | 说明 |
|---|---|---|
| `sendMouseInput` | `instance, flags, x, y` | 发送鼠标事件（移动/点击/滚轮），flags 为 RDP PTR_FLAGS |
| `sendKupdownInput` | `instance, down, keycode` | 发送修饰键按下/抬起，内部经 `jsKeyCodeToScancode` 转换 |
| `sendKpressInput` | `instance, charcode` | 发送 Unicode 字符（按下后立即释放） |
| `sendResizeInput` | `instance, width, height` | 通过 RDPEDISP 通道发送分辨率调整，需 g_dispCtx 就绪 |

#### `jsKeyCodeToScancode` 映射表

| JS keyCode | 按键 | RDP 扫描码 |
|---|---|---|
| 8 | Backspace | 0x0E |
| 16 | Shift | 0x2A |
| 17 | Ctrl | 0x1D |
| 18 | Alt | 0x38 |
| 20 | CapsLock | 0x3A |
| 144 | NumLock | 0x45 |
| 145 | ScrollLock | 0x46 |

> 仅以上修饰键走扫描码路径，普通字符走 Unicode 路径（`sendKpressInput`）。

### 3.4 辅助工具函数

| C 函数 | 说明 |
|---|---|
| `convertColor(color, srcBpp, dstBpp)` | 在 16bpp（RGB565）和 32bpp（BGRX32）之间转换像素颜色 |
| `flipImageData(data, width, height, bpp)` | 原地垂直翻转位图（RDP bottom-up → Canvas top-down） |
| `getSettings(instance)` | 获取 freerdp 实例的配置项指针 |

### 3.5 预连接回调（cbPreConnect）

在 FreeRDP 建立连接前调用，注册所有绘图/指针/位图回调，并加载 DISP 动态虚拟通道：

```
注册绘图回调: PatBlt / ScrBlt / OpaqueRect / MultiOpaqueRect
注册帧边界回调: BeginPaint / EndPaint / SetBounds / BitmapUpdate
注册指针回调: PointerNew/Cached/System/Position/Color/Large（均为空实现）
注册通道事件: registerChannelEvents()
添加 DISP 虚拟通道: freerdp_client_add_dynamic_channel()
调用 Go 层: preConnect()
```

### 3.6 连接成功回调（cbPostConnect）

连接建立后调用，初始化 GDI 并重新注册绘图回调：

```
gdi_init(instance, PIXEL_FORMAT_XRGB32)   ← 必须调用，否则指针缓存为 NULL 会崩溃
freerdp_client_load_channels()             ← 加载 DISP 通道插件
重新注册所有绘图/指针回调                    ← gdi_init 会覆盖之前的注册
postConnect()                              ← 通知 Go 层连接成功
```

> ⚠️ **重要**：`gdi_init` 会覆盖 `cbPreConnect` 中注册的回调，因此 `cbPostConnect` 中必须重新注册一遍。这是二次开发时的常见陷阱。

### 3.7 事件循环驱动（checkEventHandles）

```c
static BOOL checkEventHandles(freerdp* instance);
```

最多等待 100ms，驱动 FreeRDP 内部事件（收发 RDP 数据包）。在主事件循环的 `default` 分支中反复调用。

---

## 4. Go 数据结构

### 4.1 操作码常量

```go
const (
    WSOP_SC_BEGINPAINT       uint32 = 0  // 帧开始
    WSOP_SC_ENDPAINT         uint32 = 1  // 帧结束
    WSOP_SC_BITMAP           uint32 = 2  // 位图更新
    WSOP_SC_OPAQUERECT       uint32 = 3  // 矩形填充
    WSOP_SC_SETBOUNDS        uint32 = 4  // 裁剪边界
    WSOP_SC_PATBLT           uint32 = 5  // 图案填充
    WSOP_SC_MULTI_OPAQUERECT uint32 = 6  // 多矩形填充
    WSOP_SC_SCRBLT           uint32 = 7  // 屏幕复制
    WSOP_SC_PTR_NEW          uint32 = 8  // 新建光标
    WSOP_SC_PTR_FREE         uint32 = 9  // 释放光标
    WSOP_SC_PTR_SET          uint32 = 10 // 设置光标
    WSOP_SC_PTR_SETNULL      uint32 = 11 // 隐藏光标
    WSOP_SC_PTR_SETDEFAULT   uint32 = 12 // 默认光标
)
```

> `PTR_NEW~PTR_SETDEFAULT` 当前在 Go 层未实现（指针回调均返回 TRUE 但不发送消息），留作扩展。

### 4.2 消息结构体

#### bitmapUpdateMeta（op=2 位图更新消息头）

```go
type bitmapUpdateMeta struct {
    op  uint32 // WSOP_SC_BITMAP
    x   uint32 // 目标左上角 X
    y   uint32 // 目标左上角 Y
    w   uint32 // 源位图宽度
    h   uint32 // 源位图高度
    dw  uint32 // 目标区域宽度
    dh  uint32 // 目标区域高度
    bpp uint32 // 每像素位数（16）
    cf  uint32 // 是否压缩（0=原始，1=压缩）
    sz  uint32 // 位图数据字节长度
}
```

**二进制布局（小端，共 40 字节 + 位图数据）：**

```
偏移  大小  字段
0     4     op
4     4     x
8     4     y
12    4     w（源宽）
16    4     h（源高）
20    4     dw（目标宽）
24    4     dh（目标高）
28    4     bpp
32    4     cf（压缩标志）
36    4     sz（数据长度）
40    sz    像素数据
```

#### primaryPatBltMeta（op=5 图案填充）

```
偏移  大小  字段
0     4     op = 5
4     4     x（int32）
8     4     y（int32）
12    4     w（int32）
16    4     h（int32）
20    4     fg（uint32，BGRX32 前景色）
24    4     rop（uint32，GDI 光栅操作码）
```

#### primaryScrBltMeta（op=7 屏幕复制）

```
偏移  大小  字段
0     4     op = 7
4     4     rop（uint32）
8     4     x（int32，目标左上角）
12    4     y（int32）
16    4     w（int32）
20    4     h（int32）
24    4     sx（int32，源区域左上角）
28    4     sy（int32）
```

### 4.3 inputEvent（客户端→服务端输入事件）

```go
type inputEvent struct {
    op uint32 // 操作类型
    a  uint32 // 参数 a
    b  uint32 // 参数 b
    c  uint32 // 参数 c（可选）
}
```

| op | 含义 | a | b | c |
|---|---|---|---|---|
| 0 | 鼠标事件 | PTR_FLAGS | X 坐标 | Y 坐标 |
| 1 | 修饰键按下/抬起 | 1=按下，0=抬起 | JS keyCode | - |
| 2 | Unicode 字符输入 | 修饰键（未用） | Unicode 码点 | - |
| 3 | 分辨率调整 | 新宽度 | 新高度 | - |

### 4.4 rdpConnectionSettings（连接参数）

```go
type rdpConnectionSettings struct {
    hostname *string // RDP 主机名或 IP
    username *string // 登录用户名
    password *string // 登录密码
    width    int     // 初始桌面宽度
    height   int     // 初始桌面高度
    port     int     // RDP 端口（默认 3389）
}
```

### 4.5 rdpContextData（Go 侧会话数据）

```go
type rdpContextData struct {
    sendq    chan []byte             // 图像数据 → WebSocket 发送队列（缓冲 100）
    recvq    chan []byte             // 断开控制信号队列（缓冲 5）
    settings *rdpConnectionSettings // 连接参数
}
```

---

## 5. Context 注册表机制

### 5.1 设计背景

CGo 规范禁止在 C 管理的内存中存储 Go 指针。FreeRDP 的 `rdpContext` 由 C 分配，无法直接嵌入 Go 对象。

**解决方案**：使用全局 `map[uintptr]*rdpContextData`，以 C `rdpContext` 指针的数值作为 key，关联 Go 侧数据。

### 5.2 API

```go
// 注册：RDP 连接建立时调用
func registerCtx(ctx *C.rdpContext, d *rdpContextData)

// 注销：RDP 连接断开时 defer 调用
func unregisterCtx(ctx *C.rdpContext)

// 查找：在所有 export 回调中调用，获取 Go 侧数据
func lookupCtx(ctx *C.rdpContext) *rdpContextData
```

所有操作均通过 `sync.Mutex`（`contextMu`）保护并发安全。

### 5.3 生命周期

```
rdpconnect() 启动
    ↓
registerCtx(instance.context, data)    ← 注册
    ↓
FreeRDP 运行（回调中 lookupCtx 查找）
    ↓
defer unregisterCtx(instance.context)  ← 函数退出时自动注销
```

---

## 6. RDP 连接主流程

### 6.1 函数签名

```go
func rdpconnect(
    sendq    chan []byte,
    recvq    chan []byte,
    inputq   chan inputEvent,
    settings *rdpConnectionSettings,
)
```

由 `main.go` 的 `initSocket()` 通过 `go rdpconnect(...)` 启动，每个 WebSocket 会话对应一个独立 goroutine。

### 6.2 流程图

```
runtime.LockOSThread()           ← 锁定 OS 线程（FreeRDP 要求）
freerdp_new()                    ← 创建 FreeRDP 实例
bindCallbacks()                  ← 绑定 PreConnect/PostConnect
freerdp_context_new()            ← 初始化 rdpContext
registerCtx()                    ← 注册 Go 数据

freerdp_connect()
    ├── 失败 → freerdp_free() → return
    └── 成功 → 进入主事件循环

主事件循环（for mainEventLoop）：
    ├── <-recvq        → WebSocket 断开，退出
    ├── <-inputq ev    → 分发输入事件（鼠标/键盘/分辨率）
    └── default        → checkEventHandles()（驱动 RDP 收发）
                         freerdp_error_info() 检查错误码
                         freerdp_shall_disconnect() 检查断开信号

freerdp_free()                   ← 清理资源
```

### 6.3 RDP 错误码处理

| 错误码 | 含义 | 处理 |
|---|---|---|
| 1, 2, 7, 9 | 手动断开等 | 退出事件循环 |
| 5 | 另一用户连接了同一会话 | 仅记录，继续运行 |
| 0 | 无错误 | 继续 |

### 6.4 preConnect 连接参数配置

`preConnect` 是在 FreeRDP 调用 `cbPreConnect` 时触发的 Go 导出函数，负责将连接参数写入 FreeRDP settings：

| 配置项 | 值 | 说明 |
|---|---|---|
| ServerHostname | settings.hostname | 目标主机 |
| Username | settings.username | 用户名 |
| Password | settings.password | 密码 |
| DesktopWidth/Height | settings.width/height | 初始分辨率 |
| ServerPort | settings.port | RDP 端口 |
| IgnoreCertificate | TRUE | 忽略证书验证 |
| ColorDepth | 16 | RGB565，16bpp |
| NlaSecurity | FALSE | 禁用 NLA |
| TlsSecurity | FALSE | 禁用 TLS |
| RdpSecurity | TRUE | 使用经典 RDP 安全层 |
| PerformanceFlags | PERF_DISABLE_* | 禁用壁纸/主题/动画 |
| RemoteFxCodec | FALSE | 禁用 RemoteFX |
| FastPathOutput | TRUE | 启用快速路径输出 |
| SupportDisplayControl (5185) | TRUE | 支持 RDPEDISP |
| DynamicResolutionUpdate (1558) | TRUE | 动态分辨率更新 |

---

## 7. 绘图回调函数

所有回调均通过 `//export` 声明为 CGo 导出函数，由 C 层适配器（`cbPrimary*` 等）调用。

### 7.1 回调一览

| Go 函数 | 触发时机 | 发送消息 |
|---|---|---|
| `beginPaint` | 帧绘制开始 | `WSOP_SC_BEGINPAINT`（4字节） |
| `endPaint` | 帧绘制结束 | `WSOP_SC_ENDPAINT`（4字节） |
| `setBounds` | 设置裁剪边界 | `WSOP_SC_SETBOUNDS + rdpBounds`（20字节） |
| `bitmapUpdate` | 位图矩形批次更新 | 每个矩形一条 `WSOP_SC_BITMAP` 消息 |
| `primaryPatBlt` | 图案填充（PatBlt） | `WSOP_SC_PATBLT`（28字节，仅实心画刷） |
| `primaryScrBlt` | 屏幕区域复制 | `WSOP_SC_SCRBLT`（32字节） |
| `primaryOpaqueRect` | 单矩形填充 | `WSOP_SC_OPAQUERECT`（24字节） |
| `primaryMultiOpaqueRect` | 多矩形填充 | `WSOP_SC_MULTI_OPAQUERECT`（变长） |
| `preConnect` | 连接前配置 | 无（写入 FreeRDP settings） |
| `postConnect` | 连接成功 | 无（打印日志） |

### 7.2 bitmapUpdate 详解

```go
func bitmapUpdate(rawContext *C.rdpContext, bitmap *C.BITMAP_UPDATE) C.BOOL
```

遍历 `bitmap.number` 个矩形，对每个矩形：

1. 调用 `C.nextBitmapRectangle(bitmap, i)` 取得 `BITMAP_DATA`
2. 若未压缩（`compressed == 0`），调用 `C.flipImageData` 垂直翻转
3. 构造 `bitmapUpdateMeta` 头 + 原始像素数据，写入 `sendq`

**字段映射：**

```
bmd.destLeft          → meta.x
bmd.destTop           → meta.y
bmd.width             → meta.w
bmd.height            → meta.h
bmd.destRight-Left+1  → meta.dw（目标区域宽）
bmd.destBottom-Top+1  → meta.dh（目标区域高）
bmd.bitsPerPixel      → meta.bpp
bmd.compressed        → meta.cf
bmd.bitmapLength      → meta.sz
```

### 7.3 primaryOpaqueRect 内部结构体

`primaryOpaqueRect` 内部定义了匿名局部结构体：

```go
type opaqueRectOrder struct {
    nLeftRect int32
    nTopRect  int32
    nWidth    int32
    nHeight   int32
    color     uint32  // 已经过 convertColor 转为 BGRX32
}
```

发送格式：`uint32(WSOP_SC_OPAQUERECT) + opaqueRectOrder`（共 24 字节）

### 7.4 primaryMultiOpaqueRect 发送格式

```
偏移  大小        字段
0     4           op = WSOP_SC_MULTI_OPAQUERECT
4     4           color（int32，BGRX32）
8     4           numRectangles（int32，矩形数量）
12    16×N        DELTA_RECT[N]（C 结构体，直接写入）
```

> `DELTA_RECT` 为 C 结构体，直接通过 `binary.Write(buf, binary.LittleEndian, r)` 写入，依赖平台字节序和 C 内存布局。

---

## 8. WebSocket 二进制协议（服务端→客户端）

所有消息均使用 **小端字节序**（`binary.LittleEndian`）编码，作为 WebSocket Binary Frame 发送。

### 8.1 消息格式总表

| op | 名称 | 消息体（字节数） | 说明 |
|---|---|---|---|
| 0 | BeginPaint | 4 | 仅 op 字段 |
| 1 | EndPaint | 4 | 仅 op 字段 |
| 2 | Bitmap | 40 + dataLen | bitmapUpdateMeta + 像素数据 |
| 3 | OpaqueRect | 24 | op + opaqueRectOrder |
| 4 | SetBounds | 20 | op + rdpBounds（4×int32） |
| 5 | PatBlt | 28 | primaryPatBltMeta |
| 6 | MultiOpaqueRect | 12 + 16×N | op + color + nrects + DELTA_RECT[] |
| 7 | ScrBlt | 32 | primaryScrBltMeta |
| 8~12 | PTR_* | 未实现 | 保留扩展 |

### 8.2 SetBounds 消息体（op=4）

`rdpBounds` C 结构体内存布局（共 16 字节）：

```
left   int32
top    int32
right  int32
bottom int32
```

前端需将 right/bottom 转换为 width/height：`w = right - left`，`h = bottom - top`。

---

## 9. 输入事件协议（客户端→服务端）

来自 main.go 的 `initSocket()`，每条消息为 **16 字节**，小端 uint32×4。

### 9.1 格式定义

```
字节 0-3:   op（操作类型）
字节 4-7:   a（参数 a）
字节 8-11:  b（参数 b）
字节 12-15: c（参数 c，可选，不足则为 0）
```

### 9.2 鼠标事件（op=0）

| 字段 | 含义 |
|---|---|
| a | RDP PTR_FLAGS（见下表） |
| b | 鼠标 X 坐标 |
| c | 鼠标 Y 坐标 |

**PTR_FLAGS 常用值：**

| 标志 | 十六进制 | 含义 |
|---|---|---|
| PTR_FLAGS_MOVE | 0x0800 | 移动 |
| PTR_FLAGS_BUTTON1 | 0x1000 | 左键 |
| PTR_FLAGS_BUTTON2 | 0x2000 | 右键 |
| PTR_FLAGS_BUTTON3 | 0x4000 | 中键 |
| PTR_FLAGS_DOWN | 0x8000 | 按下（与按键 OR） |
| PTR_FLAGS_WHEEL | 0x0200 | 滚轮 |

### 9.3 修饰键事件（op=1）

| 字段 | 含义 |
|---|---|
| a | 1=按下，0=抬起 |
| b | JS keyCode（仅支持 8/16/17/18/20/144/145） |

### 9.4 Unicode 字符输入（op=2）

| 字段 | 含义 |
|---|---|
| b | Unicode 码点 |

由 Go 转为按下+立即释放的两次 `freerdp_input_send_unicode_keyboard_event` 调用。

### 9.5 分辨率调整（op=3）

| 字段 | 含义 |
|---|---|
| a | 新宽度（像素） |
| b | 新高度（像素） |

通过 RDPEDISP 虚拟通道的 `SendMonitorLayout` 发送，需 DISP 通道已连接（`g_dispCtx != NULL`）。

---

## 10. 二次开发指南

### 10.1 新增绘图回调

以添加一个新的绘图指令处理为例（如 `LineTo`）：

**步骤 1**：在 CGo C 代码区声明 Go 导出函数：

```c
extern BOOL primaryLineTo(rdpContext* context, LINE_TO_ORDER* lineto);

static BOOL cbPrimaryLineTo(rdpContext* context, const LINE_TO_ORDER* lineto) {
    return primaryLineTo(context, (LINE_TO_ORDER*)lineto);
}
```

**步骤 2**：在 `cbPreConnect` 和 `cbPostConnect` 中注册回调：

```c
primary->LineTo = cbPrimaryLineTo;
```

**步骤 3**：在 Go 代码中实现处理函数：

```go
//export primaryLineTo
func primaryLineTo(rawContext *C.rdpContext, lineto *C.LINE_TO_ORDER) C.BOOL {
    d := lookupCtx(rawContext)
    if d == nil {
        return C.TRUE
    }
    // 定义新操作码常量（在文件顶部）
    // const WSOP_SC_LINETO uint32 = 13
    buf := new(bytes.Buffer)
    binary.Write(buf, binary.LittleEndian, WSOP_SC_LINETO)
    // 写入线段参数...
    sendBinary(d.sendq, buf)
    return C.TRUE
}
```

**步骤 4**：在前端 `wsgate-debug.js` 的 `_pmsg()` 中处理新的 op 码。

### 10.2 支持多并发 RDP 连接

当前 `g_dispCtx` 为全局单变量，只能支持单会话。支持多会话需：

1. 扩展 FreeRDP 的 `rdpContext`（在 CGo C 代码区定义自定义 context 结构体）：

```c
typedef struct {
    rdpContext base;       // 必须放在第一位
    DispClientContext* dispCtx;
} MyRdpContext;
```

2. 将 `instance.ContextSize` 改为 `sizeof(MyRdpContext)`
3. 在 `onChannelConnected` 等回调中通过 `(MyRdpContext*)context` 访问 per-instance 字段

### 10.3 增加颜色深度支持

当前强制使用 16bpp（RGB565）。若需支持 32bpp：

1. `preConnect` 中修改 `FreeRDP_ColorDepth` 为 32
2. `cbPostConnect` 中 `gdi_init` 的 `PIXEL_FORMAT_XRGB32` 保持不变（已是 32bpp）
3. `convertColor` 中处理 `srcBpp=32` 的情况
4. `flipImageData` 中 `bpp=32`，`scanline = width * 4`
5. 前端 `wsgate-debug.js` 中增加 32bpp 位图解码路径

### 10.4 增加新的输入事件类型

在 `rdpconnect` 的主事件循环中扩展 `switch ev.op`：

```go
case 4: // 自定义：剪贴板文本
    // C.sendClipboardText(instance, ...)
case 5: // 自定义：文件传输触发
    // ...
```

同时更新前端发送格式和 `inputEvent` 的 op 文档。

### 10.5 添加 WebSocket 安全认证

当前 `initSocket` 无认证逻辑。可在 `main.go` 中为 `/ws` 添加 HTTP Basic Auth 或 Token 中间件：

```go
http.Handle("/ws", authMiddleware(websocket.Handler(initSocket)))
```

`rdp.go` 无需修改，认证在 HTTP 握手阶段完成。

### 10.6 sendq/recvq/inputq 容量调整

| channel | 当前容量 | 调整建议 |
|---|---|---|
| sendq | 100 | 高分辨率/高刷新率场景可适当增大 |
| recvq | 5 | 无需调整，仅传递控制信号 |
| inputq | 50 | 高频鼠标移动场景可适当增大 |

---

## 11. 已知限制

| 限制 | 说明 |
|---|---|
| 颜色深度 | 固定 16bpp（RGB565），不支持 24/32 bpp |
| 多会话 | `g_dispCtx` 为全局变量，严格单会话；多并发会话会互相覆盖分辨率通道 |
| 安全协议 | 强制使用经典 RDP 安全层，禁用 NLA/TLS，不适合公网部署 |
| 证书验证 | `IgnoreCertificate=TRUE`，存在中间人攻击风险 |
| 修饰键 | 仅支持 7 个特定修饰键的扫描码路径，其余键均走 Unicode 字符路径 |
| 花纹画刷 | `primaryPatBlt` 仅处理 `GDI_BS_SOLID` 实心画刷，其他画刷类型直接忽略 |
| 光标同步 | 指针回调（PTR_NEW/SET 等）注册但均返回 TRUE 不处理，自定义光标未实现 |
| DELTA_RECT 布局 | `primaryMultiOpaqueRect` 依赖 C 结构体内存布局直接序列化，跨平台需验证 |
| 自动重连 | 无重连机制，断开后需前端刷新页面 |

---

*文档基于 `rdp.go`（FreeRDP 3.x CGo 绑定版）及 `main.go` 生成*

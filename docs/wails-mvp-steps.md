# Wails 桌面化 MVP 实施步骤

> 核心原则：先换壳，不换芯。`rdp.go` 和二进制帧协议保持不变，只替换宿主层和前端框架。

## 前置条件

- Go 1.24+
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- FreeRDP 3.x（macOS: `brew install freerdp`）
- Node.js 20.19+ 或 22.12+（Vite 前端构建要求）

---

## Step 1: 初始化 Wails 项目骨架

### 目标

在现有仓库中创建 Wails 应用结构，不影响现有代码。

### 操作

```bash
# 在项目根目录创建 Wails 骨架（前端选 vanilla TypeScript）
wails init -n desktop -t vanilla-ts -d cmd/desktop
```

### 产出目录结构

```
cmd/desktop/
├── main.go              # Wails 入口
├── app.go               # Wails 绑定对象
├── frontend/
│   ├── index.html
│   ├── src/
│   │   └── main.ts
│   ├── package.json
│   └── tsconfig.json
└── wails.json
```

### 注意

- `wails.json` 中的 CGo flags 需要与现有 `rdp.go` 保持一致
- 暂不删除根目录 `main.go`，保留旧入口可用

---

## Step 2: 提取 RDP 核心为独立 package

### 目标

将 `rdp.go` 和连接逻辑从根目录提取到 `internal/rdp/`，使 Wails 入口和旧入口都能调用。

### 操作

1. 创建 `internal/rdp/` 目录
2. 将 `rdp.go` 移入，调整 package 声明为 `package rdp`
3. 导出关键函数和类型：

```go
// internal/rdp/types.go
package rdp

type ConnectionSettings struct {
    Host       string
    Port       int
    User       string
    Pass       string
    Width      int
    Height     int
}
```

4. 将 `initSocket` 中的通道创建和 `rdpconnect` 启动逻辑封装为：

```go
// internal/rdp/session.go
package rdp

type Session struct {
    Sendq  chan []byte
    Recvq  chan []byte
    Inputq chan InputEvent
}

func NewSession(settings ConnectionSettings) *Session { ... }
func (s *Session) Start() { ... }    // 启动 rdpconnect goroutine
func (s *Session) Stop()  { ... }    // 发送断开信号
func (s *Session) SendInput(ev InputEvent) { ... }
```

5. 将 `main.go` 中的 WebSocket 桥接逻辑提取为：

```go
// internal/wsbridge/bridge.go
package wsbridge

func StartWSBridge(listenAddr string, rdpSettings rdp.ConnectionSettings) error
```

6. 旧 `main.go` 改为调用 `wsbridge.StartWSBridge()`，确保旧入口仍可用

### 验证

- `go build ./...` 编译通过
- 旧入口 `go run .` 功能不变
- 现有测试 `test/rdp/rdp_test.go` 通过

---

## Step 3: Wails 入口集成 WebSocket 桥接

### 目标

Wails 启动时在 `127.0.0.1` 启动本地 WebSocket 服务，前端通过该通道接收 RDP 帧。

### 操作

1. 编写 `cmd/desktop/app.go`：

```go
package main

import (
    "context"
    "fmt"
    "net"

    "github.com/user/project/internal/rdp"
    "github.com/user/project/internal/wsbridge"
)

type App struct {
    ctx    context.Context
    wsPort int
}

func NewApp() *App {
    return &App{}
}

func (a *App) startup(ctx context.Context) {
    a.ctx = ctx
    // 随机端口启动本地 WebSocket
    listener, _ := net.Listen("tcp", "127.0.0.1:0")
    a.wsPort = listener.Addr().(*net.TCPAddr).Port
    listener.Close()
    go wsbridge.StartWSBridge(fmt.Sprintf("127.0.0.1:%d", a.wsPort), rdp.ConnectionSettings{})
}

// Connect 供前端调用，返回 WebSocket 地址
func (a *App) Connect(host, user, pass string, port, width, height int) string {
    addr := fmt.Sprintf("ws://127.0.0.1:%d/ws?dtsize=%dx%d&host=%s&user=%s&pass=%s&port=%d",
        a.wsPort, width, height, host, user, pass, port)
    return addr
}
```

2. 编写 `cmd/desktop/main.go`：

```go
package main

import (
    "embed"
    "github.com/wailsapp/wails/v2"
    "github.com/wailsapp/wails/v2/pkg/options"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
    app := NewApp()
    wails.Run(&options.App{
        Title:     "QRDP",
        Width:     1024,
        Height:    768,
        Assets:    assets,
        OnStartup: app.startup,
        Bind:      []interface{}{app},
    })
}
```

### CGo 构建配置

`cmd/desktop/wails.json` 中需要添加：

```json
{
  "build": {
    "cgo": true,
    "cgoFlags": "-I/usr/local/opt/freerdp/include/freerdp3 -I/usr/local/opt/freerdp/include/winpr3",
    "cgoLdflags": "-L/usr/local/opt/freerdp/lib -lfreerdp3 -lfreerdp-client3 -lwinpr3"
  }
}
```

### 验证

- `cd cmd/desktop && wails dev` 能启动窗口
- 日志显示 WebSocket 服务已在随机端口监听

---

## Step 4: 前端实现 Canvas RDP 客户端

### 目标

用现代 TypeScript 重写 `wsgate-debug.js` 的核心逻辑（约 500 行）。

### 前端目录结构

```
cmd/desktop/frontend/src/
├── main.ts                 # 入口：页面路由（登录页 ↔ 远程桌面）
├── rdp/
│   ├── client.ts           # RDP WebSocket 客户端
│   ├── renderer.ts         # Canvas 渲染引擎
│   ├── input.ts            # 鼠标/键盘/触摸事件编码
│   └── protocol.ts         # 二进制帧协议常量和解析
├── ui/
│   ├── connect-form.ts     # 连接表单
│   └── toolbar.ts          # 顶部工具栏（断开按钮等）
└── style.css
```

### 核心模块说明

#### protocol.ts — 二进制帧协议

从 `wsgate-debug.js` 移植操作码和解析逻辑：

```typescript
// 服务端 → 客户端 操作码
export const WSOP = {
  BEGINPAINT: 0,
  ENDPAINT: 1,
  BITMAP: 2,
  OPAQUERECT: 3,
  SETBOUNDS: 4,
  PATBLT: 5,
  MULTI_OPAQUERECT: 6,
  SCRBLT: 7,
  // 光标: 8-12
} as const;

// 客户端 → 服务端 操作码
export const INPUT_OP = {
  MOUSE: 0,
  KEY_UPDOWN: 1,
  KEY_CHAR: 2,
  RESIZE: 3,
} as const;
```

#### renderer.ts — Canvas 渲染

移植 `_pmsg()` 中的渲染逻辑：

- `handleBitmap()` — RLE16 解码 + putImageData
- `handleOpaqueRect()` — fillRect 纯色填充
- `handleScrBlt()` — getImageData/putImageData 屏幕块复制
- `handleSetBounds()` — Canvas clip 区域设置

#### input.ts — 输入事件编码

移植鼠标/键盘事件编码为 16 字节二进制消息：

```typescript
export function encodeMouseEvent(flags: number, x: number, y: number): ArrayBuffer
export function encodeKeyEvent(down: boolean, keycode: number): ArrayBuffer
export function encodeCharEvent(modifiers: number, charcode: number): ArrayBuffer
export function encodeResizeEvent(width: number, height: number): ArrayBuffer
```

#### client.ts — WebSocket 客户端

```typescript
export class RDPClient {
  private ws: WebSocket;
  private renderer: RDPRenderer;

  connect(wsUrl: string): void {
    this.ws = new WebSocket(wsUrl);
    this.ws.binaryType = 'arraybuffer';
    this.ws.onmessage = (e) => this.handleMessage(e.data);
  }

  private handleMessage(data: ArrayBuffer): void {
    // 解析操作码，分派到 renderer
  }

  sendInput(buf: ArrayBuffer): void {
    this.ws.send(buf);
  }

  disconnect(): void {
    this.ws.close();
  }
}
```

### 页面流程

```
启动 → 连接表单页
  ↓ 用户填写并点击"连接"
调用 window.go.main.App.Connect(host, user, pass, port, w, h)
  ↓ 返回 ws://127.0.0.1:PORT/ws?...
RDPClient.connect(wsUrl)
  ↓ 切换到 Canvas 远程桌面页
点击"断开" → RDPClient.disconnect() → 返回连接表单页
```

### 验证

- 连接真实 RDP 服务器，Canvas 能渲染远程桌面画面
- 鼠标移动/点击、键盘输入正常
- 断开连接后能重新连接

---

## Step 5: 连接参数走 Wails 绑定，不走 URL

### 目标

密码等敏感信息不再出现在 WebSocket URL 中。

### 操作

1. 修改 `wsbridge`，支持通过 Go 侧注册连接参数：

```go
// internal/wsbridge/bridge.go
var pendingSettings sync.Map // token → ConnectionSettings

func RegisterConnection(settings rdp.ConnectionSettings) string {
    token := uuid.New().String()
    pendingSettings.Store(token, settings)
    return token
}
```

2. WebSocket 连接改为只传 token：

```
ws://127.0.0.1:PORT/ws?token=xxx&dtsize=WxH
```

3. `initSocket` 中通过 token 查找完整连接参数

4. `App.Connect()` 改为：

```go
func (a *App) Connect(host, user, pass string, port, width, height int) string {
    token := wsbridge.RegisterConnection(rdp.ConnectionSettings{
        Host: host, Port: port,
        User: user, Pass: pass,
        Width: width, Height: height,
    })
    return fmt.Sprintf("ws://127.0.0.1:%d/ws?token=%s", a.wsPort, token)
}
```

### 验证

- URL 中不包含密码
- 连接功能不受影响

---

## Step 6: 验证 FreeRDP 动态库打包

### 目标

确保 `wails build` 产物在目标平台可直接运行，无需手动设置 `DYLD_LIBRARY_PATH`。

### macOS

```bash
cd cmd/desktop && wails build
# 检查产物中的 dylib 依赖
otool -L build/bin/desktop.app/Contents/MacOS/desktop
```

方案选项：
- **方案 a**：用 `install_name_tool` 修改 rpath，将 dylib 拷入 `.app/Contents/Frameworks/`
- **方案 b**：静态链接 FreeRDP（需从源码编译，工作量大）

### Linux

```bash
wails build
ldd build/bin/desktop
```

- 打包为 AppImage 时将 `.so` 放入 `usr/lib/`
- 或依赖系统安装 `libfreerdp3`

### Windows

- 将 `freerdp3.dll` / `winpr3.dll` 放在可执行文件同目录
- 或嵌入安装包（NSIS / WiX）

### 验证

- 在干净环境（无 FreeRDP 开发包）中运行打包产物
- 应用能正常启动并连接 RDP

---

## 里程碑检查清单

| # | 里程碑 | 完成标志 |
|---|--------|----------|
| 1 | Wails 骨架可运行 | `wails dev` 打开空窗口 |
| 2 | RDP 核心提取完成 | 旧入口 `go run .` 功能不变，测试通过 |
| 3 | Wails + WebSocket 桥接 | 窗口内可看到 WebSocket 服务日志 |
| 4 | Canvas 渲染可用 | 连接真实 RDP 服务器，画面正常显示 |
| 5 | 输入事件可用 | 鼠标点击、键盘输入、窗口缩放正常 |
| 6 | 密码不走 URL | URL 中无明文密码 |
| 7 | 打包验证通过 | 干净环境可直接运行 |

---

## 暂不处理（后续 Phase）

- 去掉旧 `main.go` HTTP 入口（Phase 2）
- 多会话/多标签页支持（Phase 4）
- `g_dispCtx` 全局状态改 per-session（Phase 4）
- 系统托盘、自动更新、凭据安全存储（Phase 4）
- 旧前端 `webroot/` 删除（Phase 2 完成后）

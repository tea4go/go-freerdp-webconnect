# wsgate-debug.js 设计文档

> 文件路径：`webroot/js/wsgate-debug.js`
> 依赖：MooTools、modernizr、simpletabs、vkb-debug.js

---

## 目录

1. [概述](#1-概述)
2. [架构总览](#2-架构总览)
3. [快速上手](#3-快速上手)
4. [公共 API 参考](#4-公共-api-参考)
5. [WebSocket 通信协议](#5-websocket-通信协议)
6. [RDP 渲染管线](#6-rdp-渲染管线)
7. [位图解码与颜色转换](#7-位图解码与颜色转换)
8. [RLE16 解压算法](#8-rle16-解压算法)
9. [输入事件处理](#9-输入事件处理)
10. [光标管理](#10-光标管理)
11. [工具函数](#11-工具函数)
12. [已知限制](#12-已知限制)

---

## 1. 概述

`wsgate-debug.js` 是 QRDP 项目的前端核心模块，实现了一个**基于 HTML5 Canvas + WebSocket 的 RDP 远程桌面客户端**。

主要职责：

- 通过 WebSocket 与后端（Go 服务）保持长连接
- 接收二进制 RDP 绘图指令并渲染到 `<canvas>` 元素
- 将用户的鼠标、键盘、触摸输入转换为二进制消息发回服务端
- 管理服务端下发的自定义光标

所有公共标识符均挂载在 `wsgate` 命名空间下，避免全局变量污染。

---

## 2. 架构总览

### 2.1 类层次

```
MooTools Events (Mixin)
    │
    └── wsgate.WSrunner          ← WebSocket 连接基类
            │
            └── wsgate.RDP       ← RDP 客户端主类（继承 WSrunner）

wsgate.Log                       ← 日志工具（独立类，在 RDP 内部实例化）
```

### 2.2 模块职责划分

| 模块 / 标识符 | 职责 |
|---|---|
| `wsgate.Log` | 双通道日志：浏览器控制台 + WebSocket 远程日志 |
| `wsgate.WSrunner` | WebSocket 生命周期管理（创建、绑定四个事件处理器） |
| `wsgate.RDP` | RDP 绘图指令解析、Canvas 渲染、输入事件采集 |
| `wsgate.dRLE16_RGBA` | RLE16（NSCodec）位图解压 |
| `wsgate.dRGB162RGBA` | RGB565 → RGBA 无压缩解码 |
| `wsgate.flipV` | 位图垂直翻转（处理 bottom-up 存储格式） |
| `wsgate.copyRGBA` 等 | 像素级操作辅助函数 |

### 2.3 数据流

```
服务端 (Go)
    │  WebSocket
    │  ┌─────────────────────────────┐
    │  │  文本消息（控制信令）         │
    │  │    T:标题  S:会话ID          │
    │  │    E:错误  I:信息            │
    │  │    W:警告  D:调试            │
    │  └─────────────────────────────┘
    │  ┌─────────────────────────────┐
    │  │  二进制消息（RDP 绘图指令）   │
    │  │  [uint32 op | 参数 | 数据]  │
    │  └─────────────────────────────┘
    ↓
wsgate.RDP.onWSmsg()
    ├── 文本消息 → 分发 alert/connected/disconnected 事件
    └── 二进制消息 → _pmsg() → 解析 op 码 → Canvas 绘图

用户输入（鼠标/键盘/触摸）
    ↓
wsgate.RDP 事件处理器（onMm/onMd/onMu/onKd/onKu/onTs 等）
    ↓
构造 16 字节 ArrayBuffer
    ↓
WebSocket.send() → 服务端
```

---

## 3. 快速上手

### 3.1 最小化示例

```html
<!-- 依赖顺序不可更改 -->
<script src="js/modernizr-debug.js"></script>
<script src="js/mootools-debug.js"></script>
<script src="js/simpletabs-debug.js"></script>
<script src="js/wsgate-debug.js"></script>
<script src="js/vkb-debug.js"></script>

<canvas id="screen" width="1024" height="768"></canvas>

<script>
// WebSocket 地址：ws[s]://<host>:<port>/ws?dtsize=<宽>x<高>
var wsUri = 'ws://localhost:4455/ws?dtsize=1024x768';

// 构造 RDP 客户端
//   参数1: WebSocket URL
//   参数2: Canvas DOM 元素
//   参数3: 是否用 CSS cursor（true=CSS；false=<img>元素）
//   参数4: 是否启用触摸支持
//   参数5: 虚拟键盘对象（可为 null）
var rdp = new wsgate.RDP(wsUri, $('screen'), true, false, null);

// 注册事件回调
rdp.addEvent('alert',        function(msg) { alert(msg); });
rdp.addEvent('connected',    function()    { console.log('已连接'); });
rdp.addEvent('disconnected', function()    { console.log('已断开'); });

// 启动连接
rdp.Run();
</script>
```

### 3.2 带虚拟键盘的完整示例

参见 `webroot/index-debug.html`，其中演示了：

- 触摸手势与虚拟键盘的联动（`touch2`/`touch3`/`touch4` 事件）
- Canvas 自适应窗口大小（`Resize` API）
- 桌面尺寸参数从 URL query string 传入服务端

---

## 4. 公共 API 参考

### 4.1 `wsgate.RDP` 构造函数

```javascript
new wsgate.RDP(url, canvas, cssCursor, useTouch, vkbd)
```

| 参数 | 类型 | 说明 |
|---|---|---|
| `url` | `string` | WebSocket 服务器地址，含 `?dtsize=WxH` 查询参数 |
| `canvas` | `Element` | 渲染目标 Canvas DOM 元素 |
| `cssCursor` | `boolean` | `true`：使用 CSS `cursor` 属性显示自定义光标；`false`：用绝对定位 `<img>` 模拟 |
| `useTouch` | `boolean` | 是否启用触摸事件绑定（平板/手机场景） |
| `vkbd` | `Object\|null` | 虚拟键盘实例，需提供 `vkpress` 事件；不需要时传 `null` |

### 4.2 实例方法

| 方法 | 说明 |
|---|---|
| `Run()` | 建立 WebSocket 连接并绑定所有输入事件监听器 |
| `Disconnect()` | 主动断开连接，重置所有状态 |
| `Resize(w, h)` | 通知服务端调整桌面分辨率，同步更新 Canvas 和离屏缓冲尺寸 |
| `SetArtificialMouseFlags(mf)` | 设置人工鼠标修饰标志（触摸屏右键/中键/Alt/Shift/Ctrl 模拟） |

#### `Resize(w, h)` 详解

```javascript
rdp.Resize(1280, 720);
// 1. 发送 16 字节控制消息给服务端（op=WSOP_CS_RDPSET）
// 2. 更新 canvas.width / canvas.height
// 3. 同步更新离屏缓冲 bstore 的尺寸
```

#### `SetArtificialMouseFlags(mf)` 详解

```javascript
// 设置：触摸点击模拟右键+Ctrl
rdp.SetArtificialMouseFlags({ r: true, m: false, a: false, s: false, c: true });

// 清除（恢复正常左键行为）
rdp.SetArtificialMouseFlags(null);
```

| 标志键 | 含义 |
|---|---|
| `r` | 右键点击 |
| `m` | 中键点击 |
| `a` | Alt 修饰 |
| `s` | Shift 修饰 |
| `c` | Ctrl 修饰 |

### 4.3 事件系统

`wsgate.RDP` 实现了 MooTools `Events` mixin，支持 `addEvent` / `removeEvent`：

| 事件名 | 触发时机 | 回调参数 |
|---|---|---|
| `alert` | 服务端发送 `E:` 错误消息 | `msg: string` |
| `connected` | WebSocket 连接成功建立 | 无 |
| `disconnected` | WebSocket 连接关闭 | 无 |
| `mouserelease` | 鼠标按键释放（用于重置触摸修饰标志） | 无 |
| `touch2` | 检测到双指触摸手势 | 无 |
| `touch3` | 检测到三指触摸手势 | 无 |
| `touch4` | 检测到四指触摸手势 | 无 |

---

## 5. WebSocket 通信协议

### 5.1 连接地址格式

```
ws[s]://<host>:<port>/ws?dtsize=<宽>x<高>
```

示例：`ws://localhost:4455/ws?dtsize=1024x768`

### 5.2 文本消息（客户端 → 服务端，用于日志）

由 `wsgate.Log` 发送，格式为 `<前缀><内容>`：

| 前缀 | 级别 |
|---|---|
| `D:` | DEBUG |
| `I:` | INFO |
| `W:` | WARN |
| `E:` | ERROR |

### 5.3 文本消息（服务端 → 客户端，控制信令）

| 前缀 | 含义 | 处理 |
|---|---|---|
| `T:` | 页面标题 | 更新 `document.title` |
| `S:` | 会话 ID | 保存为 `this.sid`，用于拼接光标图片 URL |
| `E:` | 错误信息 | 触发 `alert` 事件 |
| `I:` | 信息 | 触发 `alert` 事件 |
| `W:` | 警告 | 触发 `alert` 事件 |
| `D:` | 调试 | 记录日志 |

### 5.4 二进制消息（客户端 → 服务端）

所有客户端发出的二进制消息均为 **16 字节**，使用 `ArrayBuffer` + `Uint32Array`。

#### 鼠标移动消息（WSOP_CS_MOUSE）

```
偏移  类型      值
0     uint32   0x00000000  (op = WSOP_CS_MOUSE)
4     uint32   PTR_FLAGS   (指针标志，见下表)
8     uint32   x           (鼠标 X 坐标)
12    uint32   y           (鼠标 Y 坐标)
```

| PTR_FLAGS 常量 | 十六进制 | 含义 |
|---|---|---|
| `PTR_FLAGS_MOVE` | `0x0800` | 鼠标移动 |
| `PTR_FLAGS_BUTTON1` | `0x1000` | 左键 |
| `PTR_FLAGS_BUTTON2` | `0x2000` | 右键 |
| `PTR_FLAGS_BUTTON3` | `0x4000` | 中键 |
| `PTR_FLAGS_DOWN` | `0x8000` | 按下（与按键标志 OR） |
| `PTR_FLAGS_WHEEL` | `0x0200` | 滚轮 |
| `PTR_FLAGS_WHEEL_NEGATIVE` | `0x0100` | 滚轮反转 |
| `WheelRotationMask` | `0x01FF` | 滚轮旋转量掩码 |

#### 键盘消息（WSOP_CS_RDPKEY）

```
偏移  类型      值
0     uint32   0x00000001  (op = WSOP_CS_RDPKEY)
4     uint32   flags       (KBD_FLAGS_DOWN=0x4000 或 KBD_FLAGS_RELEASE=0x8000)
8     uint32   keyCode     (JavaScript event.keyCode)
12    uint32   0           (保留)
```

#### 桌面大小调整消息（WSOP_CS_RDPSET）

```
偏移  类型      值
0     uint32   0x00000002  (op = WSOP_CS_RDPSET)
4     uint32   w           (新宽度)
8     uint32   h           (新高度)
12    uint32   0           (保留)
```

### 5.5 二进制消息（服务端 → 客户端，RDP 绘图指令）

格式：`[uint32 op | 参数字段 | 可变长度数据]`，详见第 6 节。

---

## 6. RDP 渲染管线

### 6.1 操作码（op）总表

| op | 名称 | 说明 |
|---|---|---|
| 0 | `BeginPaint` | 开始绘制帧，保存 Canvas 状态（`save()`） |
| 1 | `EndPaint` | 结束绘制帧，恢复 Canvas 状态（`restore()`） |
| 2 | `Bitmap` | 渲染单张位图（支持 RGB565 无压缩和 RLE16 压缩） |
| 3 | `OpaqueRect` | 不透明颜色矩形填充（`fillRect`） |
| 4 | `SetBounds` | 设置裁剪区域（替换当前 clip path） |
| 5 | `PatBlt` | 图案位块传输（支持实心画刷 + ROP3） |
| 6 | `MultiOpaqueRect` | 批量不透明矩形填充 |
| 7 | `ScrBlt` | 屏幕内区域复制（`getImageData` → `putImageData`） |
| 8 | `PTR_NEW` | 注册新光标（含 id、热点坐标） |
| 9 | `PTR_FREE` | 删除光标缓存 |
| 10 | `PTR_SET` | 切换到指定光标 |
| 11 | `PTR_SETNULL` | 隐藏光标 |
| 12 | `PTR_SETDEFAULT` | 恢复默认系统光标 |

### 6.2 消息体格式详解

#### op=2 位图（Bitmap）

```
偏移  大小    字段
0     4       op = 2
4     4×9=36  header: [x, y, srcW, srcH, dstW, dstH, bpp, compressed, dataLen]
40    dataLen 像素数据（RGB565 或 RLE16）
```

- `bpp`：目前仅支持 `15` 和 `16`（RGB565）
- `compressed = 1`：数据为 RLE16 格式，需先解压再垂直翻转
- `compressed = 0`：数据为原始 RGB565，直接解码

#### op=3 不透明矩形

```
偏移  大小  字段
0     4     op = 3
4     16    Int32[4]: [x, y, w, h]
20    4     Uint8[4]: [R, G, B, A]
```

#### op=4 设置裁剪区（SetBounds）

```
偏移  大小  字段
0     4     op = 4
4     16    Int32[4]: [left, top, right, bottom]
```

转换：`w = right - left`，`h = bottom - top`

#### op=5 PatBlt（实心画刷）

```
偏移  大小  字段
0     4     op = 5
4     16    Int32[4]: [x, y, w, h]
20    4     Uint8[4]: [R, G, B, A]
24    4     Uint32: rop3 操作码
```

总长度 28 字节。花纹画刷（patterned brush）尚未实现。

#### op=6 批量矩形

```
偏移  大小  字段
0     4     op = 6
4     4     Uint8[4]: [R, G, B, A]
8     4     Uint32: nrects（矩形数量）
12    4×4×nrects  Uint32[nrects][4]: [x, y, w, h] 数组
```

#### op=7 ScrBlt（屏幕复制）

```
偏移  大小  字段
0     4     op = 7
4     4     Uint32: rop3
8     24    Int32[6]: [x, y, w, h, sx, sy]
```

### 6.3 Canvas 状态管理

`BeginPaint`/`EndPaint` 使用嵌套计数器 `ccnt` 保证 `save()`/`restore()` 调用平衡：

```
ccnt=0  →  BeginPaint → save() → ccnt=1
                       SetBounds → clip
                       Bitmap/Fill/ScrBlt ...
           EndPaint   → restore() → ccnt=0
```

### 6.4 离屏缓冲的设计原因

HTML5 Canvas 的 `putImageData()` **不遵守** `clip()` 设置的裁剪区域，而 `drawImage()` **遵守**裁剪。

因此当位图目标区域**跨越裁剪边界**时，采用以下两步策略：

```
1. bctx.putImageData(outB, 0, 0)    ← 写入离屏缓冲（不受裁剪）
2. cctx.drawImage(bstore, ...)       ← 从离屏缓冲贴图（受裁剪）
```

当目标区域完全在裁剪范围内时，直接 `cctx.putImageData()` 以提升性能。

### 6.5 ROP3 光栅操作映射

| ROP3 操作码 | GDI 名称 | Canvas 合成模式 |
|---|---|---|
| `0x005A0049` | PATINVERT（D = P ^ D） | `xor` |
| `0x00F00021` | PATCOPY（D = P） | `copy` |
| `0x00CC0020` | SRCCOPY（D = S） | `source-over` |

其他 ROP3 操作码记录警告并跳过。

---

## 7. 位图解码与颜色转换

### 7.1 RGB565 → RGBA（无压缩）

函数：`wsgate.dRGB162RGBA(src, len, dst)`

RGB565 格式（16 位小端）：

```
位15-11: R (5位) → R8 = (R5 << 3) | (R5 >> 2)
位10-5:  G (6位) → G8 = (G6 << 2) | (G6 >> 4)
位4-0:   B (5位) → B8 = (B5 << 3) | (B5 >> 2)
A8 = 255（不透明）
```

### 7.2 垂直翻转（bottom-up 修正）

RDP 规范中位图数据**从底行开始**存储（bottom-up），Canvas 从顶行开始渲染（top-down）。

函数：`wsgate.flipV(buf, width, height)`

逐行交换（仅交换前半行与后半行）：

```
行0  ←→  行(height-1)
行1  ←→  行(height-2)
...
```

每个像素 4 字节（RGBA），行步长 = `width × 4`。

### 7.3 像素工具函数汇总

| 函数 | 说明 |
|---|---|
| `wsgate.copyRGBA(src, sOff, dst, dOff)` | 复制单个 RGBA 像素（4 字节） |
| `wsgate.xorbufRGBAPel16(dst, dOff, src, sOff)` | 将 RGB565 像素 XOR 到 RGBA 缓冲区 |
| `wsgate.buf2RGBA(src, sOff, dst, dOff, count)` | 批量复制 RGBA 像素 |
| `wsgate.pel2RGBA(pel, dst, dOff)` | 将 RGB565 整数写入 RGBA 缓冲区 |

---

## 8. RLE16 解压算法

### 8.1 概述

函数：`wsgate.dRLE16_RGBA(src, srcLen, width, dst)`

实现 RDP NSCodec 的 RLE16（Run-Length Encoding for 16-bit bitmaps）解压。
输出为 RGBA 格式，直接可用于 `ImageData.data`。

### 8.2 辅助函数

```
wsgate.ExtractCodeId(byte)      → 提取操作码 ID（高 4 位）
wsgate.ExtractRunLength(byte, src, off) → 提取游程长度（可能读取额外字节）
wsgate.WriteFgBgImage16toRGBA(...)   → 写入 FG/BG 位掩码行
wsgate.WriteFirstLineFgBgImage16toRGBA(...) → 写入第一行 FG/BG 位掩码
```

### 8.3 操作码表

| 操作码范围 | 名称 | 说明 |
|---|---|---|
| `0x00~0x0F` | REGULAR_BG_RUN | 重复写入背景色 |
| `0x10~0x1F` | MEGA_MEGA_BG_RUN | 大游程背景色（后跟 2 字节长度） |
| `0x20~0x2F` | REGULAR_FG_RUN | 重复写入前景色 |
| `0x30~0x3F` | MEGA_MEGA_FG_RUN | 大游程前景色 |
| `0x40~0x4F` | REGULAR_FGBG_IMAGE | FG/BG 位掩码游程 |
| `0x50~0x5F` | MEGA_MEGA_FGBG_IMAGE | 大游程 FG/BG 位掩码 |
| `0x60~0x6F` | REGULAR_COLOR_RUN | 重复写入指定颜色 |
| `0x70~0x7F` | MEGA_MEGA_COLOR_RUN | 大游程指定颜色 |
| `0x80~0x8F` | REGULAR_COLOR_IMAGE | 原始像素游程（无编码） |
| `0x90~0x9F` | MEGA_MEGA_COLOR_IMAGE | 大游程原始像素 |
| `0xA0~0xAF` | REGULAR_PACKED_COLOR_RUN | 紧凑颜色游程 |
| `0xB0~0xBF` | MEGA_MEGA_PACKED_COLOR_RUN | 大游程紧凑颜色 |
| `0xC0~0xC7` | LITE_SET_FG_FG_RUN | 设置前景色并重复 |
| `0xC8~0xCF` | MEGA_MEGA_SET_FG_FG_RUN | 大游程设置前景色 |
| `0xD0~0xD7` | LITE_SET_FG_FGBG_IMAGE | 设置前景色的 FG/BG 位掩码 |
| `0xD8~0xDF` | MEGA_MEGA_SET_FG_FGBG_IMAGE | 大游程 |
| `0xE0~0xE7` | LITE_DITHERED_RUN | 双色交替游程 |
| `0xE8~0xEF` | MEGA_MEGA_DITHERED_RUN | 大游程双色交替 |
| `0xF0` | SPECIAL_FGBG_1 | 特殊 FG/BG（固定掩码 0xAA） |
| `0xF1` | SPECIAL_FGBG_2 | 特殊 FG/BG（固定掩码 0x55） |
| `0xF8` | WHITE | 写入白色像素 |
| `0xF9` | BLACK | 写入黑色像素 |

### 8.4 游程长度编码规则

```
低4位 == 0xF → 后跟 uint16 大游程长度（MEGA_MEGA 系列除外）
MEGA_MEGA 系列 → 后跟固定 2 字节长度
否则 → 低4位即为短游程长度（最小值 1）
```

### 8.5 FG/BG 位掩码模式

FG/BG 模式每字节控制 8 个像素，bit=1 写前景色，bit=0 写背景色（第一行例外，bit=0 写入 0x0000 黑色）：

```
位7 → 像素0，位6 → 像素1，...，位0 → 像素7
```

---

## 9. 输入事件处理

### 9.1 鼠标事件

| 内部方法 | DOM 事件 | 说明 |
|---|---|---|
| `onMm` | `mousemove` | 发送移动消息，更新自定义光标位置 |
| `onMd` | `mousedown` | 发送按下消息（左/中/右键） |
| `onMu` | `mouseup` | 发送释放消息，触发 `mouserelease` 事件 |
| `onMw` | `mousewheel` / `DOMMouseScroll` | 发送滚轮消息（含方向和旋转量） |

鼠标消息编码（以鼠标移动为例）：

```javascript
a[0] = 0;          // WSOP_CS_MOUSE
a[1] = 0x0800;     // PTR_FLAGS_MOVE
a[2] = x;
a[3] = y;
```

### 9.2 键盘事件

| 内部方法 | DOM 事件 | 说明 |
|---|---|---|
| `onKd` | `keydown` | 发送按键消息（含修饰键判断） |
| `onKu` | `keyup` | 发送按键释放消息 |
| `onKv` | `vkpress`（虚拟键盘） | 处理虚拟键盘的 keydown/keyup 序列 |
| `onKp` | `keypress` | 阻止默认行为，防止浏览器吞掉特殊键 |

特殊修饰键列表（需特殊处理，不走常规 keyCode 路径）：

```
Backspace(8), Shift(16), Ctrl(17), Alt(18),
CapsLock(20), NumLock(144), ScrollLock(145)
```

### 9.3 触摸事件处理

触摸模式下，单指触摸转换为鼠标事件，多指触摸触发手势事件：

```
1指 → 模拟鼠标（mousemove/mousedown/mouseup）
2指 → 触发 'touch2' 事件（显示/隐藏鼠标辅助面板）
3指 → 触发 'touch3' 事件（切换虚拟键盘）
4指 → 触发 'touch4' 事件（显示断开确认对话框）
```

#### 防抖冷却机制

多指触摸结束后进入**冷却状态**（`Tcool = false`），冷却期内忽略后续触摸事件，防止多指触摸释放后单指误操作：

```
多指 touchend → 触发手势事件 → 启动 500ms 冷却定时器
冷却期内的 touchstart → 忽略
冷却结束 → Tcool = true → 恢复正常触摸处理
```

---

## 10. 光标管理

### 10.1 两种光标模式

**CSS cursor 模式**（`cssCursor=true`，推荐，桌面浏览器）：

- 服务端下发光标后，构造 CSS `url()` 字符串缓存在 `this.cursors[id]`
- 切换时调用 `canvas.setStyle('cursor', ...)`
- 光标图片 URL 格式：`/cur/<sessionId>/<cursorId>`

**IMG 元素模式**（`cssCursor=false`，BlackBerry Tablet 等兼容场景）：

- 在 Canvas 上方叠加绝对定位的 `<img>` 元素（z-index: 998）
- `mousemove` 时调用 `cP()` 更新 `<img>` 位置
- 热点坐标 `(chx, chy)` 用于对齐实际点击位置

### 10.2 光标操作码流程

```
PTR_NEW(id, xhot, yhot) → 缓存 cursors[id]
PTR_SET(id)              → 切换到 cursors[id]
PTR_SETNULL              → 隐藏光标（cursor:none 或 c_none.png）
PTR_SETDEFAULT           → 恢复默认（cursor:default 或 c_default.png）
PTR_FREE(id)             → cursors[id] = undefined
```

---

## 11. 工具函数

### 11.1 `wsgate.o2s(obj, depth)`

将任意 JS 对象序列化为可读字符串，用于日志输出。支持：
- 基础类型：直接转换
- 数组/对象：递归展开
- 循环引用：检测并返回 `{SELF}`
- UIEvent 的危险属性（`layerX`/`layerY`/`view`）：跳过
- `HTMLElement`：输出 `{HTMLElement}` 占位符

### 11.2 `wsgate.Log`

双通道日志工具：

```javascript
var log = new wsgate.Log();
log.setWS(websocket);   // 设置 WebSocket 输出通道
log.debug('msg', obj);  // → 控制台 console.debug + WebSocket D: 前缀
log.info('...');        // I:
log.warn('...');        // W:
log.err('...');         // E:
log.drop();             // 空操作（丢弃日志）
```

---

## 12. 已知限制

| 限制 | 说明 |
|---|---|
| 颜色深度 | 仅支持 RGB565（15/16 bpp），不支持 24/32 bpp |
| PatBlt | 仅支持实心画刷，花纹画刷（patterned brush）未实现 |
| ROP3 | 仅支持 PATINVERT / PATCOPY / SRCCOPY 三种操作 |
| 花纹画刷 | PatBlt 的花纹画刷分支输出 warn 但不处理 |
| `_fR` | 已禁用（函数开头直接 return），保留代码仅供参考 |
| 触摸 | 多指手势依赖 500ms 冷却，快速连续手势可能被丢弃 |
| WebSocket 重连 | 无自动重连机制，断开后需页面刷新 |

---

*文档生成于 wsgate-debug.js（含中文注释版，1663 行）*

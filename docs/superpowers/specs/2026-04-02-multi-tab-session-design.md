# 多页签会话管理系统 — 设计规格

## 概述

为 QRDP 应用添加左侧竖排多页签系统，支持同时管理多个 RDP 远程桌面会话。页签支持右键上下文菜单，提供断开、全屏、自适应、窗口控制等操作。

## 布局结构

### 整体布局

```
┌──────┬────────────────────────────────────┐
│      │           内容区域                  │
│  页  │                                    │
│  签  │   主页（ConnectForm）               │
│  栏  │   或                               │
│      │   远程桌面（RemoteDesktop）          │
│ 48px │                                    │
│      │                                    │
└──────┴────────────────────────────────────┘
```

- **页签栏**：宽度 48px，固定于左侧，深色背景 (`#1e293b`)
- **内容区**：占满剩余空间，显示当前激活页签对应的内容

### 页签样式

| 页签类型 | 图标 | 状态指示 | 是否可关闭 |
|---------|------|---------|-----------|
| 主页 | 房子图标 | 无 | 否（始终存在） |
| 远程桌面 | 显示器图标 | 绿色圆点（已连接） | 通过右键"断开"关闭 |

- 激活页签：蓝色背景 (`#3b82f6`)，白色图标
- 非激活页签：深灰背景 (`#334155`)，灰色图标 (`#94a3b8`)
- 尺寸：40x40px，圆角 8px
- 间距：页签间 2px gap
- 悬停：背景色变亮至 `#475569`
- 鼠标悬停显示 tooltip（主机名或"主页"）

## 会话管理

### 数据模型

```typescript
interface Session {
  id: string            // 唯一标识（时间戳+随机数）
  wsUrl: string         // WebSocket URL
  host: string          // 显示用主机名
  width: number         // RDP 分辨率宽
  height: number        // RDP 分辨率高
  status: 'connecting' | 'connected' | 'disconnected'
}
```

### 多会话并行

- 所有 RemoteDesktop 实例使用 `v-show` 而非 `v-if` 控制显隐
- 切换页签时，非激活会话的 WebSocket 连接和 canvas 保持运行
- 断开连接时，移除对应 session 并销毁组件实例
- 主页始终存在，`id` 为 `'home'`

### 状态管理

在 `App.vue` 中使用响应式状态：

```typescript
const sessions = ref<Session[]>([])
const activeTabId = ref<string>('home')  // 'home' 或 session.id
```

不引入额外的状态管理库，保持与现有代码风格一致（ref + reactive）。

## 右键上下文菜单

### 远程桌面页签菜单

| 菜单项 | 操作 | 分组 |
|-------|------|------|
| 断开 | 断开 WebSocket，移除 session，切换到主页 | 连接 |
| --- | 分隔线 | |
| 全屏远程桌面 | 调用 `WindowFullscreen()`，隐藏页签栏 | 显示 |
| 自适应远程桌面 | 先尝试 DISP resize，失败则 CSS 缩放画布 | 显示 |
| --- | 分隔线 | |
| 窗口置顶 | 调用 `WindowSetAlwaysOnTop(true/false)`，菜单项带勾选状态 | 窗口 |
| 窗口最大化 | 调用 `WindowMaximise()` | 窗口 |
| 窗口最小化 | 调用 `WindowMinimise()` | 窗口 |
| --- | 分隔线 | |
| 退出 | 断开所有会话，调用 `Quit()` 关闭应用 | 应用 |

### 主页页签菜单

| 菜单项 | 操作 |
|-------|------|
| 窗口置顶 | 同上 |
| 窗口最大化 | 同上 |
| 窗口最小化 | 同上 |
| --- | 分隔线 |
| 退出 | 同上 |

### 菜单行为

- 右键页签图标触发，出现在鼠标位置附近
- 点击菜单外区域或执行操作后自动关闭
- 菜单样式：白色背景、圆角 8px、阴影 `0 4px 16px rgba(0,0,0,0.15)`
- 菜单项高度：30px，悬停高亮
- "退出" 文字为红色 (`#ef4444`)
- "窗口置顶" 激活时显示勾选标记 (✓)

## 全屏模式

- 调用 Wails JS Runtime 的 `WindowFullscreen()` 进入全屏
- 全屏时隐藏左侧页签栏，远程桌面画布占满整个窗口
- 按 `Esc` 键或双击画布顶部边缘区域退出全屏，调用 `WindowUnfullscreen()`
- 退出全屏后恢复页签栏显示
- 用一个 `isFullscreen` ref 跟踪全屏状态

## 自适应远程桌面

### 策略

优先通过 RDP DISP 通道动态调整远程桌面分辨率（无黑边无滚动条），如不支持则回退为 CSS 缩放画布适应窗口。

### 实现

1. **DISP resize（优先）**：发送 `encodeResizeEvent(newWidth, newHeight)` 到 WebSocket，后端通过 `sendResizeInput` C 函数调整远程分辨率
2. **CSS 缩放（回退）**：计算 `scale = min(containerWidth/canvasWidth, containerHeight/canvasHeight)`，通过 `transform: scale()` + `transform-origin: top left` 缩放 canvas
3. 用一个 `fitMode` 状态标记当前会话是否启用自适应，默认关闭
4. 启用后监听窗口 resize 事件，动态调整

## 窗口控制

全部通过 `@wailsapp/runtime` 的 JS API：

| 操作 | API |
|------|-----|
| 置顶 | `WindowSetAlwaysOnTop(bool)` |
| 最大化 | `WindowMaximise()` |
| 最小化 | `WindowMinimise()` |
| 全屏 | `WindowFullscreen()` / `WindowUnfullscreen()` |
| 退出 | `Quit()` |

退出前应断开所有活跃的 RDP 会话，确保资源正确释放。

## 组件结构

### 改动文件

```
frontend/src/
├── App.vue                    # 重构：布局容器（页签栏 + 内容区）
├── components/
│   ├── TabBar.vue             # 新增：左侧页签栏组件
│   └── ContextMenu.vue        # 新增：通用右键菜单组件
├── views/
│   ├── ConnectForm.vue        # 微调：connect 事件参数不变
│   └── RemoteDesktop.vue      # 改造：支持 v-show、自适应、全屏
```

### App.vue（重构）

- 管理 `sessions` 和 `activeTabId` 状态
- 水平 flex 布局：`TabBar` + 内容区
- 内容区中 `ConnectForm`（v-show="activeTabId === 'home'"）和多个 `RemoteDesktop`（v-show="activeTabId === session.id"，v-for 遍历 sessions）
- 处理连接/断开事件，维护 sessions 数组

### TabBar.vue（新增）

- Props: `sessions`, `activeTabId`
- Emits: `select(tabId)`, `disconnect(sessionId)`, `fullscreen(sessionId)`, `fitscreen(sessionId)`, `window-action(action)`
- 渲染主页图标 + 远程桌面图标列表
- 管理右键菜单显示/隐藏

### ContextMenu.vue（新增）

- Props: `items`（菜单项数组）, `x`, `y`（位置）, `visible`
- Emits: `select(action)`, `close`
- 通用菜单组件，支持分隔线、勾选状态、红色危险项
- 点击外部自动关闭（通过 `v-click-outside` 或 document click 监听）

### RemoteDesktop.vue（改造）

- 新增 prop: `fitMode`（是否自适应）
- 新增逻辑：自适应时监听容器 resize，计算缩放或发送 resize 命令
- `v-show` 控制显隐时，canvas 和 WebSocket 保持活跃

## 连接流程

1. 用户在 ConnectForm 点击"连接"
2. App.vue 接收 connect 事件，创建新 Session 对象，添加到 sessions 数组
3. activeTabId 切换到新 session.id
4. TabBar 显示新页签（显示器图标 + 连接状态点）
5. RemoteDesktop 组件挂载并连接 WebSocket
6. 连接成功后 session.status 更新为 'connected'

## 断开流程

1. 用户右键页签 → 点击"断开"
2. App.vue 调用对应 RemoteDesktop 的 disconnect 方法
3. 从 sessions 数组移除该 session
4. 如果断开的是当前激活页签，切换到主页
5. RemoteDesktop 组件通过 `v-if` 销毁（断开后不需要保留）

## 退出流程

1. 用户右键任意页签 → 点击"退出"
2. 遍历所有 sessions，逐个断开 WebSocket
3. 调用 `Quit()` 关闭应用

## 边界情况

- **远程主动断开**：WebSocket 收到断开事件时，更新 session.status 为 'disconnected'，页签状态点变灰/红，不自动移除页签，让用户决定
- **连接失败**：session.status 保持 'connecting'，超时后标记为 'disconnected'
- **全屏时断开**：自动退出全屏，切换到主页
- **最后一个会话断开**：自动切换到主页

# 多页签会话管理系统 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 QRDP 应用添加左侧竖排多页签系统，支持多 RDP 会话并行、右键菜单（断开/全屏/自适应/窗口控制）、全屏模式和自适应远程桌面。

**Architecture:** App.vue 重构为水平 flex 布局（TabBar + 内容区），新增 ContextMenu 和 TabBar 组件。多会话通过 `v-show` 保持 WebSocket 活跃，状态集中在 App.vue 的 `sessions` ref 中管理。窗口操作通过 Wails JS Runtime API 实现。

**Tech Stack:** Vue 3 (Composition API), TypeScript, Wails v2 JS Runtime

---

## 文件结构

| 文件 | 操作 | 职责 |
|------|------|------|
| `frontend/src/components/ContextMenu.vue` | 新建 | 通用右键菜单组件 |
| `frontend/src/components/TabBar.vue` | 新建 | 左侧竖排页签栏 |
| `frontend/src/App.vue` | 重写 | 布局容器 + 会话状态管理 |
| `frontend/src/views/ConnectForm.vue` | 小改 | 移除 emit 类型定义中的 width/height（改由 App.vue 解析） |
| `frontend/src/views/RemoteDesktop.vue` | 改造 | 支持自适应、expose disconnect 方法 |

---

### Task 1: 创建 ContextMenu 通用组件

**Files:**
- Create: `frontend/src/components/ContextMenu.vue`

- [ ] **Step 1: 创建 ContextMenu.vue**

```vue
<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="context-menu-overlay"
      @mousedown.self="emit('close')"
    >
      <div
        class="context-menu"
        :style="{ left: x + 'px', top: y + 'px' }"
        ref="menuRef"
      >
        <template v-for="(item, i) in items" :key="i">
          <div v-if="item.type === 'separator'" class="context-menu-sep" />
          <div
            v-else
            class="context-menu-item"
            :class="{ danger: item.danger }"
            @click="handleClick(item)"
          >
            <span class="context-menu-check">{{ item.checked ? '✓' : '' }}</span>
            <span>{{ item.label }}</span>
          </div>
        </template>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'

export interface MenuItem {
  label?: string
  action?: string
  type?: 'separator'
  danger?: boolean
  checked?: boolean
}

const props = defineProps<{
  visible: boolean
  x: number
  y: number
  items: MenuItem[]
}>()

const emit = defineEmits<{
  select: [action: string]
  close: []
}>()

const menuRef = ref<HTMLDivElement | null>(null)

// 确保菜单不超出视口
watch(() => props.visible, async (v) => {
  if (!v) return
  await nextTick()
  const el = menuRef.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  if (rect.right > window.innerWidth) {
    el.style.left = (window.innerWidth - rect.width - 4) + 'px'
  }
  if (rect.bottom > window.innerHeight) {
    el.style.top = (window.innerHeight - rect.height - 4) + 'px'
  }
})

function handleClick(item: MenuItem) {
  if (item.action) {
    emit('select', item.action)
  }
  emit('close')
}
</script>

<style scoped>
.context-menu-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 10000;
}

.context-menu {
  position: fixed;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
  padding: 4px;
  min-width: 160px;
  z-index: 10001;
}

.context-menu-sep {
  height: 1px;
  background: #e2e8f0;
  margin: 2px 8px;
}

.context-menu-item {
  padding: 7px 12px;
  border-radius: 4px;
  cursor: pointer;
  color: #334155;
  font-size: 13px;
  display: flex;
  align-items: center;
  gap: 8px;
  transition: background 0.1s;
}

.context-menu-item:hover {
  background: #f1f5f9;
}

.context-menu-item.danger {
  color: #ef4444;
}

.context-menu-item.danger:hover {
  background: #fef2f2;
}

.context-menu-check {
  width: 16px;
  text-align: center;
  font-size: 12px;
  flex-shrink: 0;
}
</style>
```

- [ ] **Step 2: 验证文件语法**

Run: `cd /Users/dxy/Documents/Code/github.com/dingxyang/go-freerdp-webconnect && npx vue-tsc --noEmit --pretty 2>&1 | head -20`
Expected: 无与 ContextMenu.vue 相关的错误

- [ ] **Step 3: 提交**

```bash
git add frontend/src/components/ContextMenu.vue
git commit -m "feat: add generic ContextMenu component"
```

---

### Task 2: 创建 TabBar 页签栏组件

**Files:**
- Create: `frontend/src/components/TabBar.vue`

- [ ] **Step 1: 创建 TabBar.vue**

```vue
<template>
  <div class="tab-bar">
    <!-- 主页页签 -->
    <div
      class="tab-item"
      :class="{ active: activeTabId === 'home' }"
      title="主页"
      @click="emit('select', 'home')"
      @contextmenu.prevent="onContextMenu('home', $event)"
    >
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>
        <polyline points="9 22 9 12 15 12 15 22"/>
      </svg>
    </div>

    <!-- 远程桌面页签 -->
    <div
      v-for="s in sessions"
      :key="s.id"
      class="tab-item"
      :class="{ active: activeTabId === s.id }"
      :title="s.host"
      @click="emit('select', s.id)"
      @contextmenu.prevent="onContextMenu(s.id, $event)"
    >
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <rect x="2" y="3" width="20" height="14" rx="2"/>
        <line x1="8" y1="21" x2="16" y2="21"/>
        <line x1="12" y1="17" x2="12" y2="21"/>
      </svg>
      <span
        class="status-dot"
        :class="s.status"
      />
    </div>

    <!-- 右键菜单 -->
    <ContextMenu
      :visible="menu.visible"
      :x="menu.x"
      :y="menu.y"
      :items="menuItems"
      @select="onMenuSelect"
      @close="menu.visible = false"
    />
  </div>
</template>

<script setup lang="ts">
import { reactive, computed } from 'vue'
import ContextMenu from './ContextMenu.vue'
import type { MenuItem } from './ContextMenu.vue'

export interface Session {
  id: string
  wsUrl: string
  host: string
  width: number
  height: number
  status: 'connecting' | 'connected' | 'disconnected'
}

const props = defineProps<{
  sessions: Session[]
  activeTabId: string
  isAlwaysOnTop: boolean
}>()

const emit = defineEmits<{
  select: [tabId: string]
  disconnect: [sessionId: string]
  fullscreen: [sessionId: string]
  fitscreen: [sessionId: string]
  'window-action': [action: string]
}>()

const menu = reactive({
  visible: false,
  x: 0,
  y: 0,
  targetId: '' as string,
})

const menuItems = computed<MenuItem[]>(() => {
  if (menu.targetId === 'home') {
    return [
      { label: '窗口置顶', action: 'toggle-on-top', checked: props.isAlwaysOnTop },
      { label: '窗口最大化', action: 'maximize' },
      { label: '窗口最小化', action: 'minimize' },
      { type: 'separator' },
      { label: '退出', action: 'quit', danger: true },
    ]
  }
  return [
    { label: '断开', action: 'disconnect' },
    { type: 'separator' },
    { label: '全屏远程桌面', action: 'fullscreen' },
    { label: '自适应远程桌面', action: 'fitscreen' },
    { type: 'separator' },
    { label: '窗口置顶', action: 'toggle-on-top', checked: props.isAlwaysOnTop },
    { label: '窗口最大化', action: 'maximize' },
    { label: '窗口最小化', action: 'minimize' },
    { type: 'separator' },
    { label: '退出', action: 'quit', danger: true },
  ]
})

function onContextMenu(tabId: string, e: MouseEvent) {
  menu.targetId = tabId
  menu.x = e.clientX
  menu.y = e.clientY
  menu.visible = true
}

function onMenuSelect(action: string) {
  const targetId = menu.targetId
  switch (action) {
    case 'disconnect':
      emit('disconnect', targetId)
      break
    case 'fullscreen':
      emit('fullscreen', targetId)
      break
    case 'fitscreen':
      emit('fitscreen', targetId)
      break
    case 'toggle-on-top':
    case 'maximize':
    case 'minimize':
    case 'quit':
      emit('window-action', action)
      break
  }
}
</script>

<style scoped>
.tab-bar {
  width: 48px;
  background: #1e293b;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 8px 0;
  gap: 2px;
  flex-shrink: 0;
  user-select: none;
  --wails-draggable: no-drag;
}

.tab-item {
  width: 40px;
  height: 40px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: #94a3b8;
  background: #334155;
  position: relative;
  transition: background 0.15s, color 0.15s;
}

.tab-item:first-child {
  margin-bottom: 4px;
}

.tab-item:hover {
  background: #475569;
  color: #e2e8f0;
}

.tab-item.active {
  background: #3b82f6;
  color: #ffffff;
}

.status-dot {
  position: absolute;
  top: 3px;
  right: 3px;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #94a3b8;
}

.status-dot.connecting {
  background: #f59e0b;
}

.status-dot.connected {
  background: #22c55e;
}

.status-dot.disconnected {
  background: #94a3b8;
}
</style>
```

- [ ] **Step 2: 验证文件语法**

Run: `cd /Users/dxy/Documents/Code/github.com/dingxyang/go-freerdp-webconnect && npx vue-tsc --noEmit --pretty 2>&1 | head -20`
Expected: 无与 TabBar.vue 相关的错误

- [ ] **Step 3: 提交**

```bash
git add frontend/src/components/TabBar.vue
git commit -m "feat: add TabBar sidebar component with context menu"
```

---

### Task 3: 改造 RemoteDesktop.vue

**Files:**
- Modify: `frontend/src/views/RemoteDesktop.vue`

- [ ] **Step 1: 重写 RemoteDesktop.vue 支持自适应和 expose**

将现有的 RemoteDesktop.vue 完整替换为以下内容：

```vue
<template>
  <div class="rdp-desktop" ref="containerRef">
    <div class="canvas-wrapper" :class="{ fitting: fitMode }">
      <canvas
        ref="canvasRef"
        :width="width"
        :height="height"
        :style="canvasStyle"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, onActivated, onDeactivated, watch } from 'vue'
import { RDPClient } from '../rdp'

const props = defineProps<{
  wsUrl: string
  width: number
  height: number
  host: string
  fitMode: boolean
}>()

const emit = defineEmits<{
  disconnected: []
  statusChange: [status: 'connecting' | 'connected' | 'disconnected']
}>()

const canvasRef = ref<HTMLCanvasElement | null>(null)
const containerRef = ref<HTMLDivElement | null>(null)
const client = ref<RDPClient | null>(null)
const containerWidth = ref(0)
const containerHeight = ref(0)

const canvasStyle = computed(() => {
  if (!props.fitMode) return {}
  if (containerWidth.value === 0 || containerHeight.value === 0) return {}
  const scaleX = containerWidth.value / props.width
  const scaleY = containerHeight.value / props.height
  const scale = Math.min(scaleX, scaleY, 1) // 不放大，只缩小
  return {
    transform: `scale(${scale})`,
    transformOrigin: 'top left',
  }
})

let resizeObserver: ResizeObserver | null = null

function updateContainerSize() {
  if (!containerRef.value) return
  const rect = containerRef.value.getBoundingClientRect()
  containerWidth.value = rect.width
  containerHeight.value = rect.height
}

function tryResize() {
  if (!props.fitMode || !client.value) return
  // 先尝试通过 DISP 通道调整远程分辨率
  if (containerWidth.value > 0 && containerHeight.value > 0) {
    const w = Math.floor(containerWidth.value)
    const h = Math.floor(containerHeight.value)
    // 发送 resize 命令到远程；如果远程不支持 DISP，画布会通过 CSS scale 回退
    client.value.resize(w, h)
  }
}

watch(() => props.fitMode, (fit) => {
  if (fit) {
    updateContainerSize()
    tryResize()
  }
})

client.value = new RDPClient()

client.value.on('connected', () => {
  emit('statusChange', 'connected')
})

client.value.on('disconnected', () => {
  emit('statusChange', 'disconnected')
  emit('disconnected')
})

client.value.on('error', (msg) => {
  console.error('RDP Error:', msg)
  emit('statusChange', 'disconnected')
  emit('disconnected')
})

onMounted(() => {
  if (canvasRef.value && client.value) {
    emit('statusChange', 'connecting')
    client.value.connect(props.wsUrl, canvasRef.value)
  }

  resizeObserver = new ResizeObserver(() => {
    updateContainerSize()
    if (props.fitMode) {
      tryResize()
    }
  })
  if (containerRef.value) {
    resizeObserver.observe(containerRef.value)
  }
})

onUnmounted(() => {
  resizeObserver?.disconnect()
  client.value?.disconnect()
})

function doDisconnect() {
  client.value?.disconnect()
}

defineExpose({ disconnect: doDisconnect })
</script>

<style scoped>
.rdp-desktop {
  width: 100%;
  height: 100%;
  overflow: auto;
  display: flex;
  justify-content: center;
  align-items: flex-start;
  background: #0f172a;
}

.canvas-wrapper {
  padding: 0;
  display: flex;
  justify-content: center;
  align-items: flex-start;
}

.canvas-wrapper.fitting {
  width: 100%;
  height: 100%;
  overflow: hidden;
}

canvas {
  display: block;
  cursor: default;
}
</style>
```

- [ ] **Step 2: 验证语法**

Run: `cd /Users/dxy/Documents/Code/github.com/dingxyang/go-freerdp-webconnect && npx vue-tsc --noEmit --pretty 2>&1 | head -20`
Expected: 无错误

- [ ] **Step 3: 提交**

```bash
git add frontend/src/views/RemoteDesktop.vue
git commit -m "feat: refactor RemoteDesktop to support fit-mode and expose disconnect"
```

---

### Task 4: 重写 App.vue 为多页签布局容器

**Files:**
- Modify: `frontend/src/App.vue`

- [ ] **Step 1: 重写 App.vue**

将现有 App.vue 完整替换为以下内容：

```vue
<template>
  <div class="app-root" :class="{ fullscreen: isFullscreen }">
    <TabBar
      v-show="!isFullscreen"
      :sessions="sessions"
      :active-tab-id="activeTabId"
      :is-always-on-top="isAlwaysOnTop"
      @select="activeTabId = $event"
      @disconnect="handleDisconnect"
      @fullscreen="handleFullscreen"
      @fitscreen="handleFitscreen"
      @window-action="handleWindowAction"
    />
    <div class="content-area">
      <ConnectForm
        v-show="activeTabId === 'home'"
        @connect="onConnect"
      />
      <RemoteDesktop
        v-for="s in sessions"
        :key="s.id"
        v-show="activeTabId === s.id"
        :ref="(el: any) => setSessionRef(s.id, el)"
        :ws-url="s.wsUrl"
        :width="s.width"
        :height="s.height"
        :host="s.host"
        :fit-mode="s.fitMode"
        @disconnected="onSessionDisconnected(s.id)"
        @status-change="onStatusChange(s.id, $event)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onUnmounted } from 'vue'
import {
  WindowSetAlwaysOnTop,
  WindowMaximise,
  WindowMinimise,
  WindowFullscreen,
  WindowUnfullscreen,
  Quit,
} from './wailsjs/runtime/runtime'
import TabBar from './components/TabBar.vue'
import type { Session } from './components/TabBar.vue'
import ConnectForm from './views/ConnectForm.vue'
import RemoteDesktop from './views/RemoteDesktop.vue'

interface SessionState extends Session {
  fitMode: boolean
}

const sessions = ref<SessionState[]>([])
const activeTabId = ref<string>('home')
const isFullscreen = ref(false)
const isAlwaysOnTop = ref(false)

// 存储 RemoteDesktop 组件引用
const sessionRefs = new Map<string, InstanceType<typeof RemoteDesktop>>()

function setSessionRef(id: string, el: any) {
  if (el) {
    sessionRefs.set(id, el)
  } else {
    sessionRefs.delete(id)
  }
}

function onConnect(wsUrl: string, width: number, height: number) {
  // 从 URL 提取 host 参数用于标题显示
  let host = '远程桌面'
  try {
    const u = new URL(wsUrl)
    host = u.searchParams.get('host') || '远程桌面'
  } catch {
    // ignore
  }

  const session: SessionState = {
    id: Date.now().toString(36) + Math.random().toString(36).slice(2, 6),
    wsUrl,
    host,
    width,
    height,
    status: 'connecting',
    fitMode: false,
  }

  sessions.value.push(session)
  activeTabId.value = session.id
}

function onStatusChange(sessionId: string, status: 'connecting' | 'connected' | 'disconnected') {
  const s = sessions.value.find(s => s.id === sessionId)
  if (s) s.status = status
}

function onSessionDisconnected(sessionId: string) {
  // 远程主动断开时，更新状态但不自动移除
  const s = sessions.value.find(s => s.id === sessionId)
  if (s) s.status = 'disconnected'

  // 如果正在全屏且断开的是当前会话，退出全屏
  if (isFullscreen.value && activeTabId.value === sessionId) {
    exitFullscreen()
  }
}

function handleDisconnect(sessionId: string) {
  const rdp = sessionRefs.get(sessionId)
  if (rdp) {
    rdp.disconnect()
  }
  sessions.value = sessions.value.filter(s => s.id !== sessionId)
  sessionRefs.delete(sessionId)

  if (activeTabId.value === sessionId) {
    activeTabId.value = 'home'
  }

  if (isFullscreen.value) {
    exitFullscreen()
  }
}

function handleFullscreen(sessionId: string) {
  activeTabId.value = sessionId
  isFullscreen.value = true
  WindowFullscreen()
}

function exitFullscreen() {
  isFullscreen.value = false
  WindowUnfullscreen()
}

function handleFitscreen(sessionId: string) {
  const s = sessions.value.find(s => s.id === sessionId)
  if (s) {
    s.fitMode = !s.fitMode
  }
}

function handleWindowAction(action: string) {
  switch (action) {
    case 'toggle-on-top':
      isAlwaysOnTop.value = !isAlwaysOnTop.value
      WindowSetAlwaysOnTop(isAlwaysOnTop.value)
      break
    case 'maximize':
      WindowMaximise()
      break
    case 'minimize':
      WindowMinimise()
      break
    case 'quit':
      // 断开所有会话
      for (const s of sessions.value) {
        const rdp = sessionRefs.get(s.id)
        if (rdp) rdp.disconnect()
      }
      Quit()
      break
  }
}

// 全屏时按 Esc 退出
function onKeyDown(e: KeyboardEvent) {
  if (e.key === 'Escape' && isFullscreen.value) {
    exitFullscreen()
  }
}

document.addEventListener('keydown', onKeyDown)
onUnmounted(() => {
  document.removeEventListener('keydown', onKeyDown)
})
</script>

<style scoped>
.app-root {
  display: flex;
  width: 100vw;
  height: 100vh;
  overflow: hidden;
  background: #f3f4f6;
}

.content-area {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.app-root.fullscreen .content-area {
  width: 100vw;
}
</style>
```

- [ ] **Step 2: 验证语法**

Run: `cd /Users/dxy/Documents/Code/github.com/dingxyang/go-freerdp-webconnect && npx vue-tsc --noEmit --pretty 2>&1 | head -20`
Expected: 无错误

- [ ] **Step 3: 提交**

```bash
git add frontend/src/App.vue
git commit -m "feat: rewrite App.vue as multi-tab session layout"
```

---

### Task 5: 调整 ConnectForm.vue 适配新布局

**Files:**
- Modify: `frontend/src/views/ConnectForm.vue`

ConnectForm 目前通过 `emit('connect', wsUrl, w, h)` 发射连接事件。App.vue 中的新 `onConnect` 方法签名与此匹配，所以 ConnectForm 不需要改 emit 逻辑。但需要确保 ConnectForm 的根 `.app-layout` 在新布局中正确填满内容区。

- [ ] **Step 1: 确保 ConnectForm 填满内容区**

在 `ConnectForm.vue` 的 `<style scoped>` 中，`.app-layout` 当前设置了 `width: 100vw; height: 100vh;`。这在新的多页签布局中需要改为 `width: 100%; height: 100%;`，因为它不再是全屏根组件，而是内容区的子元素。

在 `frontend/src/views/ConnectForm.vue` 中做如下替换：

```
旧:
.app-layout {
  display: flex;
  flex-direction: column;
  width: 100vw;
  height: 100vh;
  overflow: hidden;
  background: #f3f4f6;
}

新:
.app-layout {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: #f3f4f6;
}
```

- [ ] **Step 2: 调整响应式断点中 form-panel 的 top 值**

`.form-panel` 的 `@media (max-width: 799px)` 中 `top: 86px` 是基于旧布局计算的（title-bar 44px + toolbar 42px = 86px）。在新布局中 ConnectForm 内部结构不变，但它不再从视口顶部开始，所以用 `top: 0` 相对于 `.app-layout` 会更正确。不过因为 form-panel 用的是 `position: fixed`，改为 `position: absolute` + 相对定位父元素更好：

```
旧:
@media (max-width: 799px) {
  .form-panel {
    position: fixed;
    right: 0;
    top: 86px;
    bottom: 28px;
    width: 360px;
    z-index: 100;
    box-shadow: -4px 0 16px rgba(0, 0, 0, 0.1);
  }

新:
@media (max-width: 799px) {
  .form-panel {
    position: absolute;
    right: 0;
    top: 86px;
    bottom: 28px;
    width: 360px;
    z-index: 100;
    box-shadow: -4px 0 16px rgba(0, 0, 0, 0.1);
  }
```

同时在 `.app-layout` 中添加 `position: relative;`：

```
旧:
.app-layout {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: #f3f4f6;
}

新:
.app-layout {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: #f3f4f6;
  position: relative;
}
```

- [ ] **Step 3: 验证语法和构建**

Run: `cd /Users/dxy/Documents/Code/github.com/dingxyang/go-freerdp-webconnect && npx vue-tsc --noEmit --pretty 2>&1 | head -20`
Expected: 无错误

- [ ] **Step 4: 提交**

```bash
git add frontend/src/views/ConnectForm.vue
git commit -m "fix: adapt ConnectForm layout for multi-tab container"
```

---

### Task 6: 前端构建验证

**Files:** 无新文件

- [ ] **Step 1: 运行前端构建**

Run: `cd /Users/dxy/Documents/Code/github.com/dingxyang/go-freerdp-webconnect/frontend && npm run build 2>&1`
Expected: 构建成功，无错误

- [ ] **Step 2: 检查类型错误**

Run: `cd /Users/dxy/Documents/Code/github.com/dingxyang/go-freerdp-webconnect && npx vue-tsc --noEmit --pretty 2>&1`
Expected: 无类型错误

- [ ] **Step 3: 如有错误，修复后重新构建并提交**

```bash
git add -A
git commit -m "fix: resolve build errors for multi-tab feature"
```

---

### Task 7: 集成测试和修复

**Files:** 可能涉及所有新/改动文件

这个任务是手动验证整体功能。由于项目没有自动化测试框架，需要通过构建验证和代码审查确保正确性。

- [ ] **Step 1: 检查所有 import 路径和类型引用**

手动检查：
1. `App.vue` 导入 `TabBar`, `ConnectForm`, `RemoteDesktop` 路径正确
2. `App.vue` 导入 `Session` 类型从 `TabBar.vue` 正确
3. `TabBar.vue` 导入 `ContextMenu` 和 `MenuItem` 类型正确
4. Wails Runtime API 导入路径 `./wailsjs/runtime/runtime` 正确

- [ ] **Step 2: 检查事件链路完整性**

验证事件传递链：
1. `ConnectForm` emit `connect(wsUrl, w, h)` → `App.vue` `onConnect()` 创建 session
2. `RemoteDesktop` emit `disconnected` → `App.vue` `onSessionDisconnected()` 更新状态
3. `RemoteDesktop` emit `statusChange` → `App.vue` `onStatusChange()` 更新 session.status
4. `TabBar` emit `select` → `App.vue` 切换 `activeTabId`
5. `TabBar` emit `disconnect` → `App.vue` `handleDisconnect()` 断开并移除
6. `TabBar` emit `fullscreen` → `App.vue` `handleFullscreen()` 全屏
7. `TabBar` emit `fitscreen` → `App.vue` `handleFitscreen()` 切换自适应
8. `TabBar` emit `window-action` → `App.vue` `handleWindowAction()` 窗口操作

- [ ] **Step 3: 最终构建验证**

Run: `cd /Users/dxy/Documents/Code/github.com/dingxyang/go-freerdp-webconnect/frontend && npm run build 2>&1`
Expected: 构建成功

- [ ] **Step 4: 如有问题，修复并提交**

```bash
git add -A
git commit -m "fix: integration fixes for multi-tab session system"
```

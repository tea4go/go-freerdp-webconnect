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
      class="tab-item session-tab"
      :class="{ active: activeTabId === s.id }"
      :title="s.name + ' (' + s.host + ')'"
      @click="emit('select', s.id)"
      @contextmenu.prevent="onContextMenu(s.id, $event)"
    >
      <svg class="tab-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <rect x="2" y="3" width="20" height="14" rx="2"/>
        <line x1="8" y1="21" x2="16" y2="21"/>
        <line x1="12" y1="17" x2="12" y2="21"/>
      </svg>
      <span class="tab-label">{{ s.name }}</span>
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
  name: string
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
  overflow-y: auto;
  overflow-x: hidden;
}

.tab-bar::-webkit-scrollbar {
  width: 0;
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
  flex-shrink: 0;
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

.session-tab {
  width: 40px;
  height: 40px;
  flex-direction: column;
  gap: 1px;
  padding: 4px 2px;
}

.tab-icon {
  flex-shrink: 0;
}

.tab-label {
  font-size: 8px;
  line-height: 1.1;
  max-width: 36px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: center;
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

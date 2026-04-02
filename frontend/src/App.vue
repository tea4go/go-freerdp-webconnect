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
  const s = sessions.value.find(s => s.id === sessionId)
  if (s) s.status = 'disconnected'

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

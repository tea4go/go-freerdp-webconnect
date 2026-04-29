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

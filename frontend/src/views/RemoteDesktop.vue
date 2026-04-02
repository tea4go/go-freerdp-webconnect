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
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
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
  if (containerWidth.value > 0 && containerHeight.value > 0) {
    const w = Math.floor(containerWidth.value)
    const h = Math.floor(containerHeight.value)
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

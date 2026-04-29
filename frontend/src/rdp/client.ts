// RDP WebSocket 客户端
// 管理 WebSocket 连接，分派消息到渲染器，发送输入事件

import { RDPRenderer } from './renderer'
import {
  encodeMouseEvent, encodeKeyEvent, encodeCharEvent, encodeResizeEvent,
  MOD_KEYS, PTR_FLAGS,
} from './input'

export type RDPClientEvents = {
  connected: () => void
  disconnected: () => void
  error: (msg: string) => void
}

export class RDPClient {
  private ws: WebSocket | null = null
  private renderer: RDPRenderer | null = null
  private canvas: HTMLCanvasElement | null = null
  private listeners: Partial<RDPClientEvents> = {}
  private boundHandlers: Record<string, EventListener> = {}

  on<K extends keyof RDPClientEvents>(event: K, handler: RDPClientEvents[K]) {
    this.listeners[event] = handler
  }

  connect(wsUrl: string, canvas: HTMLCanvasElement) {
    this.canvas = canvas
    this.renderer = new RDPRenderer(canvas)

    try {
      this.ws = new WebSocket(wsUrl)
    } catch {
      this.listeners.error?.('WebSocket 连接失败')
      return
    }

    this.ws.binaryType = 'arraybuffer'

    this.ws.onopen = () => {
      this.bindInputEvents()
      this.listeners.connected?.()
    }

    this.ws.onclose = () => {
      this.unbindInputEvents()
      this.renderer?.clear()
      this.listeners.disconnected?.()
    }

    this.ws.onerror = () => {
      this.listeners.error?.('WebSocket 连接失败')
      this.unbindInputEvents()
    }

    this.ws.onmessage = (evt: MessageEvent) => {
      if (evt.data instanceof ArrayBuffer) {
        this.renderer?.handleMessage(evt.data)
      }
    }
  }

  disconnect() {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.close()
    }
    this.unbindInputEvents()
  }

  resize(width: number, height: number) {
    this.renderer?.resize(width, height)
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(encodeResizeEvent(width, height))
    }
  }

  private send(data: ArrayBuffer) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(data)
    }
  }

  private bindInputEvents() {
    if (!this.canvas) return

    const onMouseMove = (e: MouseEvent) => {
      e.preventDefault()
      this.send(encodeMouseEvent(PTR_FLAGS.MOVE, e.offsetX, e.offsetY))
    }

    const onMouseDown = (e: MouseEvent) => {
      e.preventDefault()
      const btn = this.getButton(e)
      this.send(encodeMouseEvent(PTR_FLAGS.DOWN | btn, e.offsetX, e.offsetY))
    }

    const onMouseUp = (e: MouseEvent) => {
      e.preventDefault()
      const btn = this.getButton(e)
      this.send(encodeMouseEvent(btn, e.offsetX, e.offsetY))
    }

    const onWheel = (e: WheelEvent) => {
      e.preventDefault()
      const flags = PTR_FLAGS.WHEEL | (e.deltaY < 0 ? PTR_FLAGS.WHEEL_UP : PTR_FLAGS.WHEEL_DOWN)
      this.send(encodeMouseEvent(flags, 0, 0))
    }

    const onContextMenu = (e: Event) => e.preventDefault()

    const onKeyDown = (e: KeyboardEvent) => {
      if (MOD_KEYS.includes(e.keyCode)) {
        e.preventDefault()
        this.send(encodeKeyEvent(true, e.keyCode))
      }
    }

    const onKeyUp = (e: KeyboardEvent) => {
      if (MOD_KEYS.includes(e.keyCode)) {
        e.preventDefault()
        this.send(encodeKeyEvent(false, e.keyCode))
      }
    }

    const onKeyPress = (e: KeyboardEvent) => {
      e.preventDefault()
      if (MOD_KEYS.includes(e.keyCode)) return
      const mods = (e.shiftKey ? 1 : 0) | (e.ctrlKey ? 2 : 0) | (e.altKey ? 4 : 0) | (e.metaKey ? 8 : 0)
      this.send(encodeCharEvent(mods, e.charCode || e.keyCode))
    }

    this.canvas.addEventListener('mousemove', onMouseMove)
    this.canvas.addEventListener('mousedown', onMouseDown)
    this.canvas.addEventListener('mouseup', onMouseUp)
    this.canvas.addEventListener('wheel', onWheel, { passive: false })
    this.canvas.addEventListener('contextmenu', onContextMenu)
    document.addEventListener('keydown', onKeyDown)
    document.addEventListener('keyup', onKeyUp)
    document.addEventListener('keypress', onKeyPress)

    this.boundHandlers = {
      mousemove: onMouseMove as EventListener,
      mousedown: onMouseDown as EventListener,
      mouseup: onMouseUp as EventListener,
      wheel: onWheel as EventListener,
      contextmenu: onContextMenu,
      keydown: onKeyDown as EventListener,
      keyup: onKeyUp as EventListener,
      keypress: onKeyPress as EventListener,
    }
  }

  private unbindInputEvents() {
    if (this.canvas) {
      for (const [evt, fn] of Object.entries(this.boundHandlers)) {
        if (['keydown', 'keyup', 'keypress'].includes(evt)) {
          document.removeEventListener(evt, fn)
        } else {
          this.canvas.removeEventListener(evt, fn)
        }
      }
    }
    this.boundHandlers = {}
  }

  private getButton(e: MouseEvent): number {
    switch (e.button) {
      case 0: return PTR_FLAGS.BUTTON1
      case 1: return PTR_FLAGS.BUTTON3
      case 2: return PTR_FLAGS.BUTTON2
      default: return PTR_FLAGS.BUTTON1
    }
  }
}

// 鼠标/键盘/触摸输入事件编码
// 将用户输入编码为 RDP 二进制消息，通过 WebSocket 发送

import { INPUT_OP } from './protocol'

/** 编码鼠标事件为 16 字节 ArrayBuffer */
export function encodeMouseEvent(flags: number, x: number, y: number): ArrayBuffer {
  const buf = new ArrayBuffer(16)
  const a = new Uint32Array(buf)
  a[0] = INPUT_OP.MOUSE
  a[1] = flags
  a[2] = x
  a[3] = y
  return buf
}

/** 编码修饰键按下/抬起事件为 12 字节 ArrayBuffer */
export function encodeKeyEvent(down: boolean, keycode: number): ArrayBuffer {
  const buf = new ArrayBuffer(12)
  const a = new Uint32Array(buf)
  a[0] = INPUT_OP.KEY_UPDOWN
  a[1] = down ? 1 : 0
  a[2] = keycode
  return buf
}

/** 编码 Unicode 字符输入事件为 12 字节 ArrayBuffer */
export function encodeCharEvent(modifiers: number, charcode: number): ArrayBuffer {
  const buf = new ArrayBuffer(12)
  const a = new Uint32Array(buf)
  a[0] = INPUT_OP.KEY_CHAR
  a[1] = modifiers
  a[2] = charcode
  return buf
}

/** 编码窗口大小调整事件为 12 字节 ArrayBuffer */
export function encodeResizeEvent(width: number, height: number): ArrayBuffer {
  const buf = new ArrayBuffer(12)
  const a = new Uint32Array(buf)
  a[0] = INPUT_OP.RESIZE
  a[1] = width
  a[2] = height
  return buf
}

/** 需要特殊处理的修饰键键码列表 */
export const MOD_KEYS = [8, 16, 17, 18, 20, 144, 145]

/** 鼠标标志位 */
export const PTR_FLAGS = {
  MOVE: 0x0800,
  DOWN: 0x8000,
  BUTTON1: 0x1000, // 左键
  BUTTON2: 0x2000, // 右键
  BUTTON3: 0x4000, // 中键
  WHEEL: 0x0200,
  WHEEL_UP: 0x0087,
  WHEEL_DOWN: 0x0188,
} as const

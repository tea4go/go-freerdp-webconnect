// Canvas 渲染引擎
// 将 RDP 二进制帧渲染到 Canvas

import { WSOP, ROP3 } from './protocol'
import { dRLE16_RGBA, dRGB162RGBA, flipV } from './codec'

export class RDPRenderer {
  private canvas: HTMLCanvasElement
  private ctx: CanvasRenderingContext2D
  private bstore: HTMLCanvasElement
  private bctx: CanvasRenderingContext2D
  private saveCount = 0
  // 裁剪区域
  private clx = 0
  private cly = 0
  private clw = 0
  private clh = 0

  constructor(canvas: HTMLCanvasElement) {
    this.canvas = canvas
    this.ctx = canvas.getContext('2d')!
    this.ctx.strokeStyle = 'rgba(255,255,255,0)'
    this.ctx.fillStyle = 'rgba(255,255,255,0)'

    this.bstore = document.createElement('canvas')
    this.bstore.width = canvas.width
    this.bstore.height = canvas.height
    this.bctx = this.bstore.getContext('2d')!
  }

  /** 调整 Canvas 和离屏缓冲尺寸 */
  resize(width: number, height: number) {
    this.canvas.width = width
    this.canvas.height = height
    this.bstore.width = width
    this.bstore.height = height
  }

  /** 清空画布 */
  clear() {
    while (this.saveCount > 0) {
      this.ctx.restore()
      this.saveCount--
    }
    this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height)
    this.clx = this.cly = this.clw = this.clh = 0
  }

  /** 处理一条 RDP 二进制消息 */
  handleMessage(data: ArrayBuffer) {
    const op = new Uint32Array(data, 0, 1)[0]

    switch (op) {
      case WSOP.BEGINPAINT:
        this.ctx.save()
        this.saveCount++
        break

      case WSOP.ENDPAINT:
        this.ctx.restore()
        this.saveCount--
        break

      case WSOP.BITMAP:
        this.handleBitmap(data)
        break

      case WSOP.OPAQUERECT:
        this.handleOpaqueRect(data)
        break

      case WSOP.SETBOUNDS:
        this.handleSetBounds(data)
        break

      case WSOP.PATBLT:
        this.handlePatBlt(data)
        break

      case WSOP.MULTI_OPAQUERECT:
        this.handleMultiOpaqueRect(data)
        break

      case WSOP.SCRBLT:
        this.handleScrBlt(data)
        break

      default:
        // 忽略光标相关消息（PTR_NEW/PTR_FREE/PTR_SET 等）
        break
    }
  }

  private checkClip(x: number, y: number): boolean {
    if (this.clw || this.clh) {
      return x >= this.clx && x <= this.clx + this.clw &&
             y >= this.cly && y <= this.cly + this.clh
    }
    return true
  }

  private setClipRect(x: number, y: number, w: number, h: number, save: boolean) {
    if (save) {
      this.clx = x
      this.cly = y
      this.clw = w
      this.clh = h
    }
    this.ctx.beginPath()
    this.ctx.rect(0, 0, this.canvas.width, this.canvas.height)
    this.ctx.clip()
    if (x === 0 && y === 0 && (w === 0 && h === 0 || w === this.canvas.width && h === this.canvas.height)) {
      return
    }
    this.ctx.beginPath()
    this.ctx.rect(x, y, w, h)
    this.ctx.clip()
  }

  private setROP(rop: number): boolean {
    switch (rop) {
      case ROP3.PATINVERT:
        this.ctx.globalCompositeOperation = 'xor'
        return true
      case ROP3.PATCOPY:
        this.ctx.globalCompositeOperation = 'copy'
        return true
      case ROP3.SRCCOPY:
        this.ctx.globalCompositeOperation = 'source-over'
        return true
    }
    return false
  }

  private c2s(rgba: Uint8Array): string {
    return `rgba(${rgba[0]},${rgba[1]},${rgba[2]},${rgba[3] / 255})`
  }

  private handleBitmap(data: ArrayBuffer) {
    const hdr = new Uint32Array(data, 4, 9)
    const bmdata = new Uint8Array(data, 40)
    const x = hdr[0], y = hdr[1], w = hdr[2], h = hdr[3]
    const dw = hdr[4], dh = hdr[5], bpp = hdr[6]
    const compressed = hdr[7] !== 0
    const len = hdr[8]

    if (bpp !== 16 && bpp !== 15) return

    const inClip = this.checkClip(x, y) && this.checkClip(x + dw, y + dh)
    const targetCtx = inClip ? this.ctx : this.bctx
    const outB = targetCtx.createImageData(w, h)

    if (compressed) {
      dRLE16_RGBA(bmdata, len, w, outB.data)
      flipV(outB.data, w, h)
    } else {
      dRGB162RGBA(bmdata, len, outB.data)
    }

    if (inClip) {
      this.ctx.putImageData(outB, x, y, 0, 0, dw, dh)
    } else {
      this.bctx.putImageData(outB, 0, 0, 0, 0, dw, dh)
      this.ctx.drawImage(this.bstore, 0, 0, dw, dh, x, y, dw, dh)
    }
  }

  private handleOpaqueRect(data: ArrayBuffer) {
    const hdr = new Int32Array(data, 4, 4)
    const rgba = new Uint8Array(data, 20, 4)
    this.ctx.fillStyle = this.c2s(rgba)
    this.ctx.fillRect(hdr[0], hdr[1], hdr[2], hdr[3])
  }

  private handleSetBounds(data: ArrayBuffer) {
    const hdr = new Int32Array(data, 4, 4)
    this.setClipRect(hdr[0], hdr[1], hdr[2] - hdr[0], hdr[3] - hdr[1], true)
  }

  private handlePatBlt(data: ArrayBuffer) {
    if (data.byteLength !== 28) return
    const hdr = new Int32Array(data, 4, 4)
    const rgba = new Uint8Array(data, 20, 4)
    const rop = new Uint32Array(data, 24, 1)[0]

    this.ctx.save()
    this.saveCount++
    this.setClipRect(hdr[0], hdr[1], hdr[2], hdr[3], false)
    if (this.setROP(rop)) {
      this.ctx.fillStyle = this.c2s(rgba)
      this.ctx.fillRect(hdr[0], hdr[1], hdr[2], hdr[3])
    }
    this.ctx.restore()
    this.saveCount--
  }

  private handleMultiOpaqueRect(data: ArrayBuffer) {
    const rgba = new Uint8Array(data, 4, 4)
    const count = new Uint32Array(data, 8, 1)[0]
    const rects = new Uint32Array(data, 12, count * 4)
    this.ctx.fillStyle = this.c2s(rgba)
    for (let i = 0, offs = 0; i < count; i++, offs += 4) {
      this.ctx.fillRect(rects[offs], rects[offs + 1], rects[offs + 2], rects[offs + 3])
    }
  }

  private handleScrBlt(data: ArrayBuffer) {
    const hdr = new Int32Array(data, 8, 6)
    const rop = new Uint32Array(data, 4, 1)[0]
    const x = hdr[0], y = hdr[1], w = hdr[2], h = hdr[3]
    const sx = hdr[4], sy = hdr[5]
    if (w <= 0 || h <= 0) return
    if (!this.setROP(rop)) return

    if (this.checkClip(x, y) && this.checkClip(x + w, y + h)) {
      this.ctx.putImageData(this.ctx.getImageData(sx, sy, w, h), x, y)
    } else {
      this.bctx.putImageData(this.ctx.getImageData(sx, sy, w, h), 0, 0)
      this.ctx.drawImage(this.bstore, 0, 0, w, h, x, y, w, h)
    }
  }
}

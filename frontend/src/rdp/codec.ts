// RDP RLE16 位图编解码器
// 从 wsgate-debug.js 移植，将 RGB565 / RLE16 数据解码为 RGBA

/** 将 RGB565 像素值解码为 RGBA 写入目标数组 */
export function pel2RGBA(pel: number, outA: Uint8ClampedArray | Uint8Array, outI: number) {
  let pelR = (pel & 0xf800) >> 11
  let pelG = (pel & 0x07e0) >> 5
  let pelB = pel & 0x001f
  pelR = ((pelR << 3) & ~0x7) | (pelR >> 2)
  pelG = ((pelG << 2) & ~0x3) | (pelG >> 4)
  pelB = ((pelB << 3) & ~0x7) | (pelB >> 2)
  outA[outI] = pelR
  outA[outI + 1] = pelG
  outA[outI + 2] = pelB
  outA[outI + 3] = 255
}

/** 从缓冲区读取 RGB565 小端像素并解码为 RGBA */
export function buf2RGBA(inA: Uint8Array, inI: number, outA: Uint8ClampedArray | Uint8Array, outI: number) {
  pel2RGBA(inA[inI] | (inA[inI + 1] << 8), outA, outI)
}

/** 将无压缩 RGB565 数据解码为 RGBA */
export function dRGB162RGBA(inA: Uint8Array, inLength: number, outA: Uint8ClampedArray | Uint8Array) {
  let inI = 0, outI = 0
  while (inI < inLength) {
    buf2RGBA(inA, inI, outA, outI)
    inI += 2
    outI += 4
  }
}

/** RGBA 像素数据垂直翻转 */
export function flipV(inA: Uint8ClampedArray | Uint8Array, width: number, height: number) {
  const sll = width * 4
  const half = Math.floor(height / 2)
  let lbot = sll * (height - 1)
  let ltop = 0
  const tmp = new Uint8Array(sll)
  for (let i = 0; i < half; i++) {
    tmp.set(inA.subarray(ltop, ltop + sll))
    inA.set(inA.subarray(lbot, lbot + sll), ltop)
    inA.set(tmp, lbot)
    ltop += sll
    lbot -= sll
  }
}

function copyRGBA(inA: Uint8ClampedArray | Uint8Array, inI: number, outA: Uint8ClampedArray | Uint8Array, outI: number) {
  outA.set(inA.subarray(inI, inI + 4), outI)
}

function xorbufRGBAPel16(inA: Uint8ClampedArray | Uint8Array, inI: number, outA: Uint8ClampedArray | Uint8Array, outI: number, pel: number) {
  let pelR = (pel & 0xf800) >> 11
  let pelG = (pel & 0x07e0) >> 5
  let pelB = pel & 0x001f
  pelR = ((pelR << 3) & ~0x7) | (pelR >> 2)
  pelG = ((pelG << 2) & ~0x3) | (pelG >> 4)
  pelB = ((pelB << 3) & ~0x7) | (pelB >> 2)
  outA[outI] = inA[inI] ^ pelR
  outA[outI + 1] = inA[inI + 1] ^ pelG
  outA[outI + 2] = inA[inI + 2] ^ pelB
  outA[outI + 3] = 255
}

function writeFgBgImage(outA: Uint8ClampedArray | Uint8Array, outI: number, rowDelta: number, bitmask: number, fgPel: number, cBits: number): number {
  let cmpMask = 0x01
  while (cBits-- > 0) {
    if (bitmask & cmpMask) {
      xorbufRGBAPel16(outA, outI - rowDelta, outA, outI, fgPel)
    } else {
      copyRGBA(outA, outI - rowDelta, outA, outI)
    }
    outI += 4
    cmpMask <<= 1
  }
  return outI
}

function writeFirstLineFgBgImage(outA: Uint8ClampedArray | Uint8Array, outI: number, bitmask: number, fgPel: number, cBits: number): number {
  let cmpMask = 0x01
  while (cBits-- > 0) {
    pel2RGBA(bitmask & cmpMask ? fgPel : 0, outA, outI)
    outI += 4
    cmpMask <<= 1
  }
  return outI
}

function extractCodeId(b: number): number {
  switch (b) {
    case 0xf0: case 0xf1: case 0xf6: case 0xf8:
    case 0xf3: case 0xf2: case 0xf7: case 0xf4:
    case 0xf9: case 0xfa: case 0xfd: case 0xfe:
      return b
  }
  const code = b >> 5
  switch (code) {
    case 0x00: case 0x01: case 0x03: case 0x02: case 0x04:
      return code
  }
  return b >> 4
}

function extractRunLength(code: number, inA: Uint8Array, inI: number, advance: { val: number }): number {
  let runLength = 0
  let ladvance = 1
  switch (code) {
    case 0x02:
      runLength = inA[inI] & 0x1f
      if (runLength === 0) { runLength = inA[inI + 1] + 1; ladvance++ } else { runLength *= 8 }
      break
    case 0x0d:
      runLength = inA[inI] & 0x0f
      if (runLength === 0) { runLength = inA[inI + 1] + 1; ladvance++ } else { runLength *= 8 }
      break
    case 0x00: case 0x01: case 0x03: case 0x04:
      runLength = inA[inI] & 0x1f
      if (runLength === 0) { runLength = inA[inI + 1] + 32; ladvance++ }
      break
    case 0x0c: case 0x0e:
      runLength = inA[inI] & 0x0f
      if (runLength === 0) { runLength = inA[inI + 1] + 16; ladvance++ }
      break
    case 0xf0: case 0xf1: case 0xf6: case 0xf8:
    case 0xf3: case 0xf2: case 0xf7: case 0xf4:
      runLength = inA[inI + 1] | (inA[inI + 2] << 8)
      ladvance += 2
      break
  }
  advance.val = ladvance
  return runLength
}

/** 解码 RDP RLE16 压缩位图数据为 RGBA */
export function dRLE16_RGBA(inA: Uint8Array, inLength: number, width: number, outA: Uint8ClampedArray | Uint8Array) {
  let inI = 0, outI = 0
  let fInsertFgPel = false, fFirstLine = true
  let fgPel = 0xffffff
  const rowDelta = width * 4
  const advance = { val: 0 }

  while (inI < inLength) {
    if (fFirstLine && outI >= rowDelta) {
      fFirstLine = false
      fInsertFgPel = false
    }

    const code = extractCodeId(inA[inI])

    if (code === 0x00 || code === 0xf0) {
      let runLength = extractRunLength(code, inA, inI, advance)
      inI += advance.val
      if (fFirstLine) {
        if (fInsertFgPel) { pel2RGBA(fgPel, outA, outI); outI += 4; runLength-- }
        while (runLength-- > 0) { pel2RGBA(0, outA, outI); outI += 4 }
      } else {
        if (fInsertFgPel) { xorbufRGBAPel16(outA, outI - rowDelta, outA, outI, fgPel); outI += 4; runLength-- }
        while (runLength-- > 0) { copyRGBA(outA, outI - rowDelta, outA, outI); outI += 4 }
      }
      fInsertFgPel = true
      continue
    }

    fInsertFgPel = false

    switch (code) {
      case 0x01: case 0xf1: case 0x0c: case 0xf6: {
        let runLength = extractRunLength(code, inA, inI, advance)
        inI += advance.val
        if (code === 0x0c || code === 0xf6) { fgPel = inA[inI] | (inA[inI + 1] << 8); inI += 2 }
        if (fFirstLine) {
          while (runLength-- > 0) { pel2RGBA(fgPel, outA, outI); outI += 4 }
        } else {
          while (runLength-- > 0) { xorbufRGBAPel16(outA, outI - rowDelta, outA, outI, fgPel); outI += 4 }
        }
        break
      }
      case 0x0e: case 0xf8: {
        let runLength = extractRunLength(code, inA, inI, advance)
        inI += advance.val
        const pixelA = inA[inI] | (inA[inI + 1] << 8); inI += 2
        const pixelB = inA[inI] | (inA[inI + 1] << 8); inI += 2
        while (runLength-- > 0) { pel2RGBA(pixelA, outA, outI); outI += 4; pel2RGBA(pixelB, outA, outI); outI += 4 }
        break
      }
      case 0x03: case 0xf3: {
        let runLength = extractRunLength(code, inA, inI, advance)
        inI += advance.val
        const pixelA = inA[inI] | (inA[inI + 1] << 8); inI += 2
        while (runLength-- > 0) { pel2RGBA(pixelA, outA, outI); outI += 4 }
        break
      }
      case 0x02: case 0xf2: case 0x0d: case 0xf7: {
        let runLength = extractRunLength(code, inA, inI, advance)
        inI += advance.val
        if (code === 0x0d || code === 0xf7) { fgPel = inA[inI] | (inA[inI + 1] << 8); inI += 2 }
        while (runLength >= 8) {
          const bitmask = inA[inI++]
          if (fFirstLine) { outI = writeFirstLineFgBgImage(outA, outI, bitmask, fgPel, 8) }
          else { outI = writeFgBgImage(outA, outI, rowDelta, bitmask, fgPel, 8) }
          runLength -= 8
        }
        if (runLength > 0) {
          const bitmask = inA[inI++]
          if (fFirstLine) { outI = writeFirstLineFgBgImage(outA, outI, bitmask, fgPel, runLength) }
          else { outI = writeFgBgImage(outA, outI, rowDelta, bitmask, fgPel, runLength) }
        }
        break
      }
      case 0x04: case 0xf4: {
        let runLength = extractRunLength(code, inA, inI, advance)
        inI += advance.val
        while (runLength-- > 0) { pel2RGBA(inA[inI] | (inA[inI + 1] << 8), outA, outI); inI += 2; outI += 4 }
        break
      }
      case 0xf9:
        inI++
        if (fFirstLine) { outI = writeFirstLineFgBgImage(outA, outI, 0x03, fgPel, 8) }
        else { outI = writeFgBgImage(outA, outI, rowDelta, 0x03, fgPel, 8) }
        break
      case 0xfa:
        inI++
        if (fFirstLine) { outI = writeFirstLineFgBgImage(outA, outI, 0x05, fgPel, 8) }
        else { outI = writeFgBgImage(outA, outI, rowDelta, 0x05, fgPel, 8) }
        break
      case 0xfd:
        inI++
        pel2RGBA(0xffff, outA, outI); outI += 4
        break
      case 0xfe:
        inI++
        pel2RGBA(0, outA, outI); outI += 4
        break
    }
  }
}

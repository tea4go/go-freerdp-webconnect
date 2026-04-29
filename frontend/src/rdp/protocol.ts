// RDP 二进制帧协议常量和解析

// 服务端 → 客户端 操作码
export const WSOP = {
  BEGINPAINT: 0,
  ENDPAINT: 1,
  BITMAP: 2,
  OPAQUERECT: 3,
  SETBOUNDS: 4,
  PATBLT: 5,
  MULTI_OPAQUERECT: 6,
  SCRBLT: 7,
  PTR_NEW: 8,
  PTR_FREE: 9,
  PTR_SET: 10,
  PTR_SETNULL: 11,
  PTR_SETDEFAULT: 12,
} as const

// 客户端 → 服务端 操作码
export const INPUT_OP = {
  MOUSE: 0,
  KEY_UPDOWN: 1,
  KEY_CHAR: 2,
  RESIZE: 3,
} as const

// RDP ROP3 操作码
export const ROP3 = {
  PATINVERT: 0x005a0049,
  PATCOPY: 0x00f00021,
  SRCCOPY: 0x00cc0020,
} as const

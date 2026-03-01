package main

/*
#cgo CFLAGS: -I${SRCDIR}/install/include/freerdp3 -I${SRCDIR}/install/include/winpr3
#cgo LDFLAGS: -L${SRCDIR}/install/lib -lfreerdp3 -lfreerdp-client3 -lwinpr3
#include <freerdp/freerdp.h>
#include <freerdp/codec/color.h>
#include <freerdp/gdi/gdi.h>
#include <freerdp/settings.h>
#include <freerdp/input.h>
#include <freerdp/client.h>
#include <freerdp/client/disp.h>
#include <freerdp/client/cmdline.h>
#include <winpr/synch.h>
#include <winpr/collections.h>
#include <unistd.h>
#include <stdlib.h>
#include <string.h>

// 全局显示通道上下文指针，当 DISP 虚拟通道连接时设置
static DispClientContext* g_dispCtx = NULL;

// 通道连接事件回调：捕获 DISP 虚拟通道的上下文指针，用于动态分辨率调整
static void onChannelConnected(void* context, const ChannelConnectedEventArgs* e) {
    if (strcmp(e->name, DISP_DVC_CHANNEL_NAME) == 0) {
        g_dispCtx = (DispClientContext*)e->pInterface;
    }
}

// 通道断开事件回调：清除 DISP 虚拟通道上下文指针
static void onChannelDisconnected(void* context, const ChannelDisconnectedEventArgs* e) {
    if (strcmp(e->name, DISP_DVC_CHANNEL_NAME) == 0) {
        g_dispCtx = NULL;
    }
}

// 注册通道连接/断开事件，以便在 DISP 通道就绪时获取其上下文
static void registerChannelEvents(freerdp* instance) {
    wPubSub* pubSub = instance->context->pubSub;
    PubSub_SubscribeChannelConnected(pubSub, onChannelConnected);
    PubSub_SubscribeChannelDisconnected(pubSub, onChannelDisconnected);
}

// 将 JavaScript keyCode（仅限修饰键）转换为 RDP 扫描码
// JS 修饰键列表: [8=退格, 16=Shift, 17=Ctrl, 18=Alt, 20=CapsLock, 144=NumLock, 145=ScrollLock]
static UINT32 jsKeyCodeToScancode(UINT32 keycode) {
    switch (keycode) {
        case 8:   return 0x0E;  // Backspace
        case 16:  return 0x2A;  // Left Shift
        case 17:  return 0x1D;  // Left Ctrl
        case 18:  return 0x38;  // Left Alt
        case 20:  return 0x3A;  // CapsLock
        case 144: return 0x45;  // NumLock
        case 145: return 0x46;  // ScrollLock
        default:  return 0;
    }
}

// 发送鼠标事件到 RDP 服务器（移动、点击、滚轮等）
static void sendMouseInput(freerdp* instance, UINT32 flags, UINT32 x, UINT32 y) {
    freerdp_input_send_mouse_event(instance->context->input, (UINT16)flags, (UINT16)x, (UINT16)y);
}

// 发送键盘按下/抬起事件（仅处理修饰键，通过扫描码传递）
static void sendKupdownInput(freerdp* instance, UINT32 down, UINT32 keycode) {
    UINT32 scancode = jsKeyCodeToScancode(keycode);
    if (scancode == 0) return;
    freerdp_input_send_keyboard_event_ex(instance->context->input,
        down ? TRUE : FALSE, FALSE, scancode);
}

// 通过 RDPEDISP 虚拟通道发送动态分辨率调整请求
// 需要 g_dispCtx 已就绪（DISP 通道已连接）
static void sendResizeInput(freerdp* instance, UINT32 width, UINT32 height) {
    if (!g_dispCtx || !g_dispCtx->SendMonitorLayout) return;
    DISPLAY_CONTROL_MONITOR_LAYOUT monitor = {0};
    monitor.Flags = DISPLAY_CONTROL_MONITOR_PRIMARY;
    monitor.Left = 0;
    monitor.Top = 0;
    monitor.Width = width;
    monitor.Height = height;
    monitor.PhysicalWidth = 0;
    monitor.PhysicalHeight = 0;
    monitor.Orientation = 0;
    monitor.DesktopScaleFactor = 100;
    monitor.DeviceScaleFactor = 100;
    g_dispCtx->SendMonitorLayout(g_dispCtx, 1, &monitor);
}

// 发送 Unicode 键盘字符输入（按下后立即释放，用于普通字符输入）
static void sendKpressInput(freerdp* instance, UINT32 charcode) {
    freerdp_input_send_unicode_keyboard_event(instance->context->input,
        0, (UINT16)charcode);
    freerdp_input_send_unicode_keyboard_event(instance->context->input,
        KBD_FLAGS_RELEASE, (UINT16)charcode);
}

// 颜色格式转换辅助函数：在 16bpp 和 32bpp 之间转换像素颜色值
static inline UINT32 convertColor(UINT32 color, UINT32 srcBpp, UINT32 dstBpp) {
    UINT32 srcFormat = (srcBpp == 16) ? PIXEL_FORMAT_RGB16 : PIXEL_FORMAT_BGRX32;
    UINT32 dstFormat = (dstBpp == 32) ? PIXEL_FORMAT_BGRX32 : PIXEL_FORMAT_RGB16;
    return FreeRDPConvertColor(color, srcFormat, dstFormat, NULL);
}

// 图像垂直翻转辅助函数：RDP 位图数据为自底向上存储，需翻转为自顶向下
static inline void flipImageData(BYTE* data, int width, int height, int bpp) {
    int scanline = width * (bpp / 8);
    BYTE* tmpLine = (BYTE*)malloc(scanline);
    if (!tmpLine) return;
    for (int i = 0; i < height / 2; i++) {
        BYTE* line1 = data + (i * scanline);
        BYTE* line2 = data + ((height - 1 - i) * scanline);
        memcpy(tmpLine, line1, scanline);
        memcpy(line1, line2, scanline);
        memcpy(line2, tmpLine, scanline);
    }
    free(tmpLine);
}

// 从 freerdp 实例获取配置项指针的辅助函数
static inline rdpSettings* getSettings(freerdp* instance) {
    return instance->context->settings;
}

extern BOOL preConnect(freerdp* instance);
extern void postConnect(freerdp* instance);
extern BOOL primaryPatBlt(rdpContext* context, PATBLT_ORDER* patblt);
extern BOOL primaryScrBlt(rdpContext* context, SCRBLT_ORDER* scrblt);
extern BOOL primaryOpaqueRect(rdpContext* context, OPAQUE_RECT_ORDER* oro);
extern BOOL primaryMultiOpaqueRect(rdpContext* context, MULTI_OPAQUE_RECT_ORDER* moro);
extern BOOL beginPaint(rdpContext* context);
extern BOOL endPaint(rdpContext* context);
extern BOOL setBounds(rdpContext* context, rdpBounds* bounds);
extern BOOL bitmapUpdate(rdpContext* context, BITMAP_UPDATE* bitmap);

static BOOL cbPrimaryPatBlt(rdpContext* context, PATBLT_ORDER* patblt) {
	return primaryPatBlt(context, patblt);
}

static BOOL cbPrimaryScrBlt(rdpContext* context, const SCRBLT_ORDER* scrblt) {
	return primaryScrBlt(context, (SCRBLT_ORDER*)scrblt);
}

static BOOL cbPrimaryOpaqueRect(rdpContext* context, const OPAQUE_RECT_ORDER* oro) {
	return primaryOpaqueRect(context, (OPAQUE_RECT_ORDER*)oro);
}

static BOOL cbPrimaryMultiOpaqueRect(rdpContext* context, const MULTI_OPAQUE_RECT_ORDER* moro) {
	return primaryMultiOpaqueRect(context, (MULTI_OPAQUE_RECT_ORDER*)moro);
}

static BOOL cbBeginPaint(rdpContext* context) {
	return beginPaint(context);
}
static BOOL cbEndPaint(rdpContext* context) {
	return endPaint(context);
}
static BOOL cbSetBounds(rdpContext* context, const rdpBounds* bounds) {
	return setBounds(context, (rdpBounds*)bounds);
}
static BOOL cbBitmapUpdate(rdpContext* context, const BITMAP_UPDATE* bitmap) {
	return bitmapUpdate(context, (BITMAP_UPDATE*)bitmap);
}

static BOOL cbPointerNew(rdpContext* context, const POINTER_NEW_UPDATE* p) { return TRUE; }
static BOOL cbPointerCached(rdpContext* context, const POINTER_CACHED_UPDATE* p) { return TRUE; }
static BOOL cbPointerSystem(rdpContext* context, const POINTER_SYSTEM_UPDATE* p) { return TRUE; }
static BOOL cbPointerPosition(rdpContext* context, const POINTER_POSITION_UPDATE* p) { return TRUE; }
static BOOL cbPointerColor(rdpContext* context, const POINTER_COLOR_UPDATE* p) { return TRUE; }
static BOOL cbPointerLarge(rdpContext* context, const POINTER_LARGE_UPDATE* p) { return TRUE; }

// 预连接回调（cbPreConnect）：在建立 RDP 连接前注册所有绘图/输入回调函数
// 包括：图形绘制、位图更新、鼠标指针、动态分辨率通道等
static BOOL cbPreConnect(freerdp* instance) {
	rdpContext* context = instance->context;
	rdpUpdate* update = context->update;
	rdpPrimaryUpdate* primary = update->primary;
	rdpPointerUpdate* pointer = update->pointer;

	// 注册图形绘制命令回调
	primary->PatBlt = cbPrimaryPatBlt;
	primary->ScrBlt = cbPrimaryScrBlt;
	primary->OpaqueRect = cbPrimaryOpaqueRect;
	primary->MultiOpaqueRect = cbPrimaryMultiOpaqueRect;

	// 注册帧边界和位图更新回调
	update->BeginPaint = cbBeginPaint;
	update->EndPaint = cbEndPaint;
	update->SetBounds = cbSetBounds;
	update->BitmapUpdate = cbBitmapUpdate;

	// 注册鼠标指针回调（当前均为空实现，忽略服务器指针更新）
	pointer->PointerNew = cbPointerNew;
	pointer->PointerCached = cbPointerCached;
	pointer->PointerSystem = cbPointerSystem;
	pointer->PointerPosition = cbPointerPosition;
	pointer->PointerColor = cbPointerColor;
	pointer->PointerLarge = cbPointerLarge;

	// 注册通道连接/断开事件，以便捕获 DISP 虚拟通道上下文
	registerChannelEvents(instance);

	// 加载 DISP 动态虚拟通道，用于支持动态分辨率调整
	const char* dispName = DISP_DVC_CHANNEL_NAME;
	freerdp_client_add_dynamic_channel(instance->context->settings, 1, &dispName);

	return preConnect(instance);
}

// 连接成功回调（cbPostConnect）：初始化 GDI 并重新注册绘图回调
// 注意：gdi_init 会覆盖之前注册的回调，因此需要在此重新注册
static BOOL cbPostConnect(freerdp* instance) {
	// 初始化 GDI 缓存子系统（含指针缓存），使用 32 位 XRGB 格式
	// 若不调用此函数，context->cache 为 NULL，指针更新会崩溃
	if (!gdi_init(instance, PIXEL_FORMAT_XRGB32))
		return FALSE;

	// 加载通道插件（含 DISP），需在连接建立后调用
	freerdp_client_load_channels(instance);

	// gdi_init 会用 GDI 默认回调覆盖我们的回调，此处重新注册
	rdpContext* context = instance->context;
	rdpUpdate* update = context->update;
	rdpPrimaryUpdate* primary = update->primary;
	rdpPointerUpdate* pointer = update->pointer;

	primary->PatBlt = cbPrimaryPatBlt;
	primary->ScrBlt = cbPrimaryScrBlt;
	primary->OpaqueRect = cbPrimaryOpaqueRect;
	primary->MultiOpaqueRect = cbPrimaryMultiOpaqueRect;

	update->BeginPaint = cbBeginPaint;
	update->EndPaint = cbEndPaint;
	update->SetBounds = cbSetBounds;
	update->BitmapUpdate = cbBitmapUpdate;

	pointer->PointerNew = cbPointerNew;
	pointer->PointerCached = cbPointerCached;
	pointer->PointerSystem = cbPointerSystem;
	pointer->PointerPosition = cbPointerPosition;
	pointer->PointerColor = cbPointerColor;
	pointer->PointerLarge = cbPointerLarge;

	postConnect(instance);
	return TRUE;
}

// 获取位图更新中第 i 个矩形区域的数据指针
static BITMAP_DATA* nextBitmapRectangle(BITMAP_UPDATE* bitmap, int i) {
	return &bitmap->rectangles[i];
}

// 获取多矩形填充命令中第 i 个矩形的数据指针
static DELTA_RECT* nextMultiOpaqueRectangle(MULTI_OPAQUE_RECT_ORDER* moro, int i) {
	return &moro->rectangles[i];
}

// 绑定预连接和后连接回调到 freerdp 实例
static void bindCallbacks(freerdp* instance) {
	instance->PreConnect = cbPreConnect;
	instance->PostConnect = cbPostConnect;
}

// 检查并处理 FreeRDP 事件句柄（最多等待 100ms）
// 用于主事件循环中驱动 RDP 协议收发
static BOOL checkEventHandles(freerdp* instance) {
	HANDLE events[64] = {0};
	DWORD nCount = freerdp_get_event_handles(instance->context, events, 64);
	if (nCount == 0) return FALSE;
	DWORD status = WaitForMultipleObjects(nCount, events, FALSE, 100);
	if (status == WAIT_FAILED) return FALSE;
	return freerdp_check_event_handles(instance->context);
}
*/
import (
	"C"
)
import (
	"bytes"
	"encoding/binary"
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

// WebSocket 操作码常量，服务端发往客户端的消息类型
const (
	WSOP_SC_BEGINPAINT       uint32 = 0  // 开始绘制帧
	WSOP_SC_ENDPAINT         uint32 = 1  // 结束绘制帧
	WSOP_SC_BITMAP           uint32 = 2  // 位图更新
	WSOP_SC_OPAQUERECT       uint32 = 3  // 填充不透明矩形
	WSOP_SC_SETBOUNDS        uint32 = 4  // 设置绘制边界
	WSOP_SC_PATBLT           uint32 = 5  // 图案填充（PatBlt）
	WSOP_SC_MULTI_OPAQUERECT uint32 = 6  // 多矩形填充
	WSOP_SC_SCRBLT           uint32 = 7  // 屏幕区域复制（ScrBlt）
	WSOP_SC_PTR_NEW          uint32 = 8  // 新建鼠标指针
	WSOP_SC_PTR_FREE         uint32 = 9  // 释放鼠标指针
	WSOP_SC_PTR_SET          uint32 = 10 // 设置当前鼠标指针
	WSOP_SC_PTR_SETNULL      uint32 = 11 // 隐藏鼠标指针
	WSOP_SC_PTR_SETDEFAULT   uint32 = 12 // 恢复默认鼠标指针
)

// bitmapUpdateMeta 是发送给客户端的位图更新消息头
// 包含目标位置、尺寸、色深、是否压缩及数据长度
type bitmapUpdateMeta struct {
	op  uint32 // 操作码，固定为 WSOP_SC_BITMAP
	x   uint32 // 目标区域左上角 X 坐标
	y   uint32 // 目标区域左上角 Y 坐标
	w   uint32 // 源位图宽度
	h   uint32 // 源位图高度
	dw  uint32 // 目标区域宽度
	dh  uint32 // 目标区域高度
	bpp uint32 // 每像素位数
	cf  uint32 // 是否压缩（0=未压缩，非0=压缩）
	sz  uint32 // 位图数据字节长度
}

// primaryPatBltMeta 是 PatBlt 图案填充命令的消息结构
type primaryPatBltMeta struct {
	op  uint32 // 操作码，固定为 WSOP_SC_PATBLT
	x   int32  // 矩形左上角 X
	y   int32  // 矩形左上角 Y
	w   int32  // 矩形宽度
	h   int32  // 矩形高度
	fg  uint32 // 前景色（32bpp BGRX）
	rop uint32 // 光栅操作码
}

// primaryScrBltMeta 是 ScrBlt 屏幕区域复制命令的消息结构
type primaryScrBltMeta struct {
	op  uint32 // 操作码，固定为 WSOP_SC_SCRBLT
	rop uint32 // 光栅操作码
	x   int32  // 目标左上角 X
	y   int32  // 目标左上角 Y
	w   int32  // 宽度
	h   int32  // 高度
	sx  int32  // 源区域左上角 X
	sy  int32  // 源区域左上角 Y
}

// inputEvent 携带一条来自 WebSocket 客户端的输入消息
// op=0 鼠标事件: a=flags b=x c=y
// op=1 键盘按下/抬起（修饰键）: a=down(1)/up(0) b=keycode
// op=2 键盘字符输入（Unicode）: a=修饰键（未使用） b=charcode
// op=3 分辨率调整: a=width b=height
type inputEvent struct {
	op uint32
	a  uint32
	b  uint32
	c  uint32
}

// rdpConnectionSettings 保存建立 RDP 连接所需的参数
type rdpConnectionSettings struct {
	hostname *string
	username *string
	password *string
	width    int
	height   int
	port     int
}

// rdpContextData 保存单个 RDP 连接的所有 Go 侧数据
// 完全在 Go 内存中分配，由 GC 管理
// 不在 C 分配的内存中存储 Go 指针（CGo 规范要求）
type rdpContextData struct {
	sendq    chan []byte             // 向 WebSocket 客户端发送数据的队列
	recvq    chan []byte             // 接收断开信号的队列
	settings *rdpConnectionSettings // RDP 连接参数
}

// 全局注册表：C rdpContext 指针值 → Go 数据
// key 使用 uintptr（原始地址），而非 Go 指针，符合 CGo 安全规范
var (
	contextMu  sync.Mutex
	contextMap = make(map[uintptr]*rdpContextData)
)

// registerCtx 将 C rdpContext 指针与 Go 数据关联，存入全局注册表
func registerCtx(ctx *C.rdpContext, d *rdpContextData) {
	contextMu.Lock()
	contextMap[uintptr(unsafe.Pointer(ctx))] = d
	contextMu.Unlock()
}

// unregisterCtx 从全局注册表中移除指定 C rdpContext 的关联数据
func unregisterCtx(ctx *C.rdpContext) {
	contextMu.Lock()
	delete(contextMap, uintptr(unsafe.Pointer(ctx)))
	contextMu.Unlock()
}

// lookupCtx 根据 C rdpContext 指针查找对应的 Go 数据
func lookupCtx(ctx *C.rdpContext) *rdpContextData {
	contextMu.Lock()
	d := contextMap[uintptr(unsafe.Pointer(ctx))]
	contextMu.Unlock()
	return d
}

// rdpconnect 是 RDP 连接的主函数，运行在独立 goroutine 中
// 负责建立连接、驱动事件循环、处理输入事件，直到连接断开
func rdpconnect(sendq chan []byte, recvq chan []byte, inputq chan inputEvent, settings *rdpConnectionSettings) {
	// 锁定当前 OS 线程：FreeRDP 的传输层/TLS 使用线程本地状态，
	// 所有 FreeRDP 调用必须在同一 OS 线程上执行
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	fmt.Println("RDP Connecting...")

	instance := C.freerdp_new()
	C.bindCallbacks(instance)

	// 使用原生 C rdpContext 大小，不在 C 内存中嵌入 Go 字段
	instance.ContextSize = C.size_t(C.sizeof_rdpContext)
	C.freerdp_context_new(instance)

	// 将 Go 数据注册到全局 map，以 C context 指针为 key
	// 这是 CGo 安全的替代方案，避免在 C 内存中存储 Go 指针
	data := &rdpContextData{sendq: sendq, recvq: recvq, settings: settings}
	registerCtx(instance.context, data)
	defer unregisterCtx(instance.context)

	if C.freerdp_connect(instance) == 0 {
		fmt.Println("RDP connection failed")
		C.freerdp_free(instance)
		return
	}

	mainEventLoop := true

	for mainEventLoop {
		select {
		case <-recvq:
			// WebSocket 出错或客户端断开，退出事件循环
			fmt.Println("Disconnecting (websocket error)")
			mainEventLoop = false
		case ev := <-inputq:
			// 处理来自客户端的输入事件
			switch ev.op {
			case 0: // 鼠标事件
				C.sendMouseInput(instance, C.UINT32(ev.a), C.UINT32(ev.b), C.UINT32(ev.c))
			case 1: // 修饰键按下/抬起
				C.sendKupdownInput(instance, C.UINT32(ev.a), C.UINT32(ev.b))
			case 2: // Unicode 字符输入
				C.sendKpressInput(instance, C.UINT32(ev.b))
			case 3: // 动态分辨率调整
				C.sendResizeInput(instance, C.UINT32(ev.a), C.UINT32(ev.b))
			}
		default:
			// 检查 RDP 服务端是否发送了错误或断开信号
			e := int(C.freerdp_error_info(instance))
			if e != 0 {
				switch e {
				case 1:
				case 2:
				case 7:
				case 9:
					// 手动断开连接等情况
					fmt.Println("Disconnecting (manual)")
					mainEventLoop = false
				case 5:
					// 另一个用户连接了同一会话
				}
			}
			if int(C.freerdp_shall_disconnect(instance)) != 0 {
				fmt.Println("Disconnecting (RDC said so)")
				mainEventLoop = false
			}
			if mainEventLoop {
				C.checkEventHandles(instance)
			}
		}
	}
	C.freerdp_free(instance)
}

// sendBinary 将缓冲区数据发送到 WebSocket 发送队列
func sendBinary(sendq chan []byte, data *bytes.Buffer) {
	sendq <- data.Bytes()
}

//export primaryPatBlt
// primaryPatBlt 处理 PatBlt 图案填充命令：仅处理纯色填充（GDI_BS_SOLID），转换颜色后发送给客户端
func primaryPatBlt(rawContext *C.rdpContext, patblt *C.PATBLT_ORDER) C.BOOL {
	d := lookupCtx(rawContext)
	if d == nil {
		return C.TRUE
	}

	if C.GDI_BS_SOLID == patblt.brush.style {
		color := uint32(C.convertColor(patblt.foreColor, 16, 32))

		meta := primaryPatBltMeta{
			WSOP_SC_PATBLT,
			int32(patblt.nLeftRect),
			int32(patblt.nTopRect),
			int32(patblt.nWidth),
			int32(patblt.nHeight),
			color,
			uint32(patblt.bRop),
		}

		buf := new(bytes.Buffer)
		binary.Write(buf, binary.LittleEndian, meta)
		sendBinary(d.sendq, buf)
	}
	return C.TRUE
}

//export primaryScrBlt
// primaryScrBlt 处理屏幕区域复制命令（ScrBlt），将源区域复制到目标区域
func primaryScrBlt(rawContext *C.rdpContext, scrblt *C.SCRBLT_ORDER) C.BOOL {
	d := lookupCtx(rawContext)
	if d == nil {
		return C.TRUE
	}

	meta := primaryScrBltMeta{
		WSOP_SC_SCRBLT,
		uint32(scrblt.bRop),
		int32(scrblt.nLeftRect),
		int32(scrblt.nTopRect),
		int32(scrblt.nWidth),
		int32(scrblt.nHeight),
		int32(scrblt.nXSrc),
		int32(scrblt.nYSrc),
	}

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, meta)
	sendBinary(d.sendq, buf)
	return C.TRUE
}

//export primaryOpaqueRect
// primaryOpaqueRect 处理单个不透明矩形填充命令，转换颜色后发送给客户端
func primaryOpaqueRect(rawContext *C.rdpContext, oro *C.OPAQUE_RECT_ORDER) C.BOOL {
	d := lookupCtx(rawContext)
	if d == nil {
		return C.TRUE
	}

	color := C.convertColor(oro.color, 16, 32)

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, WSOP_SC_OPAQUERECT)

	type opaqueRectOrder struct {
		nLeftRect int32
		nTopRect  int32
		nWidth    int32
		nHeight   int32
		color     uint32
	}

	order := opaqueRectOrder{
		nLeftRect: int32(oro.nLeftRect),
		nTopRect:  int32(oro.nTopRect),
		nWidth:    int32(oro.nWidth),
		nHeight:   int32(oro.nHeight),
		color:     uint32(color),
	}

	binary.Write(buf, binary.LittleEndian, order)
	sendBinary(d.sendq, buf)
	return C.TRUE
}

//export primaryMultiOpaqueRect
// primaryMultiOpaqueRect 处理多矩形填充命令，将所有矩形区域一次性发送给客户端
func primaryMultiOpaqueRect(rawContext *C.rdpContext, moro *C.MULTI_OPAQUE_RECT_ORDER) C.BOOL {
	d := lookupCtx(rawContext)
	if d == nil {
		return C.TRUE
	}

	color := C.convertColor(moro.color, 16, 32)

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, WSOP_SC_MULTI_OPAQUERECT)
	binary.Write(buf, binary.LittleEndian, int32(color))
	binary.Write(buf, binary.LittleEndian, int32(moro.numRectangles))

	var r *C.DELTA_RECT
	for i := 1; i <= int(moro.numRectangles); i++ {
		r = C.nextMultiOpaqueRectangle(moro, C.int(i))
		binary.Write(buf, binary.LittleEndian, r)
	}

	sendBinary(d.sendq, buf)
	return C.TRUE
}

//export beginPaint
func beginPaint(rawContext *C.rdpContext) C.BOOL {
	d := lookupCtx(rawContext)
	if d == nil {
		return C.TRUE
	}
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, WSOP_SC_BEGINPAINT)
	sendBinary(d.sendq, buf)
	return C.TRUE
}

//export endPaint
func endPaint(rawContext *C.rdpContext) C.BOOL {
	d := lookupCtx(rawContext)
	if d == nil {
		return C.TRUE
	}
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, WSOP_SC_ENDPAINT)
	sendBinary(d.sendq, buf)
	return C.TRUE
}

//export setBounds
func setBounds(rawContext *C.rdpContext, bounds *C.rdpBounds) C.BOOL {
	d := lookupCtx(rawContext)
	if d == nil {
		return C.TRUE
	}
	if bounds != nil {
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.LittleEndian, WSOP_SC_SETBOUNDS)
		binary.Write(buf, binary.LittleEndian, bounds)
		sendBinary(d.sendq, buf)
	}
	return C.TRUE
}

//export bitmapUpdate
func bitmapUpdate(rawContext *C.rdpContext, bitmap *C.BITMAP_UPDATE) C.BOOL {
	d := lookupCtx(rawContext)
	if d == nil {
		return C.TRUE
	}

	for i := 0; i < int(bitmap.number); i++ {
		bmd := C.nextBitmapRectangle(bitmap, C.int(i))

		buf := new(bytes.Buffer)

		meta := bitmapUpdateMeta{
			WSOP_SC_BITMAP,
			uint32(bmd.destLeft),
			uint32(bmd.destTop),
			uint32(bmd.width),
			uint32(bmd.height),
			uint32(bmd.destRight - bmd.destLeft + 1),
			uint32(bmd.destBottom - bmd.destTop + 1),
			uint32(bmd.bitsPerPixel),
			uint32(bmd.compressed),
			uint32(bmd.bitmapLength),
		}
		if int(bmd.compressed) == 0 {
			C.flipImageData(bmd.bitmapDataStream, C.int(bmd.width), C.int(bmd.height), C.int(bmd.bitsPerPixel))
		}

		binary.Write(buf, binary.LittleEndian, meta)

		clen := int(bmd.bitmapLength)
		bitmapDataStream := unsafe.Slice((*byte)(unsafe.Pointer(bmd.bitmapDataStream)), clen)
		binary.Write(buf, binary.LittleEndian, bitmapDataStream)

		sendBinary(d.sendq, buf)
	}
	return C.TRUE
}

//export postConnect
func postConnect(_ *C.freerdp) {
	fmt.Println("Connected.")
}

//export preConnect
func preConnect(instance *C.freerdp) C.BOOL {
	d := lookupCtx(instance.context)
	if d == nil {
		return C.FALSE
	}
	settings := C.getSettings(instance)

	hostname := C.CString(*d.settings.hostname)
	username := C.CString(*d.settings.username)
	password := C.CString(*d.settings.password)
	defer C.free(unsafe.Pointer(hostname))
	defer C.free(unsafe.Pointer(username))
	defer C.free(unsafe.Pointer(password))

	C.freerdp_settings_set_string(settings, C.FreeRDP_ServerHostname, hostname)
	C.freerdp_settings_set_string(settings, C.FreeRDP_Username, username)
	C.freerdp_settings_set_string(settings, C.FreeRDP_Password, password)
	C.freerdp_settings_set_uint32(settings, C.FreeRDP_DesktopWidth, C.UINT32(d.settings.width))
	C.freerdp_settings_set_uint32(settings, C.FreeRDP_DesktopHeight, C.UINT32(d.settings.height))
	C.freerdp_settings_set_uint32(settings, C.FreeRDP_ServerPort, C.UINT32(d.settings.port))
	C.freerdp_settings_set_bool(settings, C.FreeRDP_IgnoreCertificate, C.TRUE)
	C.freerdp_settings_set_uint32(settings, C.FreeRDP_ColorDepth, 16)

	// Security settings - disable NLA and TLS, use RDP security
	C.freerdp_settings_set_bool(settings, C.FreeRDP_NlaSecurity, C.FALSE)
	C.freerdp_settings_set_bool(settings, C.FreeRDP_TlsSecurity, C.FALSE)
	C.freerdp_settings_set_bool(settings, C.FreeRDP_RdpSecurity, C.TRUE)
	C.freerdp_settings_set_bool(settings, C.FreeRDP_UseRdpSecurityLayer, C.TRUE)

	// Performance flags
	perfFlags := C.PERF_DISABLE_WALLPAPER | C.PERF_DISABLE_THEMING |
		C.PERF_DISABLE_MENUANIMATIONS | C.PERF_DISABLE_FULLWINDOWDRAG
	C.freerdp_settings_set_uint32(settings, C.FreeRDP_PerformanceFlags, C.UINT32(perfFlags))

	C.freerdp_settings_set_uint32(settings, C.FreeRDP_ConnectionType, C.CONNECTION_TYPE_BROADBAND_HIGH)
	C.freerdp_settings_set_bool(settings, C.FreeRDP_RemoteFxCodec, C.FALSE)
	C.freerdp_settings_set_bool(settings, C.FreeRDP_FastPathOutput, C.TRUE)
	C.freerdp_settings_set_uint32(settings, C.FreeRDP_FrameAcknowledge, 1)
	C.freerdp_settings_set_uint32(settings, C.FreeRDP_LargePointerFlag, 1)
	C.freerdp_settings_set_uint32(settings, C.FreeRDP_GlyphSupportLevel, C.GLYPH_SUPPORT_FULL)
	C.freerdp_settings_set_bool(settings, C.FreeRDP_BitmapCacheV3Enabled, C.FALSE)
	C.freerdp_settings_set_uint32(settings, C.FreeRDP_OffscreenSupportLevel, 0)

	// Enable dynamic desktop resize via RDPEDISP virtual channel
	// FreeRDP_SupportDisplayControl=5185, FreeRDP_DynamicResolutionUpdate=1558
	C.freerdp_settings_set_bool(settings, 5185, C.TRUE)
	C.freerdp_settings_set_bool(settings, 1558, C.TRUE)

	return C.TRUE
}

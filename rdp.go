package main

/*
#cgo darwin CFLAGS: -I/usr/local/opt/freerdp/include/freerdp3 -I/usr/local/opt/freerdp/include/winpr3
#cgo darwin LDFLAGS: -L/usr/local/opt/freerdp/lib -lfreerdp3 -lfreerdp-client3 -lwinpr3
#cgo linux CFLAGS: -I${SRCDIR}/install/include/freerdp3 -I${SRCDIR}/install/include/winpr3
#cgo linux LDFLAGS: -L${SRCDIR}/install/lib -lfreerdp3 -lfreerdp-client3 -lwinpr3
#include <freerdp/freerdp.h>
#include <freerdp/codec/color.h>
#include <freerdp/gdi/gdi.h>
#include <freerdp/settings.h>
#include <freerdp/input.h>
#include <freerdp/client.h>
#include <freerdp/client/disp.h>
#include <freerdp/client/cmdline.h>
#include <freerdp/client/channels.h>
#include <freerdp/addin.h>
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

	// DISP 通道由 freerdp_client_load_addins（通过 LoadChannels 回调）自动添加，
	// 使用内部名称 "disp"（而非协议层名 DISP_DVC_CHANNEL_NAME），无需在此手动添加。

	return preConnect(instance);
}

// 连接成功回调（cbPostConnect）：初始化 GDI 并重新注册绘图回调
// 注意：gdi_init 会覆盖之前注册的回调，因此需要在此重新注册
static BOOL cbPostConnect(freerdp* instance) {
	// 初始化 GDI 缓存子系统（含指针缓存），使用 32 位 XRGB 格式
	// 若不调用此函数，context->cache 为 NULL，指针更新会崩溃
	if (!gdi_init(instance, PIXEL_FORMAT_XRGB32))
		return FALSE;

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

// 绑定预连接和后连接回调到 freerdp 实例，并设置 LoadChannels 回调。
// LoadChannels 由 FreeRDP 在 cbPreConnect 之后、TCP 连接之前通过
// utils_reload_channels 自动调用，确保通道管理器（pChannelMgr）
// 在 drdynvc 加载 DVC 插件时已完成初始化。
static void bindCallbacks(freerdp* instance) {
	instance->PreConnect = cbPreConnect;
	instance->PostConnect = cbPostConnect;
	instance->LoadChannels = freerdp_client_load_channels;
}

// 注册静态通道插件提供者（freerdp_context_new 不会自动注册，需手动调用）
// 注册后 freerdp_client_load_channels 可从静态表中查找通道入口点，
// 不再依赖不存在的 .so 动态插件文件，消除 ERROR 日志。
static BOOL registerStaticAddinProvider(void) {
	return freerdp_register_addin_provider(freerdp_channels_load_static_addin_entry, 0) == CHANNEL_RC_OK;
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
	sendq    chan []byte            // 向 WebSocket 客户端发送数据的队列
	recvq    chan []byte            // 接收断开信号的队列
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

// GetFreeRDPVersion 从 FreeRDP 动态库读取版本字符串
func GetFreeRDPVersion() string {
	return C.GoString(C.freerdp_get_version_string())
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

	// 注册静态通道提供者：freerdp_context_new 不会自动注册，
	// 手动注册后通道可从静态表加载，无需依赖不存在的 .so 插件文件
	C.registerStaticAddinProvider()

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

// primaryPatBlt 处理 PatBlt 图案填充命令：仅处理纯色填充（GDI_BS_SOLID），转换颜色后发送给客户端
//
//export primaryPatBlt
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

// primaryScrBlt 处理屏幕区域复制命令（ScrBlt），将源区域复制到目标区域
//
//export primaryScrBlt
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

// primaryOpaqueRect 处理单个不透明矩形填充命令，转换颜色后发送给客户端
//
//export primaryOpaqueRect
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

// primaryMultiOpaqueRect 处理多矩形填充命令，将所有矩形区域一次性发送给客户端
//
//export primaryMultiOpaqueRect
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

// beginPaint 处理帧绘制开始事件，向客户端发送 BEGINPAINT 信号，通知前端准备接收本帧更新
//
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

// endPaint 处理帧绘制结束事件，向客户端发送 ENDPAINT 信号，通知前端本帧所有更新已发送完毕
//
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

// setBounds 处理绘制裁剪边界设置命令，向客户端发送 SETBOUNDS 消息限定本帧的有效绘制区域
// bounds 为 nil 时忽略（服务器取消裁剪区域时会发送 nil）
//
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

// bitmapUpdate 处理位图更新命令，将服务器发来的一批位图矩形逐一编码后发送给 WebSocket 客户端
// 对未压缩位图需先做垂直翻转（RDP 位图为自底向上存储，浏览器 Canvas 需自顶向下）
//
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

// postConnect 在 RDP 连接成功建立后由 C 层回调，用于记录连接成功日志
//
//export postConnect
func postConnect(_ *C.freerdp) {
	fmt.Println("Connected.")
}

// preConnect 在建立 RDP 连接前由 C 层回调，负责将 Go 侧的连接参数写入 FreeRDP 配置，
// 包括主机名、用户名、密码、分辨率、端口、安全协议、性能优化参数及动态分辨率支持等
//
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

	// 通过AI来问：在freerdp里，FreeRDP_UseRdpSecurityLayer是什么意思，有什么作用，可以填哪些值。
	C.freerdp_settings_set_string(settings, C.FreeRDP_ServerHostname, hostname)
	C.freerdp_settings_set_string(settings, C.FreeRDP_Username, username)
	C.freerdp_settings_set_string(settings, C.FreeRDP_Password, password)
	C.freerdp_settings_set_uint32(settings, C.FreeRDP_DesktopWidth, C.UINT32(d.settings.width))
	C.freerdp_settings_set_uint32(settings, C.FreeRDP_DesktopHeight, C.UINT32(d.settings.height))
	C.freerdp_settings_set_uint32(settings, C.FreeRDP_ServerPort, C.UINT32(d.settings.port))
	C.freerdp_settings_set_bool(settings, C.FreeRDP_IgnoreCertificate, C.TRUE)
	C.freerdp_settings_set_uint32(settings, C.FreeRDP_ColorDepth, 16)

	// 安全协议设置：禁用 NLA 和 TLS，改用经典 RDP 安全层（兼容旧版 Windows 及不支持 NLA 的环境）
	C.freerdp_settings_set_bool(settings, C.FreeRDP_NlaSecurity, C.FALSE)        // 是与网络安全层认证（Network Level Authentication，NLA）相关的配置选项，用于控制客户端与远程桌面服务（如Windows RDP）之间的安全认证方式。
	C.freerdp_settings_set_bool(settings, C.FreeRDP_TlsSecurity, C.FALSE)        // 一个配置选项，用于控制客户端与远程服务器之间通过 TLS（Transport Layer Security） 建立安全连接时的行为模式。 0 禁用TLS加密（不安全，不推荐）。
	C.freerdp_settings_set_bool(settings, C.FreeRDP_RdpSecurity, C.TRUE)         //用于配置RDP连接的安全协议和加密方式。它决定了客户端与服务器之间通信的安全级别和兼容性。
	C.freerdp_settings_set_bool(settings, C.FreeRDP_UseRdpSecurityLayer, C.TRUE) //一个配置选项，用于控制RDP连接时使用的安全协议层（Security Layer）的行为,1 强制使用 RDP标准加密（不尝试TLS/SSL），仅用于旧版服务器兼容性。0 禁用RDP标准加密，强制要求TLS/SSL（若服务器不支持，连接会失败）。

	// 性能优化标志：禁用壁纸、主题、菜单动画，保留完整窗口拖拽
	perfFlags := C.PERF_DISABLE_WALLPAPER /*桌面上的壁纸未显示*/ |
		C.PERF_DISABLE_THEMING /*主题处于禁用状态*/ |
		C.PERF_DISABLE_MENUANIMATIONS /*菜单动画已禁用*/ |
		0x00000010 /*TS_PERF_ENABLE_ENHANCED - 启用增强型图形*/ |
		0x00000020 /*TS_PERF_DISABLE_CURSOR_SHADOW - 光标不显示阴影*/ |
		0x00000040 /*TS_PERF_DISABLE_CURSORSETTINGS - 禁用光标闪烁*/ |
		0x00000080 /*TS_PERF_ENABLE_FONT_SMOOTHING - 启用字体平滑*/ |
		0x00000100 /*TS_PERF_ENABLE_DESKTOP_COMPOSITION - 启用桌面组合*/
	C.freerdp_settings_set_uint32(settings, C.FreeRDP_PerformanceFlags, C.UINT32(perfFlags))

	C.freerdp_settings_set_uint32(settings, C.FreeRDP_ConnectionType, 0x06)                    /*CONNECTION_TYPE_LAN (0x6) 局域网 (LAN) (10 Mbps 或更高)*/
	C.freerdp_settings_set_bool(settings, C.FreeRDP_RemoteFxCodec, C.TRUE)                     //指 RemoteFX 编解码器 的实现模块，主要用于处理微软 RemoteFX 技术中的视频和图像压缩/解压缩功能。以下是其详细作用和工作原理：
	C.freerdp_settings_set_bool(settings, C.FreeRDP_FastPathOutput, C.TRUE)                    //Fast-Path 是 RDP（远程桌面协议）的一种优化传输模式，与传统 Slow-Path（基于标准 T.124 协议）相比，它通过减少协议头开销和简化数据封装，显著提升数据传输效率。Fast-Path 常用于图形更新、输入事件等高频率操作。
	C.freerdp_settings_set_uint32(settings, C.FreeRDP_FrameAcknowledge, 2)                     //一个与远程桌面协议（RDP）图形渲染和帧确认机制相关的功能
	C.freerdp_settings_set_uint32(settings, C.FreeRDP_LargePointerFlag, 0x00000001|0x00000002) //用于控制远程桌面会话中鼠标指针的显示和处理方式
	C.freerdp_settings_set_uint32(settings, C.FreeRDP_GlyphSupportLevel, 0x00000000)           //GLYPH_SUPPORT_NONE 禁用字形缓存，所有文本直接作为位图传输。兼容性最强，但性能最低（带宽占用高）。
	C.freerdp_settings_set_bool(settings, C.FreeRDP_BitmapCacheV3Enabled, C.TRUE)              //一个配置选项，用于控制是否启用 Bitmap Cache Version 3（位图缓存V3） 功能，1：启用 Bitmap Cache V3（默认推荐，除非遇到兼容性问题）,0：禁用 Bitmap Cache V3，使用更旧的缓存机制。
	C.freerdp_settings_set_uint32(settings, C.FreeRDP_OffscreenSupportLevel, 0)                //0 - 完全禁用离屏表面支持。离屏表面是RDP协议中的一种特性，允许服务器将部分图形内容缓存到客户端的离屏缓冲区中，以便后续快速重用（如窗口拖动、动画渲染等）。

	// 启用 RDPEDISP 虚拟通道支持动态桌面分辨率调整
	// FreeRDP_SupportDisplayControl=5185，FreeRDP_DynamicResolutionUpdate=1558
	C.freerdp_settings_set_bool(settings, 5185, C.TRUE) //一个与动态显示分辨率调整相关的配置选项，主要用于控制客户端是否支持在远程会话过程中动态更改显示设置（如分辨率、方向等）
	C.freerdp_settings_set_bool(settings, 1558, C.TRUE) //一个与动态分辨率更新相关的配置选项，主要用于远程桌面会话期间根据客户端窗口大小变化自动调整服务器端的分辨率

	// 禁用 rdpdr 通道及其所有触发条件，避免因插件库不存在而产生 ERROR 日志。
	// freerdp_client_load_addins 中以下任一条件为 TRUE 时会强制开启 DeviceRedirection：
	//   NetworkAutoDetect / SupportHeartbeatPdu / SupportMultitransport (RDP8 特性)
	//   AudioPlayback (rdpsnd 依赖 rdpdr)
	// 因此需要全部禁用。
	C.freerdp_settings_set_bool(settings, 4160, C.FALSE) // FreeRDP_DeviceRedirection
	C.freerdp_settings_set_bool(settings, 137, C.FALSE)  // FreeRDP_NetworkAutoDetect 一个与网络自动检测（Network Auto-Detect）功能相关的配置选项
	C.freerdp_settings_set_bool(settings, 144, C.TRUE)   // FreeRDP_SupportHeartbeatPdu 一个配置选项，用于控制客户端与远程桌面服务器（如Windows远程桌面服务）之间的心跳机制, 0 禁用心跳机制（默认值），依赖底层TCP保活机制或应用层超时设置。
	C.freerdp_settings_set_bool(settings, 513, C.TRUE)   // FreeRDP_SupportMultitransport 一个配置选项，用于控制是否启用 RDP 多传输协议（Multitransport） 功能, 1 启用多传输支持（默认推荐，如果服务端支持）, 0 禁用多传输，仅使用传统 TCP 传输。
	C.freerdp_settings_set_bool(settings, 714, C.FALSE)  // FreeRDP_AudioPlayback 一个配置选项，用于控制客户端在远程桌面会话中的音频重定向（播放）行为
	C.freerdp_settings_set_bool(settings, 715, C.FALSE)  // FreeRDP_AudioCapture 一个配置选项，用于控制客户端是否启用音频捕获（即从客户端麦克风捕获音频并传输到远程服务器）

	return C.TRUE
}

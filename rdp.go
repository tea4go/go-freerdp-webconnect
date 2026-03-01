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

// Global disp context pointer (set when disp channel connects)
static DispClientContext* g_dispCtx = NULL;

static void onChannelConnected(void* context, const ChannelConnectedEventArgs* e) {
    if (strcmp(e->name, DISP_DVC_CHANNEL_NAME) == 0) {
        g_dispCtx = (DispClientContext*)e->pInterface;
    }
}

static void onChannelDisconnected(void* context, const ChannelDisconnectedEventArgs* e) {
    if (strcmp(e->name, DISP_DVC_CHANNEL_NAME) == 0) {
        g_dispCtx = NULL;
    }
}

static void registerChannelEvents(freerdp* instance) {
    wPubSub* pubSub = instance->context->pubSub;
    PubSub_SubscribeChannelConnected(pubSub, onChannelConnected);
    PubSub_SubscribeChannelDisconnected(pubSub, onChannelDisconnected);
}

// Map JavaScript keyCode (modifier keys only) to RDP scancode.
// modkeys in JS: [8, 16, 17, 18, 20, 144, 145]
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

static void sendMouseInput(freerdp* instance, UINT32 flags, UINT32 x, UINT32 y) {
    freerdp_input_send_mouse_event(instance->context->input, (UINT16)flags, (UINT16)x, (UINT16)y);
}

static void sendKupdownInput(freerdp* instance, UINT32 down, UINT32 keycode) {
    UINT32 scancode = jsKeyCodeToScancode(keycode);
    if (scancode == 0) return;
    freerdp_input_send_keyboard_event_ex(instance->context->input,
        down ? TRUE : FALSE, FALSE, scancode);
}

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

static void sendKpressInput(freerdp* instance, UINT32 charcode) {
    freerdp_input_send_unicode_keyboard_event(instance->context->input,
        0, (UINT16)charcode);
    freerdp_input_send_unicode_keyboard_event(instance->context->input,
        KBD_FLAGS_RELEASE, (UINT16)charcode);
}

// Helper function to convert color using FreeRDP 3.x API
static inline UINT32 convertColor(UINT32 color, UINT32 srcBpp, UINT32 dstBpp) {
    UINT32 srcFormat = (srcBpp == 16) ? PIXEL_FORMAT_RGB16 : PIXEL_FORMAT_BGRX32;
    UINT32 dstFormat = (dstBpp == 32) ? PIXEL_FORMAT_BGRX32 : PIXEL_FORMAT_RGB16;
    return FreeRDPConvertColor(color, srcFormat, dstFormat, NULL);
}

// Helper function to flip image data
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

// Helper function to get settings from instance
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

static BOOL cbPreConnect(freerdp* instance) {
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

	// Register channel connected/disconnected events to capture DispClientContext
	registerChannelEvents(instance);

	// Load disp dynamic virtual channel for dynamic resolution update
	const char* dispName = DISP_DVC_CHANNEL_NAME;
	freerdp_client_add_dynamic_channel(instance->context->settings, 1, &dispName);

	return preConnect(instance);
}

static BOOL cbPostConnect(freerdp* instance) {
	// gdi_init initializes the cache subsystem (including pointer cache).
	// Without it, context->cache is NULL and pointer updates crash.
	if (!gdi_init(instance, PIXEL_FORMAT_XRGB32))
		return FALSE;

	// Load channel addins (including disp) after connection is established
	freerdp_client_load_channels(instance);

	// Re-register our own update callbacks after gdi_init, which would
	// have overwritten them with GDI defaults.
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

static BITMAP_DATA* nextBitmapRectangle(BITMAP_UPDATE* bitmap, int i) {
	return &bitmap->rectangles[i];
}

static DELTA_RECT* nextMultiOpaqueRectangle(MULTI_OPAQUE_RECT_ORDER* moro, int i) {
	return &moro->rectangles[i];
}

static void bindCallbacks(freerdp* instance) {
	instance->PreConnect = cbPreConnect;
	instance->PostConnect = cbPostConnect;
}

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

const (
	WSOP_SC_BEGINPAINT       uint32 = 0
	WSOP_SC_ENDPAINT         uint32 = 1
	WSOP_SC_BITMAP           uint32 = 2
	WSOP_SC_OPAQUERECT       uint32 = 3
	WSOP_SC_SETBOUNDS        uint32 = 4
	WSOP_SC_PATBLT           uint32 = 5
	WSOP_SC_MULTI_OPAQUERECT uint32 = 6
	WSOP_SC_SCRBLT           uint32 = 7
	WSOP_SC_PTR_NEW          uint32 = 8
	WSOP_SC_PTR_FREE         uint32 = 9
	WSOP_SC_PTR_SET          uint32 = 10
	WSOP_SC_PTR_SETNULL      uint32 = 11
	WSOP_SC_PTR_SETDEFAULT   uint32 = 12
)

type bitmapUpdateMeta struct {
	op  uint32
	x   uint32
	y   uint32
	w   uint32
	h   uint32
	dw  uint32
	dh  uint32
	bpp uint32
	cf  uint32
	sz  uint32
}

type primaryPatBltMeta struct {
	op  uint32
	x   int32
	y   int32
	w   int32
	h   int32
	fg  uint32
	rop uint32
}

type primaryScrBltMeta struct {
	op  uint32
	rop uint32
	x   int32
	y   int32
	w   int32
	h   int32
	sx  int32
	sy  int32
}

// inputEvent carries one client input message from the WebSocket reader to rdpconnect.
// op=0 mouse: a=flags b=x c=y
// op=1 kupdown: a=down(1)/up(0) b=keycode
// op=2 kpress: a=modifiers(unused) b=charcode
type inputEvent struct {
	op uint32
	a  uint32
	b  uint32
	c  uint32
}

type rdpConnectionSettings struct {
	hostname *string
	username *string
	password *string
	width    int
	height   int
	port     int
}

// rdpContextData holds all Go-managed data for one RDP connection.
// It lives entirely in Go memory, so the GC can track it properly.
// We never store Go pointers inside C-allocated memory (CGo violation).
type rdpContextData struct {
	sendq    chan []byte
	recvq    chan []byte
	settings *rdpConnectionSettings
}

// Global registry: C rdpContext pointer value → Go data.
// The key is a uintptr (raw address), NOT a Go pointer, so it is safe
// to store in a Go map even though it came from C.
var (
	contextMu   sync.Mutex
	contextMap  = make(map[uintptr]*rdpContextData)
)

func registerCtx(ctx *C.rdpContext, d *rdpContextData) {
	contextMu.Lock()
	contextMap[uintptr(unsafe.Pointer(ctx))] = d
	contextMu.Unlock()
}

func unregisterCtx(ctx *C.rdpContext) {
	contextMu.Lock()
	delete(contextMap, uintptr(unsafe.Pointer(ctx)))
	contextMu.Unlock()
}

func lookupCtx(ctx *C.rdpContext) *rdpContextData {
	contextMu.Lock()
	d := contextMap[uintptr(unsafe.Pointer(ctx))]
	contextMu.Unlock()
	return d
}

func rdpconnect(sendq chan []byte, recvq chan []byte, inputq chan inputEvent, settings *rdpConnectionSettings) {
	// Lock the OS thread: FreeRDP's transport/TLS uses thread-local state,
	// so all FreeRDP calls must happen on the same OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	fmt.Println("RDP Connecting...")

	instance := C.freerdp_new()
	C.bindCallbacks(instance)

	// Use the native C rdpContext size — no extra Go fields in C memory.
	instance.ContextSize = C.size_t(C.sizeof_rdpContext)
	C.freerdp_context_new(instance)

	// Register Go-managed data in a global map keyed by the C context pointer.
	// This is the CGo-safe alternative to embedding Go pointers in C memory.
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
			fmt.Println("Disconnecting (websocket error)")
			mainEventLoop = false
		case ev := <-inputq:
			switch ev.op {
			case 0: // mouse
				C.sendMouseInput(instance, C.UINT32(ev.a), C.UINT32(ev.b), C.UINT32(ev.c))
			case 1: // key up/down (modifier keys only)
				C.sendKupdownInput(instance, C.UINT32(ev.a), C.UINT32(ev.b))
			case 2: // key press (unicode)
				C.sendKpressInput(instance, C.UINT32(ev.b))
			case 3: // desktop resize
				C.sendResizeInput(instance, C.UINT32(ev.a), C.UINT32(ev.b))
			}
		default:
			e := int(C.freerdp_error_info(instance))
			if e != 0 {
				switch e {
				case 1:
				case 2:
				case 7:
				case 9:
					// Manual disconnections and such
					fmt.Println("Disconnecting (manual)")
					mainEventLoop = false
				case 5:
					// Another user connected
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

func sendBinary(sendq chan []byte, data *bytes.Buffer) {
	sendq <- data.Bytes()
}

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

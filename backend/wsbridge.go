package backend

// WebSocket ⇄ RDP 桥接模块
// 职责：
// 1) 在本地 127.0.0.1 启动 WebSocket 服务
// 2) 从前端读取分辨率与输入事件
// 3) 通过 goroutine 与通道与 RDP 后端协作

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/net/websocket"
)

// pendingConnections 存储通过 token 注册的连接参数
var pendingConnections sync.Map

// RegisterConnection 注册一个待连接的 RDP 会话，返回 token
func RegisterConnection(settings *rdpConnectionSettings) string {
	token := fmt.Sprintf("%d", &settings)
	pendingConnections.Store(token, settings)
	return token
}

// getResolution 从 WebSocket 请求中解析客户端期望的分辨率。
// 返回值 width/height 会被限制在合理范围，避免异常参数导致后端资源浪费。
func getResolution(ws *websocket.Conn) (width int64, height int64) {
	request := ws.Request()               // 取出握手时的 HTTP 请求
	dtsize := request.FormValue("dtsize") // 形如 "1366x768" 的分辨率参数

	if !strings.Contains(dtsize, "x") {
		width = 800 // 无参数时采用安全默认值
		height = 600
	} else {
		sizeparts := strings.Split(dtsize, "x") // 拆分宽和高

		width, _ = strconv.ParseInt(sizeparts[0], 10, 32)  // 宽
		height, _ = strconv.ParseInt(sizeparts[1], 10, 32) // 高

		// 限定范围：既避免太小体验差，也避免太大占用过多资源
		if width < 400 {
			width = 400
		} else if width > 1920 {
			width = 1920
		}

		if height < 300 {
			height = 300
		} else if height > 1080 {
			height = 1080
		}
	}

	return width, height
}

func parseBoolParam(v string, def bool) bool {
	if v == "" {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "on", "yes":
		return true
	case "0", "false", "off", "no":
		return false
	default:
		return def
	}
}

// processSendQ 将后端 RDP 产生的帧数据通过 WebSocket 推送给浏览器。
// 一旦发送失败，尝试向 recvq 写入结束信号以驱动清理流程。
func processSendQ(ws *websocket.Conn, sendq chan []byte, recvq chan []byte) {
	for {
		buf := <-sendq                         // 阻塞等待待发送数据
		err := websocket.Message.Send(ws, buf) // 通过 WS 发送给前端
		if err != nil {
			select { // 非阻塞通知：下游可据此中断会话
			case recvq <- []byte("1"):
			default:
			}
			return
		}
	}
}

// initSocket 是 /ws 的连接入口：
// 支持两种模式：
// 1) token 模式：通过 URL 参数 token 查找预注册的连接参数（Wails 桌面模式）
// 2) 直接参数模式：通过 URL 参数 host/user/pass/port 传入（兼容旧模式）
func initSocket(ws *websocket.Conn) {
	sendq := make(chan []byte, 100) // 发送到浏览器的数据队列（带缓冲）
	recvq := make(chan []byte, 5)   // 接收控制信号的队列（如断开通知）

	width, height := getResolution(ws)
	fmt.Printf("User requested size %d x %d\n", width, height)

	req := ws.Request()
	var settings *rdpConnectionSettings

	// 优先使用 token 查找预注册的连接参数
	if token := req.FormValue("token"); token != "" {
		if val, ok := pendingConnections.LoadAndDelete(token); ok {
			settings = val.(*rdpConnectionSettings)
			// 使用前端传入的分辨率覆盖
			settings.width = int(width)
			settings.height = int(height)
		}
	}

	// token 未命中时回退到 URL 参数模式
	if settings == nil {
		host := req.FormValue("host")
		user := req.FormValue("user")
		pass := req.FormValue("pass")
		port := 3389
		if portStr := req.FormValue("port"); portStr != "" {
			if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
				port = p
			}
		}

		if host == "" {
			fmt.Println("Missing host parameter")
			return
		}

		settings = &rdpConnectionSettings{
			hostname: &host,
			username: &user,
			password: &pass,
			width:    int(width),
			height:   int(height),
			port:     port,
			perf:     0,
			fntlm:    0,
			nowallp:  parseBoolParam(req.FormValue("nowallp"), false),
			nowdrag:  parseBoolParam(req.FormValue("nowdrag"), false),
			nomani:   parseBoolParam(req.FormValue("nomani"), false),
			notheme:  parseBoolParam(req.FormValue("notheme"), false),
			nonla:    parseBoolParam(req.FormValue("nonla"), false),
			notls:    parseBoolParam(req.FormValue("notls"), false),
		}
		if perfStr := req.FormValue("perf"); perfStr != "" {
			if p, err := strconv.Atoi(perfStr); err == nil && p >= 0 && p <= 2 {
				settings.perf = p
			}
		}
		if fntlmStr := req.FormValue("fntlm"); fntlmStr != "" {
			if n, err := strconv.Atoi(fntlmStr); err == nil && n >= 0 && n <= 2 {
				settings.fntlm = n
			}
		}
	}

	fmt.Printf("Connecting to %s:%d as %s\n", *settings.hostname, settings.port, *settings.username)

	inputq := make(chan inputEvent, 50)           // 浏览器输入事件队列
	go rdpconnect(sendq, recvq, inputq, settings) // 后端：建立并维护 RDP 连接
	go processSendQ(ws, sendq, recvq)             // 前端：推送图像/数据给浏览器

	read := make([]byte, 1024) // 复用缓冲区承接浏览器发来的事件数据
	for {
		n, err := ws.Read(read) // 读取一条来自浏览器的二进制事件
		if err != nil {
			recvq <- []byte("1") // 读取出错，通知后端收尾
			return
		}
		if n >= 12 { // 至少包含 op/a/b 三个 32 位字段
			op := binary.LittleEndian.Uint32(read[0:4]) // 操作类型
			a := binary.LittleEndian.Uint32(read[4:8])  // 参数 a
			b := binary.LittleEndian.Uint32(read[8:12]) // 参数 b
			var c uint32
			if n >= 16 { // 可选的参数 c
				c = binary.LittleEndian.Uint32(read[12:16])
			}
			select {
			case inputq <- inputEvent{op, a, b, c}: // 投递到输入事件队列
			default:
			}
		}
	}
}

// StartWSBridge 在指定地址启动本地 WebSocket 桥接服务
// 返回实际监听的端口号
func StartWSBridge(listenAddr, appVersion string) (int, error) {
	mux := http.NewServeMux()

	// WebSocket 入口
	mux.Handle("/ws", websocket.Handler(initSocket))

	// 版本信息接口
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"app":     appVersion,
			"freerdp": GetFreeRDPVersion(),
		})
	})

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return 0, fmt.Errorf("WebSocket bridge listen failed: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	fmt.Printf("WebSocket bridge listening on 127.0.0.1:%d\n", port)

	go http.Serve(listener, mux)

	return port, nil
}

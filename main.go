package main

// WebSocket ⇄ RDP 网关主程序
// 职责：
// 1) 从浏览器读取期望分辨率与输入事件
// 2) 通过 goroutine 与通道与 RDP 后端协作
// 3) 暴露 /ws 接口并提供静态页面

import (
	"encoding/binary"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	logs "github.com/tea4go/gh/log4go"
	"github.com/tea4go/gh/network"
	"golang.org/x/net/websocket"
)

var (
	rdpHost    string // 目标 RDP 主机地址
	rdpUser    string // 登录用户名
	rdpPass    string // 登录密码
	rdpPort    int    // RDP 服务端口
	listenPort int    // 本地 HTTP 监听端口
)

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
// - 准备收发通道与输入事件队列
// - 读取分辨率与 RDP 连接参数（优先使用 URL 查询参数，缺省时回退到命令行参数）
// - 启动 RDP 后端与发送协程
// - 循环读取来自浏览器的输入事件，转为 inputEvent 投递给 RDP
func initSocket(ws *websocket.Conn) {
	sendq := make(chan []byte, 100) // 发送到浏览器的数据队列（带缓冲）
	recvq := make(chan []byte, 5)   // 接收控制信号的队列（如断开通知）

	width, height := getResolution(ws)
	fmt.Printf("User requested size %d x %d\n", width, height)

	// 优先使用浏览器表单通过 URL 参数传入的凭据，缺省时回退到命令行参数
	req := ws.Request()
	host := req.FormValue("host")
	if host == "" {
		host = rdpHost
	}
	user := req.FormValue("user")
	if user == "" {
		user = rdpUser
	}
	pass := req.FormValue("pass")
	if pass == "" {
		pass = rdpPass
	}
	port := rdpPort
	if portStr := req.FormValue("port"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
			port = p
		}
	}

	fmt.Printf("Connecting to %s:%d as %s\n", host, port, user)

	settings := &rdpConnectionSettings{
		&host,       // 目标主机
		&user,       // 用户名
		&pass,       // 密码
		int(width),  // 分辨率宽
		int(height), // 分辨率高
		port,        // RDP 端口
	}

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

// 标准程序块
var appName string = "gofreerdp" // 应用名称
var appVer string = "0.0.2"      // 应用版本
var IsBeta string                // 是否为 Beta 版本，由构建注入
var BuildTime string             // 构建时间，由构建注入

// main 是程序入口，负责解析命令行参数、初始化日志与自更新、启动 HTTP 服务。
func main() {
	pflag.StringVarP(&rdpHost, "host", "", "10.88.16.102", "远程桌面服务器地址")
	pflag.IntVarP(&rdpPort, "port", "", 53389, "远程桌面服务器端口")
	pflag.StringVarP(&rdpUser, "user", "", "administrator", "用户名")
	pflag.StringVarP(&rdpPass, "pass", "", "", "密码（可选，也可在网页表单中填写）")
	pflag.IntVarP(&listenPort, "listen", "", 54455, "HTTP 监听端口")
	pflag.Parse()

	// 标准程序块
	network.SetAppVersion(appName, appVer, IsBeta, BuildTime) // 设置应用版本号，便于自动更新
	logs.StartLogger(appName)                                 // 初始化日志系统
	network.StartSelfUpdate()                                 // 后台自更新（如存在）

	runtime.GOMAXPROCS(runtime.NumCPU()) // 充分利用多核

	// WebSocket 入口：浏览器通过 /ws 建立连接
	http.Handle("/ws", websocket.Handler(initSocket))

	// 静态文件服务：禁用缓存确保浏览器始终获取最新页面
	noCacheFS := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		http.FileServer(http.Dir("webroot")).ServeHTTP(w, r)
	})
	http.Handle("/", noCacheFS)

	fmt.Printf("请访问: http://localhost:%d/index-debug.html\n", listenPort)
	err := http.ListenAndServe(fmt.Sprintf(":%d", listenPort), nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error()) // 启动失败直接崩溃并输出原因
	}
}

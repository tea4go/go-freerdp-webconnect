package main

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
	rdpHost    string
	rdpUser    string
	rdpPass    string
	rdpPort    int
	listenPort int
)

func getResolution(ws *websocket.Conn) (width int64, height int64) {
	request := ws.Request()
	dtsize := request.FormValue("dtsize")

	if !strings.Contains(dtsize, "x") {
		width = 800
		height = 600
	} else {
		sizeparts := strings.Split(dtsize, "x")

		width, _ = strconv.ParseInt(sizeparts[0], 10, 32)
		height, _ = strconv.ParseInt(sizeparts[1], 10, 32)

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

func processSendQ(ws *websocket.Conn, sendq chan []byte, recvq chan []byte) {
	for {
		buf := <-sendq
		err := websocket.Message.Send(ws, buf)
		if err != nil {
			select {
			case recvq <- []byte("1"):
			default:
			}
			return
		}
	}
}

func initSocket(ws *websocket.Conn) {
	sendq := make(chan []byte, 100)
	recvq := make(chan []byte, 5)

	width, height := getResolution(ws)
	fmt.Printf("User requested size %d x %d\n", width, height)

	host := rdpHost
	user := rdpUser
	pass := rdpPass
	port := rdpPort

	fmt.Printf("Connecting to %s:%d as %s\n", host, port, user)

	settings := &rdpConnectionSettings{
		&host,
		&user,
		&pass,
		int(width),
		int(height),
		port,
	}

	inputq := make(chan inputEvent, 50)
	go rdpconnect(sendq, recvq, inputq, settings)
	go processSendQ(ws, sendq, recvq)

	read := make([]byte, 1024)
	for {
		n, err := ws.Read(read)
		if err != nil {
			recvq <- []byte("1")
			return
		}
		if n >= 12 {
			op := binary.LittleEndian.Uint32(read[0:4])
			a := binary.LittleEndian.Uint32(read[4:8])
			b := binary.LittleEndian.Uint32(read[8:12])
			var c uint32
			if n >= 16 {
				c = binary.LittleEndian.Uint32(read[12:16])
			}
			select {
			case inputq <- inputEvent{op, a, b, c}:
			default:
			}
		}
	}
}

// 标准程序块
var appName string = "gofreerdp"
var appVer string = "0.0.2"
var IsBeta string
var BuildTime string

func main() {
	pflag.StringVarP(&rdpHost, "host", "", "10.88.16.102", "远程桌面服务器地址")
	pflag.IntVarP(&rdpPort, "port", "", 53389, "远程桌面服务器端口")
	pflag.StringVarP(&rdpUser, "user", "", "administrator", "用户名")
	pflag.StringVarP(&rdpPass, "pass", "", "", "密码")
	pflag.IntVarP(&listenPort, "listen", "", 54455, "HTTP 监听端口")
	pflag.Parse()

	if rdpHost == "" || rdpPass == "" {
		fmt.Printf("用法: %s [选项]\n\n选项:\n", appName)
		pflag.PrintDefaults()
		return
	}

	// 标准程序块
	network.SetAppVersion(appName, appVer, IsBeta, BuildTime) //设置应用版本号，便于自动更新
	logs.StartLogger(appName)
	network.StartSelfUpdate()

	runtime.GOMAXPROCS(runtime.NumCPU())

	// WebSocket handler for RDP connection
	http.Handle("/ws", websocket.Handler(initSocket))

	// Static file server for webroot
	http.Handle("/", http.FileServer(http.Dir("webroot")))

	fmt.Printf("请访问: http://localhost:%d/index-debug.html\n", listenPort)
	err := http.ListenAndServe(fmt.Sprintf(":%d", listenPort), nil)
	if err != nil {
		panic("ListenANdServe: " + err.Error())
	}
}

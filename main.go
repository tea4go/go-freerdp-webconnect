package main

// Wails 桌面应用入口
// 启动 Wails 窗口 + 本地 WebSocket 桥接

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gofreerdp/backend"

	logs "github.com/tea4go/gh/log4go"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed frontend/dist
var assets embed.FS

// 标准程序块
var appName string = "gofreerdp" // 应用名称
var appVer string = "0.0.2"      // 应用版本
var IsBeta string                // 是否为 Beta 版本，由构建注入
var BuildTime string             // 构建时间，由构建注入

func filepathJoin(elem ...string) string {
	path := filepath.Join(elem...)
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(path, "\\", "/")
	}
	return path
}

func main() {

	//#region 处理日志
	logs_file_name := filepathJoin(os.TempDir(), "ulog_qrdp.log")
	logs.SetLogger("file", fmt.Sprintf(`{"filename":"%s", "perm": "0666"}`, logs_file_name))
	logs.Notice("Start Application - %s Build:%s", appVer, BuildTime)
	logs.StartLogger()

	logs.Notice("= 日志级别 = console %s", logs.GetLevelName(logs.GetLevel("console")))
	logs.Notice("= 日志级别 = file    %s", logs.GetLevelName(logs.GetLevel("file")))
	logs.Notice("= 日志文件 = %s", logs_file_name)

	app := backend.NewApp(appVer)

	err := wails.Run(&options.App{
		Title:  "QRDP",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.Startup,
		OnShutdown: app.Shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

package backend

// Wails 绑定对象
// 提供给前端调用的 Go 方法

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 是 Wails 绑定对象，暴露给前端 JS 调用
type App struct {
	ctx        context.Context
	wsPort     int32 // WebSocket 桥接端口（atomic）
	appVersion string
}

// NewApp 创建 App 实例
func NewApp(version string) *App {
	return &App{appVersion: version}
}

// Startup 在 Wails 应用启动时调用
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	// 启动本地 WebSocket 桥接服务（随机端口）
	port, err := StartWSBridge("127.0.0.1:0", a.appVersion)
	if err != nil {
		fmt.Println("Failed to start WebSocket bridge:", err)
		return
	}
	atomic.StoreInt32(&a.wsPort, int32(port))
}

// Shutdown 在 Wails 应用关闭时调用
func (a *App) Shutdown(ctx context.Context) {
	fmt.Println("Application shutting down")
}

// Connect 由前端调用，注册 RDP 连接参数并返回 WebSocket 地址
func (a *App) Connect(
	host, user, pass string,
	port, width, height int,
	perf, fntlm int,
	nowallp, nowdrag, nomani, notheme, nonla, notls bool,
) string {
	settings := &rdpConnectionSettings{
		hostname: &host,
		username: &user,
		password: &pass,
		width:    width,
		height:   height,
		port:     port,
		perf:     perf,
		fntlm:    fntlm,
		nowallp:  nowallp,
		nowdrag:  nowdrag,
		nomani:   nomani,
		notheme:  notheme,
		nonla:    nonla,
		notls:    notls,
	}

	token := RegisterConnection(settings)
	wsPort := atomic.LoadInt32(&a.wsPort)

	return fmt.Sprintf("ws://127.0.0.1:%d/ws?token=%s&dtsize=%dx%d", wsPort, token, width, height)
}

// GetVersion 返回应用版本和 FreeRDP 版本
func (a *App) GetVersion() map[string]string {
	return map[string]string{
		"app":     a.appVersion,
		"freerdp": GetFreeRDPVersion(),
	}
}

// SaveFile 弹出系统保存文件对话框，将内容写入用户选择的路径
func (a *App) SaveFile(defaultFilename string, content string) error {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: defaultFilename,
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON 文件 (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return err
	}
	if path == "" {
		return nil // 用户取消
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// OpenFile 弹出系统打开文件对话框，读取并返回文件内容
func (a *App) OpenFile() (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON 文件 (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil // 用户取消
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

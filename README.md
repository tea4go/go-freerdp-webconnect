# go-freerdp-webconnect

基于 [FreeRDP-WebConnect](https://github.com/FreeRDP/FreeRDP-WebConnect) 的 Go 版本实现，使用 `cgo` 调用 `libfreerdp`，并通过 Wails 提供桌面端 UI。

## 多平台编译说明

### 1. 通用要求

- Go（建议 `1.24+`）
- Git
- 仅在使用 Wails 开发/打包时需要：Node.js `20.19+` 或 `22.12+`、npm、Wails CLI

### 2. Linux

首次构建（自动安装依赖 + 编译 FreeRDP + 编译 Go）：

```bash
./lib_build_linux.sh --auto-install
```

常用参数：

- `--skip-deps`：跳过依赖安装
- `--skip-freerdp`：跳过 FreeRDP 编译
- `--force-freerdp`：强制重编 FreeRDP

产物：

- `./go-freerdp-webconnect`
- 本地 FreeRDP 安装目录：`./install`

### 3. macOS

先安装依赖：

```bash
brew install go freerdp
```

然后构建：

```bash
./lib_build_macos.sh
```

产物：

- `./go-freerdp-webconnect`

### 4. Windows（MSYS2 MinGW64）

推荐环境：

- MSYS2 + MinGW64（gcc/cmake/mingw32-make）
- Go（可在 `cmd` 或 PowerShell 中使用）

构建命令（在项目根目录）：

```bat
lib_build_windows.cmd
```

可选参数：

- `--skip-freerdp`
- `--force-freerdp`
- `--no-clone`

默认 MSYS2 路径为 `C:\DevDisk\DevTools\msys64`。如不同，请设置环境变量：

```bat
set MSYS64_ROOT=D:\path\to\msys64
```

产物：

- `gofreerdp-windows.exe`
- `install\bin\*.dll`（运行时依赖）

## Wails 三端脚本

在执行 Wails 脚本前，请先完成对应平台的基础构建（确保 FreeRDP 库已就绪）。

开发模式：

- Linux：`./wails_dev_linux.sh`
- macOS：`./wails_dev_macos.sh`
- Windows：`wails_dev_windows.cmd`

打包构建：

- Linux：`./wails_build_linux.sh`
- macOS：`./wails_build_macos.sh`
- Windows：`wails_build_windows.cmd`

## 相关文档

- macOS 说明：`README_macOS.md`
- Windows 详细说明：`README_windows.md`

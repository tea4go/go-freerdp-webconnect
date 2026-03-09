# Go-FreeRDP-WebConnect macOS 适配说明

## 适配概述

本项目已成功适配 macOS，可以在 macOS 上编译和运行。

## 与原 Ubuntu 版本的区别

### 1. 依赖安装方式
- **Ubuntu**: 使用 `apt-get` 安装依赖包
- **macOS**: 使用 `Homebrew` 安装 FreeRDP

### 2. FreeRDP 路径
- **Ubuntu**: 从源码编译，安装到 `./install` 目录
- **macOS**: 使用 Homebrew 预编译包，路径为 `/usr/local/opt/freerdp`

### 3. CGO 配置
修改了 `rdp.go` 中的 CGO 指令，添加平台区分：

```go
#cgo darwin CFLAGS: -I/usr/local/opt/freerdp/include/freerdp3 -I/usr/local/opt/freerdp/include/winpr3
#cgo darwin LDFLAGS: -L/usr/local/opt/freerdp/lib -lfreerdp3 -lfreerdp-client3 -lwinpr3
#cgo linux CFLAGS: -I${SRCDIR}/install/include/freerdp3 -I${SRCDIR}/install/include/winpr3
#cgo linux LDFLAGS: -L${SRCDIR}/install/lib -lfreerdp3 -lfreerdp-client3 -lwinpr3
```

### 4. 库路径设置
- **Ubuntu**: 使用 `LD_LIBRARY_PATH`
- **macOS**: 使用 `DYLD_LIBRARY_PATH`

## 文件变更

### 修改的文件
- `rdp.go`: 添加 macOS CGO 配置

### 新增的文件
- `build_macos.sh`: macOS 构建脚本
- `run_macos.sh`: macOS 运行脚本
- `test_macos.sh`: macOS 测试脚本
- `README_macOS.md`: 本文档

## 使用方法

### 快速开始

1. **克隆仓库**
   ```bash
   git clone https://github.com/tea4go/go-freerdp-webconnect.git
   cd go-freerdp-webconnect
   ```

2. **构建**
   ```bash
   ./build_macos.sh
   ```

3. **测试**
   ```bash
   ./test_macos.sh
   ```

4. **运行**
   ```bash
   ./run_macos.sh -h <RDP服务器地址> -u <用户名> -p <密码>
   ```

   示例：
   ```bash
   ./run_macos.sh -h 192.168.1.100 -u administrator -p mypassword
   ```

5. **访问 Web 界面**
   
   打开浏览器访问：
   ```
   http://localhost:54455/index-debug.html
   ```

### 手动构建

如果自动构建脚本不能满足需求，可以手动构建：

```bash
# 安装 FreeRDP
brew install freerdp

# 下载依赖
go mod tidy

# 编译
go build -o go-freerdp-webconnect

# 运行
export DYLD_LIBRARY_PATH=/usr/local/opt/freerdp/lib:$DYLD_LIBRARY_PATH
./go-freerdp-webconnect --host=<服务器地址> --user=<用户名> --pass=<密码>
```

## 系统要求

- macOS 10.15 (Catalina) 或更高版本
- Homebrew
- Go 1.21 或更高版本

## 已知问题

1. **废弃函数警告**: 编译时会出现 `freerdp_shall_disconnect` 已废弃的警告，这是 FreeRDP 3.x 的 API 变更导致的，不影响程序运行。

2. **X11 依赖**: FreeRDP 依赖 X11 (XQuartz)，如果需要显示 RDP 窗口（不只是 Web 查看器），需要安装 XQuartz：
   ```bash
   brew install --cask xquartz
   ```

## 故障排除

### 问题: 找不到 libfreerdp3.dylib
**解决**: 确保设置了 `DYLD_LIBRARY_PATH` 环境变量：
```bash
export DYLD_LIBRARY_PATH=/usr/local/opt/freerdp/lib:$DYLD_LIBRARY_PATH
```

### 问题: 端口被占用
**解决**: 使用 `--listen` 参数指定其他端口：
```bash
./go-freerdp-webconnect --listen=55555
```

### 问题: 无法连接到 RDP 服务器
**解决**: 
1. 检查 RDP 服务器地址和端口是否正确
2. 检查用户名和密码是否正确
3. 检查网络连接是否正常
4. 检查 RDP 服务器是否允许远程连接

## 技术细节

### FreeRDP 版本
- Homebrew 安装的 FreeRDP 版本: 3.15.0
- 与代码中使用的 FreeRDP 3.x API 兼容

### 编译参数
- CGO 启用（用于调用 FreeRDP C 库）
- 使用动态链接库（.dylib）

## 贡献

如果你发现任何问题或有改进建议，欢迎提交 Issue 或 Pull Request。

## 许可证

与原项目保持一致。

#!/bin/bash

# Go-FreeRDP-WebConnect macOS 构建脚本
# 替代原项目的 Ubuntu 构建脚本

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}=== Go-FreeRDP-WebConnect macOS 构建脚本 ===${NC}"
echo ""

# 项目根目录
PROJECT_ROOT=$(cd "$(dirname "$0")" && pwd)
cd "$PROJECT_ROOT"

# 1. 检查依赖
echo -e "${YELLOW}[1/3] 检查系统依赖...${NC}"

# 检查 Homebrew
if ! command -v brew &> /dev/null; then
    echo -e "${RED}错误: Homebrew 未安装${NC}"
    echo "请访问 https://brew.sh 安装 Homebrew"
    exit 1
fi

# 检查 Go
if ! command -v go &> /dev/null; then
    echo -e "${RED}错误: Go 未安装${NC}"
    echo "请运行: brew install go"
    exit 1
fi

echo -e "${GREEN}✓ 系统依赖检查完成${NC}"

# 2. 安装 FreeRDP
echo ""
echo -e "${YELLOW}[2/3] 检查 FreeRDP...${NC}"

if brew list freerdp &> /dev/null; then
    echo -e "${GREEN}✓ FreeRDP 已安装${NC}"
    FREERDP_VERSION=$(brew list --versions freerdp | awk '{print $2}')
    echo "  版本: $FREERDP_VERSION"
else
    echo "FreeRDP 未安装，正在安装..."
    brew install freerdp
    echo -e "${GREEN}✓ FreeRDP 安装完成${NC}"
fi

# 3. 编译 Go 项目
echo ""
echo -e "${YELLOW}[3/3] 编译 Go 项目...${NC}"

echo "下载 Go 依赖..."
go mod tidy

echo "编译..."
go build -o go-freerdp-webconnect

echo -e "${GREEN}✓ 编译完成${NC}"

echo ""
echo -e "${GREEN}=== 构建成功! ===${NC}"
echo ""
echo -e "可执行文件: ${GREEN}./go-freerdp-webconnect${NC}"
echo ""
echo "运行前请确保设置库路径:"
echo -e "  ${YELLOW}export DYLD_LIBRARY_PATH=/usr/local/opt/freerdp/lib:\$DYLD_LIBRARY_PATH${NC}"
echo ""
echo "Wails 开发启动:"
echo -e "  ${YELLOW}./wails_dev_macos.sh${NC}"
echo ""
echo "Wails 打包:"
echo -e "  ${YELLOW}./wails_build_macos.sh${NC}"

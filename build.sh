#!/bin/bash
set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}=== Go-FreeRDP-WebConnect 编译脚本 ===${NC}"

# 项目根目录
PROJECT_ROOT=$(pwd)
FREERDP_SRC="${PROJECT_ROOT}/src/FreeRDP"
FREERDP_BUILD="${PROJECT_ROOT}/build/freerdp"
FREERDP_INSTALL="${PROJECT_ROOT}/install"

# 1. 检查依赖
echo -e "\n${YELLOW}[1/5] 检查系统依赖...${NC}"
check_command() {
    if ! command -v $1 &> /dev/null; then
        echo -e "${RED}错误: $1 未安装${NC}"
        echo "请运行: sudo apt-get install $2"
        exit 1
    fi
}

check_command cmake cmake
check_command gcc build-essential
check_command pkg-config pkg-config
check_command go golang

echo -e "${GREEN}✓ 系统依赖检查完成${NC}"

# 2. 安装 FreeRDP 编译依赖
echo -e "\n${YELLOW}[2/5] 检查 FreeRDP 编译依赖...${NC}"
echo "需要安装以下依赖包："
echo "  - libssl-dev"
echo "  - libx11-dev"
echo "  - libxext-dev"
echo "  - libxinerama-dev"
echo "  - libxcursor-dev"
echo "  - libxdamage-dev"
echo "  - libxv-dev"
echo "  - libxkbfile-dev"
echo "  - libasound2-dev"
echo "  - libcups2-dev"
echo "  - libxml2-dev"
echo "  - libxrandr-dev"
echo "  - libgstreamer1.0-dev"
echo "  - libgstreamer-plugins-base1.0-dev"
echo "  - libxi-dev"
echo "  - libxtst-dev"
echo "  - libgtk-3-dev"
echo "  - libgcrypt20-dev"
echo "  - libpulse-dev"
echo "  - libusb-1.0-0-dev"
echo "  - libudev-dev"
echo "  - libdbus-glib-1-dev"
echo "  - uuid-dev"
echo "  - libxkbcommon-dev"
echo "  - libopus-dev"

# 检查是否有 --auto-install 参数
AUTO_INSTALL=false
if [[ "$1" == "--auto-install" ]] || [[ "$AUTO_INSTALL_DEPS" == "1" ]]; then
    AUTO_INSTALL=true
fi

if [ "$AUTO_INSTALL" = true ]; then
    echo -e "${GREEN}自动安装依赖...${NC}"
    sudo apt-get update
    sudo apt-get install -y \
        libssl-dev libx11-dev libxext-dev libxinerama-dev \
        libxcursor-dev libxdamage-dev libxv-dev libxkbfile-dev \
        libasound2-dev libcups2-dev libxml2-dev libxrandr-dev \
        libgstreamer1.0-dev libgstreamer-plugins-base1.0-dev \
        libxi-dev libxtst-dev zlib1g-dev \
        libgtk-3-dev libgcrypt20-dev libpulse-dev \
        libusb-1.0-0-dev libudev-dev libdbus-glib-1-dev \
        uuid-dev libxkbcommon-dev libopus-dev
else
    read -p "是否自动安装这些依赖? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        sudo apt-get update
        sudo apt-get install -y \
            libssl-dev libx11-dev libxext-dev libxinerama-dev \
            libxcursor-dev libxdamage-dev libxv-dev libxkbfile-dev \
            libasound2-dev libcups2-dev libxml2-dev libxrandr-dev \
            libgstreamer1.0-dev libgstreamer-plugins-base1.0-dev \
            libxi-dev libxtst-dev zlib1g-dev \
            libgtk-3-dev libgcrypt20-dev libpulse-dev \
            libusb-1.0-0-dev libudev-dev libdbus-glib-1-dev \
            uuid-dev libxkbcommon-dev libopus-dev
    fi
fi

# 3. 编译 FreeRDP
echo -e "\n${YELLOW}[3/5] 编译 FreeRDP...${NC}"
mkdir -p "${FREERDP_BUILD}"
mkdir -p "${FREERDP_INSTALL}"

# 检测系统 zlib 共享库路径，避免被 /opt 下第三方软件的私有 libz.a 干扰
SYS_ZLIB_LIB=$(ldconfig -p 2>/dev/null | awk '/libz\.so / {print $NF}' | head -1)
if [ -z "${SYS_ZLIB_LIB}" ]; then
    # 回退到常见路径
    for f in /usr/lib/x86_64-linux-gnu/libz.so \
              /usr/lib/aarch64-linux-gnu/libz.so \
              /usr/lib/libz.so; do
        if [ -f "$f" ]; then SYS_ZLIB_LIB="$f"; break; fi
    done
fi
echo -e "${GREEN}使用系统 zlib: ${SYS_ZLIB_LIB}${NC}"

cd "${FREERDP_BUILD}"
cmake "${FREERDP_SRC}" \
    -DCMAKE_INSTALL_PREFIX="${FREERDP_INSTALL}" \
    -DCMAKE_BUILD_TYPE=Release \
    -DZLIB_LIBRARY="${SYS_ZLIB_LIB}" \
    -DZLIB_INCLUDE_DIR=/usr/include \
    -DWITH_SSE2=ON \
    -DWITH_CUPS=OFF \
    -DWITH_WAYLAND=OFF \
    -DWITH_PULSE=OFF \
    -DWITH_FFMPEG=OFF \
    -DWITH_SWSCALE=OFF \
    -DWITH_DSP_FFMPEG=OFF \
    -DWITH_FUSE=OFF \
    -DWITH_GSTREAMER_1_0=OFF \
    -DWITH_CLIENT=OFF \
    -DWITH_SERVER=OFF \
    -DBUILD_TESTING=OFF \
    -DCHANNEL_URBDRC=OFF

make -j$(nproc)
make install

echo -e "${GREEN}✓ FreeRDP 编译完成${NC}"

# 4. 配置 Go 模块
echo -e "\n${YELLOW}[4/5] 配置 Go 依赖...${NC}"
cd "${PROJECT_ROOT}"
go mod tidy
echo -e "${GREEN}✓ Go 依赖下载完成${NC}"

# 5. 编译 Go 项目
echo -e "\n${YELLOW}[5/5] 编译 Go 项目...${NC}"

# 设置 CGO 环境变量
export CGO_CFLAGS="-I${FREERDP_INSTALL}/include"
export CGO_LDFLAGS="-L${FREERDP_INSTALL}/lib -L${FREERDP_INSTALL}/lib/x86_64-linux-gnu -lfreerdp3 -lfreerdp-client3 -lwinpr3"
export LD_LIBRARY_PATH="${FREERDP_INSTALL}/lib:${FREERDP_INSTALL}/lib/x86_64-linux-gnu:${LD_LIBRARY_PATH}"

# 编译
go build -o go-freerdp-webconnect

echo -e "\n${GREEN}=== 编译成功! ===${NC}"
echo -e "可执行文件: ${GREEN}./go-freerdp-webconnect${NC}"
echo -e "\n运行前请设置环境变量:"
echo -e "  ${YELLOW}export LD_LIBRARY_PATH=${FREERDP_INSTALL}/lib:${FREERDP_INSTALL}/lib/x86_64-linux-gnu:\$LD_LIBRARY_PATH${NC}"
echo -e "\n或使用运行脚本:"
echo -e "  ${YELLOW}./run.sh -h <hostname> -u <username> -p <password>${NC}"

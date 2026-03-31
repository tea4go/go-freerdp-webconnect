#!/usr/bin/env bash
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FREERDP_SRC="${PROJECT_ROOT}/src/FreeRDP"
FREERDP_BUILD="${PROJECT_ROOT}/build/freerdp-linux"
FREERDP_INSTALL="${PROJECT_ROOT}/install"
FREERDP_TAG="${FREERDP_TAG:-3.12.0}"

AUTO_INSTALL=false
SKIP_DEPS=false
SKIP_FREERDP=false
FORCE_FREERDP=false

usage() {
    cat <<'EOF'
用法: ./lib_build_linux.sh [选项]
  --auto-install   自动 apt 安装依赖
  --skip-deps      跳过 apt 依赖安装提示/安装
  --skip-freerdp   跳过 FreeRDP 编译
  --force-freerdp  强制重新编译 FreeRDP
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --auto-install) AUTO_INSTALL=true; shift ;;
        --skip-deps) SKIP_DEPS=true; shift ;;
        --skip-freerdp) SKIP_FREERDP=true; shift ;;
        --force-freerdp) FORCE_FREERDP=true; shift ;;
        -h|--help) usage; exit 0 ;;
        *)
            echo -e "${RED}未知参数: $1${NC}"
            usage
            exit 1
            ;;
    esac
done

check_command() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo -e "${RED}错误: 缺少命令 $1${NC}"
        exit 1
    fi
}

echo -e "${GREEN}=== Linux 构建脚本 ===${NC}"
echo "项目目录: ${PROJECT_ROOT}"

echo -e "\n${YELLOW}[1/5] 检查基础工具...${NC}"
check_command cmake
check_command gcc
check_command pkg-config
check_command go
check_command git
echo -e "${GREEN}✓ 基础工具检查完成${NC}"

echo -e "\n${YELLOW}[2/5] 处理 Linux 依赖...${NC}"
APT_DEPS=(
    libssl-dev libx11-dev libxext-dev libxinerama-dev libxcursor-dev
    libxdamage-dev libxv-dev libxkbfile-dev libasound2-dev libcups2-dev
    libxml2-dev libxrandr-dev libgstreamer1.0-dev
    libgstreamer-plugins-base1.0-dev libxi-dev libxtst-dev zlib1g-dev
    libgtk-3-dev libgcrypt20-dev libpulse-dev libusb-1.0-0-dev libudev-dev
    libdbus-glib-1-dev uuid-dev libxkbcommon-dev libopus-dev
)

if [[ "${SKIP_DEPS}" == true ]]; then
    echo -e "${YELLOW}跳过依赖处理${NC}"
elif [[ "${AUTO_INSTALL}" == true ]]; then
    if ! command -v apt-get >/dev/null 2>&1; then
        echo -e "${RED}错误: 当前系统没有 apt-get，无法 --auto-install${NC}"
        exit 1
    fi
    sudo apt-get update
    sudo apt-get install -y "${APT_DEPS[@]}"
else
    echo "如果编译失败，请先安装依赖："
    echo "sudo apt-get update && sudo apt-get install -y ${APT_DEPS[*]}"
fi
echo -e "${GREEN}✓ 依赖阶段完成${NC}"

echo -e "\n${YELLOW}[3/5] 编译 FreeRDP...${NC}"
if [[ "${SKIP_FREERDP}" == true ]]; then
    echo -e "${YELLOW}跳过 FreeRDP 编译${NC}"
else
    if [[ ! -f "${FREERDP_SRC}/CMakeLists.txt" ]]; then
        echo "FreeRDP 源码不存在，开始拉取 ${FREERDP_TAG} ..."
        mkdir -p "${PROJECT_ROOT}/src"
        git clone --depth 1 --branch "${FREERDP_TAG}" \
            https://github.com/FreeRDP/FreeRDP.git "${FREERDP_SRC}"
    fi

    if [[ "${FORCE_FREERDP}" == true ]]; then
        rm -rf "${FREERDP_BUILD}"
    fi

    mkdir -p "${FREERDP_BUILD}" "${FREERDP_INSTALL}"
    cd "${FREERDP_BUILD}"

    SYS_ZLIB_LIB="$(ldconfig -p 2>/dev/null | awk '/libz\.so / {print $NF}' | head -1 || true)"
    if [[ -z "${SYS_ZLIB_LIB}" ]]; then
        for f in /usr/lib/x86_64-linux-gnu/libz.so /usr/lib/aarch64-linux-gnu/libz.so /usr/lib/libz.so; do
            if [[ -f "${f}" ]]; then
                SYS_ZLIB_LIB="${f}"
                break
            fi
        done
    fi

    CMAKE_ARGS=(
        -DCMAKE_INSTALL_PREFIX="${FREERDP_INSTALL}"
        -DCMAKE_BUILD_TYPE=Release
        -DWITH_SSE2=ON
        -DWITH_CUPS=OFF
        -DWITH_WAYLAND=OFF
        -DWITH_PULSE=OFF
        -DWITH_FFMPEG=OFF
        -DWITH_SWSCALE=OFF
        -DWITH_DSP_FFMPEG=OFF
        -DWITH_FUSE=OFF
        -DWITH_GSTREAMER_1_0=OFF
        -DWITH_CLIENT=OFF
        -DWITH_SERVER=OFF
        -DBUILD_TESTING=OFF
        -DCHANNEL_URBDRC=OFF
        -DWITH_KRB5=OFF
        -DWITH_PCSC=OFF
        -DWITH_ALSA=OFF
    )
    if [[ -n "${SYS_ZLIB_LIB}" ]]; then
        CMAKE_ARGS+=(-DZLIB_LIBRARY="${SYS_ZLIB_LIB}" -DZLIB_INCLUDE_DIR=/usr/include)
    fi

    cmake "${FREERDP_SRC}" "${CMAKE_ARGS[@]}"

    cmake --build . -- -j"$(nproc)"
    cmake --install .
fi
echo -e "${GREEN}✓ FreeRDP 阶段完成${NC}"

echo -e "\n${YELLOW}[4/5] Go 依赖...${NC}"
cd "${PROJECT_ROOT}"
go mod tidy
echo -e "${GREEN}✓ Go 依赖就绪${NC}"

echo -e "\n${YELLOW}[5/5] 编译 Go 程序...${NC}"
LIB_CANDIDATES=(
    "${FREERDP_INSTALL}/lib"
    "${FREERDP_INSTALL}/lib64"
    "${FREERDP_INSTALL}/lib/x86_64-linux-gnu"
    "${FREERDP_INSTALL}/lib/aarch64-linux-gnu"
)

LDFLAG_PARTS=()
RUNTIME_LIB_PATHS=()
for libdir in "${LIB_CANDIDATES[@]}"; do
    if [[ -d "${libdir}" ]]; then
        LDFLAG_PARTS+=("-L${libdir}")
        RUNTIME_LIB_PATHS+=("${libdir}")
    fi
done

if [[ ${#LDFLAG_PARTS[@]} -eq 0 ]]; then
    echo -e "${RED}错误: 未找到 FreeRDP 库目录，请先执行 FreeRDP 编译${NC}"
    exit 1
fi

export CGO_CFLAGS="-I${FREERDP_INSTALL}/include"
export CGO_LDFLAGS="${LDFLAG_PARTS[*]} -lfreerdp3 -lfreerdp-client3 -lwinpr3"
export LD_LIBRARY_PATH="$(IFS=:; echo "${RUNTIME_LIB_PATHS[*]}"):${LD_LIBRARY_PATH:-}"

go build -o go-freerdp-webconnect .

echo -e "\n${GREEN}=== 构建成功 ===${NC}"
echo "产物: ${PROJECT_ROOT}/go-freerdp-webconnect"

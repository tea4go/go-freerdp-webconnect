#!/bin/bash
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PROJECT_ROOT=$(pwd)

echo -e "${YELLOW}=== 清理编译产物 ===${NC}"

# FreeRDP cmake 构建目录
if [ -d "${PROJECT_ROOT}/build" ]; then
    echo -e "删除 ${RED}build/${NC}"
    rm -rf "${PROJECT_ROOT}/build"
fi

# FreeRDP 安装目录
if [ -d "${PROJECT_ROOT}/install" ]; then
    echo -e "删除 ${RED}install/${NC}"
    rm -rf "${PROJECT_ROOT}/install"
fi

# Go 可执行文件
if [ -f "${PROJECT_ROOT}/go-freerdp-webconnect" ]; then
    echo -e "删除 ${RED}go-freerdp-webconnect${NC}"
    rm -f "${PROJECT_ROOT}/go-freerdp-webconnect"
fi

echo -e "\n${GREEN}✓ 清理完成${NC}"

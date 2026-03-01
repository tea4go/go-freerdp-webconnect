#!/bin/bash

echo 清理运行的程序
clear
pkill gofreerdp 2>/dev/null || true
rm -rf gofreerdp

echo 设置库路径
PROJECT_ROOT=$(dirname "$(readlink -f "$0")")
echo ${PROJECT_ROOT}
FREERDP_INSTALL="${PROJECT_ROOT}/install"
echo ${FREERDP_INSTALL}

export LD_LIBRARY_PATH="${FREERDP_INSTALL}/lib:${FREERDP_INSTALL}/lib/x86_64-linux-gnu:${LD_LIBRARY_PATH}"

echo 编译
cd "${PROJECT_ROOT}"
go build -o gofreerdp . || exit 1

echo 运行程序

exec "${PROJECT_ROOT}/gofreerdp" "$@"

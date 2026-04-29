@echo off
setlocal EnableExtensions EnableDelayedExpansion

set "PROJECT_ROOT=%~dp0"
if "%PROJECT_ROOT:~-1%"=="\" set "PROJECT_ROOT=%PROJECT_ROOT:~0,-1%"

set "MSYS64=%MSYS64_ROOT%"
if not defined MSYS64 set "MSYS64=C:\DevDisk\DevTools\msys64"

set "MINGW_BIN=%MSYS64%\mingw64\bin"
set "FREERDP_SRC=%PROJECT_ROOT%\src\FreeRDP"
set "FREERDP_BUILD=%PROJECT_ROOT%\build\freerdp-windows"
set "FREERDP_INSTALL=%PROJECT_ROOT%\install"
set "FREERDP_BIN=%FREERDP_INSTALL%\bin"
set "FREERDP_TAG=3.12.0"
set "SKIP_FREERDP=0"
set "FORCE_FREERDP=0"
set "NO_CLONE=0"
set "MINGW_RUNTIME_DLLS=libssl-3-x64.dll libcrypto-3-x64.dll zlib1.dll libgcc_s_seh-1.dll libwinpthread-1.dll"

:parse_args
if "%~1"=="" goto args_done
if /i "%~1"=="--skip-freerdp" (set "SKIP_FREERDP=1" & shift & goto parse_args)
if /i "%~1"=="--force-freerdp" (set "FORCE_FREERDP=1" & shift & goto parse_args)
if /i "%~1"=="--no-clone" (set "NO_CLONE=1" & shift & goto parse_args)
if /i "%~1"=="-h" goto usage
if /i "%~1"=="--help" goto usage
echo ERROR: Unknown argument "%~1"
goto usage_error

:args_done
echo === Windows Build Script ===
echo PROJECT_ROOT=%PROJECT_ROOT%
echo MSYS64=%MSYS64%

if not exist "%MINGW_BIN%\gcc.exe" (
    echo ERROR: gcc.exe not found: %MINGW_BIN%\gcc.exe
    echo Install MSYS2 and packages:
    echo   pacman -S mingw-w64-x86_64-gcc mingw-w64-x86_64-cmake mingw-w64-x86_64-make
    exit /b 1
)
if not exist "%MINGW_BIN%\cmake.exe" (
    echo ERROR: cmake.exe not found: %MINGW_BIN%\cmake.exe
    exit /b 1
)
if not exist "%MINGW_BIN%\mingw32-make.exe" (
    echo ERROR: mingw32-make.exe not found: %MINGW_BIN%\mingw32-make.exe
    exit /b 1
)

where go >nul 2>nul
if errorlevel 1 (
    echo WARNING: go not found in PATH, skip go check
)

set "PATH=%MINGW_BIN%;%PATH%"

if not exist "%FREERDP_SRC%\CMakeLists.txt" (
    if "%NO_CLONE%"=="1" (
        echo ERROR: FreeRDP source missing and --no-clone specified
        exit /b 1
    )
    where git >nul 2>nul
    if errorlevel 1 (
        echo ERROR: git not found in PATH
        exit /b 1
    )
    if not exist "%PROJECT_ROOT%\src" mkdir "%PROJECT_ROOT%\src"
    echo Cloning FreeRDP %FREERDP_TAG% ...
    git clone --depth 1 --branch %FREERDP_TAG% https://github.com/FreeRDP/FreeRDP.git "%FREERDP_SRC%"
    if errorlevel 1 (
        echo ERROR: Failed to clone FreeRDP
        exit /b 1
    )
)

echo.
echo [1/1] Build FreeRDP
if "%SKIP_FREERDP%"=="1" (
    echo Skip FreeRDP build
    goto go_build
)

if "%FORCE_FREERDP%"=="1" (
    if exist "%FREERDP_BUILD%" rmdir /s /q "%FREERDP_BUILD%"
)

if exist "%FREERDP_INSTALL%\bin\libfreerdp3.dll" if not "%FORCE_FREERDP%"=="1" (
    echo FreeRDP already installed, skip.
    goto go_build
)

if not exist "C:\Temp" mkdir "C:\Temp"
if not exist "%FREERDP_BUILD%" mkdir "%FREERDP_BUILD%"
if not exist "%FREERDP_INSTALL%" mkdir "%FREERDP_INSTALL%"
set "TEMP=C:\Temp"
set "TMP=C:\Temp"

cd /d "%FREERDP_BUILD%"

"%MINGW_BIN%\cmake.exe" "%FREERDP_SRC%" ^
  -G "MinGW Makefiles" ^
  -DCMAKE_INSTALL_PREFIX="%FREERDP_INSTALL%" ^
  -DCMAKE_BUILD_TYPE=Release ^
  -DCMAKE_C_COMPILER="%MINGW_BIN%\gcc.exe" ^
  -DCMAKE_MAKE_PROGRAM="%MINGW_BIN%\mingw32-make.exe" ^
  "-DCMAKE_C_FLAGS=-D__STDC_NO_THREADS__=1 -Wno-incompatible-pointer-types" ^
  -DWITH_SSE2=OFF ^
  -DWITH_SIMD=OFF ^
  -DWITH_CUPS=OFF ^
  -DWITH_WAYLAND=OFF ^
  -DWITH_PULSE=OFF ^
  -DWITH_FFMPEG=OFF ^
  -DWITH_SWSCALE=OFF ^
  -DWITH_DSP_FFMPEG=OFF ^
  -DWITH_FUSE=OFF ^
  -DWITH_GSTREAMER_1_0=OFF ^
  -DWITH_CLIENT=OFF ^
  -DWITH_SERVER=OFF ^
  -DBUILD_TESTING=OFF ^
  -DCHANNEL_URBDRC=OFF ^
  -DWITH_X11=OFF ^
  -DWITH_ALSA=OFF ^
  -DUSE_UNWIND=OFF ^
  -DWITH_OPENSSL=OFF
if errorlevel 1 (
    echo ERROR: CMake configure failed
    exit /b 1
)

set "NPROC=%NUMBER_OF_PROCESSORS%"
if not defined NPROC set "NPROC=4"
"%MINGW_BIN%\mingw32-make.exe" -j%NPROC%
if errorlevel 1 (
    echo ERROR: FreeRDP build failed
    exit /b 1
)
"%MINGW_BIN%\mingw32-make.exe" install
if errorlevel 1 (
    echo ERROR: FreeRDP install failed
    exit /b 1
)

:go_build
echo.
echo [sync] Copy MinGW runtime DLLs to install\bin
if not exist "%FREERDP_BIN%" mkdir "%FREERDP_BIN%"
for %%F in (%MINGW_RUNTIME_DLLS%) do (
    if not exist "%MINGW_BIN%\%%F" (
        echo ERROR: missing %MINGW_BIN%\%%F
        echo Please install required packages in MSYS2 MinGW64.
        exit /b 1
    )
    copy /Y "%MINGW_BIN%\%%F" "%FREERDP_BIN%\" >nul
)

echo.
echo === Build successful ===
echo FreeRDP installed to: %FREERDP_INSTALL%
exit /b 0

:usage
echo Usage: lib_build_windows.cmd [--skip-freerdp] [--force-freerdp] [--no-clone]
exit /b 0

:usage_error
echo Usage: lib_build_windows.cmd [--skip-freerdp] [--force-freerdp] [--no-clone]
exit /b 1

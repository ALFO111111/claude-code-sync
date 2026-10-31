@echo off
rem ============================================================
rem  claude-code-sync 一键打包
rem  双击运行：自动关掉正在运行的程序 -> wails build -> 提示结果
rem ============================================================
setlocal
cd /d "%~dp0"

echo [1/3] 检查程序是否在运行...
tasklist /FI "IMAGENAME eq claude-code-sync.exe" 2>nul | find /I "claude-code-sync.exe" >nul
if not errorlevel 1 (
    echo       程序正在运行，先关闭（不影响数据，下次打开会继续）
    taskkill /F /IM claude-code-sync.exe >nul 2>&1
    rem 等文件句柄释放
    ping -n 3 127.0.0.1 >nul
) else (
    echo       没在运行，直接打包
)

echo [2/3] 检查 Go / Wails 环境...
rem PATH 里没有 go 时，到常见安装位置找。版本不用对上：go.mod 要求的版本（1.27.2）Go 会自动下载
where go >nul 2>nul
if errorlevel 1 if defined GOROOT if exist "%GOROOT%\bin\go.exe" set "PATH=%GOROOT%\bin;%PATH%"
where go >nul 2>nul
if errorlevel 1 for %%P in ("%USERPROFILE%\sdk\go" "C:\Program Files\Go" "%LOCALAPPDATA%\Programs\Go" "D:\develop\Go" "D:\Go" "C:\Go") do if exist "%%~P\bin\go.exe" set "PATH=%%~P\bin;%PATH%"
where go >nul 2>nul
if errorlevel 1 for /d %%D in ("%USERPROFILE%\sdk\go1.*" "%USERPROFILE%\go\go1.*") do if exist "%%D\go\bin\go.exe" (set "PATH=%%D\go\bin;%PATH%") else if exist "%%D\bin\go.exe" set "PATH=%%D\bin;%PATH%"
where go >nul 2>nul
if errorlevel 1 (
    echo [失败] 没找到 go，请先安装 Go（https://go.dev/dl/）并加进 PATH
    pause
    exit /b 1
)
rem 允许 Go 自动下载 go.mod 要求的版本（被设成 local 时会直接报版本太低）
set "GOTOOLCHAIN=auto"
where wails >nul 2>nul
if errorlevel 1 (
    if exist "%USERPROFILE%\go\bin\wails.exe" (
        set "PATH=%USERPROFILE%\go\bin;%PATH%"
    ) else (
        echo [失败] 没找到 wails，请运行: go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
        pause
        exit /b 1
    )
)
where node >nul 2>nul
if errorlevel 1 (
    echo [失败] 没找到 node，请先安装 Node.js 24（https://nodejs.org/）
    pause
    exit /b 1
)

echo [3/3] 开始打包（前端构建 + Go 编译，约 1 分钟；第一次会自动下载 Go 1.27.2）...
wails build
if errorlevel 1 (
    echo.
    echo [失败] 打包没成功，看上方的报错
    pause
    exit /b 1
)
rem wails build 会改写这个哈希文件，还原掉，免得每次打包都多出一个无关改动
git checkout -- frontend/package.json.md5 >nul 2>nul

echo.
echo ============================================================
echo  打包成功: build\bin\claude-code-sync.exe
echo  双击那个 exe 就能重新打开程序
echo ============================================================
pause

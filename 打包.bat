@echo off
rem ============================================================
rem  claude-code-sync 一键打包
rem  双击运行：自动关掉正在运行的程序 -> wails build -> 显示结果
rem ============================================================
setlocal
cd /d "%~dp0"

echo [1/3] 检查程序是否在运行...
tasklist /FI "IMAGENAME eq claude-code-sync.exe" 2>nul | find /I "claude-code-sync.exe" >nul
if not errorlevel 1 (
    echo       发现正在运行，先关闭（不影响数据，下次打开会继续）
    taskkill /F /IM claude-code-sync.exe >nul 2>&1
    rem 等文件句柄释放
    ping -n 3 127.0.0.1 >nul
) else (
    echo       没有在运行，直接打包
)

echo [2/3] 检查 Go / Wails 环境...
rem PATH 里没有 go 时，到当前用户目录下的常见安装位置找（sdk\go 或 sdk\go1.x.y\go）
where go >nul 2>nul
if errorlevel 1 if exist "%USERPROFILE%\sdk\go\bin\go.exe" set "PATH=%USERPROFILE%\sdk\go\bin;%PATH%"
where go >nul 2>nul
if errorlevel 1 for /d %%D in ("%USERPROFILE%\sdk\go1.*") do if exist "%%D\go\bin\go.exe" set "PATH=%%D\go\bin;%PATH%"
where go >nul 2>nul
if errorlevel 1 (
    echo [失败] 没找到 go，请先安装 Go 或把它加进 PATH
    pause
    exit /b 1
)
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

echo [3/3] 开始打包（前端构建 + Go 编译，约 1 分钟）...
wails build
if errorlevel 1 (
    echo.
    echo [失败] 打包没成功，往上翻看看报错
    pause
    exit /b 1
)

echo.
echo ============================================================
echo  打包成功: build\bin\claude-code-sync.exe
echo  双击那个 exe 即可重新打开程序
echo ============================================================
pause

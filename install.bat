@echo off
chcp 65001 >nul 2>&1
setlocal

echo.
echo === sushiro Windows 一键安装 ===
echo.
echo 即将从 GitHub 下载最新版本并安装到 %%LOCALAPPDATA%%\sushiro
echo.

powershell -NoProfile -ExecutionPolicy Bypass -Command ^
    "$ProgressPreference='Continue'; try { irm https://raw.githubusercontent.com/Ryujoxys/sushiro-overdose/master/install/install.ps1 | iex } catch { Write-Host ''; Write-Host \"安装失败: $_\" -ForegroundColor Red; exit 1 }"

set EXITCODE=%ERRORLEVEL%

echo.
if %EXITCODE% NEQ 0 (
    echo 安装未成功完成（退出码 %EXITCODE%）。
    echo 常见原因：
    echo   1. 网络无法访问 GitHub - 请检查网络
    echo   2. PowerShell 执行策略限制 - 以管理员身份运行 PowerShell 后执行: Set-ExecutionPolicy -Scope CurrentUser RemoteSigned
    echo   3. 杀毒软件拦截 - 请暂时关闭或加入白名单
    echo.
)

pause
endlocal
exit /b %EXITCODE%

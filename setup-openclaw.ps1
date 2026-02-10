# OpenClaw Manager - OpenClaw 连接配置脚本 (Windows PowerShell)
# 支持 QQ + 微信双通道配置

$ErrorActionPreference = "Stop"

$OpenClawConfig = "$env:USERPROFILE\.openclaw\openclaw.json"

Write-Host "=========================================="
Write-Host " OpenClaw Manager - 连接配置"
Write-Host "=========================================="

if (-not (Test-Path $OpenClawConfig)) {
    Write-Host "[错误] 未找到 OpenClaw 配置文件: $OpenClawConfig" -ForegroundColor Red
    Write-Host "请先安装 OpenClaw："
    Write-Host "  参考 https://openclaw.ai 安装指南"
    exit 1
}

Write-Host "[1/2] 配置 OpenClaw QQ + 微信频道..."

try {
    $config = Get-Content $OpenClawConfig -Raw | ConvertFrom-Json

    # 确保 channels 存在
    if (-not $config.channels) {
        $config | Add-Member -NotePropertyName "channels" -NotePropertyValue ([PSCustomObject]@{}) -Force
    }

    # 配置 QQ 频道
    if (-not $config.channels.qq) {
        $config.channels | Add-Member -NotePropertyName "qq" -NotePropertyValue ([PSCustomObject]@{}) -Force
    }
    $config.channels.qq | Add-Member -NotePropertyName "enabled" -NotePropertyValue $true -Force
    $config.channels.qq | Add-Member -NotePropertyName "wsUrl" -NotePropertyValue "ws://127.0.0.1:6199/onebot" -Force
    $config.channels.qq | Add-Member -NotePropertyName "accessToken" -NotePropertyValue "" -Force

    # 确保 plugins 存在
    if (-not $config.plugins) {
        $config | Add-Member -NotePropertyName "plugins" -NotePropertyValue ([PSCustomObject]@{}) -Force
    }
    if (-not $config.plugins.entries) {
        $config.plugins | Add-Member -NotePropertyName "entries" -NotePropertyValue ([PSCustomObject]@{}) -Force
    }
    $config.plugins.entries | Add-Member -NotePropertyName "qq" -NotePropertyValue ([PSCustomObject]@{ enabled = $true }) -Force

    $config | ConvertTo-Json -Depth 10 | Set-Content $OpenClawConfig -Encoding UTF8

    Write-Host "[OK] OpenClaw 频道配置已更新" -ForegroundColor Green
    Write-Host "  QQ wsUrl: ws://127.0.0.1:6199/onebot"
    Write-Host "  QQ enabled: true"
} catch {
    Write-Host "[错误] 配置更新失败: $_" -ForegroundColor Red
    exit 1
}

Write-Host "[2/2] 重启 OpenClaw..."

$openclawCmd = Get-Command openclaw -ErrorAction SilentlyContinue
if ($openclawCmd) {
    Write-Host "[提示] 请手动重启 OpenClaw："
    Write-Host "  openclaw gateway restart"
} else {
    Write-Host "[提示] 请手动重启 OpenClaw 使配置生效"
}

Write-Host ""
Write-Host "=========================================="
Write-Host " 配置完成！"
Write-Host "=========================================="
Write-Host ""
Write-Host "接下来："
Write-Host "  1. 访问管理后台 http://你的服务器IP:6199"
Write-Host "  2. QQ 登录：在 QQ 登录页面扫码"
Write-Host "  3. 微信登录：在微信登录页面扫码"
Write-Host "  4. 用另一个 QQ/微信号发私聊消息测试 AI 回复"
Write-Host ""

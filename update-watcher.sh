#!/usr/bin/env bash
# ============================================================
# ClawPanel Update Watcher — 监听容器更新信号并执行 openclaw update
# 用法: ./update-watcher.sh &  (后台运行)
# 或:   nohup ./update-watcher.sh &>/dev/null &
# ============================================================
set -uo pipefail

OPENCLAW_DIR="${OPENCLAW_DIR:-$HOME/.openclaw}"
SIGNAL_FILE="${OPENCLAW_DIR}/update-signal.json"
RESULT_FILE="${OPENCLAW_DIR}/update-result.json"
LOG_FILE="${OPENCLAW_DIR}/update-log.txt"
POLL_INTERVAL=3

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"; }

log "Update watcher started. Watching: ${SIGNAL_FILE}"
log "OpenClaw dir: ${OPENCLAW_DIR}"

while true; do
    if [ -f "${SIGNAL_FILE}" ]; then
        log "Update signal detected!"
        
        # Remove signal file immediately
        rm -f "${SIGNAL_FILE}"
        
        # Write running status
        echo '{"status":"running","startedAt":"'"$(date -Iseconds)"'","log":["开始执行 openclaw update..."]}' > "${RESULT_FILE}"
        
        # Clear log file
        > "${LOG_FILE}"
        echo "[$(date '+%H:%M:%S')] 开始执行 openclaw update..." >> "${LOG_FILE}"
        
        # Find openclaw binary
        OPENCLAW_BIN=""
        if command -v openclaw &>/dev/null; then
            OPENCLAW_BIN="openclaw"
        elif [ -f "$HOME/.local/bin/openclaw" ]; then
            OPENCLAW_BIN="$HOME/.local/bin/openclaw"
        elif [ -f "/usr/local/bin/openclaw" ]; then
            OPENCLAW_BIN="/usr/local/bin/openclaw"
        elif [ -f "$HOME/openclaw/app/openclaw.mjs" ]; then
            OPENCLAW_BIN="node $HOME/openclaw/app/openclaw.mjs"
        fi
        
        if [ -z "${OPENCLAW_BIN}" ]; then
            echo "[$(date '+%H:%M:%S')] ❌ 找不到 openclaw 命令" >> "${LOG_FILE}"
            echo '{"status":"failed","finishedAt":"'"$(date -Iseconds)"'","log":["找不到 openclaw 命令"]}' > "${RESULT_FILE}"
            log "openclaw binary not found!"
            sleep "${POLL_INTERVAL}"
            continue
        fi
        
        echo "[$(date '+%H:%M:%S')] 使用: ${OPENCLAW_BIN}" >> "${LOG_FILE}"
        log "Using: ${OPENCLAW_BIN}"
        
        # Execute update, streaming output to log file
        echo "[$(date '+%H:%M:%S')] 执行: ${OPENCLAW_BIN} update" >> "${LOG_FILE}"
        
        EXIT_CODE=0
        ${OPENCLAW_BIN} update 2>&1 | while IFS= read -r line; do
            echo "[$(date '+%H:%M:%S')] ${line}" >> "${LOG_FILE}"
            log "  ${line}"
        done
        EXIT_CODE=${PIPESTATUS[0]}
        
        if [ "${EXIT_CODE}" -eq 0 ]; then
            echo "[$(date '+%H:%M:%S')] ✅ 更新完成！" >> "${LOG_FILE}"
            
            # Get new version
            NEW_VER=$(${OPENCLAW_BIN} --version 2>/dev/null || echo "unknown")
            echo "[$(date '+%H:%M:%S')] 当前版本: ${NEW_VER}" >> "${LOG_FILE}"
            
            # Read full log for result file
            LOG_LINES=$(cat "${LOG_FILE}" | python3 -c "import sys,json; print(json.dumps([l.strip() for l in sys.stdin if l.strip()]))" 2>/dev/null || echo '[]')
            echo "{\"status\":\"success\",\"finishedAt\":\"$(date -Iseconds)\",\"newVersion\":\"${NEW_VER}\",\"log\":${LOG_LINES}}" > "${RESULT_FILE}"
            log "Update completed successfully! Version: ${NEW_VER}"
        else
            echo "[$(date '+%H:%M:%S')] ❌ 更新失败 (exit code: ${EXIT_CODE})" >> "${LOG_FILE}"
            LOG_LINES=$(cat "${LOG_FILE}" | python3 -c "import sys,json; print(json.dumps([l.strip() for l in sys.stdin if l.strip()]))" 2>/dev/null || echo '[]')
            echo "{\"status\":\"failed\",\"finishedAt\":\"$(date -Iseconds)\",\"exitCode\":${EXIT_CODE},\"log\":${LOG_LINES}}" > "${RESULT_FILE}"
            log "Update failed with exit code ${EXIT_CODE}"
        fi
    fi
    
    sleep "${POLL_INTERVAL}"
done

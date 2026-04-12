package updater

import "fmt"

func updaterHTML(currentVersion, token string, panelPort int, edition string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>ClawPanel 更新工具</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
:root{--bg:#0f0f14;--card:#1a1a24;--border:#2a2a3a;--text:#e0e0ef;--muted:#888;--accent:#7c5cfc;--accent2:#a78bfa;--green:#22c55e;--red:#ef4444;--amber:#f59e0b;--blue:#3b82f6}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:var(--bg);color:var(--text);min-height:100vh;display:flex;flex-direction:column;align-items:center;padding:2rem 1rem}
.container{max-width:640px;width:100%%}
.header{text-align:center;margin-bottom:2rem}
.header h1{font-size:1.5rem;font-weight:700;background:linear-gradient(135deg,var(--accent),var(--accent2));-webkit-background-clip:text;-webkit-text-fill-color:transparent;margin-bottom:.25rem}
.header p{color:var(--muted);font-size:.8rem}
.card{background:var(--card);border:1px solid var(--border);border-radius:12px;padding:1.5rem;margin-bottom:1rem}
.card h2{font-size:.85rem;font-weight:700;margin-bottom:1rem;display:flex;align-items:center;gap:.5rem}
.badge{display:inline-block;padding:2px 8px;border-radius:6px;font-size:.7rem;font-weight:600;font-family:monospace}
.badge-ver{background:rgba(124,92,252,.15);color:var(--accent2)}
.badge-new{background:rgba(245,158,11,.15);color:var(--amber)}
.badge-ok{background:rgba(34,197,94,.15);color:var(--green)}
.badge-err{background:rgba(239,68,68,.15);color:var(--red)}
.badge-src{background:rgba(59,130,246,.15);color:var(--blue)}
.steps{display:flex;flex-direction:column;gap:.5rem}
.step{display:flex;align-items:center;gap:.75rem;padding:.6rem .8rem;border-radius:8px;background:rgba(255,255,255,.02);border:1px solid var(--border);font-size:.8rem;transition:all .2s}
.step.running{border-color:var(--accent);background:rgba(124,92,252,.05)}
.step.done{border-color:var(--green);opacity:.8}
.step.error{border-color:var(--red);background:rgba(239,68,68,.05)}
.step.skipped{opacity:.4}
.step-icon{width:24px;height:24px;border-radius:50%%;display:flex;align-items:center;justify-content:center;font-size:.7rem;flex-shrink:0}
.step.pending .step-icon{background:var(--border);color:var(--muted)}
.step.running .step-icon{background:var(--accent);color:white;animation:pulse 1.5s infinite}
.step.done .step-icon{background:var(--green);color:white}
.step.error .step-icon{background:var(--red);color:white}
.step.skipped .step-icon{background:var(--border);color:var(--muted)}
.step-info{flex:1;min-width:0}
.step-name{font-weight:600;font-size:.78rem}
.step-msg{color:var(--muted);font-size:.7rem;margin-top:1px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
@keyframes pulse{0%%,100%%{opacity:1}50%%{opacity:.5}}
.progress-bar{height:6px;background:var(--border);border-radius:3px;overflow:hidden;margin:1rem 0}
.progress-fill{height:100%%;background:linear-gradient(90deg,var(--accent),var(--accent2));border-radius:3px;transition:width .3s}
.log-box{background:#0a0a10;border:1px solid var(--border);border-radius:8px;padding:.75rem;max-height:200px;overflow-y:auto;font-family:'Cascadia Code','Fira Code',monospace;font-size:.7rem;line-height:1.6;color:var(--muted)}
.log-box::-webkit-scrollbar{width:4px}
.log-box::-webkit-scrollbar-thumb{background:var(--border);border-radius:2px}
.btn{padding:.6rem 1.2rem;border:none;border-radius:8px;font-size:.8rem;font-weight:600;cursor:pointer;transition:all .15s;display:inline-flex;align-items:center;gap:.5rem}
.btn:disabled{opacity:.4;cursor:not-allowed}
.btn-primary{background:linear-gradient(135deg,var(--accent),#6d4de6);color:white}
.btn-primary:hover:not(:disabled){filter:brightness(1.1);transform:translateY(-1px)}
.btn-secondary{background:rgba(255,255,255,.05);color:var(--text);border:1px solid var(--border)}
.btn-secondary:hover:not(:disabled){background:rgba(255,255,255,.08)}
.btn-danger{background:rgba(239,68,68,.15);color:var(--red)}
.btn-upload{background:rgba(59,130,246,.12);color:var(--blue);border:2px dashed rgba(59,130,246,.3);width:100%%;padding:1.5rem;justify-content:center;flex-direction:column;gap:.25rem}
.btn-upload:hover:not(:disabled){border-color:var(--blue);background:rgba(59,130,246,.18)}
.actions{display:flex;gap:.75rem;margin-top:1rem;flex-wrap:wrap}
.version-grid{display:grid;grid-template-columns:1fr 1fr;gap:.75rem;margin-bottom:1rem}
.version-box{padding:.75rem;border-radius:8px;background:rgba(255,255,255,.03);border:1px solid var(--border)}
.version-box label{font-size:.65rem;color:var(--muted);text-transform:uppercase;font-weight:600;letter-spacing:.05em}
.version-box .ver{font-size:1rem;font-weight:700;font-family:monospace;margin-top:2px}
.note-box{background:rgba(255,255,255,.02);border:1px solid var(--border);border-radius:8px;padding:.75rem;font-size:.75rem;line-height:1.7;max-height:150px;overflow-y:auto;white-space:pre-wrap;color:var(--muted)}
.warn-box{background:rgba(245,158,11,.08);border:1px solid rgba(245,158,11,.2);border-radius:8px;padding:.75rem;font-size:.75rem;color:var(--amber);display:flex;align-items:flex-start;gap:.5rem;margin-bottom:.75rem}
.err-box{background:rgba(239,68,68,.08);border:1px solid rgba(239,68,68,.2);border-radius:8px;padding:.75rem;font-size:.75rem;color:var(--red);margin-bottom:.75rem}
.ok-box{background:rgba(34,197,94,.08);border:1px solid rgba(34,197,94,.2);border-radius:8px;padding:.75rem;font-size:.75rem;color:var(--green);margin-bottom:.75rem}
.source-box{display:grid;gap:.5rem;margin-top:1rem}
.source-option{display:flex;align-items:flex-start;gap:.75rem;padding:.75rem .9rem;border-radius:10px;background:rgba(255,255,255,.03);border:1px solid var(--border);cursor:pointer;transition:all .15s}
.source-option:hover{border-color:rgba(124,92,252,.45);background:rgba(124,92,252,.05)}
.source-option input{margin-top:.15rem}
.source-title{font-size:.78rem;font-weight:700}
.source-desc{font-size:.7rem;color:var(--muted);margin-top:.15rem;line-height:1.5}
.hidden{display:none}
.footer{text-align:center;margin-top:2rem;color:var(--muted);font-size:.7rem}
.modal-overlay{position:fixed;inset:0;background:rgba(0,0,0,.6);display:flex;align-items:center;justify-content:center;z-index:100;backdrop-filter:blur(4px)}
.modal{background:var(--card);border:1px solid var(--border);border-radius:16px;padding:1.5rem;max-width:400px;width:90%%}
.modal h3{font-size:.9rem;font-weight:700;margin-bottom:.75rem}
.modal p{font-size:.78rem;color:var(--muted);margin-bottom:1rem;line-height:1.5}
.modal .actions{justify-content:flex-end}
</style>
</head>
<body>
<div class="container">
  <div class="header">
    <h1 id="page-title">🐾 ClawPanel 更新工具</h1>
    <p id="page-subtitle">独立更新服务 · 进程隔离 · 安全可靠</p>
  </div>

  <!-- Unauthorized state -->
  <div id="unauthorized" class="card hidden">
    <div class="err-box">⛔ 授权令牌无效或已过期。请返回 ClawPanel 面板重新点击「前往更新」。</div>
    <button class="btn btn-secondary" onclick="goBack()">← 返回面板</button>
  </div>

  <!-- Main content -->
  <div id="main-content" class="hidden">
    <!-- Version info card -->
    <div class="card" id="version-card">
      <h2>📦 版本信息</h2>
      <div class="version-grid">
        <div class="version-box">
          <label>当前版本</label>
          <div class="ver" id="cur-ver">%s</div>
        </div>
        <div class="version-box">
          <label>最新版本</label>
          <div class="ver" id="new-ver">检测中...</div>
        </div>
      </div>
      <div id="ver-status"></div>
      <div id="release-note" class="hidden">
        <h2 style="font-size:.78rem;margin-bottom:.5rem">📋 更新日志</h2>
        <div class="note-box" id="note-content"></div>
      </div>
      <div id="major-warn" class="hidden warn-box">⚠️ <span id="major-warn-text"></span></div>
    </div>

    <!-- Actions card -->
    <div class="card" id="action-card">
      <h2>🚀 更新操作</h2>
      <div class="actions" id="update-actions">
        <button class="btn btn-primary" id="btn-update" onclick="confirmUpdate()" disabled>
          🔄 开始更新
        </button>
        <button class="btn btn-secondary" id="btn-refresh" onclick="checkVersion()">
          🔍 重新检测
        </button>
      </div>
      <div id="source-section" class="source-box hidden">
        <div class="warn-box">🌐 更新说明：<span>更新元数据和二进制会先同步到本机缓存目录，再从本机缓存执行替换，降低停服后网络抖动导致的失败概率。</span></div>
        <label class="source-option">
          <input type="radio" name="download-source" value="github">
          <div>
            <div class="source-title">本机镜像</div>
            <div class="source-desc">推荐；先通过代理同步 GitHub Release 到本机，再从本机缓存更新。</div>
          </div>
        </label>
        <label class="source-option">
          <input type="radio" name="download-source" value="accel">
          <div>
            <div class="source-title">本机镜像</div>
            <div class="source-desc">兼容保留项；当前同样走本机缓存，不再依赖外部加速服务器。</div>
          </div>
        </label>
      </div>
      <div id="upload-section" style="margin-top:1rem">
        <button class="btn btn-upload" id="btn-upload" onclick="document.getElementById('file-input').click()">
          📁 离线更新：上传可执行文件
          <span style="font-size:.65rem;color:var(--muted)">适用于无网络环境</span>
        </button>
        <input type="file" id="file-input" style="display:none" accept=".exe,*" onchange="handleUpload(event)">
      </div>
    </div>

    <!-- Progress card -->
    <div class="card hidden" id="progress-card">
      <h2>📊 更新进度 <span class="badge badge-ver" id="progress-pct">0%%</span></h2>
      <div class="progress-bar"><div class="progress-fill" id="progress-fill" style="width:0%%"></div></div>
      <div class="steps" id="steps-list"></div>
      <div style="margin-top:1rem">
        <h2 style="font-size:.78rem;margin-bottom:.5rem">📜 更新日志</h2>
        <div class="log-box" id="log-box"></div>
      </div>
    </div>

    <!-- Result card -->
    <div class="card hidden" id="result-card">
      <div id="result-content"></div>
      <div class="actions">
        <button class="btn btn-primary" onclick="goBack()">← 返回版本管理页面</button>
      </div>
    </div>
  </div>
</div>

<div class="footer">ClawPanel Update Tool · Isolated Process · v%s</div>

<!-- Confirm modal -->
<div id="confirm-modal" class="modal-overlay hidden" onclick="closeModal()">
  <div class="modal" onclick="event.stopPropagation()">
    <h3>⚠️ 确认更新</h3>
    <p id="confirm-text">确定要更新吗？</p>
    <div class="actions">
      <button class="btn btn-secondary" onclick="closeModal()">取消</button>
      <button class="btn btn-primary" onclick="doUpdate()">确认更新</button>
    </div>
  </div>
</div>

<script>
const TOKEN = '%s';
const PANEL_PORT = %d;
const UPDATER_BASE = window.location.origin;
const UPDATER_PATH_BASE = '/api/panel/updater';
const EDITION = '%s';
const PANEL_URL = window.location.protocol + '//' + window.location.hostname + ':' + PANEL_PORT;
let pollTimer = null;
let versionInfo = null;

// Detect mode from URL: ?mode=openclaw or default (clawpanel)
const urlParams = new URLSearchParams(window.location.search);
const MODE = urlParams.get('mode') === 'openclaw' ? 'openclaw' : 'clawpanel';

// Mode-specific config
const CFG = MODE === 'openclaw' ? {
  title: '🦞 OpenClaw 更新工具',
  subtitle: '可视化更新 · 实时日志 · openclaw update',
  checkApi: 'check-openclaw-version',
  startApi: 'start-openclaw-update',
  progressApi: 'openclaw-progress',
  confirmMsg: '确定要更新 OpenClaw 吗？将执行 openclaw update 命令。',
  hasUpload: false,
  productName: 'OpenClaw'
} : {
  title: '🐾 ClawPanel 更新工具',
  subtitle: '独立更新服务 · 进程隔离 · 安全可靠',
  checkApi: 'check-version',
  startApi: 'start-update',
  progressApi: 'progress',
  confirmMsg: '确定要更新 ClawPanel 吗？更新过程中面板服务将暂时停止。',
  hasUpload: true,
  productName: 'ClawPanel'
};

const SOURCE_STORAGE_KEY = 'clawpanel-update-source:' + EDITION + ':' + MODE;

function getSelectedSource() {
  const checked = document.querySelector('input[name="download-source"]:checked');
  return checked ? checked.value : (localStorage.getItem(SOURCE_STORAGE_KEY) || 'accel');
}

function setSelectedSource(value) {
  const normalized = value === 'github' ? 'github' : 'accel';
  const radio = document.querySelector('input[name="download-source"][value="' + normalized + '"]');
  if (radio) radio.checked = true;
  localStorage.setItem(SOURCE_STORAGE_KEY, normalized);
}

async function api(path, opts) {
  const params = new URLSearchParams();
  params.set('token', TOKEN);
  if (MODE === 'clawpanel' && (path === CFG.checkApi || path === CFG.startApi)) {
    params.set('source', getSelectedSource());
  }
  const sep = path.includes('?') ? '&' : '?';
  const url = UPDATER_BASE + UPDATER_PATH_BASE + '/api/' + path + sep + params.toString();
  const resp = await fetch(url, opts);
  return resp.json();
}

async function init() {
  // Set mode-specific UI
  document.getElementById('page-title').textContent = CFG.title;
  document.getElementById('page-subtitle').textContent = CFG.subtitle;
  document.title = CFG.productName + ' 更新工具';
  if (!CFG.hasUpload) {
    document.getElementById('upload-section').style.display = 'none';
  }
  if (MODE === 'clawpanel') {
    document.getElementById('source-section').classList.remove('hidden');
    setSelectedSource(localStorage.getItem(SOURCE_STORAGE_KEY) || 'accel');
    document.querySelectorAll('input[name="download-source"]').forEach(function(el){
      el.addEventListener('change', function(){
        setSelectedSource(el.value);
        checkVersion();
      });
    });
    if (EDITION === 'lite') {
      document.getElementById('btn-upload').innerHTML = '📁 离线更新：上传 Lite 整包<span style="font-size:.65rem;color:var(--muted)">支持上传 .tar.gz，保留现有 data 目录</span>';
      document.getElementById('file-input').setAttribute('accept', '.tar.gz,.tgz,*');
    }
  }

  // Validate token
  const r = await api('validate');
  if (!r.ok) {
    document.getElementById('unauthorized').classList.remove('hidden');
    return;
  }
  document.getElementById('main-content').classList.remove('hidden');
  checkVersion();
}

async function checkVersion() {
  document.getElementById('new-ver').textContent = '检测中...';
  document.getElementById('ver-status').innerHTML = '';
  document.getElementById('btn-update').disabled = true;
  document.getElementById('release-note').classList.add('hidden');
  document.getElementById('major-warn').classList.add('hidden');

  try {
    const r = await api(CFG.checkApi);
    if (!r.ok) {
      document.getElementById('ver-status').innerHTML = '<div class="err-box">检测失败: ' + (r.error||'未知错误') + '</div>';
      return;
    }
    versionInfo = r;
    document.getElementById('cur-ver').textContent = r.currentVersion || '-';
    document.getElementById('new-ver').textContent = r.latestVersion || '-';
    if (r.hasUpdate) {
      document.getElementById('ver-status').innerHTML = '<div class="warn-box">⬆️ 发现新版本！当前 ' + r.currentVersion + ' → ' + r.latestVersion + (r.source ? ' <span class="badge badge-src" style="margin-left:4px">' + r.source + '</span>' : '') + '</div>';
      document.getElementById('btn-update').disabled = false;
      document.getElementById('btn-update').textContent = '🔄 开始更新';
    } else {
      document.getElementById('ver-status').innerHTML = '<div class="ok-box">✅ 当前已是最新版本</div>';
      // Still allow force update
      document.getElementById('btn-update').disabled = false;
      document.getElementById('btn-update').textContent = '🔄 强制更新';
    }
    if (r.releaseNote) {
      document.getElementById('release-note').classList.remove('hidden');
      document.getElementById('note-content').textContent = r.releaseNote;
    }
    if (r.majorChange && r.changeWarning) {
      document.getElementById('major-warn').classList.remove('hidden');
      document.getElementById('major-warn-text').textContent = r.changeWarning;
    }
  } catch(e) {
    document.getElementById('ver-status').innerHTML = '<div class="err-box">网络错误: ' + e.message + '</div>';
  }
}

function confirmUpdate() {
  var t = CFG.confirmMsg;
  if (versionInfo && versionInfo.hasUpdate) {
    t = '确定要从 ' + versionInfo.currentVersion + ' 更新到 ' + versionInfo.latestVersion + ' 吗？';
    if (MODE === 'clawpanel') t += '\\n\\n更新过程中 ClawPanel 面板服务将暂时停止。';
    if (MODE === 'openclaw') t += '\\n\\n将执行 openclaw update 命令。';
  }
  if (MODE === 'clawpanel') {
    t += '\\n\\n更新来源：本机更新镜像';
    if (EDITION === 'lite') t += '\\n本次将整包更新 Lite（面板 + 内置 OpenClaw + 预置插件），不会覆盖现有 data 目录。';
  }
  document.getElementById('confirm-text').textContent = t;
  const modal = document.getElementById('confirm-modal');
  modal.style.display = '';
  modal.classList.remove('hidden');
}

function closeModal() {
  document.getElementById('confirm-modal').classList.add('hidden');
}

async function doUpdate() {
  const modal = document.getElementById('confirm-modal');
  modal.classList.add('hidden');
  modal.style.display = 'none';
  showProgress();
  try {
    const r = await api(CFG.startApi, {method:'POST'});
    if (!r.ok) {
      addLog('❌ 启动更新失败: ' + (r.error||''));
      return;
    }
    startPolling();
  } catch(e) {
    if (MODE === 'clawpanel') {
      addLog('⚠️ 启动请求发送完成（服务可能正在重启中）');
    } else {
      addLog('⚠️ 网络错误: ' + e.message);
    }
    startPolling();
  }
}

function addLog(msg) {
  const logEl = document.getElementById('log-box');
  logEl.innerHTML += '<div>' + msg + '</div>';
  logEl.scrollTop = logEl.scrollHeight;
}

async function handleUpload(e) {
  const file = e.target.files[0];
  if (!file) return;
  if (!confirm('确定要使用上传的文件 "' + file.name + '" 进行离线更新吗？')) {
    e.target.value = '';
    return;
  }
  showProgress();
  const fd = new FormData();
  fd.append('file', file);
  try {
    const url = UPDATER_BASE + UPDATER_PATH_BASE + '/api/upload-update?token=' + TOKEN;
    const resp = await fetch(url, {method:'POST', body:fd});
    const r = await resp.json();
    if (!r.ok) { alert('上传失败: ' + (r.error||'')); return; }
    startPolling();
  } catch(e) {
    alert('上传失败: ' + e.message);
  }
  e.target.value = '';
}

function showProgress() {
  document.getElementById('action-card').classList.add('hidden');
  document.getElementById('version-card').classList.add('hidden');
  document.getElementById('progress-card').classList.remove('hidden');
  document.getElementById('result-card').classList.add('hidden');
}

function startPolling() {
  if (pollTimer) clearInterval(pollTimer);
  pollTimer = setInterval(pollProgress, 800);
  pollProgress();
}

async function pollProgress() {
  try {
    const r = await api(CFG.progressApi);
    if (!r.ok) return;
    const st = r.state;
    document.getElementById('progress-pct').textContent = (st.progress||0) + '%%';
    document.getElementById('progress-fill').style.width = (st.progress||0) + '%%';
    const stepsEl = document.getElementById('steps-list');
    stepsEl.innerHTML = (st.steps||[]).map(function(s) {
      const icons = {pending:'○', running:'◉', done:'✓', error:'✕', skipped:'—'};
      return '<div class="step ' + s.status + '">' +
        '<div class="step-icon">' + (icons[s.status]||'○') + '</div>' +
        '<div class="step-info"><div class="step-name">' + s.name + '</div>' +
        (s.message ? '<div class="step-msg">' + s.message + '</div>' : '') +
        '</div></div>';
    }).join('');
    const logEl = document.getElementById('log-box');
    logEl.innerHTML = (st.log||[]).map(function(l){return '<div>'+l+'</div>'}).join('');
    logEl.scrollTop = logEl.scrollHeight;
    if (st.phase === 'done' || st.phase === 'error' || st.phase === 'rolled_back') {
      clearInterval(pollTimer); pollTimer = null;
      setTimeout(function(){showResult(st)}, 500);
    }
  } catch(e) {
    // Server might be restarting
  }
}

function showResult(st) {
  document.getElementById('progress-card').classList.add('hidden');
  document.getElementById('result-card').classList.remove('hidden');
  const el = document.getElementById('result-content');
  if (st.phase === 'done') {
    el.innerHTML = '<div class="ok-box" style="font-size:.85rem">🎉 ' + CFG.productName + ' 更新成功！</div>' +
      '<p style="font-size:.78rem;color:var(--muted);margin-bottom:.5rem">' +
      (st.from_ver ? st.from_ver + ' → ' + st.to_ver : '更新完成') +
      (st.source ? ' · 线路: ' + st.source : '') + '</p>';
  } else if (st.phase === 'rolled_back') {
    el.innerHTML = '<div class="warn-box">⚠️ 更新失败，已自动回滚到旧版本</div>' +
      '<div class="err-box">' + (st.error||'未知错误') + '</div>';
  } else {
    el.innerHTML = '<div class="err-box" style="font-size:.85rem">❌ 更新失败</div>' +
      '<div class="err-box">' + (st.error||'未知错误') + '</div>';
  }
}

function goBack() {
  window.location.href = PANEL_URL + '/#/system?tab=version';
}

window.addEventListener('beforeunload', function(e) {
  if (pollTimer) {
    e.preventDefault();
    e.returnValue = '更新正在进行中，离开可能导致更新中断。';
  }
});

init();
</script>
</body>
</html>`, currentVersion, currentVersion, token, panelPort, edition)
}

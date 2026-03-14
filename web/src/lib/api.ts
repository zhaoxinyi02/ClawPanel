import { mockApi } from './mockApi';

const IS_DEMO = import.meta.env.VITE_DEMO === 'true';
const BASE = '/api';

function headers() {
  const h: Record<string, string> = { 'Content-Type': 'application/json' };
  const token = localStorage.getItem('admin-token');
  if (token) h['Authorization'] = `Bearer ${token}`;
  return h;
}

async function get(path: string) {
  const res = await fetch(BASE + path, { headers: headers() });
  return res.json();
}

async function post(path: string, body?: any) {
  const res = await fetch(BASE + path, { method: 'POST', headers: headers(), body: body ? JSON.stringify(body) : undefined });
  return res.json();
}

async function postLong(path: string, body?: any, timeoutMs = 120000) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const res = await fetch(BASE + path, { method: 'POST', headers: headers(), body: body ? JSON.stringify(body) : undefined, signal: controller.signal });
    return res.json();
  } finally { clearTimeout(timer); }
}

async function put(path: string, body?: any) {
  const res = await fetch(BASE + path, { method: 'PUT', headers: headers(), body: body ? JSON.stringify(body) : undefined });
  return res.json();
}

async function del(path: string) {
  const res = await fetch(BASE + path, { method: 'DELETE', headers: headers() });
  return res.json();
}

function authHeader() {
  const token = localStorage.getItem('admin-token');
  return token ? { 'Authorization': `Bearer ${token}` } : {};
}

async function uploadFormData(path: string, formData: FormData) {
  const res = await fetch(BASE + path, {
    method: 'POST',
    headers: authHeader() as Record<string, string>,
    body: formData,
  });
  return res.json();
}

const _api = {
  login: (token: string) => post('/auth/login', { token }),
  changePassword: (oldPassword: string, newPassword: string) => post('/auth/change-password', { oldPassword, newPassword }),
  getStatus: () => get('/status'),
  getOpenClawConfig: () => get('/openclaw/config'),
  updateOpenClawConfig: (config: any) => put('/openclaw/config', { config }),
  getFeishuDMDiagnosis: () => get('/openclaw/feishu-dm-diagnosis'),
  getAgentsConfig: () => get('/openclaw/agents'),
  createAgent: (agent: any) => post('/openclaw/agents', { agent }),
  updateAgent: (id: string, agent: any) => put(`/openclaw/agents/${id}`, { agent }),
  deleteAgent: (id: string, preserveSessions = true) => del(`/openclaw/agents/${id}?preserveSessions=${preserveSessions ? 'true' : 'false'}`),
  getAgentCoreFiles: (id: string) => get(`/openclaw/agents/${id}/core-files`),
  saveAgentCoreFile: (id: string, name: string, content: string) => put(`/openclaw/agents/${id}/core-files`, { name, content }),
  getBindings: () => get('/openclaw/bindings'),
  updateBindings: (bindings: any[]) => put('/openclaw/bindings', { bindings }),
  previewRoute: (meta: any) => post('/openclaw/route/preview', { meta }),
  getModels: () => get('/openclaw/models'),
  updateModels: (data: any) => put('/openclaw/models', data),
  getChannels: () => get('/openclaw/channels'),
  updateChannel: (id: string, data: any) => put(`/openclaw/channels/${id}`, data),
  updatePlugin: (id: string, data: any) => put(`/openclaw/plugins/${id}`, data),
  getAdminConfig: () => get('/admin/config'),
  updateAdminConfig: (data: any) => put('/admin/config', data),
  updateAdminSection: (section: string, data: any) => put(`/admin/config/${section}`, data),
  getGroups: () => get('/bot/groups'),
  getFriends: () => get('/bot/friends'),
  sendMessage: (type: string, id: number, message: any[]) => post('/bot/send', { type, id, message }),
  reconnectBot: () => post('/bot/reconnect'),
  getRequests: () => get('/requests'),
  approveRequest: (flag: string) => post(`/requests/${flag}/approve`),
  rejectRequest: (flag: string, reason?: string) => post(`/requests/${flag}/reject`, { reason }),
  // NapCat Login
  napcatLoginStatus: () => post('/napcat/login-status'),
  napcatGetQRCode: () => post('/napcat/qrcode'),
  napcatRefreshQRCode: () => post('/napcat/qrcode/refresh'),
  napcatQuickLoginList: () => get('/napcat/quick-login-list'),
  napcatQuickLogin: (uin: string) => post('/napcat/quick-login', { uin }),
  napcatPasswordLogin: (uin: string, password: string) => post('/napcat/password-login', { uin, password }),
  napcatLoginInfo: () => get('/napcat/login-info'),
  napcatLogout: () => post('/napcat/logout'),
  napcatRestart: () => post('/napcat/restart'),
  napcatStatus: () => get('/napcat/status'),
  napcatReconnectLogs: () => get('/napcat/reconnect-logs'),
  napcatReconnect: () => post('/napcat/reconnect'),
  napcatMonitorConfig: (data: { autoReconnect?: boolean; maxReconnect?: number }) => put('/napcat/monitor-config', data),
  napcatDiagnose: (repair?: boolean) => postLong('/napcat/diagnose', { repair: !!repair }, 120000),
  systemDiagnose: () => get('/system/diagnose'),
  // Config check & fix
  checkConfig: () => get('/openclaw/config/check'),
  fixConfig: (issueIds: string[]) => post('/openclaw/config/fix', { issueIds }),
  toggleChannel: (channelId: string, enabled: boolean) => post('/openclaw/toggle-channel', { channelId, enabled }),
  getQQChannelState: () => get('/openclaw/qq-channel/state'),
  setupQQChannel: () => post('/openclaw/qq-channel/setup'),
  repairQQChannel: () => post('/openclaw/qq-channel/repair'),
  cleanupQQChannel: () => post('/openclaw/qq-channel/cleanup'),
  deleteQQChannel: () => post('/openclaw/qq-channel/delete'),
  switchFeishuVariant: (variant: 'official' | 'clawteam') => post('/openclaw/feishu-variant', { variant }),
  // WeChat
  wechatStatus: () => get('/wechat/status'),
  wechatLoginUrl: () => get('/wechat/login-url'),
  wechatSend: (to: string, content: string, isRoom?: boolean) => post('/wechat/send', { to, content, isRoom }),
  wechatSendFile: (to: string, fileUrl: string, isRoom?: boolean) => post('/wechat/send-file', { to, fileUrl, isRoom }),
  wechatConfig: () => get('/wechat/config'),
  wechatUpdateConfig: (data: any) => put('/wechat/config', data),
  // Workspace
  workspaceFiles: (subPath?: string) => get('/workspace/files' + (subPath ? `?path=${encodeURIComponent(subPath)}` : '')),
  workspaceStats: () => get('/workspace/stats'),
  workspaceConfig: () => get('/workspace/config'),
  workspaceUpdateConfig: (data: any) => put('/workspace/config', data),
  workspaceUpload: (files: File[], subPath?: string) => {
    const fd = new FormData();
    files.forEach(f => fd.append('files', f));
    if (subPath) fd.append('path', subPath);
    return uploadFormData('/workspace/upload', fd);
  },
  workspaceMkdir: (name: string, subPath?: string) => post('/workspace/mkdir', { name, path: subPath || '' }),
  workspaceDelete: (paths: string[]) => post('/workspace/delete', { paths }),
  workspaceClean: () => post('/workspace/clean'),
  workspaceDownloadUrl: (filePath: string) => BASE + '/workspace/download?path=' + encodeURIComponent(filePath) + '&token=' + (localStorage.getItem('admin-token') || ''),
  workspacePreviewUrl: (filePath: string) => BASE + '/workspace/preview?path=' + encodeURIComponent(filePath) + '&token=' + (localStorage.getItem('admin-token') || ''),
  agentIdentityAvatarUrl: (agentId: string) => BASE + `/openclaw/agents/${encodeURIComponent(agentId)}/identity/avatar?token=` + (localStorage.getItem('admin-token') || ''),
  workspacePreview: (filePath: string) => get('/workspace/preview?path=' + encodeURIComponent(filePath)),
  workspaceNotes: () => get('/workspace/notes'),
  workspaceSetNote: (filePath: string, note: string) => put('/workspace/notes', { path: filePath, note }),
  // System
  getSystemEnv: () => get('/system/env'),
  getSystemVersion: () => get('/system/version'),
  createBackup: () => post('/system/backup'),
  getBackups: () => get('/system/backups'),
  restoreBackup: (backupName: string) => post('/system/restore', { backupName }),
  getSkills: (agentId?: string) => get('/system/skills' + (agentId ? `?agentId=${encodeURIComponent(agentId)}` : '')),
  syncClawHub: () => post('/system/clawhub-sync'),
  searchClawHub: (query?: string, agentId?: string) => {
    const params = new URLSearchParams();
    if (query) params.set('q', query);
    if (agentId) params.set('agentId', agentId);
    const suffix = params.toString();
    return get('/system/clawhub/search' + (suffix ? `?${suffix}` : ''));
  },
  installClawHubSkill: (skillId: string, agentId?: string) => post('/system/clawhub/install', { skillId, agentId }),
  getCronJobs: () => get('/system/cron'),
  updateCronJobs: (jobs: any[]) => put('/system/cron', { jobs }),
  getDocs: () => get('/system/docs'),
  saveDoc: (docPath: string, content: string) => put('/system/docs', { path: docPath, content }),
  getIdentityDocs: () => get('/system/identity-docs'),
  saveIdentityDoc: (docPath: string, content: string) => put('/system/identity-docs', { path: docPath, content }),
  checkUpdate: () => post('/system/check-update'),
  doUpdate: () => post('/system/do-update'),
  getUpdateStatus: () => get('/system/update-status'),
  // ClawPanel 面板自检更新
  getPanelVersion: () => get('/panel/version'),
  checkPanelUpdate: () => get('/panel/check-update'),
  doPanelUpdate: () => post('/panel/do-update'),
  getPanelUpdateProgress: () => get('/panel/update-progress'),
  getUpdatePopup: () => get('/panel/update-popup'),
  markUpdatePopupShown: () => post('/panel/update-popup/shown'),
  generateUpdateToken: () => post('/panel/update-token'),
  getUpdateHistory: () => get('/panel/update-history'),
  restartProcess: () => post('/process/restart'),
  restartGateway: () => post('/system/restart-gateway'),
  restartPanel: () => post('/system/restart-panel'),
  getRestartGatewayStatus: () => get('/system/restart-gateway-status'),
  getAdminToken: () => get('/system/admin-token'),
  getSudoPassword: () => get('/system/sudo-password'),
  setSudoPassword: (password: string) => put('/system/sudo-password', { password }),
  // Skill toggle
  toggleSkill: (id: string, enabled: boolean, aliases?: string[]) => put(`/system/skills/${id}/toggle`, { enabled, aliases }),
  // Model health check
  checkModelHealth: (baseUrl: string, apiKey: string, apiType: string, modelId?: string) => post('/system/model-health', { baseUrl, apiKey, apiType, modelId }),
  // AI Assistant chat
  aiChat: (messages: { role: string; content: string }[], providerId?: string, modelId?: string) => postLong('/system/ai-chat', { messages, providerId, modelId }, 120000),
  // Event Log
  getEvents: (opts?: { limit?: number; offset?: number; source?: string; search?: string }) => {
    const params = new URLSearchParams();
    if (opts?.limit) params.set('limit', String(opts.limit));
    if (opts?.offset) params.set('offset', String(opts.offset));
    if (opts?.source) params.set('source', opts.source);
    if (opts?.search) params.set('search', opts.search);
    return get('/events?' + params.toString());
  },
  clearEvents: () => post('/events/clear'),
  // Sessions
  getSessions: (agent?: string) => get(`/sessions${agent ? '?agent=' + agent : ''}`),
  getSessionDetail: (id: string, agent?: string) => get(`/sessions/${id}${agent ? '?agent=' + agent : ''}`),
  deleteSession: (id: string, agent?: string) => del(`/sessions/${id}${agent ? '?agent=' + agent : ''}`),
  // Software & Tasks
  getSoftwareList: () => get('/software/list'),
  getOpenClawInstances: () => get('/software/openclaw-instances'),
  installSoftware: (software: string) => post('/software/install', { software }),
  getTasks: () => get('/tasks'),
  getTaskDetail: (id: string) => get(`/tasks/${id}`),
  // Plugin Center
  getPluginList: () => get('/plugins/list'),
  getInstalledPlugins: () => get('/plugins/installed'),
  getPluginDetail: (id: string) => get(`/plugins/${id}`),
  refreshPluginRegistry: () => post('/plugins/registry/refresh'),
  installPlugin: (pluginId: string, source?: string) => post('/plugins/install', { pluginId, source }),
  uninstallPlugin: (id: string, cleanupConfig: boolean = true) => del(`/plugins/${id}?cleanupConfig=${cleanupConfig ? 'true' : 'false'}`),
  togglePlugin: (id: string, enabled: boolean) => put(`/plugins/${id}/toggle`, { enabled }),
  getPluginConfig: (id: string) => get(`/plugins/${id}/config`),
  updatePluginConfig: (id: string, config: any) => put(`/plugins/${id}/config`, config),
  getPluginLogs: (id: string) => get(`/plugins/${id}/logs`),
  updatePluginVersion: (id: string) => post(`/plugins/${id}/update`),
  // Workflow Center
  getWorkflowSettings: () => get('/workflows/settings'),
  updateWorkflowSettings: (data: any) => put('/workflows/settings', data),
  getWorkflowTemplates: () => get('/workflows/templates'),
  saveWorkflowTemplate: (template: any) => post('/workflows/templates', { template }),
  deleteWorkflowTemplate: (id: string) => del(`/workflows/templates/${id}`),
  generateWorkflowTemplate: (prompt: string, category?: string, settings?: any) => postLong('/workflows/templates/generate', { prompt, category, settings }, 120000),
  getWorkflowRuns: (status?: string) => get(`/workflows/runs${status ? `?status=${encodeURIComponent(status)}` : ''}`),
  getWorkflowRun: (id: string) => get(`/workflows/runs/${id}`),
  startWorkflowRun: (templateId: string, data?: any) => post(`/workflows/templates/${templateId}/run`, data || {}),
  controlWorkflowRun: (id: string, action: string, reply?: string) => post(`/workflows/runs/${id}/control`, { action, reply }),
  resendWorkflowArtifact: (id: string, data: { stepKey?: string; fileName?: string }) => post(`/workflows/runs/${id}/artifacts/resend`, data),
  deleteWorkflowRun: (id: string) => del(`/workflows/runs/${id}`),
};

// In demo mode, replace all API calls with mock data
export const api = IS_DEMO ? mockApi as unknown as typeof _api : _api;

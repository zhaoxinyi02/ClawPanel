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

async function put(path: string, body?: any) {
  const res = await fetch(BASE + path, { method: 'PUT', headers: headers(), body: body ? JSON.stringify(body) : undefined });
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
  getStatus: () => get('/status'),
  getOpenClawConfig: () => get('/openclaw/config'),
  updateOpenClawConfig: (config: any) => put('/openclaw/config', { config }),
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
  toggleChannel: (channelId: string, enabled: boolean) => post('/openclaw/toggle-channel', { channelId, enabled }),
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
  workspacePreview: (filePath: string) => get('/workspace/preview?path=' + encodeURIComponent(filePath)),
  workspaceNotes: () => get('/workspace/notes'),
  workspaceSetNote: (filePath: string, note: string) => put('/workspace/notes', { path: filePath, note }),
  // System
  getSystemEnv: () => get('/system/env'),
  getSystemVersion: () => get('/system/version'),
  createBackup: () => post('/system/backup'),
  getBackups: () => get('/system/backups'),
  restoreBackup: (backupName: string) => post('/system/restore', { backupName }),
  getSkills: () => get('/system/skills'),
  syncClawHub: () => post('/system/clawhub-sync'),
  getCronJobs: () => get('/system/cron'),
  updateCronJobs: (jobs: any[]) => put('/system/cron', { jobs }),
  getDocs: () => get('/system/docs'),
  saveDoc: (docPath: string, content: string) => put('/system/docs', { path: docPath, content }),
  getIdentityDocs: () => get('/system/identity-docs'),
  saveIdentityDoc: (docPath: string, content: string) => put('/system/identity-docs', { path: docPath, content }),
  checkUpdate: () => post('/system/check-update'),
  doUpdate: () => post('/system/do-update'),
  getUpdateStatus: () => get('/system/update-status'),
  restartGateway: () => post('/system/restart-gateway'),
  getRestartGatewayStatus: () => get('/system/restart-gateway-status'),
  getAdminToken: () => get('/system/admin-token'),
  getSudoPassword: () => get('/system/sudo-password'),
  setSudoPassword: (password: string) => put('/system/sudo-password', { password }),
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
};

// In demo mode, replace all API calls with mock data
export const api = IS_DEMO ? mockApi as unknown as typeof _api : _api;

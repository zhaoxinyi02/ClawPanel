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

export const api = {
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
  // WeChat
  wechatStatus: () => get('/wechat/status'),
  wechatLoginUrl: () => get('/wechat/login-url'),
  wechatSend: (to: string, content: string, isRoom?: boolean) => post('/wechat/send', { to, content, isRoom }),
  wechatSendFile: (to: string, fileUrl: string, isRoom?: boolean) => post('/wechat/send-file', { to, fileUrl, isRoom }),
  wechatConfig: () => get('/wechat/config'),
  wechatUpdateConfig: (data: any) => put('/wechat/config', data),
};

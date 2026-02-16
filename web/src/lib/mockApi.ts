// Mock API layer for demo site — returns fake data, no backend needed

const delay = (ms = 300) => new Promise(r => setTimeout(r, ms));

const FAKE_LOGS = [
  { id: 'log1', time: Date.now() - 5000, source: 'qq', direction: 'incoming', type: 'text', content: '你好，请问今天天气怎么样？', sender: { id: '10001', name: '张三' }, group: { id: '88001', name: '技术交流群' } },
  { id: 'log2', time: Date.now() - 4000, source: 'openclaw', direction: 'outgoing', type: 'text', content: '今天北京天气晴朗，气温25°C，适合户外活动。', sender: { id: 'bot', name: 'OpenClaw' } },
  { id: 'log3', time: Date.now() - 3000, source: 'qq', direction: 'incoming', type: 'text', content: '帮我写一首关于春天的诗', sender: { id: '10002', name: '李四' }, group: { id: '88001', name: '技术交流群' } },
  { id: 'log4', time: Date.now() - 2000, source: 'openclaw', direction: 'outgoing', type: 'text', content: '春风拂面花自开，\n绿柳依依燕归来。\n碧水蓝天映日暖，\n万物复苏展新怀。', sender: { id: 'bot', name: 'OpenClaw' } },
  { id: 'log5', time: Date.now() - 1000, source: 'system', direction: 'system', type: 'text', content: '[System] Skill "web-search" executed successfully (320ms)', sender: { id: 'system', name: 'System' } },
  { id: 'log6', time: Date.now() - 800, source: 'qq', direction: 'incoming', type: 'text', content: '搜索一下最新的AI新闻', sender: { id: '10003', name: '王五' } },
  { id: 'log7', time: Date.now() - 500, source: 'openclaw', direction: 'outgoing', type: 'text', content: '以下是最新的AI新闻摘要：\n1. OpenAI发布GPT-5，性能大幅提升\n2. Google DeepMind推出新一代Gemini模型\n3. 国内多家AI公司获得新一轮融资', sender: { id: 'bot', name: 'OpenClaw' } },
  { id: 'log8', time: Date.now() - 200, source: 'qq', direction: 'incoming', type: 'media', content: '[图片消息]', sender: { id: '10001', name: '张三' }, group: { id: '88002', name: 'AI爱好者群' } },
];

const FAKE_SKILLS = [
  { id: 'web-search', name: 'Web Search', description: '联网搜索能力，支持Google/Bing/DuckDuckGo', enabled: true, source: 'app-skill', version: '1.2.0', requires: { env: ['SEARCH_API_KEY'], bins: [] } },
  { id: 'image-gen', name: 'Image Generation', description: 'AI图像生成，支持DALL-E/Stable Diffusion', enabled: true, source: 'app-skill', version: '1.0.3', requires: { env: ['OPENAI_API_KEY'], bins: [] } },
  { id: 'code-runner', name: 'Code Runner', description: '安全沙箱代码执行，支持Python/JS/Shell', enabled: true, source: 'skill', version: '0.9.1', requires: { env: [], bins: ['python3', 'node'] } },
  { id: 'weather', name: 'Weather', description: '天气查询插件，支持全球城市', enabled: true, source: 'installed', version: '1.1.0' },
  { id: 'translator', name: 'Translator', description: '多语言翻译，支持100+语言', enabled: false, source: 'installed', version: '0.8.0' },
  { id: 'reminder', name: 'Reminder', description: '定时提醒功能', enabled: true, source: 'workspace', version: '0.5.0' },
  { id: 'knowledge-base', name: 'Knowledge Base', description: 'RAG知识库检索', enabled: false, source: 'config-ext', version: '1.0.0', requires: { env: ['EMBEDDING_API_KEY'], bins: [] } },
];

const FAKE_CRON_JOBS = [
  { id: 'cron_1', name: '每日早报', enabled: true, schedule: { kind: 'cron', expr: '0 8 * * *' }, sessionTarget: 'main', wakeMode: 'now', payload: { kind: 'text', text: '请生成今日早报，包含科技、AI、财经要闻', deliver: true, channel: 'qq' }, state: { lastRunAtMs: Date.now() - 86400000, lastStatus: 'ok' }, createdAtMs: Date.now() - 604800000 },
  { id: 'cron_2', name: '系统健康检查', enabled: true, schedule: { kind: 'cron', expr: '*/30 * * * *' }, sessionTarget: 'main', wakeMode: 'now', payload: { kind: 'text', text: '检查系统状态并报告', deliver: false }, state: { lastRunAtMs: Date.now() - 1800000, lastStatus: 'ok' }, createdAtMs: Date.now() - 1209600000 },
  { id: 'cron_3', name: '周报生成', enabled: false, schedule: { kind: 'cron', expr: '0 18 * * 5' }, sessionTarget: 'main', wakeMode: 'now', payload: { kind: 'text', text: '生成本周工作总结', deliver: true, channel: 'qq' }, state: {}, createdAtMs: Date.now() - 2592000000 },
];

const FAKE_CONFIG: any = {
  models: {
    providers: {
      'deepseek': { baseUrl: 'https://api.deepseek.com/v1', apiKey: 'sk-demo-***', api: 'openai-completions', models: ['deepseek-chat', 'deepseek-reasoner'] },
      'openai': { baseUrl: 'https://api.openai.com/v1', apiKey: 'sk-demo-***', api: 'openai-completions', models: ['gpt-4o', 'gpt-4o-mini'] },
    },
  },
  agents: { defaults: { model: { primary: 'deepseek/deepseek-chat' } } },
  channels: {
    qq: { enabled: true, ownerQQ: '123456789' },
  },
  gateway: { wsUrl: 'ws://127.0.0.1:3001', accessToken: 'demo-token' },
  env: { vars: {} },
  hooks: {},
  commands: {},
};

const FAKE_FILES: any[] = [
  { name: 'openclaw.json', path: 'openclaw.json', size: 2048, sizeHuman: '2.0 KB', isDirectory: false, modifiedAt: new Date(Date.now() - 3600000).toISOString(), extension: '.json', ageDays: 0 },
  { name: 'identity', path: 'identity', size: 0, sizeHuman: '-', isDirectory: true, modifiedAt: new Date(Date.now() - 86400000).toISOString(), extension: '', ageDays: 1 },
  { name: 'plugins', path: 'plugins', size: 0, sizeHuman: '-', isDirectory: true, modifiedAt: new Date(Date.now() - 172800000).toISOString(), extension: '', ageDays: 2 },
  { name: 'logs', path: 'logs', size: 0, sizeHuman: '-', isDirectory: true, modifiedAt: new Date(Date.now() - 43200000).toISOString(), extension: '', ageDays: 0 },
  { name: 'README.md', path: 'README.md', size: 1234, sizeHuman: '1.2 KB', isDirectory: false, modifiedAt: new Date(Date.now() - 604800000).toISOString(), extension: '.md', ageDays: 7 },
  { name: 'system-prompt.md', path: 'system-prompt.md', size: 4567, sizeHuman: '4.5 KB', isDirectory: false, modifiedAt: new Date(Date.now() - 7200000).toISOString(), extension: '.md', ageDays: 0 },
  { name: 'avatar.png', path: 'avatar.png', size: 45678, sizeHuman: '44.6 KB', isDirectory: false, modifiedAt: new Date(Date.now() - 2592000000).toISOString(), extension: '.png', ageDays: 30 },
];

export const mockApi = {
  login: async (_token: string) => { await delay(500); return { ok: true, token: 'demo-token' }; },
  getStatus: async () => {
    await delay(100);
    return {
      ok: true,
      napcat: { connected: true, selfId: '2854196310', nickname: 'OpenClaw Demo Bot', groupCount: 12, friendCount: 86 },
      wechat: { loggedIn: false },
      admin: { uptime: 172800, memoryMB: 256 },
      openclaw: { currentModel: 'deepseek/deepseek-chat', enabledChannels: [
        { id: 'qq', label: 'QQ (NapCat)', type: 'builtin' },
        { id: 'telegram', label: 'Telegram', type: 'plugin' },
      ] },
    };
  },
  getOpenClawConfig: async () => { await delay(200); return { ok: true, config: JSON.parse(JSON.stringify(FAKE_CONFIG)) }; },
  updateOpenClawConfig: async (_config: any) => { await delay(300); return { ok: true }; },
  getModels: async () => { await delay(100); return { ok: true, models: FAKE_CONFIG.models }; },
  updateModels: async (_data: any) => { await delay(200); return { ok: true }; },
  getChannels: async () => { await delay(100); return { ok: true, channels: FAKE_CONFIG.channels }; },
  updateChannel: async (_id: string, _data: any) => { await delay(200); return { ok: true }; },
  updatePlugin: async (_id: string, _data: any) => { await delay(200); return { ok: true }; },
  getAdminConfig: async () => { await delay(100); return { ok: true, config: {} }; },
  updateAdminConfig: async (_data: any) => { await delay(200); return { ok: true }; },
  updateAdminSection: async (_section: string, _data: any) => { await delay(200); return { ok: true }; },
  getGroups: async () => { await delay(100); return { ok: true, groups: [] }; },
  getFriends: async () => { await delay(100); return { ok: true, friends: [] }; },
  sendMessage: async (_type: string, _id: number, _message: any[]) => { await delay(200); return { ok: true }; },
  reconnectBot: async () => { await delay(500); return { ok: true }; },
  getRequests: async () => { await delay(100); return { ok: true, requests: [
    { flag: 'req1', type: 'group', userId: '10086', groupId: '88003', comment: '我想加入技术交流群' },
    { flag: 'req2', type: 'friend', userId: '10010', comment: '你好，我是小明' },
  ] }; },
  approveRequest: async (_flag: string) => { await delay(200); return { ok: true }; },
  rejectRequest: async (_flag: string, _reason?: string) => { await delay(200); return { ok: true }; },
  napcatLoginStatus: async () => { await delay(100); return { ok: true, status: 'online' }; },
  napcatGetQRCode: async () => { await delay(300); return { ok: true, qrcode: '' }; },
  napcatRefreshQRCode: async () => { await delay(300); return { ok: true, qrcode: '' }; },
  napcatQuickLoginList: async () => { await delay(200); return { ok: true, list: ['2854196310', '1234567890'] }; },
  napcatQuickLogin: async (_uin: string) => { await delay(500); return { ok: true }; },
  napcatPasswordLogin: async (_uin: string, _password: string) => { await delay(500); return { ok: true }; },
  napcatLoginInfo: async () => { await delay(100); return { ok: true, uin: '2854196310', nickname: 'OpenClaw Demo Bot' }; },
  napcatLogout: async () => { await delay(500); return { ok: true }; },
  toggleChannel: async (_channelId: string, _enabled: boolean) => { await delay(300); return { ok: true, message: 'OK' }; },
  wechatStatus: async () => { await delay(100); return { ok: true, loggedIn: false }; },
  wechatLoginUrl: async () => { await delay(100); return { ok: true, url: '' }; },
  wechatSend: async () => { await delay(200); return { ok: true }; },
  wechatSendFile: async () => { await delay(200); return { ok: true }; },
  wechatConfig: async () => { await delay(100); return { ok: true, config: {} }; },
  wechatUpdateConfig: async () => { await delay(200); return { ok: true }; },
  workspaceFiles: async (_subPath?: string) => { await delay(200); return { ok: true, files: FAKE_FILES, currentPath: '', parentPath: null }; },
  workspaceStats: async () => { await delay(100); return { ok: true, totalFiles: 42, totalSize: 1048576, totalSizeHuman: '1.0 MB', oldFiles: 3 }; },
  workspaceConfig: async () => { await delay(100); return { ok: true, config: { autoCleanEnabled: true, autoCleanDays: 30, excludePatterns: ['*.json', 'identity/*'] } }; },
  workspaceUpdateConfig: async (data: any) => { await delay(200); return { ok: true, config: data }; },
  workspaceUpload: async () => { await delay(500); return { ok: true, files: [{ name: 'uploaded.txt' }] }; },
  workspaceMkdir: async () => { await delay(200); return { ok: true }; },
  workspaceDelete: async (paths: string[]) => { await delay(200); return { ok: true, deleted: paths }; },
  workspaceClean: async () => { await delay(300); return { ok: true, deleted: ['old-file.log'] }; },
  workspaceDownloadUrl: (_filePath: string) => '#',
  workspacePreviewUrl: (_filePath: string) => '/logo.jpg',
  workspacePreview: async (_filePath: string) => { await delay(200); return { ok: true, type: 'text', content: '# OpenClaw Demo\n\nThis is a demo workspace file.\n\n## Features\n- AI-powered chatbot management\n- Multi-channel support\n- Skill plugins\n- Scheduled tasks' }; },
  workspaceNotes: async () => { await delay(100); return { ok: true, notes: { 'openclaw.json': '主配置文件', 'system-prompt.md': 'Bot 系统提示词' } }; },
  workspaceSetNote: async () => { await delay(200); return { ok: true }; },
  getSystemEnv: async () => { await delay(300); return { ok: true, os: { platform: 'linux', arch: 'x64', release: '5.15.0-91-generic', hostname: 'openclaw-demo' }, software: { node: 'v20.11.0', npm: '10.2.4', openclaw: '4.2.1', napcat: '2.5.0' }, runtime: { pid: 12345, cwd: '/app', uptime: 172800 } }; },
  getSystemVersion: async () => { await delay(200); return { ok: true, version: '4.2.1', latest: '4.2.1', updateAvailable: false }; },
  createBackup: async () => { await delay(500); return { ok: true }; },
  getBackups: async () => { await delay(200); return { ok: true, backups: [
    { name: 'backup-2026-02-17-120000.json', size: '2.1 KB', createdAt: new Date(Date.now() - 86400000).toISOString() },
    { name: 'backup-2026-02-16-080000.json', size: '2.0 KB', createdAt: new Date(Date.now() - 172800000).toISOString() },
  ] }; },
  restoreBackup: async () => { await delay(500); return { ok: true }; },
  getSkills: async () => { await delay(200); return { ok: true, skills: FAKE_SKILLS }; },
  syncClawHub: async () => { await delay(800); return { ok: true, skills: [] }; },
  getCronJobs: async () => { await delay(200); return { ok: true, jobs: FAKE_CRON_JOBS }; },
  updateCronJobs: async () => { await delay(300); return { ok: true }; },
  getDocs: async () => { await delay(200); return { ok: true, docs: [{ name: 'system-prompt.md', path: 'system-prompt.md', content: '# System Prompt\n\nYou are OpenClaw, a helpful AI assistant.' }] }; },
  saveDoc: async () => { await delay(300); return { ok: true }; },
  getIdentityDocs: async () => { await delay(200); return { ok: true, docs: [{ name: 'identity.md', path: 'identity/identity.md', content: '# Identity\n\nI am OpenClaw, an AI-powered chatbot manager.' }] }; },
  saveIdentityDoc: async () => { await delay(300); return { ok: true }; },
  checkUpdate: async () => { await delay(1000); return { ok: true, updateAvailable: false, currentVersion: '4.2.1', latestVersion: '4.2.1' }; },
  doUpdate: async () => { await delay(2000); return { ok: true }; },
  getUpdateStatus: async () => { await delay(100); return { ok: true, status: 'idle' }; },
  restartGateway: async () => { await delay(500); return { ok: true }; },
  getRestartGatewayStatus: async () => { await delay(100); return { ok: true, status: 'ok' }; },
  getAdminToken: async () => { await delay(100); return { ok: true, token: 'demo-admin-token' }; },
  getSudoPassword: async () => { await delay(100); return { ok: true, password: '' }; },
  setSudoPassword: async () => { await delay(200); return { ok: true }; },
  getEvents: async () => { await delay(200); return { ok: true, events: FAKE_LOGS }; },
  clearEvents: async () => { await delay(100); return { ok: true }; },
};

// Fake WebSocket data for demo
export const DEMO_LOG_ENTRIES = FAKE_LOGS;
export const DEMO_NAPCAT_STATUS = { connected: true, selfId: '2854196310', nickname: 'OpenClaw Demo Bot', groupCount: 12, friendCount: 86 };
export const DEMO_WECHAT_STATUS = { loggedIn: false };

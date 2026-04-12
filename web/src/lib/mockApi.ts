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
  { id: 'web-search', skillKey: 'web-search', name: 'Web Search', description: '联网搜索能力，支持Google/Bing/DuckDuckGo', enabled: true, source: 'app-skill', version: '1.2.0', requires: { env: ['SEARCH_API_KEY'], bins: [], config: ['tools.web.search.provider', 'tools.web.search.maxResults'] } },
  { id: 'image-gen', skillKey: 'image-gen', name: 'Image Generation', description: 'AI图像生成，支持DALL-E/Stable Diffusion', enabled: true, source: 'app-skill', version: '1.0.3', requires: { env: ['OPENAI_API_KEY'], bins: [] } },
  { id: 'code-runner', skillKey: 'code-runner', name: 'Code Runner', description: '安全沙箱代码执行，支持Python/JS/Shell', enabled: true, source: 'managed', version: '0.9.1', requires: { env: [], bins: ['python3', 'node'], config: ['tools.exec.timeoutSec', 'tools.exec.ask'] } },
  { id: 'weather', skillKey: 'weather', name: 'Weather', description: '天气查询插件，支持全球城市', enabled: true, source: 'managed', version: '1.1.0' },
  { id: 'translator', skillKey: 'translator', name: 'Translator', description: '多语言翻译，支持100+语言', enabled: false, source: 'managed', version: '0.8.0' },
  { id: 'reminder', skillKey: 'reminder', name: 'Reminder', description: '定时提醒功能', enabled: true, source: 'workspace', version: '0.5.0' },
  { id: 'knowledge-base', skillKey: 'knowledge-base', name: 'Knowledge Base', description: 'RAG知识库检索', enabled: false, source: 'extra-dir', version: '1.0.0', requires: { env: ['EMBEDDING_API_KEY'], bins: [] } },
];

const FAKE_CLAWHUB_SKILLS = [
  { id: 'weather', name: 'Weather', description: '官方天气查询技能', version: '1.1.0', installed: true, installedVersion: '1.1.0' },
  { id: 'jira-helper', name: 'Jira Helper', description: '读取与总结 Jira 工单上下文', version: '0.6.2', installed: false },
  { id: 'feishu-notes', name: 'Feishu Notes', description: '将结果整理并发送到飞书文档', version: '0.4.5', installed: false },
];

let mockSkillHubCliInstalled = false;

const FAKE_CRON_JOBS = [
  { id: 'cron_1', name: '每日早报', enabled: true, schedule: { kind: 'cron', expr: '0 8 * * *' }, agentId: 'main', sessionTarget: 'main', wakeMode: 'now', payload: { kind: 'text', text: '请生成今日早报，包含科技、AI、财经要闻', deliver: true, channel: 'qq' }, state: { lastRunAtMs: Date.now() - 86400000, lastStatus: 'ok' }, createdAtMs: Date.now() - 604800000 },
  { id: 'cron_2', name: '系统健康检查', enabled: true, schedule: { kind: 'every', everyMs: 1800000 }, agentId: 'main', sessionTarget: 'isolated', wakeMode: 'now', payload: { kind: 'text', text: '检查系统状态并报告', deliver: false }, state: { lastRunAtMs: Date.now() - 1800000, lastStatus: 'ok' }, createdAtMs: Date.now() - 1209600000 },
  { id: 'cron_3', name: '周报生成', enabled: false, schedule: { kind: 'cron', expr: '0 18 * * 5' }, agentId: 'main', sessionTarget: 'main', wakeMode: 'now', payload: { kind: 'text', text: '生成本周工作总结', deliver: true, channel: 'qq' }, state: {}, createdAtMs: Date.now() - 2592000000 },
];

const FAKE_AGENTS = {
  default: 'main',
  defaults: { model: { primary: 'deepseek/deepseek-chat' } },
  list: [
    {
      id: 'main',
      default: true,
      workspace: '/workspaces/main',
      agentDir: 'agents/main',
      sessions: 12,
      lastActive: Date.now() - 60000,
      tools: {
        profile: 'full',
        agentToAgent: { enabled: true, allow: ['translation', 'reviewer'] },
        sessions: { visibility: 'agent' },
      },
      groupChat: { enabled: true },
      sandbox: { mode: 'all', workspaceAccess: 'rw' },
      identity: {
        name: 'Main Assistant',
        theme: 'general assistant',
        emoji: '🤖',
        avatar: 'avatars/main.png',
        description: '负责默认路由与综合问题处理',
        tone: '专业、稳定',
      },
    },
    {
      id: 'work',
      default: false,
      workspace: '/workspaces/work',
      agentDir: 'agents/work',
      sessions: 4,
      lastActive: Date.now() - 3600000,
      tools: {
        profile: 'coding',
        allow: ['read', 'edit', 'exec'],
        deny: ['browser'],
        agentToAgent: { enabled: true, allow: ['main'] },
      },
      groupChat: { enabled: false },
      sandbox: { mode: 'all', workspaceAccess: 'ro' },
      identity: {
        name: 'Work Specialist',
        theme: 'workflow specialist',
        emoji: '🛠️',
        avatar: 'avatars/work.png',
        description: '处理工作流与执行类请求',
        tone: '直接、聚焦结果',
      },
    },
    {
      id: 'translation',
      default: false,
      workspace: '/workspaces/translation',
      agentDir: 'agents/translation',
      sessions: 2,
      lastActive: Date.now() - 7200000,
      tools: { profile: 'minimal', agentToAgent: { enabled: false } },
      groupChat: { enabled: false },
      sandbox: { mode: 'none' },
      identity: { name: 'Translator', theme: 'translation', emoji: '🌐', description: '多语言翻译代理' },
    },
    {
      id: 'reviewer',
      default: false,
      workspace: '/workspaces/reviewer',
      agentDir: 'agents/reviewer',
      sessions: 1,
      lastActive: Date.now() - 86400000,
      tools: { profile: 'read-only', agentToAgent: { enabled: true, allow: ['main'] } },
      groupChat: { enabled: false },
      sandbox: { mode: 'all', workspaceAccess: 'ro' },
      identity: { name: 'Code Reviewer', theme: 'code review', emoji: '🔍', description: '代码审查与质量检测' },
    },
  ],
  bindings: [
    { type: 'route', agentId: 'work', comment: 'work-group', match: { channel: 'qq', peer: { kind: 'group', id: '123' } } },
    { type: 'route', agentId: 'translation', comment: 'feishu-translate', enabled: true, match: { channel: 'feishu', accountId: 'default' } },
  ],
};

const FAKE_AGENT_CORE_FILES: Record<string, any[]> = {
  main: [
    { name: 'AGENTS.md', path: '/workspaces/main/AGENTS.md', exists: true, size: 720, modified: new Date(Date.now() - 3600000).toISOString(), content: '# AGENTS.md\n\nMain agent instructions.' },
    { name: 'SOUL.md', path: '/workspaces/main/SOUL.md', exists: true, size: 248, modified: new Date(Date.now() - 7200000).toISOString(), content: '# SOUL.md\n\nStay calm and helpful.' },
    { name: 'TOOLS.md', path: '/workspaces/main/TOOLS.md', exists: true, size: 312, modified: new Date(Date.now() - 9600000).toISOString(), content: '# TOOLS.md\n\nPrefer structured tool usage.' },
    { name: 'IDENTITY.md', path: '/workspaces/main/IDENTITY.md', exists: true, size: 154, modified: new Date(Date.now() - 9600000).toISOString(), content: '# IDENTITY.md\n\nMain assistant identity.' },
    { name: 'USER.md', path: '/workspaces/main/USER.md', exists: true, size: 120, modified: new Date(Date.now() - 18600000).toISOString(), content: '# USER.md\n\nPrimary operator notes.' },
    { name: 'HEARTBEAT.md', path: '/workspaces/main/HEARTBEAT.md', exists: true, size: 88, modified: new Date(Date.now() - 28600000).toISOString(), content: '# HEARTBEAT.md\n\nCheck status regularly.' },
    { name: 'BOOTSTRAP.md', path: '/workspaces/main/BOOTSTRAP.md', exists: true, size: 132, modified: new Date(Date.now() - 38600000).toISOString(), content: '# BOOTSTRAP.md\n\nBootstrap sequence.' },
    { name: 'MEMORY.md', path: '/workspaces/main/MEMORY.md', exists: false, size: 0, content: '' },
  ],
  work: [
    { name: 'AGENTS.md', path: '/workspaces/work/AGENTS.md', exists: true, size: 340, modified: new Date(Date.now() - 4600000).toISOString(), content: '# AGENTS.md\n\nWork specialist instructions.' },
    { name: 'SOUL.md', path: '/workspaces/work/SOUL.md', exists: false, size: 0, content: '' },
    { name: 'TOOLS.md', path: '/workspaces/work/TOOLS.md', exists: true, size: 160, modified: new Date(Date.now() - 5600000).toISOString(), content: '# TOOLS.md\n\nKeep execution scoped.' },
    { name: 'IDENTITY.md', path: '/workspaces/work/IDENTITY.md', exists: true, size: 144, modified: new Date(Date.now() - 8600000).toISOString(), content: '# IDENTITY.md\n\nWork specialist.' },
    { name: 'USER.md', path: '/workspaces/work/USER.md', exists: false, size: 0, content: '' },
    { name: 'HEARTBEAT.md', path: '/workspaces/work/HEARTBEAT.md', exists: false, size: 0, content: '' },
    { name: 'BOOTSTRAP.md', path: '/workspaces/work/BOOTSTRAP.md', exists: true, size: 118, modified: new Date(Date.now() - 18600000).toISOString(), content: '# BOOTSTRAP.md\n\nWork bootstrap.' },
    { name: 'MEMORY.md', path: '/workspaces/work/MEMORY.md', exists: true, size: 90, modified: new Date(Date.now() - 9600000).toISOString(), content: '# MEMORY.md\n\nRecent work context.' },
  ],
};

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
    feishu: {
      appId: 'cli_demo',
      appSecret: 'demo-secret',
      defaultAccount: 'default',
      accounts: {
        default: { appId: 'cli_demo', appSecret: 'demo-secret', botName: '默认机器人', enabled: true },
        backup: { appId: 'cli_backup', appSecret: 'backup-secret', botName: '备用机器人', enabled: false },
      },
      dmPolicy: 'pairing',
    },
    wecom: {
      enabled: true,
      botId: 'demo-bot-id',
      secret: 'demo-secret',
      dmPolicy: 'open',
      allowFrom: ['*'],
    },
  },
  session: {
    dmScope: 'per-peer',
  },
  plugins: {
    entries: {
      feishu: { enabled: true },
      wecom: { enabled: true },
    },
  },
  gateway: { wsUrl: 'ws://127.0.0.1:3001', accessToken: 'demo-token' },
  env: { vars: {} },
  hooks: {},
  commands: {},
  tools: {
    sessions: { visibility: 'tree' },
    web: { search: { provider: 'brave', apiKey: 'brave-demo-key', maxResults: 5 } },
    exec: { timeoutSec: 30, security: 'allowlist', ask: 'on-miss', safeBins: ['ls', 'cat', 'echo', 'grep', 'git'] },
  },
};

function configPathSegments(path: string): string[] {
  return path.split('.').map(part => part.trim()).filter(Boolean);
}

function getNestedConfigValue(root: any, path: string): { ok: boolean; value?: any } {
  let current = root;
  for (const segment of configPathSegments(path)) {
    if (!current || typeof current !== 'object' || !(segment in current)) {
      return { ok: false };
    }
    current = current[segment];
  }
  return { ok: true, value: current };
}

function setNestedConfigValue(root: any, path: string, value: any) {
  const segments = configPathSegments(path);
  let current = root;
  for (const segment of segments.slice(0, -1)) {
    if (!current[segment] || typeof current[segment] !== 'object') {
      current[segment] = {};
    }
    current = current[segment];
  }
  current[segments[segments.length - 1]] = value;
}

function deleteNestedConfigValue(root: any, path: string) {
  const segments = configPathSegments(path);
  let current = root;
  for (const segment of segments.slice(0, -1)) {
    if (!current || typeof current !== 'object' || !current[segment] || typeof current[segment] !== 'object') {
      return;
    }
    current = current[segment];
  }
  delete current[segments[segments.length - 1]];
}

const FAKE_FILES: any[] = [
  { name: 'openclaw.json', path: 'openclaw.json', size: 2048, sizeHuman: '2.0 KB', isDirectory: false, modifiedAt: new Date(Date.now() - 3600000).toISOString(), extension: '.json', ageDays: 0 },
  { name: 'identity', path: 'identity', size: 0, sizeHuman: '-', isDirectory: true, modifiedAt: new Date(Date.now() - 86400000).toISOString(), extension: '', ageDays: 1 },
  { name: 'plugins', path: 'plugins', size: 0, sizeHuman: '-', isDirectory: true, modifiedAt: new Date(Date.now() - 172800000).toISOString(), extension: '', ageDays: 2 },
  { name: 'logs', path: 'logs', size: 0, sizeHuman: '-', isDirectory: true, modifiedAt: new Date(Date.now() - 43200000).toISOString(), extension: '', ageDays: 0 },
  { name: 'README.md', path: 'README.md', size: 1234, sizeHuman: '1.2 KB', isDirectory: false, modifiedAt: new Date(Date.now() - 604800000).toISOString(), extension: '.md', ageDays: 7 },
  { name: 'system-prompt.md', path: 'system-prompt.md', size: 4567, sizeHuman: '4.5 KB', isDirectory: false, modifiedAt: new Date(Date.now() - 7200000).toISOString(), extension: '.md', ageDays: 0 },
  { name: 'avatar.png', path: 'avatar.png', size: 45678, sizeHuman: '44.6 KB', isDirectory: false, modifiedAt: new Date(Date.now() - 2592000000).toISOString(), extension: '.png', ageDays: 30 },
];

const FAKE_PLUGINS = [
  { id: 'telegram', name: 'Telegram Bridge', description: 'Telegram 通道插件', enabled: true, version: '1.3.0', installed: true, source: 'clawhub', hasUpdate: false },
  { id: 'calendar', name: 'Calendar Tools', description: '日历与提醒插件', enabled: false, version: '0.9.0', installed: true, source: 'manual', hasUpdate: true, latestVersion: '1.0.0' },
  { id: 'notion-sync', name: 'Notion Sync', description: 'Notion 双向同步插件', enabled: false, version: '0.7.2', installed: false, source: 'clawhub', hasUpdate: false },
];

const FAKE_WORKFLOW_TEMPLATES = [
  { id: 'tpl-1', name: '日报生成', description: '自动生成每日工作日报', category: 'report', steps: [{ key: 'gather', type: 'agent', prompt: '汇总今日工作' }, { key: 'format', type: 'agent', prompt: '格式化为日报' }], createdAt: Date.now() - 604800000 },
  { id: 'tpl-2', name: '代码审查', description: '自动化代码 Review 流程', category: 'dev', steps: [{ key: 'diff', type: 'tool', tool: 'exec', args: 'git diff' }, { key: 'review', type: 'agent', prompt: '审查代码变更' }], createdAt: Date.now() - 1209600000 },
];

const FAKE_WORKFLOW_RUNS = [
  { id: 'run-1', templateId: 'tpl-1', templateName: '日报生成', status: 'completed', startedAt: Date.now() - 3600000, completedAt: Date.now() - 3500000, steps: [{ key: 'gather', status: 'completed' }, { key: 'format', status: 'completed' }] },
  { id: 'run-2', templateId: 'tpl-2', templateName: '代码审查', status: 'running', startedAt: Date.now() - 600000, steps: [{ key: 'diff', status: 'completed' }, { key: 'review', status: 'running' }] },
];

const INITIAL_FAKE_SESSIONS = [
  { key: 'main:sess-1', sessionId: 'sess-1', agentId: 'main', chatType: 'direct', lastChannel: 'qq', lastTo: 'ou_demo_a', updatedAt: Date.now() - 3600000, originLabel: '演示私聊 A', originProvider: 'qq', originFrom: 'ou_demo_a', messageCount: 24 },
  { key: 'main:sess-2', sessionId: 'sess-2', agentId: 'main', chatType: 'group', lastChannel: 'qq', lastTo: 'oc_demo_group', updatedAt: Date.now() - 7200000, originLabel: '演示群聊', originProvider: 'qq', originFrom: 'oc_demo_group', messageCount: 8 },
  { key: 'work:sess-3', sessionId: 'sess-3', agentId: 'work', chatType: 'direct', lastChannel: 'wechat', lastTo: 'ou_demo_b', updatedAt: Date.now() - 86400000, originLabel: '工作助手会话', originProvider: 'wechat', originFrom: 'ou_demo_b', messageCount: 15 },
];
type FakeSession = (typeof INITIAL_FAKE_SESSIONS)[number];
type FakeSessionMessage = { id: string; role: string; content: string; timestamp: string };

const INITIAL_FAKE_SESSION_MESSAGES: Record<string, FakeSessionMessage[]> = {
  'main:sess-1': [
    { id: 'sess-1-u1', role: 'user', content: '今天有什么要注意的消息？', timestamp: new Date(Date.now() - 120000).toISOString() },
    { id: 'sess-1-a1', role: 'assistant', content: '今天的重点是模型额度、水位监控和日报提醒。', timestamp: new Date(Date.now() - 115000).toISOString() },
  ],
  'main:sess-2': [
    { id: 'sess-2-u1', role: 'user', content: '帮我整理一下今天的 AI 新闻。', timestamp: new Date(Date.now() - 240000).toISOString() },
    { id: 'sess-2-a1', role: 'assistant', content: '已整理成 3 条摘要，分别是模型发布、融资和产品更新。', timestamp: new Date(Date.now() - 230000).toISOString() },
  ],
  'work:sess-3': [
    { id: 'sess-3-u1', role: 'user', content: '请生成本周工作总结。', timestamp: new Date(Date.now() - 3600000).toISOString() },
    { id: 'sess-3-a1', role: 'assistant', content: '已根据本周会话记录生成总结草稿。', timestamp: new Date(Date.now() - 3540000).toISOString() },
  ],
};

const LEGACY_DEMO_SESSIONS_STORAGE_KEY = 'clawpanel-demo-sessions-v1';
const LEGACY_DEMO_SESSION_MESSAGES_STORAGE_KEY = 'clawpanel-demo-session-messages-v1';
const DEMO_SESSIONS_STORAGE_KEY = 'clawpanel-demo-sessions-v2';
const DEMO_SESSION_MESSAGES_STORAGE_KEY = 'clawpanel-demo-session-messages-v2';

function cloneDemoState<T>(value: T): T {
  return JSON.parse(JSON.stringify(value));
}

function loadDemoState<T>(keys: string[], fallback: T, normalize?: (value: unknown) => T | null): T {
  if (typeof window === 'undefined') return cloneDemoState(fallback);
  for (const key of keys) {
    try {
      const raw = window.sessionStorage.getItem(key);
      if (!raw) continue;
      const parsed = JSON.parse(raw) as unknown;
      if (!normalize) return parsed as T;
      const normalized = normalize(parsed);
      if (normalized !== null) return normalized;
    } catch {}
  }
  return cloneDemoState(fallback);
}

function saveDemoState(key: string, value: unknown) {
  if (typeof window === 'undefined') return;
  try {
    window.sessionStorage.setItem(key, JSON.stringify(value));
  } catch {}
}

function getFakeSessionIdentity(agentId: string | undefined, sessionId: string) {
  return `${agentId || 'main'}:${sessionId}`;
}

function normalizeDemoEpoch(value: number): number {
  return Math.abs(value) < 1e11 ? value * 1000 : value;
}

function normalizeDemoTimestampMs(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) return normalizeDemoEpoch(value);
  const raw = String(value ?? '').trim();
  if (!raw) return null;
  if (/^\d+$/.test(raw)) {
    const numericTimestamp = Number(raw);
    return Number.isFinite(numericTimestamp) ? normalizeDemoEpoch(numericTimestamp) : null;
  }
  const parsedTimestamp = Date.parse(raw);
  return Number.isNaN(parsedTimestamp) ? null : parsedTimestamp;
}

function normalizeDemoChatType(value: unknown): FakeSession['chatType'] | null {
  if (value === 'direct' || value === 'group') return value;
  if (value === 'dm') return 'direct';
  return null;
}

function normalizeDemoSession(value: unknown): FakeSession | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  const raw = value as Record<string, unknown>;
  const sessionId = String(raw.sessionId ?? raw.id ?? '').trim();
  const agentId = String(raw.agentId ?? 'main').trim() || 'main';
  const chatType = normalizeDemoChatType(raw.chatType ?? raw.type);
  if (!sessionId || !chatType) return null;

  const lastChannel = String(raw.lastChannel ?? raw.channel ?? raw.originProvider ?? '').trim();
  const lastTo = String(raw.lastTo ?? raw.peer ?? raw.originFrom ?? '').trim();
  const originFrom = String(raw.originFrom ?? lastTo).trim();
  const updatedAt = normalizeDemoTimestampMs(raw.updatedAt ?? raw.lastMessageAt) ?? Date.now();
  const messageCountRaw = Number(raw.messageCount ?? 0);
  const messageCount = Number.isFinite(messageCountRaw) && messageCountRaw >= 0 ? messageCountRaw : 0;
  const originLabel = String(raw.originLabel ?? raw.title ?? lastTo ?? sessionId).trim() || sessionId;
  const originProvider = String(raw.originProvider ?? lastChannel).trim();
  const key = String(raw.key ?? getFakeSessionIdentity(agentId, sessionId)).trim() || getFakeSessionIdentity(agentId, sessionId);

  return {
    key,
    sessionId,
    agentId,
    chatType,
    lastChannel,
    lastTo,
    updatedAt,
    originLabel,
    originProvider,
    originFrom,
    messageCount,
  };
}

function normalizeDemoSessions(value: unknown): FakeSession[] | null {
  if (!Array.isArray(value)) return null;
  const normalized = value
    .map(item => normalizeDemoSession(item))
    .filter((item): item is FakeSession => item !== null);
  if (value.length > 0 && normalized.length === 0) return null;
  return normalized;
}

function normalizeDemoSessionMessage(value: unknown): FakeSessionMessage | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  const raw = value as Record<string, unknown>;
  const content = String(raw.content ?? '').trim();
  const timestampMs = normalizeDemoTimestampMs(raw.timestamp);
  const timestamp = timestampMs === null ? '' : new Date(timestampMs).toISOString();
  if (!content || !timestamp) return null;
  const role = String(raw.role ?? 'assistant').trim() || 'assistant';
  const id = String(raw.id ?? '').trim() || `${timestamp}:${role}:${content}`;
  return {
    id,
    role,
    content,
    timestamp,
  };
}

function normalizeDemoSessionMessages(value: unknown, sessions: FakeSession[]): Record<string, FakeSessionMessage[]> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  const sourceEntries = Object.entries(value as Record<string, unknown>);
  const sessionIdCounts = new Map<string, number>();
  for (const session of sessions) {
    sessionIdCounts.set(session.sessionId, (sessionIdCounts.get(session.sessionId) || 0) + 1);
  }
  const keyAliases = new Map<string, string>();
  for (const session of sessions) {
    const canonicalKey = getFakeSessionIdentity(session.agentId, session.sessionId);
    keyAliases.set(canonicalKey, canonicalKey);
    if (session.key) keyAliases.set(session.key, canonicalKey);
    if ((sessionIdCounts.get(session.sessionId) || 0) === 1) {
      keyAliases.set(session.sessionId, canonicalKey);
    }
  }

  const validKeys = new Set(sessions.map(session => getFakeSessionIdentity(session.agentId, session.sessionId)));
  const normalized: Record<string, FakeSessionMessage[]> = {};
  for (const [key, rawMessages] of sourceEntries) {
    const canonicalKey = keyAliases.get(key);
    if (!canonicalKey || !validKeys.has(canonicalKey) || !Array.isArray(rawMessages)) continue;
    const nextMessages = rawMessages
      .map(item => normalizeDemoSessionMessage(item))
      .filter((item): item is FakeSessionMessage => item !== null);
    normalized[canonicalKey] = [...(normalized[canonicalKey] || []), ...nextMessages];
  }
  if (sourceEntries.length > 0 && Object.keys(normalized).length === 0) return null;
  for (const [canonicalKey, messages] of Object.entries(normalized)) {
    const deduped = new Map<string, FakeSessionMessage>();
    for (const message of messages) {
      const fallbackKey = `${message.timestamp}:${message.role}:${message.content}`;
      const dedupeKey = message.id || fallbackKey;
      if (!deduped.has(dedupeKey)) deduped.set(dedupeKey, message);
    }
    normalized[canonicalKey] = Array.from(deduped.values()).sort(
      (left, right) => Date.parse(left.timestamp) - Date.parse(right.timestamp),
    );
  }
  return normalized;
}

let fakeSessions: FakeSession[] = loadDemoState(
  [DEMO_SESSIONS_STORAGE_KEY, LEGACY_DEMO_SESSIONS_STORAGE_KEY],
  INITIAL_FAKE_SESSIONS,
  normalizeDemoSessions,
);
let fakeSessionMessages: Record<string, FakeSessionMessage[]> = loadDemoState(
  [DEMO_SESSION_MESSAGES_STORAGE_KEY, LEGACY_DEMO_SESSION_MESSAGES_STORAGE_KEY],
  INITIAL_FAKE_SESSION_MESSAGES,
  value => normalizeDemoSessionMessages(value, fakeSessions),
);
saveDemoState(DEMO_SESSIONS_STORAGE_KEY, fakeSessions);
saveDemoState(DEMO_SESSION_MESSAGES_STORAGE_KEY, fakeSessionMessages);

let fakePanelChatSessions: any[] = [
  {
    id: 'panel-demo-1',
    openclawSessionId: 'panel-demo-1',
    agentId: 'main',
    chatType: 'direct',
    title: '本地开发助手',
    createdAt: Date.now() - 3600000,
    updatedAt: Date.now() - 120000,
    messageCount: 4,
    lastMessage: '帮我看看当前工作区里有哪些关键文件',
  },
];

let fakePanelChatMessages: Record<string, FakeSessionMessage[]> = {
  'panel-demo-1': [
    { id: 'pc-1', role: 'user', content: '你好', timestamp: new Date(Date.now() - 300000).toISOString() },
    { id: 'pc-2', role: 'assistant', content: '你好，我已经在面板里待命了。你可以直接让我查看工作区、读取文件或调用已安装技能。', timestamp: new Date(Date.now() - 294000).toISOString() },
    { id: 'pc-3', role: 'user', content: '帮我看看当前工作区里有哪些关键文件', timestamp: new Date(Date.now() - 130000).toISOString() },
    { id: 'pc-4', role: 'assistant', content: 'Demo 模式下我会返回模拟结果：`AGENTS.md`、`IDENTITY.md`、`BOOTSTRAP.md` 这些都是当前工作区的关键文件。', timestamp: new Date(Date.now() - 120000).toISOString() },
  ],
};

let fakeCompanyTasks: any[] = [];
let fakeCompanyTeams = [
  {
    id: 'default',
    name: '默认团队',
    description: '基于当前智能体自动生成',
    managerAgentId: 'main',
    defaultSummaryAgentId: 'main',
    status: 'active',
    agents: [
      { agentId: 'main', agentName: 'AI公司主管', roleType: 'manager', dutyLabel: '统筹', enabled: true },
      { agentId: 'coding', agentName: 'Coding工程师', roleType: 'worker', dutyLabel: '实现', enabled: true },
      { agentId: 'reviewer', agentName: '质量审校', roleType: 'worker', dutyLabel: '检查', enabled: true },
    ],
  },
];

function findFakeSessionsById(sessionId: string, agentId?: string) {
  return fakeSessions.filter(item => item.sessionId === sessionId && (!agentId || agentId === 'all' || item.agentId === agentId));
}

export const mockApi = {
  login: async (_token: string) => { await delay(500); return { ok: true, token: 'demo-token' }; },
  changePassword: async (_old: string, _new: string) => { await delay(300); return { ok: true }; },
  getStatus: async () => {
    await delay(100);
    return {
      ok: true,
      napcat: { connected: true, selfId: '2854196310', nickname: 'OpenClaw Demo Bot', groupCount: 12, friendCount: 86 },
      wechat: { loggedIn: false },
      admin: { uptime: 172800, memoryMB: 256 },
      gateway: { running: true },
      process: { running: true, pid: 9527, uptime: 172800 },
      panel: { edition: 'pro', version: '5.2.10' },
      openclaw: {
        configured: true,
        edition: 'pro',
        currentModel: 'deepseek/deepseek-chat',
        enabledChannels: [
          { id: 'qq', label: 'QQ (NapCat)', type: 'builtin' },
          { id: 'telegram', label: 'Telegram', type: 'plugin' },
        ],
        runtime: {
          state: 'healthy',
          healthy: true,
          degraded: false,
          processRunning: true,
          gatewayRunning: true,
          title: 'OpenClaw 运行正常',
          message: 'OpenClaw 与网关均在线，消息处理与配置写入可正常进行。',
        },
      },
    };
  },
  getOpenClawConfig: async () => { await delay(200); return { ok: true, config: JSON.parse(JSON.stringify(FAKE_CONFIG)) }; },
  updateOpenClawConfig: async (_config: any) => { await delay(300); return { ok: true }; },
  getFeishuDMDiagnosis: async () => {
    await delay(160);
    return {
      ok: true,
      diagnosis: {
        configuredDmScope: 'per-peer',
        effectiveDmScope: 'per-peer',
        recommendedDmScope: 'per-peer',
        defaultAgent: 'main',
        scannedAgentIds: ['main'],
        accountCount: 1,
        accountIds: ['default'],
        defaultAccount: 'default',
        dmPolicy: 'pairing',
        sessionIndexExists: true,
        feishuSessionCount: 1,
        feishuSessionKeys: ['agent:main:feishu:dm:ou_demo'],
        hasSharedMainSessionKey: false,
        mainSessionKey: 'agent:main:main',
        pendingPairingCount: 0,
        authorizedSenderCount: 2,
        authorizedSenders: [
          {
            accountId: 'default',
            accountConfigured: true,
            senderCount: 2,
            senderIds: ['ou_demo_a', 'ou_demo_b'],
            sourceFiles: ['/Users/demo/.openclaw/credentials/feishu-default-allowFrom.json'],
          },
        ],
      },
    };
  },
  getAgentsConfig: async () => { await delay(150); return { ok: true, agents: JSON.parse(JSON.stringify(FAKE_AGENTS)) }; },
  createAgent: async (_agent: any) => { await delay(200); return { ok: true }; },
  updateAgent: async (_id: string, _agent: any) => { await delay(200); return { ok: true }; },
  deleteAgent: async (_id: string, _preserveSessions = true) => { await delay(200); return { ok: true }; },
  getAgentCoreFiles: async (id: string) => {
    await delay(160);
    const files = FAKE_AGENT_CORE_FILES[id] || FAKE_AGENT_CORE_FILES.main;
    const workspace = (FAKE_AGENTS.list.find(agent => agent.id === id) || FAKE_AGENTS.list[0])?.workspace || '';
    return { ok: true, agentId: id, workspace, files: JSON.parse(JSON.stringify(files)) };
  },
  saveAgentCoreFile: async (id: string, name: string, content: string) => {
    await delay(180);
    const list = FAKE_AGENT_CORE_FILES[id] || FAKE_AGENT_CORE_FILES.main;
    const next = list.find(file => file.name === name);
    if (next) {
      next.content = content;
      next.exists = true;
      next.size = content.length;
      next.modified = new Date().toISOString();
    }
    return { ok: true };
  },
  getBindings: async () => { await delay(120); return { ok: true, bindings: JSON.parse(JSON.stringify(FAKE_AGENTS.bindings)) }; },
  updateBindings: async (_bindings: any[]) => { await delay(200); return { ok: true }; },
  previewRoute: async (_meta: any) => { await delay(180); return { ok: true, result: { agent: 'work', matchedBy: 'binding.peer', matchedIndex: 0, trace: ['scope channel=qq account= defaultAccount= peer=group:123 parentPeer=none guild= team= roles=none', 'select bindings[0]: peer'] } }; },
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
  switchFeishuVariant: async (_variant: 'official' | 'clawteam') => { await delay(220); return { ok: true, message: '飞书版本已切换（Demo）' }; },
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
  getSkills: async (_agentId?: string) => {
    await delay(200);
    return {
      ok: true,
      agentId: _agentId || FAKE_AGENTS.default,
      workspace: (_agentId && FAKE_AGENTS.list.find(agent => agent.id === _agentId)?.workspace) || FAKE_AGENTS.list[0]?.workspace || '',
      skills: JSON.parse(JSON.stringify(FAKE_SKILLS)),
      plugins: [
        { id: 'telegram', name: 'Telegram Bridge', description: 'Telegram 通道插件', enabled: true, path: '/demo/plugins/telegram', source: 'installed' },
        { id: 'calendar', name: 'Calendar Tools', description: '日历与提醒插件', enabled: false, path: '/demo/plugins/calendar', source: 'config-ext' },
      ],
    };
  },
  getSkillConfig: async (skillId: string) => {
    await delay(160);
    const skill = FAKE_SKILLS.find(item => (item.skillKey || item.id) === skillId || item.id === skillId);
    const configKeys = Array.isArray(skill?.requires?.config) ? skill!.requires!.config : [];
    const values: Record<string, any> = {};
    configKeys.forEach((key: string) => {
      const result = getNestedConfigValue(FAKE_CONFIG, key);
      if (result.ok) values[key] = result.value;
    });
    return { ok: true, skillId: skill?.id || skillId, skillKey: skill?.skillKey || skillId, configKeys, values };
  },
  updateSkillConfig: async (skillId: string, values: Record<string, any>) => {
    await delay(220);
    const skill = FAKE_SKILLS.find(item => (item.skillKey || item.id) === skillId || item.id === skillId);
    const configKeys = Array.isArray(skill?.requires?.config) ? skill!.requires!.config : [];
    const allowed = new Set(configKeys);
    Object.entries(values || {}).forEach(([key, value]) => {
      if (!allowed.has(key)) return;
      if (value == null) deleteNestedConfigValue(FAKE_CONFIG, key);
      else setNestedConfigValue(FAKE_CONFIG, key, value);
    });
    const snapshot: Record<string, any> = {};
    configKeys.forEach((key: string) => {
      const result = getNestedConfigValue(FAKE_CONFIG, key);
      if (result.ok) snapshot[key] = result.value;
    });
    return { ok: true, skillId: skill?.id || skillId, skillKey: skill?.skillKey || skillId, configKeys, values: snapshot };
  },
  syncClawHub: async () => { await delay(800); return { ok: true, skills: [] }; },
  searchClawHub: async (query?: string, _agentId?: string, page?: number, limit?: number, _installTarget?: 'agent' | 'global') => {
    await delay(500);
    const q = (query || '').toLowerCase();
    const all = q
      ? FAKE_CLAWHUB_SKILLS.filter(skill =>
          skill.id.toLowerCase().includes(q) ||
          skill.name.toLowerCase().includes(q) ||
          skill.description.toLowerCase().includes(q),
        )
      : FAKE_CLAWHUB_SKILLS;
    const p = page || 1;
    const l = limit || 30;
    const start = (p - 1) * l;
    const skills = all.slice(start, start + l);
    return { ok: true, registryBase: 'https://clawhub.ai', skills: JSON.parse(JSON.stringify(skills)), page: p, limit: l, total: all.length };
  },
  installClawHubSkill: async (skillId: string, _agentId?: string, _installTarget?: 'agent' | 'global') => {
    await delay(800);
    return { ok: true, skillId, agentId: _agentId || FAKE_AGENTS.default, installTarget: _installTarget || 'agent', version: '1.0.0' };
  },
  uninstallSkill: async (skillId: string, _agentId?: string, _installTarget?: 'agent' | 'global') => {
    await delay(600);
    return { ok: true, skillId, agentId: _agentId || FAKE_AGENTS.default, installTarget: _installTarget || 'agent' };
  },
  checkSkillDeps: async (env?: string[], bins?: string[], anyBins?: string[]) => {
    await delay(300);
    const envR = (env || []).map(e => ({ name: e, found: Math.random() > 0.3 }));
    const binR = (bins || []).map(b => ({ name: b, found: Math.random() > 0.3 }));
    const anyR = (anyBins || []).map(b => ({ name: b, found: Math.random() > 0.5 }));
    const allMet = envR.every(r => r.found) && binR.every(r => r.found) && (anyR.length === 0 || anyR.some(r => r.found));
    return { ok: true, allMet, env: envR, bins: binR, anyBins: anyR };
  },
  getSkillHubCatalog: async (_agentId?: string, _installTarget?: 'agent' | 'global') => {
    await delay(600);
    return {
      ok: true, total: 12911,
      generatedAt: '2026-03-01T13:44:23Z',
      featured: ['github', 'openai-whisper', 'skill-vetter', 'sequential-thinking', 'browser', 'google-maps', 'firecrawl', 'fetch', 'puppeteer', 'slack'],
      categories: {
        'AI \u667a\u80fd': ['ai', 'llm', 'gpt', 'openai', 'claude', 'machine-learning'],
        '\u5f00\u53d1\u5de5\u5177': ['developer', 'code', 'git', 'github', 'devops'],
        '\u6548\u7387\u63d0\u5347': ['productivity', 'automation', 'workflow'],
        '\u6570\u636e\u5206\u6790': ['data', 'analytics', 'visualization'],
        '\u5185\u5bb9\u521b\u4f5c': ['content', 'writing', 'blog', 'seo'],
        '\u5b89\u5168\u5408\u89c4': ['security', 'audit', 'compliance'],
        '\u901a\u8baf\u534f\u4f5c': ['communication', 'slack', 'discord'],
      },
      skills: [
        { slug: 'github', name: 'Github', description: 'GitHub integration for repository management', description_zh: 'GitHub \u96c6\u6210\u5de5\u5177\uff0c\u652f\u6301\u4ed3\u5e93\u7ba1\u7406\u3001PR\u3001Issue', version: '1.0.1', homepage: 'https://clawhub.ai/skills/github', tags: ['developer', 'git'], downloads: 59000, stars: 198, score: 128858.2, owner: 'clawhub', updated_at: 1772065840450 },
        { slug: 'openai-whisper', name: 'OpenAI Whisper', description: 'Speech-to-text using OpenAI Whisper', description_zh: 'OpenAI Whisper \u8bed\u97f3\u8f6c\u6587\u5b57', version: '2.1.0', homepage: 'https://clawhub.ai/skills/openai-whisper', tags: ['ai', 'llm'], downloads: 32000, stars: 156, score: 98234.5, owner: 'clawhub', updated_at: 1772065840450 },
        { slug: 'sequential-thinking', name: 'Sequential Thinking', description: 'Step-by-step reasoning tool', description_zh: '\u987a\u5e8f\u601d\u7ef4\u63a8\u7406\u5de5\u5177', version: '1.3.2', homepage: 'https://clawhub.ai/skills/sequential-thinking', tags: ['ai', 'productivity'], downloads: 45000, stars: 210, score: 115000, owner: 'clawhub', updated_at: 1772065840450 },
        { slug: 'browser', name: 'Browser', description: 'Web browsing and scraping tool', description_zh: '\u7f51\u9875\u6d4f\u89c8\u4e0e\u6293\u53d6\u5de5\u5177', version: '1.5.0', homepage: 'https://clawhub.ai/skills/browser', tags: ['developer', 'automation'], downloads: 28000, stars: 120, score: 82000, owner: 'clawhub', updated_at: 1772065840450 },
        { slug: 'google-maps', name: 'Google Maps', description: 'Google Maps integration', description_zh: '\u8c37\u6b4c\u5730\u56fe\u96c6\u6210', version: '1.0.0', homepage: 'https://clawhub.ai/skills/google-maps', tags: ['data', 'visualization'], downloads: 12000, stars: 65, score: 35000, owner: 'clawhub', updated_at: 1772065840450 },
        { slug: 'skill-vetter', name: 'Skill Vetter', description: 'Skill quality assessment tool', description_zh: '\u6280\u80fd\u8d28\u91cf\u8bc4\u4f30\u5de5\u5177', version: '0.9.0', homepage: 'https://clawhub.ai/skills/skill-vetter', tags: ['security', 'audit'], downloads: 8000, stars: 42, score: 22000, owner: 'clawhub', updated_at: 1772065840450 },
      ],
    };
  },
  getSkillHubStatus: async () => {
    await delay(150);
    return mockSkillHubCliInstalled
      ? { ok: true, installed: true, binPath: '/Users/demo/.local/bin/skillhub', installGuideURL: 'https://skillhub.tencent.com/', skillInstallCommand: 'skillhub install <slug>' }
      : { ok: true, installed: false, installGuideURL: 'https://skillhub.tencent.com/', skillInstallCommand: 'skillhub install <slug>', error: 'SkillHub CLI not found; install SkillHub CLI first' };
  },
  installSkillHubCLI: async () => {
    await delay(1400);
    mockSkillHubCliInstalled = true;
    return { ok: true, installed: true, binPath: '/Users/demo/.local/bin/skillhub', output: 'installed cli' };
  },
  installSkillHubSkill: async (skillId: string, _agentId?: string, _installTarget?: 'agent' | 'global') => {
    await delay(1000);
    if (!mockSkillHubCliInstalled) {
      return { ok: false, error: 'SkillHub CLI not found; install SkillHub CLI first', needsCLI: true };
    }
    return { ok: true, skillId, agentId: _agentId || FAKE_AGENTS.default, installTarget: _installTarget || 'agent', output: `installed ${skillId}` };
  },
  getCronJobs: async () => { await delay(200); return { ok: true, jobs: FAKE_CRON_JOBS }; },
  updateCronJobs: async () => { await delay(300); return { ok: true }; },
  getDocs: async () => { await delay(200); return { ok: true, docs: [{ name: 'system-prompt.md', path: 'system-prompt.md', content: '# System Prompt\n\nYou are OpenClaw, a helpful AI assistant.' }] }; },
  saveDoc: async () => { await delay(300); return { ok: true }; },
  getIdentityDocs: async () => { await delay(200); return { ok: true, docs: [{ name: 'identity.md', path: 'identity/identity.md', content: '# Identity\n\nI am OpenClaw, an AI-powered chatbot manager.' }] }; },
  saveIdentityDoc: async () => { await delay(300); return { ok: true }; },
  checkUpdate: async () => { await delay(1000); return { ok: true, updateAvailable: false, currentVersion: '4.2.1', latestVersion: '4.2.1' }; },
  doUpdate: async () => { await delay(2000); return { ok: true }; },
  getUpdateStatus: async () => { await delay(100); return { ok: true, status: 'idle' }; },
  getUpdatePopup: async () => { await delay(80); return { ok: true, show: false, version: '', releaseNote: '' }; },
  markUpdatePopupShown: async () => { await delay(50); return { ok: true }; },
  restartGateway: async () => { await delay(500); return { ok: true }; },
  getRestartGatewayStatus: async () => { await delay(100); return { ok: true, status: 'ok' }; },
  getAdminToken: async () => { await delay(100); return { ok: true, token: 'demo-admin-token' }; },
  getSudoPassword: async () => { await delay(100); return { ok: true, password: '' }; },
  setSudoPassword: async () => { await delay(200); return { ok: true }; },
  toggleSkill: async (_id: string, _enabled: boolean) => { await delay(200); return { ok: true }; },
  getEvents: async () => { await delay(200); return { ok: true, events: FAKE_LOGS }; },
  clearEvents: async () => { await delay(100); return { ok: true }; },
  getTasks: async () => { await delay(80); return { ok: true, tasks: [] }; },
  getTaskDetail: async (_id: string) => { await delay(80); return { ok: true, task: null }; },

  // --- Plugin Center ---
  getPluginList: async () => { await delay(200); return { ok: true, plugins: JSON.parse(JSON.stringify(FAKE_PLUGINS)) }; },
  getInstalledPlugins: async () => { await delay(150); return { ok: true, plugins: FAKE_PLUGINS.filter(p => p.installed) }; },
  getPluginDetail: async (id: string) => { await delay(150); const p = FAKE_PLUGINS.find(x => x.id === id); return p ? { ok: true, plugin: JSON.parse(JSON.stringify(p)) } : { ok: false, error: 'not found' }; },
  refreshPluginRegistry: async () => { await delay(800); return { ok: true }; },
  installPlugin: async (pluginId: string, _source?: string) => { await delay(1000); return { ok: true, pluginId }; },
  uninstallPlugin: async (_id: string, _cleanupConfig?: boolean) => { await delay(500); return { ok: true }; },
  togglePlugin: async (_id: string, _enabled: boolean) => { await delay(200); return { ok: true }; },
  getPluginConfig: async (_id: string) => { await delay(150); return { ok: true, config: {} }; },
  updatePluginConfig: async (_id: string, _config: any) => { await delay(200); return { ok: true }; },
  getPluginLogs: async (_id: string) => { await delay(200); return { ok: true, logs: [] }; },
  updatePluginVersion: async (_id: string) => { await delay(1000); return { ok: true }; },

  // --- Workflow Center ---
  getWorkflowSettings: async () => { await delay(100); return { ok: true, settings: { enabled: true, maxConcurrent: 3, defaultTimeout: 300 } }; },
  updateWorkflowSettings: async (_data: any) => { await delay(200); return { ok: true }; },
  getWorkflowTemplates: async () => { await delay(200); return { ok: true, templates: JSON.parse(JSON.stringify(FAKE_WORKFLOW_TEMPLATES)) }; },
  saveWorkflowTemplate: async (_template: any) => { await delay(300); return { ok: true }; },
  deleteWorkflowTemplate: async (_id: string) => { await delay(200); return { ok: true }; },
  generateWorkflowTemplate: async (_prompt: string, _category?: string, _settings?: any) => { await delay(2000); return { ok: true, template: { id: 'tpl-gen', name: '生成的模板', steps: [{ key: 'step1', type: 'agent', prompt: '执行任务' }] } }; },
  getWorkflowRuns: async (_status?: string) => { await delay(200); return { ok: true, runs: JSON.parse(JSON.stringify(FAKE_WORKFLOW_RUNS)) }; },
  getWorkflowRun: async (id: string) => { await delay(150); const r = FAKE_WORKFLOW_RUNS.find(x => x.id === id); return r ? { ok: true, run: JSON.parse(JSON.stringify(r)) } : { ok: false, error: 'not found' }; },
  startWorkflowRun: async (_templateId: string, _data?: any) => { await delay(500); return { ok: true, runId: 'run-new-' + Date.now() }; },
  controlWorkflowRun: async (_id: string, _action: string, _reply?: string) => { await delay(300); return { ok: true }; },
  resendWorkflowArtifact: async (_id: string, _data: any) => { await delay(300); return { ok: true }; },
  deleteWorkflowRun: async (_id: string) => { await delay(200); return { ok: true }; },

  // --- Sessions ---
  getSessions: async (_agent?: string) => {
    await delay(200);
    const sessions = !_agent || _agent === 'all'
      ? fakeSessions
      : fakeSessions.filter(session => session.agentId === _agent);
    return { ok: true, sessions: JSON.parse(JSON.stringify(sessions)) };
  },
  getSessionDetail: async (id: string, _agent?: string) => {
    await delay(150);
    const matches = findFakeSessionsById(id, _agent);
    if (matches.length === 0) return { ok: false, error: 'not found' };
    if ((!_agent || _agent === 'all') && matches.length > 1) {
      return { ok: false, error: 'ambiguous session' };
    }
    const session = matches[0];
    return { ok: true, messages: JSON.parse(JSON.stringify(fakeSessionMessages[getFakeSessionIdentity(session.agentId, id)] || [])) };
  },
  deleteSession: async (_id: string, _agent?: string) => {
    await delay(200);
    const matches = findFakeSessionsById(_id, _agent);
    if (matches.length === 0) return { ok: false, error: 'not found' };
    if ((!_agent || _agent === 'all') && matches.length > 1) {
      return { ok: false, error: 'ambiguous session' };
    }
    const deleted = matches[0];
    fakeSessions = fakeSessions.filter(item => !(item.sessionId === _id && item.agentId === deleted.agentId));
    delete fakeSessionMessages[getFakeSessionIdentity(deleted.agentId, _id)];
    saveDemoState(DEMO_SESSIONS_STORAGE_KEY, fakeSessions);
    saveDemoState(DEMO_SESSION_MESSAGES_STORAGE_KEY, fakeSessionMessages);
    return { ok: true };
  },

  // --- Software & OpenClaw Instances ---
  getSoftwareList: async () => { await delay(300); return { ok: true, software: [{ id: 'openclaw', name: 'OpenClaw', version: '4.2.1', installed: true }, { id: 'napcat', name: 'NapCat', version: '2.5.0', installed: true }, { id: 'node', name: 'Node.js', version: 'v20.11.0', installed: true }] }; },
  getOpenClawInstances: async () => { await delay(200); return { ok: true, instances: [{ path: '/usr/local/bin/openclaw', version: '4.2.1', active: true }] }; },
  installSoftware: async (_software: string) => { await delay(3000); return { ok: true }; },

  // --- Panel Update ---
  getPanelVersion: async () => { await delay(100); return { ok: true, version: '5.2.10', edition: 'pro', buildTime: new Date().toISOString() }; },
  checkPanelUpdate: async () => { await delay(1000); return { ok: true, hasUpdate: false, currentVersion: '5.2.10', latestVersion: '5.2.10', edition: 'pro' }; },
  doPanelUpdate: async () => { await delay(2000); return { ok: true }; },
  getPanelUpdateProgress: async () => { await delay(100); return { ok: true, status: 'idle', progress: 0 }; },
  generateUpdateToken: async () => { await delay(200); return { ok: true, token: 'demo-update-token' }; },
  getUpdateHistory: async () => { await delay(200); return { ok: true, history: [] }; },

  // --- System Diagnostics ---
  systemDiagnose: async () => { await delay(500); return { ok: true, results: [{ id: 'config-valid', label: '配置文件格式', status: 'pass' }, { id: 'binary-found', label: 'OpenClaw 可执行文件', status: 'pass' }, { id: 'node-version', label: 'Node.js 版本', status: 'pass', detail: 'v20.11.0' }] }; },
  checkConfig: async () => { await delay(300); return { ok: true, issues: [] }; },
  fixConfig: async (_issueIds: string[]) => { await delay(500); return { ok: true, fixed: [] }; },

  // --- NapCat Advanced ---
  napcatRestart: async () => { await delay(500); return { ok: true }; },
  napcatStatus: async () => { await delay(100); return { ok: true, connected: true, selfId: '2854196310', nickname: 'Demo Bot' }; },
  napcatReconnectLogs: async () => { await delay(200); return { ok: true, logs: [] }; },
  napcatReconnect: async () => { await delay(500); return { ok: true }; },
  napcatMonitorConfig: async (_data: any) => { await delay(200); return { ok: true }; },
  napcatDiagnose: async (_repair?: boolean) => { await delay(2000); return { ok: true, results: [{ id: 'napcat-conn', label: '连接状态', status: 'pass' }] }; },

  // --- QQ Channel ---
  getQQChannelState: async () => { await delay(200); return { ok: true, state: 'not_configured' }; },
  setupQQChannel: async () => { await delay(1000); return { ok: true }; },
  repairQQChannel: async () => { await delay(1000); return { ok: true }; },
  cleanupQQChannel: async () => { await delay(500); return { ok: true }; },
  deleteQQChannel: async () => { await delay(500); return { ok: true }; },

  // --- Misc ---
  checkModelHealth: async (_baseUrl: string, _apiKey: string, _apiType: string, _modelId?: string) => { await delay(1500); return { ok: true, healthy: true, latencyMs: 320, model: _modelId || 'default' }; },
  aiChat: async (_messages: any[], _providerId?: string, _modelId?: string) => { await delay(1000); return { ok: true, reply: { role: 'assistant', content: '这是 Demo 模式的 AI 回复。在前端开发模式下，AI 聊天功能返回模拟数据。' } }; },
  getPanelChatSessions: async () => { await delay(120); return { ok: true, sessions: JSON.parse(JSON.stringify(fakePanelChatSessions)) }; },
  createPanelChatSession: async (data?: { title?: string; chatType?: 'direct' | 'group'; agentId?: string; agentIds?: string[]; summaryAgentId?: string; sharedContextPaths?: string[] }) => {
    await delay(180);
    const now = Date.now();
    const participantAgentIds = Array.from(new Set((data?.agentIds?.length ? data.agentIds : [data?.agentId || 'main']).filter(Boolean)));
    const participants = participantAgentIds.map((agentId, index) => ({ agentId, name: agentId, roleType: data?.summaryAgentId === agentId ? 'summary' : 'assistant', orderIndex: index, autoReply: true, enabled: true, isSummary: data?.summaryAgentId === agentId }));
    const session = {
      id: `${(data?.chatType === 'group' || participantAgentIds.length > 1) ? 'group' : 'panel'}-demo-${now}`,
      openclawSessionId: `panel-demo-${now}`,
      agentId: participantAgentIds[0] || data?.agentId || 'main',
      chatType: (data?.chatType === 'group' || participantAgentIds.length > 1) ? 'group' : 'direct',
      title: data?.title || '新对话',
      createdAt: now,
      updatedAt: now,
      messageCount: 0,
      lastMessage: '',
      participantCount: participants.length,
      participants,
      summaryAgentId: data?.summaryAgentId || '',
      sharedContexts: (data?.sharedContextPaths || []).map(path => ({ path, title: path.split('/').pop() || path })),
    };
    fakePanelChatSessions = [session, ...fakePanelChatSessions];
    fakePanelChatMessages[session.id] = [];
    return { ok: true, session };
  },
  getPanelChatSessionDetail: async (id: string) => {
    await delay(120);
    const session = fakePanelChatSessions.find(item => item.id === id);
    if (!session) return { ok: false, error: 'not found' };
    return { ok: true, session, participants: JSON.parse(JSON.stringify(session.participants || [])), sharedContexts: JSON.parse(JSON.stringify(session.sharedContexts || [])), messages: JSON.parse(JSON.stringify(fakePanelChatMessages[id] || [])) };
  },
  renamePanelChatSession: async (id: string, title: string) => {
    await delay(120);
    const session = fakePanelChatSessions.find(item => item.id === id);
    if (!session) return { ok: false, error: 'not found' };
    session.title = title || '新对话';
    return { ok: true, session };
  },
  sendPanelChatMessage: async (id: string, message: string) => {
    await delay(900);
    const session = fakePanelChatSessions.find(item => item.id === id);
    if (!session) return { ok: false, error: 'not found' };
    const now = Date.now();
    const participants = Array.isArray(session.participants) && session.participants.length > 0 ? session.participants : [{ agentId: session.agentId, name: session.agentId, isSummary: false }];
    const userMessage = { id: `user-${now}`, role: 'user', senderType: 'user', messageType: 'chat', content: message, timestamp: new Date(now).toISOString() };
    const botMessages = participants.map((participant: any, index: number) => ({ id: `assistant-${participant.agentId}-${now + index}`, role: 'assistant', senderType: 'agent', agentId: participant.agentId, agentName: participant.name || participant.agentId, messageType: participant.isSummary ? 'summary' : 'chat', content: participant.isSummary ? `总结 AI 已汇总本轮讨论：用户提问“${message}”，建议优先执行并统一输出。` : `Demo 模式下，${participant.name || participant.agentId} 的回复：\n\n我对“${message}”的补充意见是这里会展示不同 AI 的顺序回复。`, timestamp: new Date(now + (index + 1) * 1000).toISOString() }));
    fakePanelChatMessages[id] = [...(fakePanelChatMessages[id] || []), userMessage, ...botMessages];
    session.updatedAt = now + 1000;
    session.messageCount = fakePanelChatMessages[id].length;
    session.lastMessage = message;
    if (!session.title || session.title === '新对话') {
      session.title = message.slice(0, 20) || '新对话';
    }
    fakePanelChatSessions = [session, ...fakePanelChatSessions.filter(item => item.id !== id)];
    return { ok: true, session, participants: JSON.parse(JSON.stringify(participants)), messages: JSON.parse(JSON.stringify(fakePanelChatMessages[id])), reply: botMessages[botMessages.length - 1]?.content || '' };
  },
  deletePanelChatSession: async (id: string) => {
    await delay(150);
    fakePanelChatSessions = fakePanelChatSessions.filter(item => item.id !== id);
    delete fakePanelChatMessages[id];
    return { ok: true };
  },
  getPanelChatSessionSharedContext: async (id: string) => {
    await delay(120);
    const session = fakePanelChatSessions.find(item => item.id === id);
    if (!session) return { ok: false, error: 'not found' };
    return { ok: true, sharedContexts: JSON.parse(JSON.stringify(session.sharedContexts || [])) };
  },
  updatePanelChatSessionSharedContext: async (id: string, paths: string[]) => {
    await delay(120);
    const session = fakePanelChatSessions.find(item => item.id === id);
    if (!session) return { ok: false, error: 'not found' };
    session.sharedContexts = (paths || []).map(path => ({ path, title: path.split('/').pop() || path }));
    return { ok: true, sharedContexts: JSON.parse(JSON.stringify(session.sharedContexts || [])) };
  },
  getPanelChatAgentKnowledgeBindings: async (_agentId: string) => {
    await delay(120);
    return { ok: true, bindings: [] };
  },
  updatePanelChatAgentKnowledgeBindings: async (_agentId: string, paths: string[]) => {
    await delay(120);
    return { ok: true, bindings: (paths || []).map(path => ({ path, title: path.split('/').pop() || path })) };
  },
  getCompanyOverview: async () => {
    await delay(120);
    return { ok: true, overview: { teamCount: fakeCompanyTeams.length, taskCount: fakeCompanyTasks.length, runningCount: fakeCompanyTasks.filter(item => item.status === 'running').length, completedCount: fakeCompanyTasks.filter(item => item.status === 'completed').length, recentTasks: JSON.parse(JSON.stringify(fakeCompanyTasks.slice(0, 8))) } };
  },
  getCompanyChannels: async () => {
    await delay(120);
    return { ok: true, channels: [
      { channelType: 'panel_chat', label: '面板聊天', sourceReady: true, deliveryReady: true, status: 'active' },
      { channelType: 'panel_manual', label: '面板手工创建', sourceReady: true, deliveryReady: false, status: 'active' },
      { channelType: 'qq', label: 'QQ (NapCat)', sourceReady: false, deliveryReady: false, status: 'reserved' },
      { channelType: 'qqbot', label: 'QQ 官方机器人', sourceReady: false, deliveryReady: false, status: 'reserved' },
      { channelType: 'wechat', label: '微信', sourceReady: false, deliveryReady: false, status: 'reserved' },
      { channelType: 'feishu', label: '飞书 / Lark', sourceReady: false, deliveryReady: false, status: 'reserved' },
      { channelType: 'wecom', label: '企业微信（机器人）', sourceReady: false, deliveryReady: false, status: 'reserved' },
      { channelType: 'wecom-app', label: '企业微信（自建应用）', sourceReady: false, deliveryReady: false, status: 'reserved' },
      { channelType: 'api', label: 'API', sourceReady: false, deliveryReady: false, status: 'reserved' },
      { channelType: 'webhook', label: 'Webhook', sourceReady: false, deliveryReady: false, status: 'reserved' },
    ] };
  },
  getCompanyCapabilities: async () => {
    await delay(120);
    return { ok: true, sourceTypes: ['panel_chat', 'panel_manual', 'qq', 'qqbot', 'wechat', 'feishu', 'wecom', 'wecom-app', 'api', 'webhook'], deliveryTypes: ['write_back_panel_session', 'notify_only', 'send_to_qq', 'send_to_qqbot', 'send_to_wechat', 'send_to_feishu', 'send_to_wecom', 'send_to_wecom_app', 'api_response', 'webhook_callback'] };
  },
  getCompanyTeams: async () => {
    await delay(120);
    return { ok: true, teams: JSON.parse(JSON.stringify(fakeCompanyTeams)) };
  },
  getCompanyTeamDetail: async (id: string) => {
    await delay(120);
    const team = fakeCompanyTeams.find(item => item.id === id) || fakeCompanyTeams[0];
    return { ok: true, team: JSON.parse(JSON.stringify(team)) };
  },
  createCompanyTeam: async (data: any) => {
    await delay(260);
    const now = Date.now();
    const managerAgentId = String(data?.managerAgentId || 'company_manager').trim() || 'company_manager';
    const workerAgentIds: string[] = Array.from(new Set((Array.isArray(data?.workerAgentIds) ? data.workerAgentIds : []).map((item: unknown) => String(item || '').trim()).filter((item: string) => Boolean(item)).filter((item: string) => item !== managerAgentId)));
    const team = {
      id: `company-team-${now}`,
      name: String(data?.name || '').trim() || '新执行团队',
      description: String(data?.description || '').trim(),
      managerAgentId,
      defaultSummaryAgentId: managerAgentId,
      status: 'active',
      agents: [
        { agentId: managerAgentId, agentName: managerAgentId === 'company_manager' ? 'AI Company Manager' : managerAgentId, roleType: 'manager', dutyLabel: '调度 / 汇总', enabled: true },
        ...workerAgentIds.map((agentId: string, index: number) => ({ agentId, agentName: agentId, roleType: 'worker', dutyLabel: `执行 ${index + 1}`, enabled: true, sortOrder: index })),
      ],
    };
    fakeCompanyTeams = [team, ...fakeCompanyTeams];
    return { ok: true, team: JSON.parse(JSON.stringify(team)) };
  },
  updateCompanyTeam: async (id: string, data: any) => {
    await delay(260);
    const target = fakeCompanyTeams.find(item => item.id === id);
    if (!target) return { ok: false, error: 'not found' };
    const managerAgentId = String(data?.managerAgentId || target.managerAgentId || 'company_manager').trim() || 'company_manager';
    const workerAgentIds: string[] = Array.from(new Set((Array.isArray(data?.workerAgentIds) ? data.workerAgentIds : []).map((item: unknown) => String(item || '').trim()).filter((item: string) => Boolean(item)).filter((item: string) => item !== managerAgentId)));
    target.name = String(data?.name || '').trim() || target.name;
    target.description = String(data?.description || '').trim();
    target.managerAgentId = managerAgentId;
    target.defaultSummaryAgentId = managerAgentId;
    target.agents = [
      { agentId: managerAgentId, agentName: managerAgentId === 'company_manager' ? 'AI Company Manager' : managerAgentId, roleType: 'manager', dutyLabel: '调度 / 汇总', enabled: true },
      ...workerAgentIds.map((agentId: string, index: number) => ({ agentId, agentName: agentId, roleType: 'worker', dutyLabel: `执行 ${index + 1}`, enabled: true, sortOrder: index })),
    ];
    fakeCompanyTeams = [...fakeCompanyTeams];
    return { ok: true, team: JSON.parse(JSON.stringify(target)) };
  },
  getCompanyTasks: async () => {
    await delay(120);
    return { ok: true, tasks: JSON.parse(JSON.stringify(fakeCompanyTasks)) };
  },
  createCompanyTask: async (data: any) => {
    await delay(900);
    const now = Date.now();
    const team = fakeCompanyTeams.find(item => item.id === (data?.teamId || 'default')) || fakeCompanyTeams[0];
    const workers = (Array.isArray(data?.workerAgentIds) && data.workerAgentIds.length > 0) ? data.workerAgentIds : team.agents.filter((item: any) => item.roleType !== 'manager').map((item: any) => item.agentId);
    const taskId = `company-task-${now}`;
    const steps = workers.map((worker: string, index: number) => ({ id: `${taskId}-step-${index + 1}`, taskId, stepKey: `step-${index + 1}`, title: `子任务 ${index + 1}`, instruction: `围绕目标执行：${data.goal}`, workerAgentId: worker, status: 'completed', orderIndex: index, outputText: `${worker} 已完成执行建议输出。`, createdAt: now, updatedAt: now + 600 + index * 120 }));
    const events = [
      { id: now + 1, taskId, eventType: 'task_created', message: '任务已创建', createdAt: now },
      { id: now + 2, taskId, eventType: 'task_planned', message: `Manager 已拆解 ${steps.length} 个步骤`, createdAt: now + 160 },
      ...steps.map((step: any, index: number) => ({ id: now + 10 + index, taskId, stepId: step.id, eventType: 'step_completed', message: `${step.workerAgentId} 已完成：${step.title}`, createdAt: now + 350 + index * 130 })),
      { id: now + 99, taskId, eventType: 'task_reviewed', message: 'Manager 已完成审核', createdAt: now + 1000 },
    ];
    const task = { id: taskId, teamId: team.id, title: data?.title || '新任务', goal: data.goal, status: 'completed', managerAgentId: data?.managerAgentId || team.managerAgentId, summaryAgentId: data?.summaryAgentId || team.defaultSummaryAgentId, sourceType: data?.sourceType || 'panel_manual', sourceRefId: data?.sourceRefId || '', deliveryType: data?.deliveryType || 'notify_only', deliveryTargetId: data?.deliveryTargetId || '', panelSessionId: data?.panelSessionId || '', resultText: `Manager 最终汇总：\n\n${data.goal}`, reviewResult: 'approved', reviewComment: 'Demo review', createdAt: now, updatedAt: now + 1000, steps, events };
    fakeCompanyTasks = [task, ...fakeCompanyTasks.filter(item => item.id !== taskId)];
    let messages: any[] = [];
    if ((data?.deliveryType || '') === 'write_back_panel_session' && data?.panelSessionId) {
      const additions = [
        { id: `system-${taskId}-1`, role: 'system', senderType: 'system', messageType: 'task_created', content: `已创建协作任务：${task.title}`, timestamp: new Date(now).toISOString(), taskId },
        { id: `system-${taskId}-2`, role: 'system', senderType: 'system', messageType: 'task_reviewed', content: 'Manager 已完成审核', timestamp: new Date(now + 800).toISOString(), taskId },
        { id: `assistant-${taskId}-1`, role: 'assistant', senderType: 'agent', messageType: 'task_summary', agentId: task.summaryAgentId, agentName: task.summaryAgentId, content: task.resultText, timestamp: new Date(now + 1000).toISOString(), taskId },
      ];
      fakePanelChatMessages[data.panelSessionId] = [...(fakePanelChatMessages[data.panelSessionId] || []), ...additions];
      messages = JSON.parse(JSON.stringify(fakePanelChatMessages[data.panelSessionId] || []));
      const session = fakePanelChatSessions.find(item => item.id === data.panelSessionId);
      if (session) {
        session.updatedAt = now + 1000;
        session.messageCount = fakePanelChatMessages[data.panelSessionId].length;
        session.lastMessage = data.goal;
      }
    }
    return { ok: true, task: JSON.parse(JSON.stringify(task)), messages, result: task.resultText };
  },
  getCompanyTaskDetail: async (id: string) => {
    await delay(120);
    const task = fakeCompanyTasks.find(item => item.id === id);
    return task ? { ok: true, task: JSON.parse(JSON.stringify(task)) } : { ok: false, error: 'not found' };
  },
  getCompanyTaskSteps: async (id: string) => {
    await delay(120);
    const task = fakeCompanyTasks.find(item => item.id === id);
    return { ok: true, steps: JSON.parse(JSON.stringify(task?.steps || [])) };
  },
  getCompanyTaskEvents: async (id: string) => {
    await delay(120);
    const task = fakeCompanyTasks.find(item => item.id === id);
    return { ok: true, events: JSON.parse(JSON.stringify(task?.events || [])) };
  },
  getCompanyTaskResult: async (id: string) => {
    await delay(120);
    const task = fakeCompanyTasks.find(item => item.id === id);
    return task ? { ok: true, result: task.resultText, reviewResult: task.reviewResult, reviewComment: task.reviewComment } : { ok: false, error: 'not found' };
  },
  restartProcess: async () => { await delay(500); return { ok: true }; },
  restartPanel: async () => { await delay(500); return { ok: true }; },
  workspaceDownloadUrl: (_filePath: string) => '#',
  workspacePreviewUrl: (_filePath: string) => '/logo.jpg',
  agentIdentityAvatarUrl: (_agentId: string) => '/logo.jpg',
};

// Fake WebSocket data for demo
export const DEMO_LOG_ENTRIES = FAKE_LOGS;
export const DEMO_NAPCAT_STATUS = { connected: true, selfId: '2854196310', nickname: 'OpenClaw Demo Bot', groupCount: 12, friendCount: 86 };
export const DEMO_WECHAT_STATUS = { loggedIn: false };

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
  { id: 'web-search', skillKey: 'web-search', name: 'Web Search', description: '联网搜索能力，支持Google/Bing/DuckDuckGo', enabled: true, source: 'app-skill', version: '1.2.0', requires: { env: ['SEARCH_API_KEY'], bins: [] } },
  { id: 'image-gen', skillKey: 'image-gen', name: 'Image Generation', description: 'AI图像生成，支持DALL-E/Stable Diffusion', enabled: true, source: 'app-skill', version: '1.0.3', requires: { env: ['OPENAI_API_KEY'], bins: [] } },
  { id: 'code-runner', skillKey: 'code-runner', name: 'Code Runner', description: '安全沙箱代码执行，支持Python/JS/Shell', enabled: true, source: 'managed', version: '0.9.1', requires: { env: [], bins: ['python3', 'node'] } },
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
  ],
  bindings: [
    { type: 'route', agentId: 'work', comment: 'work-group', match: { channel: 'qq', peer: { kind: 'group', id: '123' } } },
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
  },
  session: {
    dmScope: 'per-peer',
  },
  plugins: {
    entries: {
      feishu: { enabled: true },
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

const HELP_ONBOARDING_KEY = 'mock.help.onboarding.status';
const HELP_ONBOARDING_POLICY_KEY = 'mock.help.onboarding.policy';
const HELP_CONFIG_KEY = 'mock.help.config';

function getMockOnboardingStore(): Record<string, { completed: boolean; completed_at: string | null }> {
  try {
    return JSON.parse(localStorage.getItem(HELP_ONBOARDING_KEY) || '{}');
  } catch {
    return {};
  }
}

function setMockOnboardingStore(store: Record<string, { completed: boolean; completed_at: string | null }>) {
  localStorage.setItem(HELP_ONBOARDING_KEY, JSON.stringify(store));
}

function getMockOnboardingPolicy() {
  try {
    const raw = JSON.parse(localStorage.getItem(HELP_ONBOARDING_POLICY_KEY) || '{}');
    return {
      version: raw.version || 'v1',
      enabled_at: raw.enabled_at || new Date(Date.now() - 86400000).toISOString(),
      force_token: raw.force_token || 'base',
      force_updated_at: raw.force_updated_at || null,
    };
  } catch {
    return {
      version: 'v1',
      enabled_at: new Date(Date.now() - 86400000).toISOString(),
      force_token: 'base',
      force_updated_at: null,
    };
  }
}

function setMockOnboardingPolicy(policy: { version: string; enabled_at: string; force_token: string; force_updated_at: string | null }) {
  localStorage.setItem(HELP_ONBOARDING_POLICY_KEY, JSON.stringify(policy));
}

function getMockHelpConfig() {
  try {
    const raw = JSON.parse(localStorage.getItem(HELP_CONFIG_KEY) || '{}');
    return {
      docs_default_lang: raw.docs_default_lang || 'zh-CN',
      search_backend: raw.search_backend === 'backend' ? 'backend' : 'frontend',
    };
  } catch {
    return {
      docs_default_lang: 'zh-CN',
      search_backend: 'frontend',
    };
  }
}

function setMockHelpConfig(config: { docs_default_lang: string; search_backend: 'frontend' | 'backend' }) {
  localStorage.setItem(HELP_CONFIG_KEY, JSON.stringify(config));
}

function slugifyHeading(text: string): string {
  const normalized = text.trim().toLowerCase().replace(/\s+/g, '-');
  return encodeURIComponent(normalized).replace(/%/g, '') || 'heading';
}

async function searchReadmeMarkdown(query: string, limit = 20) {
  const q = query.trim().toLowerCase();
  if (!q) return [];

  let markdown = '';
  try {
    const res = await fetch('/README.md');
    if (res.ok) markdown = await res.text();
  } catch {}

  if (!markdown) {
    return [
      {
        sectionId: 'clawpanel',
        sectionTitle: 'ClawPanel',
        sectionPath: 'ClawPanel',
        snippet: 'ClawPanel 在线帮助文档搜索当前使用 mock 数据返回。',
        score: 1,
      },
    ].filter(item => `${item.sectionTitle} ${item.snippet}`.toLowerCase().includes(q)).slice(0, limit);
  }

  const lines = markdown.split('\n');
  const results: Array<{ sectionId: string; sectionTitle: string; sectionPath: string; snippet: string; score: number }> = [];
  const stack: Array<{ id: string; text: string; level: number }> = [];
  let inCodeFence = false;

  lines.forEach((raw) => {
    const line = raw.trim();
    if (/^```/.test(line)) {
      inCodeFence = !inCodeFence;
      return;
    }
    if (inCodeFence || !line) return;

    const headingMatch = raw.match(/^(#{1,6})\s+(.*)$/);
    if (headingMatch) {
      const level = headingMatch[1].length;
      const text = headingMatch[2].trim();
      while (stack.length > 0 && stack[stack.length - 1].level >= level) stack.pop();
      stack.push({ id: slugifyHeading(text), text, level });
      return;
    }

    if (!line.toLowerCase().includes(q)) return;
    const current = stack[stack.length - 1];
    if (!current) return;
    results.push({
      sectionId: current.id,
      sectionTitle: current.text,
      sectionPath: stack.map(item => item.text).join(' > '),
      snippet: line.slice(0, 240),
      score: 1,
    });
  });

  return results.slice(0, limit);
}

const FAKE_SESSIONS = [
  { id: 'sess-1', agentId: 'main', type: 'dm', peer: 'ou_demo_a', messageCount: 24, createdAt: Date.now() - 86400000, lastMessageAt: Date.now() - 3600000 },
  { id: 'sess-2', agentId: 'main', type: 'group', peer: 'oc_demo_group', messageCount: 8, createdAt: Date.now() - 172800000, lastMessageAt: Date.now() - 7200000 },
  { id: 'sess-3', agentId: 'work', type: 'dm', peer: 'ou_demo_b', messageCount: 15, createdAt: Date.now() - 259200000, lastMessageAt: Date.now() - 86400000 },
];

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
      openclaw: { configured: true, currentModel: 'deepseek/deepseek-chat', enabledChannels: [
        { id: 'qq', label: 'QQ (NapCat)', type: 'builtin' },
        { id: 'telegram', label: 'Telegram', type: 'plugin' },
      ] },
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
  syncClawHub: async () => { await delay(800); return { ok: true, skills: [] }; },
  searchClawHub: async (query?: string, _agentId?: string) => {
    await delay(500);
    const q = (query || '').toLowerCase();
    const skills = q
      ? FAKE_CLAWHUB_SKILLS.filter(skill =>
          skill.id.toLowerCase().includes(q) ||
          skill.name.toLowerCase().includes(q) ||
          skill.description.toLowerCase().includes(q),
        )
      : FAKE_CLAWHUB_SKILLS;
    return { ok: true, registryBase: 'https://clawhub.ai', skills: JSON.parse(JSON.stringify(skills)) };
  },
  installClawHubSkill: async (skillId: string, _agentId?: string) => {
    await delay(800);
    return { ok: true, skillId, agentId: _agentId || FAKE_AGENTS.default, version: '1.0.0' };
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

  // --- Help System ---
  getHelpOnboardingPolicy: async () => { await delay(120); return { ok: true, data: getMockOnboardingPolicy() }; },
  updateHelpOnboardingPolicy: async (data: { version: string }) => {
    await delay(140);
    const now = new Date().toISOString();
    const current = getMockOnboardingPolicy();
    const next = {
      version: data.version || current.version || 'v1',
      enabled_at: now,
      force_token: now,
      force_updated_at: now,
    };
    setMockOnboardingPolicy(next);
    return { ok: true, data: next };
  },
  resetHelpOnboarding: async (data?: { user_id?: string; version?: string }) => {
    await delay(140);
    if (data?.version) {
      const store = getMockOnboardingStore();
      delete store[data.version];
      setMockOnboardingStore(store);
    } else {
      setMockOnboardingStore({});
    }
    return { ok: true, data: { affected: 1 } };
  },
  getHelpOnboardingStatus: async (version?: string) => {
    await delay(120);
    const policy = getMockOnboardingPolicy();
    const key = version || policy.version;
    const store = getMockOnboardingStore();
    const row = store[key] || { completed: false, completed_at: null };
    return { ok: true, data: { user_id: 'demo', version: key, completed: row.completed, completed_at: row.completed_at } };
  },
  completeHelpOnboarding: async (version: string) => {
    await delay(120);
    const store = getMockOnboardingStore();
    store[version] = { completed: true, completed_at: new Date().toISOString() };
    setMockOnboardingStore(store);
    return { ok: true, data: { user_id: 'demo', version, completed: true, completed_at: store[version].completed_at } };
  },
  getHelpConfig: async () => { await delay(120); return { ok: true, data: getMockHelpConfig() }; },
  updateHelpConfig: async (data: { docs_default_lang: string; search_backend: 'frontend' | 'backend' }) => {
    await delay(140);
    const next: { docs_default_lang: string; search_backend: 'frontend' | 'backend' } = {
      docs_default_lang: data.docs_default_lang || 'zh-CN',
      search_backend: data.search_backend === 'backend' ? 'backend' : 'frontend',
    };
    setMockHelpConfig(next);
    return { ok: true, data: next };
  },
  searchHelpDocs: async (q: string, _lang?: string, limit = 20, _docId?: string) => {
    await delay(120);
    const results = await searchReadmeMarkdown(q, limit);
    return { ok: true, data: results };
  },

  // --- Sessions ---
  getSessions: async (_agent?: string) => { await delay(200); const sessions = _agent ? FAKE_SESSIONS.filter(s => s.agentId === _agent) : FAKE_SESSIONS; return { ok: true, sessions: JSON.parse(JSON.stringify(sessions)) }; },
  getSessionDetail: async (id: string, _agent?: string) => { await delay(150); const s = FAKE_SESSIONS.find(x => x.id === id); return s ? { ok: true, session: { ...s, messages: [{ role: 'user', content: 'Hello', timestamp: Date.now() - 60000 }, { role: 'assistant', content: '你好！有什么可以帮你的？', timestamp: Date.now() - 59000 }] } } : { ok: false, error: 'not found' }; },
  deleteSession: async (_id: string, _agent?: string) => { await delay(200); return { ok: true }; },

  // --- Software & OpenClaw Instances ---
  getSoftwareList: async () => { await delay(300); return { ok: true, software: [{ id: 'openclaw', name: 'OpenClaw', version: '4.2.1', installed: true }, { id: 'napcat', name: 'NapCat', version: '2.5.0', installed: true }, { id: 'node', name: 'Node.js', version: 'v20.11.0', installed: true }] }; },
  getOpenClawInstances: async () => { await delay(200); return { ok: true, instances: [{ path: '/usr/local/bin/openclaw', version: '4.2.1', active: true }] }; },
  installSoftware: async (_software: string) => { await delay(3000); return { ok: true }; },

  // --- Panel Update ---
  getPanelVersion: async () => { await delay(100); return { ok: true, version: '5.0.0', buildTime: new Date().toISOString() }; },
  checkPanelUpdate: async () => { await delay(1000); return { ok: true, updateAvailable: false, currentVersion: '5.0.0', latestVersion: '5.0.0' }; },
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

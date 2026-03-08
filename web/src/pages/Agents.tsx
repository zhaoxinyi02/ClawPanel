import { useEffect, useMemo, useRef, useState } from 'react';
import { api } from '../lib/api';
import { Plus, RefreshCw, Save, Trash2, ArrowUp, ArrowDown, Route, Bot, Settings, Brain, Shield, ChevronDown, ChevronRight, Sparkles, FileText } from 'lucide-react';

interface AgentItem {
  id: string;
  name?: string;
  workspace?: string;
  agentDir?: string;
  model?: any;
  contextTokens?: number;
  compaction?: any;
  tools?: any;
  sandbox?: any;
  groupChat?: any;
  identity?: any;
  subagents?: any;
  params?: any;
  runtime?: any;
  default?: boolean;
  implicit?: boolean;
  sessions?: number;
  lastActive?: number;
}

type AgentFormSection = 'basic' | 'behavior' | 'access' | 'collaboration' | 'advanced';
type InheritToggle = 'inherit' | 'enabled' | 'disabled';

interface AgentFormState {
  id: string;
  name: string;
  workspace: string;
  agentDir: string;
  isDefault: boolean;
  modelPrimary: string;
  modelFallbacks: string;
  paramTemperature: string;
  paramTopP: string;
  paramMaxTokens: string;
  contextTokens: string;
  compactionMode: '' | 'default' | 'safeguard';
  compactionMaxHistoryShare: string;
  identityName: string;
  identityTheme: string;
  identityEmoji: string;
  identityAvatar: string;
  sandboxMode: '' | 'off' | 'non-main' | 'all';
  sandboxScope: '' | 'session' | 'agent' | 'shared';
  sandboxWorkspaceAccess: '' | 'none' | 'ro' | 'rw';
  sandboxWorkspaceRoot: string;
  sandboxDockerNetwork: string;
  sandboxDockerReadOnlyRoot: InheritToggle;
  sandboxDockerSetupCommand: string;
  sandboxDockerBinds: string;
  groupChatMode: InheritToggle;
  toolProfile: '' | 'minimal' | 'coding' | 'messaging' | 'full';
  toolAllow: string;
  toolDeny: string;
  agentToAgentMode: InheritToggle;
  agentToAgentAllow: string;
  sessionVisibility: '' | 'same-agent' | 'all-agents';
  subagentAllowAgents: string;
  modelText: string;
  toolsText: string;
  sandboxText: string;
  groupChatText: string;
  identityText: string;
  subagentsText: string;
  paramsText: string;
  runtimeText: string;
}

interface AgentStructuredTouchedState {
  model: boolean;
  params: boolean;
  context: boolean;
  identity: boolean;
  sandbox: boolean;
  groupChat: boolean;
  tools: boolean;
  subagents: boolean;
}

interface AgentDefaultsState {
  contextTokens?: number;
  compactionMode: '' | 'default' | 'safeguard';
  compactionMaxHistoryShare?: number;
}

interface BindingDraft {
  name: string;
  agent: string;
  enabled: boolean;
  match: Record<string, any>;
  matchText: string;
  mode: 'structured' | 'json';
  rowError?: string;
}

interface PreviewResult {
  agent?: string;
  matchedBy?: string;
  trace?: string[];
}

interface ChannelMeta {
  accounts: string[];
  defaultAccount?: string;
}

interface AgentModelResponse {
  ok?: boolean;
  providers?: Record<string, any>;
  defaults?: Record<string, any>;
  models?: {
    providers?: Record<string, any>;
  };
}

type AgentsWorkbenchView = 'directory' | 'routing';
type AgentDetailTab = 'overview' | 'model' | 'tools' | 'files' | 'capabilities' | 'context' | 'advanced';
type PreviewMetaKey = 'channel' | 'sender' | 'peer' | 'parentPeer' | 'guildId' | 'teamId' | 'accountId' | 'roles';
type NumberInputConstraint = { integer?: boolean; min?: number; max?: number };

interface AgentCoreFileEntry {
  name: string;
  path: string;
  exists?: boolean;
  size?: number;
  modified?: string;
  content: string;
}

interface SkillEntry {
  id: string;
  name: string;
  description?: string;
  enabled: boolean;
  source: string;
  version?: string;
}

interface CronJob {
  id: string;
  name: string;
  enabled: boolean;
  sessionTarget: string;
  schedule: { kind: string; expr?: string; everyMs?: number; atMs?: number; tz?: string };
  state?: { nextRunAtMs?: number; lastRunAtMs?: number; lastStatus?: string; lastError?: string };
}

interface SessionInfo {
  agentId?: string;
  key: string;
  sessionId: string;
  lastChannel: string;
  updatedAt: number;
  originLabel: string;
  messageCount: number;
}

const ALLOWED_MATCH_KEYS = ['channel', 'sender', 'peer', 'parentPeer', 'guildId', 'teamId', 'accountId', 'roles'];
const AGENT_FORM_SECTIONS: { id: AgentFormSection; title: string; description: string }[] = [
  { id: 'basic', title: '基础信息 (Basic)', description: '核心身份、工作区与默认关系' },
  { id: 'behavior', title: '行为设定 (Behavior)', description: '模型、常用参数与身份摘要' },
  { id: 'access', title: '访问与安全 (Access & Safety)', description: 'sandbox 与群聊行为' },
  { id: 'collaboration', title: '协作策略 (Collaboration)', description: '子 Agent 与跨 Agent 可见性' },
  { id: 'advanced', title: '高级 JSON (Advanced)', description: '完整 JSON 覆盖与 runtime' },
];
const SANDBOX_STARTERS = [
  { key: 'inherit', label: '继承默认', help: '不写 sandbox 覆盖，沿用全局或默认 Agent 配置。', text: '' },
  { key: 'non-main', label: '仅非主会话沙箱', help: '官方常见起步：主会话继续在宿主机运行，群聊 / channel / 非 main 会话进入 sandbox。', text: '{\n  "mode": "non-main",\n  "scope": "session",\n  "workspaceAccess": "none"\n}' },
  { key: 'read-only', label: '只读沙箱', help: '官方语义：始终启用 sandbox，并把 agent workspace 以只读方式挂载进去。', text: '{\n  "mode": "all",\n  "workspaceAccess": "ro"\n}' },
  { key: 'workspace-write', label: '工作区可写', help: '官方语义：始终启用 sandbox，并允许在 workspace 内读写。', text: '{\n  "mode": "all",\n  "workspaceAccess": "rw"\n}' },
  { key: 'danger-full-access', label: '高权限', help: '官方语义：关闭 sandbox，让工具直接在宿主机环境运行。', text: '{\n  "mode": "off"\n}' },
] as const;

const DEFAULT_AGENT_FORM: AgentFormState = {
  id: '',
  name: '',
  workspace: '',
  agentDir: '',
  isDefault: false,
  modelPrimary: '',
  modelFallbacks: '',
  paramTemperature: '',
  paramTopP: '',
  paramMaxTokens: '',
  contextTokens: '',
  compactionMode: '',
  compactionMaxHistoryShare: '',
  identityName: '',
  identityTheme: '',
  identityEmoji: '',
  identityAvatar: '',
  sandboxMode: '',
  sandboxScope: '',
  sandboxWorkspaceAccess: '',
  sandboxWorkspaceRoot: '',
  sandboxDockerNetwork: '',
  sandboxDockerReadOnlyRoot: 'inherit',
  sandboxDockerSetupCommand: '',
  sandboxDockerBinds: '',
  groupChatMode: 'inherit',
  toolProfile: '',
  toolAllow: '',
  toolDeny: '',
  agentToAgentMode: 'inherit',
  agentToAgentAllow: '',
  sessionVisibility: '',
  subagentAllowAgents: '',
  modelText: '',
  toolsText: '',
  sandboxText: '',
  groupChatText: '',
  identityText: '',
  subagentsText: '',
  paramsText: '',
  runtimeText: '',
};

const DEFAULT_AGENT_STRUCTURED_TOUCHED: AgentStructuredTouchedState = {
  model: false,
  params: false,
  context: false,
  identity: false,
  sandbox: false,
  groupChat: false,
  tools: false,
  subagents: false,
};

const EMPTY_AGENT_DEFAULTS: AgentDefaultsState = {
  compactionMode: '',
};

const AGENT_DIRECTORY_TABS: { id: AgentDetailTab; title: string; description: string }[] = [
  { id: 'overview', title: '概览 (Overview)', description: '先理解定位、默认关系与关键摘要' },
  { id: 'model', title: '模型与身份 (Model & Identity)', description: '模型、参数与身份口吻' },
  { id: 'tools', title: '工具与权限 (Tools & Access)', description: '工具策略、委派与 sandbox' },
  { id: 'files', title: '核心文件 (Core Files)', description: '查看并编辑工作区里的关键文档' },
  { id: 'capabilities', title: '技能与上下文 (Skills · Channels · Cron)', description: '补齐官方单 Agent 面板的运行态快照' },
  { id: 'context', title: '路由上下文 (Routing Context)', description: '查看它被哪些规则和会话引用' },
  { id: 'advanced', title: '高级 JSON (Advanced)', description: '需要完整 JSON 时再进入' },
];

const TOOL_POLICY_PRESETS: Array<{ id: '' | 'minimal' | 'coding' | 'messaging' | 'full'; label: string; help: string }> = [
  { id: '', label: 'Inherit', help: '不写 per-agent profile，继续继承全局工具策略。' },
  { id: 'minimal', label: 'Minimal', help: '只保留最小会话能力，适合作为最保守的起点。' },
  { id: 'coding', label: 'Coding', help: '偏向编码/文件/运行时工具，适合代码型 Agent。' },
  { id: 'messaging', label: 'Messaging', help: '偏向消息入口与渠道协作，适合路由型 Agent。' },
  { id: 'full', label: 'Full', help: '完整工具面；除非明确需要，否则不建议长期给高权限 Agent。' },
];

const AGENT_CORE_FILE_META: Record<string, { label: string; description: string }> = {
  'AGENTS.md': { label: '工作说明 (AGENTS.md)', description: '定义这个 Agent 的总任务边界与协作规则。' },
  'SOUL.md': { label: '人格内核 (SOUL.md)', description: '记录风格、价值观与长期行为倾向。' },
  'TOOLS.md': { label: '工具策略 (TOOLS.md)', description: '补充工具使用原则、限制与推荐模式。' },
  'IDENTITY.md': { label: '身份设定 (IDENTITY.md)', description: '描述角色、人设和对外表达方式。' },
  'USER.md': { label: '用户偏好 (USER.md)', description: '放业务方偏好、长期协作约定与注意事项。' },
  'HEARTBEAT.md': { label: '自检节奏 (HEARTBEAT.md)', description: '约定这个 Agent 的周期性自检或提醒。' },
  'BOOTSTRAP.md': { label: '启动引导 (BOOTSTRAP.md)', description: '首次加载时最应该先看的说明或初始化步骤。' },
  'MEMORY.md': { label: '记忆摘录 (MEMORY.md)', description: '存放长期记忆或需要沉淀给后续会话的内容。' },
};

const CHANNEL_DISPLAY_NAMES: Record<string, string> = {
  qq: 'QQ',
  discord: 'Discord',
  feishu: '飞书 (Feishu/Lark)',
  slack: 'Slack',
  telegram: 'Telegram',
  wechat: '微信 (WeChat)',
};

const DEFAULT_PREVIEW_META: Record<PreviewMetaKey, string> = {
  channel: '',
  sender: '',
  peer: '',
  parentPeer: '',
  guildId: '',
  teamId: '',
  accountId: '',
  roles: '',
};

const PREVIEW_FIELD_META: Record<PreviewMetaKey, { label: string; placeholder: string; help: string; listId?: string }> = {
  channel: {
    label: '通道',
    placeholder: '例如 qq / discord / feishu',
    help: '先确定消息来自哪个 channel，是所有路由判断的第一步。',
    listId: 'agent-channel-options',
  },
  accountId: {
    label: '账号',
    placeholder: '留空=按默认账号预览，*=全部账号',
    help: '留空时，面板会在已知 channel 下自动代入 defaultAccount；需要测试兜底规则时再写 *。',
  },
  sender: {
    label: '发送者',
    placeholder: '例如 +15551230001',
    help: '通常用于特定联系人、特定用户或机器人分流。',
  },
  peer: {
    label: '会话',
    placeholder: '例如 group:88001',
    help: '使用 kind:id 形式，例如 direct:alice 或 group:88001。',
  },
  parentPeer: {
    label: '上级会话',
    placeholder: '例如 thread:release-notes',
    help: '用于线程、子话题或父群组场景；不需要时留空。',
  },
  guildId: {
    label: 'Discord 服务器',
    placeholder: '例如 guild-01',
    help: '只在 Discord 等 server 场景有意义。',
  },
  teamId: {
    label: 'Slack 工作区',
    placeholder: '例如 workspace-main',
    help: '只在 Slack / team 场景有意义。',
  },
  roles: {
    label: '角色',
    placeholder: 'admin, maintainer',
    help: '多个角色可逗号分隔；通常需要与 Discord 服务器一起使用。',
  },
};

const PREVIEW_GROUPS: { title: string; description: string; fields: PreviewMetaKey[] }[] = [
  {
    title: '消息来源',
    description: '先描述消息来自哪个通道、哪个账号、谁发的。',
    fields: ['channel', 'accountId', 'sender'],
  },
  {
    title: '会话范围',
    description: '当你要验证群聊、频道、服务器或线程规则时，再补这些条件。',
    fields: ['peer', 'parentPeer', 'guildId', 'teamId', 'roles'],
  },
];

function isPlainObject(v: any): v is Record<string, any> {
  return !!v && typeof v === 'object' && !Array.isArray(v);
}

function deepClone<T>(v: T): T {
  return JSON.parse(JSON.stringify(v));
}

function splitPeer(raw: string): { kind: string; id: string } {
  const text = (raw || '').trim();
  if (!text) return { kind: '', id: '' };
  const parts = text.split(':');
  if (parts.length <= 1) return { kind: parts[0].trim(), id: '' };
  return { kind: parts[0].trim(), id: parts.slice(1).join(':').trim() };
}

function normalizePeerValue(v: any): { kind: string; id: string } | null {
  if (typeof v === 'string') {
    const out = splitPeer(v);
    if (!out.kind) return null;
    return out;
  }
  if (isPlainObject(v)) {
    const kind = String(v.kind ?? v.type ?? '').trim();
    let id = String(v.id ?? '').trim();
    if (!kind && typeof v.raw === 'string') {
      const out = splitPeer(v.raw);
      if (!out.kind) return null;
      return out;
    }
    if (!kind) return null;
    if (!id && typeof v.raw === 'string') {
      const out = splitPeer(v.raw);
      id = out.id;
    }
    return { kind, id };
  }
  return null;
}

function compactMatch(raw: any): Record<string, any> {
  if (!isPlainObject(raw)) return {};
  const out: Record<string, any> = {};
  for (const key of ALLOWED_MATCH_KEYS) {
    if (!(key in raw)) continue;
    const v = raw[key];
    if (v === undefined || v === null) continue;

    if (key === 'peer' || key === 'parentPeer') {
      if (isPlainObject(v)) {
        const peer = normalizePeerValue(v);
        if (!peer || !peer.kind) continue;
        out[key] = peer.id ? { kind: peer.kind, id: peer.id } : { kind: peer.kind };
        continue;
      }
      if (typeof v === 'string') {
        const s = v.trim();
        if (!s) continue;
        out[key] = s;
        continue;
      }
      if (Array.isArray(v)) {
        const arr = v.map(item => String(item).trim()).filter(Boolean);
        if (arr.length > 0) out[key] = arr;
      }
      continue;
    }

    if (Array.isArray(v)) {
      const arr = v.map(item => String(item).trim()).filter(Boolean);
      if (arr.length > 0) out[key] = arr;
      continue;
    }

    if (typeof v === 'string') {
      const s = v.trim();
      if (!s) continue;
      out[key] = s;
      continue;
    }

    out[key] = v;
  }
  return out;
}

function isStructuredMatchSupported(match: Record<string, any>): boolean {
  if (!isPlainObject(match)) return false;
  for (const key of Object.keys(match)) {
    if (!ALLOWED_MATCH_KEYS.includes(key)) return false;
    const v = match[key];
    if (key === 'roles') {
      if (typeof v === 'string') continue;
      if (Array.isArray(v) && v.every(item => typeof item === 'string')) continue;
      return false;
    }
    if (key === 'peer' || key === 'parentPeer') {
      if (typeof v === 'string') continue;
      if (isPlainObject(v)) {
        if (normalizePeerValue(v)) continue;
        return false;
      }
      return false;
    }
    if (typeof v === 'string') continue;
    return false;
  }
  return true;
}

function toBindingDraft(raw: any, fallbackAgent: string): BindingDraft {
  const match = compactMatch(isPlainObject(raw?.match) ? deepClone(raw.match) : {});
  const mode: 'structured' | 'json' = isStructuredMatchSupported(match) ? 'structured' : 'json';
  return {
    name: String(raw?.name || ''),
    agent: String(raw?.agentId || raw?.agent || fallbackAgent || 'main'),
    enabled: raw?.enabled !== false,
    match,
    mode,
    matchText: JSON.stringify(match, null, 2),
    rowError: '',
  };
}

function hasWildcard(raw: any): boolean {
  if (typeof raw === 'string') {
    const s = raw.trim();
    return /[*?\[\]]/.test(s);
  }
  if (Array.isArray(raw)) {
    return raw.some(hasWildcard);
  }
  return false;
}

function parseCSV(input: string): string[] {
  return (input || '').split(',').map(x => x.trim()).filter(Boolean);
}

function parseLineList(input: string): string[] {
  return (input || '').split('\n').map(x => x.trim()).filter(Boolean);
}

function stringifyLineList(raw: any): string {
  if (Array.isArray(raw)) return raw.map(item => String(item).trim()).filter(Boolean).join('\n');
  if (typeof raw === 'string') return raw.trim();
  return '';
}

function matchPriorityLabel(matchRaw: any): string {
  const match = compactMatch(matchRaw);
  if ('sender' in match) return 'sender';
  if ('peer' in match) return 'peer';
  if ('parentPeer' in match) return 'parentPeer';
  if ('guildId' in match && 'roles' in match) return 'guildId+roles';
  if ('guildId' in match) return 'guildId';
  if ('teamId' in match) return 'teamId';
  if ('accountId' in match) return hasWildcard(match.accountId) ? 'accountId:*' : 'accountId';
  if ('channel' in match) return 'channel';
  return 'generic';
}

function validateBindingMatchClient(matchRaw: any, idx: number): string | null {
  if (!isPlainObject(matchRaw)) {
    return `第 ${idx} 条 binding 的 match 必须是对象`;
  }
  for (const key of Object.keys(matchRaw)) {
    if (!ALLOWED_MATCH_KEYS.includes(key)) {
      return `第 ${idx} 条 binding 使用了不支持字段: ${key}`;
    }
  }
  const match = compactMatch(matchRaw);
  if (!('channel' in match)) {
    return `第 ${idx} 条 binding 缺少 match.channel`;
  }
  for (const key of Object.keys(match)) {
    if (key === 'roles' && !('guildId' in match)) {
      return `第 ${idx} 条 binding 的 roles 必须与 guildId 同时使用`;
    }
    if ((key === 'peer' || key === 'parentPeer') && isPlainObject(match[key])) {
      const peer = normalizePeerValue(match[key]);
      if (!peer || !peer.kind) {
        return `第 ${idx} 条 binding 的 ${key}.kind 不能为空`;
      }
    }
  }
  return null;
}

function extractTextValue(v: any): string {
  if (typeof v === 'string') return v;
  return '';
}

function extractRolesText(match: Record<string, any>): string {
  const raw = match.roles;
  if (Array.isArray(raw)) return raw.map(x => String(x).trim()).filter(Boolean).join(', ');
  if (typeof raw === 'string') return raw;
  return '';
}

function extractPeerForm(match: Record<string, any>, key: 'peer' | 'parentPeer'): { kind: string; id: string } {
  const peer = normalizePeerValue(match[key]);
  return peer || { kind: '', id: '' };
}

function formatLastActive(raw?: number): string {
  return raw ? new Date(raw).toLocaleString('zh-CN') : '—';
}

function formatDateTime(raw?: string | number): string {
  if (!raw) return '—';
  const date = typeof raw === 'number' ? new Date(raw) : new Date(raw);
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString('zh-CN');
}

function formatFileSize(size?: number): string {
  if (!size) return '0 B';
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function humanizeChannelName(id: string): string {
  return CHANNEL_DISPLAY_NAMES[id] || id;
}

function humanizeMatchField(key: string): string {
  switch (key) {
    case 'channel':
      return '通道';
    case 'sender':
      return '发送者';
    case 'peer':
      return '会话';
    case 'parentPeer':
      return '上级会话';
    case 'guildId':
      return 'Discord 服务器';
    case 'guildId+roles':
      return 'Discord 服务器 + 角色';
    case 'roles':
      return '角色';
    case 'teamId':
      return 'Slack 工作区';
    case 'accountId':
      return '账号';
    case 'accountId:*':
      return '全部账号';
    default:
      return key;
  }
}

function humanizeBindingPriority(priority: string): string {
  switch (priority) {
    case 'sender':
      return '发送者';
    case 'peer':
      return '会话';
    case 'parentPeer':
      return '上级会话';
    case 'guildId+roles':
      return 'Discord 服务器 + 角色';
    case 'guildId':
      return 'Discord 服务器';
    case 'teamId':
      return 'Slack 工作区';
    case 'accountId':
      return '指定账号';
    case 'accountId:*':
      return '全部账号';
    case 'channel':
      return '通道';
    default:
      return '通用';
  }
}

function describeTriState(raw: InheritToggle, enabledLabel: string, disabledLabel: string): string {
  if (raw === 'enabled') return enabledLabel;
  if (raw === 'disabled') return disabledLabel;
  return '继承默认';
}

function describeSessionVisibility(raw: '' | 'same-agent' | 'all-agents'): string {
  if (raw === 'same-agent') return '仅当前 Agent';
  if (raw === 'all-agents') return '所有 Agent';
  return '继承默认';
}

function describeSandboxMode(raw: any): string {
  const draft = extractSandboxDraft(raw);
  if (!draft.mode && !draft.scope && !draft.workspaceAccess && !draft.workspaceRoot && !draft.dockerNetwork && draft.dockerReadOnlyRoot === 'inherit' && !draft.dockerSetupCommand && !draft.dockerBinds) {
    return '继承默认';
  }
  if (draft.mode === 'non-main') return '仅非主会话沙箱';
  if (draft.mode === 'off') return '高权限';
  if (draft.mode === 'all' && draft.workspaceAccess === 'ro') return '只读沙箱';
  if (draft.mode === 'all' && draft.workspaceAccess === 'rw') return '工作区可写';
  if (draft.mode === 'all') return '全部会话沙箱';
  return '自定义 JSON';
}

function describeSandboxModeValue(raw: string): string {
  switch (raw) {
    case 'off':
      return '关闭（off，宿主机运行）';
    case 'non-main':
      return '仅非主会话（non-main）';
    case 'all':
      return '全部会话（all）';
    default:
      return '继承默认';
  }
}

function describeSandboxScopeValue(raw: string): string {
  switch (raw) {
    case 'session':
      return '每会话一个容器（session）';
    case 'agent':
      return '每 Agent 一个容器（agent）';
    case 'shared':
      return '全部共享一个容器（shared）';
    default:
      return '继承默认';
  }
}

function describeWorkspaceAccessValue(raw: string): string {
  switch (raw) {
    case 'none':
      return '仅沙箱工作区（none）';
    case 'ro':
      return '只读挂载 Agent workspace（ro）';
    case 'rw':
      return '可写挂载 Agent workspace（rw）';
    default:
      return '继承默认';
  }
}

function describeToggleValue(raw: InheritToggle, enabledLabel: string, disabledLabel: string): string {
  if (raw === 'enabled') return enabledLabel;
  if (raw === 'disabled') return disabledLabel;
  return '继承默认';
}

function normalizeLegacySandboxDraft(raw: any): Record<string, any> {
  if (!isPlainObject(raw)) return {};
  const next = deepClone(raw);
  const mode = String(next.mode || '').trim();
  if (mode === 'read-only') {
    next.mode = 'all';
    if (next.workspaceAccess === undefined) next.workspaceAccess = 'ro';
  } else if (mode === 'workspace-write') {
    next.mode = 'all';
    if (next.workspaceAccess === undefined) next.workspaceAccess = 'rw';
  } else if (mode === 'danger-full-access') {
    next.mode = 'off';
  }
  return next;
}

function extractSandboxDraft(raw: any) {
  const sandbox = normalizeLegacySandboxDraft(raw);
  const docker = isPlainObject(sandbox.docker) ? sandbox.docker : {};
  return {
    mode: (() => {
      const mode = String(sandbox.mode || '').trim();
      return mode === 'off' || mode === 'non-main' || mode === 'all' ? mode : '';
    })(),
    scope: (() => {
      const scope = String(sandbox.scope || '').trim();
      return scope === 'session' || scope === 'agent' || scope === 'shared' ? scope : '';
    })(),
    workspaceAccess: (() => {
      const access = String(sandbox.workspaceAccess || '').trim();
      return access === 'none' || access === 'ro' || access === 'rw' ? access : '';
    })(),
    workspaceRoot: String(sandbox.workspaceRoot || '').trim(),
    dockerNetwork: docker.network === undefined ? '' : String(docker.network).trim(),
    dockerReadOnlyRoot: docker.readOnlyRoot === true ? 'enabled' : docker.readOnlyRoot === false ? 'disabled' : 'inherit',
    dockerSetupCommand: String(docker.setupCommand || '').trim(),
    dockerBinds: stringifyLineList(docker.binds),
  } as const;
}

function buildStructuredSandboxPreview(form: Pick<AgentFormState, 'sandboxMode' | 'sandboxScope' | 'sandboxWorkspaceAccess' | 'sandboxWorkspaceRoot' | 'sandboxDockerNetwork' | 'sandboxDockerReadOnlyRoot' | 'sandboxDockerSetupCommand' | 'sandboxDockerBinds'>): any {
  const sandbox: Record<string, any> = {};
  if (form.sandboxMode) sandbox.mode = form.sandboxMode;
  if (form.sandboxScope) sandbox.scope = form.sandboxScope;
  if (form.sandboxWorkspaceAccess) sandbox.workspaceAccess = form.sandboxWorkspaceAccess;
  if (form.sandboxWorkspaceRoot.trim()) sandbox.workspaceRoot = form.sandboxWorkspaceRoot.trim();

  const docker: Record<string, any> = {};
  if (form.sandboxDockerNetwork.trim()) docker.network = form.sandboxDockerNetwork.trim();
  const readOnlyRoot = triStateToValue(form.sandboxDockerReadOnlyRoot);
  if (readOnlyRoot !== undefined) docker.readOnlyRoot = readOnlyRoot;
  if (form.sandboxDockerSetupCommand.trim()) docker.setupCommand = form.sandboxDockerSetupCommand.trim();
  const binds = parseLineList(form.sandboxDockerBinds);
  if (binds.length > 0) docker.binds = binds;
  if (Object.keys(docker).length > 0) sandbox.docker = docker;
  return cleanupObject(sandbox);
}

function formatCronSchedule(schedule: CronJob['schedule'] | undefined): string {
  if (!schedule) return '未设置';
  if (schedule.kind === 'cron') return `Cron · ${schedule.expr || '—'}`;
  if (schedule.kind === 'every') return `Every · ${Math.round((schedule.everyMs || 0) / 60000)} min`;
  if (schedule.kind === 'at') return `At · ${formatDateTime(schedule.atMs || 0)}`;
  return schedule.kind || '未设置';
}

function buildBindingSummary(matchRaw: any, channelMeta: Record<string, ChannelMeta>): string {
  const match = compactMatch(matchRaw);
  const parts: string[] = [];
  const channel = extractTextValue(match.channel);
  const accountId = extractTextValue(match.accountId);
  const sender = extractTextValue(match.sender);
  const peer = extractPeerForm(match, 'peer');
  const parentPeer = extractPeerForm(match, 'parentPeer');
  const guildId = extractTextValue(match.guildId);
  const roles = extractRolesText(match);
  const teamId = extractTextValue(match.teamId);

  if (channel) parts.push(`通道 ${channel}`);
  if (accountId === '*') parts.push('全部账号');
  else if (accountId) parts.push(`账号 ${accountId}`);
  else if (channel) parts.push(`默认账号${channelMeta[channel]?.defaultAccount ? ` (${channelMeta[channel]?.defaultAccount})` : ''}`);
  if (sender) parts.push(`发送者 ${sender}`);
  if (peer.kind) parts.push(`会话 ${peer.kind}${peer.id ? `:${peer.id}` : ''}`);
  if (parentPeer.kind) parts.push(`上级 ${parentPeer.kind}${parentPeer.id ? `:${parentPeer.id}` : ''}`);
  if (guildId && roles) parts.push(`Discord ${guildId} / ${roles}`);
  else if (guildId) parts.push(`Discord ${guildId}`);
  else if (roles) parts.push(`角色 ${roles}`);
  if (teamId) parts.push(`Slack ${teamId}`);

  return parts.length > 0 ? parts.join(' · ') : '未设置匹配条件';
}

function buildBindingTags(matchRaw: any): string[] {
  const match = compactMatch(matchRaw);
  const tags: string[] = [];
  if (extractTextValue(match.channel)) tags.push('通道');
  if ('accountId' in match) tags.push(extractTextValue(match.accountId) === '*' ? '全部账号' : '账号');
  if (extractTextValue(match.sender)) tags.push('发送者');
  if (extractPeerForm(match, 'peer').kind) tags.push('会话');
  if (extractPeerForm(match, 'parentPeer').kind) tags.push('上级会话');
  if (extractTextValue(match.guildId)) tags.push('Discord');
  if (extractRolesText(match)) tags.push('角色');
  if (extractTextValue(match.teamId)) tags.push('Slack');
  return tags;
}

function parseChannelsMeta(raw: any): Record<string, ChannelMeta> {
  const out: Record<string, ChannelMeta> = {};
  if (!isPlainObject(raw)) return out;

  for (const [channel, cfgRaw] of Object.entries(raw)) {
    const channelID = String(channel || '').trim();
    if (!channelID) continue;
    const cfg = isPlainObject(cfgRaw) ? cfgRaw : {};
    const accountsObj = isPlainObject(cfg.accounts) ? cfg.accounts : {};
    const accounts = Object.keys(accountsObj).map(x => x.trim()).filter(Boolean).sort();
    let defaultAccount = String(cfg.defaultAccount || '').trim();
    if (!defaultAccount) {
      if (accounts.includes('default')) defaultAccount = 'default';
      else if (accounts.length > 0) defaultAccount = accounts[0];
    }
    out[channelID] = { accounts, defaultAccount: defaultAccount || undefined };
  }

  return out;
}

function buildPreviewMetaPayload(previewMeta: Record<PreviewMetaKey, string>, channelMeta: Record<string, ChannelMeta>): Record<string, any> {
  const meta: Record<string, any> = {};
  for (const [key, raw] of Object.entries(previewMeta) as Array<[PreviewMetaKey, string]>) {
    const text = raw.trim();
    if (!text) continue;
    if (key === 'roles') {
      const roles = parseCSV(text);
      if (roles.length > 0) meta[key] = roles;
      continue;
    }
    meta[key] = text;
  }

  const channel = typeof meta.channel === 'string' ? meta.channel.trim() : '';
  if (channel && meta.accountId === undefined) {
    const defaultAccount = String(channelMeta[channel]?.defaultAccount || '').trim();
    if (defaultAccount) meta.accountId = defaultAccount;
  }

  return meta;
}

function stringifyJSON(raw: any): string {
  return raw === undefined ? '' : JSON.stringify(raw, null, 2);
}

function getNestedValue(raw: any, path: string): any {
  return path.split('.').reduce<any>((acc, key) => (isPlainObject(acc) ? acc[key] : undefined), raw);
}

function setNestedValue(target: Record<string, any>, path: string, value: any) {
  const parts = path.split('.');
  let cursor: Record<string, any> = target;
  for (let i = 0; i < parts.length - 1; i++) {
    const key = parts[i];
    if (!isPlainObject(cursor[key])) cursor[key] = {};
    cursor = cursor[key];
  }
  cursor[parts[parts.length - 1]] = value;
}

function deleteNestedValue(target: Record<string, any>, path: string) {
  const parts = path.split('.');
  const stack: Array<{ parent: Record<string, any>; key: string }> = [];
  let cursor: Record<string, any> = target;
  for (let i = 0; i < parts.length - 1; i++) {
    const key = parts[i];
    if (!isPlainObject(cursor[key])) return;
    stack.push({ parent: cursor, key });
    cursor = cursor[key];
  }
  delete cursor[parts[parts.length - 1]];
  for (let i = stack.length - 1; i >= 0; i--) {
    const { parent, key } = stack[i];
    if (isPlainObject(parent[key]) && Object.keys(parent[key]).length === 0) {
      delete parent[key];
    }
  }
}

function cleanupObject(raw: any): any {
  if (!isPlainObject(raw)) return raw;
  const out: Record<string, any> = {};
  for (const [key, value] of Object.entries(raw)) {
    if (value === undefined) continue;
    if (isPlainObject(value)) {
      const nested = cleanupObject(value);
      if (nested && Object.keys(nested).length > 0) out[key] = nested;
      continue;
    }
    out[key] = value;
  }
  return Object.keys(out).length > 0 ? out : undefined;
}

function triStateFromValue(raw: any): InheritToggle {
  if (raw === true) return 'enabled';
  if (raw === false) return 'disabled';
  return 'inherit';
}

function triStateToValue(raw: InheritToggle): boolean | undefined {
  if (raw === 'enabled') return true;
  if (raw === 'disabled') return false;
  return undefined;
}

function parseStringList(raw: any): string[] {
  if (Array.isArray(raw)) return raw.map(item => String(item).trim()).filter(Boolean);
  if (typeof raw === 'string') return parseCSV(raw);
  return [];
}

function parseNumberInput(raw: string, label: string, constraint?: NumberInputConstraint): number | undefined {
  const text = raw.trim();
  if (!text) return undefined;
  const value = Number(text);
  if (!Number.isFinite(value)) throw new Error(`${label} 必须是数字`);
  if (constraint?.integer && !Number.isInteger(value)) throw new Error(`${label} 必须是整数`);
  if (constraint?.min !== undefined && value < constraint.min) throw new Error(`${label} 不能小于 ${constraint.min}`);
  if (constraint?.max !== undefined && value > constraint.max) throw new Error(`${label} 不能大于 ${constraint.max}`);
  return value;
}

function extractModelDraft(raw: any): { primary: string; fallbacks: string } {
  if (typeof raw === 'string') {
    return { primary: raw, fallbacks: '' };
  }
  if (!isPlainObject(raw)) {
    return { primary: '', fallbacks: '' };
  }
  return {
    primary: String(raw.primary || '').trim(),
    fallbacks: parseStringList(raw.fallbacks).join(', '),
  };
}

function extractCompactionDraft(raw: any): { mode: '' | 'default' | 'safeguard'; maxHistoryShare: string } {
  if (!isPlainObject(raw)) {
    return { mode: '', maxHistoryShare: '' };
  }
  const mode = String(raw.mode || '').trim();
  return {
    mode: mode === 'default' || mode === 'safeguard' ? mode : '',
    maxHistoryShare: raw.maxHistoryShare === undefined ? '' : String(raw.maxHistoryShare),
  };
}

function extractIdentityDraft(raw: any): { name: string; theme: string; emoji: string; avatar: string } {
  if (!isPlainObject(raw)) {
    return { name: '', theme: '', emoji: '', avatar: '' };
  }
  return {
    name: String(raw.name || '').trim(),
    theme: String(raw.theme ?? raw.description ?? raw.vibe ?? raw.tone ?? '').trim(),
    emoji: String(raw.emoji || '').trim(),
    avatar: String(raw.avatar || '').trim(),
  };
}

function detectSandboxStarter(raw: string): string {
  const text = raw.trim();
  if (!text) return 'inherit';
  try {
    const parsed = JSON.parse(text);
    if (isPlainObject(parsed)) {
      const mode = String(parsed.mode || '').trim();
      const workspaceAccess = String(parsed.workspaceAccess || '').trim();
      if (mode === 'non-main') return 'non-main';
      if (mode === 'read-only') return 'read-only';
      if (mode === 'workspace-write') return 'workspace-write';
      if (mode === 'danger-full-access') return 'danger-full-access';
      if (mode === 'all' && workspaceAccess === 'ro') return 'read-only';
      if (mode === 'all' && workspaceAccess === 'rw') return 'workspace-write';
      if (mode === 'off') return 'danger-full-access';
    }
  } catch {
    // fall through to exact-text detection for freeform JSON
  }
  for (const starter of SANDBOX_STARTERS) {
    if (!starter.text) continue;
    if (starter.text.trim() === text) return starter.key;
  }
  return 'custom';
}

function getUnsupportedSandboxStructuredMessages(raw: string): string[] {
  const text = raw.trim();
  if (!text) return [];
  try {
    const parsed = JSON.parse(text);
    if (!isPlainObject(parsed)) return ['当前 sandbox 不是对象，结构化表单无法安全编辑。'];
    const messages: string[] = [];
    const mode = String(parsed.mode || '').trim();
    if (mode && !['off', 'non-main', 'all', 'read-only', 'workspace-write', 'danger-full-access'].includes(mode)) {
      messages.push(`sandbox.mode=${mode}`);
    }
    const scope = String(parsed.scope || '').trim();
    if (scope && !['session', 'agent', 'shared'].includes(scope)) {
      messages.push(`sandbox.scope=${scope}`);
    }
    const workspaceAccess = String(parsed.workspaceAccess || '').trim();
    if (workspaceAccess && !['none', 'ro', 'rw'].includes(workspaceAccess)) {
      messages.push(`sandbox.workspaceAccess=${workspaceAccess}`);
    }
    return messages;
  } catch {
    return [];
  }
}

function extractProviderModelOptions(raw: any): string[] {
  if (!isPlainObject(raw)) return [];
  const options = new Set<string>();
  for (const [providerID, providerRaw] of Object.entries(raw)) {
    const provider = String(providerID || '').trim();
    if (!provider) continue;
    const providerCfg = isPlainObject(providerRaw) ? providerRaw : {};
    const models = Array.isArray(providerCfg.models) ? providerCfg.models : [];
    for (const modelRaw of models) {
      let modelID = '';
      if (typeof modelRaw === 'string') modelID = modelRaw.trim();
      else if (isPlainObject(modelRaw)) modelID = String(modelRaw.id || modelRaw.name || '').trim();
      if (modelID) options.add(`${provider}/${modelID}`);
    }
  }
  return Array.from(options).sort((a, b) => a.localeCompare(b));
}

function isImplicitAgent(agent?: AgentItem): boolean {
  if (!agent) return false;
  if (typeof agent.implicit === 'boolean') return agent.implicit;
  if (agent.id !== 'main') return false;
  return !(
    agent.name ||
    agent.workspace ||
    agent.agentDir ||
    agent.model ||
    agent.contextTokens !== undefined ||
    agent.compaction ||
    agent.tools ||
    agent.sandbox ||
    agent.groupChat ||
    agent.identity ||
    agent.subagents ||
    agent.params ||
    agent.runtime
  );
}

function isImplicitMainAgent(agent?: AgentItem): boolean {
  return isImplicitAgent(agent) && agent?.id === 'main';
}

function createAgentFormState(agent?: AgentItem): AgentFormState {
  const modelDraft = extractModelDraft(agent?.model);
  const params = isPlainObject(agent?.params) ? agent?.params : {};
  const identityDraft = extractIdentityDraft(agent?.identity);
  const tools = isPlainObject(agent?.tools) ? agent?.tools : {};
  const subagents = isPlainObject(agent?.subagents) ? agent?.subagents : {};
  const groupChat = isPlainObject(agent?.groupChat) ? agent?.groupChat : {};
  const sandboxDraft = extractSandboxDraft(agent?.sandbox);
  const compactionDraft = extractCompactionDraft(agent?.compaction);

  return {
    ...DEFAULT_AGENT_FORM,
    id: agent?.id || '',
    name: agent?.name || '',
    workspace: agent?.workspace || '',
    agentDir: agent?.agentDir || '',
    isDefault: !!agent?.default,
    modelPrimary: modelDraft.primary,
    modelFallbacks: modelDraft.fallbacks,
    paramTemperature: params.temperature === undefined ? '' : String(params.temperature),
    paramTopP: params.topP === undefined ? '' : String(params.topP),
    paramMaxTokens: params.maxTokens === undefined ? '' : String(params.maxTokens),
    contextTokens: agent?.contextTokens === undefined ? '' : String(agent.contextTokens),
    compactionMode: compactionDraft.mode,
    compactionMaxHistoryShare: compactionDraft.maxHistoryShare,
    identityName: identityDraft.name,
    identityTheme: identityDraft.theme,
    identityEmoji: identityDraft.emoji,
    identityAvatar: identityDraft.avatar,
    sandboxMode: sandboxDraft.mode,
    sandboxScope: sandboxDraft.scope,
    sandboxWorkspaceAccess: sandboxDraft.workspaceAccess,
    sandboxWorkspaceRoot: sandboxDraft.workspaceRoot,
    sandboxDockerNetwork: sandboxDraft.dockerNetwork,
    sandboxDockerReadOnlyRoot: sandboxDraft.dockerReadOnlyRoot,
    sandboxDockerSetupCommand: sandboxDraft.dockerSetupCommand,
    sandboxDockerBinds: sandboxDraft.dockerBinds,
    groupChatMode: triStateFromValue(getNestedValue(groupChat, 'enabled')),
    toolProfile: (() => {
      const raw = getNestedValue(tools, 'profile');
      return raw === 'minimal' || raw === 'coding' || raw === 'messaging' || raw === 'full' ? raw : '';
    })(),
    toolAllow: parseStringList(getNestedValue(tools, 'allow')).join(', '),
    toolDeny: parseStringList(getNestedValue(tools, 'deny')).join(', '),
    agentToAgentMode: triStateFromValue(getNestedValue(tools, 'agentToAgent.enabled')),
    agentToAgentAllow: parseStringList(getNestedValue(tools, 'agentToAgent.allow')).join(', '),
    sessionVisibility: (() => {
      const raw = getNestedValue(tools, 'sessions.visibility');
      return raw === 'same-agent' || raw === 'all-agents' ? raw : '';
    })(),
    subagentAllowAgents: parseStringList(getNestedValue(subagents, 'allowAgents')).join(', '),
    modelText: stringifyJSON(agent?.model),
    toolsText: stringifyJSON(agent?.tools),
    sandboxText: stringifyJSON(agent?.sandbox),
    groupChatText: stringifyJSON(agent?.groupChat),
    identityText: stringifyJSON(agent?.identity),
    subagentsText: stringifyJSON(agent?.subagents),
    paramsText: stringifyJSON(agent?.params),
    runtimeText: stringifyJSON(agent?.runtime),
  };
}

function summarizeAdvancedBlocks(agent?: AgentItem): string[] {
  if (!agent) return [];
  return [
    agent.model ? 'model' : '',
    agent.tools ? 'tools' : '',
    agent.sandbox ? 'sandbox' : '',
    agent.groupChat ? 'groupChat' : '',
    agent.identity ? 'identity' : '',
    agent.subagents ? 'subagents' : '',
    agent.params ? 'params' : '',
    agent.runtime ? 'runtime' : '',
  ].filter(Boolean);
}

function buildPreviewExplanation(result: PreviewResult | null, bindings: BindingDraft[], defaultAgent: string): {
  headline: string;
  detail: string;
  ruleLabel: string;
} | null {
  if (!result) return null;
  const agentID = result.agent || defaultAgent || '未命名 Agent';
  if (!result.matchedBy || result.matchedBy === 'default') {
    return {
      headline: `消息会回落给默认 Agent「${agentID}」`,
      detail: '本次输入没有命中任何显式路由规则，所以系统使用默认 Agent 兜底。',
      ruleLabel: '默认回落',
    };
  }

  const matched = result.matchedBy.match(/^bindings\[(\d+)\]\.match\.(.+)$/);
  if (!matched) {
    return {
      headline: `消息会交给 Agent「${agentID}」`,
      detail: `命中条件：${result.matchedBy}`,
      ruleLabel: result.matchedBy,
    };
  }

  const index = Number(matched[1]);
  const field = matched[2];
  const binding = bindings[index];
  const ruleName = binding?.name?.trim() || `规则 ${index + 1}`;
  return {
    headline: `消息会交给 Agent「${agentID}」`,
    detail: `命中了「${ruleName}」里的「${humanizeMatchField(field)}」条件。`,
    ruleLabel: ruleName,
  };
}

export default function Agents() {
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState('');

  const [workbenchView, setWorkbenchView] = useState<AgentsWorkbenchView>('directory');
  const [selectedAgentId, setSelectedAgentId] = useState('');
  const [detailTab, setDetailTab] = useState<AgentDetailTab>('overview');
  const [expandedBindingIndex, setExpandedBindingIndex] = useState<number | null>(0);

  const [defaultAgent, setDefaultAgent] = useState('main');
  const [defaultConfigured, setDefaultConfigured] = useState(false);
  const [agents, setAgents] = useState<AgentItem[]>([]);
  const [bindings, setBindings] = useState<BindingDraft[]>([]);
  const [channelMeta, setChannelMeta] = useState<Record<string, ChannelMeta>>({});
  const [channelConfigs, setChannelConfigs] = useState<Record<string, any>>({});
  const [modelOptions, setModelOptions] = useState<string[]>([]);
  const [defaultModelHint, setDefaultModelHint] = useState('');
  const [agentDefaults, setAgentDefaults] = useState<AgentDefaultsState>(EMPTY_AGENT_DEFAULTS);
  const [skills, setSkills] = useState<SkillEntry[]>([]);
  const [cronJobs, setCronJobs] = useState<CronJob[]>([]);
  const [coreFilesByAgent, setCoreFilesByAgent] = useState<Record<string, AgentCoreFileEntry[]>>({});
  const [coreFilesLoading, setCoreFilesLoading] = useState(false);
  const [selectedCoreFileName, setSelectedCoreFileName] = useState('AGENTS.md');
  const [coreFileDraft, setCoreFileDraft] = useState('');
  const [coreFileSaving, setCoreFileSaving] = useState(false);
  const coreFileDraftResetRef = useRef(false);
  const [sessionsByAgent, setSessionsByAgent] = useState<Record<string, SessionInfo[]>>({});
  const [sessionsLoading, setSessionsLoading] = useState(false);

  const [editingId, setEditingId] = useState<string | null>(null);
  const [materializingImplicitAgent, setMaterializingImplicitAgent] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [formSection, setFormSection] = useState<AgentFormSection>('basic');
  const [form, setForm] = useState<AgentFormState>(DEFAULT_AGENT_FORM);
  const [sandboxClearIntent, setSandboxClearIntent] = useState(false);
  const [saveAttempted, setSaveAttempted] = useState(false);
  const [structuredTouched, setStructuredTouched] = useState<AgentStructuredTouchedState>(DEFAULT_AGENT_STRUCTURED_TOUCHED);

  const [previewMeta, setPreviewMeta] = useState<Record<PreviewMetaKey, string>>(DEFAULT_PREVIEW_META);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewResult, setPreviewResult] = useState<PreviewResult | null>(null);

  const agentOptions = useMemo(() => {
    return agents.map(a => a.id).filter(Boolean);
  }, [agents]);

  const explicitAgents = useMemo(() => {
    return agents.filter(agent => !isImplicitAgent(agent));
  }, [agents]);

  const channelOptions = useMemo(() => {
    return Object.keys(channelMeta).sort();
  }, [channelMeta]);

  const agentDirConflict = useMemo(() => {
    const value = form.agentDir.trim();
    if (!value) return null;
    return agents.find(agent => agent.id !== editingId && String(agent.agentDir || '').trim() === value) || null;
  }, [agents, editingId, form.agentDir]);

  const workspaceConflict = useMemo(() => {
    const value = form.workspace.trim();
    if (!value) return null;
    return agents.find(agent => agent.id !== editingId && String(agent.workspace || '').trim() === value) || null;
  }, [agents, editingId, form.workspace]);

  const sandboxStarter = useMemo(() => {
    if (structuredTouched.sandbox) {
      return detectSandboxStarter(stringifyJSON(buildStructuredSandboxPreview(form)));
    }
    return detectSandboxStarter(form.sandboxText);
  }, [form, structuredTouched.sandbox]);
  const sandboxStructuredIssues = useMemo(() => getUnsupportedSandboxStructuredMessages(form.sandboxText), [form.sandboxText]);
  const sandboxStructuredLocked = sandboxStructuredIssues.length > 0;
  const isBlankAgentID = !form.id.trim();
  const agentIDError = useMemo(() => {
    const value = form.id.trim();
    if (!value) return 'Agent ID 不能为空';
    if (!/^[A-Za-z0-9_-]+$/.test(value)) return 'Agent ID 仅支持字母、数字、下划线和中划线';
    const duplicated = agents.find(agent => {
      if (agent.id !== value || agent.id === editingId) return false;
      if (isImplicitAgent(agent)) return false;
      return true;
    });
    if (duplicated) return `Agent ID 已存在：${value}`;
    return null;
  }, [agents, editingId, form.id]);
  const quickModelOptions = useMemo(() => {
    const merged = [defaultModelHint, ...modelOptions].map(item => item.trim()).filter(Boolean);
    return Array.from(new Set(merged)).slice(0, 6);
  }, [defaultModelHint, modelOptions]);
  const implicitMainOnly = useMemo(() => {
    return !editingId && agents.length === 1 && defaultAgent === 'main' && isImplicitMainAgent(agents[0]);
  }, [agents, defaultAgent, editingId]);
  const firstExplicitAgentWillBecomeDefault = useMemo(() => {
    const id = form.id.trim();
    if (!id || explicitAgents.length > 0) return false;
    if (!editingId || materializingImplicitAgent) {
      if (!defaultConfigured) return true;
      return id === defaultAgent;
    }
    return false;
  }, [defaultAgent, defaultConfigured, editingId, explicitAgents.length, form.id, materializingImplicitAgent]);
  const selectedAgent = useMemo(() => {
    return agents.find(agent => agent.id === selectedAgentId) || agents[0] || null;
  }, [agents, selectedAgentId]);
  const selectedAgentBindings = useMemo(() => {
    if (!selectedAgent) return [];
    return bindings
      .map((binding, index) => ({ binding, index }))
      .filter(item => item.binding.agent === selectedAgent.id);
  }, [bindings, selectedAgent]);
  const selectedAgentCoreFiles = useMemo(() => {
    if (!selectedAgent) return [];
    return coreFilesByAgent[selectedAgent.id] || [];
  }, [coreFilesByAgent, selectedAgent]);
  const selectedCoreFile = useMemo(() => {
    if (selectedAgentCoreFiles.length === 0) return null;
    return selectedAgentCoreFiles.find(file => file.name === selectedCoreFileName) || selectedAgentCoreFiles[0];
  }, [selectedAgentCoreFiles, selectedCoreFileName]);
  const coreFileDirty = useMemo(() => {
    if (!selectedCoreFile) return coreFileDraft.trim().length > 0;
    return coreFileDraft !== (selectedCoreFile.content || '');
  }, [coreFileDraft, selectedCoreFile]);
  const selectedAgentCronJobs = useMemo(() => {
    if (!selectedAgent) return [];
    return cronJobs.filter(job => String(job.sessionTarget || '').trim() === selectedAgent.id);
  }, [cronJobs, selectedAgent]);
  const selectedAgentSessions = useMemo(() => {
    if (!selectedAgent) return [];
    return sessionsByAgent[selectedAgent.id] || [];
  }, [sessionsByAgent, selectedAgent]);
  const routingStats = useMemo(() => {
    const activeAgents = new Set(bindings.map(item => item.agent).filter(Boolean));
    const activeChannels = new Set(
      bindings
        .map(item => extractTextValue(compactMatch(item.match).channel))
        .filter(Boolean),
    );
    return {
      total: bindings.length,
      enabled: bindings.filter(item => item.enabled !== false).length,
      agents: activeAgents.size,
      channels: activeChannels.size,
    };
  }, [bindings]);
  const enabledSkills = useMemo(() => skills.filter(skill => skill.enabled), [skills]);
  const selectedAgentChannelSnapshot = useMemo(() => {
    const routeCountByChannel = new Map<string, number>();
    selectedAgentBindings.forEach(({ binding }) => {
      const channel = extractTextValue(compactMatch(binding.match).channel);
      if (!channel) return;
      routeCountByChannel.set(channel, (routeCountByChannel.get(channel) || 0) + 1);
    });
    return Object.entries(channelConfigs)
      .map(([id, raw]) => {
        const accounts = channelMeta[id]?.accounts || [];
        return {
          id,
          label: humanizeChannelName(id),
          enabled: isPlainObject(raw) ? raw.enabled !== false : true,
          configuredAccounts: accounts.length,
          defaultAccount: channelMeta[id]?.defaultAccount || '',
          routeCount: routeCountByChannel.get(id) || 0,
          raw,
        };
      })
      .sort((a, b) => {
        if (a.routeCount !== b.routeCount) return b.routeCount - a.routeCount;
        return a.id.localeCompare(b.id);
      });
  }, [channelConfigs, channelMeta, selectedAgentBindings]);
  const previewExplanation = useMemo(() => buildPreviewExplanation(previewResult, bindings, defaultAgent), [previewResult, bindings, defaultAgent]);
  const effectiveIsDefault = firstExplicitAgentWillBecomeDefault ? true : form.isDefault;
  const saveBlockedReason = useMemo(() => {
    if (agentIDError) return agentIDError;
    if (workspaceConflict) return `workspace 已被 Agent “${workspaceConflict.id}” 使用`;
    if (agentDirConflict) return `agentDir 已被 Agent “${agentDirConflict.id}” 使用`;
    return '';
  }, [agentDirConflict, agentIDError, workspaceConflict]);
  const showAgentIDError = saveAttempted || !isBlankAgentID;
  const footerValidationMessage = useMemo(() => {
    if (!saveBlockedReason) return '保存会继续沿用现有后端接口与 payload 格式；bindings 与路由预览不会受到影响。';
    if (isBlankAgentID && !saveAttempted) return '完成 Basic 中的 Agent ID 后即可保存。';
    return saveBlockedReason;
  }, [isBlankAgentID, saveAttempted, saveBlockedReason]);
  const footerValidationIsError = !!saveBlockedReason && !(isBlankAgentID && !saveAttempted);

  const touchStructured = (key: keyof AgentStructuredTouchedState) => {
    setStructuredTouched(prev => (prev[key] ? prev : { ...prev, [key]: true }));
  };

  const updateForm = (patch: Partial<AgentFormState>, touchKey?: keyof AgentStructuredTouchedState) => {
    setForm(prev => ({ ...prev, ...patch }));
    if (touchKey) touchStructured(touchKey);
  };

  const updateSandboxForm = (patch: Partial<AgentFormState>) => {
    updateForm(patch, 'sandbox');
    setSandboxClearIntent(false);
  };

  const applyToolPolicyPreset = (preset: AgentFormState['toolProfile']) => {
    updateForm({ toolProfile: preset }, 'tools');
  };

  const applyHeadlessToolPreset = () => {
    updateForm({
      toolProfile: 'minimal',
      toolAllow: 'group:web, group:fs',
      toolDeny: 'group:runtime, group:ui, group:nodes, group:automation',
    }, 'tools');
  };

  const resetToolPolicyOverrides = () => {
    updateForm({ toolProfile: '', toolAllow: '', toolDeny: '' }, 'tools');
  };

  const closeForm = () => {
    setShowForm(false);
    setMaterializingImplicitAgent(false);
    setSandboxClearIntent(false);
    setSaveAttempted(false);
    setStructuredTouched(DEFAULT_AGENT_STRUCTURED_TOUCHED);
  };

  const loadData = async () => {
    setLoading(true);
    try {
      const [agentsRes, channelsRes, modelsRes, skillsRes, cronRes] = await Promise.all([
        api.getAgentsConfig(),
        api.getChannels(),
        api.getModels(),
        api.getSkills(),
        api.getCronJobs(),
      ]);
      let nextDefaultModelHint = '';

      if (agentsRes?.ok) {
        const data = agentsRes.agents || {};
        const list: AgentItem[] = data.list || [];
        const incomingBindings = (data.bindings || []) as any[];
        const fallback = data.default || 'main';
        const defaults = isPlainObject(data.defaults) ? data.defaults : {};
        const defaultCompaction = isPlainObject(defaults.compaction) ? defaults.compaction : {};
        setDefaultAgent(fallback);
        setDefaultConfigured(data.defaultConfigured === true);
        setAgents(list);
        setBindings(incomingBindings.map((b: any) => toBindingDraft(b, fallback)));
        nextDefaultModelHint = extractModelDraft(defaults.model).primary;
        setDefaultModelHint(nextDefaultModelHint);
        const defaultContextTokens = Number(defaults.contextTokens);
        const defaultMaxHistoryShare = Number(defaultCompaction.maxHistoryShare);
        setAgentDefaults({
          contextTokens: defaults.contextTokens === undefined || !Number.isFinite(defaultContextTokens) ? undefined : defaultContextTokens,
          compactionMode: (() => {
            const mode = String(defaultCompaction.mode || '').trim();
            return mode === 'default' || mode === 'safeguard' ? mode : '';
          })(),
          compactionMaxHistoryShare: defaultCompaction.maxHistoryShare === undefined || !Number.isFinite(defaultMaxHistoryShare)
            ? undefined
            : defaultMaxHistoryShare,
        });
      } else {
        setDefaultAgent('main');
        setDefaultConfigured(false);
        setAgents([]);
        setBindings([]);
        setDefaultModelHint('');
        setAgentDefaults(EMPTY_AGENT_DEFAULTS);
      }

      if (channelsRes?.ok) {
        setChannelConfigs(channelsRes.channels || {});
        setChannelMeta(parseChannelsMeta(channelsRes.channels || {}));
      } else {
        setChannelConfigs({});
        setChannelMeta({});
      }

      const modelsPayload = (modelsRes || {}) as AgentModelResponse;
      if (modelsPayload.ok) {
        const providers = isPlainObject(modelsPayload.providers)
          ? modelsPayload.providers
          : isPlainObject(modelsPayload.models?.providers)
            ? modelsPayload.models?.providers
            : {};
        const hint = nextDefaultModelHint || extractModelDraft(modelsPayload.defaults?.model).primary;
        setModelOptions(extractProviderModelOptions(providers));
        if (hint) setDefaultModelHint(hint);
      } else {
        setModelOptions([]);
      }

      if (skillsRes?.ok) {
        setSkills(skillsRes.skills || []);
      } else {
        setSkills([]);
      }

      if (cronRes?.ok) {
        setCronJobs(cronRes.jobs || []);
      } else {
        setCronJobs([]);
      }
    } catch {
      setDefaultAgent('main');
      setDefaultConfigured(false);
      setAgents([]);
      setBindings([]);
      setChannelConfigs({});
      setChannelMeta({});
      setModelOptions([]);
      setDefaultModelHint('');
      setSkills([]);
      setCronJobs([]);
    } finally {
      setLoading(false);
    }
  };

  const loadAgentCoreFiles = async (agentId: string, force = false) => {
    if (!agentId) return;
    if (!force && coreFilesByAgent[agentId]) return;
    setCoreFilesLoading(true);
    try {
      const response = await api.getAgentCoreFiles(agentId);
      if (response?.ok) {
        setCoreFilesByAgent(prev => ({ ...prev, [agentId]: response.files || [] }));
      } else if (response?.error) {
        setMsg(`加载核心文件失败: ${response.error}`);
        setTimeout(() => setMsg(''), 4000);
      }
    } catch (err) {
      setMsg(`加载核心文件失败: ${String(err)}`);
      setTimeout(() => setMsg(''), 4000);
    } finally {
      setCoreFilesLoading(false);
    }
  };

  const loadAgentSessions = async (agentId: string, force = false) => {
    if (!agentId) return;
    if (!force && sessionsByAgent[agentId]) return;
    setSessionsLoading(true);
    try {
      const response = await api.getSessions(agentId);
      if (response?.ok) {
        setSessionsByAgent(prev => ({ ...prev, [agentId]: response.sessions || [] }));
      }
    } catch {
      // Ignore session snapshot fetch errors on the Agents page.
    } finally {
      setSessionsLoading(false);
    }
  };

  const confirmDiscardCoreFileDraft = (actionLabel: string) => {
    if (!coreFileDirty) return true;
    const fileLabel = selectedCoreFile?.name || '当前核心文件';
    const confirmed = window.confirm(`当前 ${fileLabel} 有未保存修改，确认继续${actionLabel}并放弃这些修改吗？`);
    if (confirmed) {
      coreFileDraftResetRef.current = true;
    }
    return confirmed;
  };

  const saveAgentCoreFile = async () => {
    if (!selectedAgent || !selectedCoreFile) return;
    setCoreFileSaving(true);
    try {
      const response = await api.saveAgentCoreFile(selectedAgent.id, selectedCoreFile.name, coreFileDraft);
      if (response?.ok === false) {
        setMsg('保存核心文件失败: ' + (response.error || 'unknown error'));
        return;
      }
      await loadAgentCoreFiles(selectedAgent.id, true);
      setMsg(`已保存 ${selectedCoreFile.name}`);
    } catch (err) {
      setMsg('保存核心文件失败: ' + String(err));
    } finally {
      setCoreFileSaving(false);
      setTimeout(() => setMsg(''), 4000);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  useEffect(() => {
    const fallback = agents.find(agent => agent.id === defaultAgent)?.id || agents[0]?.id || '';
    const next = agents.find(agent => agent.id === selectedAgentId)?.id || fallback;
    if (next !== selectedAgentId) setSelectedAgentId(next);
  }, [agents, defaultAgent, selectedAgentId]);

  useEffect(() => {
    if (bindings.length === 0) {
      if (expandedBindingIndex !== null) setExpandedBindingIndex(null);
      return;
    }
    if (expandedBindingIndex === null || expandedBindingIndex >= bindings.length) {
      setExpandedBindingIndex(0);
    }
  }, [bindings.length, expandedBindingIndex]);

  useEffect(() => {
    if (!showForm) return;
    if (msg.includes('不能为空') || msg.includes('已被占用') || msg.includes('仅支持') || msg.includes('已存在')) {
      setMsg('');
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [form]);

  useEffect(() => {
    if (!selectedAgent) return;
    if (detailTab === 'files') {
      loadAgentCoreFiles(selectedAgent.id);
    }
    if (detailTab === 'capabilities' || detailTab === 'context') {
      loadAgentSessions(selectedAgent.id);
    }
  }, [detailTab, selectedAgent?.id]);

  useEffect(() => {
    const allowReset = coreFileDraftResetRef.current || !coreFileDirty;
    if (selectedAgentCoreFiles.length === 0) {
      if (!allowReset) return;
      setSelectedCoreFileName('AGENTS.md');
      setCoreFileDraft('');
      coreFileDraftResetRef.current = false;
      return;
    }
    const nextSelected = selectedAgentCoreFiles.find(file => file.name === selectedCoreFileName) || selectedAgentCoreFiles[0];
    const nextContent = nextSelected.content || '';
    if (!allowReset && (nextSelected.name !== selectedCoreFileName || nextContent !== coreFileDraft)) return;
    if (nextSelected.name !== selectedCoreFileName) {
      setSelectedCoreFileName(nextSelected.name);
    }
    if (coreFileDraft !== nextContent) {
      setCoreFileDraft(nextContent);
    }
    coreFileDraftResetRef.current = false;
  }, [coreFileDirty, coreFileDraft, selectedAgentCoreFiles, selectedCoreFileName]);

  const openCreate = (section: AgentFormSection = 'basic') => {
    setMsg('');
    setMaterializingImplicitAgent(false);
    setSandboxClearIntent(false);
    setSaveAttempted(false);
    setStructuredTouched(DEFAULT_AGENT_STRUCTURED_TOUCHED);
    setEditingId(null);
    setFormSection(section);
    setForm(createAgentFormState());
    setShowForm(true);
  };

  const openEdit = (agent: AgentItem, section: AgentFormSection = 'basic') => {
    setMsg('');
    const implicitAgent = isImplicitAgent(agent);
    setMaterializingImplicitAgent(implicitAgent);
    setSandboxClearIntent(false);
    setSaveAttempted(false);
    setStructuredTouched(DEFAULT_AGENT_STRUCTURED_TOUCHED);
    setEditingId(agent.id);
    setFormSection(section);
    setForm(createAgentFormState(agent));
    setShowForm(true);
  };

  const parseJSONText = (raw: string, fieldName: string) => {
    const text = raw.trim();
    if (!text) return undefined;
    try {
      return JSON.parse(text);
    } catch (err) {
      throw new Error(`${fieldName} JSON 格式错误: ${String(err)}`);
    }
  };

  const applySandboxStarter = (starterKey: string) => {
    const starter = SANDBOX_STARTERS.find(item => item.key === starterKey);
    if (!starter) return;
    const sandboxObj = parseJSONText(starter.text, 'sandbox');
    const draft = extractSandboxDraft(sandboxObj);
    setForm(prev => ({
      ...prev,
      sandboxMode: draft.mode,
      sandboxScope: draft.scope,
      sandboxWorkspaceAccess: draft.workspaceAccess,
      sandboxWorkspaceRoot: draft.workspaceRoot,
      sandboxDockerNetwork: draft.dockerNetwork,
      sandboxDockerReadOnlyRoot: draft.dockerReadOnlyRoot,
      sandboxDockerSetupCommand: draft.dockerSetupCommand,
      sandboxDockerBinds: draft.dockerBinds,
    }));
    touchStructured('sandbox');
    setSandboxClearIntent(starterKey === 'inherit');
  };

  const syncStructuredFieldsFromAdvanced = () => {
    try {
      const modelObj = parseJSONText(form.modelText, 'model');
      const toolsObj = parseJSONText(form.toolsText, 'tools');
      const sandboxObj = parseJSONText(form.sandboxText, 'sandbox');
      const groupChatObj = parseJSONText(form.groupChatText, 'groupChat');
      const identityObj = parseJSONText(form.identityText, 'identity');
      const subagentsObj = parseJSONText(form.subagentsText, 'subagents');
      const paramsObj = parseJSONText(form.paramsText, 'params');
      const modelDraft = extractModelDraft(modelObj);
      const sandboxDraft = extractSandboxDraft(sandboxObj);
      const identityDraft = extractIdentityDraft(identityObj);
      setForm(prev => ({
        ...prev,
        modelPrimary: modelDraft.primary,
        modelFallbacks: modelDraft.fallbacks,
        paramTemperature: isPlainObject(paramsObj) && paramsObj.temperature !== undefined ? String(paramsObj.temperature) : '',
        paramTopP: isPlainObject(paramsObj) && paramsObj.topP !== undefined ? String(paramsObj.topP) : '',
        paramMaxTokens: isPlainObject(paramsObj) && paramsObj.maxTokens !== undefined ? String(paramsObj.maxTokens) : '',
        identityName: identityDraft.name,
        identityTheme: identityDraft.theme,
        identityEmoji: identityDraft.emoji,
        identityAvatar: identityDraft.avatar,
        sandboxMode: sandboxDraft.mode,
        sandboxScope: sandboxDraft.scope,
        sandboxWorkspaceAccess: sandboxDraft.workspaceAccess,
        sandboxWorkspaceRoot: sandboxDraft.workspaceRoot,
        sandboxDockerNetwork: sandboxDraft.dockerNetwork,
        sandboxDockerReadOnlyRoot: sandboxDraft.dockerReadOnlyRoot,
        sandboxDockerSetupCommand: sandboxDraft.dockerSetupCommand,
        sandboxDockerBinds: sandboxDraft.dockerBinds,
        groupChatMode: triStateFromValue(isPlainObject(groupChatObj) ? getNestedValue(groupChatObj, 'enabled') : undefined),
        toolProfile: (() => {
          const raw = isPlainObject(toolsObj) ? getNestedValue(toolsObj, 'profile') : '';
          return raw === 'minimal' || raw === 'coding' || raw === 'messaging' || raw === 'full' ? raw : '';
        })(),
        toolAllow: isPlainObject(toolsObj) ? parseStringList(getNestedValue(toolsObj, 'allow')).join(', ') : '',
        toolDeny: isPlainObject(toolsObj) ? parseStringList(getNestedValue(toolsObj, 'deny')).join(', ') : '',
        agentToAgentMode: triStateFromValue(isPlainObject(toolsObj) ? getNestedValue(toolsObj, 'agentToAgent.enabled') : undefined),
        agentToAgentAllow: isPlainObject(toolsObj) ? parseStringList(getNestedValue(toolsObj, 'agentToAgent.allow')).join(', ') : '',
        sessionVisibility: (() => {
          const raw = isPlainObject(toolsObj) ? getNestedValue(toolsObj, 'sessions.visibility') : '';
          return raw === 'same-agent' || raw === 'all-agents' ? raw : '';
        })(),
        subagentAllowAgents: isPlainObject(subagentsObj) ? parseStringList(getNestedValue(subagentsObj, 'allowAgents')).join(', ') : '',
      }));
      setSandboxClearIntent(false);
      setMsg('已从 Advanced JSON 同步结构化字段');
      setTimeout(() => setMsg(''), 3000);
    } catch (err) {
      setMsg('同步失败: ' + String(err));
      setTimeout(() => setMsg(''), 4000);
    }
  };

  const saveAgent = async () => {
    setSaveAttempted(true);
    if (saveBlockedReason) {
      setFormSection('basic');
      setMsg(saveBlockedReason);
      return;
    }
    if (sandboxStructuredLocked && structuredTouched.sandbox) {
      setFormSection('advanced');
      setMsg('当前 sandbox 含有结构化表单不支持的值，请先在 Advanced JSON 中调整。');
      return;
    }
    const id = form.id.trim();
    if (!id) {
      setMsg('Agent ID 不能为空');
      return;
    }
    const baseAgent = agents.find(agent => agent.id === (editingId || id));
    let modelObj: any;
    let toolsObj: any;
    let sandboxObj: any;
    let groupChatObj: any;
    let identityObj: any;
    let subagentsObj: any;
    let paramsObj: any;
    let runtimeObj: any;
    let parsedContextTokens: number | undefined;
    let parsedCompactionMaxHistoryShare: number | undefined;
    try {
      modelObj = parseJSONText(form.modelText, 'model');
      toolsObj = parseJSONText(form.toolsText, 'tools');
      sandboxObj = parseJSONText(form.sandboxText, 'sandbox');
      groupChatObj = parseJSONText(form.groupChatText, 'groupChat');
      identityObj = parseJSONText(form.identityText, 'identity');
      subagentsObj = parseJSONText(form.subagentsText, 'subagents');
      paramsObj = parseJSONText(form.paramsText, 'params');
      runtimeObj = parseJSONText(form.runtimeText, 'runtime');

      const primaryModel = form.modelPrimary.trim();
      const fallbackModels = parseCSV(form.modelFallbacks);
      if (structuredTouched.model) {
        if (!primaryModel && fallbackModels.length === 0) {
          modelObj = undefined;
        } else {
          const baseModel = typeof modelObj === 'string' ? { primary: modelObj } : isPlainObject(modelObj) ? deepClone(modelObj) : {};
          baseModel.primary = primaryModel || String(baseModel.primary || '').trim();
          if (!baseModel.primary) {
            throw new Error('Behavior / Model Primary 不能为空');
          }
          if (fallbackModels.length > 0) baseModel.fallbacks = fallbackModels;
          else delete baseModel.fallbacks;
          modelObj = cleanupObject(baseModel);
        }
      }

      if (structuredTouched.params) {
        if (paramsObj !== undefined && !isPlainObject(paramsObj)) {
          throw new Error('params JSON 必须是对象才能与结构化参数合并');
        }
        const nextParams = isPlainObject(paramsObj) ? deepClone(paramsObj) : {};
        const temperature = parseNumberInput(form.paramTemperature, 'temperature');
        const topP = parseNumberInput(form.paramTopP, 'topP');
        const maxTokens = parseNumberInput(form.paramMaxTokens, 'maxTokens');
        if (temperature === undefined) delete nextParams.temperature;
        else nextParams.temperature = temperature;
        if (topP === undefined) delete nextParams.topP;
        else nextParams.topP = topP;
        if (maxTokens === undefined) delete nextParams.maxTokens;
        else nextParams.maxTokens = maxTokens;
        paramsObj = cleanupObject(nextParams);
      }

      parsedContextTokens = parseNumberInput(form.contextTokens, 'contextTokens', { integer: true, min: 1 });
      parsedCompactionMaxHistoryShare = parseNumberInput(form.compactionMaxHistoryShare, 'compaction.maxHistoryShare', { min: 0, max: 1 });

      if (structuredTouched.identity) {
        if (identityObj !== undefined && !isPlainObject(identityObj)) {
          throw new Error('identity JSON 必须是对象才能与结构化字段合并');
        }
        const nextIdentity = isPlainObject(identityObj) ? deepClone(identityObj) : {};
        delete nextIdentity.description;
        delete nextIdentity.tone;
        if (form.identityName.trim()) nextIdentity.name = form.identityName.trim();
        else delete nextIdentity.name;
        if (form.identityTheme.trim()) nextIdentity.theme = form.identityTheme.trim();
        else delete nextIdentity.theme;
        if (form.identityEmoji.trim()) nextIdentity.emoji = form.identityEmoji.trim();
        else delete nextIdentity.emoji;
        if (form.identityAvatar.trim()) nextIdentity.avatar = form.identityAvatar.trim();
        else delete nextIdentity.avatar;
        identityObj = cleanupObject(nextIdentity);
      }

      if (structuredTouched.groupChat) {
        if (groupChatObj !== undefined && !isPlainObject(groupChatObj)) {
          throw new Error('groupChat JSON 必须是对象才能与结构化字段合并');
        }
        const nextGroupChat = isPlainObject(groupChatObj) ? deepClone(groupChatObj) : {};
        const groupChatEnabled = triStateToValue(form.groupChatMode);
        if (groupChatEnabled === undefined) deleteNestedValue(nextGroupChat, 'enabled');
        else setNestedValue(nextGroupChat, 'enabled', groupChatEnabled);
        groupChatObj = cleanupObject(nextGroupChat);
      }

      if (structuredTouched.tools) {
        if (toolsObj !== undefined && !isPlainObject(toolsObj)) {
          throw new Error('tools JSON 必须是对象才能与结构化字段合并');
        }
        const nextTools = isPlainObject(toolsObj) ? deepClone(toolsObj) : {};
        if (form.toolProfile) nextTools.profile = form.toolProfile;
        else delete nextTools.profile;
        const toolAllowList = parseCSV(form.toolAllow);
        if (toolAllowList.length > 0) nextTools.allow = toolAllowList;
        else delete nextTools.allow;
        const toolDenyList = parseCSV(form.toolDeny);
        if (toolDenyList.length > 0) nextTools.deny = toolDenyList;
        else delete nextTools.deny;
        const agentToAgentEnabled = triStateToValue(form.agentToAgentMode);
        if (agentToAgentEnabled === undefined) deleteNestedValue(nextTools, 'agentToAgent.enabled');
        else setNestedValue(nextTools, 'agentToAgent.enabled', agentToAgentEnabled);
        const allowList = parseCSV(form.agentToAgentAllow);
        if (allowList.length > 0) setNestedValue(nextTools, 'agentToAgent.allow', allowList);
        else deleteNestedValue(nextTools, 'agentToAgent.allow');
        if (form.sessionVisibility) setNestedValue(nextTools, 'sessions.visibility', form.sessionVisibility);
        else deleteNestedValue(nextTools, 'sessions.visibility');
        toolsObj = cleanupObject(nextTools);
      }

      if (structuredTouched.sandbox) {
        if (sandboxObj !== undefined && !isPlainObject(sandboxObj)) {
          throw new Error('sandbox JSON 必须是对象才能与结构化字段合并');
        }
        const nextSandbox = isPlainObject(sandboxObj) ? deepClone(sandboxObj) : {};
        if (form.sandboxMode) nextSandbox.mode = form.sandboxMode;
        else delete nextSandbox.mode;
        if (form.sandboxScope) nextSandbox.scope = form.sandboxScope;
        else delete nextSandbox.scope;
        if (form.sandboxWorkspaceAccess) nextSandbox.workspaceAccess = form.sandboxWorkspaceAccess;
        else delete nextSandbox.workspaceAccess;
        if (form.sandboxWorkspaceRoot.trim()) nextSandbox.workspaceRoot = form.sandboxWorkspaceRoot.trim();
        else delete nextSandbox.workspaceRoot;

        const nextDocker = isPlainObject(nextSandbox.docker) ? nextSandbox.docker : {};
        if (form.sandboxDockerNetwork.trim()) nextDocker.network = form.sandboxDockerNetwork.trim();
        else delete nextDocker.network;
        const readOnlyRoot = triStateToValue(form.sandboxDockerReadOnlyRoot);
        if (readOnlyRoot === undefined) delete nextDocker.readOnlyRoot;
        else nextDocker.readOnlyRoot = readOnlyRoot;
        if (form.sandboxDockerSetupCommand.trim()) nextDocker.setupCommand = form.sandboxDockerSetupCommand.trim();
        else delete nextDocker.setupCommand;
        const binds = parseLineList(form.sandboxDockerBinds);
        if (binds.length > 0) nextDocker.binds = binds;
        else delete nextDocker.binds;
        if (Object.keys(nextDocker).length > 0) nextSandbox.docker = nextDocker;
        else delete nextSandbox.docker;
        sandboxObj = cleanupObject(nextSandbox);
      }

      if (structuredTouched.subagents) {
        if (subagentsObj !== undefined && !isPlainObject(subagentsObj)) {
          throw new Error('subagents JSON 必须是对象才能与结构化字段合并');
        }
        const nextSubagents = isPlainObject(subagentsObj) ? deepClone(subagentsObj) : {};
        const allowList = parseCSV(form.subagentAllowAgents);
        if (allowList.length > 0) setNestedValue(nextSubagents, 'allowAgents', allowList);
        else deleteNestedValue(nextSubagents, 'allowAgents');
        subagentsObj = cleanupObject(nextSubagents);
      }
    } catch (err) {
      setMsg(String(err));
      return;
    }

    const payload: any = {
      id,
      name: form.name.trim() || undefined,
      workspace: form.workspace.trim() || undefined,
      agentDir: form.agentDir.trim() || undefined,
      default: effectiveIsDefault,
    };
    const clearsSandboxOverride = structuredTouched.sandbox && sandboxObj === undefined && form.sandboxText.trim() !== '' && !!editingId;
    const clearsToolsOverride = structuredTouched.tools && toolsObj === undefined && form.toolsText.trim() !== '' && !!editingId;
    if (modelObj !== undefined) payload.model = modelObj;
    if (clearsToolsOverride && editingId) payload.tools = null;
    else if (toolsObj !== undefined) payload.tools = toolsObj;
    if (clearsSandboxOverride && editingId) payload.sandbox = null;
    else if (sandboxObj !== undefined) payload.sandbox = sandboxObj;
    if (groupChatObj !== undefined) payload.groupChat = groupChatObj;
    if (identityObj !== undefined) payload.identity = identityObj;
    if (subagentsObj !== undefined) payload.subagents = subagentsObj;
    if (paramsObj !== undefined) payload.params = paramsObj;
    if (runtimeObj !== undefined) payload.runtime = runtimeObj;
    if (structuredTouched.context) {
      if (parsedContextTokens === undefined) {
        if (baseAgent?.contextTokens !== undefined) payload.contextTokens = null;
      } else {
        payload.contextTokens = parsedContextTokens;
      }

      const nextCompaction = isPlainObject(baseAgent?.compaction) ? deepClone(baseAgent.compaction) : {};
      if (form.compactionMode) nextCompaction.mode = form.compactionMode;
      else delete nextCompaction.mode;
      if (parsedCompactionMaxHistoryShare === undefined) delete nextCompaction.maxHistoryShare;
      else nextCompaction.maxHistoryShare = parsedCompactionMaxHistoryShare;
      const cleanedCompaction = cleanupObject(nextCompaction);
      if (cleanedCompaction === undefined) {
        if (isPlainObject(baseAgent?.compaction)) payload.compaction = null;
      } else {
        payload.compaction = cleanedCompaction;
      }
    }

    setSaving(true);
    try {
      const response = (editingId && !materializingImplicitAgent)
        ? await api.updateAgent(editingId, payload)
        : await api.createAgent(payload);
      if (response?.ok === false) {
        setMsg('保存失败: ' + (response.error || 'unknown error'));
        return;
      }
      const affectedAgentID = editingId || payload.id;
      setMsg('Agent 保存成功');
      closeForm();
      await loadData();
      const isCurrentAgent = selectedAgentId === affectedAgentID;
      if (detailTab === 'files') {
        if (isCurrentAgent) {
          if (confirmDiscardCoreFileDraft('并刷新核心文件列表')) {
            await loadAgentCoreFiles(affectedAgentID, true);
          } else {
            setMsg('Agent 已保存；当前核心文件草稿未刷新，请手动保存或刷新。');
          }
        } else {
          setCoreFilesByAgent(prev => {
            if (!prev[affectedAgentID]) return prev;
            const next = { ...prev };
            delete next[affectedAgentID];
            return next;
          });
        }
      } else {
        setCoreFilesByAgent(prev => {
          if (!prev[affectedAgentID]) return prev;
          const next = { ...prev };
          delete next[affectedAgentID];
          return next;
        });
      }
      if (detailTab === 'capabilities' || detailTab === 'context') {
        if (isCurrentAgent) {
          await loadAgentSessions(affectedAgentID, true);
        } else {
          setSessionsByAgent(prev => {
            if (!prev[affectedAgentID]) return prev;
            const next = { ...prev };
            delete next[affectedAgentID];
            return next;
          });
        }
      } else {
        setSessionsByAgent(prev => {
          if (!prev[affectedAgentID]) return prev;
          const next = { ...prev };
          delete next[affectedAgentID];
          return next;
        });
      }
    } catch (err) {
      setMsg('保存失败: ' + String(err));
    } finally {
      setSaving(false);
      setTimeout(() => setMsg(''), 4000);
    }
  };

  const deleteAgent = async (agent: AgentItem) => {
    if (!window.confirm(`确认删除 Agent "${agent.id}"？`)) return;
    const preserveSessions = window.confirm('是否保留该 Agent 的 sessions 文件？\n确定=保留，取消=删除');
    try {
      const r = await api.deleteAgent(agent.id, preserveSessions);
      if (r?.ok === false) {
        setMsg('删除失败: ' + (r.error || 'unknown error'));
        return;
      }
      setMsg('删除成功');
      await loadData();
    } catch (err) {
      setMsg('删除失败: ' + String(err));
    } finally {
      setTimeout(() => setMsg(''), 4000);
    }
  };

  const setBindingAt = (idx: number, updater: (row: BindingDraft) => BindingDraft) => {
    setBindings(prev => prev.map((row, i) => (i === idx ? updater(row) : row)));
  };

  const touchBindingMatch = (idx: number, updater: (match: Record<string, any>) => Record<string, any>) => {
    setBindingAt(idx, row => {
      const nextMatch = compactMatch(updater(deepClone(row.match || {})));
      return {
        ...row,
        match: nextMatch,
        matchText: JSON.stringify(nextMatch, null, 2),
        rowError: '',
      };
    });
  };

  const setPeerField = (idx: number, key: 'peer' | 'parentPeer', part: 'kind' | 'id', value: string) => {
    touchBindingMatch(idx, cur => {
      const now = extractPeerForm(cur, key);
      const kind = part === 'kind' ? value.trim() : now.kind;
      const id = part === 'id' ? value.trim() : now.id;
      if (!kind && !id) {
        delete cur[key];
        return cur;
      }
      if (!kind) {
        // 结构化模式下 peer 必须有 kind，避免生成歧义字符串。
        delete cur[key];
        return cur;
      }
      cur[key] = id ? { kind, id } : { kind };
      return cur;
    });
  };

  const switchBindingMode = (idx: number, targetMode: 'structured' | 'json') => {
    setBindingAt(idx, row => {
      if (row.mode === targetMode) return row;
      if (targetMode === 'json') {
        const match = compactMatch(row.match);
        return {
          ...row,
          mode: 'json',
          match,
          matchText: JSON.stringify(match, null, 2),
          rowError: '',
        };
      }

      try {
        const parsed = compactMatch(JSON.parse(row.matchText || '{}'));
        if (!isStructuredMatchSupported(parsed)) {
          return {
            ...row,
            rowError: '当前 match 包含数组或高级表达式，请继续使用 JSON 模式。',
          };
        }
        return {
          ...row,
          mode: 'structured',
          match: parsed,
          matchText: JSON.stringify(parsed, null, 2),
          rowError: '',
        };
      } catch (err) {
        return {
          ...row,
          rowError: 'JSON 解析失败，无法切换到结构化模式: ' + String(err),
        };
      }
    });
  };

  const saveBindings = async () => {
    const parsed: any[] = [];
    const nextBindings = [...bindings];

    for (let i = 0; i < nextBindings.length; i++) {
      const row = nextBindings[i];
      if (!row.agent.trim()) {
        setMsg(`第 ${i + 1} 条 binding 缺少 agent`);
        return;
      }

      let matchObj: Record<string, any>;
      if (row.mode === 'json') {
        try {
          const parsedRaw = JSON.parse(row.matchText || '{}');
          const clientError = validateBindingMatchClient(parsedRaw, i + 1);
          if (clientError) {
            nextBindings[i] = {
              ...row,
              rowError: clientError,
            };
            setBindings(nextBindings);
            setMsg(clientError);
            return;
          }
          matchObj = compactMatch(parsedRaw);
          nextBindings[i] = {
            ...row,
            match: matchObj,
            matchText: JSON.stringify(matchObj, null, 2),
            rowError: '',
          };
        } catch (err) {
          nextBindings[i] = {
            ...row,
            rowError: `match JSON 错误: ${String(err)}`,
          };
          setBindings(nextBindings);
          setMsg(`第 ${i + 1} 条 binding 的 match JSON 错误`);
          return;
        }
      } else {
        matchObj = compactMatch(row.match);
      }

      const clientError = validateBindingMatchClient(matchObj, i + 1);
      if (clientError) {
        nextBindings[i] = {
          ...row,
          rowError: clientError,
        };
        setBindings(nextBindings);
        setMsg(clientError);
        return;
      }

      parsed.push({
        name: row.name.trim() || undefined,
        agentId: row.agent.trim(),
        enabled: row.enabled,
        match: matchObj,
      });
    }

    setBindings(nextBindings);
    setSaving(true);
    try {
      const r = await api.updateBindings(parsed);
      if (r?.ok === false) {
        setMsg('Bindings 保存失败: ' + (r.error || 'unknown error'));
        return;
      }
      setMsg('Bindings 保存成功');
      await loadData();
    } catch (err) {
      setMsg('Bindings 保存失败: ' + String(err));
    } finally {
      setSaving(false);
      setTimeout(() => setMsg(''), 4000);
    }
  };

  const addBinding = () => {
    const firstChannel = channelOptions[0] || 'qq';
    const match = compactMatch({ channel: firstChannel });
    setExpandedBindingIndex(bindings.length);
    setBindings(prev => [
      ...prev,
      {
        name: '',
        agent: defaultAgent || agentOptions[0] || 'main',
        enabled: true,
        match,
        matchText: JSON.stringify(match, null, 2),
        mode: 'structured',
        rowError: '',
      },
    ]);
  };

  const removeBinding = (idx: number) => {
    setBindings(prev => prev.filter((_, i) => i !== idx));
    setExpandedBindingIndex(prev => {
      if (prev === null) return null;
      if (prev === idx) return null;
      if (prev > idx) return prev - 1;
      return prev;
    });
  };

  const moveBinding = (idx: number, delta: number) => {
    const to = idx + delta;
    if (to < 0 || to >= bindings.length) return;
    setBindings(prev => {
      const arr = [...prev];
      const [item] = arr.splice(idx, 1);
      arr.splice(to, 0, item);
      return arr;
    });
    setExpandedBindingIndex(prev => {
      if (prev === null) return null;
      if (prev === idx) return to;
      if (delta > 0 && prev > idx && prev <= to) return prev - 1;
      if (delta < 0 && prev < idx && prev >= to) return prev + 1;
      return prev;
    });
  };

  const runPreview = async () => {
    const meta = buildPreviewMetaPayload(previewMeta, channelMeta);

    setPreviewLoading(true);
    try {
      const r = await api.previewRoute(meta);
      if (r.ok) {
        setPreviewResult(r.result || {});
      } else {
        setPreviewResult({ trace: [r.error || '预览失败'] });
      }
    } catch (err) {
      setPreviewResult({ trace: [String(err)] });
    } finally {
      setPreviewLoading(false);
    }
  };

  const selectedModelDraft = extractModelDraft(selectedAgent?.model);
  const selectedCompactionDraft = extractCompactionDraft(selectedAgent?.compaction);
  const selectedIdentity = extractIdentityDraft(selectedAgent?.identity);
  const selectedTools = isPlainObject(selectedAgent?.tools) ? selectedAgent.tools : {};
  const selectedSubagents = isPlainObject(selectedAgent?.subagents) ? selectedAgent.subagents : {};
  const selectedGroupChat = isPlainObject(selectedAgent?.groupChat) ? selectedAgent.groupChat : {};
  const selectedSandboxDraft = extractSandboxDraft(selectedAgent?.sandbox);
  const selectedAdvancedBlocks = summarizeAdvancedBlocks(selectedAgent);
  const selectedRouteCount = selectedAgentBindings.length;
  const selectedToolAllow = parseStringList(getNestedValue(selectedTools, 'allow'));
  const selectedToolDeny = parseStringList(getNestedValue(selectedTools, 'deny'));
  const selectedToolProfile = String(getNestedValue(selectedTools, 'profile') || '').trim();

  if (loading) {
    return (
      <div className="py-16 text-center text-gray-400 text-sm">
        <RefreshCw size={18} className="animate-spin inline mr-2" />
        加载中...
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <datalist id="agent-channel-options">
        {channelOptions.map(ch => (
          <option key={ch} value={ch} />
        ))}
      </datalist>
      <datalist id="agent-model-options">
        {modelOptions.map(model => (
          <option key={model} value={model} />
        ))}
      </datalist>

      <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h2 className="text-xl font-bold text-gray-900 dark:text-white">智能体（Agents）</h2>
          <p className="text-sm text-gray-500 mt-1">在这里维护智能体配置、查看核心文件，并快速检查一条消息最终会交给谁处理。</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button onClick={loadData} className="flex items-center gap-2 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 transition-colors shadow-sm">
            <RefreshCw size={14} /> 刷新（Refresh）
          </button>
          <button onClick={() => openCreate('basic')} className="flex items-center gap-2 px-4 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 shadow-sm shadow-violet-200 dark:shadow-none transition-all">
            <Plus size={14} /> 新建智能体（New Agent）
          </button>
        </div>
      </div>

      {msg && (
        <div className={`px-4 py-3 rounded-lg text-sm ${msg.includes('失败') || msg.includes('错误') ? 'bg-red-50 dark:bg-red-900/20 text-red-600' : 'bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600'}`}>
          {msg}
        </div>
      )}

      <div className="rounded-xl border border-violet-100 dark:border-violet-900/40 bg-violet-50/80 dark:bg-violet-950/20 p-4">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <p className="text-xs font-semibold tracking-wide text-violet-700 dark:text-violet-300 uppercase">使用方式（How it works）</p>
            <h3 className="text-base font-semibold text-gray-900 dark:text-white mt-1">先管理单个智能体，再检查路由命中</h3>
            <p className="text-sm text-gray-600 dark:text-gray-300 mt-1">
              “Agent 目录”适合查看单个智能体的配置、文件与上下文；“路由工作台”适合检查规则、模拟消息并确认最终分流结果。
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              onClick={() => setWorkbenchView('directory')}
              className={`inline-flex items-center gap-2 rounded-lg border px-3 py-2 text-xs font-medium transition-colors ${
                workbenchView === 'directory'
                  ? 'border-violet-600 bg-violet-600 text-white'
                  : 'border-white/80 dark:border-gray-700 bg-white dark:bg-gray-900 text-gray-600 dark:text-gray-300 hover:border-violet-300'
              }`}
            >
              <Bot size={14} />
                Agent 目录（Directory）
            </button>
            <button
              onClick={() => setWorkbenchView('routing')}
              className={`inline-flex items-center gap-2 rounded-lg border px-3 py-2 text-xs font-medium transition-colors ${
                workbenchView === 'routing'
                  ? 'border-violet-600 bg-violet-600 text-white'
                  : 'border-white/80 dark:border-gray-700 bg-white dark:bg-gray-900 text-gray-600 dark:text-gray-300 hover:border-violet-300'
              }`}
            >
              <Route size={14} />
                路由工作台（Routing Studio）
            </button>
          </div>
        </div>
      </div>

      {workbenchView === 'directory' ? (
        <div className="grid grid-cols-1 xl:grid-cols-[320px,1fr] gap-6">
          <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-100 dark:border-gray-700/50 shadow-sm">
            <div className="px-4 py-3 border-b border-gray-100 dark:border-gray-700/50 flex items-center justify-between">
              <div>
                <h3 className="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                  <Bot size={15} className="text-violet-500" />
                  Agent 目录（Directory）
                </h3>
                <p className="text-[11px] text-gray-500 mt-1">先选 Agent，再看它的配置、核心文件、能力快照与路由上下文。</p>
              </div>
              <span className="text-xs text-gray-500">默认（Default）: {defaultAgent}</span>
            </div>
            <div className="p-4 space-y-3">
              {agents.length === 0 ? (
                <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-8 text-center text-sm text-gray-400">
                    还没有显式 Agent，先创建一个吧（Create your first explicit Agent）。
                </div>
              ) : (
                agents.map(agent => {
                  const implicitAgent = isImplicitAgent(agent);
                  const active = selectedAgent?.id === agent.id;
                  const identity = isPlainObject(agent.identity) ? agent.identity : {};
                  const modelDraft = extractModelDraft(agent.model);
                  const cardTitle = String(agent.name || identity.name || '').trim() || agent.id;
                  const routeCount = bindings.filter(item => item.agent === agent.id).length;
                  return (
                    <div
                      key={agent.id}
                      className={`rounded-xl border p-3 transition-colors ${
                        active
                          ? 'border-violet-300 bg-violet-50/80 dark:bg-violet-950/20 dark:border-violet-800/60'
                          : 'border-gray-100 dark:border-gray-700 hover:border-violet-200 dark:hover:border-violet-800/60'
                      }`}
                    >
                      <button
                        onClick={() => {
                          if (agent.id !== selectedAgentId && !confirmDiscardCoreFileDraft('并切换到另一个 Agent')) return;
                          setSelectedAgentId(agent.id);
                          setDetailTab('overview');
                        }}
                        className="w-full text-left"
                      >
                        <div className="flex items-start justify-between gap-2">
                          <div>
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="font-medium text-sm text-gray-900 dark:text-white">{cardTitle}</span>
                              {agent.default && <span className="text-[10px] px-1.5 py-0.5 rounded bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-200">默认</span>}
                              {implicitAgent && <span className="text-[10px] px-1.5 py-0.5 rounded bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-200">占位</span>}
                            </div>
                            <div className="mt-1 font-mono text-[11px] text-gray-500">{agent.id}</div>
                          </div>
                          <span className="text-[11px] text-gray-400">{agent.sessions ?? 0} 会话</span>
                        </div>
                        <div className="mt-3 grid grid-cols-2 gap-2 text-[11px] text-gray-500">
                          <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-2.5 py-2">
                              <div className="text-gray-400">模型（Model）</div>
                            <div className="mt-1 truncate text-gray-700 dark:text-gray-200">{modelDraft.primary || defaultModelHint || '继承默认'}</div>
                          </div>
                          <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-2.5 py-2">
                              <div className="text-gray-400">路由规则（Bindings）</div>
                            <div className="mt-1 text-gray-700 dark:text-gray-200">{routeCount > 0 ? `${routeCount} 条` : '未绑定'}</div>
                          </div>
                        </div>
                        <div className="mt-3 space-y-1 text-[11px] text-gray-500">
                          <div>工作区（Workspace）：<span className="font-mono text-gray-700 dark:text-gray-200">{agent.workspace || '—'}</span></div>
                          <div>最后活跃（Last Active）：<span className="text-gray-700 dark:text-gray-200">{formatLastActive(agent.lastActive)}</span></div>
                        </div>
                      </button>
                      <div className="mt-3 flex items-center gap-2">
                        <button onClick={() => openEdit(agent)} className="px-2.5 py-1.5 text-xs rounded bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600">
                           {implicitAgent ? '配置（Materialize）' : '编辑（Edit）'}
                        </button>
                        <button
                          disabled={implicitAgent}
                          title={implicitAgent ? '运行时占位 Agent 无需删除' : ''}
                          onClick={() => deleteAgent(agent)}
                          className="px-2.5 py-1.5 text-xs rounded bg-red-50 text-red-600 hover:bg-red-100 disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                           删除（Delete）
                        </button>
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          </div>

          <div className="space-y-4">
            {selectedAgent ? (
              <>
                <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-100 dark:border-gray-700/50 shadow-sm p-5">
                  <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                    <div className="space-y-3">
                      <div>
                        <div className="flex flex-wrap items-center gap-2">
                          <h3 className="text-lg font-semibold text-gray-900 dark:text-white">{String(selectedAgent.name || selectedIdentity.name || '').trim() || selectedAgent.id}</h3>
                          {selectedAgent.default && <span className="text-[10px] px-1.5 py-0.5 rounded bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-200">默认 Agent</span>}
                          {isImplicitAgent(selectedAgent) && <span className="text-[10px] px-1.5 py-0.5 rounded bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-200">运行时占位</span>}
                        </div>
                        <p className="mt-1 text-sm text-gray-500">
                          <span className="font-mono">{selectedAgent.id}</span>
                          {selectedIdentity.theme ? ` · ${selectedIdentity.theme}` : ''}
                        </p>
                      </div>
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 px-3 py-3 bg-gray-50/80 dark:bg-gray-900">
                          <div className="text-gray-400">模型（Model）</div>
                          <div className="mt-1 font-medium text-gray-900 dark:text-white">{selectedModelDraft.primary || defaultModelHint || '继承默认'}</div>
                          <div className="mt-1 text-gray-500">fallback：{selectedModelDraft.fallbacks || '未设置'}</div>
                        </div>
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 px-3 py-3 bg-gray-50/80 dark:bg-gray-900">
                          <div className="text-gray-400">工具与权限（Tools & Access）</div>
                          <div className="mt-1 font-medium text-gray-900 dark:text-white">{describeSandboxMode(selectedAgent.sandbox)}</div>
                          <div className="mt-1 text-gray-500">{describeSessionVisibility((getNestedValue(selectedTools, 'sessions.visibility') as '' | 'same-agent' | 'all-agents') || '')}</div>
                        </div>
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 px-3 py-3 bg-gray-50/80 dark:bg-gray-900">
                          <div className="text-gray-400">路由上下文（Routing Context）</div>
                          <div className="mt-1 font-medium text-gray-900 dark:text-white">{selectedRouteCount > 0 ? `${selectedRouteCount} 条规则指向它` : '暂无显式规则'}</div>
                          <div className="mt-1 text-gray-500">{selectedAgent.sessions ?? 0} 个活跃会话</div>
                        </div>
                      </div>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      <button onClick={() => openEdit(selectedAgent)} className="px-3 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700">
                        编辑当前智能体（Edit Agent）
                      </button>
                      <button onClick={() => setWorkbenchView('routing')} className="px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300">
                        查看路由工作台（Open Routing Studio）
                      </button>
                    </div>
                  </div>
                </div>

                <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-100 dark:border-gray-700/50 shadow-sm">
                  <div className="px-4 py-4 border-b border-gray-100 dark:border-gray-700/50">
                    <div className="flex flex-wrap gap-2">
                      {AGENT_DIRECTORY_TABS.map(tab => (
                        <button
                          key={tab.id}
                          onClick={() => setDetailTab(tab.id)}
                          className={`px-3 py-2 rounded-lg text-xs border transition-colors ${
                            detailTab === tab.id
                              ? 'bg-violet-600 text-white border-violet-600'
                              : 'bg-white dark:bg-gray-900 text-gray-600 dark:text-gray-300 border-gray-200 dark:border-gray-700 hover:border-violet-300'
                          }`}
                        >
                          <div className="font-medium">{tab.title}</div>
                          <div className={`mt-0.5 ${detailTab === tab.id ? 'text-violet-100' : 'text-gray-400'}`}>{tab.description}</div>
                        </button>
                      ))}
                    </div>
                  </div>
                  <div className="p-4">
                    {detailTab === 'overview' && (
                      <div className="space-y-4">
                        <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4">
                            <div className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                              <Brain size={15} className="text-violet-500" />
                              模型与身份（Model & Identity）
                            </div>
                            <div className="mt-3 space-y-2 text-sm text-gray-600 dark:text-gray-300">
                              <div>主模型（Primary）：<span className="font-medium text-gray-900 dark:text-white">{selectedModelDraft.primary || defaultModelHint || '继承默认'}</span></div>
                              <div>回退模型（Fallbacks）：{selectedModelDraft.fallbacks || '未设置'}</div>
                              <div>上下文预算（contextTokens）：{selectedAgent?.contextTokens !== undefined ? String(selectedAgent.contextTokens) : (agentDefaults.contextTokens !== undefined ? `继承默认 (${agentDefaults.contextTokens})` : '继承默认')}</div>
                              <div>压缩模式（compaction.mode）：{selectedCompactionDraft.mode || agentDefaults.compactionMode || '继承默认'}</div>
                              <div>身份名（Name）：{selectedIdentity.name || '未设置'}</div>
                              <div>主题（Theme）：{selectedIdentity.theme || '未设置'}</div>
                              <div>表情（Emoji）：{selectedIdentity.emoji || '未设置'}</div>
                            </div>
                            <button onClick={() => openEdit(selectedAgent, 'behavior')} className="mt-4 px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600">
                              编辑模型与身份（Edit Model & Identity）
                            </button>
                          </div>
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4">
                            <div className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                              <Shield size={15} className="text-violet-500" />
                              工具与权限（Tools & Access）
                            </div>
                            <div className="mt-3 space-y-2 text-sm text-gray-600 dark:text-gray-300">
                              <div>Sandbox：<span className="font-medium text-gray-900 dark:text-white">{describeSandboxMode(selectedAgent.sandbox)}</span></div>
                              <div>Agent 协作：{describeTriState(triStateFromValue(getNestedValue(selectedTools, 'agentToAgent.enabled')), '允许委派', '禁用委派')}</div>
                              <div>会话可见性：{describeSessionVisibility((getNestedValue(selectedTools, 'sessions.visibility') as '' | 'same-agent' | 'all-agents') || '')}</div>
                              <div>群聊模式：{describeTriState(triStateFromValue(getNestedValue(selectedGroupChat, 'enabled')), '显式启用', '显式关闭')}</div>
                            </div>
                            <button onClick={() => openEdit(selectedAgent, 'access')} className="mt-4 px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600">
                              编辑工具与权限（Edit Tools & Access）
                            </button>
                          </div>
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4">
                            <div className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                              <Route size={15} className="text-violet-500" />
                              路由与上下文（Routing Context）
                            </div>
                            <div className="mt-3 space-y-2 text-sm text-gray-600 dark:text-gray-300">
                              <div>路由规则：<span className="font-medium text-gray-900 dark:text-white">{selectedRouteCount > 0 ? `${selectedRouteCount} 条命中到它` : '暂无规则'}</span></div>
                              <div>活跃会话：{selectedAgent.sessions ?? 0}</div>
                              <div>最后活跃：{formatLastActive(selectedAgent.lastActive)}</div>
                              <div>workspace：<span className="font-mono">{selectedAgent.workspace || '—'}</span></div>
                            </div>
                            <button
                              onClick={() => {
                                setWorkbenchView('routing');
                                setDetailTab('context');
                              }}
                              className="mt-4 px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600"
                            >
                              打开路由工作台（Open Routing Studio）
                            </button>
                          </div>
                        </div>
                      </div>
                    )}

                    {detailTab === 'model' && (
                      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-3">
                          <div>
                            <h4 className="text-sm font-semibold text-gray-900 dark:text-white">模型选择（Model Settings）</h4>
                            <p className="text-xs text-gray-500 mt-1">明确区分“当前设置”和“继承默认”的关系。</p>
                          </div>
                          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">主模型（Primary Model）</div>
                              <div className="mt-1 font-medium text-gray-900 dark:text-white">{selectedModelDraft.primary || '继承默认'}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">回退模型（Fallbacks）</div>
                              <div className="mt-1 text-gray-700 dark:text-gray-200">{selectedModelDraft.fallbacks || '未设置'}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">温度（temperature）</div>
                              <div className="mt-1 text-gray-700 dark:text-gray-200">{isPlainObject(selectedAgent.params) && selectedAgent.params.temperature !== undefined ? String(selectedAgent.params.temperature) : '未覆盖'}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">最大输出（maxTokens）</div>
                              <div className="mt-1 text-gray-700 dark:text-gray-200">{isPlainObject(selectedAgent.params) && selectedAgent.params.maxTokens !== undefined ? String(selectedAgent.params.maxTokens) : '未覆盖'}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">上下文预算（contextTokens）</div>
                              <div className="mt-1 text-gray-700 dark:text-gray-200">
                                {selectedAgent?.contextTokens !== undefined
                                  ? String(selectedAgent.contextTokens)
                                  : agentDefaults.contextTokens !== undefined
                                    ? `继承默认 (${agentDefaults.contextTokens})`
                                    : '继承默认'}
                              </div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">压缩模式（compaction.mode）</div>
                              <div className="mt-1 text-gray-700 dark:text-gray-200">
                                {selectedCompactionDraft.mode || agentDefaults.compactionMode || '继承默认'}
                              </div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">历史占比上限（compaction.maxHistoryShare）</div>
                              <div className="mt-1 text-gray-700 dark:text-gray-200">
                                {selectedCompactionDraft.maxHistoryShare
                                  || (agentDefaults.compactionMaxHistoryShare !== undefined ? String(agentDefaults.compactionMaxHistoryShare) : '')
                                  || '继承默认'}
                              </div>
                            </div>
                          </div>
                          <button onClick={() => openEdit(selectedAgent, 'behavior')} className="px-3 py-2 text-xs rounded-lg bg-violet-600 text-white hover:bg-violet-700">
                            编辑模型参数（Edit Model Settings）
                          </button>
                        </div>
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-3">
                          <div>
                            <h4 className="text-sm font-semibold text-gray-900 dark:text-white">身份摘要（Identity Summary）</h4>
                            <p className="text-xs text-gray-500 mt-1">业务方通常先理解“这个 Agent 是谁、以什么口吻工作”。</p>
                          </div>
                            <div className="space-y-3 text-sm">
                              <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                                <div className="text-xs text-gray-400">显示名称（Name）</div>
                                <div className="mt-1 text-gray-900 dark:text-white">{selectedIdentity.name || '未设置'}</div>
                              </div>
                              <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                                <div className="text-xs text-gray-400">主题（Theme）</div>
                                <div className="mt-1 text-gray-700 dark:text-gray-200 whitespace-pre-wrap">{selectedIdentity.theme || '未设置'}</div>
                              </div>
                              <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                                <div className="text-xs text-gray-400">表情（Emoji）</div>
                                <div className="mt-1 text-gray-700 dark:text-gray-200">{selectedIdentity.emoji || '未设置'}</div>
                              </div>
                              <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                                <div className="text-xs text-gray-400">头像（Avatar）</div>
                                <div className="mt-1 text-gray-700 dark:text-gray-200 break-all">{selectedIdentity.avatar || '未设置'}</div>
                              </div>
                            </div>
                          <button onClick={() => openEdit(selectedAgent, 'behavior')} className="px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600">
                              编辑身份摘要（Edit Identity）
                            </button>
                          </div>
                      </div>
                    )}

                    {detailTab === 'tools' && (
                      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                          <div>
                            <h4 className="text-sm font-semibold text-gray-900 dark:text-white">协作与会话可见性（Collaboration & Sessions）</h4>
                            <p className="text-xs text-gray-500 mt-1">先给业务方可读摘要，再决定是否进入编辑弹窗或高级 JSON。</p>
                          </div>
                          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">Agent 协作（Delegation）</div>
                              <div className="mt-1 text-gray-900 dark:text-white">{describeTriState(triStateFromValue(getNestedValue(selectedTools, 'agentToAgent.enabled')), '允许委派', '禁用委派')}</div>
                              <div className="mt-1 text-xs text-gray-500">{parseStringList(getNestedValue(selectedTools, 'agentToAgent.allow')).join(', ') || 'allow 未设置'}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">会话可见范围（Session Visibility）</div>
                              <div className="mt-1 text-gray-900 dark:text-white">{describeSessionVisibility((getNestedValue(selectedTools, 'sessions.visibility') as '' | 'same-agent' | 'all-agents') || '')}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">群聊模式（Group Chat）</div>
                              <div className="mt-1 text-gray-900 dark:text-white">{describeTriState(triStateFromValue(getNestedValue(selectedGroupChat, 'enabled')), '显式启用', '显式关闭')}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">子 Agent allow（Subagents）</div>
                              <div className="mt-1 text-gray-900 dark:text-white">{parseStringList(getNestedValue(selectedSubagents, 'allowAgents')).join(', ') || '未限制'}</div>
                            </div>
                          </div>
                        </div>
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                          <div>
                            <h4 className="text-sm font-semibold text-gray-900 dark:text-white">工具策略与执行权限（Tool Policy & Sandbox）</h4>
                            <p className="text-xs text-gray-500 mt-1">这里把 “能不能调用工具” 和 “工具在哪里执行” 分开看，避免把 sandbox 误当成完整权限模型。</p>
                          </div>
                          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">工具配置（Tools Profile）</div>
                              <div className="mt-1 text-gray-900 dark:text-white">{selectedToolProfile || '继承默认'}</div>
                              <div className="mt-1 text-xs text-gray-500">allow {selectedToolAllow.length} 项 · deny {selectedToolDeny.length} 项</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">Sandbox 模式</div>
                              <div className="mt-1 text-gray-900 dark:text-white">{describeSandboxModeValue(selectedSandboxDraft.mode)}</div>
                              <div className="mt-1 text-xs text-gray-500">scope：{describeSandboxScopeValue(selectedSandboxDraft.scope)}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">工作区挂载（workspaceAccess）</div>
                              <div className="mt-1 text-gray-900 dark:text-white">{describeWorkspaceAccessValue(selectedSandboxDraft.workspaceAccess)}</div>
                              <div className="mt-1 text-xs text-gray-500 font-mono">{selectedSandboxDraft.workspaceRoot || '使用默认 sandbox workspace'}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">Docker 覆盖</div>
                              <div className="mt-1 text-gray-900 dark:text-white">{selectedSandboxDraft.dockerBinds ? `${parseLineList(selectedSandboxDraft.dockerBinds).length} 条 bind` : '未设置 bind'}</div>
                              <div className="mt-1 text-xs text-gray-500">network：{selectedSandboxDraft.dockerNetwork || '默认'} · readOnlyRoot：{describeToggleValue(selectedSandboxDraft.dockerReadOnlyRoot, '启用', '关闭')}</div>
                            </div>
                          </div>
                          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
                            <div className="rounded-lg border border-gray-100 dark:border-gray-700 px-3 py-3">
                              <div className="text-gray-400">工作区（Workspace）</div>
                              <div className="mt-1 font-mono text-gray-700 dark:text-gray-200">{selectedAgent.workspace || '未设置'}</div>
                            </div>
                            <div className="rounded-lg border border-gray-100 dark:border-gray-700 px-3 py-3">
                              <div className="text-gray-400">Agent 目录（AgentDir）</div>
                              <div className="mt-1 font-mono text-gray-700 dark:text-gray-200">{selectedAgent.agentDir || '未设置'}</div>
                            </div>
                          </div>
                          {(selectedToolAllow.length > 0 || selectedToolDeny.length > 0) && (
                            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
                              <div className="rounded-lg border border-gray-100 dark:border-gray-700 px-3 py-3">
                                <div className="text-gray-400">allow（显式允许）</div>
                                <div className="mt-1 text-gray-700 dark:text-gray-200 break-words">{selectedToolAllow.join(', ') || '未设置'}</div>
                              </div>
                              <div className="rounded-lg border border-gray-100 dark:border-gray-700 px-3 py-3">
                                <div className="text-gray-400">deny（显式拒绝）</div>
                                <div className="mt-1 text-gray-700 dark:text-gray-200 break-words">{selectedToolDeny.join(', ') || '未设置'}</div>
                              </div>
                            </div>
                          )}
                          <button onClick={() => openEdit(selectedAgent, 'access')} className="px-3 py-2 text-xs rounded-lg bg-violet-600 text-white hover:bg-violet-700">
                            编辑工具与权限（Edit Tools & Access）
                          </button>
                        </div>
                      </div>
                    )}

                    {detailTab === 'files' && (
                      <div className="space-y-4">
                        <div className="rounded-xl border border-violet-100 bg-violet-50 px-4 py-3 text-sm text-violet-700 dark:border-violet-900/40 dark:bg-violet-950/20 dark:text-violet-200">
                          对标官方 <span className="font-mono">Files</span> 面板：这里直接查看并编辑当前 Agent 工作区中的核心文件，避免人格、工具说明与长期记忆继续埋在大段 JSON 里。
                        </div>
                        <div className="grid grid-cols-1 xl:grid-cols-[300px,1fr] gap-4">
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-3">
                            <div className="flex items-start justify-between gap-3">
                              <div>
                                <h4 className="text-sm font-semibold text-gray-900 dark:text-white">核心文件列表（Core Files）</h4>
                                <p className="text-xs text-gray-500 mt-1 break-all">{selectedAgent.workspace || '当前 Agent 未配置 workspace。'}</p>
                              </div>
                              <button
                                onClick={() => {
                                  if (!confirmDiscardCoreFileDraft('并刷新核心文件列表')) return;
                                  loadAgentCoreFiles(selectedAgent.id, true);
                                }}
                                className="px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600"
                              >
                                刷新（Refresh）
                              </button>
                            </div>
                            {coreFilesLoading ? (
                              <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-8 text-center text-sm text-gray-400">
                                <RefreshCw size={16} className="animate-spin inline mr-2" />
                                正在读取工作区文件...
                              </div>
                            ) : selectedAgentCoreFiles.length === 0 ? (
                              <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-8 text-center text-sm text-gray-400">
                                当前 Agent 还没有可读取的核心文件；请先确认 workspace 已配置。
                              </div>
                            ) : (
                              <div className="space-y-2">
                                {selectedAgentCoreFiles.map(file => {
                                  const fileMeta = AGENT_CORE_FILE_META[file.name] || { label: file.name, description: '该文件用于补充当前 Agent 的工作说明。' };
                                  const active = selectedCoreFile?.name === file.name;
                                  return (
                                    <button
                                      key={file.name}
                                      onClick={() => {
                                        if (file.name === selectedCoreFileName) return;
                                        if (!confirmDiscardCoreFileDraft(`并切换到 ${file.name}`)) return;
                                        setSelectedCoreFileName(file.name);
                                      }}
                                      className={`w-full text-left rounded-xl border px-3 py-3 transition-colors ${
                                        active
                                          ? 'border-violet-300 bg-violet-50/80 dark:bg-violet-950/20 dark:border-violet-800/60'
                                          : 'border-gray-100 dark:border-gray-700 hover:border-violet-200 dark:hover:border-violet-800/60'
                                      }`}
                                    >
                                      <div className="flex items-start justify-between gap-2">
                                        <div>
                                          <div className="text-sm font-medium text-gray-900 dark:text-white">{fileMeta.label}</div>
                                          <p className="mt-1 text-[11px] text-gray-500">{fileMeta.description}</p>
                                        </div>
                                        <span className={`text-[10px] px-1.5 py-0.5 rounded-full ${file.exists ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200' : 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'}`}>
                                          {file.exists ? '已存在' : '缺失'}
                                        </span>
                                      </div>
                                      <div className="mt-2 text-[11px] text-gray-400">
                                        {file.exists ? `${formatFileSize(file.size)} · ${formatDateTime(file.modified)}` : '保存后会自动创建'}
                                      </div>
                                    </button>
                                  );
                                })}
                              </div>
                            )}
                          </div>

                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                            {selectedCoreFile ? (
                              <>
                                <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                                  <div>
                                    <h4 className="text-sm font-semibold text-gray-900 dark:text-white">
                                      {AGENT_CORE_FILE_META[selectedCoreFile.name]?.label || selectedCoreFile.name}
                                    </h4>
                                    <p className="text-xs text-gray-500 mt-1">
                                      {AGENT_CORE_FILE_META[selectedCoreFile.name]?.description || '编辑这个文件来补充当前 Agent 的长期上下文。'}
                                    </p>
                                    <div className="mt-2 flex flex-wrap gap-2 text-[11px] text-gray-400">
                                      <span className="font-mono">{selectedCoreFile.path}</span>
                                      <span>·</span>
                                      <span>{selectedCoreFile.exists ? `最近更新 ${formatDateTime(selectedCoreFile.modified)}` : '尚未创建'}</span>
                                    </div>
                                  </div>
                                  <button
                                    onClick={saveAgentCoreFile}
                                    disabled={coreFileSaving}
                                    className="px-3 py-2 text-xs rounded-lg bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50"
                                  >
                                    {coreFileSaving ? '保存中...' : `保存 ${selectedCoreFile.name}`}
                                  </button>
                                </div>
                                <textarea
                                  id="agent-core-file-content"
                                  name="agentCoreFileContent"
                                  aria-label={`${selectedCoreFile.name} 内容`}
                                  value={coreFileDraft}
                                  onChange={e => setCoreFileDraft(e.target.value)}
                                  rows={20}
                                  className="w-full min-h-[420px] px-3 py-3 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded-xl bg-gray-50 dark:bg-gray-900"
                                />
                              </>
                            ) : (
                              <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-16 text-center text-sm text-gray-400">
                                从左侧选择一个核心文件后，这里会显示它的内容与保存入口。
                              </div>
                            )}
                          </div>
                        </div>
                      </div>
                    )}

                    {detailTab === 'capabilities' && (
                      <div className="space-y-4">
                        <div className="rounded-xl border border-sky-100 bg-sky-50 px-4 py-3 text-sm text-sky-700 dark:border-sky-900/40 dark:bg-sky-950/20 dark:text-sky-200">
                          对标官方 <span className="font-mono">Skills / Channels / Cron Jobs</span> 面板：本区先提供当前 Agent 的能力快照与上下文状态。当前版本里的 Skills 仍继承全局配置，后续如支持 per-agent allowlist，这里会继续承接覆盖策略。
                        </div>
                        <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 bg-gray-50/80 dark:bg-gray-900">
                            <div className="text-xs text-gray-400">已启用技能（Enabled Skills）</div>
                            <div className="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{enabledSkills.length}</div>
                            <div className="mt-1 text-xs text-gray-500">共 {skills.length} 项，全局继承到当前 Agent。</div>
                          </div>
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 bg-gray-50/80 dark:bg-gray-900">
                            <div className="text-xs text-gray-400">通道快照（Channels Snapshot）</div>
                            <div className="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{selectedAgentChannelSnapshot.length}</div>
                            <div className="mt-1 text-xs text-gray-500">其中 {selectedAgentChannelSnapshot.filter(channel => channel.routeCount > 0).length} 个通道有显式规则指向它。</div>
                          </div>
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 bg-gray-50/80 dark:bg-gray-900">
                            <div className="text-xs text-gray-400">定时任务（Cron Jobs）</div>
                            <div className="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{selectedAgentCronJobs.length}</div>
                            <div className="mt-1 text-xs text-gray-500">以当前 Agent 作为 <span className="font-mono">sessionTarget</span> 的任务数量。</div>
                          </div>
                        </div>

                        <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-3">
                            <div>
                              <h4 className="text-sm font-semibold text-gray-900 dark:text-white">技能快照（Skills Snapshot）</h4>
                              <p className="text-xs text-gray-500 mt-1">先展示当前系统里最常用、且会被当前 Agent 继承到的 Skills。</p>
                            </div>
                            <div className="space-y-2">
                              {enabledSkills.slice(0, 5).map(skill => (
                                <div key={skill.id} className="rounded-lg border border-gray-100 dark:border-gray-700 px-3 py-3">
                                  <div className="flex items-center justify-between gap-2">
                                    <div className="text-sm font-medium text-gray-900 dark:text-white">{skill.name || skill.id}</div>
                                    <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200">
                                      已启用
                                    </span>
                                  </div>
                                  <p className="mt-1 text-[11px] text-gray-500 line-clamp-2">{skill.description || '暂无描述'}</p>
                                  <div className="mt-2 text-[11px] text-gray-400">{skill.source || 'global'}{skill.version ? ` · v${skill.version}` : ''}</div>
                                </div>
                              ))}
                              {enabledSkills.length === 0 && (
                                <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-6 text-sm text-gray-400 text-center">
                                  当前没有启用中的技能。
                                </div>
                              )}
                            </div>
                          </div>

                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-3">
                            <div>
                              <h4 className="text-sm font-semibold text-gray-900 dark:text-white">通道快照（Channels Snapshot）</h4>
                              <p className="text-xs text-gray-500 mt-1">从当前 Agent 视角查看相关通道、默认账号与显式路由规则数量。</p>
                            </div>
                            <div className="space-y-2">
                              {selectedAgentChannelSnapshot.map(channel => (
                                <div key={channel.id} className="rounded-lg border border-gray-100 dark:border-gray-700 px-3 py-3">
                                  <div className="flex items-center justify-between gap-2">
                                    <div className="text-sm font-medium text-gray-900 dark:text-white">{channel.label}</div>
                                    <span className={`text-[10px] px-1.5 py-0.5 rounded-full ${channel.enabled ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200' : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-200'}`}>
                                      {channel.enabled ? '启用中' : '未启用'}
                                    </span>
                                  </div>
                                  <div className="mt-2 grid grid-cols-2 gap-2 text-[11px] text-gray-500">
                                    <div>账号数：<span className="text-gray-700 dark:text-gray-200">{channel.configuredAccounts || 0}</span></div>
                                    <div>默认账号：<span className="text-gray-700 dark:text-gray-200">{channel.defaultAccount || '未设置'}</span></div>
                                    <div className="col-span-2">指向该 Agent 的规则：<span className="text-gray-700 dark:text-gray-200">{channel.routeCount || 0}</span></div>
                                  </div>
                                </div>
                              ))}
                              {selectedAgentChannelSnapshot.length === 0 && (
                                <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-6 text-sm text-gray-400 text-center">
                                  当前还没有配置任何通道。
                                </div>
                              )}
                            </div>
                          </div>

                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-3">
                            <div>
                              <h4 className="text-sm font-semibold text-gray-900 dark:text-white">定时任务（Cron Jobs）</h4>
                              <p className="text-xs text-gray-500 mt-1">展示当前以这个 Agent 为目标的任务，便于从单 Agent 视角做巡检。</p>
                            </div>
                            <div className="space-y-2">
                              {selectedAgentCronJobs.map(job => (
                                <div key={job.id} className="rounded-lg border border-gray-100 dark:border-gray-700 px-3 py-3">
                                  <div className="flex items-center justify-between gap-2">
                                    <div className="text-sm font-medium text-gray-900 dark:text-white">{job.name || job.id}</div>
                                    <span className={`text-[10px] px-1.5 py-0.5 rounded-full ${job.enabled ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200' : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-200'}`}>
                                      {job.enabled ? '运行中' : '已暂停'}
                                    </span>
                                  </div>
                                  <div className="mt-2 text-[11px] text-gray-500">{formatCronSchedule(job.schedule)}</div>
                                  <div className="mt-1 text-[11px] text-gray-400">最近执行：{formatDateTime(job.state?.lastRunAtMs || 0)}</div>
                                </div>
                              ))}
                              {selectedAgentCronJobs.length === 0 && (
                                <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-6 text-sm text-gray-400 text-center">
                                  暂无定时任务把消息投递给这个 Agent。
                                </div>
                              )}
                            </div>
                          </div>
                        </div>
                      </div>
                    )}

                    {detailTab === 'context' && (
                      <div className="space-y-4">
                        <div className="grid grid-cols-1 lg:grid-cols-3 gap-3">
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 bg-gray-50/80 dark:bg-gray-900">
                            <div className="text-xs text-gray-400">命中规则（Bindings）</div>
                            <div className="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{selectedRouteCount}</div>
                          </div>
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 bg-gray-50/80 dark:bg-gray-900">
                            <div className="text-xs text-gray-400">活跃会话（Active Sessions）</div>
                            <div className="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{selectedAgent.sessions ?? 0}</div>
                          </div>
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 bg-gray-50/80 dark:bg-gray-900">
                            <div className="text-xs text-gray-400">最后活跃（Last Active）</div>
                            <div className="mt-1 text-sm font-medium text-gray-900 dark:text-white">{formatLastActive(selectedAgent.lastActive)}</div>
                          </div>
                        </div>
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-3">
                          <div className="flex items-center justify-between gap-3">
                            <div>
                              <h4 className="text-sm font-semibold text-gray-900 dark:text-white">引用它的路由规则（Bindings Targeting This Agent）</h4>
                              <p className="text-xs text-gray-500 mt-1">这能帮助你快速理解“为什么消息会来到这个 Agent”。</p>
                            </div>
                            <button onClick={() => setWorkbenchView('routing')} className="px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600">
                              前往路由工作台（Open Routing Studio）
                            </button>
                          </div>
                          {selectedAgentBindings.length === 0 ? (
                            <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-6 text-sm text-gray-400 text-center">
                              暂无显式规则指向这个 Agent；未命中时会按默认 Agent 或其它规则兜底。
                            </div>
                          ) : (
                            selectedAgentBindings.map(({ binding, index }) => {
                              const summary = buildBindingSummary(binding.match, channelMeta);
                              const tags = buildBindingTags(binding.match);
                              return (
                                <div key={`${binding.agent}-${index}`} className="rounded-lg border border-gray-100 dark:border-gray-700 px-4 py-3">
                                  <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                                    <div>
                                      <div className="flex flex-wrap items-center gap-2">
                                        <span className="font-medium text-sm text-gray-900 dark:text-white">{binding.name.trim() || `规则 ${index + 1}`}</span>
                                        <span className="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-200">
                                          优先级：{humanizeBindingPriority(matchPriorityLabel(compactMatch(binding.match)))}
                                        </span>
                                        {binding.enabled === false && <span className="text-[10px] px-1.5 py-0.5 rounded bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200">已停用</span>}
                                      </div>
                                      <p className="mt-2 text-sm text-gray-600 dark:text-gray-300">{summary}</p>
                                      {tags.length > 0 && (
                                        <div className="mt-2 flex flex-wrap gap-1.5">
                                          {tags.map(tag => (
                                            <span key={`${binding.agent}-${index}-${tag}`} className="text-[10px] px-1.5 py-0.5 rounded-full bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-200">
                                              {tag}
                                            </span>
                                          ))}
                                        </div>
                                      )}
                                    </div>
                                    <button
                                      onClick={() => {
                                        setWorkbenchView('routing');
                                        setExpandedBindingIndex(index);
                                      }}
                                      className="px-3 py-2 text-xs rounded-lg bg-violet-600 text-white hover:bg-violet-700"
                                    >
                                      在工作台中编辑
                                    </button>
                                  </div>
                                </div>
                              );
                            })
                          )}
                        </div>
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-3">
                          <div className="flex items-center justify-between gap-3">
                            <div>
                              <h4 className="text-sm font-semibold text-gray-900 dark:text-white">最近会话（Recent Sessions）</h4>
                              <p className="text-xs text-gray-500 mt-1">基于现有 Sessions API 展示这个 Agent 最近活跃的会话，便于从单 Agent 视角快速巡检。</p>
                            </div>
                            <button
                              onClick={() => loadAgentSessions(selectedAgent.id, true)}
                              className="px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600"
                            >
                              刷新（Refresh）
                            </button>
                          </div>
                          {sessionsLoading && selectedAgentSessions.length === 0 ? (
                            <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-6 text-center text-sm text-gray-400">
                              <RefreshCw size={16} className="animate-spin inline mr-2" />
                              正在读取最近会话...
                            </div>
                          ) : selectedAgentSessions.length === 0 ? (
                            <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-6 text-sm text-gray-400 text-center">
                              当前没有可展示的会话，或会话仍由默认 Agent 接管。
                            </div>
                          ) : (
                            <div className="space-y-2">
                              {selectedAgentSessions.slice(0, 4).map(session => (
                                <div key={`${session.agentId || selectedAgent.id}-${session.sessionId}`} className="rounded-lg border border-gray-100 dark:border-gray-700 px-4 py-3">
                                  <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                                    <div>
                                      <div className="text-sm font-medium text-gray-900 dark:text-white">{session.originLabel || session.key || session.sessionId}</div>
                                      <div className="mt-1 text-[11px] text-gray-500">{session.lastChannel || '未知通道'} · {session.messageCount || 0} 条消息</div>
                                    </div>
                                    <div className="text-[11px] text-gray-400">{formatLastActive(session.updatedAt)}</div>
                                  </div>
                                </div>
                              ))}
                            </div>
                          )}
                        </div>
                      </div>
                    )}

                    {detailTab === 'advanced' && (
                      <div className="space-y-4">
                        <div className="rounded-xl border border-amber-100 bg-amber-50 px-4 py-4 text-sm text-amber-700 dark:border-amber-900/40 dark:bg-amber-950/20 dark:text-amber-200">
                          结构化表单优先覆盖常用配置；只有当你需要 provider 专属字段或复杂对象时，再进入 Advanced JSON。
                        </div>
                        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4">
                            <div className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                              <Sparkles size={15} className="text-violet-500" />
                              当前已使用的高级块（Advanced Blocks）
                            </div>
                            <div className="mt-3 flex flex-wrap gap-2">
                              {selectedAdvancedBlocks.length > 0 ? selectedAdvancedBlocks.map(block => (
                                <span key={block} className="text-[11px] px-2.5 py-1 rounded-full bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-200">
                                  {block}
                                </span>
                              )) : (
                                <span className="text-sm text-gray-400">当前没有额外高级块。</span>
                              )}
                            </div>
                          </div>
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4">
                            <div className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                              <FileText size={15} className="text-violet-500" />
                              下一步建议（Next Step）
                            </div>
                            <ul className="mt-3 list-disc pl-4 space-y-1.5 text-sm text-gray-600 dark:text-gray-300">
                              <li>先在结构化表单里完成常见字段，再用 JSON 补充 provider 专属项。</li>
                              <li>如果某个 Agent 长期依赖 JSON，可考虑抽出成模板或说明文档。</li>
                              <li>保存前会保留未 touched 的高级块，避免静默丢值。</li>
                            </ul>
                          </div>
                        </div>
                        <button onClick={() => openEdit(selectedAgent, 'advanced')} className="px-3 py-2 text-xs rounded-lg bg-violet-600 text-white hover:bg-violet-700">
                          打开高级 JSON（Open Advanced JSON）
                        </button>
                      </div>
                    )}
                  </div>
                </div>
              </>
            ) : (
              <div className="rounded-xl border border-dashed border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 px-6 py-16 text-center text-sm text-gray-400">
                选择左侧 Agent 后，这里会显示它的配置摘要与上下文。
              </div>
            )}
          </div>
        </div>
      ) : (
        <div className="space-y-6">
          <div className="grid grid-cols-1 lg:grid-cols-4 gap-4">
            <div className="rounded-xl border border-gray-100 dark:border-gray-700 bg-white dark:bg-gray-800 p-4 shadow-sm">
              <div className="text-xs text-gray-400">规则总数</div>
              <div className="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{routingStats.total}</div>
            </div>
            <div className="rounded-xl border border-gray-100 dark:border-gray-700 bg-white dark:bg-gray-800 p-4 shadow-sm">
              <div className="text-xs text-gray-400">已启用规则（Enabled）</div>
              <div className="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{routingStats.enabled}</div>
            </div>
            <div className="rounded-xl border border-gray-100 dark:border-gray-700 bg-white dark:bg-gray-800 p-4 shadow-sm">
              <div className="text-xs text-gray-400">被路由引用的 Agent</div>
              <div className="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{routingStats.agents}</div>
            </div>
            <div className="rounded-xl border border-gray-100 dark:border-gray-700 bg-white dark:bg-gray-800 p-4 shadow-sm">
              <div className="text-xs text-gray-400">涉及通道（Channels）</div>
              <div className="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{routingStats.channels}</div>
            </div>
          </div>

          <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-100 dark:border-gray-700/50 shadow-sm">
            <div className="px-4 py-4 border-b border-gray-100 dark:border-gray-700/50 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h3 className="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                  <Settings size={15} className="text-violet-500" />
                  路由规则（Bindings）
                </h3>
                <p className="text-xs text-gray-500 mt-1">先写“这类消息交给谁”，必要时再展开底层 JSON。对普通用户优先显示自然语言摘要。</p>
              </div>
              <div className="flex items-center gap-2">
                <button onClick={addBinding} className="px-2.5 py-1.5 text-xs rounded bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600">
                  新增规则
                </button>
                <button onClick={saveBindings} disabled={saving} className="flex items-center gap-1 px-3 py-1.5 text-xs rounded bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50">
                  <Save size={12} /> 保存规则
                </button>
              </div>
            </div>
            <div className="px-4 pt-4">
                <div className="rounded-xl border border-violet-100 bg-violet-50 px-4 py-3 text-xs text-violet-700 dark:border-violet-900/40 dark:bg-violet-950/20 dark:text-violet-200">
                  固定优先级为 sender &gt; peer &gt; parentPeer &gt; guildId+roles &gt; guildId &gt; teamId &gt; accountId &gt; accountId:* &gt; channel；只有同优先级规则才按列表从上到下比较。未命中时会回落到默认 Agent「{defaultAgent}」。
                </div>
            </div>
            <div className="p-4 space-y-3">
              {bindings.length === 0 && (
                <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-8 text-center text-sm text-gray-400">
                  还没有显式路由规则，当前所有消息都会落到默认 Agent。
                </div>
              )}

              {bindings.map((row, idx) => {
                const match = compactMatch(row.match);
                const channel = extractTextValue(match.channel);
                const accountId = extractTextValue(match.accountId);
                const peer = extractPeerForm(match, 'peer');
                const parentPeer = extractPeerForm(match, 'parentPeer');
                const channelCfg = channel ? channelMeta[channel] : undefined;
                const defaultAccount = channelCfg?.defaultAccount;
                const accounts = channelCfg?.accounts || [];
                const priority = matchPriorityLabel(match);
                const expanded = expandedBindingIndex === idx;
                const tags = buildBindingTags(match);

                return (
                  <div key={idx} className="rounded-xl border border-gray-100 dark:border-gray-700 bg-gray-50/70 dark:bg-gray-900/40">
                    <div className="px-4 py-4 flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                      <div className="flex-1 min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <button
                            onClick={() => setExpandedBindingIndex(prev => (prev === idx ? null : idx))}
                            className="inline-flex items-center justify-center p-1 rounded hover:bg-white dark:hover:bg-gray-800"
                            aria-label={expanded ? '折叠规则' : '展开规则'}
                          >
                            {expanded ? <ChevronDown size={15} /> : <ChevronRight size={15} />}
                          </button>
                          <span className="font-medium text-sm text-gray-900 dark:text-white">{row.name.trim() || `规则 ${idx + 1}`}</span>
                          <span className="text-[10px] px-1.5 py-0.5 rounded bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-200">
                            → {row.agent}
                          </span>
                          <span className="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-200">
                            优先级：{humanizeBindingPriority(priority)}
                          </span>
                          {row.enabled === false && <span className="text-[10px] px-1.5 py-0.5 rounded bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200">已停用</span>}
                        </div>
                        <p className="mt-2 text-sm text-gray-600 dark:text-gray-300">{buildBindingSummary(match, channelMeta)}</p>
                        {tags.length > 0 && (
                          <div className="mt-2 flex flex-wrap gap-1.5">
                            {tags.map(tag => (
                              <span key={`${idx}-${tag}`} className="text-[10px] px-1.5 py-0.5 rounded-full bg-white dark:bg-gray-800 text-gray-600 dark:text-gray-200 border border-gray-200 dark:border-gray-700">
                                {tag}
                              </span>
                            ))}
                          </div>
                        )}
                      </div>
                      <div className="flex items-center gap-1">
                        <button onClick={() => moveBinding(idx, -1)} className="p-1.5 rounded hover:bg-white dark:hover:bg-gray-800" title="上移"><ArrowUp size={13} /></button>
                        <button onClick={() => moveBinding(idx, 1)} className="p-1.5 rounded hover:bg-white dark:hover:bg-gray-800" title="下移"><ArrowDown size={13} /></button>
                        <button onClick={() => removeBinding(idx)} className="p-1.5 rounded text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20" title="删除"><Trash2 size={13} /></button>
                      </div>
                    </div>

                    {expanded && (
                      <div className="border-t border-gray-100 dark:border-gray-700 px-4 py-4 space-y-4">
                        <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                          <div>
                            <label htmlFor={`binding-${idx}-name`} className="text-[11px] text-gray-500">规则名</label>
                            <input
                              id={`binding-${idx}-name`}
                              name={`binding-${idx}-name`}
                              aria-label="规则名"
                              value={row.name}
                              onChange={e => setBindingAt(idx, r => ({ ...r, name: e.target.value }))}
                              placeholder="例如 Discord 维护群"
                              className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                            />
                          </div>
                          <div>
                            <label htmlFor={`binding-${idx}-agent`} className="text-[11px] text-gray-500">目标 Agent</label>
                            <select
                              id={`binding-${idx}-agent`}
                              name={`binding-${idx}-agent`}
                              aria-label="目标 Agent"
                              value={row.agent}
                              onChange={e => setBindingAt(idx, r => ({ ...r, agent: e.target.value }))}
                              className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                            >
                              {(agentOptions.length ? agentOptions : ['main']).map(id => (
                                <option key={id} value={id}>{id}</option>
                              ))}
                            </select>
                          </div>
                          <div className="flex items-end">
                            <label className="inline-flex items-center gap-2 text-xs text-gray-600 dark:text-gray-300">
                              <input
                                type="checkbox"
                                name={`binding-${idx}-enabled`}
                                aria-label="启用这条规则"
                                checked={row.enabled}
                                onChange={e => setBindingAt(idx, r => ({ ...r, enabled: e.target.checked }))}
                              />
                              启用这条规则
                            </label>
                          </div>
                        </div>

                        <div className="rounded-xl border border-sky-100 bg-sky-50 px-4 py-3 text-[12px] text-sky-700 dark:border-sky-900/40 dark:bg-sky-950/20 dark:text-sky-200">
                          结构化模式适合大多数人：直接描述“通道 / 账号 / 会话 / 角色”。只有需要复杂表达式时，再切到 JSON 高级模式。
                        </div>

                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => switchBindingMode(idx, 'structured')}
                            className={`px-2 py-1 text-[11px] rounded border ${row.mode === 'structured' ? 'bg-violet-600 text-white border-violet-600' : 'bg-white dark:bg-gray-900 text-gray-600 border-gray-200 dark:border-gray-700'}`}
                          >
                            结构化
                          </button>
                          <button
                            onClick={() => switchBindingMode(idx, 'json')}
                            className={`px-2 py-1 text-[11px] rounded border ${row.mode === 'json' ? 'bg-violet-600 text-white border-violet-600' : 'bg-white dark:bg-gray-900 text-gray-600 border-gray-200 dark:border-gray-700'}`}
                          >
                            JSON 高级模式
                          </button>
                          <span className="text-[11px] text-gray-400">accountId 留空 = 仅匹配默认账号</span>
                        </div>

                        {row.mode === 'structured' ? (
                          <div className="space-y-4">
                            <div className="rounded-lg border border-gray-100 dark:border-gray-700 p-3">
                              <h4 className="text-[11px] font-semibold text-gray-900 dark:text-white uppercase tracking-wide">消息来源</h4>
                              <div className="mt-3 grid grid-cols-1 md:grid-cols-3 gap-3">
                                <div>
                                  <label htmlFor={`binding-${idx}-channel`} className="text-[11px] text-gray-500">通道 *</label>
                                  <input
                                    list="agent-channel-options"
                                    id={`binding-${idx}-channel`}
                                    name={`binding-${idx}-channel`}
                                    aria-label="通道"
                                    value={channel}
                                    onChange={e => touchBindingMatch(idx, cur => ({ ...cur, channel: e.target.value }))}
                                    placeholder="qq / discord / feishu"
                                    className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                                  />
                                </div>
                                <div>
                                  <label htmlFor={`binding-${idx}-accountId`} className="text-[11px] text-gray-500">账号</label>
                                  {accounts.length > 0 ? (
                                    <select
                                      id={`binding-${idx}-accountId`}
                                      name={`binding-${idx}-accountId`}
                                      aria-label="账号"
                                      value={accountId}
                                      onChange={e => touchBindingMatch(idx, cur => ({ ...cur, accountId: e.target.value || undefined }))}
                                      className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                                    >
                                      <option value="">(默认账号)</option>
                                      <option value="*">*（全部账号）</option>
                                      {accounts.map(acc => <option key={acc} value={acc}>{acc}</option>)}
                                      {accountId && accountId !== '*' && !accounts.includes(accountId) && (
                                        <option value={accountId}>{accountId} (custom)</option>
                                      )}
                                    </select>
                                  ) : (
                                    <input
                                      id={`binding-${idx}-accountId`}
                                      name={`binding-${idx}-accountId`}
                                      aria-label="账号"
                                      value={accountId}
                                      onChange={e => touchBindingMatch(idx, cur => ({ ...cur, accountId: e.target.value }))}
                                      placeholder="留空=默认账号，*=全部账号"
                                      className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                                    />
                                  )}
                                  <p className="mt-1 text-[10px] text-gray-500">
                                    {!accountId
                                      ? `当前留空，仅匹配默认账号${defaultAccount ? ` (${defaultAccount})` : ''}`
                                      : accountId === '*'
                                        ? '匹配该 channel 的所有账号（常用于兜底）'
                                        : `仅匹配账号 ${accountId}`}
                                  </p>
                                </div>
                                <div>
                                  <label htmlFor={`binding-${idx}-sender`} className="text-[11px] text-gray-500">发送者</label>
                                  <input
                                    id={`binding-${idx}-sender`}
                                    name={`binding-${idx}-sender`}
                                    aria-label="发送者"
                                    value={extractTextValue(match.sender)}
                                    onChange={e => touchBindingMatch(idx, cur => ({ ...cur, sender: e.target.value }))}
                                    placeholder="例如 +15551230001"
                                    className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                                  />
                                </div>
                              </div>
                            </div>

                            <div className="rounded-lg border border-gray-100 dark:border-gray-700 p-3">
                              <h4 className="text-[11px] font-semibold text-gray-900 dark:text-white uppercase tracking-wide">会话范围</h4>
                              <div className="mt-3 grid grid-cols-1 md:grid-cols-4 gap-3">
                                <div>
                                  <label htmlFor={`binding-${idx}-peer-kind`} className="text-[11px] text-gray-500">peer.kind</label>
                                  <input
                                    id={`binding-${idx}-peer-kind`}
                                    name={`binding-${idx}-peer-kind`}
                                    aria-label="peer kind"
                                    value={peer.kind}
                                    onChange={e => setPeerField(idx, 'peer', 'kind', e.target.value)}
                                    placeholder="direct / group"
                                    className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                                  />
                                </div>
                                <div>
                                  <label htmlFor={`binding-${idx}-peer-id`} className="text-[11px] text-gray-500">peer.id</label>
                                  <input
                                    id={`binding-${idx}-peer-id`}
                                    name={`binding-${idx}-peer-id`}
                                    aria-label="peer id"
                                    value={peer.id}
                                    onChange={e => setPeerField(idx, 'peer', 'id', e.target.value)}
                                    placeholder="+1555... / group-01"
                                    className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                                  />
                                </div>
                                <div>
                                  <label htmlFor={`binding-${idx}-parent-peer-kind`} className="text-[11px] text-gray-500">parentPeer.kind</label>
                                  <input
                                    id={`binding-${idx}-parent-peer-kind`}
                                    name={`binding-${idx}-parent-peer-kind`}
                                    aria-label="parent peer kind"
                                    value={parentPeer.kind}
                                    onChange={e => setPeerField(idx, 'parentPeer', 'kind', e.target.value)}
                                    placeholder="thread / group"
                                    className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                                  />
                                </div>
                                <div>
                                  <label htmlFor={`binding-${idx}-parent-peer-id`} className="text-[11px] text-gray-500">parentPeer.id</label>
                                  <input
                                    id={`binding-${idx}-parent-peer-id`}
                                    name={`binding-${idx}-parent-peer-id`}
                                    aria-label="parent peer id"
                                    value={parentPeer.id}
                                    onChange={e => setPeerField(idx, 'parentPeer', 'id', e.target.value)}
                                    placeholder="上级会话 id"
                                    className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                                  />
                                </div>
                              </div>
                            </div>

                            <div className="rounded-lg border border-gray-100 dark:border-gray-700 p-3">
                              <h4 className="text-[11px] font-semibold text-gray-900 dark:text-white uppercase tracking-wide">渠道专属条件</h4>
                              <div className="mt-3 grid grid-cols-1 md:grid-cols-3 gap-3">
                                <div>
                                  <label htmlFor={`binding-${idx}-guildId`} className="text-[11px] text-gray-500">guildId</label>
                                  <input
                                    id={`binding-${idx}-guildId`}
                                    name={`binding-${idx}-guildId`}
                                    aria-label="guildId"
                                    value={extractTextValue(match.guildId)}
                                    onChange={e => touchBindingMatch(idx, cur => ({ ...cur, guildId: e.target.value }))}
                                    placeholder="Discord guild id"
                                    className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                                  />
                                </div>
                                <div>
                                  <label htmlFor={`binding-${idx}-roles`} className="text-[11px] text-gray-500">roles（逗号分隔）</label>
                                  <input
                                    id={`binding-${idx}-roles`}
                                    name={`binding-${idx}-roles`}
                                    aria-label="roles"
                                    value={extractRolesText(match)}
                                    onChange={e => touchBindingMatch(idx, cur => ({ ...cur, roles: parseCSV(e.target.value) }))}
                                    placeholder="admin, maintainer"
                                    className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                                  />
                                </div>
                                <div>
                                  <label htmlFor={`binding-${idx}-teamId`} className="text-[11px] text-gray-500">teamId</label>
                                  <input
                                    id={`binding-${idx}-teamId`}
                                    name={`binding-${idx}-teamId`}
                                    aria-label="teamId"
                                    value={extractTextValue(match.teamId)}
                                    onChange={e => touchBindingMatch(idx, cur => ({ ...cur, teamId: e.target.value }))}
                                    placeholder="Slack team id"
                                    className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                                  />
                                </div>
                              </div>
                            </div>
                          </div>
                        ) : (
                          <div className="space-y-2">
                            <div className="rounded-lg border border-amber-100 bg-amber-50 px-3 py-2 text-[11px] text-amber-700 dark:border-amber-900/40 dark:bg-amber-950/20 dark:text-amber-200">
                              这里只有一个原则：只在结构化表单表达不了时才写 JSON，例如数组、复杂嵌套或未来字段。
                            </div>
                            <textarea
                              name={`binding-${idx}-match-json`}
                              aria-label="路由规则 JSON"
                              value={row.matchText}
                              onChange={e => setBindingAt(idx, r => ({ ...r, matchText: e.target.value, rowError: '' }))}
                              rows={7}
                              className="w-full font-mono text-xs px-2 py-2 border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                            />
                          </div>
                        )}

                        {row.rowError && (
                          <div className="text-xs text-red-600 bg-red-50 dark:bg-red-900/20 rounded px-2 py-1.5">
                            {row.rowError}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </div>

          <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-100 dark:border-gray-700/50 shadow-sm">
            <div className="px-4 py-4 border-b border-gray-100 dark:border-gray-700/50 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h3 className="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                  <Route size={15} className="text-violet-500" />
                  消息分流模拟（Routing Simulator）
                </h3>
                <p className="text-xs text-gray-500 mt-1">把消息场景描述清楚，然后看系统最终会把它交给谁。</p>
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setPreviewMeta(DEFAULT_PREVIEW_META)}
                  className="px-3 py-1.5 text-xs rounded bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600"
                >
                  清空输入
                </button>
                <button onClick={runPreview} disabled={previewLoading} className="px-3 py-1.5 text-xs rounded bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50">
                  {previewLoading ? '模拟中...' : '执行模拟'}
                </button>
              </div>
            </div>
            <div className="p-4 space-y-4">
              <div className="rounded-xl border border-sky-100 bg-sky-50 px-4 py-3 text-xs text-sky-700 dark:border-sky-900/40 dark:bg-sky-950/20 dark:text-sky-200">
                普通使用流程：先填“通道 / 账号 / 发送者”，再按需补充群聊、线程、Discord/Slack 相关条件。
              </div>
              {PREVIEW_GROUPS.map(group => (
                <div key={group.title} className="rounded-xl border border-gray-100 dark:border-gray-700 p-4">
                  <div>
                    <h4 className="text-sm font-semibold text-gray-900 dark:text-white">{group.title}</h4>
                    <p className="text-xs text-gray-500 mt-1">{group.description}</p>
                  </div>
                  <div className="mt-3 grid grid-cols-1 md:grid-cols-3 gap-3">
                    {group.fields.map(field => {
                      const fieldMeta = PREVIEW_FIELD_META[field];
                      return (
                        <div key={field}>
                          <label htmlFor={`preview-${field}`} className="text-xs text-gray-500">{fieldMeta.label}</label>
                          <input
                            list={fieldMeta.listId}
                            id={`preview-${field}`}
                            name={`preview-${field}`}
                            aria-label={fieldMeta.label}
                            value={previewMeta[field] || ''}
                            onChange={e => setPreviewMeta(prev => ({ ...prev, [field]: e.target.value }))}
                            placeholder={fieldMeta.placeholder}
                            className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                          />
                          <p className="mt-1 text-[10px] text-gray-500">{fieldMeta.help}</p>
                        </div>
                      );
                    })}
                  </div>
                </div>
              ))}
            </div>
            {previewResult && (
              <div className="px-4 pb-4">
                <div className="rounded-xl border border-gray-100 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 p-4 space-y-4">
                  <div className="grid grid-cols-1 lg:grid-cols-3 gap-3">
                    <div className="rounded-lg bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 px-3 py-3">
                      <div className="text-xs text-gray-400">命中 Agent（Resolved Agent）</div>
                      <div className="mt-1 font-mono text-violet-600 dark:text-violet-300">{previewResult.agent || defaultAgent || '-'}</div>
                    </div>
                    <div className="rounded-lg bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 px-3 py-3">
                      <div className="text-xs text-gray-400">命中规则（Matched Rule）</div>
                      <div className="mt-1 text-gray-900 dark:text-white">{previewExplanation?.ruleLabel || previewResult.matchedBy || '默认回落'}</div>
                    </div>
                    <div className="rounded-lg bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 px-3 py-3">
                      <div className="text-xs text-gray-400">技术命中点（Trace Key）</div>
                      <div className="mt-1 font-mono text-gray-700 dark:text-gray-200">{previewResult.matchedBy || 'default'}</div>
                    </div>
                  </div>

                  {previewExplanation && (
                    <div className="rounded-xl border border-violet-100 bg-violet-50 px-4 py-3 text-sm text-violet-700 dark:border-violet-900/40 dark:bg-violet-950/20 dark:text-violet-200">
                      <div className="font-medium">{previewExplanation.headline}</div>
                      <div className="mt-1">{previewExplanation.detail}</div>
                    </div>
                  )}

                  <details className="rounded-lg border border-gray-100 dark:border-gray-700 bg-white dark:bg-gray-800">
                    <summary className="cursor-pointer list-none px-4 py-3 text-sm font-medium text-gray-900 dark:text-white flex items-center justify-between">
                      技术细节（Trace）
                      <span className="text-xs text-gray-400">展开查看完整判断路径</span>
                    </summary>
                    <div className="px-4 pb-4 text-xs">
                      {(previewResult.trace || []).length > 0 ? (
                        <ul className="list-disc pl-4 space-y-1 text-gray-600 dark:text-gray-300">
                          {(previewResult.trace || []).map((line, i) => (
                            <li key={i} className="font-mono break-all">{line}</li>
                          ))}
                        </ul>
                      ) : (
                        <div className="text-gray-400">没有额外 trace。</div>
                      )}
                    </div>
                  </details>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {showForm && (
        <div className="fixed inset-0 z-[220] bg-black/40 flex items-center justify-center p-4">
          <div className="w-full max-w-5xl max-h-[92vh] overflow-hidden rounded-xl bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 shadow-xl flex flex-col">
            <div className="sticky top-0 z-10 border-b border-gray-100 dark:border-gray-700 bg-white/95 dark:bg-gray-800/95 backdrop-blur">
              <div className="px-5 py-4 flex items-start justify-between gap-4">
                <div>
                  <h3 className="font-semibold text-gray-900 dark:text-white">{editingId ? `编辑智能体（Edit Agent）: ${editingId}` : '新建智能体（New Agent）'}</h3>
                  <p className="text-xs text-gray-500 mt-1">
                    先完成基础信息（Basic），再按需进入其它部分。高级 JSON（Advanced）会保留完整覆盖能力。
                  </p>
                </div>
                  <button onClick={closeForm} className="text-xs px-2.5 py-1.5 rounded bg-gray-100 dark:bg-gray-700 shrink-0">关闭（Close）</button>
              </div>
              <div className="px-5 pb-4 space-y-3">
                <div className="flex flex-wrap gap-2">
                  {AGENT_FORM_SECTIONS.map(section => (
                    <button
                      key={section.id}
                      onClick={() => setFormSection(section.id)}
                      className={`px-3 py-2 rounded-lg text-xs border transition-colors ${formSection === section.id ? 'bg-violet-600 text-white border-violet-600' : 'bg-white dark:bg-gray-900 text-gray-600 dark:text-gray-300 border-gray-200 dark:border-gray-700 hover:border-violet-300'}`}
                    >
                      <div className="font-medium">{section.title}</div>
                      <div className={`mt-0.5 ${formSection === section.id ? 'text-violet-100' : 'text-gray-400'}`}>{section.description}</div>
                    </button>
                  ))}
                </div>
                <div className="grid grid-cols-1 md:grid-cols-4 gap-2 text-[11px]">
                  <div className="rounded-lg bg-gray-50 dark:bg-gray-900 border border-gray-100 dark:border-gray-700 px-3 py-2">
                      <div className="text-gray-400">ID</div>
                    <div className="font-mono text-gray-700 dark:text-gray-200 mt-1">{form.id.trim() || '未设置'}</div>
                  </div>
                  <div className="rounded-lg bg-gray-50 dark:bg-gray-900 border border-gray-100 dark:border-gray-700 px-3 py-2">
                      <div className="text-gray-400">主模型（Primary Model）</div>
                    <div className="text-gray-700 dark:text-gray-200 mt-1 truncate">{form.modelPrimary.trim() || '继承 / Advanced JSON'}</div>
                  </div>
                  <div className="rounded-lg bg-gray-50 dark:bg-gray-900 border border-gray-100 dark:border-gray-700 px-3 py-2">
                      <div className="text-gray-400">工作区（Workspace）</div>
                    <div className="font-mono text-gray-700 dark:text-gray-200 mt-1 truncate">{form.workspace.trim() || '未设置'}</div>
                  </div>
                  <div className="rounded-lg bg-gray-50 dark:bg-gray-900 border border-gray-100 dark:border-gray-700 px-3 py-2">
                      <div className="text-gray-400">默认接管（Default）</div>
                    <div className="text-gray-700 dark:text-gray-200 mt-1">{effectiveIsDefault ? '是' : '否'}</div>
                  </div>
                </div>
              </div>
            </div>

            <div className="flex-1 overflow-y-auto px-5 py-5">
              {formSection === 'basic' && (
                <div className="space-y-5">
                  {implicitMainOnly && (
                    <div className="rounded-xl border border-sky-100 bg-sky-50 px-4 py-3 text-[12px] text-sky-700 space-y-1.5">
                    <div className="font-medium">当前列表里的 main 更像运行时占位，而不是已经写入配置的显式 Agent。</div>
                    <div>如果你直接新建首个显式 Agent，它会接管默认路由，并自动成为默认 Agent。</div>
                    <div>如果你想把 <span className="font-mono">main</span> 真正写进配置，请改用列表里的“编辑 main”。</div>
                  </div>
                  )}

                  {materializingImplicitAgent && (
                    <div className="rounded-xl border border-sky-100 bg-sky-50 px-4 py-3 text-[12px] text-sky-700 space-y-1.5">
                      <div className="font-medium">你正在把运行时发现的 <span className="font-mono">{form.id.trim() || editingId || 'agent'}</span> 显式写入配置。</div>
                      <div>保存后，它会真正进入 <span className="font-mono">agents.list</span>，后续即可像普通 Agent 一样继续编辑。</div>
                      {form.id.trim() === defaultAgent && (
                        <div>当前它也是默认 Agent 的运行时来源，保存后默认关系会一起落盘。</div>
                      )}
                    </div>
                  )}

                  <div className="rounded-xl border border-amber-100 bg-amber-50 px-4 py-3 text-[12px] text-amber-700 space-y-1.5">
                    <div><span className="font-medium">workspace</span> 只是默认工作目录，方便文件与上下文定位；它不是硬隔离边界。</div>
                    <div>如果需要更严格的执行限制，请在 <span className="font-medium">Access &amp; Safety</span> 里设置 <span className="font-mono">sandbox</span> 覆盖。</div>
                    <div>当前 Panel 还会把 <span className="font-mono">workspace</span> / <span className="font-mono">agentDir</span> 约束在 OpenClaw 受管目录内；同一个 <span className="font-mono">agentDir</span> 不要复用，<span className="font-mono">workspace</span> 目前也不能与其它 Agent 重复。</div>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div>
                      <label className="text-xs text-gray-500">ID *</label>
                      <input
                        value={form.id}
                        disabled={!!editingId}
                        onChange={e => updateForm({ id: e.target.value })}
                        placeholder="例如 main / work / ops_helper"
                        className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 disabled:opacity-60"
                      />
                      <p className="mt-1 text-[11px] text-gray-400">仅支持字母、数字、下划线和中划线。编辑现有 Agent 时不可修改。</p>
                      {showAgentIDError && agentIDError && (
                        <p className="mt-1 text-[11px] text-red-600">{agentIDError}</p>
                      )}
                    </div>
                    <div>
                        <label className="text-xs text-gray-500">显示名称（Name）</label>
                      <input
                        value={form.name}
                        onChange={e => updateForm({ name: e.target.value })}
                        placeholder="面向业务人员展示的名称"
                        className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                      />
                    </div>
                    <div>
                        <label className="text-xs text-gray-500">工作区目录（Workspace）</label>
                      <input
                        value={form.workspace}
                        onChange={e => updateForm({ workspace: e.target.value })}
                        placeholder="workspaces/support"
                        className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                      />
                      <p className="mt-1 text-[11px] text-gray-400">用于默认文件读写位置，不代表执行隔离；当前 Panel 只接受 OpenClaw 受管目录内的路径。</p>
                      {workspaceConflict && (
                        <p className="mt-1 text-[11px] text-red-600">该 workspace 已被 Agent “{workspaceConflict.id}” 使用。</p>
                      )}
                    </div>
                    <div>
                        <label className="text-xs text-gray-500">Agent 目录（AgentDir）</label>
                      <input
                        value={form.agentDir}
                        onChange={e => updateForm({ agentDir: e.target.value })}
                        placeholder="agents/support"
                        className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                      />
                      <p className="mt-1 text-[11px] text-gray-400">建议每个 Agent 使用独立目录，避免 auth / session 文件冲突。</p>
                      {agentDirConflict && (
                        <p className="mt-1 text-[11px] text-red-600">该 agentDir 已被 Agent “{agentDirConflict.id}” 使用。</p>
                      )}
                    </div>
                  </div>

                  <label className="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
                    <input
                      type="checkbox"
                      checked={effectiveIsDefault}
                      disabled={firstExplicitAgentWillBecomeDefault}
                      onChange={e => updateForm({ isDefault: e.target.checked })}
                    />
                    设为默认 Agent（Default Fallback，未命中 bindings 时优先接管）
                  </label>
                  {firstExplicitAgentWillBecomeDefault && (
                    <p className="text-[11px] text-gray-500">
                      当前还没有显式 Agent，本次保存后它会自动成为默认 Agent。
                    </p>
                  )}
                </div>
              )}

              {formSection === 'behavior' && (
                <div className="space-y-5">
                  <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
                    <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                      <div>
                        <h4 className="text-sm font-semibold text-gray-900 dark:text-white">模型选择（Model）</h4>
                        <p className="text-xs text-gray-500 mt-1">优先把常见选择做成结构化输入；更复杂的模型对象可在 Advanced 中继续编辑。</p>
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">主模型（Primary Model）</label>
                        <input
                          list="agent-model-options"
                          value={form.modelPrimary}
                          onChange={e => updateForm({ modelPrimary: e.target.value }, 'model')}
                          placeholder="例如 deepseek/deepseek-chat"
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                        {(defaultModelHint || quickModelOptions.length > 0) && (
                          <div className="mt-2 space-y-2">
                            {defaultModelHint && (
                              <p className="text-[11px] text-gray-400">
                                当前默认模型：<span className="font-mono text-violet-600 dark:text-violet-400">{defaultModelHint}</span>
                              </p>
                            )}
                            {quickModelOptions.length > 0 && (
                              <div className="flex flex-wrap gap-2">
                                {quickModelOptions.map(model => (
                                  <button
                                    key={model}
                                    type="button"
                                    onClick={() => updateForm({ modelPrimary: model }, 'model')}
                                    className={`px-2.5 py-1 rounded-full text-[11px] border transition-colors ${
                                      form.modelPrimary.trim() === model
                                        ? 'bg-violet-600 text-white border-violet-600'
                                        : 'bg-white dark:bg-gray-900 text-gray-600 dark:text-gray-300 border-gray-200 dark:border-gray-700 hover:border-violet-300'
                                    }`}
                                  >
                                    {model === defaultModelHint ? `${model}（默认）` : model}
                                  </button>
                                ))}
                              </div>
                            )}
                          </div>
                        )}
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">回退模型（Fallback Models）</label>
                        <input
                          value={form.modelFallbacks}
                          onChange={e => updateForm({ modelFallbacks: e.target.value }, 'model')}
                          placeholder="逗号分隔，例如 openai/gpt-4o-mini, deepseek/deepseek-chat"
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                        <p className="mt-1 text-[11px] text-gray-400">保存时会写回 <span className="font-mono">model.primary</span> 与 <span className="font-mono">model.fallbacks</span>。</p>
                      </div>
                    </div>

                    <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                      <div>
                        <h4 className="text-sm font-semibold text-gray-900 dark:text-white">常用参数（Common Params）</h4>
                        <p className="text-xs text-gray-500 mt-1">这些参数会覆盖模型默认值；如需 provider 专属参数，请转到 Advanced / params JSON。</p>
                      </div>
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                        <div>
                          <label className="text-xs text-gray-500">温度（temperature）</label>
                          <input
                            value={form.paramTemperature}
                            onChange={e => updateForm({ paramTemperature: e.target.value }, 'params')}
                            placeholder="例如 0.7"
                            className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                          />
                        </div>
                        <div>
                          <label className="text-xs text-gray-500">采样范围（topP）</label>
                          <input
                            value={form.paramTopP}
                            onChange={e => updateForm({ paramTopP: e.target.value }, 'params')}
                            placeholder="例如 0.9"
                            className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                          />
                        </div>
                        <div>
                          <label className="text-xs text-gray-500">最大输出（maxTokens）</label>
                          <input
                            value={form.paramMaxTokens}
                            onChange={e => updateForm({ paramMaxTokens: e.target.value }, 'params')}
                            placeholder="例如 2048"
                            className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                          />
                        </div>
                      </div>
                    </div>

                    <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                      <div>
                        <h4 className="text-sm font-semibold text-gray-900 dark:text-white">上下文预算（Context Budget）</h4>
                        <p className="text-xs text-gray-500 mt-1">
                          这里覆盖 <span className="font-mono">agents.defaults.contextTokens / compaction</span>；实际运行时仍会与模型真实 <span className="font-mono">contextWindow</span> 取更小值。
                        </p>
                      </div>
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                        <div>
                          <label className="text-xs text-gray-500">上下文 Token 预算（contextTokens）</label>
                          <input
                            value={form.contextTokens}
                            onChange={e => updateForm({ contextTokens: e.target.value }, 'context')}
                            placeholder={agentDefaults.contextTokens !== undefined ? `默认 ${agentDefaults.contextTokens}` : '留空=继承默认'}
                            className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                          />
                        </div>
                        <div>
                          <label className="text-xs text-gray-500">压缩模式（compaction.mode）</label>
                          <select
                            value={form.compactionMode}
                            onChange={e => updateForm({ compactionMode: e.target.value as '' | 'default' | 'safeguard' }, 'context')}
                            className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                          >
                            <option value="">继承默认</option>
                            <option value="default">default</option>
                            <option value="safeguard">safeguard</option>
                          </select>
                        </div>
                        <div>
                          <label className="text-xs text-gray-500">历史占比上限（compaction.maxHistoryShare）</label>
                          <input
                            value={form.compactionMaxHistoryShare}
                            onChange={e => updateForm({ compactionMaxHistoryShare: e.target.value }, 'context')}
                            placeholder={agentDefaults.compactionMaxHistoryShare !== undefined ? `默认 ${agentDefaults.compactionMaxHistoryShare}` : '例如 0.5'}
                            className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                          />
                        </div>
                      </div>
                      <div className="text-[11px] text-gray-500 leading-relaxed">
                        当前默认：contextTokens = {agentDefaults.contextTokens !== undefined ? String(agentDefaults.contextTokens) : '未设置'}，
                        compaction.mode = {agentDefaults.compactionMode || '未设置'}，
                        compaction.maxHistoryShare = {agentDefaults.compactionMaxHistoryShare !== undefined ? String(agentDefaults.compactionMaxHistoryShare) : '未设置'}。
                      </div>
                    </div>
                  </div>

                  <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">身份摘要（Identity）</h4>
                      <p className="text-xs text-gray-500 mt-1">已对齐官方 <span className="font-mono">openclaw agents set-identity</span>：保存时写入 <span className="font-mono">identity.name / theme / emoji / avatar</span>。</p>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                      <div>
                        <label className="text-xs text-gray-500">身份名称（Identity Name）</label>
                        <input
                          value={form.identityName}
                          onChange={e => updateForm({ identityName: e.target.value }, 'identity')}
                          placeholder="例如 OpenClaw"
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">主题（Theme）</label>
                        <input
                          value={form.identityTheme}
                          onChange={e => updateForm({ identityTheme: e.target.value }, 'identity')}
                          placeholder="例如 space lobster"
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">表情（Emoji）</label>
                        <input
                          value={form.identityEmoji}
                          onChange={e => updateForm({ identityEmoji: e.target.value }, 'identity')}
                          placeholder="例如 🦞"
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                      </div>
                      <div className="md:col-span-2">
                        <label className="text-xs text-gray-500">头像（Avatar）</label>
                        <input
                          value={form.identityAvatar}
                          onChange={e => updateForm({ identityAvatar: e.target.value }, 'identity')}
                          placeholder="工作区相对路径、http(s) URL 或 data URI"
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                      </div>
                    </div>
                  </div>
                </div>
              )}

              {formSection === 'access' && (
                <div className="space-y-5">
                  <div className="rounded-xl border border-sky-100 bg-sky-50 px-4 py-3 text-[12px] text-sky-700 space-y-1.5">
                    <div><span className="font-medium">workspace</span> 只是默认 cwd；真正决定“在哪里执行”的是 <span className="font-medium">sandbox</span>。</div>
                    <div><span className="font-medium">workspaceAccess</span> 决定工作区如何挂载，<span className="font-medium">docker.binds</span> 则会额外挂入宿主机目录，两者彼此独立。</div>
                    <div><span className="font-medium">tools.allow / deny / sandbox.tools / elevated</span> 仍属于工具策略，不在 <span className="font-mono">sandbox</span> JSON 里；它们会继续在 Tools JSON 中生效。</div>
                  </div>

                  {sandboxStructuredLocked && (
                    <div className="rounded-xl border border-amber-100 bg-amber-50 px-4 py-3 text-[12px] text-amber-700 space-y-1.5">
                      <div className="font-medium">当前 sandbox 含有结构化表单暂不支持的值。</div>
                      <div>为避免保存时丢失未知枚举，已锁定结构化 sandbox 编辑；请先到 Advanced JSON 调整这些字段：<span className="font-mono">{sandboxStructuredIssues.join(', ')}</span></div>
                    </div>
                  )}

                  <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">Sandbox 起步模板（Starter Preset）</h4>
                      <p className="text-xs text-gray-500 mt-1">先用模板建立正确心智模型，再按需补充 mode / scope / docker 覆盖；更高阶字段仍可回到 Advanced JSON。</p>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-[220px,1fr] gap-4">
                      <div>
                        <label className="text-xs text-gray-500">起步模板（Starter Preset）</label>
                        <select
                          value={sandboxStarter}
                          onChange={e => applySandboxStarter(e.target.value)}
                          disabled={sandboxStructuredLocked}
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        >
                          {SANDBOX_STARTERS.map(starter => (
                            <option key={starter.key} value={starter.key}>{starter.label}</option>
                          ))}
                          {sandboxStarter === 'custom' && <option value="custom">自定义 JSON</option>}
                        </select>
                      </div>
                      <div className="rounded-lg bg-gray-50 dark:bg-gray-900 border border-gray-100 dark:border-gray-700 px-4 py-3 text-[12px] text-gray-600 dark:text-gray-300">
                        {sandboxStarter === 'custom'
                          ? '当前 sandbox 不是内置模板。你可以保留现状，或切换到某个起步模板后再去 Advanced 微调。'
                          : SANDBOX_STARTERS.find(item => item.key === sandboxStarter)?.help}
                      </div>
                    </div>

                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div>
                        <label className="text-xs text-gray-500">运行模式（sandbox.mode）</label>
                        <select
                          value={form.sandboxMode}
                          onChange={e => updateSandboxForm({ sandboxMode: e.target.value as '' | 'off' | 'non-main' | 'all' })}
                          disabled={sandboxStructuredLocked}
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        >
                          <option value="">继承默认</option>
                          <option value="off">关闭（off，宿主机运行）</option>
                          <option value="non-main">仅非主会话（non-main）</option>
                          <option value="all">全部会话（all）</option>
                        </select>
                        <p className="mt-1 text-[11px] text-gray-400">官方里 <span className="font-mono">non-main</span> 是常见默认值；群聊 / channel / thread 会话通常都会落入这个分支。</p>
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">容器复用范围（sandbox.scope）</label>
                        <select
                          value={form.sandboxScope}
                          onChange={e => updateSandboxForm({ sandboxScope: e.target.value as '' | 'session' | 'agent' | 'shared' })}
                          disabled={sandboxStructuredLocked}
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        >
                          <option value="">继承默认</option>
                          <option value="session">每会话一个容器（session）</option>
                          <option value="agent">每 Agent 一个容器（agent）</option>
                          <option value="shared">全部共享一个容器（shared）</option>
                        </select>
                        <p className="mt-1 text-[11px] text-gray-400">若选 <span className="font-mono">shared</span>，官方语义下 per-agent <span className="font-mono">docker.binds</span> 会被忽略。</p>
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">工作区挂载方式（workspaceAccess）</label>
                        <select
                          value={form.sandboxWorkspaceAccess}
                          onChange={e => updateSandboxForm({ sandboxWorkspaceAccess: e.target.value as '' | 'none' | 'ro' | 'rw' })}
                          disabled={sandboxStructuredLocked}
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        >
                          <option value="">继承默认</option>
                          <option value="none">仅沙箱工作区（none）</option>
                          <option value="ro">只读挂载 Agent workspace（ro）</option>
                          <option value="rw">可写挂载 Agent workspace（rw）</option>
                        </select>
                        <p className="mt-1 text-[11px] text-gray-400"><span className="font-mono">none</span> 才是官方默认；<span className="font-mono">ro/rw</span> 只影响 workspace 挂载，不会自动影响其它 bind mount。</p>
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">沙箱工作区根（workspaceRoot）</label>
                        <input
                          value={form.sandboxWorkspaceRoot}
                          onChange={e => updateSandboxForm({ sandboxWorkspaceRoot: e.target.value })}
                          disabled={sandboxStructuredLocked}
                          placeholder="例如 /workspace 或 /agent"
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                        <p className="mt-1 text-[11px] text-gray-400">只在你需要自定义容器内工作区根时覆盖；留空表示继承默认。</p>
                      </div>
                    </div>
                  </div>

                  <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">Docker 覆盖（sandbox.docker）</h4>
                      <p className="text-xs text-gray-500 mt-1">这些字段决定容器根文件系统、网络和额外挂载；它们不会替代工具 allow/deny。</p>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div>
                        <label className="text-xs text-gray-500">容器网络（docker.network）</label>
                        <input
                          value={form.sandboxDockerNetwork}
                          onChange={e => updateSandboxForm({ sandboxDockerNetwork: e.target.value })}
                          disabled={sandboxStructuredLocked}
                          placeholder="例如 none / bridge / custom-network"
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                        <p className="mt-1 text-[11px] text-gray-400">官方默认是无网络；只有确实需要下载依赖或联网工具时再显式开放。</p>
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">根文件系统只读（docker.readOnlyRoot）</label>
                        <select
                          value={form.sandboxDockerReadOnlyRoot}
                          onChange={e => updateSandboxForm({ sandboxDockerReadOnlyRoot: e.target.value as InheritToggle })}
                          disabled={sandboxStructuredLocked}
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        >
                          <option value="inherit">继承默认</option>
                          <option value="enabled">显式只读</option>
                          <option value="disabled">显式可写</option>
                        </select>
                        <p className="mt-1 text-[11px] text-gray-400">如果你依赖 <span className="font-mono">setupCommand</span> 做包安装，通常需要关闭只读根并允许网络。</p>
                      </div>
                      <div className="md:col-span-2">
                        <label className="text-xs text-gray-500">一次性初始化命令（docker.setupCommand）</label>
                        <textarea
                          rows={3}
                          value={form.sandboxDockerSetupCommand}
                          onChange={e => updateSandboxForm({ sandboxDockerSetupCommand: e.target.value })}
                          disabled={sandboxStructuredLocked}
                          placeholder="例如 apt-get update && apt-get install -y nodejs"
                          className="w-full mt-1 px-3 py-2 text-sm font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                        <p className="mt-1 text-[11px] text-gray-400">它只在容器创建时执行一次；如果需要 root / 可写根 / 网络，请与上面的字段一起配置。</p>
                      </div>
                      <div className="md:col-span-2">
                        <label className="text-xs text-gray-500">额外挂载（docker.binds，一行一个）</label>
                        <textarea
                          rows={4}
                          value={form.sandboxDockerBinds}
                          onChange={e => updateSandboxForm({ sandboxDockerBinds: e.target.value })}
                          disabled={sandboxStructuredLocked}
                          placeholder={"/host/source:/source:ro\n/var/data/myapp:/data:rw"}
                          className="w-full mt-1 px-3 py-2 text-sm font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                        <p className="mt-1 text-[11px] text-amber-600">注意：<span className="font-mono">docker.binds</span> 会直接暴露宿主机路径；即使 <span className="font-mono">workspaceAccess</span> 是 <span className="font-mono">none/ro</span>，bind 仍然可能扩大访问面。</p>
                      </div>
                    </div>
                  </div>

                  <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">工具策略与逃逸口（Tool Policy & Elevated）</h4>
                      <p className="text-xs text-gray-500 mt-1">这里不直接改 <span className="font-mono">tools.sandbox.tools</span> / <span className="font-mono">tools.elevated</span>，但要先理解它们和 sandbox 的关系。</p>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
                      <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3 text-gray-600 dark:text-gray-300">
                        <div className="font-medium text-gray-900 dark:text-white">Sandbox</div>
                        <div className="mt-1">决定工具是在宿主机还是 Docker 里运行。</div>
                      </div>
                      <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3 text-gray-600 dark:text-gray-300">
                        <div className="font-medium text-gray-900 dark:text-white">Tool Policy</div>
                        <div className="mt-1"><span className="font-mono">tools.allow / deny / sandbox.tools</span> 决定“这个工具能不能被调用”。</div>
                      </div>
                      <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3 text-gray-600 dark:text-gray-300">
                        <div className="font-medium text-gray-900 dark:text-white">Elevated</div>
                        <div className="mt-1"><span className="font-mono">tools.elevated</span> 只是 <span className="font-mono">exec</span> 的宿主机逃逸口，不会授予被 deny 的工具。</div>
                      </div>
                    </div>
                    <p className="text-[11px] text-gray-400">如果你要进一步配置 <span className="font-mono">tools.sandbox.tools</span>、<span className="font-mono">tools.elevated</span>、<span className="font-mono">sandbox.browser.*</span>、<span className="font-mono">docker.image/user/env</span>，请继续在 Advanced JSON 里编辑。</p>
                  </div>

                  <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                    <div className="flex flex-col gap-2 lg:flex-row lg:items-start lg:justify-between">
                      <div>
                        <h4 className="text-sm font-semibold text-gray-900 dark:text-white">Per-Agent 工具治理（Profile / Allow / Deny）</h4>
                        <p className="text-xs text-gray-500 mt-1">对标官方 Agents 面板里最常用的 Tool Access 起点：先选 profile，再按需补 <span className="font-mono">tools.allow / tools.deny</span>。</p>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        <button
                          type="button"
                          onClick={applyHeadlessToolPreset}
                          className="px-3 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700"
                        >
                          套用“传统无头”建议
                        </button>
                        <button
                          type="button"
                          onClick={resetToolPolicyOverrides}
                          className="px-3 py-2 text-xs font-medium rounded-lg border border-gray-200 dark:border-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-900"
                        >
                          清空所有工具覆盖
                        </button>
                      </div>
                    </div>

                    <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-5 gap-3">
                      {TOOL_POLICY_PRESETS.map(preset => {
                        const active = form.toolProfile === preset.id;
                        return (
                          <button
                            key={preset.label}
                            type="button"
                            onClick={() => applyToolPolicyPreset(preset.id)}
                            className={`text-left rounded-xl border px-4 py-4 transition-colors ${
                              active
                                ? 'border-violet-400 bg-violet-50 dark:border-violet-700 dark:bg-violet-900/20'
                                : 'border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 hover:border-violet-300 dark:hover:border-violet-700'
                            }`}
                          >
                            <div className="text-sm font-semibold text-gray-900 dark:text-white">{preset.label}</div>
                            <p className="mt-2 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">{preset.help}</p>
                          </button>
                        );
                      })}
                    </div>

                    <div className="grid grid-cols-1 lg:grid-cols-[220px,1fr] gap-4">
                      <div>
                        <label className="text-xs text-gray-500">工具 Profile（tools.profile）</label>
                        <select
                          value={form.toolProfile}
                          onChange={e => updateForm({ toolProfile: e.target.value as AgentFormState['toolProfile'] }, 'tools')}
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        >
                          {TOOL_POLICY_PRESETS.map(preset => (
                            <option key={preset.label} value={preset.id}>{preset.label}</option>
                          ))}
                        </select>
                        <p className="mt-1 text-[11px] text-gray-400">留在 Inherit 时不会写入 per-agent <span className="font-mono">tools.profile</span> 覆盖。</p>
                      </div>
                      <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                        <div>
                          <label className="text-xs text-gray-500">Allow 列表（tools.allow）</label>
                          <textarea
                            rows={4}
                            value={form.toolAllow}
                            onChange={e => updateForm({ toolAllow: e.target.value }, 'tools')}
                            placeholder="逗号分隔，例如 group:web, group:fs"
                            className="w-full mt-1 px-3 py-2 text-sm font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                          />
                        </div>
                        <div>
                          <label className="text-xs text-gray-500">Deny 列表（tools.deny）</label>
                          <textarea
                            rows={4}
                            value={form.toolDeny}
                            onChange={e => updateForm({ toolDeny: e.target.value }, 'tools')}
                            placeholder="逗号分隔，例如 group:runtime, group:ui, group:nodes, group:automation"
                            className="w-full mt-1 px-3 py-2 text-sm font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                          />
                        </div>
                      </div>
                    </div>
                    <p className="text-[11px] text-gray-400">这三项是最常用的 per-agent 工具治理入口；更细的 <span className="font-mono">tools.sandbox.tools</span> 与 <span className="font-mono">tools.elevated</span> 继续放在 Advanced JSON。</p>
                  </div>

                  <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">群聊行为（Group Chat）</h4>
                      <p className="text-xs text-gray-500 mt-1">用于设置本 Agent 是否显式覆盖 <span className="font-mono">groupChat.enabled</span>。留在“继承默认”时，不会写入额外覆盖。</p>
                    </div>
                    {sandboxClearIntent && editingId && (
                      <div className="rounded-lg border border-amber-100 bg-amber-50 px-4 py-3 text-[12px] text-amber-700">
                        你当前选择了“继承默认”。保存时会优先移除结构化 sandbox 字段；如果当前 override 里只剩这些字段，就会删除整块 <span className="font-mono">sandbox</span>。若还有 <span className="font-mono">sandbox.browser.*</span> 等 Advanced 字段，它们会继续保留。
                      </div>
                    )}
                    <div className="max-w-xs">
                        <label className="text-xs text-gray-500">群聊开关（groupChat.enabled）</label>
                      <select
                        value={form.groupChatMode}
                        onChange={e => updateForm({ groupChatMode: e.target.value as InheritToggle }, 'groupChat')}
                        className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                      >
                        <option value="inherit">继承默认</option>
                        <option value="enabled">显式启用</option>
                        <option value="disabled">显式关闭</option>
                      </select>
                    </div>
                  </div>
                </div>
              )}

              {formSection === 'collaboration' && (
                <div className="space-y-5">
                  <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">Agent 间协作（Agent Collaboration）</h4>
                      <p className="text-xs text-gray-500 mt-1">结构化编辑会写回 <span className="font-mono">tools.agentToAgent</span> 与 <span className="font-mono">tools.sessions.visibility</span>。</p>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                      <div>
                        <label className="text-xs text-gray-500">Agent 间委派（Agent-to-Agent Delegation）</label>
                        <select
                          value={form.agentToAgentMode}
                          onChange={e => updateForm({ agentToAgentMode: e.target.value as InheritToggle }, 'tools')}
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        >
                          <option value="inherit">继承默认</option>
                          <option value="enabled">显式启用</option>
                          <option value="disabled">显式关闭</option>
                        </select>
                      </div>
                      <div className="md:col-span-2">
                        <label className="text-xs text-gray-500">委派白名单（Delegation Allow Rules）</label>
                        <input
                          value={form.agentToAgentAllow}
                          onChange={e => updateForm({ agentToAgentAllow: e.target.value }, 'tools')}
                          placeholder="逗号分隔，例如 *, main->work, work->reviewer"
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                        <p className="mt-1 text-[11px] text-gray-400">保存时会写为数组到 <span className="font-mono">tools.agentToAgent.allow</span>。</p>
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">会话可见性（Session Visibility）</label>
                        <select
                          value={form.sessionVisibility}
                          onChange={e => updateForm({ sessionVisibility: e.target.value as '' | 'same-agent' | 'all-agents' }, 'tools')}
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        >
                          <option value="">继承默认</option>
                          <option value="same-agent">仅当前 Agent（same-agent）</option>
                          <option value="all-agents">所有 Agent（all-agents）</option>
                        </select>
                      </div>
                    </div>
                  </div>

                  <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">子 Agent 允许列表（Subagents Allowlist）</h4>
                      <p className="text-xs text-gray-500 mt-1">用于编辑 <span className="font-mono">subagents.allowAgents</span>。留空表示不额外覆盖。</p>
                    </div>
                    <div>
                        <label className="text-xs text-gray-500">允许的 Agent（Allowed Agents）</label>
                      <input
                        value={form.subagentAllowAgents}
                        onChange={e => updateForm({ subagentAllowAgents: e.target.value }, 'subagents')}
                        placeholder="逗号分隔，例如 main, reviewer, summarizer"
                        className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                      />
                    </div>
                  </div>
                </div>
              )}

              {formSection === 'advanced' && (
                <div className="space-y-5">
                  <div className="rounded-xl border border-violet-100 bg-violet-50 dark:bg-violet-900/10 dark:border-violet-900/30 px-4 py-3 flex flex-col md:flex-row md:items-center md:justify-between gap-3">
                    <div className="text-[12px] text-violet-700 dark:text-violet-200">
                      高级 JSON（Advanced）保留完整编辑能力。保存时，结构化表单会覆盖相同路径上的值；如果你在这里改了结构化字段对应的 JSON，可先点击“从 JSON 同步结构化字段”。像 <span className="font-mono">tools.sandbox.tools</span>、<span className="font-mono">tools.elevated</span>、<span className="font-mono">sandbox.browser.*</span>、<span className="font-mono">docker.image/user/env</span> 这类高阶字段，仍建议直接在这里编辑。
                    </div>
                    <button
                      onClick={syncStructuredFieldsFromAdvanced}
                      className="px-3 py-2 text-xs rounded-lg bg-white dark:bg-gray-900 border border-violet-200 dark:border-violet-800 text-violet-700 dark:text-violet-200 shrink-0"
                    >
                      从 JSON 同步结构化字段
                    </button>
                  </div>

                  <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                    <div>
                      <label htmlFor="agent-model-json" className="text-xs text-gray-500">model (JSON)</label>
                      <textarea
                        id="agent-model-json"
                        name="agentModelJson"
                        rows={8}
                        value={form.modelText}
                        onChange={e => setForm(prev => ({ ...prev, modelText: e.target.value }))}
                        className="w-full mt-1 px-3 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900"
                      />
                    </div>
                    <div>
                      <label htmlFor="agent-params-json" className="text-xs text-gray-500">params (JSON)</label>
                      <textarea
                        id="agent-params-json"
                        name="agentParamsJson"
                        rows={8}
                        value={form.paramsText}
                        onChange={e => setForm(prev => ({ ...prev, paramsText: e.target.value }))}
                        className="w-full mt-1 px-3 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900"
                      />
                    </div>
                    <div>
                      <label htmlFor="agent-identity-json" className="text-xs text-gray-500">identity (JSON)</label>
                      <textarea
                        id="agent-identity-json"
                        name="agentIdentityJson"
                        rows={8}
                        value={form.identityText}
                        onChange={e => setForm(prev => ({ ...prev, identityText: e.target.value }))}
                        className="w-full mt-1 px-3 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900"
                      />
                    </div>
                    <div>
                      <label htmlFor="agent-groupchat-json" className="text-xs text-gray-500">groupChat (JSON)</label>
                      <textarea
                        id="agent-groupchat-json"
                        name="agentGroupChatJson"
                        rows={8}
                        value={form.groupChatText}
                        onChange={e => setForm(prev => ({ ...prev, groupChatText: e.target.value }))}
                        className="w-full mt-1 px-3 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900"
                      />
                    </div>
                    <div>
                      <label htmlFor="agent-tools-json" className="text-xs text-gray-500">tools (JSON)</label>
                      <textarea
                        id="agent-tools-json"
                        name="agentToolsJson"
                        rows={8}
                        value={form.toolsText}
                        onChange={e => setForm(prev => ({ ...prev, toolsText: e.target.value }))}
                        className="w-full mt-1 px-3 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900"
                      />
                    </div>
                    <div>
                      <label htmlFor="agent-subagents-json" className="text-xs text-gray-500">subagents (JSON)</label>
                      <textarea
                        id="agent-subagents-json"
                        name="agentSubagentsJson"
                        rows={8}
                        value={form.subagentsText}
                        onChange={e => setForm(prev => ({ ...prev, subagentsText: e.target.value }))}
                        className="w-full mt-1 px-3 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900"
                      />
                    </div>
                    <div>
                      <label htmlFor="agent-sandbox-json" className="text-xs text-gray-500">sandbox (JSON)</label>
                      <textarea
                        id="agent-sandbox-json"
                        name="agentSandboxJson"
                        rows={8}
                        value={form.sandboxText}
                        onChange={e => {
                          const value = e.target.value;
                          setForm(prev => {
                            const next = { ...prev, sandboxText: value };
                            try {
                              const parsed = value.trim() ? JSON.parse(value) : undefined;
                              const draft = extractSandboxDraft(parsed);
                              next.sandboxMode = draft.mode;
                              next.sandboxScope = draft.scope;
                              next.sandboxWorkspaceAccess = draft.workspaceAccess;
                              next.sandboxWorkspaceRoot = draft.workspaceRoot;
                              next.sandboxDockerNetwork = draft.dockerNetwork;
                              next.sandboxDockerReadOnlyRoot = draft.dockerReadOnlyRoot;
                              next.sandboxDockerSetupCommand = draft.dockerSetupCommand;
                              next.sandboxDockerBinds = draft.dockerBinds;
                            } catch {
                              // Keep the last structured snapshot until the JSON becomes valid again.
                            }
                            return next;
                          });
                          setStructuredTouched(prev => ({ ...prev, sandbox: false }));
                          setSandboxClearIntent(false);
                        }}
                        className="w-full mt-1 px-3 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900"
                      />
                    </div>
                    <div>
                      <label htmlFor="agent-runtime-json" className="text-xs text-gray-500">runtime (JSON)</label>
                      <textarea
                        id="agent-runtime-json"
                        name="agentRuntimeJson"
                        rows={8}
                        value={form.runtimeText}
                        onChange={e => setForm(prev => ({ ...prev, runtimeText: e.target.value }))}
                        placeholder="仅在需要 per-agent runtime 覆盖时填写"
                        className="w-full mt-1 px-3 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900"
                      />
                    </div>
                  </div>
                </div>
              )}
            </div>

            <div className="sticky bottom-0 border-t border-gray-100 dark:border-gray-700 bg-white/95 dark:bg-gray-800/95 backdrop-blur px-5 py-4 flex flex-col md:flex-row md:items-center md:justify-between gap-3">
              <div className={`text-[11px] ${footerValidationIsError ? 'text-red-600' : 'text-gray-500'}`}>
                {footerValidationMessage}
              </div>
              <div className="flex items-center justify-end gap-2">
                <button onClick={closeForm} className="px-4 py-2 text-xs rounded bg-gray-100 dark:bg-gray-700">取消（Cancel）</button>
                <button onClick={saveAgent} disabled={saving || !!saveBlockedReason} title={saveBlockedReason || ''} className="px-4 py-2 text-xs rounded bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50 disabled:cursor-not-allowed">
                  {saving ? '保存中...' : '保存智能体（Save Agent）'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

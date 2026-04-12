import { memo, useEffect, useMemo, useRef, useState } from 'react';
import { useOutletContext, useSearchParams } from 'react-router-dom';
import { api } from '../lib/api';
import { Plus, RefreshCw, Save, Trash2, ArrowUp, ArrowDown, Route, Bot, Settings, Brain, Shield, ChevronDown, ChevronRight, Sparkles, FileText } from 'lucide-react';
import InfoTooltip from '../components/InfoTooltip';

interface AgentItem {
  id: string;
  name?: string;
  workspace?: string;
  agentDir?: string;
  model?: any;
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
type 继承默认Toggle = 'inherit' | 'enabled' | 'disabled';

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
  identityName: string;
  identityTheme: string;
  identityEmoji: string;
  identityAvatar: string;
  sandboxMode: '' | 'off' | 'non-main' | 'all';
  sandboxScope: '' | 'session' | 'agent' | 'shared';
  sandboxWorkspaceAccess: '' | 'none' | 'ro' | 'rw';
  sandboxWorkspaceRoot: string;
  sandboxDockerNetwork: string;
  sandboxDockerReadOnlyRoot: 继承默认Toggle;
  sandboxDockerSetupCommand: string;
  sandboxDockerBinds: string;
  groupChatMode: 继承默认Toggle;
  toolProfile: '' | 'minimal' | 'coding' | 'messaging' | 'full';
  toolAllow: string;
  toolDeny: string;
  agentToAgentMode: 继承默认Toggle;
  agentToAgentAllow: string;
  sessionVisibility: SessionVisibility;
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

interface BindingAcpDraft {
  mode: string;
  label: string;
  cwd: string;
  backend: string;
}

interface BindingDraft {
  type: 'route' | 'acp';
  agentId: string;
  comment: string;
  enabled: boolean;
  match: Record<string, any>;
  acp: BindingAcpDraft;
  matchText: string;
  mode: 'structured' | 'json';
  rowError?: string;
}

interface PreviewResult {
  agent?: string;
  matchedBy?: string;
  matchedIndex?: number;
  trace?: string[];
}

interface ChannelMeta {
  accounts: string[];
  defaultAccount?: string;
}

const LITE_WORKSPACE_ROOT = '/opt/clawpanel-lite/data/openclaw-work';
const LITE_AGENT_ROOT = '/opt/clawpanel-lite/data/openclaw-config/agents';

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

interface CoreFilesLoadState {
  kind: 'ready' | 'missing' | 'restricted' | 'error';
  message?: string;
  workspace?: string;
}

interface SkillEntry {
  id: string;
  name: string;
  description?: string;
  enabled: boolean;
  source: string;
  version?: string;
}

interface AgentSkillsContext {
  agentId: string;
  workspace: string;
}

type SessionVisibility = '' | 'self' | 'tree' | 'agent' | 'all';

interface CronJob {
  id: string;
  name: string;
  enabled: boolean;
  agentId?: string;
  sessionTarget: string;
  wakeMode?: string;
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
  { id: 'basic', title: '基础信息', description: '核心身份、工作区与默认关系' },
  { id: 'behavior', title: '行为设定', description: '模型、常用参数与身份摘要' },
  { id: 'access', title: '访问与安全', description: 'sandbox 与群聊行为' },
  { id: 'collaboration', title: '协作说明', description: '当前仅支持全局协作策略' },
  { id: 'advanced', title: '高级 JSON', description: '完整 JSON 覆盖与 runtime' },
];
const SANDBOX_STARTERS = [
  { key: 'inherit', label: '继承默认', help: '不写 sandbox 覆盖，沿用全局或默认智能体 配置。', text: '' },
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
  { id: 'overview', title: '概览', description: '先理解定位、默认关系与关键摘要' },
  { id: 'model', title: '模型与身份', description: '模型、参数与身份口吻' },
  { id: 'tools', title: '工具与权限', description: '工具策略、委派与 sandbox' },
  { id: 'files', title: '核心文件', description: '查看并编辑工作区里的关键文档' },
  { id: 'capabilities', title: '技能与运行态', description: '按当前智能体的 workspace / bindings / cron 观察有效结果' },
  { id: 'context', title: '路由上下文', description: '查看它被哪些规则和会话引用' },
  { id: 'advanced', title: '高级 JSON', description: '需要完整 JSON 时再进入' },
];

const TOOL_POLICY_PRESETS: Array<{ id: '' | 'minimal' | 'coding' | 'messaging' | 'full'; label: string; help: string }> = [
  { id: '', label: '继承默认', help: '不写单智能体 profile，继续继承全局工具策略。' },
  { id: 'minimal', label: 'Minimal', help: '只保留最小会话能力，适合作为最保守的起点。' },
  { id: 'coding', label: '编码优先', help: '偏向编码/文件/运行时工具，适合代码型智能体。' },
  { id: 'messaging', label: '消息协作', help: '偏向消息入口与渠道协作，适合路由型智能体。' },
  { id: 'full', label: '完整能力', help: '完整工具面；除非明确需要，否则不建议长期给高权限智能体。' },
];

const AGENT_CORE_FILE_META: Record<string, { label: string; description: string }> = {
  'AGENTS.md': { label: '工作说明', description: '定义这个智能体的总任务边界与协作规则。' },
  'SOUL.md': { label: '人格内核', description: '记录风格、价值观与长期行为倾向。' },
  'TOOLS.md': { label: '工具策略', description: '补充工具使用原则、限制与推荐模式。' },
  'IDENTITY.md': { label: '身份设定', description: '描述角色、人设和对外表达方式。' },
  'USER.md': { label: '用户偏好', description: '放业务方偏好、长期协作约定与注意事项。' },
  'HEARTBEAT.md': { label: '自检节奏', description: '约定这个智能体的周期性自检或提醒。' },
  'BOOT.md': { label: '启动检查', description: 'Gateway 重启时可执行的简短启动检查清单。' },
  'BOOTSTRAP.md': { label: '启动引导', description: '首次加载时最应该先看的说明或初始化步骤。' },
  'MEMORY.md': { label: '记忆摘录', description: '存放长期记忆或需要沉淀给后续会话的内容。' },
};

const CHANNEL_DISPLAY_NAMES: Record<string, string> = {
  qq: 'QQ',
  discord: 'Discord',
  feishu: '飞书 / Lark',
  slack: 'Slack',
  telegram: 'Telegram',
  wechat: '微信',
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
    placeholder: '留空=按默认账号预览',
    help: '留空时，面板会在已知 channel 下自动代入 defaultAccount；需要验证指定账号规则时再填写。',
  },
  sender: {
    label: '发送者',
    placeholder: '例如 user-10001',
    help: '仅旧版 sender 兼容规则需要填写；普通按 peer 路由时通常留空。',
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
    placeholder: '例如 主工作区',
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
    description: '先描述消息来自哪个通道、哪个账号；旧 sender 规则时再补发送者。',
    fields: ['channel', 'accountId', 'sender'],
  },
  {
    title: '会话范围',
    description: '当你要验证会话、父会话继承、Discord 或 Slack 场景时，再补这些条件。',
    fields: ['peer', 'parentPeer', 'guildId', 'teamId', 'roles'],
  },
];

const BINDING_TYPE_OPTIONS: Array<{ value: 'route' | 'acp'; label: string; description: string }> = [
  { value: 'route', label: '路由规则', description: '把匹配到的消息交给目标智能体。' },
  { value: 'acp', label: 'ACP 规则', description: '在命中时附带 ACP 执行配置。' },
];

const OFFICIAL_MATCHED_BY_LABELS: Record<string, string> = {
  'binding.sender': '发送者',
  'binding.peer': '会话',
  'binding.peer.parent': '父会话继承',
  'binding.guild+roles': '服务器 + 角色',
  'binding.guild': '服务器',
  'binding.team': '团队 / 工作区',
  'binding.account': '账号',
  'binding.account.wildcard': '账号通配',
  'binding.channel': '通道',
  default: '默认兜底',
};

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

function normalizeSessionVisibility(raw: any): SessionVisibility {
  const value = String(raw || '').trim();
  switch (value) {
    case 'same-agent':
      return 'agent';
    case 'all-agents':
      return 'all';
    case 'self':
    case 'tree':
    case 'agent':
    case 'all':
      return value;
    default:
      return '';
  }
}

function extractAcpDraft(raw: any): BindingAcpDraft {
  if (!isPlainObject(raw)) {
    return { mode: '', label: '', cwd: '', backend: '' };
  }
  return {
    mode: String(raw.mode || '').trim(),
    label: String(raw.label || '').trim(),
    cwd: String(raw.cwd || '').trim(),
    backend: String(raw.backend || '').trim(),
  };
}

function compactMatch(raw: any): Record<string, any> {
  if (!isPlainObject(raw)) return {};
  const out: Record<string, any> = {};
  for (const key of ALLOWED_MATCH_KEYS) {
    if (!(key in raw)) continue;
    const v = raw[key];
    if (v === undefined || v === null) continue;

    if (key === 'peer') {
      if (isPlainObject(v)) {
        const peer = normalizePeerValue(v);
        if (!peer || !peer.kind || !peer.id) continue;
        out[key] = { kind: peer.kind, id: peer.id };
        continue;
      }
      if (typeof v === 'string') {
        const peer = normalizePeerValue(v);
        if (!peer || !peer.kind || !peer.id) continue;
        out[key] = `${peer.kind}:${peer.id}`;
        continue;
      }
      continue;
    }

    if (key === 'parentPeer') {
      if (isPlainObject(v)) {
        const peer = normalizePeerValue(v);
        if (!peer || !peer.kind || !peer.id) continue;
        out[key] = { kind: peer.kind, id: peer.id };
        continue;
      }
      if (typeof v === 'string') {
        const peer = normalizePeerValue(v);
        if (!peer || !peer.kind || !peer.id) continue;
        out[key] = `${peer.kind}:${peer.id}`;
        continue;
      }
      continue;
    }

    if (key === 'roles' && Array.isArray(v)) {
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
      if (Array.isArray(v) && v.every(item => typeof item === 'string' && item.trim())) continue;
      return false;
    }
    if (key === 'peer' || key === 'parentPeer') {
      if (isPlainObject(v)) {
        const peer = normalizePeerValue(v);
        if (peer?.kind && peer.id) continue;
        return false;
      }
      if (typeof v === 'string') {
        const peer = normalizePeerValue(v);
        if (peer?.kind && peer.id) continue;
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
    type: String(raw?.type || '').trim() === 'acp' ? 'acp' : 'route',
    agentId: String(raw?.agentId || raw?.agent || fallbackAgent || 'main'),
    comment: String(raw?.comment || raw?.name || ''),
    enabled: raw?.enabled !== false,
    match,
    acp: extractAcpDraft(raw?.acp),
    mode,
    matchText: JSON.stringify(match, null, 2),
    rowError: '',
  };
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
  if ('accountId' in match && extractTextValue(match.accountId) !== '*') return 'accountId';
  if ('accountId' in match && extractTextValue(match.accountId) === '*') return 'accountId:*';
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
    if (['channel', 'guildId', 'teamId', 'accountId'].includes(key) && !extractTextValue(match[key])) {
      return `第 ${idx} 条 binding 的 ${key} 必须是非空字符串`;
    }
    if (key === 'roles' && !('guildId' in match)) {
      return `第 ${idx} 条 binding 的 roles 必须与 guildId 同时使用`;
    }
    if (key === 'roles' && !Array.isArray(match[key])) {
      return `第 ${idx} 条 binding 的 roles 必须是字符串数组`;
    }
    if ((key === 'peer' || key === 'parentPeer') && (isPlainObject(match[key]) || typeof match[key] === 'string')) {
      const peer = normalizePeerValue(match[key]);
      if (!peer?.kind || !peer.id) {
        return `第 ${idx} 条 binding 的 ${key} 必须是 kind:id 字符串或 { kind, id } 对象`;
      }
    } else if (key === 'peer' || key === 'parentPeer') {
      return `第 ${idx} 条 binding 的 ${key} 必须是 kind:id 字符串或 { kind, id } 对象`;
    }
  }
  return null;
}

function detectDuplicateAccountBindings(rows: BindingDraft[]) {
  const owners = new Map<string, Set<string>>();
  for (const row of rows) {
    if (!row || row.enabled === false || row.type === 'acp') continue;
    const match = compactMatch(row.match);
    const channel = extractTextValue(match.channel).trim();
    const accountId = extractTextValue(match.accountId).trim();
    const agentId = String(row.agentId || '').trim();
    if (!channel || !accountId || accountId === '*' || !agentId) continue;
    const key = `${channel}\u0000${accountId}`;
    if (!owners.has(key)) owners.set(key, new Set<string>());
    owners.get(key)?.add(agentId);
  }
  return Array.from(owners.entries()).flatMap(([key, agentIds]) => {
    if (agentIds.size <= 1) return [];
    const [channel, accountId] = key.split('\u0000');
    return [{ channel, accountId, agentIds: Array.from(agentIds).sort() }];
  });
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

function resolveCronJobAgentID(job: CronJob, fallbackAgent: string): string {
  const agentID = String(job.agentId || '').trim();
  if (agentID) return agentID;
  const legacyTarget = String(job.sessionTarget || '').trim();
  if (legacyTarget && legacyTarget !== 'main' && legacyTarget !== 'isolated') return legacyTarget;
  return fallbackAgent;
}

function humanizeSkillSource(source: string): string {
  switch (source) {
    case 'workspace':
      return 'workspace/skills';
    case 'workspace-agent':
      return 'workspace/.agents/skills';
    case 'managed':
      return '~/.openclaw/skills';
    case 'global-agent':
      return '~/.agents/skills';
    case 'app-skill':
      return 'bundled';
    case 'plugin-skill':
      return 'plugin skill';
    case 'extra-dir':
      return 'extraDirs';
    default:
      return source || 'unknown';
  }
}

function summarizeSkillSourceGroup(source: string): 'workspace' | 'shared' | 'bundled' | 'plugin' | 'extra' | 'other' {
  switch (source) {
    case 'workspace':
    case 'workspace-agent':
      return 'workspace';
    case 'managed':
    case 'global-agent':
      return 'shared';
    case 'app-skill':
      return 'bundled';
    case 'plugin-skill':
      return 'plugin';
    case 'extra-dir':
      return 'extra';
    default:
      return 'other';
  }
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
      return '父会话继承';
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
      return '父会话继承';
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

function describeTriState(raw: 继承默认Toggle, enabledLabel: string, disabledLabel: string): string {
  if (raw === 'enabled') return enabledLabel;
  if (raw === 'disabled') return disabledLabel;
  return '继承默认';
}

function describeSessionVisibility(raw: any): string {
  switch (normalizeSessionVisibility(raw)) {
    case 'self':
      return '仅当前会话';
    case 'tree':
      return '当前会话树';
    case 'agent':
      return '当前智能体的全部会话';
    case 'all':
      return '所有会话';
  }
  return '继承默认';
}

function describe沙箱Mode(raw: any): string {
  const draft = extract沙箱Draft(raw);
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

function describe沙箱ModeValue(raw: string): string {
  switch (raw) {
    case 'off':
      return '关闭 - 宿主机运行';
    case 'non-main':
      return '仅非主会话';
    case 'all':
      return '全部会话';
    default:
      return '继承默认';
  }
}

function describe沙箱ScopeValue(raw: string): string {
  switch (raw) {
    case 'session':
      return '每会话一个容器';
    case 'agent':
      return '每 Agent 一个容器';
    case 'shared':
      return '全部共享一个容器';
    default:
      return '继承默认';
  }
}

function describeWorkspaceAccessValue(raw: string): string {
  switch (raw) {
    case 'none':
      return '仅沙箱工作区';
    case 'ro':
      return '只读挂载智能体工作区（ro）';
    case 'rw':
      return '可写挂载智能体工作区（rw）';
    default:
      return '继承默认';
  }
}

function describeToggleValue(raw: 继承默认Toggle, enabledLabel: string, disabledLabel: string): string {
  if (raw === 'enabled') return enabledLabel;
  if (raw === 'disabled') return disabledLabel;
  return '继承默认';
}

function normalizeLegacy沙箱Draft(raw: any): Record<string, any> {
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

function extract沙箱Draft(raw: any) {
  const sandbox = normalizeLegacy沙箱Draft(raw);
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

function buildStructured沙箱Preview(form: Pick<AgentFormState, 'sandboxMode' | 'sandboxScope' | 'sandboxWorkspaceAccess' | 'sandboxWorkspaceRoot' | 'sandboxDockerNetwork' | 'sandboxDockerReadOnlyRoot' | 'sandboxDockerSetupCommand' | 'sandboxDockerBinds'>): any {
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
  if (parentPeer.kind) parts.push(`父会话 ${parentPeer.kind}${parentPeer.id ? `:${parentPeer.id}` : ''}`);
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
  if (extractPeerForm(match, 'parentPeer').kind) tags.push('父会话');
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
    if (key === 'peer' || key === 'parentPeer') {
      const peer = normalizePeerValue(text);
      if (peer?.kind && peer.id) {
        meta[key] = { kind: peer.kind, id: peer.id };
        continue;
      }
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

function validateAvatarValue(raw: string): string {
  const value = raw.trim();
  if (!value) return '';
  const lower = value.toLowerCase();
  if (lower.startsWith('http://') || lower.startsWith('https://')) return '';
  if (lower.startsWith('data:image/') || lower.startsWith('data:')) return '';
  if (value.startsWith('~')) return '头像不支持以 ~ 开头，请使用工作区相对路径、http(s) 地址或 data URI。';
  if (/^[a-z][a-z0-9+.-]*:/i.test(value)) return '头像仅支持 http(s) 地址、data URI 或工作区相对路径。';
  if (value.startsWith('/')) return '头像不支持绝对路径，请改用工作区相对路径。';
  return '';
}

function resolveAvatarPreviewSrc(agentId: string | undefined, avatar: string): string {
  const value = avatar.trim();
  if (!value || validateAvatarValue(value)) return '';
  if (/^https?:\/\//i.test(value) || /^data:/i.test(value)) return value;
  if (!agentId) return '';
  return api.agentIdentityAvatarUrl(agentId);
}

function parseIdentityMarkdown(content: string): Partial<Record<'name' | 'theme' | 'emoji' | 'avatar' | 'creature' | 'vibe', string>> {
  const result: Partial<Record<'name' | 'theme' | 'emoji' | 'avatar' | 'creature' | 'vibe', string>> = {};
  for (const rawLine of content.split('\n')) {
    let line = rawLine.trim();
    if (!line.includes(':')) continue;
    line = line.replace(/^[-*]\s*/, '');
    const [rawKey, ...rest] = line.split(':');
    const key = rawKey.trim().toLowerCase().replace(/[*_]/g, '');
    const value = rest.join(':').trim().replace(/^[(*\s]+|[)*\s]+$/g, '');
    if (!value) continue;
    if (key === 'name' || key === 'theme' || key === 'emoji' || key === 'avatar' || key === 'creature' || key === 'vibe') {
      result[key] = value;
    }
  }
  return result;
}

function classifyCoreFilesLoadState(error: string, workspace?: string): CoreFilesLoadState {
  if (!error) return { kind: 'error', message: '读取核心文件失败', workspace };
  if (error.includes('未配置')) return { kind: 'missing', message: error, workspace };
  if (error.includes('受管工作区') || error.includes('符号链接') || error.includes('受保护') || error.includes('暂不支持')) {
    return { kind: 'restricted', message: error, workspace };
  }
  return { kind: 'error', message: error, workspace };
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

function triStateFromValue(raw: any): 继承默认Toggle {
  if (raw === true) return 'enabled';
  if (raw === false) return 'disabled';
  return 'inherit';
}

function triStateToValue(raw: 继承默认Toggle): boolean | undefined {
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

function detect沙箱Starter(raw: string): string {
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

function getUnsupported沙箱StructuredMessages(raw: string): string[] {
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
  const sandboxDraft = extract沙箱Draft(agent?.sandbox);

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
    agentToAgentMode: 'inherit',
    agentToAgentAllow: '',
    sessionVisibility: '',
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
      headline: `消息会回落给默认智能体「${agentID}」`,
      detail: '本次输入没有命中任何显式路由规则，所以系统使用默认智能体 兜底。',
      ruleLabel: '默认回落',
    };
  }
  const ruleLabel = OFFICIAL_MATCHED_BY_LABELS[result.matchedBy || ''] || result.matchedBy || '显式规则';
  const index = typeof result.matchedIndex === 'number' && result.matchedIndex >= 0 ? result.matchedIndex : -1;
  const binding = index >= 0 ? bindings[index] : undefined;
  const bindingLabel = binding?.comment?.trim() || (index >= 0 ? `规则 ${index + 1}` : '显式规则');
  return {
    headline: `消息会交给 Agent「${agentID}」`,
    detail: index >= 0
      ? `命中了「${bindingLabel}」的「${ruleLabel}」优先级。`
      : `命中优先级：${ruleLabel}。`,
    ruleLabel: bindingLabel,
  };
}

function AgentsPage() {
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const modern = uiMode === 'modern';
  const [searchParams] = useSearchParams();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState('');
  const [identityImportMsg, setIdentityImportMsg] = useState('');
  const [edition, setEdition] = useState<'lite' | 'pro'>('pro');

  const [workbenchView, setWorkbenchView] = useState<AgentsWorkbenchView>('directory');
  const [selectedAgentId, setSelectedAgentId] = useState('');
  const [detailTab, setDetailTab] = useState<AgentDetailTab>('overview');
  const [expandedBindingIndex, setExpandedBindingIndex] = useState<number | null>(null);

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
  const [skillsContext, setSkillsContext] = useState<AgentSkillsContext>({ agentId: '', workspace: '' });
  const [skillsLoading, setSkillsLoading] = useState(false);
  const [cronJobs, setCronJobs] = useState<CronJob[]>([]);
  const [coreFilesByAgent, setCoreFilesByAgent] = useState<Record<string, AgentCoreFileEntry[]>>({});
  const [coreFilesStateByAgent, setCoreFilesStateByAgent] = useState<Record<string, CoreFilesLoadState>>({});
  const [coreFilesLoading, setCoreFilesLoading] = useState(false);
  const [selectedCoreFileName, setSelectedCoreFileName] = useState('AGENTS.md');
  const [coreFileDraft, setCoreFileDraft] = useState('');
  const [coreFileTouched, setCoreFileTouched] = useState(false);
  const [coreFileSaving, setCoreFileSaving] = useState(false);
  const coreFileDraftResetRef = useRef(false);
  const [sessionsByAgent, setSessionsByAgent] = useState<Record<string, SessionInfo[]>>({});
  const [sessionsLoading, setSessionsLoading] = useState(false);

  const [editingId, setEditingId] = useState<string | null>(null);
  const [materializingImplicitAgent, setMaterializingImplicitAgent] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [formSection, setFormSection] = useState<AgentFormSection>('basic');
  const [form, setForm] = useState<AgentFormState>(DEFAULT_AGENT_FORM);
  const [sandboxClearIntent, set沙箱ClearIntent] = useState(false);
  const [saveAttempted, setSaveAttempted] = useState(false);
  const [structuredTouched, setStructuredTouched] = useState<AgentStructuredTouchedState>(DEFAULT_AGENT_STRUCTURED_TOUCHED);

  const [previewMeta, setPreviewMeta] = useState<Record<PreviewMetaKey, string>>(DEFAULT_PREVIEW_META);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [previewResult, setPreviewResult] = useState<PreviewResult | null>(null);

  const requestedAgentId = searchParams.get('agent')?.trim() || '';
  const requestedView = searchParams.get('view')?.trim() || '';
  const requestedTab = searchParams.get('tab')?.trim() || '';

  const agentOptions = useMemo(() => {
    return agents.map(a => a.id).filter(Boolean);
  }, [agents]);
  const duplicateAccountBindings = useMemo(() => detectDuplicateAccountBindings(bindings), [bindings]);

  const explicitAgents = useMemo(() => {
    return agents.filter(agent => !isImplicitAgent(agent));
  }, [agents]);

  const displayWorkspacePath = (workspace?: string) => {
    const value = (workspace || '').trim();
    if (value) return value;
    return edition === 'lite' ? LITE_WORKSPACE_ROOT : '未设置';
  };

  const displayAgentDirPath = (agentDir?: string, agentId?: string) => {
    const value = (agentDir || '').trim();
    if (value) return value;
    const resolvedAgentId = (agentId || '').trim();
    if (edition === 'lite' && resolvedAgentId) return `${LITE_AGENT_ROOT}/${resolvedAgentId}`;
    return edition === 'lite' ? LITE_AGENT_ROOT : '未设置';
  };

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
      return detect沙箱Starter(stringifyJSON(buildStructured沙箱Preview(form)));
    }
    return detect沙箱Starter(form.sandboxText);
  }, [form, structuredTouched.sandbox]);
  const sandboxStructuredIssues = useMemo(() => getUnsupported沙箱StructuredMessages(form.sandboxText), [form.sandboxText]);
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
      .filter(item => item.binding.agentId === selectedAgent.id);
  }, [bindings, selectedAgent]);
  const selectedAgentCoreFiles = useMemo(() => {
    if (!selectedAgent) return [];
    const files = coreFilesByAgent[selectedAgent.id] || [];
    return [...files].sort((a, b) => {
      if (a.exists !== b.exists) return a.exists ? -1 : 1;
      if (a.name === 'IDENTITY.md') return -1;
      if (b.name === 'IDENTITY.md') return 1;
      return a.name.localeCompare(b.name);
    });
  }, [coreFilesByAgent, selectedAgent]);
  const selectedCoreFile = useMemo(() => {
    if (selectedAgentCoreFiles.length === 0) return null;
    return selectedAgentCoreFiles.find(file => file.name === selectedCoreFileName) || selectedAgentCoreFiles[0];
  }, [selectedAgentCoreFiles, selectedCoreFileName]);
  const selectedCoreFilesState = useMemo(() => {
    if (!selectedAgent) return undefined;
    return coreFilesStateByAgent[selectedAgent.id];
  }, [coreFilesStateByAgent, selectedAgent]);
  const selectedHeartbeatFile = useMemo(() => {
    return selectedAgentCoreFiles.find(file => file.name === 'HEARTBEAT.md') || null;
  }, [selectedAgentCoreFiles]);
  const coreFileDirty = useMemo(() => {
    if (!coreFileTouched) return false;
    if (!selectedCoreFile) return coreFileDraft.trim().length > 0;
    return coreFileDraft !== (selectedCoreFile.content || '');
  }, [coreFileDraft, coreFileTouched, selectedCoreFile]);
  const selectedAgentCronJobs = useMemo(() => {
    if (!selectedAgent) return [];
    return cronJobs.filter(job => resolveCronJobAgentID(job, defaultAgent || 'main') === selectedAgent.id);
  }, [cronJobs, defaultAgent, selectedAgent]);
  const selectedAgentSessions = useMemo(() => {
    if (!selectedAgent) return [];
    return sessionsByAgent[selectedAgent.id] || [];
  }, [sessionsByAgent, selectedAgent]);
  const routingStats = useMemo(() => {
    const activeAgents = new Set(bindings.map(item => item.agentId).filter(Boolean));
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
  const selectedSkillSourceSummary = useMemo(() => {
    const counts = {
      workspace: 0,
      shared: 0,
      bundled: 0,
      plugin: 0,
      extra: 0,
      other: 0,
    };
    skills.forEach(skill => {
      counts[summarizeSkillSourceGroup(skill.source)] += 1;
    });
    return [
      { key: 'workspace', label: '当前 Agent workspace', count: counts.workspace },
      { key: 'shared', label: '共享 skills', count: counts.shared },
      { key: 'bundled', label: 'bundled', count: counts.bundled },
      { key: 'plugin', label: 'plugin skills', count: counts.plugin },
      { key: 'extra', label: 'extraDirs', count: counts.extra },
      { key: 'other', label: '其他来源', count: counts.other },
    ].filter(item => item.count > 0);
  }, [skills]);
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
      })
      .filter(channel => channel.routeCount > 0);
  }, [channelConfigs, channelMeta, selectedAgentBindings]);
  const selectedAgentIsDefault = selectedAgent?.id === defaultAgent;
  const previewExplanation = useMemo(() => buildPreviewExplanation(previewResult, bindings, defaultAgent), [previewResult, bindings, defaultAgent]);
  const effectiveIsDefault = firstExplicitAgentWillBecomeDefault ? true : form.isDefault;
  const editingBaseAgent = useMemo(() => {
    const targetID = (editingId || form.id || '').trim();
    if (!targetID) return undefined;
    return agents.find(agent => agent.id === targetID);
  }, [agents, editingId, form.id]);
  const selectedAvatarPreview = useMemo(() => {
    const avatar = extractIdentityDraft(selectedAgent?.identity).avatar;
    return resolveAvatarPreviewSrc(selectedAgent?.id, avatar);
  }, [selectedAgent]);
  const avatarNeedsStrictValidation = useMemo(() => {
    const currentAvatar = extractIdentityDraft(editingBaseAgent?.identity).avatar.trim();
    const nextAvatar = form.identityAvatar.trim();
    if (!editingBaseAgent) return nextAvatar !== '';
    return nextAvatar !== currentAvatar;
  }, [editingBaseAgent, form.identityAvatar]);
  const formAvatarValidationError = useMemo(() => {
    if (!avatarNeedsStrictValidation) return '';
    return validateAvatarValue(form.identityAvatar);
  }, [avatarNeedsStrictValidation, form.identityAvatar]);
  const formAvatarPreview = useMemo(() => {
    const agentId = (editingId || form.id || '').trim();
    return resolveAvatarPreviewSrc(agentId || undefined, form.identityAvatar);
  }, [editingId, form.id, form.identityAvatar]);
  const saveBlockedReason = useMemo(() => {
    if (agentIDError) return agentIDError;
    if (workspaceConflict) return `workspace 已被 Agent “${workspaceConflict.id}” 使用`;
    if (agentDirConflict) return `agentDir 已被 Agent “${agentDirConflict.id}” 使用`;
    if (formAvatarValidationError) return formAvatarValidationError;
    return '';
  }, [agentDirConflict, agentIDError, workspaceConflict, formAvatarValidationError]);
  const showAgentIDError = saveAttempted || !isBlankAgentID;
  const footerValidationMessage = useMemo(() => {
    if (!saveBlockedReason) return '保存会继续沿用现有后端接口与 payload 格式；bindings 与路由预览不会受到影响。';
    if (isBlankAgentID && !saveAttempted) return '完成 Basic 中的 Agent ID 后即可保存。';
    return saveBlockedReason;
  }, [isBlankAgentID, saveAttempted, saveBlockedReason]);
  const footerValidationIsError = !!saveBlockedReason && !(isBlankAgentID && !saveAttempted);
  const pageClass = modern ? 'page-modern' : '';
  const panelClass = modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 rounded-xl border border-gray-100 dark:border-gray-700/50 shadow-sm';
  const softPanelClass = modern ? 'page-modern-panel p-4 bg-white/70 dark:bg-slate-900/70' : 'rounded-xl border border-gray-100 dark:border-gray-700 p-4';
  const panelMutedClass = modern ? 'rounded-xl border border-sky-200/60 bg-sky-500/10 dark:border-sky-800/60 dark:bg-sky-500/10' : 'rounded-xl border border-sky-100 bg-sky-50 dark:border-sky-900/40 dark:bg-sky-950/20';
  const controlButtonClass = modern ? 'page-modern-control px-3 py-2 text-xs font-medium' : 'flex items-center gap-2 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 transition-colors shadow-sm';
  const accentButtonClass = modern ? 'page-modern-accent px-4 py-2 text-xs font-medium disabled:opacity-50' : 'flex items-center gap-2 px-4 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 shadow-sm shadow-violet-200 dark:shadow-none transition-all disabled:opacity-50';
  const actionButtonClass = modern ? 'page-modern-action px-3 py-2 text-xs font-medium' : 'px-3 py-2 text-xs font-medium rounded-lg bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600';
  const dangerButtonClass = modern ? 'page-modern-danger px-3 py-2 text-xs font-medium disabled:opacity-50 disabled:cursor-not-allowed' : 'px-3 py-2 text-xs font-medium rounded-lg bg-red-50 text-red-600 hover:bg-red-100 disabled:opacity-50 disabled:cursor-not-allowed';

  const touchStructured = (key: keyof AgentStructuredTouchedState) => {
    setStructuredTouched(prev => (prev[key] ? prev : { ...prev, [key]: true }));
  };

  const updateForm = (patch: Partial<AgentFormState>, touchKey?: keyof AgentStructuredTouchedState) => {
    setForm(prev => ({ ...prev, ...patch }));
    if (touchKey) touchStructured(touchKey);
  };

  const update沙箱Form = (patch: Partial<AgentFormState>) => {
    updateForm(patch, 'sandbox');
    set沙箱ClearIntent(false);
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
    set沙箱ClearIntent(false);
    setSaveAttempted(false);
    setStructuredTouched(DEFAULT_AGENT_STRUCTURED_TOUCHED);
    setIdentityImportMsg('');
  };

  const skillsLoadSeqRef = useRef(0);

  const loadData = async () => {
    setLoading(true);
    try {
      const [agentsRes, channelsRes, modelsRes, cronRes] = await Promise.all([
        api.getAgentsConfig(),
        api.getChannels(),
        api.getModels(),
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
        setExpandedBindingIndex(incomingBindings.length > 0 ? 0 : null);
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
        setExpandedBindingIndex(null);
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
      setExpandedBindingIndex(null);
      setChannelConfigs({});
      setChannelMeta({});
      setModelOptions([]);
      setDefaultModelHint('');
      setSkills([]);
      setSkillsContext({ agentId: '', workspace: '' });
      setCronJobs([]);
      setCoreFilesStateByAgent({});
    } finally {
      setLoading(false);
    }
  };

  const loadAgentCoreFiles = async (agentId: string, force = false) => {
    if (!agentId) return;
    if (!force && coreFilesByAgent[agentId]) return;
    setCoreFilesLoading(true);
    try {
      const workspaceOverride = editingId === agentId ? form.workspace.trim() || undefined : undefined;
      const response = await api.getAgentCoreFiles(agentId, workspaceOverride);
      if (response?.ok) {
        setCoreFilesByAgent(prev => ({ ...prev, [agentId]: response.files || [] }));
        setCoreFilesStateByAgent(prev => ({
          ...prev,
          [agentId]: {
            kind: 'ready',
            workspace: String(response.workspace || '').trim() || undefined,
          },
        }));
      } else if (response?.error) {
        setCoreFilesStateByAgent(prev => ({
          ...prev,
          [agentId]: classifyCoreFilesLoadState(String(response.error || ''), String(response.workspace || '').trim() || undefined),
        }));
        setMsg(`加载核心文件失败: ${response.error}`);
        setTimeout(() => setMsg(''), 4000);
      }
    } catch (err) {
      setCoreFilesStateByAgent(prev => ({
        ...prev,
        [agentId]: classifyCoreFilesLoadState(String(err)),
      }));
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
    api.getPanelVersion().then(r => {
      if (r.ok && (r.edition === 'lite' || r.edition === 'pro')) setEdition(r.edition);
    }).catch(() => {});
  }, []);

  useEffect(() => {
    loadData();
  }, []);

  useEffect(() => {
    if (!selectedAgent?.id) {
      setSkills([]);
      setSkillsContext({ agentId: '', workspace: '' });
      setSkillsLoading(false);
      return;
    }
    const seq = ++skillsLoadSeqRef.current;
    const agentID = selectedAgent.id;
    setSkillsLoading(true);
    setSkills([]);
    setSkillsContext({ agentId: agentID, workspace: String(selectedAgent.workspace || '').trim() });
    api.getSkills(agentID)
      .then(res => {
        if (seq !== skillsLoadSeqRef.current) return;
        if (res?.ok) {
          setSkills((res.skills || []) as SkillEntry[]);
          setSkillsContext({
            agentId: String(res.agentId || agentID).trim() || agentID,
            workspace: String(res.workspace || selectedAgent.workspace || '').trim(),
          });
          return;
        }
        setSkills([]);
        setSkillsContext({ agentId: agentID, workspace: String(selectedAgent.workspace || '').trim() });
      })
      .catch(() => {
        if (seq !== skillsLoadSeqRef.current) return;
        setSkills([]);
        setSkillsContext({ agentId: agentID, workspace: String(selectedAgent.workspace || '').trim() });
      })
      .finally(() => {
        if (seq === skillsLoadSeqRef.current) setSkillsLoading(false);
      });
  }, [selectedAgent]);

  useEffect(() => {
    if (requestedView !== 'routing' && requestedView !== 'directory') return;
    setWorkbenchView(prev => (prev === requestedView ? prev : requestedView));
  }, [requestedView]);

  useEffect(() => {
    const allowedTabs: AgentDetailTab[] = ['overview', 'model', 'tools', 'files', 'capabilities', 'context', 'advanced'];
    if (!allowedTabs.includes(requestedTab as AgentDetailTab)) return;
    setDetailTab(prev => (prev === requestedTab ? prev : requestedTab as AgentDetailTab));
  }, [requestedTab]);

  useEffect(() => {
    if (!requestedAgentId || agents.length === 0) return;
    if (!agents.some(agent => agent.id === requestedAgentId)) return;
    if (requestedAgentId !== selectedAgentId) setSelectedAgentId(requestedAgentId);
  }, [agents, requestedAgentId, selectedAgentId]);

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
    if (expandedBindingIndex !== null && expandedBindingIndex >= bindings.length) {
      setExpandedBindingIndex(bindings.length - 1);
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
      loadAgentCoreFiles(selectedAgent.id, true);
    }
    if (detailTab === 'capabilities' || detailTab === 'context') {
      loadAgentSessions(selectedAgent.id);
    }
  }, [detailTab, selectedAgent?.id]);

  useEffect(() => {
    const allowReset = coreFileDraftResetRef.current || !coreFileDirty;
    if (selectedAgentCoreFiles.length === 0) {
      if (!allowReset) return;
      setSelectedCoreFileName('IDENTITY.md');
      setCoreFileDraft('');
      setCoreFileTouched(false);
      coreFileDraftResetRef.current = false;
      return;
    }
    const preferredFile = selectedAgentCoreFiles.find(file => file.name === 'IDENTITY.md' && file.exists) || selectedAgentCoreFiles.find(file => file.exists) || selectedAgentCoreFiles.find(file => file.name === 'IDENTITY.md');
    const currentSelected = selectedAgentCoreFiles.find(file => file.name === selectedCoreFileName);
    const nextSelected = (!currentSelected || !currentSelected.exists) ? (preferredFile || selectedAgentCoreFiles[0]) : currentSelected;
    const nextContent = nextSelected.content || '';
    if (!allowReset && (nextSelected.name !== selectedCoreFileName || nextContent !== coreFileDraft)) return;
    if (nextSelected.name !== selectedCoreFileName) {
      setSelectedCoreFileName(nextSelected.name);
    }
    if (coreFileDraft !== nextContent) {
      setCoreFileDraft(nextContent);
    }
    setCoreFileTouched(false);
    coreFileDraftResetRef.current = false;
  }, [coreFileDirty, coreFileDraft, selectedAgentCoreFiles, selectedCoreFileName]);

  const openCreate = (section: AgentFormSection = 'basic') => {
    setMsg('');
    setIdentityImportMsg('');
    setMaterializingImplicitAgent(false);
    set沙箱ClearIntent(false);
    setSaveAttempted(false);
    setStructuredTouched(DEFAULT_AGENT_STRUCTURED_TOUCHED);
    setEditingId(null);
    setFormSection(section);
    setForm(createAgentFormState());
    setShowForm(true);
  };

  const openEdit = (agent: AgentItem, section: AgentFormSection = 'basic') => {
    setMsg('');
    setIdentityImportMsg('');
    const implicitAgent = isImplicitAgent(agent);
    setMaterializingImplicitAgent(implicitAgent);
    set沙箱ClearIntent(false);
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

  const apply沙箱Starter = (starterKey: string) => {
    const starter = SANDBOX_STARTERS.find(item => item.key === starterKey);
    if (!starter) return;
    const sandboxObj = parseJSONText(starter.text, 'sandbox');
    const draft = extract沙箱Draft(sandboxObj);
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
    set沙箱ClearIntent(starterKey === 'inherit');
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
      const sandboxDraft = extract沙箱Draft(sandboxObj);
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
        agentToAgentMode: 'inherit',
        agentToAgentAllow: '',
        sessionVisibility: '',
        subagentAllowAgents: isPlainObject(subagentsObj) ? parseStringList(getNestedValue(subagentsObj, 'allowAgents')).join(', ') : '',
      }));
      set沙箱ClearIntent(false);
      setMsg('已从 高级 JSON 同步结构化字段');
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
      setMsg('当前 sandbox 含有结构化表单不支持的值，请先在 高级 JSON 中调整。');
      return;
    }
    const id = form.id.trim();
    if (!id) {
      setMsg('Agent ID 不能为空');
      return;
    }
    let modelObj: any;
    let toolsObj: any;
    let sandboxObj: any;
    let groupChatObj: any;
    let identityObj: any;
    let subagentsObj: any;
    let paramsObj: any;
    let runtimeObj: any;

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

      if (structuredTouched.identity) {
        if (identityObj !== undefined && !isPlainObject(identityObj)) {
          throw new Error('identity JSON 必须是对象才能与结构化字段合并');
        }
        const nextIdentity = isPlainObject(identityObj) ? deepClone(identityObj) : {};
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
        delete nextTools.agentToAgent;
        delete nextTools.sessions;
        toolsObj = cleanupObject(nextTools);
      }

      if (structuredTouched.sandbox) {
        if (sandboxObj !== undefined && !isPlainObject(sandboxObj)) {
          throw new Error('sandbox JSON 必须是对象才能与结构化字段合并');
        }
        const next沙箱 = isPlainObject(sandboxObj) ? deepClone(sandboxObj) : {};
        if (form.sandboxMode) next沙箱.mode = form.sandboxMode;
        else delete next沙箱.mode;
        if (form.sandboxScope) next沙箱.scope = form.sandboxScope;
        else delete next沙箱.scope;
        if (form.sandboxWorkspaceAccess) next沙箱.workspaceAccess = form.sandboxWorkspaceAccess;
        else delete next沙箱.workspaceAccess;
        if (form.sandboxWorkspaceRoot.trim()) next沙箱.workspaceRoot = form.sandboxWorkspaceRoot.trim();
        else delete next沙箱.workspaceRoot;

        const nextDocker = isPlainObject(next沙箱.docker) ? next沙箱.docker : {};
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
        if (Object.keys(nextDocker).length > 0) next沙箱.docker = nextDocker;
        else delete next沙箱.docker;
        sandboxObj = cleanupObject(next沙箱);
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
    const clears沙箱Override = structuredTouched.sandbox && sandboxObj === undefined && form.sandboxText.trim() !== '' && !!editingId;
    const clearsToolsOverride = structuredTouched.tools && toolsObj === undefined && form.toolsText.trim() !== '' && !!editingId;
    if (modelObj !== undefined) payload.model = modelObj;
    if (clearsToolsOverride && editingId) payload.tools = null;
    else if (toolsObj !== undefined) payload.tools = toolsObj;
    if (clears沙箱Override && editingId) payload.sandbox = null;
    else if (sandboxObj !== undefined) payload.sandbox = sandboxObj;
    if (groupChatObj !== undefined) payload.groupChat = groupChatObj;
    if (identityObj !== undefined) payload.identity = identityObj;
    if (subagentsObj !== undefined) payload.subagents = subagentsObj;
    if (paramsObj !== undefined) payload.params = paramsObj;
    if (runtimeObj !== undefined) payload.runtime = runtimeObj;

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
    setBindingAt(idx, row => {
      const cur = deepClone(row.match || {});
      const now = extractPeerForm(cur, key);
      const kind = part === 'kind' ? value.trim() : now.kind;
      const id = part === 'id' ? value.trim() : now.id;
      if (!kind && !id) {
        delete cur[key];
        const nextMatch = compactMatch(cur);
        return { ...row, match: nextMatch, matchText: JSON.stringify(nextMatch, null, 2), rowError: '' };
      }
      cur[key] = { kind, id };
      const nextMatch = compactMatch(cur);
      return { ...row, match: nextMatch, matchText: JSON.stringify(nextMatch, null, 2), rowError: '' };
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
      if (!row.agentId.trim()) {
        setMsg(`第 ${i + 1} 条 binding 缺少 agentId`);
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

      const acpMode = row.acp.mode.trim();
      if (row.type === 'acp' && acpMode && !['persistent', 'oneshot'].includes(acpMode)) {
        const error = `第 ${i + 1} 条 binding 的 acp.mode 仅支持 persistent / oneshot`;
        nextBindings[i] = { ...row, rowError: error };
        setBindings(nextBindings);
        setMsg(error);
        return;
      }

      const payload: any = {
        agentId: row.agentId.trim(),
        comment: row.comment.trim() || undefined,
        match: matchObj,
      };
      if (row.enabled === false) payload.enabled = false;
      if (row.type === 'acp') {
        payload.type = 'acp';
        const acp = cleanupObject({
          mode: row.acp.mode.trim() || undefined,
          label: row.acp.label.trim() || undefined,
          cwd: row.acp.cwd.trim() || undefined,
          backend: row.acp.backend.trim() || undefined,
        });
        if (acp) payload.acp = acp;
      }
      parsed.push(payload);
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
        type: 'route',
        agentId: defaultAgent || agentOptions[0] || 'main',
        comment: '',
        enabled: true,
        match,
        acp: { mode: '', label: '', cwd: '', backend: '' },
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

  const importIdentityFromCoreFile = async () => {
    const agentId = (editingId || form.id || '').trim();
    if (!agentId) {
      const text = '请先填写 Agent ID，或在已有 Agent 上使用该导入功能。';
      setMsg(text);
      setIdentityImportMsg(text);
      setTimeout(() => { setMsg(''); setIdentityImportMsg(''); }, 4000);
      return;
    }
    try {
      let files = coreFilesByAgent[agentId];
      if (!files) {
        const response = await api.getAgentCoreFiles(agentId, form.workspace.trim() || undefined);
        if (!response?.ok) {
          const error = String(response?.error || '无法读取 IDENTITY.md');
          setCoreFilesStateByAgent(prev => ({ ...prev, [agentId]: classifyCoreFilesLoadState(error, String(response?.workspace || '').trim() || undefined) }));
          const text = `导入失败: ${error}`;
          setMsg(text);
          setIdentityImportMsg(text);
          setTimeout(() => { setMsg(''); setIdentityImportMsg(''); }, 4000);
          return;
        }
        files = response.files || [];
        setCoreFilesByAgent(prev => ({ ...prev, [agentId]: files || [] }));
        setCoreFilesStateByAgent(prev => ({
          ...prev,
          [agentId]: { kind: 'ready', workspace: String(response.workspace || '').trim() || undefined },
        }));
      }
      const identityFile = (files || []).find((file: AgentCoreFileEntry) => file.name === 'IDENTITY.md');
      if (!identityFile?.content?.trim()) {
        const text = '未找到可导入的 IDENTITY.md 内容。';
        setMsg(text);
        setIdentityImportMsg(text);
        setTimeout(() => { setMsg(''); setIdentityImportMsg(''); }, 4000);
        return;
      }
      const parsed = parseIdentityMarkdown(identityFile.content);
      if (!parsed.name && !parsed.theme && !parsed.creature && !parsed.vibe && !parsed.emoji && !parsed.avatar) {
        const text = 'IDENTITY.md 中未解析出 Name / Theme / Creature / Vibe / Emoji / Avatar。';
        setMsg(text);
        setIdentityImportMsg(text);
        setTimeout(() => { setMsg(''); setIdentityImportMsg(''); }, 4000);
        return;
      }
      updateForm({
        identityName: parsed.name || form.identityName,
        identityTheme: parsed.theme || parsed.creature || parsed.vibe || form.identityTheme,
        identityEmoji: parsed.emoji || form.identityEmoji,
        identityAvatar: parsed.avatar || form.identityAvatar,
      }, 'identity');
      const text = '已从 IDENTITY.md 导入可识别字段';
      setMsg(text);
      setIdentityImportMsg(text);
      setTimeout(() => { setMsg(''); setIdentityImportMsg(''); }, 3000);
    } catch (err) {
      const text = '导入失败: ' + String(err);
      setMsg(text);
      setIdentityImportMsg(text);
      setTimeout(() => { setMsg(''); setIdentityImportMsg(''); }, 4000);
    }
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
  const selectedIdentity = extractIdentityDraft(selectedAgent?.identity);
  const selectedTools = isPlainObject(selectedAgent?.tools) ? selectedAgent.tools : {};
  const selectedSubagents = isPlainObject(selectedAgent?.subagents) ? selectedAgent.subagents : {};
  const selectedGroupChat = isPlainObject(selectedAgent?.groupChat) ? selectedAgent.groupChat : {};
  const selected沙箱Draft = extract沙箱Draft(selectedAgent?.sandbox);
  const selectedAdvancedBlocks = summarizeAdvancedBlocks(selectedAgent);
  const selected路由Count = selectedAgentBindings.length;
  const selectedToolAllow = parseStringList(getNestedValue(selectedTools, 'allow'));
  const selectedToolDeny = parseStringList(getNestedValue(selectedTools, 'deny'));
  const selectedToolProfile = String(getNestedValue(selectedTools, 'profile') || '').trim();

  if (loading) {
    return (
      <div className={`py-16 text-center text-gray-400 text-sm ${pageClass}`}>
        <RefreshCw size={18} className="animate-spin inline mr-2" />
        加载中...
      </div>
    );
  }

  return (
    <div className={`space-y-6 ${pageClass}`}>
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

      <div className={modern ? 'page-modern-header' : 'flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between'}>
        <div>
          <h2 className={modern ? 'page-modern-title text-xl' : 'text-xl font-bold text-gray-900 dark:text-white'}>智能体</h2>
          <p className={modern ? 'page-modern-subtitle text-sm mt-1' : 'text-sm text-gray-500 mt-1'}>在这里维护智能体配置、查看核心文件，并快速检查一条消息最终会交给谁处理。</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button onClick={loadData} className={controlButtonClass}>
            <RefreshCw size={14} /> 刷新
          </button>
          <button onClick={() => openCreate('basic')} className={accentButtonClass}>
            <Plus size={14} /> 新建智能体
          </button>
        </div>
      </div>

      {msg && (
        <div className={`px-4 py-3 rounded-lg text-sm ${msg.includes('失败') || msg.includes('错误') ? 'bg-red-50 dark:bg-red-900/20 text-red-600' : 'bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600'}`}>
          {msg}
        </div>
      )}

      <div className={`${modern ? 'page-modern-panel p-4' : 'rounded-xl border border-violet-100 dark:border-violet-900/40 bg-violet-50/80 dark:bg-violet-950/20 p-4'}`}>
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <p className={`text-xs font-semibold tracking-wide uppercase ${modern ? 'text-sky-700 dark:text-sky-300' : 'text-violet-700 dark:text-violet-300'}`}>使用方式</p>
            <h3 className="text-base font-semibold text-gray-900 dark:text-white mt-1">先管理单个智能体，再检查路由命中</h3>
            <p className="text-sm text-gray-600 dark:text-gray-300 mt-1">
              “智能体目录”适合查看单个智能体的配置、文件与上下文；“路由工作台”适合检查规则、模拟消息并确认最终分流结果。
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              onClick={() => setWorkbenchView('directory')}
              className={`inline-flex items-center gap-2 rounded-lg border px-3 py-2 text-xs font-medium transition-colors ${
                workbenchView === 'directory'
                  ? (modern ? 'border-sky-500 bg-sky-500 text-white shadow-lg shadow-sky-500/20' : 'border-violet-600 bg-violet-600 text-white')
                  : (modern ? 'border-sky-100/70 dark:border-slate-700 bg-white/80 dark:bg-slate-900/70 text-slate-600 dark:text-slate-300 hover:border-sky-300' : 'border-white/80 dark:border-gray-700 bg-white dark:bg-gray-900 text-gray-600 dark:text-gray-300 hover:border-violet-300')
              }`}
            >
              <Bot size={14} />
                智能体目录
            </button>
            <button
              onClick={() => setWorkbenchView('routing')}
              className={`inline-flex items-center gap-2 rounded-lg border px-3 py-2 text-xs font-medium transition-colors ${
                workbenchView === 'routing'
                  ? (modern ? 'border-sky-500 bg-sky-500 text-white shadow-lg shadow-sky-500/20' : 'border-violet-600 bg-violet-600 text-white')
                  : (modern ? 'border-sky-100/70 dark:border-slate-700 bg-white/80 dark:bg-slate-900/70 text-slate-600 dark:text-slate-300 hover:border-sky-300' : 'border-white/80 dark:border-gray-700 bg-white dark:bg-gray-900 text-gray-600 dark:text-gray-300 hover:border-violet-300')
              }`}
            >
              <Route size={14} />
                路由工作台
            </button>
          </div>
        </div>
      </div>

      {workbenchView === 'directory' ? (
        <div className="grid grid-cols-1 xl:grid-cols-[320px,1fr] gap-6">
          <div className={panelClass}>
            <div className="px-4 py-3 border-b border-gray-100 dark:border-gray-700/50 flex items-center justify-between">
              <div>
                <h3 className="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                  <Bot size={15} className="text-violet-500" />
                  智能体目录
                </h3>
                <p className="text-[11px] text-gray-500 mt-1">先选智能体，再看它的配置、核心文件、能力快照与路由上下文。</p>
              </div>
              <span className="text-xs text-gray-500">默认：{defaultAgent}</span>
            </div>
            <div className="p-4 space-y-3">
              {agents.length === 0 ? (
                <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-8 text-center text-sm text-gray-400">
                    还没有显式 Agent，先创建一个吧。
                </div>
              ) : (
                agents.map(agent => {
                  const implicitAgent = isImplicitAgent(agent);
                  const active = selectedAgent?.id === agent.id;
                  const identity = isPlainObject(agent.identity) ? agent.identity : {};
                  const modelDraft = extractModelDraft(agent.model);
                  const cardTitle = String(agent.name || identity.name || '').trim() || agent.id;
                  const routeCount = bindings.filter(item => item.agentId === agent.id).length;
                  return (
                    <div
                      key={agent.id}
                      className={`rounded-xl border p-3 transition-colors ${
                        active
                          ? (modern ? 'border-sky-300 bg-sky-50/90 dark:bg-sky-950/20 dark:border-sky-800/60 shadow-sm' : 'border-violet-300 bg-violet-50/80 dark:bg-violet-950/20 dark:border-violet-800/60')
                          : (modern ? 'border-slate-200/70 dark:border-slate-700/80 bg-white/60 dark:bg-slate-900/40 hover:border-sky-200 dark:hover:border-sky-800/60' : 'border-gray-100 dark:border-gray-700 hover:border-violet-200 dark:hover:border-violet-800/60')
                      }`}
                    >
                      <button
                        onClick={() => {
                          if (agent.id !== selectedAgentId && !confirmDiscardCoreFileDraft('并切换到另一个 Agent')) return;
                          setSelectedAgentId(agent.id);
                          setDetailTab('overview');
                        }}
                        className="w-full min-w-0 text-left"
                      >
                        <div className="flex items-start justify-between gap-2 min-w-0">
                          <div>
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="font-medium text-sm text-gray-900 dark:text-white">{cardTitle}</span>
                              {agent.default && <span className={`text-[10px] px-1.5 py-0.5 rounded ${modern ? 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-200' : 'bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-200'}`}>默认</span>}
                              {implicitAgent && <span className="text-[10px] px-1.5 py-0.5 rounded bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-200">占位</span>}
                            </div>
                            <div className="mt-1 font-mono text-[11px] text-gray-500">{agent.id}</div>
                          </div>
                          <span className="text-[11px] text-gray-400">{agent.sessions ?? 0} 会话</span>
                        </div>
                        <div className="mt-3 grid grid-cols-2 gap-2 text-[11px] text-gray-500">
                          <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-2.5 py-2">
                              <div className="text-gray-400">模型</div>
                            <div className="mt-1 truncate text-gray-700 dark:text-gray-200">{modelDraft.primary || defaultModelHint || '继承默认'}</div>
                          </div>
                          <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-2.5 py-2">
                              <div className="text-gray-400">路由规则</div>
                            <div className="mt-1 text-gray-700 dark:text-gray-200">{routeCount > 0 ? `${routeCount} 条` : '未绑定'}</div>
                          </div>
                        </div>
                        <div className="mt-3 space-y-1 text-[11px] text-gray-500">
                          <div className="min-w-0">工作区：<span className="font-mono text-gray-700 dark:text-gray-200 break-all">{displayWorkspacePath(agent.workspace)}</span></div>
                          <div>最后活跃：<span className="text-gray-700 dark:text-gray-200">{formatLastActive(agent.lastActive)}</span></div>
                        </div>
                      </button>
                      <div className="mt-3 flex items-center gap-2">
                        <button onClick={() => openEdit(agent)} className={actionButtonClass}>
                           {implicitAgent ? '配置' : '编辑'}
                        </button>
                        <button
                          disabled={implicitAgent}
                          title={implicitAgent ? '运行时占位 Agent 无需删除' : ''}
                          onClick={() => deleteAgent(agent)}
                          className={dangerButtonClass}
                        >
                           删除
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
                <div className={`${panelClass} p-5`}>
                  <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                    <div className="space-y-3">
                      <div>
                        <div className="flex flex-wrap items-center gap-2">
                          <h3 className="text-lg font-semibold text-gray-900 dark:text-white">{String(selectedAgent.name || selectedIdentity.name || '').trim() || selectedAgent.id}</h3>
                            {selectedAgent.default && <span className={`text-[10px] px-1.5 py-0.5 rounded ${modern ? 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-200' : 'bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-200'}`}>默认智能体</span>}
                          {isImplicitAgent(selectedAgent) && <span className="text-[10px] px-1.5 py-0.5 rounded bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-200">运行时占位</span>}
                        </div>
                        <p className="mt-1 text-sm text-gray-500">
                          <span className="font-mono">{selectedAgent.id}</span>
                          {selectedIdentity.theme ? ` · ${selectedIdentity.theme}` : ''}
                        </p>
                      </div>
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 px-3 py-3 bg-gray-50/80 dark:bg-gray-900">
                          <div className="text-gray-400">模型</div>
                          <div className="mt-1 font-medium text-gray-900 dark:text-white">{selectedModelDraft.primary || defaultModelHint || '继承默认'}</div>
                          <div className="mt-1 text-gray-500">回退：{selectedModelDraft.fallbacks || '未设置'}</div>
                        </div>
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 px-3 py-3 bg-gray-50/80 dark:bg-gray-900">
                          <div className="text-gray-400">工具与权限</div>
                          <div className="mt-1 font-medium text-gray-900 dark:text-white">{describe沙箱Mode(selectedAgent.sandbox)}</div>
                          <div className="mt-1 text-gray-500">配置：{String(getNestedValue(selectedTools, 'profile') || '未设置')}</div>
                        </div>
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 px-3 py-3 bg-gray-50/80 dark:bg-gray-900">
                          <div className="text-gray-400">路由上下文</div>
                          <div className="mt-1 font-medium text-gray-900 dark:text-white">{selected路由Count > 0 ? `${selected路由Count} 条规则指向它` : '暂无显式规则'}</div>
                          <div className="mt-1 text-gray-500">{selectedAgent.sessions ?? 0} 个活跃会话</div>
                        </div>
                      </div>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      <button onClick={() => openEdit(selectedAgent)} className={accentButtonClass}>
                        编辑当前智能体
                      </button>
                      <button onClick={() => setWorkbenchView('routing')} className={actionButtonClass}>
                        查看路由工作台
                      </button>
                    </div>
                  </div>
                </div>

                <div className={panelClass}>
                  <div className="px-4 py-4 border-b border-gray-100 dark:border-gray-700/50">
                    <div className="flex flex-wrap gap-2">
                      {AGENT_DIRECTORY_TABS.map(tab => (
                        <button
                          key={tab.id}
                          onClick={() => setDetailTab(tab.id)}
                          className={`px-3 py-2 rounded-lg text-xs border transition-colors ${
                            detailTab === tab.id
                              ? (modern ? 'bg-sky-500 text-white border-sky-500 shadow-sm shadow-sky-500/20' : 'bg-violet-600 text-white border-violet-600')
                              : (modern ? 'bg-white/80 dark:bg-slate-900/70 text-slate-600 dark:text-slate-300 border-slate-200/80 dark:border-slate-700 hover:border-sky-300' : 'bg-white dark:bg-gray-900 text-gray-600 dark:text-gray-300 border-gray-200 dark:border-gray-700 hover:border-violet-300')
                          }`}
                        >
                          <div className="font-medium">{tab.title}</div>
                          <div className={`mt-0.5 ${detailTab === tab.id ? (modern ? 'text-sky-100' : 'text-violet-100') : 'text-gray-400'}`}>{tab.description}</div>
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
                              模型与身份
                            </div>
                            <div className="mt-3 space-y-2 text-sm text-gray-600 dark:text-gray-300">
                              <div>主模型：<span className="font-medium text-gray-900 dark:text-white">{selectedModelDraft.primary || defaultModelHint || '继承默认'}</span></div>
                              <div>回退模型：{selectedModelDraft.fallbacks || '未设置'}</div>
                              <div className="flex items-center gap-1">默认上下文预算 <InfoTooltip content={<>对应 <code className="font-mono text-[11px]">agents.defaults.contextTokens</code>，系统级默认值，不支持单 Agent 覆盖</>} />：{agentDefaults.contextTokens !== undefined ? String(agentDefaults.contextTokens) : '未设置'}</div>
                              <div className="flex items-center gap-1">默认压缩模式 <InfoTooltip content={<>对应 <code className="font-mono text-[11px]">agents.defaults.compaction.mode</code>，系统级默认值</>} />：{agentDefaults.compactionMode || '未设置'}</div>
                              <div>身份名：{selectedIdentity.name || '未设置'}</div>
                              <div>主题：{selectedIdentity.theme || '未设置'}</div>
                              <div>表情：{selectedIdentity.emoji || '未设置'}</div>
                            </div>
                            <button onClick={() => openEdit(selectedAgent, 'behavior')} className="mt-4 px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600">
                              编辑模型与身份
                            </button>
                          </div>
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4">
                            <div className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                              <Shield size={15} className="text-violet-500" />
                              工具与权限
                            </div>
                            <div className="mt-3 space-y-2 text-sm text-gray-600 dark:text-gray-300">
                              <div>沙箱：<span className="font-medium text-gray-900 dark:text-white">{describe沙箱Mode(selectedAgent.sandbox)}</span></div>
                              <div>工具配置：{String(getNestedValue(selectedTools, 'profile') || '未设置')}</div>
                              <div>允许：{parseStringList(getNestedValue(selectedTools, 'allow')).join(', ') || '未设置'}</div>
                              <div>拒绝：{parseStringList(getNestedValue(selectedTools, 'deny')).join(', ') || '未设置'}</div>
                              <div>群聊模式：{describeTriState(triStateFromValue(getNestedValue(selectedGroupChat, 'enabled')), '显式启用', '显式关闭')}</div>
                            </div>
                            <button onClick={() => openEdit(selectedAgent, 'access')} className={`mt-4 ${actionButtonClass}`}>
                              编辑工具与权限
                            </button>
                          </div>
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4">
                            <div className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                              <Route size={15} className="text-violet-500" />
                              路由与上下文
                            </div>
                            <div className="mt-3 space-y-2 text-sm text-gray-600 dark:text-gray-300">
                              <div>路由规则：<span className="font-medium text-gray-900 dark:text-white">{selected路由Count > 0 ? `${selected路由Count} 条命中到它` : '暂无规则'}</span></div>
                              <div>活跃会话：{selectedAgent.sessions ?? 0}</div>
                              <div>最后活跃：{formatLastActive(selectedAgent.lastActive)}</div>
                              <div>工作区：<span className="font-mono">{selectedAgent.workspace || '—'}</span></div>
                            </div>
                            <button
                              onClick={() => {
                                setWorkbenchView('routing');
                                setDetailTab('context');
                              }}
                              className="mt-4 px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600"
                            >
                              打开路由工作台
                            </button>
                          </div>
                        </div>
                      </div>
                    )}

                    {detailTab === 'model' && (
                      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-3">
                          <div>
                            <h4 className="text-sm font-semibold text-gray-900 dark:text-white">模型选择</h4>
                            <p className="text-xs text-gray-500 mt-1">明确区分“当前设置”和“继承默认”的关系。</p>
                          </div>
                          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">主模型</div>
                              <div className="mt-1 font-medium text-gray-900 dark:text-white">{selectedModelDraft.primary || '继承默认'}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">回退模型</div>
                              <div className="mt-1 text-gray-700 dark:text-gray-200">{selectedModelDraft.fallbacks || '未设置'}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">温度</div>
                              <div className="mt-1 text-gray-700 dark:text-gray-200">{isPlainObject(selectedAgent.params) && selectedAgent.params.temperature !== undefined ? String(selectedAgent.params.temperature) : '未覆盖'}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">最大输出</div>
                              <div className="mt-1 text-gray-700 dark:text-gray-200">{isPlainObject(selectedAgent.params) && selectedAgent.params.maxTokens !== undefined ? String(selectedAgent.params.maxTokens) : '未覆盖'}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400 flex items-center gap-1">默认上下文预算 <InfoTooltip content={<>对应 <code className="font-mono text-[11px]">agents.defaults.contextTokens</code>，OpenClaw 会与模型真实 contextWindow 取较小值</>} /></div>
                              <div className="mt-1 text-gray-700 dark:text-gray-200">
                                {agentDefaults.contextTokens !== undefined ? String(agentDefaults.contextTokens) : '未设置'}
                              </div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400 flex items-center gap-1">默认压缩模式 <InfoTooltip content={<>对应 <code className="font-mono text-[11px]">agents.defaults.compaction.mode</code>，可选 default / safeguard</>} /></div>
                              <div className="mt-1 text-gray-700 dark:text-gray-200">
                                {agentDefaults.compactionMode || '未设置'}
                              </div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400 flex items-center gap-1">默认历史占比上限 <InfoTooltip content={<>对应 <code className="font-mono text-[11px]">agents.defaults.compaction.maxHistoryShare</code>，取值 0~1</>} /></div>
                              <div className="mt-1 text-gray-700 dark:text-gray-200">
                                {agentDefaults.compactionMaxHistoryShare !== undefined ? String(agentDefaults.compactionMaxHistoryShare) : '未设置'}
                              </div>
                            </div>
                          </div>
                          <button onClick={() => openEdit(selectedAgent, 'behavior')} className={accentButtonClass}>
                            编辑模型参数
                          </button>
                        </div>
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-3">
                          <div>
                            <h4 className="text-sm font-semibold text-gray-900 dark:text-white">身份摘要</h4>
                            <p className="text-xs text-gray-500 mt-1">业务方通常先理解“这个 Agent 是谁、以什么口吻工作”。</p>
                          </div>
                            <div className="space-y-3 text-sm">
                              <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                                <div className="text-xs text-gray-400">头像预览</div>
                                {selectedAvatarPreview ? (
                                  <div className="mt-2 flex items-center gap-3">
                                    <img src={selectedAvatarPreview} alt={`${selectedIdentity.name || selectedAgent.id} avatar`} className="h-14 w-14 rounded-2xl object-cover border border-white/60 dark:border-slate-700 bg-white dark:bg-slate-950" />
                                    <div className="text-xs text-gray-500">
                                      {/^data:/i.test(selectedIdentity.avatar || '') ? '直接渲染 data URI' : /^https?:\/\//i.test(selectedIdentity.avatar || '') ? '直接渲染远程 URL' : '通过本地 avatar 预览接口加载'}
                                    </div>
                                  </div>
                                ) : (
                                  <div className="mt-2 text-xs text-gray-500">当前没有可用的头像预览。</div>
                                )}
                              </div>
                              <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                                <div className="text-xs text-gray-400">显示名称</div>
                                <div className="mt-1 text-gray-900 dark:text-white">{selectedIdentity.name || '未设置'}</div>
                              </div>
                              <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                                <div className="text-xs text-gray-400">主题</div>
                                <div className="mt-1 text-gray-700 dark:text-gray-200 whitespace-pre-wrap">{selectedIdentity.theme || '未设置'}</div>
                              </div>
                              <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                                <div className="text-xs text-gray-400">表情</div>
                                <div className="mt-1 text-gray-700 dark:text-gray-200">{selectedIdentity.emoji || '未设置'}</div>
                              </div>
                              <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                                <div className="text-xs text-gray-400">头像</div>
                                <div className="mt-1 text-gray-700 dark:text-gray-200 break-all">{selectedIdentity.avatar || '未设置'}</div>
                              </div>
                            </div>
                          <button onClick={() => openEdit(selectedAgent, 'behavior')} className={actionButtonClass}>
                              编辑身份摘要
                            </button>
                          </div>
                      </div>
                    )}

                    {detailTab === 'tools' && (
                      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                          <div>
                            <h4 className="text-sm font-semibold text-gray-900 dark:text-white">工具策略与群聊</h4>
                            <p className="text-xs text-gray-500 mt-1">这里只展示当前 Agent 真正受支持的工具覆盖；跨 Agent 协作策略请到系统级配置查看。</p>
                          </div>
                          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">Tool Profile</div>
                              <div className="mt-1 text-gray-900 dark:text-white">{String(getNestedValue(selectedTools, 'profile') || '未设置')}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">Allow</div>
                              <div className="mt-1 text-gray-900 dark:text-white">{parseStringList(getNestedValue(selectedTools, 'allow')).join(', ') || '未设置'}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">Deny</div>
                              <div className="mt-1 text-gray-900 dark:text-white">{parseStringList(getNestedValue(selectedTools, 'deny')).join(', ') || '未设置'}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">群聊模式</div>
                              <div className="mt-1 text-gray-900 dark:text-white">{describeTriState(triStateFromValue(getNestedValue(selectedGroupChat, 'enabled')), '显式启用', '显式关闭')}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">子 Agent allow（Subagents）</div>
                              <div className="mt-1 text-gray-900 dark:text-white">{parseStringList(getNestedValue(selectedSubagents, 'allowAgents')).join(', ') || '未额外覆盖 - 默认仅同 Agent'}</div>
                            </div>
                          </div>
                          <div className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-3 text-xs text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/40 dark:text-amber-200">
                            OpenClaw 当前不支持 <span className="font-mono">agents.list[].tools.agentToAgent</span> 和 <span className="font-mono">agents.list[].tools.sessions.visibility</span>。
                            这两项只支持系统级 <span className="font-mono">tools</span> 配置。
                          </div>
                        </div>
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                          <div>
                            <h4 className="text-sm font-semibold text-gray-900 dark:text-white">工具策略与执行权限</h4>
                            <p className="text-xs text-gray-500 mt-1">这里把 “能不能调用工具” 和 “工具在哪里执行” 分开看，避免把 sandbox 误当成完整权限模型。</p>
                          </div>
                          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">工具配置</div>
                              <div className="mt-1 text-gray-900 dark:text-white">{selectedToolProfile || '继承默认'}</div>
                              <div className="mt-1 text-xs text-gray-500">allow {selectedToolAllow.length} 项 · deny {selectedToolDeny.length} 项</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">沙箱 模式</div>
                              <div className="mt-1 text-gray-900 dark:text-white">{describe沙箱ModeValue(selected沙箱Draft.mode)}</div>
                              <div className="mt-1 text-xs text-gray-500">scope：{describe沙箱ScopeValue(selected沙箱Draft.scope)}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">工作区挂载</div>
                              <div className="mt-1 text-gray-900 dark:text-white">{describeWorkspaceAccessValue(selected沙箱Draft.workspaceAccess)}</div>
                              <div className="mt-1 text-xs text-gray-500 font-mono">{selected沙箱Draft.workspaceRoot || '使用默认 sandbox workspace'}</div>
                            </div>
                            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3">
                              <div className="text-xs text-gray-400">Docker 覆盖</div>
                              <div className="mt-1 text-gray-900 dark:text-white">{selected沙箱Draft.dockerBinds ? `${parseLineList(selected沙箱Draft.dockerBinds).length} 条 bind` : '未设置 bind'}</div>
                              <div className="mt-1 text-xs text-gray-500">network：{selected沙箱Draft.dockerNetwork || '默认'} · readOnlyRoot：{describeToggleValue(selected沙箱Draft.dockerReadOnlyRoot, '启用', '关闭')}</div>
                            </div>
                          </div>
                          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
                            <div className="rounded-lg border border-gray-100 dark:border-gray-700 px-3 py-3">
                              <div className="text-gray-400">工作区</div>
                              <div className="mt-1 font-mono text-gray-700 dark:text-gray-200 break-all">{displayWorkspacePath(selectedAgent.workspace)}</div>
                            </div>
                            <div className="rounded-lg border border-gray-100 dark:border-gray-700 px-3 py-3">
                              <div className="text-gray-400">智能体目录</div>
                              <div className="mt-1 font-mono text-gray-700 dark:text-gray-200 break-all">{displayAgentDirPath(selectedAgent.agentDir, selectedAgent.id)}</div>
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
                          <button onClick={() => openEdit(selectedAgent, 'access')} className={accentButtonClass}>
                            编辑工具与权限
                          </button>
                        </div>
                      </div>
                    )}

                    {detailTab === 'files' && (
                      <div className="space-y-4">
                        <div className={`${modern ? 'rounded-xl border border-sky-200/70 bg-sky-500/10 px-4 py-3 text-sm text-sky-700 dark:border-sky-800/50 dark:text-sky-200' : 'rounded-xl border border-violet-100 bg-violet-50 px-4 py-3 text-sm text-violet-700 dark:border-violet-900/40 dark:bg-violet-950/20 dark:text-violet-200'}`}>
                          对标官方 Files 面板：这里直接查看并编辑当前 Agent 工作区中的核心文件，避免人格、工具说明与长期记忆继续埋在大段 JSON 里。
                        </div>
                        <div className="grid grid-cols-1 xl:grid-cols-[300px,1fr] gap-4">
                          <div className={softPanelClass}>
                            <div className="flex items-start justify-between gap-3">
                              <div>
                                <h4 className="text-sm font-semibold text-gray-900 dark:text-white">核心文件列表</h4>
                                <p className="text-xs text-gray-500 mt-1 break-all">{selectedAgent.workspace || '当前 Agent 未配置 workspace。'}</p>
                              </div>
                              <button
                                onClick={() => {
                                  if (!confirmDiscardCoreFileDraft('并刷新核心文件列表')) return;
                                  loadAgentCoreFiles(selectedAgent.id, true);
                                }}
                                className={actionButtonClass}
                              >
                                刷新
                              </button>
                            </div>
                            {coreFilesLoading ? (
                              <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-8 text-center text-sm text-gray-400">
                                <RefreshCw size={16} className="animate-spin inline mr-2" />
                                正在读取工作区文件...
                              </div>
                            ) : selectedCoreFilesState?.kind === 'restricted' ? (
                              <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-6 text-sm text-amber-700 dark:border-amber-900/40 dark:bg-amber-950/20 dark:text-amber-200">
                                <div className="font-medium">工作区已配置，但当前策略不允许读取核心文件。</div>
                                <div className="mt-2 text-xs leading-relaxed">{selectedCoreFilesState.message || '当前 agent workspace 不在受管工作区内，或命中了安全限制。'}</div>
                              </div>
                            ) : selectedCoreFilesState?.kind === 'error' ? (
                              <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-6 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-950/20 dark:text-red-200">
                                <div className="font-medium">读取核心文件失败</div>
                                <div className="mt-2 text-xs leading-relaxed">{selectedCoreFilesState.message || '请稍后重试。'}</div>
                              </div>
                            ) : selectedAgentCoreFiles.length === 0 ? (
                              <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-8 text-center text-sm text-gray-400">
                                {selectedCoreFilesState?.kind === 'missing'
                                  ? '当前 Agent 尚未配置可读取的 workspace，所以暂时没有核心文件列表。'
                                  : '当前 Agent 还没有核心文件；保存任一文件后会自动在工作区中创建。'}
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
                                          ? (modern ? 'border-sky-300 bg-sky-50/80 dark:bg-sky-950/20 dark:border-sky-800/60' : 'border-violet-300 bg-violet-50/80 dark:bg-violet-950/20 dark:border-violet-800/60')
                                          : (modern ? 'border-slate-200/70 dark:border-slate-700 hover:border-sky-200 dark:hover:border-sky-800/60' : 'border-gray-100 dark:border-gray-700 hover:border-violet-200 dark:hover:border-violet-800/60')
                                      }`}
                                    >
                                      <div className="flex items-start justify-between gap-2">
                                        <div>
                                          <div className="text-sm font-medium text-gray-900 dark:text-white">{fileMeta.label}</div>
                                          <p className="mt-1 text-[11px] text-gray-500">{fileMeta.description}</p>
                                        </div>
                                        <span className={`inline-flex shrink-0 whitespace-nowrap text-[10px] px-1.5 py-0.5 rounded-full ${file.exists ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200' : 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'}`}>
                                          {file.exists ? '已存在' : '缺失'}
                                        </span>
                                      </div>
                                      <div className="mt-2 whitespace-nowrap text-[11px] text-gray-400 overflow-hidden text-ellipsis">
                                        {file.exists ? `${formatFileSize(file.size)} · ${formatDateTime(file.modified)}` : '保存后会自动创建'}
                                      </div>
                                    </button>
                                  );
                                })}
                              </div>
                            )}
                          </div>

                          <div className={softPanelClass}>
                            {selectedCoreFilesState?.kind === 'restricted' ? (
                              <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-10 text-center text-sm text-amber-700 dark:border-amber-900/40 dark:bg-amber-950/20 dark:text-amber-200">
                                当前工作区已配置，但因为安全限制无法展示文件内容。
                              </div>
                            ) : selectedCoreFilesState?.kind === 'error' ? (
                              <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-10 text-center text-sm text-red-700 dark:border-red-900/40 dark:bg-red-950/20 dark:text-red-200">
                                核心文件读取失败，请先修复上方错误后再试。
                              </div>
                            ) : selectedCoreFilesState?.kind === 'missing' ? (
                              <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-16 text-center text-sm text-gray-400">
                                先为当前 Agent 配置 workspace，随后这里会显示对应核心文件内容。
                              </div>
                            ) : selectedCoreFile ? (
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
                                    className={accentButtonClass}
                                  >
                                    {coreFileSaving ? '保存中...' : `保存 ${selectedCoreFile.name}`}
                                  </button>
                                </div>
                                <textarea
                                  id="agent-core-file-content"
                                  name="agentCoreFileContent"
                                  aria-label={`${selectedCoreFile.name} 内容`}
                                  value={coreFileDraft}
                                  onChange={e => {
                                    setCoreFileDraft(e.target.value);
                                    setCoreFileTouched(true);
                                  }}
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
                          对齐官方 Skills / Channel Routing / Cron Jobs 心智：Skills 会按当前 Agent 的工作区与共享来源重新解析，Channels 表示消息路由到此 Agent 的通道上下文，Cron Jobs 展示绑定到该 Agent 的计划任务。
                          <InfoTooltip content={<>技术细节：Skills 依据 <code className="font-mono text-[11px]">workspace</code> 解析，定时任务通过 <code className="font-mono text-[11px]">agentId</code> 绑定，<code className="font-mono text-[11px]">HEARTBEAT.md</code> 在 Core Files 中维护。</>} />
                        </div>
                        <div className="grid grid-cols-1 lg:grid-cols-4 gap-4">
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 bg-gray-50/80 dark:bg-gray-900">
                            <div className="text-xs text-gray-400">有效技能</div>
                            <div className="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{skillsLoading ? '…' : enabledSkills.length}</div>
                            <div className="mt-1 text-xs text-gray-500">
                              {skillsLoading
                                ? '正在按当前 Agent 解析 Skills…'
                                : `共 ${skills.length} 项；workspace skills 会覆盖共享 / bundled。`}
                            </div>
                          </div>
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 bg-gray-50/80 dark:bg-gray-900">
                            <div className="text-xs text-gray-400">显式路由通道</div>
                            <div className="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{selectedAgentChannelSnapshot.length}</div>
                            <div className="mt-1 text-xs text-gray-500">
                              {selectedAgentIsDefault
                                ? '显式 bindings 命中的通道数；它同时还是默认智能体 的回落目标。'
                                : '只统计显式 bindings 命中的通道路由上下文。'}
                            </div>
                          </div>
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 bg-gray-50/80 dark:bg-gray-900">
                            <div className="text-xs text-gray-400">定时任务</div>
                            <div className="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{selectedAgentCronJobs.length}</div>
                            <div className="mt-1 text-xs text-gray-500 flex items-center gap-1">只统计绑定到当前 Agent 的任务。<InfoTooltip content={<>通过 <code className="font-mono text-[11px]">agentId</code> 绑定；<code className="font-mono text-[11px]">sessionTarget</code> 决定跑在 main 还是 isolated。</>} /></div>
                          </div>
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 bg-gray-50/80 dark:bg-gray-900">
                            <div className="text-xs text-gray-400">心跳文件</div>
                            <div className="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{selectedHeartbeatFile?.exists ? '已配置' : '未配置'}</div>
                            <div className="mt-1 text-xs text-gray-500">
                              {selectedHeartbeatFile?.exists
                                ? `最近更新 ${formatDateTime(selectedHeartbeatFile.modified)}`
                                : '在 Core Files 中维护心跳检查清单。'}
                            </div>
                          </div>
                        </div>

                        <div className="grid grid-cols-1 xl:grid-cols-3 gap-4">
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-3">
                            <div>
                              <h4 className="text-sm font-semibold text-gray-900 dark:text-white">技能快照</h4>
                              <p className="text-xs text-gray-500 mt-1 flex items-center gap-1">按当前 Agent 的工作区与共享来源解析有效 Skills；启用/禁用仍是全局配置。<InfoTooltip content={<>依据 <code className="font-mono text-[11px]">workspace</code> 解析；<code className="font-mono text-[11px]">skills.entries</code> 的开关是全局生效的。</>} /></p>
                            </div>
                            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-[11px] text-gray-500">
                              <div className="rounded-lg border border-gray-100 dark:border-gray-700 px-3 py-2">
                                <div className="text-gray-400">解析 Agent</div>
                                <div className="mt-1 font-mono text-gray-700 dark:text-gray-200">{skillsContext.agentId || selectedAgent?.id || '—'}</div>
                              </div>
                              <div className="rounded-lg border border-gray-100 dark:border-gray-700 px-3 py-2">
                                <div className="text-gray-400">workspace</div>
                                <div className="mt-1 font-mono text-gray-700 dark:text-gray-200 truncate">{displayWorkspacePath(skillsContext.workspace || selectedAgent?.workspace)}</div>
                              </div>
                              {selectedSkillSourceSummary.map(item => (
                                <div key={item.key} className="rounded-lg border border-gray-100 dark:border-gray-700 px-3 py-2">
                                  <div className="text-gray-400">{item.label}</div>
                                  <div className="mt-1 text-gray-700 dark:text-gray-200">{item.count}</div>
                                </div>
                              ))}
                            </div>
                            <div className="space-y-2">
                              {skillsLoading && (
                                <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-6 text-sm text-gray-400 text-center">
                                  正在按当前 Agent 解析 Skills…
                                </div>
                              )}
                              {!skillsLoading && enabledSkills.slice(0, 5).map(skill => (
                                <div key={skill.id} className="rounded-lg border border-gray-100 dark:border-gray-700 px-3 py-3">
                                  <div className="flex items-center justify-between gap-2">
                                    <div className="text-sm font-medium text-gray-900 dark:text-white">{skill.name || skill.id}</div>
                                    <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200">
                                      已启用
                                    </span>
                                  </div>
                                  <p className="mt-1 text-[11px] text-gray-500 line-clamp-2">{skill.description || '暂无描述'}</p>
                                  <div className="mt-2 text-[11px] text-gray-400">{humanizeSkillSource(skill.source)}{skill.version ? ` · v${skill.version}` : ''}</div>
                                </div>
                              ))}
                              {!skillsLoading && enabledSkills.length === 0 && (
                                <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-6 text-sm text-gray-400 text-center">
                                  当前没有启用中的技能。
                                </div>
                              )}
                            </div>
                          </div>

                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-3">
                            <div>
                              <h4 className="text-sm font-semibold text-gray-900 dark:text-white">通道路由上下文</h4>
                              <p className="text-xs text-gray-500 mt-1">通道本身是全局 Gateway 配置；这里只列出当前 Agent 命中的显式 bindings 对应通道。{selectedAgentIsDefault ? '它同时是默认智能体，未命中显式规则的消息仍会回落到这里。' : ''}</p>
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
                                  {selectedAgentIsDefault
                                    ? '当前还没有显式 bindings 指向它；作为默认智能体，未命中路由规则的消息仍会回落到这里。'
                                    : '当前还没有显式 bindings 把任何通道路由到这个 Agent。'}
                                </div>
                              )}
                            </div>
                          </div>

                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-3">
                            <div>
                              <h4 className="text-sm font-semibold text-gray-900 dark:text-white">定时任务</h4>
                              <p className="text-xs text-gray-500 mt-1 flex items-center gap-1">展示绑定到当前 Agent 的定时任务。<InfoTooltip content={<>通过 <code className="font-mono text-[11px]">agentId</code> 绑定；<code className="font-mono text-[11px]">sessionTarget</code> 决定 main/isolated，<code className="font-mono text-[11px]">wakeMode</code> 决定是否等待心跳周期。</>} /></p>
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
                                  <div className="mt-1 text-[11px] text-gray-400">执行模式：{job.sessionTarget === 'isolated' ? '独立 cron 会话' : `main 会话 - 唤醒模式: ${job.wakeMode || 'now'}`}</div>
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
                            <div className="text-xs text-gray-400">命中规则</div>
                            <div className="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{selected路由Count}</div>
                          </div>
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 bg-gray-50/80 dark:bg-gray-900">
                            <div className="text-xs text-gray-400">活跃会话</div>
                            <div className="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{selectedAgent.sessions ?? 0}</div>
                          </div>
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 bg-gray-50/80 dark:bg-gray-900">
                            <div className="text-xs text-gray-400">最后活跃</div>
                            <div className="mt-1 text-sm font-medium text-gray-900 dark:text-white">{formatLastActive(selectedAgent.lastActive)}</div>
                          </div>
                        </div>
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-3">
                          <div className="flex items-center justify-between gap-3">
                            <div>
                              <h4 className="text-sm font-semibold text-gray-900 dark:text-white">引用它的路由规则</h4>
                              <p className="text-xs text-gray-500 mt-1">这能帮助你快速理解“为什么消息会来到这个 Agent”。</p>
                            </div>
                            <button onClick={() => setWorkbenchView('routing')} className={actionButtonClass}>
                              前往路由工作台
                            </button>
                          </div>
                          {selectedAgentBindings.length === 0 ? (
                            <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-6 text-sm text-gray-400 text-center">
                              暂无显式规则指向这个 Agent；未命中时会按默认智能体 或其它规则兜底。
                            </div>
                          ) : (
                            selectedAgentBindings.map(({ binding, index }) => {
                              const summary = buildBindingSummary(binding.match, channelMeta);
                              const tags = buildBindingTags(binding.match);
                              return (
                                <div key={`${binding.agentId}-${index}`} className="rounded-lg border border-gray-100 dark:border-gray-700 px-4 py-3">
                                  <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                                    <div>
                                      <div className="flex flex-wrap items-center gap-2">
                                        <span className="font-medium text-sm text-gray-900 dark:text-white">{binding.comment.trim() || `规则 ${index + 1}`}</span>
                                        <span className={`text-[10px] px-1.5 py-0.5 rounded ${binding.type === 'acp' ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200' : 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-200'}`}>
                                          {binding.type === 'acp' ? 'ACP' : '路由'}
                                        </span>
                                        {binding.enabled === false && (
                                          <span className="text-[10px] px-1.5 py-0.5 rounded bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200">
                                            旧版已禁用
                                          </span>
                                        )}
                                        <span className="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-200">
                                          优先级：{humanizeBindingPriority(matchPriorityLabel(compactMatch(binding.match)))}
                                        </span>
                                      </div>
                                      <p className="mt-2 text-sm text-gray-600 dark:text-gray-300">{summary}</p>
                                      {tags.length > 0 && (
                                        <div className="mt-2 flex flex-wrap gap-1.5">
                                          {tags.map(tag => (
                                            <span key={`${binding.agentId}-${index}-${tag}`} className="text-[10px] px-1.5 py-0.5 rounded-full bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-200">
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
                                      className={accentButtonClass}
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
                              <h4 className="text-sm font-semibold text-gray-900 dark:text-white">最近会话</h4>
                              <p className="text-xs text-gray-500 mt-1">基于现有 Sessions API 展示这个 Agent 最近活跃的会话，便于从单 Agent 视角快速巡检。</p>
                            </div>
                            <button
                              onClick={() => loadAgentSessions(selectedAgent.id, true)}
                              className="px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600"
                            >
                              刷新
                            </button>
                          </div>
                          {sessionsLoading && selectedAgentSessions.length === 0 ? (
                            <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-6 text-center text-sm text-gray-400">
                              <RefreshCw size={16} className="animate-spin inline mr-2" />
                              正在读取最近会话...
                            </div>
                          ) : selectedAgentSessions.length === 0 ? (
                            <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-6 text-sm text-gray-400 text-center">
                              当前没有可展示的会话，或会话仍由默认智能体 接管。
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
                          结构化表单优先覆盖常用配置；只有当你需要 provider 专属字段或复杂对象时，再进入 高级 JSON。
                        </div>
                        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4">
                            <div className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                              <Sparkles size={15} className="text-violet-500" />
                              当前已使用的高级块
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
                              下一步建议
                            </div>
                            <ul className="mt-3 list-disc pl-4 space-y-1.5 text-sm text-gray-600 dark:text-gray-300">
                              <li>先在结构化表单里完成常见字段，再用 JSON 补充 provider 专属项。</li>
                              <li>如果某个 Agent 长期依赖 JSON，可考虑抽出成模板或说明文档。</li>
                              <li>保存前会保留未 touched 的高级块，避免静默丢值。</li>
                            </ul>
                          </div>
                        </div>
                        <button onClick={() => openEdit(selectedAgent, 'advanced')} className={accentButtonClass}>
                          打开高级 JSON（Open 高级 JSON）
                        </button>
                      </div>
                    )}
                  </div>
                </div>
              </>
            ) : (
              <div className="rounded-xl border border-dashed border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 px-6 py-16 text-center text-sm text-gray-400">
                选择左侧智能体后，这里会显示它的配置摘要与上下文。
              </div>
            )}
          </div>
        </div>
      ) : (
        <div className="space-y-6">
          <div className="grid grid-cols-1 lg:grid-cols-4 gap-4">
            <div className={`${modern ? 'page-modern-panel p-4' : 'rounded-xl border border-gray-100 dark:border-gray-700 bg-white dark:bg-gray-800 p-4 shadow-sm'}`}>
              <div className="text-xs text-gray-400">规则总数</div>
              <div className="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{routingStats.total}</div>
            </div>
            <div className={`${modern ? 'page-modern-panel p-4' : 'rounded-xl border border-gray-100 dark:border-gray-700 bg-white dark:bg-gray-800 p-4 shadow-sm'}`}>
              <div className="text-xs text-gray-400">显式规则</div>
              <div className="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{routingStats.enabled}</div>
            </div>
            <div className={`${modern ? 'page-modern-panel p-4' : 'rounded-xl border border-gray-100 dark:border-gray-700 bg-white dark:bg-gray-800 p-4 shadow-sm'}`}>
              <div className="text-xs text-gray-400">被路由引用的智能体</div>
              <div className="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{routingStats.agents}</div>
            </div>
            <div className={`${modern ? 'page-modern-panel p-4' : 'rounded-xl border border-gray-100 dark:border-gray-700 bg-white dark:bg-gray-800 p-4 shadow-sm'}`}>
              <div className="text-xs text-gray-400">涉及通道</div>
              <div className="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{routingStats.channels}</div>
            </div>
          </div>

          <div className={panelClass}>
            <div className="px-4 py-4 border-b border-gray-100 dark:border-gray-700/50 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h3 className="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                  <Settings size={15} className={modern ? 'text-sky-500' : 'text-violet-500'} />
                  路由规则
                </h3>
                <p className="text-xs text-gray-500 mt-1">结构化编辑器优先输出常用字段，并兼容保留 legacy sender / parentPeer / 字符串 peer 规则；遇到更复杂的历史写法时，再切到 JSON 模式。</p>
              </div>
              <div className="flex items-center gap-2">
                <button onClick={addBinding} className={actionButtonClass}>
                  新增规则
                </button>
                <button onClick={saveBindings} disabled={saving} className={accentButtonClass}>
                  <Save size={12} /> 保存规则
                </button>
              </div>
            </div>
            <div className="px-4 pt-4">
                <div className={`${modern ? 'rounded-xl border border-sky-200/70 bg-sky-500/10 px-4 py-3 text-xs text-sky-700 dark:border-sky-800/50 dark:text-sky-200' : 'rounded-xl border border-violet-100 bg-violet-50 px-4 py-3 text-xs text-violet-700 dark:border-violet-900/40 dark:bg-violet-950/20 dark:text-violet-200'}`}>
                  当前优先级为 sender &gt; peer &gt; parent peer inheritance &gt; guild+roles &gt; guild &gt; team &gt; account &gt; account wildcard &gt; channel &gt; default；只有同一优先级才按列表顺序比较。未命中时会回落到默认智能体「{defaultAgent}」。
                </div>
            </div>
            {duplicateAccountBindings.length > 0 && (
              <div className="px-4 pt-4">
                <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
                  检测到重复账号路由：{duplicateAccountBindings.map(item => `${humanizeChannelName(item.channel)}/${item.accountId} -> ${item.agentIds.join('、')}`).join('；')}。请保证一账号只对应一个智能体，否则会产生路由歧义与旧会话残留。
                </div>
              </div>
            )}
            <div className="p-4 space-y-3">
              {bindings.length === 0 && (
                <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-8 text-center text-sm text-gray-400">
                  还没有显式路由规则，当前所有消息都会落到默认智能体。
                </div>
              )}

              {bindings.map((row, idx) => {
                const rawMatch = isPlainObject(row.match) ? row.match : {};
                const match = compactMatch(row.match);
                const channel = extractTextValue(match.channel);
                const accountId = extractTextValue(match.accountId);
                const sender = extractTextValue(match.sender);
                const peer = extractPeerForm(rawMatch, 'peer');
                const parentPeer = extractPeerForm(rawMatch, 'parentPeer');
                const channelCfg = channel ? channelMeta[channel] : undefined;
                const defaultAccount = channelCfg?.defaultAccount;
                const accounts = channelCfg?.accounts || [];
                const priority = matchPriorityLabel(match);
                const expanded = expandedBindingIndex === idx;
                const tags = buildBindingTags(match);

                return (
                  <div key={idx} className={`${modern ? 'page-modern-panel bg-white/70 dark:bg-slate-900/50' : 'rounded-xl border border-gray-100 dark:border-gray-700 bg-gray-50/70 dark:bg-gray-900/40'}`}>
                    <div className="px-4 py-4 flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                      <div className="flex-1 min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <button
                            onClick={() => setExpandedBindingIndex(prev => (prev === idx ? null : idx))}
                            className={`inline-flex items-center justify-center p-1 rounded ${modern ? 'hover:bg-sky-500/10' : 'hover:bg-white dark:hover:bg-gray-800'}`}
                            aria-label={expanded ? '折叠规则' : '展开规则'}
                          >
                            {expanded ? <ChevronDown size={15} /> : <ChevronRight size={15} />}
                          </button>
                          <span className="font-medium text-sm text-gray-900 dark:text-white">{row.comment.trim() || `规则 ${idx + 1}`}</span>
                          <span className={`text-[10px] px-1.5 py-0.5 rounded ${row.type === 'acp' ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200' : (modern ? 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-200' : 'bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-200')}`}>
                            {row.type === 'acp' ? 'ACP' : '路由'}
                          </span>
                          {row.enabled === false && (
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200">
                              旧版已禁用
                            </span>
                          )}
                          <span className={`text-[10px] px-1.5 py-0.5 rounded ${modern ? 'bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-200' : 'bg-violet-100 text-violet-700 dark:bg-violet-900/40 dark:text-violet-200'}`}>
                            → {row.agentId}
                          </span>
                          <span className="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-200">
                            优先级：{humanizeBindingPriority(priority)}
                          </span>
                        </div>
                        <p className="mt-2 text-sm text-gray-600 dark:text-gray-300">{buildBindingSummary(match, channelMeta)}</p>
                        {row.type === 'acp' && (row.acp.mode || row.acp.label || row.acp.cwd || row.acp.backend) && (
                          <p className="mt-2 text-[11px] text-emerald-700 dark:text-emerald-300">
                            ACP：{row.acp.mode || '未指定模式'}{row.acp.label ? ` · ${row.acp.label}` : ''}{row.acp.backend ? ` · backend=${row.acp.backend}` : ''}{row.acp.cwd ? ` · cwd=${row.acp.cwd}` : ''}
                          </p>
                        )}
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
                        <button onClick={() => moveBinding(idx, -1)} className={`p-1.5 rounded ${modern ? 'hover:bg-sky-500/10' : 'hover:bg-white dark:hover:bg-gray-800'}`} title="上移"><ArrowUp size={13} /></button>
                        <button onClick={() => moveBinding(idx, 1)} className={`p-1.5 rounded ${modern ? 'hover:bg-sky-500/10' : 'hover:bg-white dark:hover:bg-gray-800'}`} title="下移"><ArrowDown size={13} /></button>
                        <button onClick={() => removeBinding(idx)} className="p-1.5 rounded text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20" title="删除"><Trash2 size={13} /></button>
                      </div>
                    </div>

                    {expanded && (
                      <div className="border-t border-gray-100 dark:border-gray-700 px-4 py-4 space-y-4">
                        <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                          <div>
                            <label htmlFor={`binding-${idx}-comment`} className="text-[11px] text-gray-500">备注 / 注释</label>
                            <input
                              id={`binding-${idx}-comment`}
                              name={`binding-${idx}-comment`}
                              aria-label="binding comment"
                              value={row.comment}
                              onChange={e => setBindingAt(idx, r => ({ ...r, comment: e.target.value }))}
                              placeholder="例如维护群"
                              className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                            />
                          </div>
                          <div>
                            <label htmlFor={`binding-${idx}-agent`} className="text-[11px] text-gray-500">目标智能体（agentId）</label>
                            <select
                              id={`binding-${idx}-agent`}
                              name={`binding-${idx}-agent`}
                              aria-label="目标智能体"
                              value={row.agentId}
                              onChange={e => setBindingAt(idx, r => ({ ...r, agentId: e.target.value }))}
                              className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                            >
                              {(agentOptions.length ? agentOptions : ['main']).map(id => (
                                <option key={id} value={id}>{id}</option>
                              ))}
                            </select>
                          </div>
                          <div>
                            <label htmlFor={`binding-${idx}-type`} className="text-[11px] text-gray-500">规则类型</label>
                            <select
                              id={`binding-${idx}-type`}
                              name={`binding-${idx}-type`}
                              aria-label="binding type"
                              value={row.type}
                              onChange={e => setBindingAt(idx, r => ({ ...r, type: e.target.value === 'acp' ? 'acp' : 'route' }))}
                              className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                            >
                              {BINDING_TYPE_OPTIONS.map(option => (
                                <option key={option.value} value={option.value}>{option.label}</option>
                              ))}
                            </select>
                          </div>
                        </div>

                        <div className="rounded-xl border border-sky-100 bg-sky-50 px-4 py-3 text-[12px] text-sky-700 dark:border-sky-900/40 dark:bg-sky-950/20 dark:text-sky-200">
                          结构化模式会写入 type、agentId、comment、match，以及 type=acp 时的 acp；旧版 sender / parentPeer / 字符串 peer 规则也会继续保留。需要手动调整复杂 match JSON 时，再切到 JSON 模式。
                        </div>

                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => switchBindingMode(idx, 'structured')}
                            className={`px-2 py-1 text-[11px] rounded border ${row.mode === 'structured' ? (modern ? 'bg-sky-500 text-white border-sky-500' : 'bg-violet-600 text-white border-violet-600') : 'bg-white dark:bg-gray-900 text-gray-600 border-gray-200 dark:border-gray-700'}`}
                          >
                            结构化
                          </button>
                          <button
                            onClick={() => switchBindingMode(idx, 'json')}
                            className={`px-2 py-1 text-[11px] rounded border ${row.mode === 'json' ? (modern ? 'bg-sky-500 text-white border-sky-500' : 'bg-violet-600 text-white border-violet-600') : 'bg-white dark:bg-gray-900 text-gray-600 border-gray-200 dark:border-gray-700'}`}
                          >
                            JSON 高级模式
                          </button>
                          <span className="text-[11px] text-gray-400">accountId 留空 = 仅匹配默认账号</span>
                        </div>

                        {row.mode === 'structured' ? (
                          <div className="space-y-4">
                            {row.type === 'acp' && (
                              <div className="rounded-lg border border-emerald-100 dark:border-emerald-900/40 p-3">
                                <h4 className="text-[11px] font-semibold text-gray-900 dark:text-white uppercase tracking-wide">ACP 配置</h4>
                                <div className="mt-3 grid grid-cols-1 md:grid-cols-4 gap-3">
                                  <div>
                                    <label htmlFor={`binding-${idx}-acp-mode`} className="text-[11px] text-gray-500">acp.mode</label>
                                    <select id={`binding-${idx}-acp-mode`} value={row.acp.mode} onChange={e => setBindingAt(idx, r => ({ ...r, acp: { ...r.acp, mode: e.target.value } }))} className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900">
                                      <option value="">未设置</option>
                                      <option value="persistent">persistent</option>
                                      <option value="oneshot">oneshot</option>
                                    </select>
                                  </div>
                                  <div>
                                    <label htmlFor={`binding-${idx}-acp-label`} className="text-[11px] text-gray-500">acp.label</label>
                                    <input id={`binding-${idx}-acp-label`} value={row.acp.label} onChange={e => setBindingAt(idx, r => ({ ...r, acp: { ...r.acp, label: e.target.value } }))} placeholder="例如 support-shell" className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900" />
                                  </div>
                                  <div>
                                    <label htmlFor={`binding-${idx}-acp-cwd`} className="text-[11px] text-gray-500">acp.cwd</label>
                                    <input id={`binding-${idx}-acp-cwd`} value={row.acp.cwd} onChange={e => setBindingAt(idx, r => ({ ...r, acp: { ...r.acp, cwd: e.target.value } }))} placeholder="例如 workspaces/support" className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900" />
                                  </div>
                                  <div>
                                    <label htmlFor={`binding-${idx}-acp-backend`} className="text-[11px] text-gray-500">acp.backend</label>
                                    <input id={`binding-${idx}-acp-backend`} value={row.acp.backend} onChange={e => setBindingAt(idx, r => ({ ...r, acp: { ...r.acp, backend: e.target.value } }))} placeholder="例如 tmux" className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900" />
                                  </div>
                                </div>
                              </div>
                            )}
                            <div className="rounded-lg border border-gray-100 dark:border-gray-700 p-3">
                              <h4 className="text-[11px] font-semibold text-gray-900 dark:text-white uppercase tracking-wide">消息来源</h4>
                              <div className="mt-3 grid grid-cols-1 md:grid-cols-2 gap-3">
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
                                      {accounts.map(acc => <option key={acc} value={acc}>{acc}</option>)}
                                      {accountId && !accounts.includes(accountId) && (
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
                                      placeholder="留空=默认账号"
                                      className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                                    />
                                  )}
                                  <p className="mt-1 text-[10px] text-gray-500">
                                    {!accountId
                                      ? `当前留空，仅匹配默认账号${defaultAccount ? ` (${defaultAccount})` : ''}`
                                      : `仅匹配账号 ${accountId}`}
                                  </p>
                                </div>
                              </div>
                            </div>

                            <div className="rounded-lg border border-gray-100 dark:border-gray-700 p-3">
                              <h4 className="text-[11px] font-semibold text-gray-900 dark:text-white uppercase tracking-wide">会话匹配</h4>
                              <div className="mt-3 grid grid-cols-1 md:grid-cols-3 gap-3">
                                <div>
                                  <label htmlFor={`binding-${idx}-sender`} className="text-[11px] text-gray-500">sender - 兼容字段</label>
                                  <input
                                    id={`binding-${idx}-sender`}
                                    name={`binding-${idx}-sender`}
                                    aria-label="sender"
                                    value={sender}
                                    onChange={e => touchBindingMatch(idx, cur => ({ ...cur, sender: e.target.value }))}
                                    placeholder="例如 user-10001"
                                    className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                                  />
                                </div>
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
                              </div>
                              <div className="mt-3 grid grid-cols-1 md:grid-cols-2 gap-3">
                                <div>
                                  <label htmlFor={`binding-${idx}-parent-peer-kind`} className="text-[11px] text-gray-500">parentPeer.kind - 兼容字段</label>
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
                                  <label htmlFor={`binding-${idx}-parent-peer-id`} className="text-[11px] text-gray-500">parentPeer.id - 兼容字段</label>
                                  <input
                                    id={`binding-${idx}-parent-peer-id`}
                                    name={`binding-${idx}-parent-peer-id`}
                                    aria-label="parent peer id"
                                    value={parentPeer.id}
                                    onChange={e => setPeerField(idx, 'parentPeer', 'id', e.target.value)}
                                    placeholder="release-notes / group-01"
                                    className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                                  />
                                </div>
                              </div>
                              <p className="mt-2 text-[10px] text-gray-500">sender / parentPeer 属于旧版兼容字段；如果你只对齐当前官方 schema，可保持留空。像 <span className="font-mono">peer: \"group:*\"</span> 这类旧语法会在保存时继续原样保留。</p>
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
                                    placeholder="Discord 服务器 ID"
                                    className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                                  />
                                </div>
                                <div>
                                  <label htmlFor={`binding-${idx}-roles`} className="text-[11px] text-gray-500">roles - 逗号分隔</label>
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
                                    placeholder="Slack 团队 ID"
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

          <div className={panelClass}>
            <div className="px-4 py-4 border-b border-gray-100 dark:border-gray-700/50 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h3 className="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                  <Route size={15} className={modern ? 'text-sky-500' : 'text-violet-500'} />
                  消息分流模拟
                </h3>
                <p className="text-xs text-gray-500 mt-1">把消息场景描述清楚，然后看系统最终会把它交给谁。</p>
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setPreviewMeta(DEFAULT_PREVIEW_META)}
                  className={actionButtonClass}
                >
                  清空输入
                </button>
                <button onClick={runPreview} disabled={previewLoading} className={accentButtonClass}>
                  {previewLoading ? '模拟中...' : '执行模拟'}
                </button>
              </div>
            </div>
            <div className="p-4 space-y-4">
              <div className={`${modern ? 'rounded-xl border border-sky-200/70 bg-sky-500/10 px-4 py-3 text-xs text-sky-700 dark:border-sky-800/50 dark:text-sky-200' : 'rounded-xl border border-sky-100 bg-sky-50 px-4 py-3 text-xs text-sky-700 dark:border-sky-900/40 dark:bg-sky-950/20 dark:text-sky-200'}`}>
                普通使用流程：先填“通道 / 账号”，再按需补充会话、父会话继承、Discord/Slack 相关条件。
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
                        <div className={`mt-1 font-mono ${modern ? 'text-sky-600 dark:text-sky-300' : 'text-violet-600 dark:text-violet-300'}`}>{previewResult.agent || defaultAgent || '-'}</div>
                      </div>
                      <div className="rounded-lg bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 px-3 py-3">
                        <div className="text-xs text-gray-400">命中规则</div>
                        <div className="mt-1 text-gray-900 dark:text-white">{previewExplanation?.ruleLabel || previewResult.matchedBy || '默认回落'}</div>
                      </div>
                      <div className="rounded-lg bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 px-3 py-3">
                        <div className="text-xs text-gray-400">命中优先级</div>
                        <div className="mt-1 font-mono text-gray-700 dark:text-gray-200">{OFFICIAL_MATCHED_BY_LABELS[previewResult.matchedBy || ''] || previewResult.matchedBy || 'default'}</div>
                        {typeof previewResult.matchedIndex === 'number' && previewResult.matchedIndex >= 0 && (
                          <div className="mt-1 text-[11px] text-gray-500">bindings[{previewResult.matchedIndex}]</div>
                        )}
                      </div>
                    </div>

                  {previewExplanation && (
                    <div className={`${modern ? 'rounded-xl border border-sky-200/70 bg-sky-500/10 px-4 py-3 text-sm text-sky-700 dark:border-sky-800/50 dark:text-sky-200' : 'rounded-xl border border-violet-100 bg-violet-50 px-4 py-3 text-sm text-violet-700 dark:border-violet-900/40 dark:bg-violet-950/20 dark:text-violet-200'}`}>
                      <div className="font-medium">{previewExplanation.headline}</div>
                      <div className="mt-1">{previewExplanation.detail}</div>
                    </div>
                  )}

                  <details className="rounded-lg border border-gray-100 dark:border-gray-700 bg-white dark:bg-gray-800">
                    <summary className="cursor-pointer list-none px-4 py-3 text-sm font-medium text-gray-900 dark:text-white flex items-center justify-between">
                      技术细节
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
          <div className={`w-full max-w-5xl max-h-[92vh] overflow-hidden rounded-xl border shadow-xl flex flex-col ${modern ? 'bg-[linear-gradient(180deg,rgba(255,255,255,0.96),rgba(239,246,255,0.92))] dark:bg-[linear-gradient(180deg,rgba(15,23,42,0.96),rgba(12,74,110,0.24))] border-sky-200/60 dark:border-sky-900/40 backdrop-blur-xl' : 'bg-white dark:bg-gray-800 border-gray-100 dark:border-gray-700'}`}>
            <div className={`sticky top-0 z-10 border-b backdrop-blur ${modern ? 'border-sky-200/60 dark:border-sky-900/40 bg-white/80 dark:bg-slate-900/75' : 'border-gray-100 dark:border-gray-700 bg-white/95 dark:bg-gray-800/95'}`}>
              <div className="px-5 py-4 flex items-start justify-between gap-4">
                <div>
                  <h3 className="font-semibold text-gray-900 dark:text-white">{editingId ? `编辑智能体：${editingId}` : '新建智能体'}</h3>
                  <p className="text-xs text-gray-500 mt-1">
                    先完成基础信息，再按需进入其它部分。高级 JSON 会保留完整覆盖能力。
                  </p>
                </div>
                  <button onClick={closeForm} className={modern ? 'page-modern-action text-xs px-2.5 py-1.5 shrink-0' : 'text-xs px-2.5 py-1.5 rounded bg-gray-100 dark:bg-gray-700 shrink-0'}>关闭</button>
              </div>
              <div className="px-5 pb-4 space-y-3">
                <div className="flex flex-wrap gap-2">
                  {AGENT_FORM_SECTIONS.map(section => (
                    <button
                      key={section.id}
                      onClick={() => setFormSection(section.id)}
                      className={`px-3 py-2 rounded-lg text-xs border transition-colors ${formSection === section.id ? (modern ? 'bg-sky-500 text-white border-sky-500 shadow-sm shadow-sky-500/20' : 'bg-violet-600 text-white border-violet-600') : (modern ? 'bg-white/80 dark:bg-slate-900/70 text-slate-600 dark:text-slate-300 border-slate-200/80 dark:border-slate-700 hover:border-sky-300' : 'bg-white dark:bg-gray-900 text-gray-600 dark:text-gray-300 border-gray-200 dark:border-gray-700 hover:border-violet-300')}`}
                    >
                      <div className="font-medium">{section.title}</div>
                      <div className={`mt-0.5 ${formSection === section.id ? (modern ? 'text-sky-100' : 'text-violet-100') : 'text-gray-400'}`}>{section.description}</div>
                    </button>
                  ))}
                </div>
                <div className="grid grid-cols-1 md:grid-cols-4 gap-2 text-[11px]">
                  <div className="rounded-lg bg-gray-50 dark:bg-gray-900 border border-gray-100 dark:border-gray-700 px-3 py-2">
                      <div className="text-gray-400">ID</div>
                    <div className="font-mono text-gray-700 dark:text-gray-200 mt-1">{form.id.trim() || '未设置'}</div>
                  </div>
                  <div className="rounded-lg bg-gray-50 dark:bg-gray-900 border border-gray-100 dark:border-gray-700 px-3 py-2">
                      <div className="text-gray-400">主模型</div>
                    <div className="text-gray-700 dark:text-gray-200 mt-1 truncate">{form.modelPrimary.trim() || '继承 / 高级 JSON'}</div>
                  </div>
                  <div className="rounded-lg bg-gray-50 dark:bg-gray-900 border border-gray-100 dark:border-gray-700 px-3 py-2">
                      <div className="text-gray-400">工作区</div>
                    <div className="font-mono text-gray-700 dark:text-gray-200 mt-1 break-all">{displayWorkspacePath(form.workspace.trim())}</div>
                  </div>
                  <div className="rounded-lg bg-gray-50 dark:bg-gray-900 border border-gray-100 dark:border-gray-700 px-3 py-2">
                      <div className="text-gray-400">默认接管</div>
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
                    <div>如果你直接新建首个显式 Agent，它会接管默认路由，并自动成为默认智能体。</div>
                    <div>如果你想把 <span className="font-mono">main</span> 真正写进配置，请改用列表里的“编辑 main”。</div>
                  </div>
                  )}

                  {materializingImplicitAgent && (
                    <div className="rounded-xl border border-sky-100 bg-sky-50 px-4 py-3 text-[12px] text-sky-700 space-y-1.5">
                      <div className="font-medium">你正在把运行时发现的 <span className="font-mono">{form.id.trim() || editingId || 'agent'}</span> 显式写入配置。</div>
                      <div>保存后，它会真正进入 <span className="font-mono">agents.list</span>，后续即可像普通 Agent 一样继续编辑。</div>
                      {form.id.trim() === defaultAgent && (
                        <div>当前它也是默认智能体 的运行时来源，保存后默认关系会一起落盘。</div>
                      )}
                    </div>
                  )}

                  <div className="rounded-xl border border-amber-100 bg-amber-50 px-4 py-3 text-[12px] text-amber-700 space-y-1.5">
                    <div><span className="font-medium">workspace</span> 只是默认工作目录，方便文件与上下文定位；它不是硬隔离边界。</div>
                    <div>如果需要更严格的执行限制，请在“访问与安全”里设置 <span className="font-mono">sandbox</span> 覆盖。</div>
                    <div><span className="font-mono">agentDir</span> 可以放在 OpenClaw 状态目录外，但同一个 <span className="font-mono">agentDir</span> 不要复用；<span className="font-mono">workspace</span> 目前也不能与其它 Agent 重复。</div>
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
                        <label className="text-xs text-gray-500">显示名称</label>
                      <input
                        value={form.name}
                        onChange={e => updateForm({ name: e.target.value })}
                        placeholder="面向业务人员展示的名称"
                        className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                      />
                    </div>
                    <div>
                        <label className="text-xs text-gray-500">工作区目录</label>
                      <input
                        value={form.workspace}
                        onChange={e => updateForm({ workspace: e.target.value })}
                        placeholder="workspaces/support"
                        className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                      />
                      <p className="mt-1 text-[11px] text-gray-400">用于默认文件读写位置，不代表执行隔离；可以填写绝对路径，但部分受保护的 core-files 功能仍要求落在面板受管工作区内。</p>
                      {workspaceConflict && (
                        <p className="mt-1 text-[11px] text-red-600">该工作区已被智能体“{workspaceConflict.id}”使用。</p>
                      )}
                    </div>
                    <div>
                        <label className="text-xs text-gray-500">智能体目录</label>
                      <input
                        value={form.agentDir}
                        onChange={e => updateForm({ agentDir: e.target.value })}
                        placeholder="agents/support"
                        className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                      />
                      <p className="mt-1 text-[11px] text-gray-400">建议每个智能体使用独立目录，避免认证或模型配置互相覆盖；可填写 OpenClaw 状态目录外的绝对路径。</p>
                      {agentDirConflict && (
                        <p className="mt-1 text-[11px] text-red-600">该智能体目录已被智能体“{agentDirConflict.id}”使用。</p>
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
                    设为默认智能体（Default Fallback，未命中 bindings 时优先接管）
                  </label>
                  {firstExplicitAgentWillBecomeDefault && (
                    <p className="text-[11px] text-gray-500">
                      当前还没有显式 Agent，本次保存后它会自动成为默认智能体。
                    </p>
                  )}
                </div>
              )}

              {formSection === 'behavior' && (
                <div className="space-y-5">
                  <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
                    <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                      <div>
                        <h4 className="text-sm font-semibold text-gray-900 dark:text-white">模型选择</h4>
                        <p className="text-xs text-gray-500 mt-1">优先把常见选择做成结构化输入；更复杂的模型对象可在高级区继续编辑。</p>
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">主模型</label>
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
                                    {model === defaultModelHint ? `${model} - 默认` : model}
                                  </button>
                                ))}
                              </div>
                            )}
                          </div>
                        )}
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">回退模型</label>
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
                        <h4 className="text-sm font-semibold text-gray-900 dark:text-white">常用参数</h4>
                        <p className="text-xs text-gray-500 mt-1">这些参数会覆盖模型默认值；如需 provider 专属参数，请转到高级区 / params JSON。</p>
                      </div>
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                        <div>
                          <label className="text-xs text-gray-500">温度</label>
                          <input
                            value={form.paramTemperature}
                            onChange={e => updateForm({ paramTemperature: e.target.value }, 'params')}
                            placeholder="例如 0.7"
                            className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                          />
                        </div>
                        <div>
                          <label className="text-xs text-gray-500">采样范围</label>
                          <input
                            value={form.paramTopP}
                            onChange={e => updateForm({ paramTopP: e.target.value }, 'params')}
                            placeholder="例如 0.9"
                            className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                          />
                        </div>
                        <div>
                          <label className="text-xs text-gray-500">最大输出</label>
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
                        <h4 className="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-1.5">
                          上下文预算
                          <InfoTooltip size={14} content={<>当前 OpenClaw schema 仅支持 <code className="font-mono text-[11px]">agents.defaults.contextTokens / compaction</code>，不支持单 Agent 级覆盖。如需修改，请前往 System Config。</>} />
                        </h4>
                        <p className="text-xs text-gray-500 mt-1">
                          这里展示的是系统默认值，所有 Agent 共享相同的上下文预算配置。
                        </p>
                      </div>
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                        <div>
                          <label className="text-xs text-gray-500 flex items-center gap-1">默认上下文 Token 预算 <InfoTooltip content={<>字段路径：<code className="font-mono text-[11px]">agents.defaults.contextTokens</code></>} /></label>
                          <div className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900 text-gray-700 dark:text-gray-200">
                            {agentDefaults.contextTokens !== undefined ? String(agentDefaults.contextTokens) : '未设置'}
                          </div>
                        </div>
                        <div>
                          <label className="text-xs text-gray-500 flex items-center gap-1">默认压缩模式 <InfoTooltip content={<>字段路径：<code className="font-mono text-[11px]">agents.defaults.compaction.mode</code>，可选 default / safeguard</>} /></label>
                          <div className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900 text-gray-700 dark:text-gray-200">
                            {agentDefaults.compactionMode || '未设置'}
                          </div>
                        </div>
                        <div>
                          <label className="text-xs text-gray-500 flex items-center gap-1">默认历史占比上限 <InfoTooltip content={<>字段路径：<code className="font-mono text-[11px]">agents.defaults.compaction.maxHistoryShare</code>，取值 0~1</>} /></label>
                          <div className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900 text-gray-700 dark:text-gray-200">
                            {agentDefaults.compactionMaxHistoryShare !== undefined ? String(agentDefaults.compactionMaxHistoryShare) : '未设置'}
                          </div>
                        </div>
                      </div>
                      <div className="text-[11px] text-gray-500 leading-relaxed flex items-center gap-1">
                        旧版面板误写入的单 Agent 上下文配置会在启动或保存时自动清理。
                        <InfoTooltip content={<>具体为单 Agent 级的 <code className="font-mono text-[11px]">contextTokens</code> 和 <code className="font-mono text-[11px]">compaction</code> 字段，会被自动移除以避免 OpenClaw 配置校验错误。</>} />
                      </div>
                    </div>
                  </div>

                  <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">身份摘要</h4>
                      <div className="mt-1 flex flex-wrap items-center gap-2">
                        <p className="text-xs text-gray-500">结构化表单会写入 <span className="font-mono">identity.name / theme / emoji / avatar</span>，并保持与当前后端接受字段一致。</p>
                        <button type="button" onClick={importIdentityFromCoreFile} className={modern ? 'page-modern-action px-2.5 py-1 text-[11px]' : 'px-2.5 py-1 text-[11px] rounded bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600'}>
                          从 IDENTITY.md 导入
                        </button>
                      </div>
                      {identityImportMsg && (
                        <div className={`mt-2 px-3 py-2 rounded-lg text-xs ${identityImportMsg.includes('失败') || identityImportMsg.includes('未解析') || identityImportMsg.includes('未找到') ? 'bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400' : 'bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600 dark:text-emerald-400'}`}>
                          {identityImportMsg}
                        </div>
                      )}
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                      <div>
                        <label className="text-xs text-gray-500">身份名称</label>
                        <input
                          value={form.identityName}
                          onChange={e => updateForm({ identityName: e.target.value }, 'identity')}
                          placeholder="例如 OpenClaw"
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">主题</label>
                        <input
                          value={form.identityTheme}
                          onChange={e => updateForm({ identityTheme: e.target.value }, 'identity')}
                          placeholder="例如 space lobster"
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">表情</label>
                        <input
                          value={form.identityEmoji}
                          onChange={e => updateForm({ identityEmoji: e.target.value }, 'identity')}
                          placeholder="例如 🦞"
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                      </div>
                      <div className="md:col-span-2">
                        <label className="text-xs text-gray-500">头像</label>
                        <input
                          value={form.identityAvatar}
                          onChange={e => updateForm({ identityAvatar: e.target.value }, 'identity')}
                          placeholder="工作区相对路径、http(s) 地址或 data URI"
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                        {formAvatarValidationError ? (
                          <p className="mt-1 text-[11px] text-red-600">{formAvatarValidationError}</p>
                        ) : (
                          <p className="mt-1 text-[11px] text-gray-500">允许 http(s)、data:image/... / data: 以及工作区相对路径；不支持 ~、绝对路径和 file:// / ftp:// 等非 http(s) 协议。旧 avatar 若未修改，可继续原样保留。</p>
                        )}
                      </div>
                      <div className="md:col-span-2">
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 bg-gray-50/80 dark:bg-gray-900 px-4 py-4">
                          <div className="text-xs text-gray-400">头像预览</div>
                          {formAvatarPreview ? (
                            <div className="mt-3 flex items-center gap-4">
                              <img src={formAvatarPreview} alt="Identity avatar preview" className="h-16 w-16 rounded-2xl object-cover border border-white/60 dark:border-slate-700 bg-white dark:bg-slate-950" />
                              <div className="text-xs text-gray-500 leading-relaxed">
                                <div>{/^data:/i.test(form.identityAvatar) ? '将直接渲染 data URI。' : /^https?:\/\//i.test(form.identityAvatar) ? '将直接渲染远程地址。' : '将通过本地头像预览接口读取工作区文件。'}</div>
                                {!editingId && !form.id.trim() && !/^https?:\/\//i.test(form.identityAvatar) && !/^data:/i.test(form.identityAvatar) && (
                                  <div className="mt-1">若要预览工作区相对路径，请先填写智能体 ID。</div>
                                )}
                              </div>
                            </div>
                          ) : (
                            <div className="mt-2 text-xs text-gray-500">输入合法的头像值后，这里会显示预览。</div>
                          )}
                        </div>
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
                      <div>为避免保存时丢失未知枚举，已锁定结构化 sandbox 编辑；请先到 高级 JSON 调整这些字段：<span className="font-mono">{sandboxStructuredIssues.join(', ')}</span></div>
                    </div>
                  )}

                  <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">沙箱 起步模板</h4>
                      <p className="text-xs text-gray-500 mt-1">先用模板建立正确心智模型，再按需补充 mode / scope / docker 覆盖；更高阶字段仍可回到 高级 JSON。</p>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-[220px,1fr] gap-4">
                      <div>
                        <label className="text-xs text-gray-500">起步模板</label>
                        <select
                          value={sandboxStarter}
                          onChange={e => apply沙箱Starter(e.target.value)}
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
                        <label className="text-xs text-gray-500">运行模式</label>
                        <select
                          value={form.sandboxMode}
                          onChange={e => update沙箱Form({ sandboxMode: e.target.value as '' | 'off' | 'non-main' | 'all' })}
                          disabled={sandboxStructuredLocked}
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        >
                          <option value="">继承默认</option>
                          <option value="off">关闭 - 宿主机运行</option>
                          <option value="non-main">仅非主会话</option>
                          <option value="all">全部会话</option>
                        </select>
                        <p className="mt-1 text-[11px] text-gray-400">官方里 <span className="font-mono">non-main</span> 是常见默认值；群聊 / channel / thread 会话通常都会落入这个分支。</p>
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">容器复用范围</label>
                        <select
                          value={form.sandboxScope}
                          onChange={e => update沙箱Form({ sandboxScope: e.target.value as '' | 'session' | 'agent' | 'shared' })}
                          disabled={sandboxStructuredLocked}
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        >
                          <option value="">继承默认</option>
                          <option value="session">每会话一个容器</option>
                          <option value="agent">每 Agent 一个容器</option>
                          <option value="shared">全部共享一个容器</option>
                        </select>
                        <p className="mt-1 text-[11px] text-gray-400">若选 <span className="font-mono">shared</span>，官方语义下 per-agent <span className="font-mono">docker.binds</span> 会被忽略。</p>
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">工作区挂载方式</label>
                        <select
                          value={form.sandboxWorkspaceAccess}
                          onChange={e => update沙箱Form({ sandboxWorkspaceAccess: e.target.value as '' | 'none' | 'ro' | 'rw' })}
                          disabled={sandboxStructuredLocked}
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        >
                          <option value="">继承默认</option>
                          <option value="none">仅沙箱工作区</option>
                          <option value="ro">只读挂载智能体工作区（ro）</option>
                          <option value="rw">可写挂载智能体工作区（rw）</option>
                        </select>
                        <p className="mt-1 text-[11px] text-gray-400"><span className="font-mono">none</span> 才是官方默认；<span className="font-mono">ro/rw</span> 只影响 workspace 挂载，不会自动影响其它 bind mount。</p>
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">沙箱工作区根</label>
                        <input
                          value={form.sandboxWorkspaceRoot}
                          onChange={e => update沙箱Form({ sandboxWorkspaceRoot: e.target.value })}
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
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">Docker 覆盖</h4>
                      <p className="text-xs text-gray-500 mt-1">这些字段决定容器根文件系统、网络和额外挂载；它们不会替代工具 allow/deny。</p>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div>
                        <label className="text-xs text-gray-500">容器网络</label>
                        <input
                          value={form.sandboxDockerNetwork}
                          onChange={e => update沙箱Form({ sandboxDockerNetwork: e.target.value })}
                          disabled={sandboxStructuredLocked}
                          placeholder="例如 none / bridge / custom-network"
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                        <p className="mt-1 text-[11px] text-gray-400">官方默认是无网络；只有确实需要下载依赖或联网工具时再显式开放。</p>
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">根文件系统只读</label>
                        <select
                          value={form.sandboxDockerReadOnlyRoot}
                          onChange={e => update沙箱Form({ sandboxDockerReadOnlyRoot: e.target.value as 继承默认Toggle })}
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
                        <label className="text-xs text-gray-500">一次性初始化命令</label>
                        <textarea
                          rows={3}
                          value={form.sandboxDockerSetupCommand}
                          onChange={e => update沙箱Form({ sandboxDockerSetupCommand: e.target.value })}
                          disabled={sandboxStructuredLocked}
                          placeholder="例如 apt-get update && apt-get install -y nodejs"
                          className="w-full mt-1 px-3 py-2 text-sm font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                        <p className="mt-1 text-[11px] text-gray-400">它只在容器创建时执行一次；如果需要 root / 可写根 / 网络，请与上面的字段一起配置。</p>
                      </div>
                      <div className="md:col-span-2">
                        <label className="text-xs text-gray-500">额外挂载</label>
                        <textarea
                          rows={4}
                          value={form.sandboxDockerBinds}
                          onChange={e => update沙箱Form({ sandboxDockerBinds: e.target.value })}
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
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">工具策略与逃逸口</h4>
                      <p className="text-xs text-gray-500 mt-1">这里不直接改 <span className="font-mono">tools.sandbox.tools</span> / <span className="font-mono">tools.elevated</span>，但要先理解它们和 sandbox 的关系。</p>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
                      <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3 text-gray-600 dark:text-gray-300">
                        <div className="font-medium text-gray-900 dark:text-white">沙箱</div>
                        <div className="mt-1">决定工具是在宿主机还是容器里运行。</div>
                      </div>
                      <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3 text-gray-600 dark:text-gray-300">
                        <div className="font-medium text-gray-900 dark:text-white">工具策略</div>
                        <div className="mt-1"><span className="font-mono">tools.allow / deny / sandbox.tools</span> 决定“这个工具能不能被调用”。</div>
                      </div>
                      <div className="rounded-lg bg-gray-50 dark:bg-gray-900 px-3 py-3 text-gray-600 dark:text-gray-300">
                        <div className="font-medium text-gray-900 dark:text-white">提权通道</div>
                        <div className="mt-1"><span className="font-mono">tools.elevated</span> 只是 <span className="font-mono">exec</span> 的宿主机逃逸口，不会授予被 deny 的工具。</div>
                      </div>
                    </div>
                    <p className="text-[11px] text-gray-400">如果你要进一步配置 <span className="font-mono">tools.sandbox.tools</span>、<span className="font-mono">tools.elevated</span>、<span className="font-mono">sandbox.browser.*</span>、<span className="font-mono">docker.image/user/env</span>，请继续在高级 JSON 里编辑。</p>
                  </div>

                  <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                    <div className="flex flex-col gap-2 lg:flex-row lg:items-start lg:justify-between">
                      <div>
                        <h4 className="text-sm font-semibold text-gray-900 dark:text-white">单智能体工具治理</h4>
                        <p className="text-xs text-gray-500 mt-1">对标官方智能体面板里最常用的工具访问起点：先选配置，再按需补 <span className="font-mono">tools.allow / tools.deny</span>。</p>
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
                        <label className="text-xs text-gray-500">工具配置（tools.profile）</label>
                        <select
                          value={form.toolProfile}
                          onChange={e => updateForm({ toolProfile: e.target.value as AgentFormState['toolProfile'] }, 'tools')}
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        >
                          {TOOL_POLICY_PRESETS.map(preset => (
                            <option key={preset.label} value={preset.id}>{preset.label}</option>
                          ))}
                        </select>
                        <p className="mt-1 text-[11px] text-gray-400">留在“继承默认”时不会写入单智能体 <span className="font-mono">tools.profile</span> 覆盖。</p>
                      </div>
                      <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                        <div>
                          <label className="text-xs text-gray-500">允许列表</label>
                          <textarea
                            rows={4}
                            value={form.toolAllow}
                            onChange={e => updateForm({ toolAllow: e.target.value }, 'tools')}
                            placeholder="逗号分隔，例如 group:web, group:fs"
                            className="w-full mt-1 px-3 py-2 text-sm font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                          />
                        </div>
                        <div>
                          <label className="text-xs text-gray-500">拒绝列表</label>
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
                    <p className="text-[11px] text-gray-400">这三项是最常用的单智能体工具治理入口；更细的 <span className="font-mono">tools.sandbox.tools</span> 与 <span className="font-mono">tools.elevated</span> 继续放在高级 JSON。</p>
                  </div>

                  <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">群聊行为</h4>
                      <p className="text-xs text-gray-500 mt-1">用于设置本智能体是否显式覆盖 <span className="font-mono">groupChat.enabled</span>。留在“继承默认”时，不会写入额外覆盖。</p>
                    </div>
                    {sandboxClearIntent && editingId && (
                      <div className="rounded-lg border border-amber-100 bg-amber-50 px-4 py-3 text-[12px] text-amber-700">
                        你当前选择了“继承默认”。保存时会优先移除结构化 sandbox 字段；如果当前 override 里只剩这些字段，就会删除整块 <span className="font-mono">sandbox</span>。若还有 <span className="font-mono">sandbox.browser.*</span> 等高级字段，它们会继续保留。
                      </div>
                    )}
                    <div className="max-w-xs">
                        <label className="text-xs text-gray-500">群聊开关</label>
                      <select
                        value={form.groupChatMode}
                        onChange={e => updateForm({ groupChatMode: e.target.value as 继承默认Toggle }, 'groupChat')}
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
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">全局协作策略说明</h4>
                      <p className="text-xs text-gray-500 mt-1">OpenClaw 2026.3.8 当前只支持系统级 <span className="font-mono">tools.agentToAgent</span> 与 <span className="font-mono">tools.sessions.visibility</span>，不支持单智能体覆盖。</p>
                    </div>
                    <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-900 dark:border-amber-900/50 dark:bg-amber-950/40 dark:text-amber-100">
                      <div className="font-medium">当前智能体页面不会再写入这两个字段。</div>
                      <div className="mt-2 text-xs leading-6">
                        历史上误写到 <span className="font-mono">agents.list[].tools.agentToAgent</span> /
                        <span className="font-mono"> agents.list[].tools.sessions.visibility</span> 的值，会在下一次保存时自动清理，避免 OpenClaw 报出
                        <span className="font-mono">未知键</span>。
                      </div>
                      <div className="mt-2 text-xs leading-6">
                        需要调整跨智能体委派或会话可见性时，请改系统级 <span className="font-mono">系统配置</span> 中的全局 <span className="font-mono">tools</span> 配置。
                      </div>
                    </div>
                  </div>

                  <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">子智能体允许列表</h4>
                      <p className="text-xs text-gray-500 mt-1">用于编辑 <span className="font-mono">subagents.allowAgents</span>。这是 OpenClaw 2026.3.8 支持的正式字段；留空表示不额外覆盖，默认仅允许同一智能体自己。</p>
                    </div>
                    <div>
                        <label className="text-xs text-gray-500">允许的智能体</label>
                      <input
                        value={form.subagentAllowAgents}
                        onChange={e => updateForm({ subagentAllowAgents: e.target.value }, 'subagents')}
                        placeholder="逗号分隔，例如 research, reviewer, *"
                        className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                      />
                      <p className="mt-1 text-[11px] text-gray-400">用于限制 <span className="font-mono">sessions_spawn</span> 可指定的 <span className="font-mono">agentId</span>；<span className="font-mono">*</span> 表示允许任意智能体。</p>
                    </div>
                  </div>
                </div>
              )}

              {formSection === 'advanced' && (
                <div className="space-y-5">
                  <div className="rounded-xl border border-violet-100 bg-violet-50 dark:bg-violet-900/10 dark:border-violet-900/30 px-4 py-3 flex flex-col md:flex-row md:items-center md:justify-between gap-3">
                    <div className="text-[12px] text-violet-700 dark:text-violet-200">
                      高级 JSON 保留完整编辑能力。保存时，结构化表单会覆盖相同路径上的值；如果你在这里改了结构化字段对应的 JSON，可先点击“从 JSON 同步结构化字段”。像 <span className="font-mono">tools.sandbox.tools</span>、<span className="font-mono">tools.elevated</span>、<span className="font-mono">sandbox.browser.*</span>、<span className="font-mono">docker.image/user/env</span> 这类高阶字段，仍建议直接在这里编辑。
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
                      <label htmlFor="agent-model-json" className="text-xs text-gray-500">模型（JSON）</label>
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
                      <label htmlFor="agent-params-json" className="text-xs text-gray-500">参数（JSON）</label>
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
                      <label htmlFor="agent-identity-json" className="text-xs text-gray-500">身份（JSON）</label>
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
                      <label htmlFor="agent-groupchat-json" className="text-xs text-gray-500">群聊（JSON）</label>
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
                      <label htmlFor="agent-tools-json" className="text-xs text-gray-500">工具（JSON）</label>
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
                      <label htmlFor="agent-subagents-json" className="text-xs text-gray-500">子智能体（JSON）</label>
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
                      <label htmlFor="agent-sandbox-json" className="text-xs text-gray-500">沙箱（JSON）</label>
                      <textarea
                        id="agent-sandbox-json"
                        name="agent沙箱Json"
                        rows={8}
                        value={form.sandboxText}
                        onChange={e => {
                          const value = e.target.value;
                          setForm(prev => {
                            const next = { ...prev, sandboxText: value };
                            try {
                              const parsed = value.trim() ? JSON.parse(value) : undefined;
                              const draft = extract沙箱Draft(parsed);
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
                          set沙箱ClearIntent(false);
                        }}
                        className="w-full mt-1 px-3 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900"
                      />
                    </div>
                    <div>
                      <label htmlFor="agent-runtime-json" className="text-xs text-gray-500">运行时（JSON）</label>
                      <textarea
                        id="agent-runtime-json"
                        name="agentRuntimeJson"
                        rows={8}
                        value={form.runtimeText}
                        onChange={e => setForm(prev => ({ ...prev, runtimeText: e.target.value }))}
                        placeholder="仅在需要单智能体 runtime 覆盖时填写"
                        className="w-full mt-1 px-3 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900"
                      />
                    </div>
                  </div>
                </div>
              )}
            </div>

            <div className={`sticky bottom-0 border-t backdrop-blur px-5 py-4 flex flex-col md:flex-row md:items-center md:justify-between gap-3 ${modern ? 'border-sky-200/60 dark:border-sky-900/40 bg-white/80 dark:bg-slate-900/75' : 'border-gray-100 dark:border-gray-700 bg-white/95 dark:bg-gray-800/95'}`}>
              <div className={`text-[11px] ${footerValidationIsError ? 'text-red-600' : 'text-gray-500'}`}>
                {footerValidationMessage}
              </div>
              <div className="flex items-center justify-end gap-2">
                <button onClick={closeForm} className={modern ? 'page-modern-action px-4 py-2 text-xs' : 'px-4 py-2 text-xs rounded bg-gray-100 dark:bg-gray-700'}>取消</button>
                <button onClick={saveAgent} disabled={saving || !!saveBlockedReason} title={saveBlockedReason || ''} className={modern ? 'page-modern-accent px-4 py-2 text-xs disabled:opacity-50 disabled:cursor-not-allowed' : 'px-4 py-2 text-xs rounded bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50 disabled:cursor-not-allowed'}>
                  {saving ? '保存中...' : '保存智能体'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

const Agents = memo(AgentsPage);
Agents.displayName = 'Agents';

export default Agents;

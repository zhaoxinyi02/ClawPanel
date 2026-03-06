import { useEffect, useMemo, useState } from 'react';
import { api } from '../lib/api';
import { Plus, RefreshCw, Save, Trash2, ArrowUp, ArrowDown, Route, Bot, Settings } from 'lucide-react';

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
  identityName: string;
  identityDescription: string;
  identityTone: string;
  groupChatMode: InheritToggle;
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
  identity: boolean;
  groupChat: boolean;
  tools: boolean;
  subagents: boolean;
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

const ALLOWED_MATCH_KEYS = ['channel', 'sender', 'peer', 'parentPeer', 'guildId', 'teamId', 'accountId', 'roles'];
const AGENT_FORM_SECTIONS: { id: AgentFormSection; title: string; description: string }[] = [
  { id: 'basic', title: 'Basic', description: '核心身份与目录' },
  { id: 'behavior', title: 'Behavior', description: '模型、常用参数、身份摘要' },
  { id: 'access', title: 'Access & Safety', description: 'sandbox 与安全协作开关' },
  { id: 'collaboration', title: 'Collaboration', description: '子 Agent 与跨 Agent 可见性' },
  { id: 'advanced', title: 'Advanced', description: '完整 JSON 覆盖与 runtime' },
];
const SANDBOX_STARTERS = [
  { key: 'inherit', label: '继承默认', help: '不写 sandbox 覆盖，沿用全局或默认 Agent 配置。', text: '' },
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
  identityDescription: '',
  identityTone: '',
  groupChatMode: 'inherit',
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
  groupChat: false,
  tools: false,
  subagents: false,
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

function parseNumberInput(raw: string, label: string): number | undefined {
  const text = raw.trim();
  if (!text) return undefined;
  const value = Number(text);
  if (!Number.isFinite(value)) throw new Error(`${label} 必须是数字`);
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

function detectSandboxStarter(raw: string): string {
  const text = raw.trim();
  if (!text) return 'inherit';
  try {
    const parsed = JSON.parse(text);
    if (isPlainObject(parsed)) {
      const mode = String(parsed.mode || '').trim();
      const workspaceAccess = String(parsed.workspaceAccess || '').trim();
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
  const identity = isPlainObject(agent?.identity) ? agent?.identity : {};
  const tools = isPlainObject(agent?.tools) ? agent?.tools : {};
  const subagents = isPlainObject(agent?.subagents) ? agent?.subagents : {};
  const groupChat = isPlainObject(agent?.groupChat) ? agent?.groupChat : {};

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
    identityName: String(identity.name || '').trim(),
    identityDescription: String(identity.description || '').trim(),
    identityTone: String(identity.tone || '').trim(),
    groupChatMode: triStateFromValue(getNestedValue(groupChat, 'enabled')),
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

export default function Agents() {
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState('');

  const [defaultAgent, setDefaultAgent] = useState('main');
  const [defaultConfigured, setDefaultConfigured] = useState(false);
  const [agents, setAgents] = useState<AgentItem[]>([]);
  const [bindings, setBindings] = useState<BindingDraft[]>([]);
  const [channelMeta, setChannelMeta] = useState<Record<string, ChannelMeta>>({});
  const [modelOptions, setModelOptions] = useState<string[]>([]);
  const [defaultModelHint, setDefaultModelHint] = useState('');

  const [editingId, setEditingId] = useState<string | null>(null);
  const [materializingImplicitAgent, setMaterializingImplicitAgent] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [formSection, setFormSection] = useState<AgentFormSection>('basic');
  const [form, setForm] = useState<AgentFormState>(DEFAULT_AGENT_FORM);
  const [saveAttempted, setSaveAttempted] = useState(false);
  const [structuredTouched, setStructuredTouched] = useState<AgentStructuredTouchedState>(DEFAULT_AGENT_STRUCTURED_TOUCHED);

  const [previewMeta, setPreviewMeta] = useState<Record<string, string>>({
    channel: '',
    sender: '',
    peer: '',
    parentPeer: '',
    guildId: '',
    teamId: '',
    accountId: '',
    roles: '',
  });
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

  const sandboxStarter = useMemo(() => detectSandboxStarter(form.sandboxText), [form.sandboxText]);
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

  const closeForm = () => {
    setShowForm(false);
    setMaterializingImplicitAgent(false);
    setSaveAttempted(false);
    setStructuredTouched(DEFAULT_AGENT_STRUCTURED_TOUCHED);
  };

  const loadData = async () => {
    setLoading(true);
    try {
      const [agentsRes, channelsRes, modelsRes] = await Promise.all([
        api.getAgentsConfig(),
        api.getChannels(),
        api.getModels(),
      ]);
      let nextDefaultModelHint = '';

      if (agentsRes?.ok) {
        const data = agentsRes.agents || {};
        const list: AgentItem[] = data.list || [];
        const incomingBindings = (data.bindings || []) as any[];
        const fallback = data.default || 'main';
        setDefaultAgent(fallback);
        setDefaultConfigured(data.defaultConfigured === true);
        setAgents(list);
        setBindings(incomingBindings.map((b: any) => toBindingDraft(b, fallback)));
        nextDefaultModelHint = extractModelDraft(data.defaults?.model).primary;
        setDefaultModelHint(nextDefaultModelHint);
      } else {
        setDefaultAgent('main');
        setDefaultConfigured(false);
        setAgents([]);
        setBindings([]);
        setDefaultModelHint('');
      }

      if (channelsRes?.ok) {
        setChannelMeta(parseChannelsMeta(channelsRes.channels || {}));
      } else {
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
    } catch {
      setDefaultAgent('main');
      setDefaultConfigured(false);
      setAgents([]);
      setBindings([]);
      setChannelMeta({});
      setModelOptions([]);
      setDefaultModelHint('');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  useEffect(() => {
    if (!showForm) return;
    if (msg.includes('不能为空') || msg.includes('已被占用') || msg.includes('仅支持') || msg.includes('已存在')) {
      setMsg('');
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [form]);

  const openCreate = () => {
    setMsg('');
    setMaterializingImplicitAgent(false);
    setSaveAttempted(false);
    setStructuredTouched(DEFAULT_AGENT_STRUCTURED_TOUCHED);
    setEditingId(null);
    setFormSection('basic');
    setForm(createAgentFormState());
    setShowForm(true);
  };

  const openEdit = (agent: AgentItem) => {
    setMsg('');
    const implicitAgent = isImplicitAgent(agent);
    setMaterializingImplicitAgent(implicitAgent);
    setSaveAttempted(false);
    setStructuredTouched(DEFAULT_AGENT_STRUCTURED_TOUCHED);
    setEditingId(agent.id);
    setFormSection('basic');
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
    setForm(prev => ({ ...prev, sandboxText: starter.text }));
  };

  const syncStructuredFieldsFromAdvanced = () => {
    try {
      const modelObj = parseJSONText(form.modelText, 'model');
      const toolsObj = parseJSONText(form.toolsText, 'tools');
      const groupChatObj = parseJSONText(form.groupChatText, 'groupChat');
      const identityObj = parseJSONText(form.identityText, 'identity');
      const subagentsObj = parseJSONText(form.subagentsText, 'subagents');
      const paramsObj = parseJSONText(form.paramsText, 'params');
      const modelDraft = extractModelDraft(modelObj);
      setForm(prev => ({
        ...prev,
        modelPrimary: modelDraft.primary,
        modelFallbacks: modelDraft.fallbacks,
        paramTemperature: isPlainObject(paramsObj) && paramsObj.temperature !== undefined ? String(paramsObj.temperature) : '',
        paramTopP: isPlainObject(paramsObj) && paramsObj.topP !== undefined ? String(paramsObj.topP) : '',
        paramMaxTokens: isPlainObject(paramsObj) && paramsObj.maxTokens !== undefined ? String(paramsObj.maxTokens) : '',
        identityName: isPlainObject(identityObj) ? String(identityObj.name || '').trim() : '',
        identityDescription: isPlainObject(identityObj) ? String(identityObj.description || '').trim() : '',
        identityTone: isPlainObject(identityObj) ? String(identityObj.tone || '').trim() : '',
        groupChatMode: triStateFromValue(isPlainObject(groupChatObj) ? getNestedValue(groupChatObj, 'enabled') : undefined),
        agentToAgentMode: triStateFromValue(isPlainObject(toolsObj) ? getNestedValue(toolsObj, 'agentToAgent.enabled') : undefined),
        agentToAgentAllow: isPlainObject(toolsObj) ? parseStringList(getNestedValue(toolsObj, 'agentToAgent.allow')).join(', ') : '',
        sessionVisibility: (() => {
          const raw = isPlainObject(toolsObj) ? getNestedValue(toolsObj, 'sessions.visibility') : '';
          return raw === 'same-agent' || raw === 'all-agents' ? raw : '';
        })(),
        subagentAllowAgents: isPlainObject(subagentsObj) ? parseStringList(getNestedValue(subagentsObj, 'allowAgents')).join(', ') : '',
      }));
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
        if (form.identityDescription.trim()) nextIdentity.description = form.identityDescription.trim();
        else delete nextIdentity.description;
        if (form.identityTone.trim()) nextIdentity.tone = form.identityTone.trim();
        else delete nextIdentity.tone;
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
    if (modelObj !== undefined) payload.model = modelObj;
    if (toolsObj !== undefined) payload.tools = toolsObj;
    if (sandboxObj !== undefined) payload.sandbox = sandboxObj;
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
      setMsg('Agent 保存成功');
      closeForm();
      await loadData();
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
  };

  const runPreview = async () => {
    const meta: Record<string, any> = {};
    Object.entries(previewMeta).forEach(([k, v]) => {
      if (!v.trim()) return;
      if (k === 'roles') {
        const roles = parseCSV(v);
        if (roles.length > 0) meta[k] = roles;
        return;
      }
      meta[k] = v.trim();
    });

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

      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-bold text-gray-900 dark:text-white">Agents</h2>
          <p className="text-sm text-gray-500 mt-1">管理 OpenClaw 多智能体、bindings 路由规则和命中预览</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={loadData} className="flex items-center gap-2 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 transition-colors shadow-sm">
            <RefreshCw size={14} /> 刷新
          </button>
          <button onClick={openCreate} className="flex items-center gap-2 px-4 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 shadow-sm shadow-violet-200 dark:shadow-none transition-all">
            <Plus size={14} /> 新建 Agent
          </button>
        </div>
      </div>

      {msg && (
        <div className={`px-4 py-3 rounded-lg text-sm ${msg.includes('失败') || msg.includes('错误') ? 'bg-red-50 dark:bg-red-900/20 text-red-600' : 'bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600'}`}>
          {msg}
        </div>
      )}

      <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-100 dark:border-gray-700/50 shadow-sm">
        <div className="px-4 py-3 border-b border-gray-100 dark:border-gray-700/50 flex items-center justify-between">
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <Bot size={15} className="text-violet-500" />
            Agent 列表
          </h3>
          <span className="text-xs text-gray-500">默认: {defaultAgent}</span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-xs text-gray-500 border-b border-gray-100 dark:border-gray-700/50">
                <th className="px-4 py-2">ID</th>
                <th className="px-4 py-2">Workspace</th>
                <th className="px-4 py-2">AgentDir</th>
                <th className="px-4 py-2">会话数</th>
                <th className="px-4 py-2">最后活跃</th>
                <th className="px-4 py-2">操作</th>
              </tr>
            </thead>
            <tbody>
              {agents.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-6 text-center text-gray-400 text-xs">暂无 Agent</td>
                </tr>
              ) : agents.map(agent => {
                const implicitAgent = isImplicitAgent(agent);
                return (
                <tr key={agent.id} className="border-b border-gray-50 dark:border-gray-700/30">
                  <td className="px-4 py-3">
                    <span className="font-mono text-xs">{agent.id}</span>
                    {agent.default && <span className="ml-2 text-[10px] px-1.5 py-0.5 rounded bg-violet-100 text-violet-700">DEFAULT</span>}
                    {implicitAgent && <span className="ml-2 text-[10px] px-1.5 py-0.5 rounded bg-sky-100 text-sky-700">PLACEHOLDER</span>}
                  </td>
                  <td className="px-4 py-3 text-xs text-gray-600 dark:text-gray-300">{agent.workspace || '-'}</td>
                  <td className="px-4 py-3 text-xs text-gray-600 dark:text-gray-300">{agent.agentDir || '-'}</td>
                  <td className="px-4 py-3 text-xs">{agent.sessions ?? 0}</td>
                  <td className="px-4 py-3 text-xs text-gray-500">{agent.lastActive ? new Date(agent.lastActive).toLocaleString('zh-CN') : '-'}</td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <button onClick={() => openEdit(agent)} className="px-2 py-1 text-xs rounded bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600">{implicitAgent ? '配置' : '编辑'}</button>
                      <button disabled={implicitAgent} title={implicitAgent ? '运行时占位 Agent 无需删除' : ''} onClick={() => deleteAgent(agent)} className="px-2 py-1 text-xs rounded bg-red-50 text-red-600 hover:bg-red-100 disabled:opacity-50 disabled:cursor-not-allowed">删除</button>
                    </div>
                  </td>
                </tr>
              )})}
            </tbody>
          </table>
        </div>
      </div>

      <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-100 dark:border-gray-700/50 shadow-sm">
        <div className="px-4 py-3 border-b border-gray-100 dark:border-gray-700/50 flex items-center justify-between">
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <Settings size={15} className="text-violet-500" />
            Bindings（结构化 + JSON 高级模式）
          </h3>
          <div className="flex items-center gap-2">
            <button onClick={addBinding} className="px-2 py-1 text-xs rounded bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600">新增规则</button>
            <button onClick={saveBindings} disabled={saving} className="flex items-center gap-1 px-3 py-1.5 text-xs rounded bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50">
              <Save size={12} /> 保存 Bindings
            </button>
          </div>
        </div>

        <div className="p-4 space-y-3">
          {bindings.length === 0 && (
            <div className="text-xs text-gray-400">暂无 bindings，消息将落到默认 Agent。</div>
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

            return (
              <div key={idx} className="border border-gray-100 dark:border-gray-700 rounded-lg p-3 space-y-3">
                <div className="flex items-center gap-2 flex-wrap">
                  <input
                    value={row.name}
                    onChange={e => setBindingAt(idx, r => ({ ...r, name: e.target.value }))}
                    placeholder="规则名（可选）"
                    className="px-2 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                  />
                  <select
                    value={row.agent}
                    onChange={e => setBindingAt(idx, r => ({ ...r, agent: e.target.value }))}
                    className="px-2 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                  >
                    {(agentOptions.length ? agentOptions : ['main']).map(id => (
                      <option key={id} value={id}>{id}</option>
                    ))}
                  </select>
                  <label className="text-xs text-gray-600 flex items-center gap-1">
                    <input
                      type="checkbox"
                      checked={row.enabled}
                      onChange={e => setBindingAt(idx, r => ({ ...r, enabled: e.target.checked }))}
                    />
                    启用
                  </label>
                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-slate-100 text-slate-600 dark:bg-slate-700 dark:text-slate-200">
                    优先级: {priority}
                  </span>
                  <button onClick={() => moveBinding(idx, -1)} className="p-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700" title="上移"><ArrowUp size={13} /></button>
                  <button onClick={() => moveBinding(idx, 1)} className="p-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700" title="下移"><ArrowDown size={13} /></button>
                  <button onClick={() => removeBinding(idx)} className="p-1 rounded text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20" title="删除"><Trash2 size={13} /></button>
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
                    JSON
                  </button>
                  <span className="text-[11px] text-gray-400">官方语义：省略 accountId 仅匹配默认账号</span>
                </div>

                {row.mode === 'structured' ? (
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                    <div>
                      <label className="text-[11px] text-gray-500">channel *</label>
                      <input
                        list="agent-channel-options"
                        value={channel}
                        onChange={e => touchBindingMatch(idx, cur => ({ ...cur, channel: e.target.value }))}
                        placeholder="whatsapp / telegram / discord"
                        className="w-full mt-1 px-2 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                      />
                    </div>

                    <div>
                      <label className="text-[11px] text-gray-500">accountId</label>
                      {accounts.length > 0 ? (
                        <select
                          value={accountId}
                          onChange={e => touchBindingMatch(idx, cur => ({ ...cur, accountId: e.target.value || undefined }))}
                          className="w-full mt-1 px-2 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
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
                          value={accountId}
                          onChange={e => touchBindingMatch(idx, cur => ({ ...cur, accountId: e.target.value }))}
                          placeholder="留空=默认账号，*=全部账号"
                          className="w-full mt-1 px-2 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                        />
                      )}
                      <p className="text-[10px] text-gray-400 mt-1">
                        {!accountId
                          ? `当前留空，仅匹配默认账号${defaultAccount ? ` (${defaultAccount})` : ''}`
                          : accountId === '*'
                            ? '匹配该 channel 的所有账号（兜底规则）'
                            : `仅匹配账号 ${accountId}`}
                      </p>
                    </div>

                    <div>
                      <label className="text-[11px] text-gray-500">sender</label>
                      <input
                        value={extractTextValue(match.sender)}
                        onChange={e => touchBindingMatch(idx, cur => ({ ...cur, sender: e.target.value }))}
                        placeholder="例如 +15551230001"
                        className="w-full mt-1 px-2 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                      />
                    </div>

                    <div>
                      <label className="text-[11px] text-gray-500">peer.kind</label>
                      <input
                        value={peer.kind}
                        onChange={e => setPeerField(idx, 'peer', 'kind', e.target.value)}
                        placeholder="direct / group"
                        className="w-full mt-1 px-2 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                      />
                    </div>
                    <div>
                      <label className="text-[11px] text-gray-500">peer.id</label>
                      <input
                        value={peer.id}
                        onChange={e => setPeerField(idx, 'peer', 'id', e.target.value)}
                        placeholder="+1555... / 1203...@g.us"
                        className="w-full mt-1 px-2 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                      />
                    </div>

                    <div>
                      <label className="text-[11px] text-gray-500">guildId</label>
                      <input
                        value={extractTextValue(match.guildId)}
                        onChange={e => touchBindingMatch(idx, cur => ({ ...cur, guildId: e.target.value }))}
                        placeholder="Discord guild id"
                        className="w-full mt-1 px-2 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                      />
                    </div>

                    <div>
                      <label className="text-[11px] text-gray-500">roles（逗号分隔）</label>
                      <input
                        value={extractRolesText(match)}
                        onChange={e => touchBindingMatch(idx, cur => ({ ...cur, roles: parseCSV(e.target.value) }))}
                        placeholder="admin, maintainer"
                        className="w-full mt-1 px-2 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                      />
                    </div>

                    <div>
                      <label className="text-[11px] text-gray-500">teamId</label>
                      <input
                        value={extractTextValue(match.teamId)}
                        onChange={e => touchBindingMatch(idx, cur => ({ ...cur, teamId: e.target.value }))}
                        placeholder="Slack team id"
                        className="w-full mt-1 px-2 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                      />
                    </div>

                    <div>
                      <label className="text-[11px] text-gray-500">parentPeer.kind</label>
                      <input
                        value={parentPeer.kind}
                        onChange={e => setPeerField(idx, 'parentPeer', 'kind', e.target.value)}
                        placeholder="thread / group"
                        className="w-full mt-1 px-2 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                      />
                    </div>
                    <div>
                      <label className="text-[11px] text-gray-500">parentPeer.id</label>
                      <input
                        value={parentPeer.id}
                        onChange={e => setPeerField(idx, 'parentPeer', 'id', e.target.value)}
                        placeholder="上级会话 id"
                        className="w-full mt-1 px-2 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
                      />
                    </div>
                  </div>
                ) : (
                  <textarea
                    value={row.matchText}
                    onChange={e => setBindingAt(idx, r => ({ ...r, matchText: e.target.value, rowError: '' }))}
                    rows={7}
                    className="w-full font-mono text-xs px-2 py-2 border border-gray-200 dark:border-gray-700 rounded bg-gray-50 dark:bg-gray-900"
                  />
                )}

                {row.rowError && (
                  <div className="text-xs text-red-600 bg-red-50 dark:bg-red-900/20 rounded px-2 py-1.5">
                    {row.rowError}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>

      <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-100 dark:border-gray-700/50 shadow-sm">
        <div className="px-4 py-3 border-b border-gray-100 dark:border-gray-700/50 flex items-center justify-between">
          <h3 className="text-sm font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <Route size={15} className="text-violet-500" />
            路由预览
          </h3>
          <button onClick={runPreview} disabled={previewLoading} className="px-3 py-1.5 text-xs rounded bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50">
            {previewLoading ? '预览中...' : '执行预览'}
          </button>
        </div>
        <div className="p-4 grid grid-cols-1 md:grid-cols-3 gap-3">
          {Object.keys(previewMeta).map(key => (
            <div key={key}>
              <label className="text-xs text-gray-500">{key}</label>
              <input
                value={previewMeta[key] || ''}
                onChange={e => setPreviewMeta(prev => ({ ...prev, [key]: e.target.value }))}
                className="w-full mt-1 px-2 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-900"
              />
            </div>
          ))}
        </div>
        <div className="px-4 pb-2 text-[11px] text-gray-400">
          roles 支持逗号分隔（会转为数组）；peer / parentPeer 可直接输入 <span className="font-mono">kind:id</span>。
        </div>
        {previewResult && (
          <div className="px-4 pb-4">
            <div className="rounded-lg bg-gray-50 dark:bg-gray-900 border border-gray-100 dark:border-gray-700 p-3 text-xs space-y-2">
              <div><span className="text-gray-500">命中 Agent:</span> <span className="font-mono text-violet-600">{previewResult.agent || '-'}</span></div>
              <div><span className="text-gray-500">匹配来源:</span> <span className="font-mono">{previewResult.matchedBy || '-'}</span></div>
              <div>
                <div className="text-gray-500 mb-1">Trace:</div>
                <ul className="list-disc pl-4 space-y-1">
                  {(previewResult.trace || []).map((line, i) => (
                    <li key={i} className="font-mono">{line}</li>
                  ))}
                </ul>
              </div>
            </div>
          </div>
        )}
      </div>

      {showForm && (
        <div className="fixed inset-0 z-50 bg-black/40 flex items-center justify-center p-4">
          <div className="w-full max-w-5xl max-h-[92vh] overflow-hidden rounded-xl bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 shadow-xl flex flex-col">
            <div className="sticky top-0 z-10 border-b border-gray-100 dark:border-gray-700 bg-white/95 dark:bg-gray-800/95 backdrop-blur">
              <div className="px-5 py-4 flex items-start justify-between gap-4">
                <div>
                  <h3 className="font-semibold text-gray-900 dark:text-white">{editingId ? `编辑 Agent: ${editingId}` : '新建 Agent'}</h3>
                  <p className="text-xs text-gray-500 mt-1">
                    先完成 Basic，再按需进入其它部分。Advanced 会保留完整 JSON 能力。
                  </p>
                </div>
                  <button onClick={closeForm} className="text-xs px-2.5 py-1.5 rounded bg-gray-100 dark:bg-gray-700 shrink-0">关闭</button>
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
                    <div className="text-gray-400">Model</div>
                    <div className="text-gray-700 dark:text-gray-200 mt-1 truncate">{form.modelPrimary.trim() || '继承 / Advanced JSON'}</div>
                  </div>
                  <div className="rounded-lg bg-gray-50 dark:bg-gray-900 border border-gray-100 dark:border-gray-700 px-3 py-2">
                    <div className="text-gray-400">Workspace</div>
                    <div className="font-mono text-gray-700 dark:text-gray-200 mt-1 truncate">{form.workspace.trim() || '未设置'}</div>
                  </div>
                  <div className="rounded-lg bg-gray-50 dark:bg-gray-900 border border-gray-100 dark:border-gray-700 px-3 py-2">
                    <div className="text-gray-400">Default</div>
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
                    <div>后端会校验唯一性：同一个 <span className="font-mono">agentDir</span> 不要复用；当前实现里 <span className="font-mono">workspace</span> 也不能与其它 Agent 重复。</div>
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
                      <label className="text-xs text-gray-500">Name</label>
                      <input
                        value={form.name}
                        onChange={e => updateForm({ name: e.target.value })}
                        placeholder="面向业务人员展示的名称"
                        className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                      />
                    </div>
                    <div>
                      <label className="text-xs text-gray-500">Workspace</label>
                      <input
                        value={form.workspace}
                        onChange={e => updateForm({ workspace: e.target.value })}
                        placeholder="/data/workspaces/support"
                        className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                      />
                      <p className="mt-1 text-[11px] text-gray-400">用于默认文件读写位置，不代表执行隔离。</p>
                      {workspaceConflict && (
                        <p className="mt-1 text-[11px] text-red-600">该 workspace 已被 Agent “{workspaceConflict.id}” 使用。</p>
                      )}
                    </div>
                    <div>
                      <label className="text-xs text-gray-500">AgentDir</label>
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
                    设为默认 Agent（未命中 bindings 时优先接管）
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
                        <h4 className="text-sm font-semibold text-gray-900 dark:text-white">模型选择</h4>
                        <p className="text-xs text-gray-500 mt-1">优先把常见选择做成结构化输入；更复杂的模型对象可在 Advanced 中继续编辑。</p>
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">Primary model</label>
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
                        <label className="text-xs text-gray-500">Fallback models</label>
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
                        <p className="text-xs text-gray-500 mt-1">这些参数会覆盖模型默认值；如需 provider 专属参数，请转到 Advanced / params JSON。</p>
                      </div>
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                        <div>
                          <label className="text-xs text-gray-500">temperature</label>
                          <input
                            value={form.paramTemperature}
                            onChange={e => updateForm({ paramTemperature: e.target.value }, 'params')}
                            placeholder="例如 0.7"
                            className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                          />
                        </div>
                        <div>
                          <label className="text-xs text-gray-500">topP</label>
                          <input
                            value={form.paramTopP}
                            onChange={e => updateForm({ paramTopP: e.target.value }, 'params')}
                            placeholder="例如 0.9"
                            className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                          />
                        </div>
                        <div>
                          <label className="text-xs text-gray-500">maxTokens</label>
                          <input
                            value={form.paramMaxTokens}
                            onChange={e => updateForm({ paramMaxTokens: e.target.value }, 'params')}
                            placeholder="例如 2048"
                            className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                          />
                        </div>
                      </div>
                    </div>
                  </div>

                  <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">身份摘要</h4>
                      <p className="text-xs text-gray-500 mt-1">提供对非技术用户最常见的身份信息。保存时写入 <span className="font-mono">identity.name / description / tone</span>。</p>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                      <div>
                        <label className="text-xs text-gray-500">Identity name</label>
                        <input
                          value={form.identityName}
                          onChange={e => updateForm({ identityName: e.target.value }, 'identity')}
                          placeholder="例如 Support Agent"
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                      </div>
                      <div className="md:col-span-2">
                        <label className="text-xs text-gray-500">Description</label>
                        <input
                          value={form.identityDescription}
                          onChange={e => updateForm({ identityDescription: e.target.value }, 'identity')}
                          placeholder="一句话说明职责与边界"
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                      </div>
                      <div className="md:col-span-3">
                        <label className="text-xs text-gray-500">Tone / style</label>
                        <input
                          value={form.identityTone}
                          onChange={e => updateForm({ identityTone: e.target.value }, 'identity')}
                          placeholder="例如 专业、克制、对外沟通友好"
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                      </div>
                    </div>
                  </div>
                </div>
              )}

              {formSection === 'access' && (
                <div className="space-y-5">
                  <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">Sandbox 起步模板</h4>
                      <p className="text-xs text-gray-500 mt-1">这里提供低门槛模板，方便快速开始；如需官方完整字段，请在 Advanced 里继续编辑 <span className="font-mono">sandbox</span> JSON。</p>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-[220px,1fr] gap-4">
                      <div>
                        <label className="text-xs text-gray-500">Starter preset</label>
                        <select
                          value={sandboxStarter}
                          onChange={e => applySandboxStarter(e.target.value)}
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
                  </div>

                  <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">群聊行为</h4>
                      <p className="text-xs text-gray-500 mt-1">用于设置本 Agent 是否显式覆盖 <span className="font-mono">groupChat.enabled</span>。留在“继承默认”时，不会写入额外覆盖。</p>
                    </div>
                    <div className="max-w-xs">
                      <label className="text-xs text-gray-500">groupChat.enabled</label>
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
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">Agent 间协作</h4>
                      <p className="text-xs text-gray-500 mt-1">结构化编辑会写回 <span className="font-mono">tools.agentToAgent</span> 与 <span className="font-mono">tools.sessions.visibility</span>。</p>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                      <div>
                        <label className="text-xs text-gray-500">Agent-to-agent delegation</label>
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
                        <label className="text-xs text-gray-500">Delegation allow rules</label>
                        <input
                          value={form.agentToAgentAllow}
                          onChange={e => updateForm({ agentToAgentAllow: e.target.value }, 'tools')}
                          placeholder="逗号分隔，例如 *, main->work, work->reviewer"
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        />
                        <p className="mt-1 text-[11px] text-gray-400">保存时会写为数组到 <span className="font-mono">tools.agentToAgent.allow</span>。</p>
                      </div>
                      <div>
                        <label className="text-xs text-gray-500">Session visibility</label>
                        <select
                          value={form.sessionVisibility}
                          onChange={e => updateForm({ sessionVisibility: e.target.value as '' | 'same-agent' | 'all-agents' }, 'tools')}
                          className="w-full mt-1 px-3 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                        >
                          <option value="">继承默认</option>
                          <option value="same-agent">same-agent</option>
                          <option value="all-agents">all-agents</option>
                        </select>
                      </div>
                    </div>
                  </div>

                  <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">子 Agent 允许列表</h4>
                      <p className="text-xs text-gray-500 mt-1">用于编辑 <span className="font-mono">subagents.allowAgents</span>。留空表示不额外覆盖。</p>
                    </div>
                    <div>
                      <label className="text-xs text-gray-500">Allowed agents</label>
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
                      Advanced 保留完整 JSON 编辑能力。保存时，结构化表单会覆盖相同路径上的值；如果你在这里改了结构化字段对应的 JSON，可先点击“从 JSON 同步结构化字段”。
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
                      <label className="text-xs text-gray-500">model (JSON)</label>
                      <textarea
                        rows={8}
                        value={form.modelText}
                        onChange={e => setForm(prev => ({ ...prev, modelText: e.target.value }))}
                        className="w-full mt-1 px-3 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900"
                      />
                    </div>
                    <div>
                      <label className="text-xs text-gray-500">params (JSON)</label>
                      <textarea
                        rows={8}
                        value={form.paramsText}
                        onChange={e => setForm(prev => ({ ...prev, paramsText: e.target.value }))}
                        className="w-full mt-1 px-3 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900"
                      />
                    </div>
                    <div>
                      <label className="text-xs text-gray-500">identity (JSON)</label>
                      <textarea
                        rows={8}
                        value={form.identityText}
                        onChange={e => setForm(prev => ({ ...prev, identityText: e.target.value }))}
                        className="w-full mt-1 px-3 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900"
                      />
                    </div>
                    <div>
                      <label className="text-xs text-gray-500">groupChat (JSON)</label>
                      <textarea
                        rows={8}
                        value={form.groupChatText}
                        onChange={e => setForm(prev => ({ ...prev, groupChatText: e.target.value }))}
                        className="w-full mt-1 px-3 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900"
                      />
                    </div>
                    <div>
                      <label className="text-xs text-gray-500">tools (JSON)</label>
                      <textarea
                        rows={8}
                        value={form.toolsText}
                        onChange={e => setForm(prev => ({ ...prev, toolsText: e.target.value }))}
                        className="w-full mt-1 px-3 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900"
                      />
                    </div>
                    <div>
                      <label className="text-xs text-gray-500">subagents (JSON)</label>
                      <textarea
                        rows={8}
                        value={form.subagentsText}
                        onChange={e => setForm(prev => ({ ...prev, subagentsText: e.target.value }))}
                        className="w-full mt-1 px-3 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900"
                      />
                    </div>
                    <div>
                      <label className="text-xs text-gray-500">sandbox (JSON)</label>
                      <textarea
                        rows={8}
                        value={form.sandboxText}
                        onChange={e => setForm(prev => ({ ...prev, sandboxText: e.target.value }))}
                        className="w-full mt-1 px-3 py-2 text-xs font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900"
                      />
                    </div>
                    <div>
                      <label className="text-xs text-gray-500">runtime (JSON)</label>
                      <textarea
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
                <button onClick={closeForm} className="px-4 py-2 text-xs rounded bg-gray-100 dark:bg-gray-700">取消</button>
                <button onClick={saveAgent} disabled={saving || !!saveBlockedReason} title={saveBlockedReason || ''} className="px-4 py-2 text-xs rounded bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50 disabled:cursor-not-allowed">
                  {saving ? '保存中...' : '保存 Agent'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

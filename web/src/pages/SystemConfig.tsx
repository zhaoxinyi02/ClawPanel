import { useEffect, useState, useRef } from 'react';
import { useOutletContext, useSearchParams } from 'react-router-dom';
import { api } from '../lib/api';
import {
  Save, RefreshCw, ChevronDown, ChevronRight,
  Brain, MessageSquare, Globe, Terminal, Webhook,
  Users, Eye, EyeOff, Key, Plus, Trash2,
  Monitor, HardDrive, FileText, Archive, RotateCcw,
  CheckCircle, AlertTriangle, Package, Box, Shield, Command
} from 'lucide-react';
import { useI18n } from '../i18n';

const KNOWN_PROVIDERS: { id: string; name: string; nameZh?: string; baseUrl: string; apiType?: string; apiKeyUrl: string; models: string[]; category: 'cn' | 'intl' | 'agg' }[] = [
  // === 国内主流 ===
  { id: 'volcengine', name: 'Volcengine Ark', nameZh: '火山方舟（字节）', baseUrl: 'https://ark.cn-beijing.volces.com/api/v3', apiKeyUrl: 'https://console.volcengine.com/ark/region:ark+cn-beijing/apiKey', models: ['doubao-pro-256k', 'doubao-lite-128k', 'deepseek-v3', 'deepseek-r1'], category: 'cn' },
  { id: 'deepseek', name: 'DeepSeek', nameZh: '深度求索', baseUrl: 'https://api.deepseek.com/v1', apiKeyUrl: 'https://platform.deepseek.com/api_keys', models: ['deepseek-chat', 'deepseek-reasoner'], category: 'cn' },
  { id: 'siliconflow', name: 'SiliconFlow', nameZh: '硅基流动', baseUrl: 'https://api.siliconflow.cn/v1', apiKeyUrl: 'https://cloud.siliconflow.cn/account/ak', models: ['deepseek-ai/DeepSeek-V3', 'deepseek-ai/DeepSeek-R1', 'Qwen/Qwen2.5-72B-Instruct', 'THUDM/glm-4-9b-chat'], category: 'cn' },
  { id: 'dashscope', name: 'DashScope', nameZh: '通义千问（阿里）', baseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1', apiKeyUrl: 'https://dashscope.console.aliyun.com/apiKey', models: ['qwen-max', 'qwen-plus', 'qwen-turbo', 'qwen-vl-max', 'qwen-coder-plus'], category: 'cn' },
  { id: 'ernie', name: 'Wenxin', nameZh: '文心一言（百度）', baseUrl: 'https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat', apiKeyUrl: 'https://console.bce.baidu.com/qianfan/ais/console/applicationConsole/application', models: ['ernie-4.0-8k', 'ernie-3.5-8k', 'ernie-speed-128k', 'ernie-lite-8k'], category: 'cn' },
  { id: 'hunyuan', name: 'Hunyuan', nameZh: '混元（腾讯）', baseUrl: 'https://api.hunyuan.cloud.tencent.com/v1', apiKeyUrl: 'https://console.cloud.tencent.com/cam/capi', models: ['hunyuan-pro', 'hunyuan-standard', 'hunyuan-lite', 'hunyuan-vision'], category: 'cn' },
  { id: 'zhipu', name: 'Zhipu AI', nameZh: '智谱清言（GLM）', baseUrl: 'https://open.bigmodel.cn/api/paas/v4', apiKeyUrl: 'https://open.bigmodel.cn/usercenter/apikeys', models: ['glm-4-plus', 'glm-4', 'glm-4-flash', 'glm-4v-plus'], category: 'cn' },
  { id: 'yi', name: 'Yi / Lingyiwanwu', nameZh: '零一万物', baseUrl: 'https://api.lingyiwanwu.com/v1', apiKeyUrl: 'https://platform.lingyiwanwu.com/apikeys', models: ['yi-large', 'yi-medium', 'yi-small', 'yi-vision'], category: 'cn' },
  { id: 'minimax', name: 'MiniMax', nameZh: 'MiniMax', baseUrl: 'https://api.minimax.chat/v1', apiKeyUrl: 'https://platform.minimaxi.com/user-center/basic-information/interface-key', models: ['MiniMax-Text-01', 'abab6.5s-chat', 'abab5.5-chat'], category: 'cn' },
  { id: 'spark', name: 'Spark', nameZh: '星火（讯飞）', baseUrl: 'https://spark-api-open.xf-yun.com/v1', apiKeyUrl: 'https://console.xfyun.cn/services/bm35', models: ['spark-pro-128k', 'spark-lite', 'spark-max'], category: 'cn' },
  // === 国际主流 ===
  { id: 'openai', name: 'OpenAI', baseUrl: 'https://api.openai.com/v1', apiKeyUrl: 'https://platform.openai.com/api-keys', models: ['gpt-4o', 'gpt-4o-mini', 'gpt-4-turbo', 'o1', 'o1-mini', 'o3-mini'], category: 'intl' },
  { id: 'anthropic', name: 'Anthropic', baseUrl: 'https://api.anthropic.com/v1', apiType: 'anthropic-messages', apiKeyUrl: 'https://console.anthropic.com/settings/keys', models: ['claude-sonnet-4-5', 'claude-haiku-3-5', 'claude-3-opus'], category: 'intl' },
  { id: 'google', name: 'Google Gemini', baseUrl: 'https://generativelanguage.googleapis.com/v1beta', apiType: 'google-generative-ai', apiKeyUrl: 'https://aistudio.google.com/app/apikey', models: ['gemini-2.5-pro', 'gemini-2.5-flash', 'gemini-2.0-flash', 'gemini-1.5-pro'], category: 'intl' },
  { id: 'xai', name: 'xAI (Grok)', baseUrl: 'https://api.x.ai/v1', apiKeyUrl: 'https://console.x.ai/', models: ['grok-3', 'grok-3-mini', 'grok-2'], category: 'intl' },
  { id: 'groq', name: 'Groq', baseUrl: 'https://api.groq.com/openai/v1', apiKeyUrl: 'https://console.groq.com/keys', models: ['llama-3.3-70b-versatile', 'mixtral-8x7b-32768', 'gemma2-9b-it'], category: 'intl' },
  // === 聚合平台 ===
  { id: 'openrouter', name: 'OpenRouter', baseUrl: 'https://openrouter.ai/api/v1', apiKeyUrl: 'https://openrouter.ai/keys', models: ['anthropic/claude-sonnet-4-5', 'openai/gpt-4o', 'google/gemini-2.5-pro', 'deepseek/deepseek-r1'], category: 'agg' },
  { id: 'together', name: 'Together AI', baseUrl: 'https://api.together.xyz/v1', apiKeyUrl: 'https://api.together.xyz/settings/api-keys', models: ['meta-llama/Llama-3.3-70B-Instruct-Turbo', 'Qwen/Qwen2.5-72B-Instruct-Turbo'], category: 'agg' },
  { id: 'nvidia', name: 'NVIDIA NIM', baseUrl: 'https://integrate.api.nvidia.com/v1', apiKeyUrl: 'https://build.nvidia.com/models', models: ['meta/llama-3.1-405b-instruct', 'minimaxai/minimax-m2.1'], category: 'agg' },
];

type ConfigTab = 'models' | 'identity' | 'general' | 'version' | 'env' | 'health';
type ConfigDiffItem = { path: string; before: string; after: string };
type BrowserControlPreset = 'disabled' | 'managed' | 'custom';
type BrowserProfileMode = 'openclaw' | 'chrome' | 'custom';
type ToolProfilePreset = 'minimal' | 'coding' | 'messaging' | 'full';
type CfgField = {
  path: string;
  label: string;
  type: 'text' | 'password' | 'number' | 'toggle' | 'textarea' | 'select';
  options?: string[];
  placeholder?: string;
  help?: string;
  min?: number;
  max?: number;
  integer?: boolean;
};

function cloneConfig<T>(value: T): T {
  return JSON.parse(JSON.stringify(value ?? {}));
}

function stringifyShort(value: any): string {
  if (value === undefined) return 'undefined';
  if (value === null) return 'null';
  if (typeof value === 'string') return value;
  try {
    const raw = JSON.stringify(value);
    if (raw.length > 120) return raw.slice(0, 117) + '...';
    return raw;
  } catch {
    return String(value);
  }
}

function buildConfigDiff(before: any, after: any, prefix = ''): ConfigDiffItem[] {
  const isObj = (v: any) => v && typeof v === 'object' && !Array.isArray(v);
  if (Array.isArray(before) || Array.isArray(after)) {
    const b = JSON.stringify(before ?? null);
    const a = JSON.stringify(after ?? null);
    if (b === a) return [];
    return [{ path: prefix || '(root)', before: stringifyShort(before), after: stringifyShort(after) }];
  }
  if (!isObj(before) || !isObj(after)) {
    if (JSON.stringify(before) === JSON.stringify(after)) return [];
    return [{ path: prefix || '(root)', before: stringifyShort(before), after: stringifyShort(after) }];
  }

  const keys = Array.from(new Set([...Object.keys(before), ...Object.keys(after)])).sort();
  let result: ConfigDiffItem[] = [];
  keys.forEach((k) => {
    const nextPath = prefix ? `${prefix}.${k}` : k;
    result = result.concat(buildConfigDiff(before?.[k], after?.[k], nextPath));
  });
  return result;
}

function readConfigValue(raw: any, path: string): any {
  return path.split('.').reduce((acc: any, key: string) => (acc && typeof acc === 'object' && !Array.isArray(acc) ? acc[key] : undefined), raw);
}

function validateNumericFieldValue(raw: any, field: Pick<CfgField, 'label' | 'min' | 'max' | 'integer'>): string | null {
  if (raw === undefined || raw === null || raw === '') return null;
  const value = typeof raw === 'number' ? raw : typeof raw === 'string' ? Number(raw.trim()) : Number.NaN;
  if (!Number.isFinite(value)) return `${field.label} 必须是数字`;
  if (field.integer && !Number.isInteger(value)) return `${field.label} 必须是整数`;
  if (field.min !== undefined && value < field.min) return `${field.label} 不能小于 ${field.min}`;
  if (field.max !== undefined && value > field.max) return `${field.label} 不能大于 ${field.max}`;
  return null;
}

function getBrowserConfigDraft(config: any): Record<string, any> {
  const browser = config?.browser;
  if (browser && typeof browser === 'object' && !Array.isArray(browser)) return browser;
  return {};
}

function getRawBrowserDefaultProfile(config: any): string {
  const browser = getBrowserConfigDraft(config);
  return typeof browser.defaultProfile === 'string' ? browser.defaultProfile.trim() : '';
}

function getEffectiveBrowserEnabled(config: any): boolean {
  const browser = getBrowserConfigDraft(config);
  return browser.enabled !== false;
}

function getEffectiveBrowserDefaultProfile(config: any): string {
  const raw = getRawBrowserDefaultProfile(config);
  return raw || 'openclaw';
}

function getBrowserControlPreset(config: any): BrowserControlPreset {
  if (!getEffectiveBrowserEnabled(config)) return 'disabled';
  return getEffectiveBrowserDefaultProfile(config) === 'openclaw' ? 'managed' : 'custom';
}

function getBrowserProfileMode(config: any): BrowserProfileMode {
  const effective = getEffectiveBrowserDefaultProfile(config);
  if (effective === 'openclaw' || effective === 'chrome') return effective;
  return 'custom';
}

function parseConfigListInput(value: string): string[] {
  return value
    .split(/[\n,，]+/)
    .map(item => item.trim())
    .filter(Boolean)
    .filter((item, index, arr) => arr.indexOf(item) === index);
}

function formatConfigList(value: any): string {
  if (Array.isArray(value)) return value.map(item => String(item || '').trim()).filter(Boolean).join(', ');
  if (typeof value === 'string') return value;
  return '';
}

const TOOL_GOVERNANCE_PRESETS: Record<ToolProfilePreset, { label: string; help: string }> = {
  minimal: {
    label: 'Minimal',
    help: '只保留最小会话能力，适合作为最保守的起点。',
  },
  coding: {
    label: 'Coding',
    help: '偏向编码/文件/运行时的常用组合，适合代码型 Agent。',
  },
  messaging: {
    label: 'Messaging',
    help: '偏向消息与渠道场景，适合聊天入口型 Agent。',
  },
  full: {
    label: 'Full',
    help: '完整工具面；只有在你明确知道后果时才建议长期使用。',
  },
};

export default function SystemConfig() {
  const { t: i18n } = useI18n();
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const [searchParams] = useSearchParams();
  const modern = uiMode === 'modern';
  const [config, setConfig] = useState<any>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState('');
  const [tab, setTab] = useState<ConfigTab>('models');
  const [versionInfo, setVersionInfo] = useState<any>({});
  const [backups, setBackups] = useState<any[]>([]);
  const [backingUp, setBackingUp] = useState(false);
  const [envInfo, setEnvInfo] = useState<any>({});
  const [envLoading, setEnvLoading] = useState(false);
  const [docs, setDocs] = useState<any[]>([]);
  const [selectedDoc, setSelectedDoc] = useState<any>(null);
  const [docContent, setDocContent] = useState('');
  const [docSaving, setDocSaving] = useState(false);
  const [identityDocs, setIdentityDocs] = useState<any[]>([]);
  const [selectedIdentityDoc, setSelectedIdentityDoc] = useState<any>(null);
  const [identityContent, setIdentityContent] = useState('');
  const [identitySaving, setIdentitySaving] = useState(false);
  const [adminToken, setAdminToken] = useState('');
  const [checking, setChecking] = useState(false);
  const [configIssues, setConfigIssues] = useState<any[]>([]);
  const [configChecked, setConfigChecked] = useState(0);
  const [configProblems, setConfigProblems] = useState(0);
  const [configCheckLoading, setConfigCheckLoading] = useState(false);
  const [fixing, setFixing] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [updateLog, setUpdateLog] = useState<string[]>([]);
  const [updateStatus, setUpdateStatus] = useState('idle');
  const [showRestartPrompt, setShowRestartPrompt] = useState(false);
  const [restarting, setRestarting] = useState(false);
  const [diagReport, setDiagReport] = useState('');
  const [diagLoading, setDiagLoading] = useState(false);
  const [originConfig, setOriginConfig] = useState<any>({});
  const [diffItems, setDiffItems] = useState<ConfigDiffItem[]>([]);
  const [showDiffPreview, setShowDiffPreview] = useState(false);

  useEffect(() => { loadConfig(); }, []);
  useEffect(() => {
    const nextTab = searchParams.get('tab');
    if (nextTab && ['models', 'identity', 'general', 'version', 'env', 'health'].includes(nextTab)) {
      setTab(nextTab as ConfigTab);
    }
  }, [searchParams]);

  const loadConfig = async () => {
    setLoading(true);
    try {
      const r = await api.getOpenClawConfig();
      if (r.ok) {
        const next = r.config || {};
        setConfig(next);
        setOriginConfig(cloneConfig(next));
      }
    }
    catch {} finally { setLoading(false); }
  };

  const loadVersion = async () => {
    const [v, b] = await Promise.all([api.getSystemVersion(), api.getBackups()]);
    if (v.ok) setVersionInfo(v);
    if (b.ok) setBackups(b.backups || []);
  };

  const loadEnv = async () => {
    setEnvLoading(true);
    try { const r = await api.getSystemEnv(); if (r.ok) setEnvInfo(r); }
    catch {} finally { setEnvLoading(false); }
  };

  const loadDocs = async () => {
    const r = await api.getDocs();
    if (r.ok) setDocs(r.docs || []);
  };

  const loadIdentityDocs = async () => {
    const r = await api.getIdentityDocs();
    if (r.ok) {
      setIdentityDocs(r.docs || []);
      if (!selectedIdentityDoc && r.docs?.length > 0) {
        setSelectedIdentityDoc(r.docs[0]);
        setIdentityContent(r.docs[0].content || '');
      }
    }
  };

  const loadAdminToken = async () => {
    const r = await api.getAdminToken();
    if (r.ok) setAdminToken(r.token || '');
  };

  const loadConfigCheck = async () => {
    setConfigCheckLoading(true);
    try {
      const r = await api.checkConfig();
      if (r.ok) {
        setConfigIssues(r.issues || []);
        setConfigChecked(r.checked || 0);
        setConfigProblems(r.problems || 0);
      }
    } catch {} finally { setConfigCheckLoading(false); }
  };

  const handleFixAll = async () => {
    const fixable = configIssues.filter((i: any) => i.fixable && (i.severity === 'error' || i.severity === 'warning'));
    if (fixable.length === 0) return;
    if (!confirm(`确定要自动修复 ${fixable.length} 个配置问题？修复后可能需要重启相关服务。`)) return;
    setFixing(true);
    try {
      const r = await api.fixConfig(fixable.map((i: any) => i.id));
      if (r.ok) {
        setMsg(`✅ 已修复 ${r.fixed?.length || 0} 个问题${r.napcatRestart ? '，NapCat 容器正在重启...' : ''}`);
        setTimeout(() => loadConfigCheck(), 2000);
      }
    } catch (err) { setMsg('修复失败: ' + String(err)); }
    finally { setFixing(false); setTimeout(() => setMsg(''), 6000); }
  };

  const handleFixSingle = async (issueId: string) => {
    setFixing(true);
    try {
      const r = await api.fixConfig([issueId]);
      if (r.ok) {
        setMsg(`✅ 已修复${r.napcatRestart ? '，NapCat 容器正在重启...' : ''}`);
        setTimeout(() => loadConfigCheck(), 2000);
      }
    } catch (err) { setMsg('修复失败: ' + String(err)); }
    finally { setFixing(false); setTimeout(() => setMsg(''), 6000); }
  };

  useEffect(() => {
    if (tab === 'version') loadVersion();
    if (tab === 'env') loadEnv();
    if (tab === 'identity') { loadIdentityDocs(); loadAdminToken(); }
    if (tab === 'health') loadConfigCheck();
  }, [tab]);

  const getVal = (path: string): any => path.split('.').reduce((o: any, k: string) => o?.[k], config);
  const syncAllowedModels = (draft: any) => {
    const current = draft?.agents?.defaults?.models;
    if (!current || typeof current !== 'object' || Array.isArray(current)) return;

    const next: Record<string, any> = {};
    const providers = draft?.models?.providers || {};

    Object.entries(providers).forEach(([pid, prov]: [string, any]) => {
      (prov?.models || []).forEach((model: any) => {
        const mid = typeof model === 'string' ? model : model?.id;
        if (!pid || !mid) return;
        const key = `${pid}/${mid}`;
        next[key] = current[key] && typeof current[key] === 'object' ? current[key] : {};
      });
    });

    draft.agents.defaults.models = next;
  };
  const updateConfig = (mutate: (draft: any) => void) => {
    setConfig((prev: any) => {
      const clone = JSON.parse(JSON.stringify(prev));
      mutate(clone);
      syncAllowedModels(clone);
      return clone;
    });
  };
  const setVal = (path: string, value: any) => {
    updateConfig((clone: any) => {
      const keys = path.split('.');
      let cur = clone;
      for (let i = 0; i < keys.length - 1; i++) {
        if (!cur[keys[i]] || typeof cur[keys[i]] !== 'object' || Array.isArray(cur[keys[i]])) cur[keys[i]] = {};
        cur = cur[keys[i]];
      }
      cur[keys[keys.length - 1]] = value;
    });
  };

  const normalizeConfigForSave = (input: any) => {
    const clone = cloneConfig(input || {});
    const defaults = clone?.agents?.defaults;
    if (defaults && typeof defaults === 'object') {
      const model = defaults.model;
      if (typeof model === 'string') {
        const primary = model.trim();
        if (primary) defaults.model = { primary };
        else delete defaults.model;
      } else if (model && typeof model === 'object' && !Array.isArray(model)) {
        if (defaults.contextTokens == null && model.contextTokens != null) {
          const n = Number(model.contextTokens);
          if (Number.isFinite(n) && n > 0) defaults.contextTokens = n;
        }
        const cleaned: any = {};
        if (typeof model.primary === 'string' && model.primary.trim()) cleaned.primary = model.primary.trim();
        if (Array.isArray(model.fallbacks)) {
          const fb = model.fallbacks.filter((x: any) => typeof x === 'string' && x.trim()).map((x: string) => x.trim());
          if (fb.length > 0) cleaned.fallbacks = fb;
        }
        if (Object.keys(cleaned).length > 0) defaults.model = cleaned;
        else delete defaults.model;
      }
      const compactionMode = defaults?.compaction?.mode;
      if (compactionMode === 'aggressive') defaults.compaction.mode = 'safeguard';
      if (compactionMode === 'off') defaults.compaction.mode = 'default';
    }

    const gateway = clone?.gateway;
    if (gateway && typeof gateway === 'object') {
      if (gateway.mode === 'hosted') gateway.mode = 'remote';
      if ((!gateway.customBindHost || !String(gateway.customBindHost).trim()) && gateway.bindAddress) {
        gateway.customBindHost = String(gateway.bindAddress).trim();
      }
      if ('bindAddress' in gateway) delete gateway.bindAddress;
    }

    const hooks = clone?.hooks;
    if (hooks && typeof hooks === 'object') {
      if ((!hooks.path || !String(hooks.path).trim()) && hooks.basePath) hooks.path = String(hooks.basePath).trim();
      if ((!hooks.token || !String(hooks.token).trim()) && hooks.secret) hooks.token = hooks.secret;
      if ('basePath' in hooks) delete hooks.basePath;
      if ('secret' in hooks) delete hooks.secret;
    }

    const browser = clone?.browser;
    if (browser && typeof browser === 'object' && !Array.isArray(browser)) {
      if (browser.enabled === false) {
        delete browser.defaultProfile;
      } else if (typeof browser.defaultProfile === 'string') {
        const trimmed = browser.defaultProfile.trim();
        if (trimmed) browser.defaultProfile = trimmed;
        else delete browser.defaultProfile;
      }
    }

    const numericFields: CfgField[] = [
      { path: 'session.maintenance.maxEntries', label: '会话条目上限', type: 'number', integer: true, min: 1 },
      { path: 'agents.defaults.contextTokens', label: '默认上下文 Token 预算', type: 'number', integer: true, min: 1 },
      { path: 'agents.defaults.maxConcurrent', label: '最大并发', type: 'number', integer: true, min: 1 },
      { path: 'agents.defaults.compaction.maxHistoryShare', label: '历史占比上限', type: 'number', min: 0, max: 1 },
      { path: 'gateway.port', label: '端口', type: 'number', integer: true, min: 1, max: 65535 },
      { path: 'session.agentToAgent.maxPingPongTurns', label: '最大来回委托轮次', type: 'number', integer: true, min: 1 },
    ];
    for (const field of numericFields) {
      const error = validateNumericFieldValue(readConfigValue(clone, field.path), field);
      if (error) throw new Error(error);
    }

    return clone;
  };

  const setPrimaryModel = (value: string) => {
    setConfig((prev: any) => {
      const clone = cloneConfig(prev || {});
      if (!clone.agents || typeof clone.agents !== 'object') clone.agents = {};
      if (!clone.agents.defaults || typeof clone.agents.defaults !== 'object') clone.agents.defaults = {};
      const defaults = clone.agents.defaults;
      const currentModel = defaults.model;
      const nextModel =
        currentModel && typeof currentModel === 'object' && !Array.isArray(currentModel)
          ? { ...currentModel }
          : {};
      if (value) nextModel.primary = value;
      else delete nextModel.primary;
      if (Object.keys(nextModel).length > 0) defaults.model = nextModel;
      else delete defaults.model;
      return clone;
    });
  };

  const doSave = async () => {
    setSaving(true); setMsg('');
    try {
      const normalized = normalizeConfigForSave(config);
      await api.updateOpenClawConfig(normalized);
      setConfig(normalized);
      setMsg(i18n.sysConfig.saveSuccess);
      setOriginConfig(cloneConfig(normalized));
      setShowDiffPreview(false);
      // If on models tab, prompt to restart gateway
      if (tab === 'models') {
        setMsg('✅ 配置已保存！模型配置变更需要重启 OpenClaw 网关才能生效。');
        setShowRestartPrompt(true);
      }
      setTimeout(() => setMsg(''), 6000);
    } catch (err) { setMsg(i18n.sysConfig.saveFailed + ': ' + String(err)); }
    finally { setSaving(false); }
  };

  const handleSave = async () => {
    const diff = buildConfigDiff(originConfig || {}, config || {});
    if (diff.length === 0) {
      setMsg('未检测到配置变更');
      setTimeout(() => setMsg(''), 3000);
      return;
    }
    setDiffItems(diff);
    setShowDiffPreview(true);
  };

  const handleBackup = async () => {
    setBackingUp(true);
    try { const r = await api.createBackup(); if (r.ok) { setMsg(i18n.sysConfig.backupConfig + ' ' + i18n.common.success); loadVersion(); } }
    catch (err) { setMsg(i18n.common.failed + ': ' + String(err)); }
    finally { setBackingUp(false); setTimeout(() => setMsg(''), 3000); }
  };

  const handleRestore = async (name: string) => {
    if (!confirm(i18n.sysConfig.restoreConfirm)) return;
    try {
      const r = await api.restoreBackup(name);
      if (r.ok) { setMsg(i18n.common.success); loadConfig(); loadVersion(); }
    } catch (err) { setMsg(i18n.common.failed + ': ' + String(err)); }
    setTimeout(() => setMsg(''), 4000);
  };

  const handleSaveDoc = async () => {
    if (!selectedDoc) return;
    setDocSaving(true);
    try { await api.saveDoc(selectedDoc.path, docContent); setMsg(i18n.common.success); loadDocs(); }
    catch (err) { setMsg(i18n.sysConfig.saveFailed + ': ' + String(err)); }
    finally { setDocSaving(false); setTimeout(() => setMsg(''), 3000); }
  };

  if (loading) return <div className="text-center py-12 text-gray-400 text-xs">{i18n.common.loading}</div>;

  const providers = config?.models?.providers || {};
  const primaryModelRaw = config?.agents?.defaults?.model;
  const primaryModel = typeof primaryModelRaw === 'string' ? primaryModelRaw : (primaryModelRaw?.primary || '');

  return (
    <div className={`space-y-6 ${modern ? 'page-modern' : ''}`}>
      <div className={`${modern ? 'page-modern-header' : 'flex items-center justify-between'}`}>
        <div>
          <h2 className={`${modern ? 'page-modern-title text-xl' : 'text-xl font-bold text-gray-900 dark:text-white tracking-tight'}`}>{i18n.sysConfig.title}</h2>
          <p className={`${modern ? 'page-modern-subtitle text-sm' : 'text-sm text-gray-500 mt-1'}`}>{i18n.sysConfig.subtitle}</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={loadConfig} className={`${modern ? 'page-modern-control px-3.5 py-2 text-xs font-medium' : 'flex items-center gap-2 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 transition-colors shadow-sm'}`}>
            <RefreshCw size={14} />{i18n.common.refresh}
          </button>
          {(tab === 'models' || tab === 'general' || tab === 'identity') && (
            <button onClick={handleSave} disabled={saving}
              className={`${modern ? 'page-modern-accent px-4 py-2 text-xs font-medium disabled:opacity-50' : 'flex items-center gap-2 px-4 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 shadow-sm shadow-violet-200 dark:shadow-none transition-all hover:shadow-md hover:shadow-violet-200 dark:hover:shadow-none disabled:opacity-50'}`}>
              {saving ? <RefreshCw size={14} className="animate-spin" /> : <Save size={14} />}
              {saving ? i18n.sysConfig.savingConfig : i18n.sysConfig.saveAll}
            </button>
          )}
        </div>
      </div>

      {msg && (
        <div className={`px-4 py-3 rounded-xl text-sm font-medium flex items-center gap-2 ${msg.includes('失败') ? 'bg-red-50 dark:bg-red-900/30 text-red-600' : 'bg-emerald-50 dark:bg-emerald-900/30 text-emerald-600'}`}>
          {msg.includes('失败') ? <AlertTriangle size={16} /> : <CheckCircle size={16} />}
          {msg}
        </div>
      )}

      {/* Tabs */}
      <div className={`${modern ? 'inline-flex flex-wrap gap-2 p-1 rounded-2xl border border-blue-100/70 bg-[linear-gradient(145deg,rgba(255,255,255,0.78),rgba(239,246,255,0.62))] dark:bg-[linear-gradient(145deg,rgba(10,20,36,0.82),rgba(30,64,175,0.1))] dark:border-blue-800/20 shadow-sm backdrop-blur-xl' : 'flex gap-6 border-b border-gray-200 dark:border-gray-800 overflow-x-auto pb-px'}`}>
        {([
          { id: 'models' as ConfigTab, label: i18n.sysConfig.tabModels, icon: Brain },
          { id: 'identity' as ConfigTab, label: i18n.sysConfig.tabIdentity, icon: Users },
          { id: 'general' as ConfigTab, label: i18n.sysConfig.tabGeneral, icon: Terminal },
          { id: 'version' as ConfigTab, label: i18n.sysConfig.tabVersion, icon: Package },
          { id: 'env' as ConfigTab, label: i18n.sysConfig.tabEnv, icon: Monitor },
          { id: 'health' as ConfigTab, label: '配置检测', icon: Shield },
        ]).map(tb => (
          <button key={tb.id} onClick={() => setTab(tb.id)}
            className={`${modern ? 'flex items-center gap-2 px-3.5 py-2 rounded-xl text-sm font-medium border transition-all whitespace-nowrap' : 'flex items-center gap-2 pb-3 text-sm font-medium border-b-2 transition-all whitespace-nowrap'} ${tab === tb.id ? (modern ? 'border-blue-100/80 bg-blue-50/85 dark:bg-blue-900/20 dark:border-blue-800/40 text-blue-700 dark:text-blue-300 shadow-sm' : 'border-violet-600 text-violet-700 dark:text-violet-400') : (modern ? 'border-transparent text-gray-500 hover:bg-white/70 dark:hover:bg-slate-800/70 hover:text-gray-700 dark:hover:text-gray-300' : 'border-transparent text-gray-500 hover:text-gray-700 dark:hover:text-gray-300')}`}>
            <tb.icon size={16} />{tb.label}
          </button>
        ))}
      </div>

      {/* === Models Tab === */}
      {tab === 'models' && (
        <div className="space-y-6 animate-in fade-in slide-in-from-top-4 duration-200">
          {showRestartPrompt && (
            <div className={`${modern ? 'rounded-[24px] bg-[linear-gradient(145deg,rgba(255,251,235,0.86),rgba(239,246,255,0.62))] dark:bg-[linear-gradient(145deg,rgba(120,53,15,0.2),rgba(30,64,175,0.12))] border border-amber-200/70 dark:border-amber-800/30 overflow-hidden shadow-[0_16px_34px_rgba(245,158,11,0.08)] backdrop-blur-xl' : 'rounded-xl bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800/30 overflow-hidden'}`}>
              <div className="px-4 py-3 flex items-center justify-between gap-3">
                <div className="flex items-center gap-2 text-sm text-amber-700 dark:text-amber-300">
                  <AlertTriangle size={16} className="shrink-0" />
                  <span>模型配置已保存。OpenClaw 需要<strong>重启网关</strong>才能使用新模型。</span>
                </div>
                <button onClick={async () => {
                  setRestarting(true);
                  try {
                    await api.restartGateway();
                    setMsg('✅ 网关重启请求已发送，请等待几秒钟');
                    setShowRestartPrompt(false);
                  } catch { setMsg('❌ 重启失败'); }
                  finally { setRestarting(false); setTimeout(() => setMsg(''), 4000); }
                }} disabled={restarting}
                  className={`${modern ? 'page-modern-warn px-4 py-2 text-xs font-medium whitespace-nowrap shrink-0' : 'flex items-center gap-1.5 px-4 py-2 text-xs font-medium rounded-lg bg-amber-500 text-white hover:bg-amber-600 disabled:opacity-50 shadow-sm transition-all whitespace-nowrap shrink-0'}`}>
                  <RefreshCw size={12} className={restarting ? 'animate-spin' : ''} />
                  {restarting ? '重启中...' : '重启网关'}
                </button>
              </div>
              <div className="px-4 pb-3 text-[11px] text-amber-600/80 dark:text-amber-400/70 leading-relaxed">
                💡 重启后如果 OpenClaw 回复「Message ordering conflict」，请发送 <code className="bg-amber-100 dark:bg-amber-900/40 px-1 rounded font-mono">/new</code> 开始新会话即可。这是 OpenClaw 切换模型后的正常现象。
              </div>
            </div>
          )}
          <div className={`${modern ? 'page-modern-panel p-5' : 'bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 p-5'}`}>
            <h3 className="text-sm font-bold text-gray-900 dark:text-white mb-3 flex items-center gap-2">
              <Brain size={16} className="text-blue-500" /> {i18n.sysConfig.primaryModel}
            </h3>
            <div className="relative">
              <select value={primaryModel} onChange={e => setPrimaryModel(e.target.value)}
                className="w-full pl-4 pr-10 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all font-mono appearance-none cursor-pointer">
                <option value="">选择主模型...</option>
                {Object.entries(providers).map(([pid, prov]: [string, any]) => {
                  const hasKey = !!(prov as any).apiKey;
                  return (prov.models || []).map((m: any) => {
                    const mid = typeof m === 'string' ? m : m.id;
                    const val = `${pid}/${mid}`;
                    return <option key={val} value={val} disabled={!hasKey} style={!hasKey ? { color: '#9ca3af' } : {}}>{val}{!hasKey ? ' (未配置 API Key)' : ''}</option>;
                  });
                })}
              </select>
              <ChevronDown size={14} className="absolute right-3.5 top-1/2 -translate-y-1/2 text-gray-400 pointer-events-none" />
            </div>
            {primaryModel && (
              <p className="text-xs text-gray-500 mt-2 flex items-center gap-1.5">
                <span className="w-1.5 h-1.5 rounded-full bg-emerald-400"></span>
                当前: <code className="bg-gray-100 dark:bg-gray-800 px-1 rounded text-blue-600 dark:text-blue-400 font-mono">{primaryModel}</code>
              </p>
            )}
          </div>

          {/* Quick add provider from presets */}
          <div className={`${modern ? 'page-modern-panel p-5 space-y-3' : 'bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 p-5 space-y-3'}`}>
            <h3 className="text-sm font-bold text-gray-900 dark:text-white flex items-center gap-2">
              <Plus size={16} className="text-blue-500" /> 快速添加模型服务商
            </h3>
            <p className="text-xs text-gray-500">点击服务商名称一键添加，填入 API Key 即可使用</p>
            <div className="space-y-2">
              {[
                { label: '国内主流', cat: 'cn' as const, color: 'bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400 border-red-100 dark:border-red-800/30 hover:bg-red-100 dark:hover:bg-red-900/40' },
                { label: '国际主流', cat: 'intl' as const, color: 'bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400 border-blue-100 dark:border-blue-800/30 hover:bg-blue-100 dark:hover:bg-blue-900/40' },
                { label: '聚合平台', cat: 'agg' as const, color: 'bg-amber-50 dark:bg-amber-900/20 text-amber-600 dark:text-amber-400 border-amber-100 dark:border-amber-800/30 hover:bg-amber-100 dark:hover:bg-amber-900/40' },
              ].map(({ label, cat, color }) => (
                <div key={cat} className="flex items-center gap-1.5 flex-wrap">
                  <span className="text-[9px] font-bold text-gray-400 uppercase tracking-wider w-12 shrink-0">{label}</span>
                  {KNOWN_PROVIDERS.filter(kp => kp.category === cat).map(kp => {
                    const alreadyAdded = Object.keys(providers).includes(kp.id);
                    return (
                      <button key={kp.id} disabled={alreadyAdded} onClick={() => {
                        updateConfig((clone: any) => {
                        if (!clone.models) clone.models = {};
                        if (!clone.models.providers) clone.models.providers = {};
                        clone.models.providers[kp.id] = {
                          baseUrl: kp.baseUrl,
                          apiKey: '',
                          api: kp.apiType || 'openai-completions',
                          models: kp.models.map(m => ({ id: m, name: m, contextWindow: 128000, maxTokens: 8192 })),
                        };
                        });
                      }} className={`px-2 py-0.5 text-[10px] font-medium rounded-md border transition-colors ${alreadyAdded ? 'opacity-40 cursor-not-allowed bg-gray-50 dark:bg-gray-800 text-gray-400 border-gray-200 dark:border-gray-700' : color}`}
                        title={alreadyAdded ? '已添加' : `点击添加 ${kp.nameZh || kp.name}`}>
                        {kp.nameZh || kp.name}{alreadyAdded ? ' ✓' : ''}
                      </button>
                    );
                  })}
                </div>
              ))}
            </div>
          </div>

          <div className="space-y-4">
            <div className="flex items-center justify-between px-1">
              <h3 className="text-sm font-bold text-gray-500 uppercase tracking-wider">{i18n.sysConfig.modelProviders} ({Object.keys(providers).length})</h3>
              <button onClick={() => {
                const id = `provider-${Date.now()}`;
                updateConfig((clone: any) => {
                  if (!clone.models) clone.models = {};
                  if (!clone.models.providers) clone.models.providers = {};
                  clone.models.providers[id] = { baseUrl: '', apiKey: '', api: 'openai-completions', models: [] };
                });
              }} className={`${modern ? 'page-modern-action px-3 py-1.5 text-xs font-medium' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-violet-50 dark:bg-violet-900/30 text-violet-600 dark:text-violet-400 hover:bg-violet-100 dark:hover:bg-violet-900/50 transition-colors'}`}>
                <Plus size={14} />{i18n.sysConfig.addProvider}
              </button>
            </div>

            {Object.entries(providers).map(([pid, prov]: [string, any]) => (
              <div key={pid} className={`${modern ? 'page-modern-panel overflow-hidden transition-all hover:shadow-md' : 'bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 overflow-hidden transition-all hover:shadow-md'}`}>
                <div className="p-5 space-y-5">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                        <div className="p-2 rounded-xl border border-blue-100/80 dark:border-blue-800/40 bg-[linear-gradient(135deg,rgba(37,99,235,0.12),rgba(14,165,233,0.08))] dark:bg-[linear-gradient(135deg,rgba(37,99,235,0.2),rgba(14,165,233,0.12))] text-blue-600 dark:text-blue-300">
                          <Brain size={18} />
                        </div>
                      <div className="flex items-baseline gap-2">
                        <input value={pid} onChange={e => {
                          const newId = e.target.value;
                          if (!newId || newId === pid) return;
                          updateConfig((clone: any) => {
                          clone.models.providers[newId] = clone.models.providers[pid];
                          delete clone.models.providers[pid];
                          const modelCfg = clone.agents?.defaults?.model;
                          const primary = typeof modelCfg === 'string' ? modelCfg : (modelCfg?.primary || '');
                          if (primary.startsWith(pid + '/')) {
                            if (!clone.agents) clone.agents = {};
                            if (!clone.agents.defaults || typeof clone.agents.defaults !== 'object') clone.agents.defaults = {};
                            if (!clone.agents.defaults.model || typeof clone.agents.defaults.model !== 'object' || Array.isArray(clone.agents.defaults.model)) {
                              clone.agents.defaults.model = {};
                            }
                            clone.agents.defaults.model.primary = newId + primary.slice(pid.length);
                          }
                          });
                        }} className="text-base font-bold bg-transparent border-b border-dashed border-gray-300 dark:border-gray-600 focus:border-blue-500 outline-none px-1 py-0.5 min-w-[120px] transition-colors text-gray-900 dark:text-white" title="点击编辑 Provider ID" />
                        {prov.models?.length > 0 && <span className="text-xs text-gray-400 font-medium px-2 py-0.5 bg-gray-50 dark:bg-gray-800 rounded-full">{prov.models.length} 模型</span>}
                      </div>
                    </div>
                    <button onClick={() => {
                      updateConfig((clone: any) => {
                        delete clone.models.providers[pid];
                      });
                    }} className="p-2 text-gray-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"><Trash2 size={16} /></button>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
                    <div>
                      <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300 mb-1.5">Base URL</label>
                      <input value={prov.baseUrl || ''} onChange={e => setVal(`models.providers.${pid}.baseUrl`, e.target.value)}
                        placeholder="https://api.openai.com/v1" className="w-full px-3.5 py-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50/50 dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all font-mono" />
                    </div>
                    <div>
                      <div className="flex items-center justify-between mb-1.5">
                        <label className="text-xs font-semibold text-gray-700 dark:text-gray-300">API Key</label>
                        {(() => {
                          const matched = KNOWN_PROVIDERS.find(kp => prov.baseUrl?.includes(kp.baseUrl.replace('https://', '').split('/')[0]));
                          return matched ? (
                            <a href={matched.apiKeyUrl} target="_blank" rel="noopener noreferrer"
                              className="text-[10px] text-blue-500 hover:text-blue-700 dark:hover:text-blue-300 flex items-center gap-1 hover:underline">
                              <Key size={10} /> 获取 API Key
                            </a>
                          ) : null;
                        })()}
                      </div>
                      <div className="relative group">
                        <input type="password" value={prov.apiKey || ''} onChange={e => setVal(`models.providers.${pid}.apiKey`, e.target.value)}
                          placeholder="sk-..." className="w-full px-3.5 py-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50/50 dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all font-mono tracking-wider" />
                        <div className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none">
                          <Key size={14} />
                        </div>
                      </div>
                    </div>
                    <div>
                      <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300 mb-1.5">API 类型</label>
                      <div className="relative">
                        <select value={prov.api || 'openai-completions'} onChange={e => setVal(`models.providers.${pid}.api`, e.target.value)}
                          className="w-full px-3.5 py-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50/50 dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 appearance-none cursor-pointer">
                          <option value="openai-completions">OpenAI Chat Completions API</option>
                          <option value="openai-responses">OpenAI Responses API</option>
                          <option value="openai-codex-responses">OpenAI Codex Responses API</option>
                          <option value="anthropic-messages">Anthropic Messages API</option>
                          <option value="google-generative-ai">Google Generative AI (Gemini) API</option>
                          <option value="github-copilot">GitHub Copilot API</option>
                          <option value="bedrock-converse-stream">AWS Bedrock Converse Stream API</option>
                          <option value="ollama">Ollama 本地模型 API</option>
                        </select>
                        <ChevronDown size={14} className="absolute right-3.5 top-1/2 -translate-y-1/2 text-gray-400 pointer-events-none" />
                      </div>
                    </div>
                    <div>
                      <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300 mb-1.5">备注 (可选)</label>
                      <input value={prov._note || ''} onChange={e => setVal(`models.providers.${pid}._note`, e.target.value)}
                        placeholder="例: 公司账号 / 个人测试" className="w-full px-3.5 py-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50/50 dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all" />
                    </div>
                  </div>
                  <ProviderHealthCheck pid={pid} prov={prov} />

                  <div className="border-t border-gray-100 dark:border-gray-800 pt-4">
                    <label className="block text-xs font-bold text-gray-500 uppercase tracking-wider mb-3">模型列表</label>
                    <div className="space-y-2">
                      {(prov.models || []).map((m: any, idx: number) => {
                        const mObj = typeof m === 'string' ? { id: m, name: m } : m;
                        const updateModel = (key: string, val: any) => {
                          updateConfig((clone: any) => {
                            const models = clone.models.providers[pid].models || [];
                            if (typeof models[idx] === 'string') models[idx] = { id: models[idx], name: models[idx] };
                            models[idx] = { ...models[idx], [key]: val };
                            if (key === 'id') models[idx].name = val;
                            clone.models.providers[pid].models = models;
                          });
                        };
                        return (
                        <div key={idx} className="p-3 rounded-xl bg-[linear-gradient(145deg,rgba(255,255,255,0.76),rgba(239,246,255,0.56))] dark:bg-[linear-gradient(145deg,rgba(12,24,42,0.8),rgba(30,64,175,0.08))] border border-blue-100/70 dark:border-blue-800/20 space-y-3 group hover:border-blue-200 dark:hover:border-blue-800/40 transition-colors backdrop-blur-xl">
                          <div className="flex items-center gap-3">
                            <div className="p-1.5 rounded-xl bg-white/80 dark:bg-slate-800/70 shadow-sm text-blue-500 border border-blue-100/60 dark:border-slate-700/60">
                              <Box size={14} />
                            </div>
                            <input value={mObj.id || ''} onChange={e => updateModel('id', e.target.value)}
                              placeholder="模型 ID" className="flex-1 px-2 py-1 text-sm font-mono font-medium bg-transparent border-b border-transparent focus:border-blue-500 outline-none transition-colors" />
                            <button onClick={() => {
                              updateConfig((clone: any) => {
                                clone.models.providers[pid].models.splice(idx, 1);
                              });
                            }} className="p-1.5 text-gray-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 rounded transition-colors opacity-0 group-hover:opacity-100"><Trash2 size={14} /></button>
                          </div>
                          <div className="grid grid-cols-3 gap-3 pl-9">
                            <div>
                              <label className="text-[10px] text-gray-400 font-medium block mb-1">Context Window</label>
                              <input type="number" value={mObj.contextWindow ?? ''} onChange={e => updateModel('contextWindow', e.target.value ? Number(e.target.value) : undefined)}
                                placeholder="128k" className="w-full px-2 py-1 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-800 font-mono focus:border-blue-500 outline-none" />
                            </div>
                            <div>
                              <label className="text-[10px] text-gray-400 font-medium block mb-1">Max Tokens</label>
                              <input type="number" value={mObj.maxTokens ?? ''} onChange={e => updateModel('maxTokens', e.target.value ? Number(e.target.value) : undefined)}
                                placeholder="8k" className="w-full px-2 py-1 text-xs border border-gray-200 dark:border-gray-700 rounded bg-white dark:bg-gray-800 font-mono focus:border-blue-500 outline-none" />
                            </div>
                            <div>
                              <label className="text-[10px] text-gray-400 font-medium block mb-1">推理模型</label>
                              <button onClick={() => updateModel('reasoning', !mObj.reasoning)}
                                className={`w-full px-2 py-1 text-xs rounded border transition-colors text-left flex items-center gap-1.5 ${mObj.reasoning ? 'bg-blue-50 dark:bg-blue-900/30 border-blue-200 dark:border-blue-800 text-blue-700 dark:text-blue-300' : 'bg-white dark:bg-gray-800 border-gray-200 dark:border-gray-700 text-gray-500'}`}>
                                <div className={`w-2 h-2 rounded-full ${mObj.reasoning ? 'bg-blue-500' : 'bg-gray-300'}`}></div>
                                {mObj.reasoning ? '是' : '否'}
                              </button>
                            </div>
                          </div>
                        </div>
                        );
                      })}
                      <div className="flex gap-2 mt-2 flex-wrap pt-1">
                        <button onClick={() => {
                          updateConfig((clone: any) => {
                          if (!clone.models.providers[pid].models) clone.models.providers[pid].models = [];
                          clone.models.providers[pid].models.push({ id: '', name: '', contextWindow: 128000, maxTokens: 8192 });
                          });
                        }} className="px-3 py-1.5 text-xs font-medium rounded-lg bg-gray-50 dark:bg-gray-800 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 border border-gray-200 dark:border-gray-700 border-dashed hover:border-solid transition-all flex items-center gap-1.5">
                          <Plus size={12} /> 自定义模型
                        </button>
                        {KNOWN_PROVIDERS.filter(kp => prov.baseUrl?.includes(kp.baseUrl.replace('https://', '').split('/')[0])).flatMap(kp =>
                          kp.models.filter(m => !(prov.models || []).find((pm: any) => (typeof pm === 'string' ? pm : pm.id) === m)).slice(0, 4).map(m => (
                            <button key={m} onClick={() => {
                              updateConfig((clone: any) => {
                              if (!clone.models.providers[pid].models) clone.models.providers[pid].models = [];
                              clone.models.providers[pid].models.push({ id: m, name: m, contextWindow: 128000, maxTokens: 8192 });
                              });
                            }} className="px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/40 transition-colors flex items-center gap-1.5">
                              <Plus size={12} /> {m}
                            </button>
                          ))
                        )}
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* === Identity & Messages Tab === */}
      {tab === 'identity' && (
        <div className="space-y-6 animate-in fade-in slide-in-from-top-4 duration-200">
          {/* Login password display + change */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 p-5 space-y-3">
              <div className="flex items-center gap-2">
                <div className="p-1.5 rounded-xl bg-blue-100/80 dark:bg-blue-900/20 text-blue-600 border border-blue-100/70 dark:border-blue-800/30">
                  <Key size={16} />
                </div>
                <h3 className="text-sm font-bold text-gray-900 dark:text-white">管理后台登录密码</h3>
              </div>
              <AdminPasswordField token={adminToken} onCopy={() => { setMsg('密码已复制'); setTimeout(() => setMsg(''), 2000); }} />
              <p className="text-xs text-gray-500 flex items-center gap-1.5">
                <span className="w-1 h-1 rounded-full bg-gray-400"></span>
                此密码在 .env 文件中的 ADMIN_TOKEN 配置
              </p>
            </div>
            <ChangePasswordSection />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="space-y-6">
              <CfgSection title="身份设置" icon={Users} fields={[
                { path: 'ui.assistant.name', label: '助手名称', type: 'text' as const, placeholder: 'OpenClaw' },
                { path: 'ui.assistant.avatar', label: '助手头像', type: 'text' as const, placeholder: 'emoji或URL' },
                { path: 'ui.seamColor', label: '主题色', type: 'text' as const, placeholder: '#7c3aed' },
              ]} getVal={getVal} setVal={setVal} />
              
              <CfgSection title="消息配置" icon={MessageSquare} fields={[
                { path: 'messages.responsePrefix', label: '回复前缀', type: 'text' as const, placeholder: '[OpenClaw]' },
                { path: 'session.maintenance.maxEntries', label: '会话条目上限', type: 'number' as const, placeholder: '2000', integer: true, min: 1 },
                { path: 'messages.ackReactionScope', label: '确认反应范围', type: 'select' as const, options: ['all', 'group-mentions', 'group-all', 'direct', 'off', 'none'] },
              ]} getVal={getVal} setVal={setVal} />
            </div>
            
            <div className="space-y-6">
              <CfgSection
                title="Agent 默认设置"
                icon={Brain}
                description="这里控制所有未单独覆盖的 Agent 默认上下文预算；单个 Agent 可在 Agent 管理页继续覆盖 contextTokens / compaction。"
                defaultExpanded
                fields={[
                  {
                    path: 'agents.defaults.contextTokens',
                    label: '默认上下文 Token 预算',
                    type: 'number' as const,
                    placeholder: '200000',
                    help: 'OpenClaw 会再与模型真实 contextWindow 取更小值；留空表示不在面板里显式覆盖。',
                    integer: true,
                    min: 1,
                  },
                  { path: 'agents.defaults.maxConcurrent', label: '最大并发', type: 'number' as const, placeholder: '4', integer: true, min: 1 },
                  {
                    path: 'agents.defaults.compaction.mode',
                    label: '压缩模式',
                    type: 'select' as const,
                    options: ['default', 'safeguard'],
                    help: 'default 为常规裁剪；safeguard 会更保守地压缩工具结果。',
                  },
                  {
                    path: 'agents.defaults.compaction.maxHistoryShare',
                    label: '历史占比上限',
                    type: 'number' as const,
                    placeholder: '0.5',
                    help: '控制历史消息最多可占上下文预算的比例。',
                    min: 0,
                    max: 1,
                  },
                ]}
                getVal={getVal}
                setVal={setVal}
              />
            </div>
          </div>

          {/* Identity MD files editor */}
          <div className={`${modern ? 'page-modern-panel overflow-hidden flex flex-col h-[600px]' : 'bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 overflow-hidden flex flex-col h-[600px]'}`}>
            <div className="px-5 py-4 flex items-center gap-3 border-b border-gray-100 dark:border-gray-800 bg-gray-50/30 dark:bg-gray-900/30">
              <div className="p-1.5 rounded-lg bg-blue-100 dark:bg-blue-900/30 text-blue-600">
                <FileText size={16} />
              </div>
              <div>
                <h3 className="text-sm font-bold text-gray-900 dark:text-white">身份文档 (Markdown)</h3>
                <p className="text-xs text-gray-500 mt-0.5">编辑核心人格设定与系统提示词</p>
              </div>
            </div>
            <div className="flex-1 grid grid-cols-1 lg:grid-cols-4 overflow-hidden">
              <div className="p-3 border-r border-gray-100 dark:border-gray-800 overflow-y-auto bg-gray-50/30 dark:bg-gray-900/30 space-y-1">
                <h4 className="text-xs font-bold text-gray-400 uppercase tracking-wider px-3 py-2">文件列表</h4>
                {identityDocs.map((doc: any) => (
                  <button key={doc.name} onClick={() => { setSelectedIdentityDoc(doc); setIdentityContent(doc.content || ''); }}
                    className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-left text-xs transition-all duration-200 group ${
                      selectedIdentityDoc?.name === doc.name
                        ? 'bg-white dark:bg-gray-800 text-blue-700 dark:text-blue-300 font-medium shadow-sm ring-1 ring-blue-100 dark:ring-blue-900'
                        : 'text-gray-600 dark:text-gray-400 hover:bg-white dark:hover:bg-gray-800 hover:shadow-sm'
                    }`}>
                    <div className={`shrink-0 ${selectedIdentityDoc?.name === doc.name ? 'text-blue-500' : 'text-gray-400 group-hover:text-gray-500'}`}>
                      <FileText size={14} />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="truncate font-medium">{doc.name}</div>
                      <div className="text-[10px] text-gray-400 truncate opacity-80">{doc.exists === false ? '未创建' : `${(doc.size / 1024).toFixed(1)} KB`}</div>
                    </div>
                  </button>
                ))}
              </div>
              <div className="lg:col-span-3 flex flex-col h-full bg-white dark:bg-gray-800">
                {selectedIdentityDoc ? (
                  <>
                    <div className="px-5 py-3 border-b border-gray-100 dark:border-gray-800 flex items-center justify-between bg-white dark:bg-gray-800 z-10">
                      <div className="flex items-center gap-2">
                        <FileText size={14} className="text-gray-400" />
                        <span className="text-sm font-semibold text-gray-900 dark:text-gray-100">{selectedIdentityDoc.name}</span>
                      </div>
                      <button onClick={async () => {
                        setIdentitySaving(true);
                        try {
                          await api.saveIdentityDoc(selectedIdentityDoc.path, identityContent);
                          setMsg('文档已保存');
                          loadIdentityDocs();
                        } catch (err) { setMsg('保存失败: ' + String(err)); }
                        finally { setIdentitySaving(false); setTimeout(() => setMsg(''), 3000); }
                      }} disabled={identitySaving}
                        className={`${modern ? 'page-modern-accent px-4 py-1.5 text-xs font-medium disabled:opacity-50' : 'flex items-center gap-1.5 px-4 py-1.5 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50 shadow-sm transition-all hover:shadow-md hover:shadow-violet-200 dark:hover:shadow-none'}`}>
                        {identitySaving ? <RefreshCw size={12} className="animate-spin" /> : <Save size={12} />}
                        {identitySaving ? '保存中...' : '保存更改'}
                      </button>
                    </div>
                    <div className="flex-1 relative">
                      <textarea value={identityContent} onChange={e => setIdentityContent(e.target.value)}
                        className="absolute inset-0 w-full h-full p-5 text-sm font-mono leading-relaxed bg-transparent border-none outline-none resize-none text-gray-800 dark:text-gray-200" 
                        spellCheck={false} />
                    </div>
                  </>
                ) : (
                  <div className="flex flex-col items-center justify-center h-full text-gray-400 gap-3">
                    <FileText size={48} className="opacity-10" />
                    <p className="text-sm">选择左侧文档进行编辑</p>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* === General Config Tab === */}
      {tab === 'general' && (
        <div className="space-y-3">
          <CfgSection title="网关配置" icon={Globe} fields={[
            { path: 'gateway.port', label: '端口', type: 'number' as const, placeholder: '18789', integer: true, min: 1, max: 65535 },
            { path: 'gateway.mode', label: '模式', type: 'select' as const, options: ['local', 'remote'] },
            { path: 'gateway.bind', label: '绑定', type: 'select' as const, options: ['auto', 'loopback', 'lan', 'tailnet', 'custom'] },
            { path: 'gateway.customBindHost', label: '自定义绑定地址', type: 'text' as const, placeholder: '0.0.0.0 / 127.0.0.1 / ::1' },
            { path: 'gateway.auth.mode', label: '认证模式', type: 'select' as const, options: ['none', 'token', 'password', 'trusted-proxy'] },
            { path: 'gateway.auth.token', label: '认证Token', type: 'password' as const },
          ]} getVal={getVal} setVal={setVal} />
          <CfgSection title="多智能体协同" icon={Users} fields={[
            { path: 'tools.agentToAgent.enabled', label: '启用 Agent 间委托', type: 'toggle' as const },
            { path: 'session.agentToAgent.maxPingPongTurns', label: '最大来回委托轮次', type: 'number' as const, placeholder: '4', integer: true, min: 1 },
            { path: 'tools.sessions.visibility', label: '会话可见性', type: 'select' as const, options: ['same-agent', 'all-agents'] },
            { path: 'session.dmScope', label: '私聊隔离范围', type: 'select' as const, options: ['main', 'per-peer', 'per-channel-peer', 'per-account-channel-peer'] },
          ]} getVal={getVal} setVal={setVal} />
          <SessionIsolationSection config={config} updateConfig={updateConfig} />
          <BrowserControlSection config={config} updateConfig={updateConfig} />
          <ToolGovernanceSection config={config} updateConfig={updateConfig} />
          <div className={`${modern ? 'page-modern-panel p-5 space-y-2' : 'bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 p-5 space-y-2'}`}>
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-bold text-gray-900 dark:text-white">Agent 间委托白名单</h3>
              <code className="text-[9px] text-gray-400 font-mono bg-gray-50 dark:bg-gray-900 px-1.5 py-0.5 rounded border border-gray-100 dark:border-gray-800">tools.agentToAgent.allow</code>
            </div>
            <input
              value={(() => {
                const raw = getVal('tools.agentToAgent.allow');
                if (Array.isArray(raw)) return raw.join(', ');
                if (typeof raw === 'string') return raw;
                return '';
              })()}
              onChange={e => {
                const list = parseConfigListInput(e.target.value);
                setVal('tools.agentToAgent.allow', list);
              }}
              placeholder="例如: *, main->work, work->main"
              className="w-full px-3.5 py-2.5 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all font-mono"
            />
            <p className="text-[11px] text-gray-500">用逗号分隔规则；保存时会写为数组。</p>
          </div>
          <CfgSection title="Hooks" icon={Webhook} fields={[
            { path: 'hooks.enabled', label: '启用Hooks', type: 'toggle' as const },
            { path: 'hooks.path', label: '基础路径', type: 'text' as const, placeholder: '/hooks' },
            { path: 'hooks.token', label: 'Webhook密钥', type: 'password' as const },
          ]} getVal={getVal} setVal={setVal} />
          <CfgSection title="命令配置" icon={Terminal} fields={[
            { path: 'commands.native', label: '原生命令', type: 'select' as const, options: ['auto', 'on', 'off'] },
            { path: 'commands.nativeSkills', label: '原生技能', type: 'select' as const, options: ['auto', 'on', 'off'] },
            { path: 'commands.restart', label: '允许重启', type: 'toggle' as const },
          ]} getVal={getVal} setVal={setVal} />
          <CfgSection title="认证密钥" icon={Key} fields={[
            { path: 'env.vars.ANTHROPIC_API_KEY', label: 'Anthropic API Key', type: 'password' as const },
            { path: 'env.vars.OPENAI_API_KEY', label: 'OpenAI API Key', type: 'password' as const },
            { path: 'env.vars.GOOGLE_API_KEY', label: 'Google API Key', type: 'password' as const },
          ]} getVal={getVal} setVal={setVal} />
          <SudoPasswordSection />
          <details className={`${modern ? 'page-modern-panel overflow-hidden' : 'card'}`}>
            <summary className="px-4 py-3 text-xs font-medium text-gray-500 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800/50">高级 JSON 只读预览</summary>
            <div className="px-4 pt-2 text-[11px] text-gray-400">以下内容为当前编辑态配置快照（只读），保存前会先弹出差异预览。</div>
            <pre className="px-4 pb-4 text-[11px] text-gray-600 dark:text-gray-400 overflow-x-auto max-h-96 overflow-y-auto font-mono">{JSON.stringify(config, null, 2)}</pre>
          </details>
        </div>
      )}

      {/* === Version Management Tab === */}
      {tab === 'version' && (
        <div className="space-y-6 animate-in fade-in slide-in-from-top-4 duration-200">
          <div className={`${modern ? 'page-modern-panel p-6 space-y-6' : 'bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 p-6 space-y-6'}`}>
            <h3 className="text-sm font-bold text-gray-900 dark:text-white flex items-center gap-2">
              <Package size={16} className="text-blue-500" /> OpenClaw 版本
            </h3>
            
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div className={`${modern ? 'rounded-[22px] p-4 border border-blue-100/70 dark:border-blue-800/20 bg-[linear-gradient(145deg,rgba(255,255,255,0.78),rgba(239,246,255,0.58))] dark:bg-[linear-gradient(145deg,rgba(12,24,42,0.82),rgba(30,64,175,0.1))] flex items-center gap-4 backdrop-blur-xl' : 'bg-gray-50 dark:bg-gray-900/50 rounded-xl p-4 border border-gray-100 dark:border-gray-800 flex items-center gap-4'}`}>
                <div className="w-10 h-10 rounded-xl bg-blue-100/80 dark:bg-blue-900/20 border border-blue-100/70 dark:border-blue-800/30 flex items-center justify-center shrink-0">
                  <Package size={20} className="text-blue-600 dark:text-blue-300" />
                </div>
                <div>
                  <p className="text-[10px] font-semibold text-gray-500 uppercase tracking-wider">当前版本</p>
                  <p className="text-base font-bold text-gray-900 dark:text-white font-mono mt-0.5">{versionInfo.currentVersion || '加载中...'}</p>
                </div>
              </div>
              
              <div className={`${modern ? 'rounded-[22px] p-4 border border-blue-100/70 dark:border-blue-800/20 bg-[linear-gradient(145deg,rgba(255,255,255,0.78),rgba(239,246,255,0.58))] dark:bg-[linear-gradient(145deg,rgba(12,24,42,0.82),rgba(30,64,175,0.1))] flex items-center gap-4 backdrop-blur-xl' : 'bg-gray-50 dark:bg-gray-900/50 rounded-xl p-4 border border-gray-100 dark:border-gray-800 flex items-center gap-4'}`}>
                <div className={`w-10 h-10 rounded-lg flex items-center justify-center shrink-0 ${versionInfo.updateAvailable ? 'bg-amber-100 dark:bg-amber-900/30 text-amber-600 dark:text-amber-400' : 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-600 dark:text-emerald-400'}`}>
                  {versionInfo.updateAvailable ? <AlertTriangle size={20} /> : <CheckCircle size={20} />}
                </div>
                <div>
                  <p className="text-[10px] font-semibold text-gray-500 uppercase tracking-wider">最新版本</p>
                  <p className="text-base font-bold text-gray-900 dark:text-white font-mono mt-0.5">{versionInfo.latestVersion || '-'}</p>
                </div>
              </div>
              
              <div className={`${modern ? 'rounded-[22px] p-4 border border-blue-100/70 dark:border-blue-800/20 bg-[linear-gradient(145deg,rgba(255,255,255,0.78),rgba(239,246,255,0.58))] dark:bg-[linear-gradient(145deg,rgba(12,24,42,0.82),rgba(30,64,175,0.1))] flex items-center gap-4 backdrop-blur-xl' : 'bg-gray-50 dark:bg-gray-900/50 rounded-xl p-4 border border-gray-100 dark:border-gray-800 flex items-center gap-4'}`}>
                <div className="w-10 h-10 rounded-lg bg-gray-100 dark:bg-gray-800 flex items-center justify-center shrink-0">
                  <RefreshCw size={20} className="text-gray-400" />
                </div>
                <div>
                  <p className="text-[10px] font-semibold text-gray-500 uppercase tracking-wider">上次检查</p>
                  <p className="text-xs text-gray-700 dark:text-gray-300 mt-1 font-medium">{versionInfo.lastCheckedAt ? new Date(versionInfo.lastCheckedAt).toLocaleString('zh-CN') : versionInfo.checkedAt ? new Date(versionInfo.checkedAt).toLocaleString('zh-CN') : '-'}</p>
                </div>
              </div>
            </div>

            <UpdateSection versionInfo={versionInfo} updating={updating} setUpdating={setUpdating} updateStatus={updateStatus} setUpdateStatus={setUpdateStatus} updateLog={updateLog} setUpdateLog={setUpdateLog} checking={checking} setChecking={setChecking} setVersionInfo={setVersionInfo} setMsg={setMsg} loadVersion={loadVersion} />
          </div>

          {/* ClawPanel 面板自检更新 */}
          <PanelUpdateSection />

          <div className={`${modern ? 'page-modern-panel p-6 space-y-4' : 'bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 p-6 space-y-4'}`}>
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-bold text-gray-900 dark:text-white flex items-center gap-2">
                <Archive size={16} className="text-blue-500" /> 备份与恢复
              </h3>
              <button onClick={handleBackup} disabled={backingUp}
                className={`${modern ? 'page-modern-accent px-4 py-2 text-xs font-medium disabled:opacity-50' : 'flex items-center gap-2 px-4 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50 shadow-sm shadow-violet-200 dark:shadow-none transition-all hover:shadow-md hover:shadow-violet-200 dark:hover:shadow-none'}`}>
                {backingUp ? <RefreshCw size={14} className="animate-spin" /> : <Archive size={14} />}
                {backingUp ? '备份中...' : '立即备份'}
              </button>
            </div>
            <p className="text-xs text-gray-500">备份包含 openclaw.json 配置和定时任务。恢复前会自动备份当前配置。</p>
            
            {backups.length === 0 ? (
              <div className="text-center py-8 text-gray-400 text-xs border-2 border-dashed border-gray-100 dark:border-gray-800 rounded-xl">暂无备份记录</div>
            ) : (
              <div className="space-y-2 max-h-64 overflow-y-auto pr-1">
                {backups.map((b: any) => (
                  <div key={b.name} className={`${modern ? 'flex items-center justify-between p-3 rounded-xl bg-[linear-gradient(145deg,rgba(255,255,255,0.74),rgba(239,246,255,0.56))] dark:bg-[linear-gradient(145deg,rgba(12,24,42,0.8),rgba(30,64,175,0.08))] border border-blue-100/70 dark:border-blue-800/20 hover:border-blue-200 dark:hover:border-blue-800/40 transition-colors group backdrop-blur-xl' : 'flex items-center justify-between p-3 rounded-lg bg-gray-50 dark:bg-gray-900/50 border border-gray-100 dark:border-gray-800 hover:border-violet-200 dark:hover:border-violet-800 transition-colors group'}`}>
                    <div className="flex items-center gap-3">
                        <div className="p-2 rounded-xl bg-white/80 dark:bg-slate-800/70 shadow-sm text-gray-400 group-hover:text-blue-500 transition-colors border border-blue-100/60 dark:border-slate-700/60">
                          <Archive size={16} />
                        </div>
                      <div>
                        <p className="font-mono text-xs font-medium text-gray-700 dark:text-gray-300">{b.name}</p>
                        <p className="text-[10px] text-gray-400 mt-0.5">{new Date(b.time).toLocaleString('zh-CN')} · <span className="font-mono">{(b.size / 1024).toFixed(1)} KB</span></p>
                      </div>
                    </div>
                    <button onClick={() => handleRestore(b.name)}
                      className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/40 opacity-0 group-hover:opacity-100 transition-all">
                      <RotateCcw size={12} /> 恢复
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {/* === Environment Detection Tab === */}
      {tab === 'env' && (
        <div className="space-y-6 animate-in fade-in slide-in-from-top-4 duration-200">
          {envLoading ? (
            <div className="flex flex-col items-center justify-center py-16 text-gray-400 gap-3">
              <RefreshCw size={32} className="animate-spin text-blue-500/50" />
              <p className="text-sm">检测运行环境中...</p>
            </div>
          ) : (
            <>
              <div className={`${modern ? 'page-modern-panel p-6 space-y-4' : 'bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 p-6 space-y-4'}`}>
                <h3 className="text-sm font-bold text-gray-900 dark:text-white flex items-center gap-2">
                  <Monitor size={16} className="text-blue-500" /> 操作系统
                </h3>
                <div className="grid grid-cols-2 md:grid-cols-4 gap-y-4 gap-x-6">
                  {[
                    ['平台', envInfo.os?.platform, 'bg-gray-100 dark:bg-gray-800'], 
                    ['架构', envInfo.os?.arch, 'bg-gray-100 dark:bg-gray-800'],
                    ['发行版', envInfo.os?.distro, 'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300'], 
                    ['内核', envInfo.os?.release, 'bg-gray-100 dark:bg-gray-800'],
                    ['主机名', envInfo.os?.hostname, 'bg-gray-100 dark:bg-gray-800'], 
                    ['用户', envInfo.os?.userInfo, 'bg-gray-100 dark:bg-gray-800'],
                    ['CPU 核心', envInfo.os?.cpus ? `${envInfo.os.cpus} 核` : '-', 'bg-gray-100 dark:bg-gray-800'],
                    ['CPU 型号', envInfo.os?.cpuModel, 'bg-gray-100 dark:bg-gray-800 col-span-2 md:col-span-1'],
                    ['总内存', envInfo.os?.totalMemMB ? `${(envInfo.os.totalMemMB / 1024).toFixed(1)} GB` : '-', 'bg-gray-100 dark:bg-gray-800'],
                    ['可用内存', envInfo.os?.freeMemMB ? `${(envInfo.os.freeMemMB / 1024).toFixed(1)} GB` : '-', 'bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600'],
                    ['系统运行', envInfo.os?.uptime ? formatEnvUptime(envInfo.os.uptime) : '-', 'bg-blue-50 dark:bg-blue-900/20 text-blue-600'],
                    ['负载均值', envInfo.os?.loadAvg, 'bg-gray-100 dark:bg-gray-800'],
                  ].map(([label, value, bg]) => (
                    <div key={label as string} className={value === envInfo.os?.cpuModel ? "col-span-2 md:col-span-1" : ""}>
                      <p className="text-[10px] font-semibold text-gray-500 uppercase tracking-wider mb-1">{label}</p>
                      <p className={`text-xs font-medium truncate px-2 py-1 rounded-md inline-block max-w-full ${bg || 'bg-gray-50 dark:bg-gray-900'}`} title={String(value || '')}>{(value as string) || '-'}</p>
                    </div>
                  ))}
                </div>
              </div>
              
              <SoftwareEnvironment envInfo={envInfo} onRefresh={loadEnv} />

              {/* System Diagnose Report */}
              <div className={`${modern ? 'page-modern-panel p-6 space-y-4' : 'bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 p-6 space-y-4'}`}>
                <div className="flex items-center justify-between">
                  <h3 className="text-sm font-bold text-gray-900 dark:text-white flex items-center gap-2">
                    <FileText size={16} className="text-blue-500" /> 系统诊断报告
                  </h3>
                  <div className="flex items-center gap-2">
                    {diagReport && (
                      <button onClick={() => { navigator.clipboard.writeText(diagReport); setMsg('已复制到剪贴板'); setTimeout(() => setMsg(''), 2000); }}
                        className={`${modern ? 'page-modern-action px-3 py-1.5 text-xs font-medium' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors'}`}>
                        <Archive size={12} /> 复制报告
                      </button>
                    )}
                    <button onClick={async () => {
                      setDiagLoading(true);
                      try {
                        const r = await api.systemDiagnose();
                        if (r.ok) setDiagReport(r.report || '');
                        else setMsg(r.error || '诊断失败');
                      } catch (err) { setMsg('诊断失败: ' + String(err)); }
                      finally { setDiagLoading(false); }
                    }} disabled={diagLoading}
                      className={`${modern ? 'page-modern-accent px-3 py-1.5 text-xs font-medium disabled:opacity-50' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50 transition-all shadow-sm'}`}>
                      {diagLoading ? <RefreshCw size={12} className="animate-spin" /> : <Terminal size={12} />}
                      {diagLoading ? '生成中...' : '生成诊断报告'}
                    </button>
                  </div>
                </div>
                {diagReport && (
                  <pre className="bg-gray-900 dark:bg-black text-green-400 text-[11px] font-mono p-4 rounded-lg overflow-auto max-h-96 whitespace-pre-wrap leading-relaxed border border-gray-700">
                    {diagReport}
                  </pre>
                )}
              </div>
            </>
          )}
        </div>
      )}

      {/* === Config Health Check Tab === */}
      {tab === 'health' && (
        <div className="space-y-6 animate-in fade-in slide-in-from-top-4 duration-200">
          {configCheckLoading ? (
            <div className="flex flex-col items-center justify-center py-16 text-gray-400 gap-3">
              <RefreshCw size={32} className="animate-spin text-blue-500/50" />
              <p className="text-sm">正在扫描配置文件...</p>
            </div>
          ) : (
            <>
              {/* Summary */}
              <div className={`${modern ? 'page-modern-panel p-5' : 'bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 p-5'}`}>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-4">
                    <div className={`p-3 rounded-xl ${configProblems === 0 ? 'bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600' : 'bg-amber-50 dark:bg-amber-900/20 text-amber-600'}`}>
                      {configProblems === 0 ? <CheckCircle size={24} /> : <AlertTriangle size={24} />}
                    </div>
                    <div>
                      <h3 className="text-sm font-bold text-gray-900 dark:text-white">
                        {configProblems === 0 ? '配置正常' : `发现 ${configProblems} 个配置问题`}
                      </h3>
                      <p className="text-xs text-gray-500 mt-0.5">已检查 {configChecked} 项配置</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    {configProblems > 0 && configIssues.some((i: any) => i.fixable) && (
                      <button onClick={handleFixAll} disabled={fixing}
                        className={`${modern ? 'page-modern-accent px-4 py-2 text-xs font-medium disabled:opacity-50' : 'flex items-center gap-1.5 px-4 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50 transition-all shadow-sm'}`}>
                        {fixing ? <RefreshCw size={12} className="animate-spin" /> : <Command size={12} />}
                        {fixing ? '修复中...' : '一键修复全部'}
                      </button>
                    )}
                      <button onClick={loadConfigCheck} disabled={configCheckLoading}
                       className={`${modern ? 'page-modern-control px-3 py-2 text-xs font-medium' : 'flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-700 border border-gray-200 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300 transition-colors'}`}>
                      <RefreshCw size={12} className={configCheckLoading ? 'animate-spin' : ''} />
                      重新检测
                    </button>
                  </div>
                </div>
              </div>

              {/* Issue list */}
              {configIssues.length > 0 ? (
                <div className="space-y-3">
                  {configIssues.map((issue: any) => (
                    <div key={issue.id} className={`${modern ? 'page-modern-panel overflow-hidden transition-all' : 'bg-white dark:bg-gray-800 rounded-xl shadow-sm border overflow-hidden transition-all'} ${
                      issue.severity === 'error' ? 'border-red-200 dark:border-red-800/30' :
                      issue.severity === 'warning' ? 'border-amber-200 dark:border-amber-800/30' :
                      'border-gray-100 dark:border-gray-700/50'
                    }`}>
                      <div className="p-4 flex items-start gap-3">
                        <div className={`p-1.5 rounded-lg shrink-0 mt-0.5 ${
                          issue.severity === 'error' ? 'bg-red-50 dark:bg-red-900/20 text-red-500' :
                          issue.severity === 'warning' ? 'bg-amber-50 dark:bg-amber-900/20 text-amber-500' :
                          'bg-blue-50 dark:bg-blue-900/20 text-blue-500'
                        }`}>
                          <AlertTriangle size={14} />
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2 mb-1">
                            <span className={`text-[10px] font-bold uppercase px-1.5 py-0.5 rounded ${
                              issue.severity === 'error' ? 'bg-red-100 dark:bg-red-900/30 text-red-600' :
                              issue.severity === 'warning' ? 'bg-amber-100 dark:bg-amber-900/30 text-amber-600' :
                              'bg-blue-100 dark:bg-blue-900/30 text-blue-600'
                            }`}>{issue.severity === 'error' ? '错误' : issue.severity === 'warning' ? '警告' : '信息'}</span>
                            <span className="text-[10px] font-medium text-gray-400 uppercase">{issue.component}</span>
                          </div>
                          <h4 className="text-sm font-semibold text-gray-900 dark:text-white">{issue.title}</h4>
                          <p className="text-xs text-gray-500 mt-1">{issue.description}</p>
                          {(issue.currentValue || issue.expectedValue) && (
                            <div className="flex items-center gap-3 mt-2 text-[11px]">
                              {issue.currentValue && <span className="text-gray-500">当前: <code className="bg-red-50 dark:bg-red-900/20 text-red-600 px-1 rounded font-mono">{issue.currentValue}</code></span>}
                              {issue.expectedValue && <span className="text-gray-500">期望: <code className="bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600 px-1 rounded font-mono">{issue.expectedValue}</code></span>}
                            </div>
                          )}
                          {issue.filePath && (
                            <p className="text-[10px] text-gray-400 mt-1.5 font-mono truncate" title={issue.filePath}>{issue.filePath}</p>
                          )}
                        </div>
                        {issue.fixable && (
                          <button onClick={() => handleFixSingle(issue.id)} disabled={fixing}
                            className={`${modern ? 'page-modern-action shrink-0 px-3 py-1.5 text-xs font-medium disabled:opacity-50' : 'shrink-0 flex items-center gap-1 px-3 py-1.5 text-xs font-medium rounded-lg bg-violet-50 dark:bg-violet-900/30 text-violet-600 dark:text-violet-400 hover:bg-violet-100 dark:hover:bg-violet-900/50 disabled:opacity-50 transition-colors'}`}>
                            <Command size={12} />修复
                          </button>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="flex flex-col items-center justify-center py-12 text-gray-400 gap-2">
                  <CheckCircle size={40} className="text-emerald-400" />
                  <p className="text-sm font-medium text-emerald-600">所有配置项检查通过</p>
                  <p className="text-xs text-gray-400">OpenClaw 和 NapCat 配置均正常</p>
                </div>
              )}
            </>
          )}
        </div>
      )}

      {showDiffPreview && (
        <div className="fixed inset-0 z-50 bg-black/40 flex items-center justify-center p-4">
          <div className={`${modern ? 'w-full max-w-4xl max-h-[88vh] overflow-hidden rounded-[28px] bg-[linear-gradient(145deg,rgba(255,255,255,0.92),rgba(239,246,255,0.72))] dark:bg-[linear-gradient(145deg,rgba(12,24,42,0.92),rgba(30,64,175,0.14))] border border-blue-100/70 dark:border-blue-800/20 shadow-xl backdrop-blur-xl flex flex-col' : 'w-full max-w-4xl max-h-[88vh] overflow-hidden rounded-xl bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700 shadow-xl flex flex-col'}`}>
            <div className="px-5 py-4 border-b border-gray-100 dark:border-gray-700 flex items-center justify-between">
              <h3 className="text-sm font-bold text-gray-900 dark:text-white">保存前差异预览</h3>
              <button onClick={() => setShowDiffPreview(false)} className={`${modern ? 'page-modern-action px-2.5 py-1.5 text-xs' : 'px-2 py-1 text-xs rounded bg-gray-100 dark:bg-gray-700'}`}>关闭</button>
            </div>
            <div className="p-4 overflow-y-auto space-y-2">
              <div className="text-xs text-gray-500">共检测到 {diffItems.length} 项变更，确认后将写入 <code className="font-mono">openclaw.json</code>。</div>
              {diffItems.slice(0, 200).map((item, idx) => (
                <div key={idx} className="text-xs border border-gray-100 dark:border-gray-700 rounded-lg p-2">
                  <div className="font-mono text-blue-600 dark:text-blue-300">{item.path}</div>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-2 mt-1">
                    <div className="rounded bg-red-50 dark:bg-red-900/20 p-2">
                      <div className="text-[10px] text-red-500 mb-1">Before</div>
                      <div className="font-mono text-[11px] break-all text-red-700 dark:text-red-300">{item.before}</div>
                    </div>
                    <div className="rounded bg-emerald-50 dark:bg-emerald-900/20 p-2">
                      <div className="text-[10px] text-emerald-500 mb-1">After</div>
                      <div className="font-mono text-[11px] break-all text-emerald-700 dark:text-emerald-300">{item.after}</div>
                    </div>
                  </div>
                </div>
              ))}
              {diffItems.length > 200 && (
                <div className="text-xs text-gray-400">仅展示前 200 项差异，剩余 {diffItems.length - 200} 项未展开。</div>
              )}
            </div>
            <div className="px-5 py-4 border-t border-gray-100 dark:border-gray-700 flex items-center justify-end gap-2">
              <button onClick={() => setShowDiffPreview(false)} className={`${modern ? 'page-modern-action px-4 py-2 text-xs' : 'px-4 py-2 text-xs rounded bg-gray-100 dark:bg-gray-700'}`}>取消</button>
              <button onClick={doSave} disabled={saving} className={`${modern ? 'page-modern-accent px-4 py-2 text-xs disabled:opacity-50' : 'px-4 py-2 text-xs rounded bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50'}`}>
                {saving ? '保存中...' : '确认保存'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Docs tab removed — merged into Identity & 文档 tab */}
    </div>
  );
}

function AdminPasswordField({ token, onCopy }: { token: string; onCopy: () => void }) {
  const [visible, setVisible] = useState(false);
  return (
    <div className="flex items-center gap-2">
      <div className="relative flex-1">
        <input type={visible ? 'text' : 'password'} readOnly value={token}
          className="w-full pl-3 pr-10 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900 font-mono text-gray-600 dark:text-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all" />
        <button onClick={() => setVisible(!visible)} className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 transition-colors">
          {visible ? <EyeOff size={14} /> : <Eye size={14} />}
        </button>
      </div>
      <button onClick={() => { navigator.clipboard.writeText(token); onCopy(); }}
        className="px-3 py-2 text-xs font-medium rounded-lg bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/50 transition-colors border border-blue-100 dark:border-blue-800/30">
        复制
      </button>
    </div>
  );
}

function formatEnvUptime(s: number) {
  if (s < 60) return `${Math.floor(s)}秒`;
  if (s < 3600) return `${Math.floor(s / 60)}分${Math.floor(s % 60)}秒`;
  if (s < 86400) return `${Math.floor(s / 3600)}时${Math.floor((s % 3600) / 60)}分`;
  return `${Math.floor(s / 86400)}天${Math.floor((s % 86400) / 3600)}时${Math.floor(((s % 86400) % 3600) / 60)}分`;
}

function SudoPasswordSection() {
  const [pwd, setPwd] = useState('');
  const [configured, setConfigured] = useState(false);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState('');
  const [platform, setPlatform] = useState('');

  useEffect(() => {
    api.getSudoPassword().then(r => { if (r.ok) setConfigured(r.configured); });
    api.getSoftwareList().then(r => { if (r.ok && r.platform) setPlatform(r.platform); });
  }, []);

  // Windows doesn't use sudo - hide this section entirely
  if (platform === 'windows') return null;

  const handleSave = async () => {
    setSaving(true);
    try {
      const r = await api.setSudoPassword(pwd);
      if (r.ok) { setMsg('已保存'); setConfigured(true); setPwd(''); }
      else setMsg('保存失败');
    } catch { setMsg('保存失败'); }
    finally { setSaving(false); setTimeout(() => setMsg(''), 3000); }
  };

  return (
    <div className="page-modern-panel overflow-hidden">
      <div className="px-5 py-4 flex items-center justify-between border-b border-gray-100 dark:border-gray-800 bg-gray-50/30 dark:bg-gray-900/30">
        <div className="flex items-center gap-3">
          <div className="p-1.5 rounded-xl bg-amber-100/80 dark:bg-amber-900/20 text-amber-600 border border-amber-100/70 dark:border-amber-800/30">
            <Shield size={16} />
          </div>
          <div>
            <h3 className="text-sm font-bold text-gray-900 dark:text-white">Sudo 密码</h3>
            <p className="text-[10px] text-gray-500 mt-0.5">用于系统更新等需要 sudo 权限的操作</p>
          </div>
        </div>
        {configured ? (
          <span className="text-[10px] px-2 py-0.5 rounded-full bg-emerald-50 dark:bg-emerald-900/30 text-emerald-600 dark:text-emerald-400 font-medium border border-emerald-100 dark:border-emerald-900/50">已配置</span>
        ) : (
          <span className="text-[10px] px-2 py-0.5 rounded-full bg-gray-100 dark:bg-gray-800 text-gray-500 font-medium border border-gray-200 dark:border-gray-700">未配置</span>
        )}
      </div>
      <div className="p-5 space-y-3">
        <div className="flex items-center gap-3">
          <div className="relative flex-1">
            <input type="password" value={pwd} onChange={e => setPwd(e.target.value)} 
              placeholder={configured ? '••••••（已配置，留空不修改）' : '输入 sudo 密码'}
              className="w-full pl-4 pr-4 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-amber-500/20 focus:border-amber-500 transition-all placeholder:text-gray-400" />
          </div>
          <button onClick={handleSave} disabled={saving || !pwd}
            className="page-modern-warn px-4 py-2 text-xs font-medium disabled:opacity-50">
            {saving ? '保存中...' : '保存'}
          </button>
        </div>
        {msg && (
          <div className="flex items-center gap-1.5 text-xs text-emerald-600 bg-emerald-50 dark:bg-emerald-900/20 px-3 py-2 rounded-lg border border-emerald-100 dark:border-emerald-900/30">
            <CheckCircle size={12} /> {msg}
          </div>
        )}
      </div>
    </div>
  );
}

function ChangePasswordSection() {
  const { t } = useI18n();
  const [oldPwd, setOldPwd] = useState('');
  const [newPwd, setNewPwd] = useState('');
  const [confirmPwd, setConfirmPwd] = useState('');
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState('');
  const [msgOk, setMsgOk] = useState(true);

  const handleChange = async () => {
    if (!oldPwd || !newPwd) return;
    if (newPwd !== confirmPwd) { setMsg(t.sysConfig?.passwordMismatch || '两次输入的密码不一致'); setMsgOk(false); setTimeout(() => setMsg(''), 3000); return; }
    if (newPwd.length < 4) { setMsg(t.sysConfig?.passwordTooShort || '密码至少4位'); setMsgOk(false); setTimeout(() => setMsg(''), 3000); return; }
    setSaving(true);
    try {
      const r = await api.changePassword(oldPwd, newPwd);
      if (r.ok) {
        setMsg(t.sysConfig?.passwordChanged || '密码修改成功，即将退出登录...');
        setMsgOk(true);
        setTimeout(() => { localStorage.removeItem('admin-token'); window.location.reload(); }, 2000);
      } else {
        setMsg(r.error === 'Wrong current password' ? (t.sysConfig?.wrongPassword || '当前密码错误') : (r.error || '修改失败'));
        setMsgOk(false);
      }
    } catch { setMsg('修改失败'); setMsgOk(false); }
    finally { setSaving(false); setTimeout(() => setMsg(''), 4000); }
  };

  return (
    <div className="page-modern-panel overflow-hidden">
      <div className="px-5 py-4 flex items-center gap-3 border-b border-gray-100 dark:border-gray-800 bg-gray-50/30 dark:bg-gray-900/30">
        <div className="p-1.5 rounded-xl bg-blue-100/80 dark:bg-blue-900/20 text-blue-600 border border-blue-100/70 dark:border-blue-800/30">
          <Key size={16} />
        </div>
        <div>
          <h3 className="text-sm font-bold text-gray-900 dark:text-white">{t.sysConfig?.changePassword || '修改管理密码'}</h3>
          <p className="text-[10px] text-gray-500 mt-0.5">{t.sysConfig?.changePasswordDesc || '修改 ClawPanel 管理后台登录密码，修改后需重新登录'}</p>
        </div>
      </div>
      <div className="p-5 space-y-3">
        <input type="password" value={oldPwd} onChange={e => setOldPwd(e.target.value)}
          placeholder={t.sysConfig?.currentPassword || '当前密码'}
          className="w-full px-4 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all placeholder:text-gray-400" />
        <input type="password" value={newPwd} onChange={e => setNewPwd(e.target.value)}
          placeholder={t.sysConfig?.newPassword || '新密码'}
          className="w-full px-4 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all placeholder:text-gray-400" />
        <input type="password" value={confirmPwd} onChange={e => setConfirmPwd(e.target.value)}
          placeholder={t.sysConfig?.confirmPassword || '确认新密码'}
          className="w-full px-4 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all placeholder:text-gray-400" />
        <button onClick={handleChange} disabled={saving || !oldPwd || !newPwd || !confirmPwd}
          className="w-full px-4 py-2.5 text-xs font-medium page-modern-accent disabled:opacity-50">
          {saving ? '修改中...' : (t.sysConfig?.changePasswordBtn || '修改密码')}
        </button>
        {msg && (
          <div className={`flex items-center gap-1.5 text-xs px-3 py-2 rounded-lg border ${msgOk ? 'text-emerald-600 bg-emerald-50 dark:bg-emerald-900/20 border-emerald-100 dark:border-emerald-900/30' : 'text-red-600 bg-red-50 dark:bg-red-900/20 border-red-100 dark:border-red-900/30'}`}>
            {msgOk ? <CheckCircle size={12} /> : <AlertTriangle size={12} />} {msg}
          </div>
        )}
      </div>
    </div>
  );
}

function PanelUpdateSection() {
  const [panelVersion, setPanelVersion] = useState('');
  const [checkingPanel, setCheckingPanel] = useState(false);
  const [panelUpdateInfo, setPanelUpdateInfo] = useState<any>(null);
  const [navigating, setNavigating] = useState(false);
  const [updateHistory, setUpdateHistory] = useState<any[]>([]);
  const [showHistory, setShowHistory] = useState(false);

  useEffect(() => {
    api.getPanelVersion().then(r => { if (r.ok) setPanelVersion(r.version); }).catch(() => {});
  }, []);

  const checkPanelUpdate = async () => {
    setCheckingPanel(true);
    try {
      const r = await api.checkPanelUpdate();
      if (r.ok) setPanelUpdateInfo(r);
      else setPanelUpdateInfo({ error: r.error || '检查失败' });
    } catch { setPanelUpdateInfo({ error: '网络错误' }); }
    finally { setCheckingPanel(false); }
  };

  const goToUpdater = async () => {
    setNavigating(true);
    try {
      const r = await api.generateUpdateToken();
      if (!r.ok) { alert('生成更新令牌失败: ' + (r.error || '')); setNavigating(false); return; }
      window.open(r.updaterURL, '_blank');
    } catch (e) { alert('网络错误: ' + e); }
    finally { setNavigating(false); }
  };

  const loadHistory = async () => {
    try {
      const r = await api.getUpdateHistory();
      if (r.ok) setUpdateHistory(r.history || []);
    } catch {}
  };

  return (
    <div className="page-modern-panel p-6 space-y-5">
      <h3 className="text-sm font-bold text-gray-900 dark:text-white flex items-center gap-2">
        <Box size={16} className="text-blue-500" /> ClawPanel 版本更新
        <span className="text-[10px] font-mono text-gray-400 bg-gray-100 dark:bg-gray-900 px-2 py-0.5 rounded ml-1">{panelVersion || '...'}</span>
        <span className="text-[10px] text-gray-400 ml-auto">🛡️ 独立更新工具</span>
      </h3>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="rounded-[22px] p-4 border border-blue-100/70 dark:border-blue-800/20 bg-[linear-gradient(145deg,rgba(255,255,255,0.78),rgba(239,246,255,0.58))] dark:bg-[linear-gradient(145deg,rgba(12,24,42,0.82),rgba(30,64,175,0.1))] flex items-center gap-4 backdrop-blur-xl">
          <div className="w-10 h-10 rounded-xl bg-blue-100/80 dark:bg-blue-900/20 border border-blue-100/70 dark:border-blue-800/30 flex items-center justify-center shrink-0">
            <Box size={20} className="text-blue-600 dark:text-blue-300" />
          </div>
          <div>
            <p className="text-[10px] font-semibold text-gray-500 uppercase tracking-wider">当前版本</p>
            <p className="text-base font-bold text-gray-900 dark:text-white font-mono mt-0.5">{panelVersion || '加载中...'}</p>
          </div>
        </div>
        <div className="rounded-[22px] p-4 border border-blue-100/70 dark:border-blue-800/20 bg-[linear-gradient(145deg,rgba(255,255,255,0.78),rgba(239,246,255,0.58))] dark:bg-[linear-gradient(145deg,rgba(12,24,42,0.82),rgba(30,64,175,0.1))] flex items-center gap-4 backdrop-blur-xl">
          <div className={`w-10 h-10 rounded-lg flex items-center justify-center shrink-0 ${panelUpdateInfo?.hasUpdate ? 'bg-amber-100 dark:bg-amber-900/30 text-amber-600 dark:text-amber-400' : panelUpdateInfo && !panelUpdateInfo.error ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-600 dark:text-emerald-400' : 'bg-gray-100 dark:bg-gray-800 text-gray-400'}`}>
            {panelUpdateInfo?.hasUpdate ? <AlertTriangle size={20} /> : panelUpdateInfo && !panelUpdateInfo.error ? <CheckCircle size={20} /> : <RefreshCw size={20} />}
          </div>
          <div>
            <p className="text-[10px] font-semibold text-gray-500 uppercase tracking-wider">最新版本</p>
            <p className="text-base font-bold text-gray-900 dark:text-white font-mono mt-0.5">{panelUpdateInfo?.latestVersion || '-'}</p>
          </div>
        </div>
        <div className="rounded-[22px] p-4 border border-blue-100/70 dark:border-blue-800/20 bg-[linear-gradient(145deg,rgba(255,255,255,0.78),rgba(239,246,255,0.58))] dark:bg-[linear-gradient(145deg,rgba(12,24,42,0.82),rgba(30,64,175,0.1))] flex items-center gap-4 backdrop-blur-xl">
          <div className="w-10 h-10 rounded-lg bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center shrink-0">
            <Shield size={20} className="text-blue-600 dark:text-blue-400" />
          </div>
          <div>
            <p className="text-[10px] font-semibold text-gray-500 uppercase tracking-wider">更新方式</p>
            <p className="text-xs text-gray-700 dark:text-gray-300 mt-1 font-medium">独立进程 · 自动回滚</p>
          </div>
        </div>
      </div>

      <div className="flex items-center gap-3 flex-wrap">
        <button onClick={checkPanelUpdate} disabled={checkingPanel}
          className="flex items-center gap-2 px-4 py-2 text-xs font-medium rounded-lg bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/50 disabled:opacity-50 transition-colors">
          <RefreshCw size={14} className={checkingPanel ? 'animate-spin' : ''} />
          {checkingPanel ? '检测中...' : '检测新版本'}
        </button>

        {panelUpdateInfo?.hasUpdate && (
          <button onClick={goToUpdater} disabled={navigating}
          className="flex items-center gap-2 px-4 py-2 text-xs font-medium page-modern-accent disabled:opacity-50">
            {navigating ? <RefreshCw size={14} className="animate-spin" /> : <Package size={14} />}
            {navigating ? '正在跳转...' : '前往更新'}
          </button>
        )}

        {panelUpdateInfo && !panelUpdateInfo.hasUpdate && !panelUpdateInfo.error && (
          <span className="flex items-center gap-1.5 px-3 py-1.5 text-xs text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-900/20 rounded-lg border border-emerald-100 dark:border-emerald-900/30">
            <CheckCircle size={12} /> 当前为最新版本
          </span>
        )}

        <button onClick={() => { setShowHistory(!showHistory); if (!showHistory) loadHistory(); }}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors ml-auto">
          <FileText size={12} /> 更新记录
        </button>
      </div>

      {panelUpdateInfo?.error && (
        <div className="flex items-center gap-2 px-4 py-3 rounded-xl bg-red-50 dark:bg-red-900/20 text-xs text-red-700 dark:text-red-300 border border-red-100 dark:border-red-900/30">
          <AlertTriangle size={14} /> {panelUpdateInfo.error}
        </div>
      )}

      {panelUpdateInfo?.hasUpdate && panelUpdateInfo.releaseNote && (
        <div className="space-y-2">
          <div className="flex items-center gap-2 px-4 py-3 rounded-xl bg-amber-50 dark:bg-amber-900/20 text-xs text-amber-700 dark:text-amber-300 border border-amber-100 dark:border-amber-900/30">
            <AlertTriangle size={14} className="shrink-0" />
            <span>发现新版本 <strong>{panelUpdateInfo.latestVersion}</strong>（发布于 {panelUpdateInfo.releaseTime ? new Date(panelUpdateInfo.releaseTime).toLocaleString('zh-CN') : '-'}）</span>
          </div>
          <div className="bg-gray-50 dark:bg-gray-900/50 rounded-xl p-4 border border-gray-100 dark:border-gray-800 text-xs text-gray-700 dark:text-gray-300 whitespace-pre-wrap max-h-48 overflow-y-auto leading-relaxed">
            {panelUpdateInfo.releaseNote}
          </div>
        </div>
      )}

      {showHistory && (
        <div className="space-y-2">
          <h4 className="text-xs font-bold text-gray-700 dark:text-gray-300">📋 更新历史</h4>
          {updateHistory.length === 0 ? (
            <div className="text-center py-4 text-gray-400 text-xs border-2 border-dashed border-gray-100 dark:border-gray-800 rounded-xl">暂无更新记录</div>
          ) : (
            <div className="space-y-1.5 max-h-40 overflow-y-auto">
              {updateHistory.slice().reverse().map((h: any, i: number) => (
                <div key={i} className="flex items-center justify-between p-2.5 rounded-lg bg-gray-50 dark:bg-gray-900/50 border border-gray-100 dark:border-gray-800 text-xs">
                  <div className="flex items-center gap-2">
                    <span className={`w-1.5 h-1.5 rounded-full ${h.result === 'done' ? 'bg-emerald-500' : h.result === 'rolled_back' ? 'bg-amber-500' : 'bg-red-500'}`} />
                    <span className="font-mono text-gray-600 dark:text-gray-400">{h.from || '?'} → {h.to || '?'}</span>
                  </div>
                  <div className="flex items-center gap-2 text-gray-400">
                    <span className="text-[10px]">{h.source || '-'}</span>
                    <span className="text-[10px]">{h.time ? new Date(h.time).toLocaleString('zh-CN') : '-'}</span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function UpdateSection({ versionInfo, updating, setUpdating, updateStatus, setUpdateStatus, updateLog, setUpdateLog, checking, setChecking, setVersionInfo, setMsg, loadVersion }: any) {
  const logRef = useRef<HTMLDivElement>(null);
  const [elapsed, setElapsed] = useState(0);
  const startTimeRef = useRef<number>(0);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Auto-scroll log to bottom
  useEffect(() => {
    if (logRef.current) logRef.current.scrollTop = logRef.current.scrollHeight;
  }, [updateLog]);

  // Elapsed timer
  useEffect(() => {
    if (updateStatus === 'running') {
      if (!startTimeRef.current) startTimeRef.current = Date.now();
      timerRef.current = setInterval(() => setElapsed(Math.floor((Date.now() - startTimeRef.current) / 1000)), 1000);
    } else {
      if (timerRef.current) clearInterval(timerRef.current);
      if (updateStatus !== 'running') startTimeRef.current = 0;
    }
    return () => { if (timerRef.current) clearInterval(timerRef.current); };
  }, [updateStatus]);

  const startUpdate = async () => {
    setUpdating(true);
    try {
      const r = await api.generateUpdateToken();
      if (!r.ok) { alert('生成更新令牌失败: ' + (r.error || '')); setUpdating(false); return; }
      // Redirect to updater page with openclaw mode
      const url = r.updaterURL + '&mode=openclaw';
      window.open(url, '_blank');
    } catch (e) { alert('网络错误: ' + e); }
    finally { setUpdating(false); }
  };

  const fmtElapsed = (s: number) => s < 60 ? `${s}s` : `${Math.floor(s / 60)}m${s % 60}s`;

  return (
    <>
      <div className="flex items-center gap-3 pt-2">
        <button onClick={async () => {
          setChecking(true);
          try {
            const r = await api.checkUpdate();
            if (r.ok) {
              setVersionInfo({ ...versionInfo, ...r, lastCheckedAt: r.checkedAt || new Date().toISOString() });
              setMsg(r.updateAvailable ? `发现新版本: ${r.latestVersion}` : '已是最新版本');
            } else { setMsg('检查更新失败'); }
          } catch { setMsg('检查更新失败'); }
          finally { setChecking(false); setTimeout(() => setMsg(''), 3000); }
        }} disabled={checking || updating}
          className="flex items-center gap-2 px-4 py-2 text-xs font-medium rounded-lg bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/50 disabled:opacity-50 transition-colors">
          <RefreshCw size={14} className={checking ? 'animate-spin' : ''} />
          {checking ? '检查中...' : '检查更新'}
        </button>
        
        {!updating && updateStatus !== 'running' && (
          <button onClick={startUpdate}
            className={`flex items-center gap-2 px-4 py-2 text-xs font-medium rounded-lg shadow-sm transition-all hover:shadow-md ${versionInfo.updateAvailable ? 'bg-amber-500 text-white hover:bg-amber-600 hover:shadow-amber-200 dark:hover:shadow-none' : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'}`}>
            <Package size={14} />
            {versionInfo.updateAvailable ? '前往更新' : '强制更新'}
          </button>
        )}
      </div>

      {versionInfo.updateAvailable && !updating && updateStatus !== 'running' && (
        <div className="px-4 py-3 rounded-xl bg-amber-50 dark:bg-amber-900/20 text-xs text-amber-700 dark:text-amber-300 border border-amber-100 dark:border-amber-900/30 flex items-center gap-2">
          <AlertTriangle size={16} className="shrink-0" />
          <span>有新版本可用！点击「前往更新」可视化一键升级，或在终端运行: <code className="font-mono bg-amber-100 dark:bg-amber-900/50 px-1.5 py-0.5 rounded font-bold">openclaw update</code></span>
        </div>
      )}

      {/* Update progress */}
      {(updating || updateStatus === 'running' || updateLog.length > 0) && (
        <div className="space-y-3 pt-4 border-t border-gray-100 dark:border-gray-800">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2 text-sm font-semibold">
              <Terminal size={16} className="text-gray-400" />
              <span>更新日志</span>
            </div>
            <div className="flex items-center gap-3">
              {updateStatus === 'running' && (
                <span className="text-[10px] font-mono text-gray-400 bg-gray-100 dark:bg-gray-800 px-2 py-0.5 rounded">{fmtElapsed(elapsed)}</span>
              )}
              {updateStatus === 'running' && <span className="flex items-center gap-1.5 text-xs text-blue-500"><RefreshCw size={12} className="animate-spin" /> 正在更新...</span>}
              {updateStatus === 'success' && <span className="flex items-center gap-1.5 text-xs text-emerald-500"><CheckCircle size={12} /> 更新完成</span>}
              {updateStatus === 'failed' && <span className="flex items-center gap-1.5 text-xs text-red-500"><AlertTriangle size={12} /> 更新失败</span>}
              {updateStatus !== 'running' && updateLog.length > 0 && (
                <button onClick={() => { setUpdateLog([]); setUpdateStatus('idle'); }} className="text-[10px] text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">清除</button>
              )}
            </div>
          </div>
          <div ref={logRef} className="bg-gray-900 dark:bg-black rounded-xl p-4 max-h-72 overflow-y-auto font-mono text-[11px] text-gray-300 space-y-0.5 shadow-inner scroll-smooth">
            {updateLog.map((line: string, i: number) => (
              <div key={i} className={`break-all border-l-2 pl-2 py-0.5 ${line.includes('✅') || line.includes('成功') ? 'border-emerald-500 text-emerald-400' : line.includes('❌') || line.includes('失败') || line.includes('error') ? 'border-red-500 text-red-400' : line.includes('⏳') || line.includes('等待') ? 'border-blue-500 text-blue-400' : 'border-gray-700'}`}>
                {line}
              </div>
            ))}
            {updateStatus === 'running' && <div className="animate-pulse text-blue-400 pl-2 pt-1">▌</div>}
          </div>
          {updateStatus === 'running' && (
            <div className="text-[10px] text-gray-400 flex items-center gap-1.5">
              <AlertTriangle size={10} />
              提示：更新正在独立更新工具中执行，请勿关闭更新页面
            </div>
          )}
        </div>
      )}
    </>
  );
}

function ProviderHealthCheck({ pid, prov }: { pid: string; prov: any }) {
  const [checking, setChecking] = useState(false);
  const [result, setResult] = useState<{ healthy?: boolean; error?: string } | null>(null);

  const check = async () => {
    if (!prov.baseUrl || !prov.apiKey) { setResult({ healthy: false, error: '请先填写 Base URL 和 API Key' }); return; }
    setChecking(true); setResult(null);
    try {
      const firstModel = (prov.models || [])[0];
      const modelId = typeof firstModel === 'string' ? firstModel : firstModel?.id;
      const r = await api.checkModelHealth(prov.baseUrl, prov.apiKey, prov.api || 'openai-completions', modelId);
      setResult(r);
    } catch (err: any) {
      setResult({ healthy: false, error: err.message || '检测失败' });
    } finally { setChecking(false); }
  };

  return (
    <div className="flex items-center gap-3 pt-1">
      <button onClick={check} disabled={checking}
        className="flex items-center gap-1.5 px-3 py-1.5 text-[10px] font-medium rounded-lg bg-gray-50 dark:bg-gray-800 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 border border-gray-200 dark:border-gray-700 transition-colors disabled:opacity-50">
        {checking ? <RefreshCw size={12} className="animate-spin" /> : <CheckCircle size={12} />}
        {checking ? '检测中...' : '检测连通性'}
      </button>
      {result && (
        <span className={`flex items-center gap-1.5 text-[10px] font-medium px-2.5 py-1 rounded-full border ${
          result.healthy
            ? 'bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600 border-emerald-100 dark:border-emerald-800/30'
            : 'bg-red-50 dark:bg-red-900/20 text-red-600 border-red-100 dark:border-red-800/30'
        }`}>
          {result.healthy ? <><CheckCircle size={10} /> 可用</> : <><AlertTriangle size={10} /> {result.error || '不可用'}</>}
        </span>
      )}
    </div>
  );
}

function CfgSection({ title, icon: Icon, description, defaultExpanded = false, fields, getVal, setVal }: {
  title: string; icon: any; description?: string; defaultExpanded?: boolean;
  fields: CfgField[];
  getVal: (p: string) => any; setVal: (p: string, v: any) => void;
}) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  return (
    <div className="page-modern-panel overflow-hidden transition-all hover:shadow-md">
      <button onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-4 px-5 py-4 text-left hover:bg-gray-50/50 dark:hover:bg-gray-700/20 transition-colors">
        <div className={`p-2 rounded-xl transition-colors border ${expanded ? 'bg-blue-100/80 dark:bg-blue-900/20 border-blue-100 dark:border-blue-800/30 text-blue-600' : 'bg-gray-100 dark:bg-gray-800 border-transparent text-gray-500'}`}>
          <Icon size={18} />
        </div>
        <div className="flex-1">
          <span className="text-sm font-bold text-gray-900 dark:text-white block">{title}</span>
          {description ? (
            <>
              <span className="text-[10px] text-gray-400 mt-0.5 block leading-relaxed">{description}</span>
              <span className="text-[10px] text-gray-400 mt-1 block">{fields.length} 个配置项</span>
            </>
          ) : (
            <span className="text-[10px] text-gray-400 mt-0.5">{fields.length} 个配置项</span>
          )}
        </div>
        {expanded ? <ChevronDown size={16} className="text-gray-400" /> : <ChevronRight size={16} className="text-gray-400" />}
      </button>
      
      {expanded && (
        <div className="px-5 pb-6 pt-2 border-t border-gray-50 dark:border-gray-800/50 space-y-5 animate-in slide-in-from-top-2 duration-200">
          {fields.map(field => (
            <div key={field.path}>
              <div className="flex items-center justify-between mb-1.5">
                <label className="text-xs font-semibold text-gray-700 dark:text-gray-300">{field.label}</label>
                <code className="text-[9px] text-gray-400 font-mono bg-gray-50 dark:bg-gray-900 px-1.5 py-0.5 rounded border border-gray-100 dark:border-gray-800">{field.path}</code>
              </div>
              
              {field.type === 'toggle' ? (
                <div className="flex items-center gap-3 p-3 rounded-lg border border-gray-100 dark:border-gray-800 bg-gray-50/30 dark:bg-gray-900/30">
                  <button onClick={() => setVal(field.path, !getVal(field.path))}
                    className={`relative w-9 h-5 rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-blue-500 ${getVal(field.path) ? 'bg-blue-600' : 'bg-gray-300 dark:bg-gray-600'}`}>
                    <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow-sm transition-transform ${getVal(field.path) ? 'translate-x-4' : ''}`} />
                  </button>
                  <span className={`text-xs font-medium ${getVal(field.path) ? 'text-blue-600 dark:text-blue-400' : 'text-gray-500'}`}>
                    {getVal(field.path) ? '已启用' : '已禁用'}
                  </span>
                </div>
              ) : field.type === 'textarea' ? (
                <textarea value={getVal(field.path) || ''} onChange={e => setVal(field.path, e.target.value)}
                  placeholder={field.placeholder} rows={4}
                  className="w-full px-3.5 py-2.5 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all resize-none font-mono leading-relaxed" />
              ) : field.type === 'select' ? (
                <div className="relative">
                  <select value={getVal(field.path) || ''} onChange={e => setVal(field.path, e.target.value)}
                    className="w-full px-3.5 py-2.5 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 appearance-none cursor-pointer">
                    <option value="">选择...</option>
                    {field.options?.map(o => <option key={o} value={o}>{o}</option>)}
                  </select>
                  <ChevronDown size={14} className="absolute right-3.5 top-1/2 -translate-y-1/2 text-gray-400 pointer-events-none" />
                </div>
              ) : field.type === 'number' ? (
                <CfgNumberInput
                  value={getVal(field.path)}
                  placeholder={field.placeholder}
                  label={field.label}
                  integer={field.integer}
                  min={field.min}
                  max={field.max}
                  onCommit={next => setVal(field.path, next)}
                />
              ) : (
                <div className="relative group">
                  <input type={field.type === 'password' ? 'password' : 'text'}
                    value={getVal(field.path) ?? ''}
                    onChange={e => setVal(field.path, e.target.value)}
                    placeholder={field.placeholder}
                    className="w-full px-3.5 py-2.5 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all placeholder:text-gray-400" />
                  {field.type === 'password' && (
                    <div className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none">
                      <Key size={14} />
                    </div>
                  )}
                </div>
              )}
              {field.help && (
                <p className="mt-1.5 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">{field.help}</p>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function SessionIsolationSection({
  config,
  updateConfig,
}: {
  config: any;
  updateConfig: (mutate: (draft: any) => void) => void;
}) {
  const [expanded, setExpanded] = useState(true);
  const rawDmScope = typeof readConfigValue(config, 'session.dmScope') === 'string'
    ? String(readConfigValue(config, 'session.dmScope')).trim()
    : '';
  const effectiveDmScope = rawDmScope || 'main';
  const activeCardId = rawDmScope || 'default';
  const scopeCards: Array<{ id: string; value: string; label: string; help: string; preview: string; badge?: string }> = [
    {
      id: 'default',
      value: '',
      label: '默认',
      help: '不写入 session.dmScope，保留当前全局默认行为；运行时仍等价于 main。',
      preview: '不写入 session.dmScope',
      badge: '未显式配置',
    },
    {
      id: 'main',
      value: 'main',
      label: 'main',
      help: '所有私聊尽量复用主会话，隔离最弱，最容易出现上下文串扰。',
      preview: 'session.dmScope = "main"',
    },
    {
      id: 'per-peer',
      value: 'per-peer',
      label: 'per-peer',
      help: '按私聊对端拆分，适合单账号、单渠道的简单场景。',
      preview: 'session.dmScope = "per-peer"',
    },
    {
      id: 'per-channel-peer',
      value: 'per-channel-peer',
      label: 'per-channel-peer',
      help: '按渠道 + 私聊对端拆分，适合多渠道并行时避免跨渠道串上下文。',
      preview: 'session.dmScope = "per-channel-peer"',
    },
    {
      id: 'per-account-channel-peer',
      value: 'per-account-channel-peer',
      label: 'per-account-channel-peer',
      help: '按账号 + 渠道 + 私聊对端拆分，适合飞书多账号或依赖 accountId 路由。',
      preview: 'session.dmScope = "per-account-channel-peer"',
      badge: '飞书推荐',
    },
  ];

  const setDmScope = (next: string) => {
    updateConfig((draft: any) => {
      if (!draft.session || typeof draft.session !== 'object' || Array.isArray(draft.session)) draft.session = {};
      draft.session.dmScope = next;
    });
  };

  const clearDmScope = () => {
    updateConfig((draft: any) => {
      if (draft.session && typeof draft.session === 'object' && !Array.isArray(draft.session)) {
        delete draft.session.dmScope;
        if (Object.keys(draft.session).length === 0) delete draft.session;
      }
    });
  };

  const applyCard = (cardId: string) => {
    const card = scopeCards.find(item => item.id === cardId);
    if (!card) return;
    if (!card.value) {
      clearDmScope();
      return;
    }
    setDmScope(card.value);
  };

  const statusTone =
    !rawDmScope
      ? 'border-gray-200 bg-gray-50/70 text-gray-700 dark:border-gray-700 dark:bg-gray-900/50 dark:text-gray-200'
      : rawDmScope === 'main'
        ? 'border-amber-200 bg-amber-50/70 text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200'
        : rawDmScope === 'per-account-channel-peer'
          ? 'border-emerald-200 bg-emerald-50/70 text-emerald-800 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-200'
          : 'border-sky-200 bg-sky-50/70 text-sky-800 dark:border-sky-900/60 dark:bg-sky-950/30 dark:text-sky-200';

  const statusTitle = rawDmScope
    ? `当前已显式配置 session.dmScope = ${rawDmScope}`
    : '当前使用默认行为（未显式写入 session.dmScope）';

  const statusDescription = rawDmScope
    ? rawDmScope === 'per-account-channel-peer'
      ? '它已经作为全局私聊分桶规则生效，群聊 / 私聊准入策略不会替代它。当前配置也正好是飞书多账号与 accountId 路由场景的推荐值。'
      : rawDmScope === 'main'
        ? '它已经作为全局私聊分桶规则生效，群聊 / 私聊准入策略不会替代它。当前等价于让所有私聊尽量复用主会话，隔离最弱。'
        : '它已经作为全局私聊分桶规则生效，群聊 / 私聊准入策略不会替代它。若你在飞书中启用了多账号或依赖 accountId 路由，可继续收紧到 per-account-channel-peer。'
    : 'session.dmScope 是全局私聊隔离开关。当前配置文件未显式写入该字段，OpenClaw 运行时等价于 main；如果你在飞书中启用了多账号或依赖 accountId 路由，建议显式选择 per-account-channel-peer。';

  return (
    <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 overflow-hidden transition-all hover:shadow-md">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-4 px-5 py-4 text-left hover:bg-gray-50/50 dark:hover:bg-gray-700/20 transition-colors"
      >
        <div className={`p-2 rounded-lg transition-colors ${expanded ? 'bg-sky-100 dark:bg-sky-900/30 text-sky-600' : 'bg-gray-100 dark:bg-gray-800 text-gray-500'}`}>
          <Users size={18} />
        </div>
        <div className="flex-1">
          <span className="text-sm font-bold text-gray-900 dark:text-white block">私聊上下文隔离</span>
          <span className="text-[10px] text-gray-400 mt-0.5 block leading-relaxed">
            配置 <span className="font-mono">session.dmScope</span>，控制 OpenClaw 如何为私聊拆分会话键。
          </span>
          <span className="text-[10px] text-gray-400 mt-1 block">
            当前：{rawDmScope || '未显式设置（运行时等价 main）'}
          </span>
        </div>
        {expanded ? <ChevronDown size={16} className="text-gray-400" /> : <ChevronRight size={16} className="text-gray-400" />}
      </button>

      {expanded && (
        <div className="px-5 pb-6 pt-2 border-t border-gray-50 dark:border-gray-800/50 space-y-5 animate-in slide-in-from-top-2 duration-200">
          <div className={`rounded-xl border p-4 ${statusTone}`}>
            <div className="flex flex-wrap items-center gap-2">
              {!rawDmScope ? <RotateCcw size={16} /> : rawDmScope === 'main' ? <AlertTriangle size={16} /> : <CheckCircle size={16} />}
              <span className="text-xs font-semibold">{statusTitle}</span>
              <span className="ml-auto inline-flex items-center gap-1 rounded-full border border-current/15 bg-white/60 dark:bg-black/10 px-2 py-0.5 text-[10px] font-semibold">
                全局生效
              </span>
              <span className="inline-flex items-center gap-1 rounded-full border border-current/15 bg-white/60 dark:bg-black/10 px-2 py-0.5 text-[10px] font-semibold">
                当前等效：{effectiveDmScope}
              </span>
            </div>
            <p className="mt-2 text-[11px] leading-relaxed opacity-90">{statusDescription}</p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-5 gap-3">
            {scopeCards.map(card => {
              const active = activeCardId === card.id;
              return (
                <button
                  key={card.id}
                  type="button"
                  onClick={() => applyCard(card.id)}
                  className={`text-left rounded-xl border p-4 transition-all ${
                    active
                      ? 'border-sky-300 bg-sky-50/80 dark:border-sky-700 dark:bg-sky-950/25 shadow-sm'
                      : 'border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 hover:border-sky-200 hover:bg-gray-50 dark:hover:bg-gray-900/40'
                  }`}
                >
                  <div className="flex items-center justify-between gap-2">
                    <div>
                      <span className="text-sm font-semibold text-gray-900 dark:text-white block">{card.label}</span>
                      {card.badge && (
                        <span className={`mt-2 inline-flex items-center gap-1 text-[10px] px-2 py-0.5 rounded-full ${
                          card.id === 'per-account-channel-peer'
                            ? 'bg-violet-100 dark:bg-violet-900/30 text-violet-700 dark:text-violet-300'
                            : 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-300'
                        }`}>
                          {card.badge}
                        </span>
                      )}
                    </div>
                    <span className={`w-5 h-5 rounded-full border flex items-center justify-center ${
                      active
                        ? 'border-sky-500 bg-sky-500 text-white'
                        : 'border-gray-300 dark:border-gray-600 text-transparent'
                    }`}>
                      <CheckCircle size={12} />
                    </span>
                  </div>
                  <p className="mt-3 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">{card.help}</p>
                  <div className="mt-3 rounded-lg border border-gray-100 dark:border-gray-800 bg-gray-50/90 dark:bg-gray-950/40 px-3 py-2 text-[11px] font-mono text-gray-600 dark:text-gray-300 break-all">
                    {card.preview}
                  </div>
                  {active && (
                    <div className="mt-3 inline-flex items-center gap-1 text-[10px] font-semibold text-sky-700 dark:text-sky-300">
                      <CheckCircle size={11} />
                      当前选择
                    </div>
                  )}
                </button>
              );
            })}
          </div>

          <div className="rounded-xl border border-sky-100 bg-sky-50/80 dark:border-sky-900/40 dark:bg-sky-950/20 px-4 py-3 text-[12px] text-sky-800 dark:text-sky-200 space-y-1.5">
            <div className="font-medium">选择建议</div>
            <div>
              单账号场景通常从 <span className="font-mono">per-peer</span> 或 <span className="font-mono">per-channel-peer</span> 起步；
              飞书启用多账号、默认账号映射，或 Agent 路由依赖 <span className="font-mono">accountId</span> 时，优先使用
              {' '}
              <span className="font-mono">per-account-channel-peer</span>。
            </div>
            <div>
              只有在你明确希望所有私聊尽量共享主会话时，才建议显式写成 <span className="font-mono">main</span>。
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function ToolGovernanceSection({
  config,
  updateConfig,
}: {
  config: any;
  updateConfig: (mutate: (draft: any) => void) => void;
}) {
  const [expanded, setExpanded] = useState(true);
  const rawProfile = String(readConfigValue(config, 'tools.profile') || '').trim();
  const allowText = formatConfigList(readConfigValue(config, 'tools.allow'));
  const denyText = formatConfigList(readConfigValue(config, 'tools.deny'));
  const [allowDraft, setAllowDraft] = useState(allowText);
  const [denyDraft, setDenyDraft] = useState(denyText);
  const suppressAllowSyncRef = useRef(false);
  const suppressDenySyncRef = useRef(false);

  useEffect(() => {
    if (suppressAllowSyncRef.current) {
      suppressAllowSyncRef.current = false;
      return;
    }
    setAllowDraft(allowText);
  }, [allowText]);

  useEffect(() => {
    if (suppressDenySyncRef.current) {
      suppressDenySyncRef.current = false;
      return;
    }
    setDenyDraft(denyText);
  }, [denyText]);

  const mutateTools = (mutate: (toolsDraft: Record<string, any>) => void) => {
    updateConfig((draft: any) => {
      if (!draft.tools || typeof draft.tools !== 'object' || Array.isArray(draft.tools)) draft.tools = {};
      mutate(draft.tools);
    });
  };

  const setToolList = (key: 'allow' | 'deny', value: string) => {
    mutateTools((toolsDraft) => {
      const list = parseConfigListInput(value);
      if (list.length > 0) toolsDraft[key] = list;
      else delete toolsDraft[key];
    });
  };

  const setToolProfile = (next: string) => {
    mutateTools((toolsDraft) => {
      if (next) toolsDraft.profile = next;
      else delete toolsDraft.profile;
    });
  };

  const clearToolProfile = () => setToolProfile('');

  const applyHeadlessPreset = () => {
    mutateTools((toolsDraft) => {
      toolsDraft.profile = 'minimal';
      toolsDraft.allow = ['group:web', 'group:fs'];
      toolsDraft.deny = ['group:runtime', 'group:ui', 'group:nodes', 'group:automation'];
    });
  };

  const currentProfileMeta = (rawProfile && rawProfile in TOOL_GOVERNANCE_PRESETS)
    ? TOOL_GOVERNANCE_PRESETS[rawProfile as ToolProfilePreset]
    : null;
  const allowCount = parseConfigListInput(allowDraft).length;
  const denyCount = parseConfigListInput(denyDraft).length;
  const statusTone =
    !rawProfile
      ? 'border-gray-200 bg-gray-50/70 text-gray-700 dark:border-gray-700 dark:bg-gray-900/50 dark:text-gray-200'
      : rawProfile === 'minimal'
        ? 'border-emerald-200 bg-emerald-50/70 text-emerald-800 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-200'
        : rawProfile === 'full'
          ? 'border-amber-200 bg-amber-50/70 text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200'
          : 'border-violet-200 bg-violet-50/70 text-violet-800 dark:border-violet-900/60 dark:bg-violet-950/30 dark:text-violet-200';

  return (
    <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 overflow-hidden transition-all hover:shadow-md">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-4 px-5 py-4 text-left hover:bg-gray-50/50 dark:hover:bg-gray-700/20 transition-colors"
      >
        <div className={`p-2 rounded-lg transition-colors ${expanded ? 'bg-violet-100 dark:bg-violet-900/30 text-violet-600' : 'bg-gray-100 dark:bg-gray-800 text-gray-500'}`}>
          <Shield size={18} />
        </div>
        <div className="flex-1">
          <span className="text-sm font-bold text-gray-900 dark:text-white block">工具治理</span>
          <span className="text-[10px] text-gray-400 mt-0.5 block leading-relaxed">
            把 <span className="font-mono">tools.profile / tools.allow / tools.deny</span> 做成可视化入口，方便把 OpenClaw 收敛到“传统无头”范围。
          </span>
          <span className="text-[10px] text-gray-400 mt-1 block">
            当前 profile：{rawProfile || '未显式设置'}
          </span>
        </div>
        {expanded ? <ChevronDown size={16} className="text-gray-400" /> : <ChevronRight size={16} className="text-gray-400" />}
      </button>

      {expanded && (
        <div className="px-5 pb-6 pt-2 border-t border-gray-50 dark:border-gray-800/50 space-y-5 animate-in slide-in-from-top-2 duration-200">
          <div className={`rounded-xl border p-4 ${statusTone}`}>
            <div className="flex flex-wrap items-center gap-2">
              {!rawProfile ? <RotateCcw size={16} /> : rawProfile === 'full' ? <AlertTriangle size={16} /> : <CheckCircle size={16} />}
              <span className="text-xs font-semibold">
                {rawProfile ? `当前 profile = ${currentProfileMeta?.label || rawProfile}` : '当前未显式设置 tools.profile'}
              </span>
              <span className="ml-auto inline-flex items-center gap-1 rounded-full border border-current/15 bg-white/60 dark:bg-black/10 px-2 py-0.5 text-[10px] font-semibold">
                allow {allowCount} 条
              </span>
              <span className="inline-flex items-center gap-1 rounded-full border border-current/15 bg-white/60 dark:bg-black/10 px-2 py-0.5 text-[10px] font-semibold">
                deny {denyCount} 条
              </span>
            </div>
            <p className="mt-2 text-[11px] leading-relaxed opacity-90">
              {rawProfile
                ? `${currentProfileMeta?.help || '当前配置使用自定义工具预设。'} deny 的优先级始终高于 allow；被拒绝的工具不会再暴露给模型。`
                : 'tools.profile / tools.allow / tools.deny 共同决定模型能看到哪些工具。浏览器、原生命令、重启和插件启停仍分别在本页其他区域或插件页管理；这里专门负责“模型能看到哪些工具”。'}
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-3">
            {(Object.entries(TOOL_GOVERNANCE_PRESETS) as Array<[ToolProfilePreset, { label: string; help: string }]>).map(([presetId, meta]) => {
              const active = rawProfile === presetId;
              return (
                <button
                  key={presetId}
                  type="button"
                  onClick={() => setToolProfile(presetId)}
                  className={`text-left rounded-xl border p-4 transition-all ${
                    active
                      ? 'border-violet-300 bg-violet-50/80 dark:border-violet-700 dark:bg-violet-950/25 shadow-sm'
                      : 'border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 hover:border-violet-200 hover:bg-gray-50 dark:hover:bg-gray-900/40'
                  }`}
                >
                  <div className="flex items-center justify-between gap-2">
                    <div className="text-sm font-semibold text-gray-900 dark:text-white">{meta.label}</div>
                    <span className={`w-5 h-5 rounded-full border flex items-center justify-center ${
                      active
                        ? 'border-violet-500 bg-violet-500 text-white'
                        : 'border-gray-300 dark:border-gray-600 text-transparent'
                    }`}>
                      <CheckCircle size={12} />
                    </span>
                  </div>
                  <p className="mt-3 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">{meta.help}</p>
                  {active && (
                    <div className="mt-3 inline-flex items-center gap-1 text-[10px] font-semibold text-violet-700 dark:text-violet-300">
                      <CheckCircle size={11} />
                      当前选择
                    </div>
                  )}
                </button>
              );
            })}
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-[minmax(0,320px),1fr] gap-4">
            <div className="rounded-[22px] border border-violet-100/80 dark:border-violet-900/40 bg-[linear-gradient(145deg,rgba(255,255,255,0.84),rgba(245,243,255,0.72))] dark:bg-[linear-gradient(145deg,rgba(24,16,42,0.82),rgba(76,29,149,0.16))] p-4 space-y-4">
              <div>
                <div className="text-sm font-semibold text-gray-900 dark:text-white">快捷操作</div>
                <p className="mt-2 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
                  如果你想把 OpenClaw 收敛到“传统无头”范围，可以直接套用建议预设；如果更想完全手控 allow / deny，则把 profile 恢复为未配置即可。
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <button
                  type="button"
                  onClick={applyHeadlessPreset}
                  className="inline-flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700"
                >
                  <Shield size={12} />
                  一键套用“传统无头”建议
                </button>
                <button
                  type="button"
                  onClick={clearToolProfile}
                  className="inline-flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg border border-violet-200 text-violet-700 hover:bg-white dark:border-violet-800 dark:text-violet-300 dark:hover:bg-violet-950/30"
                >
                  <RotateCcw size={12} />
                  恢复为未配置
                </button>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="rounded-xl border border-white/80 dark:border-slate-800 bg-white/80 dark:bg-slate-900/50 px-3 py-2.5">
                  <div className="text-[11px] text-gray-500">当前 profile</div>
                  <div className="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{currentProfileMeta?.label || '未配置'}</div>
                </div>
                <div className="rounded-xl border border-white/80 dark:border-slate-800 bg-white/80 dark:bg-slate-900/50 px-3 py-2.5">
                  <div className="text-[11px] text-gray-500">治理优先级</div>
                  <div className="mt-1 text-sm font-semibold text-gray-900 dark:text-white">deny &gt; allow</div>
                </div>
              </div>
              <p className="text-[11px] text-gray-500 dark:text-gray-400 leading-relaxed">
                建议预设会把 profile 设为 <span className="font-mono">minimal</span>，只补回 <span className="font-mono">group:web / group:fs</span>，
                并显式拒绝 <span className="font-mono">runtime / ui / nodes / automation</span> 四大扩展面。
              </p>
            </div>

            <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
              <div className="rounded-xl border border-gray-100 dark:border-gray-800 bg-gray-50/60 dark:bg-gray-900/30 p-4">
                <div className="flex items-center justify-between">
                  <label className="text-xs font-semibold text-gray-700 dark:text-gray-300">tools.allow</label>
                  <span className="text-[10px] text-gray-400">{allowCount} 条</span>
                </div>
                <p className="mt-1 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
                  支持逗号、中文逗号或换行分隔。这里写“额外放行”的工具组，适合在 profile 之外做少量补充。
                </p>
                <textarea
                  rows={4}
                  value={allowDraft}
                  onChange={e => {
                    suppressAllowSyncRef.current = true;
                    setAllowDraft(e.target.value);
                    setToolList('allow', e.target.value);
                  }}
                  placeholder="例如: group:web, group:fs"
                  className="w-full mt-3 px-3 py-2.5 text-sm font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                />
              </div>
              <div className="rounded-xl border border-gray-100 dark:border-gray-800 bg-gray-50/60 dark:bg-gray-900/30 p-4">
                <div className="flex items-center justify-between">
                  <label className="text-xs font-semibold text-gray-700 dark:text-gray-300">tools.deny</label>
                  <span className="text-[10px] text-gray-400">{denyCount} 条</span>
                </div>
                <p className="mt-1 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
                  deny 的优先级始终高于 allow。这里适合写你明确不想暴露给模型的工具面。
                </p>
                <textarea
                  rows={4}
                  value={denyDraft}
                  onChange={e => {
                    suppressDenySyncRef.current = true;
                    setDenyDraft(e.target.value);
                    setToolList('deny', e.target.value);
                  }}
                  placeholder="例如: group:runtime, group:ui, group:nodes, group:automation"
                  className="w-full mt-3 px-3 py-2.5 text-sm font-mono border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                />
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function BrowserControlSection({
  config,
  updateConfig,
}: {
  config: any;
  updateConfig: (mutate: (draft: any) => void) => void;
}) {
  const [expanded, setExpanded] = useState(true);
  const browser = getBrowserConfigDraft(config);
  const enabled = getEffectiveBrowserEnabled(config);
  const rawDefaultProfile = getRawBrowserDefaultProfile(config);
  const effectiveDefaultProfile = getEffectiveBrowserDefaultProfile(config);
  const preset = getBrowserControlPreset(config);
  const profileMode = getBrowserProfileMode(config);
  const advancedKeys = Object.keys(browser).filter(key => !['enabled', 'defaultProfile'].includes(key));

  const mutateBrowser = (mutate: (browserDraft: Record<string, any>) => void) => {
    updateConfig((draft: any) => {
      if (!draft.browser || typeof draft.browser !== 'object' || Array.isArray(draft.browser)) draft.browser = {};
      mutate(draft.browser);
    });
  };

  const applyPreset = (next: Extract<BrowserControlPreset, 'disabled' | 'managed'>) => {
    mutateBrowser((browserDraft) => {
      if (next === 'disabled') {
        browserDraft.enabled = false;
        delete browserDraft.defaultProfile;
        return;
      }
      browserDraft.enabled = true;
      browserDraft.defaultProfile = 'openclaw';
    });
  };

  const toggleEnabled = () => {
    mutateBrowser((browserDraft) => {
      const next = browserDraft.enabled === false;
      browserDraft.enabled = next;
      if (next) {
        browserDraft.defaultProfile = 'openclaw';
      } else {
        delete browserDraft.defaultProfile;
      }
    });
  };

  const setProfileMode = (next: Exclude<BrowserProfileMode, 'custom'>) => {
    mutateBrowser((browserDraft) => {
      browserDraft.enabled = true;
      browserDraft.defaultProfile = next;
    });
  };

  const statusTone =
    preset === 'disabled'
      ? 'border-gray-200 bg-gray-50/70 text-gray-700 dark:border-gray-700 dark:bg-gray-900/50 dark:text-gray-200'
      : preset === 'managed'
        ? 'border-emerald-200 bg-emerald-50/70 text-emerald-800 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-200'
        : 'border-amber-200 bg-amber-50/70 text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200';

  const statusTitle =
    preset === 'disabled'
      ? '当前状态：浏览器控制已禁用'
      : preset === 'managed'
        ? advancedKeys.length > 0
          ? `当前状态：默认 profile = ${effectiveDefaultProfile}（含高级 browser.* 配置）`
          : `当前状态：托管浏览器（${effectiveDefaultProfile}）`
        : `当前状态：自定义 browser.defaultProfile = ${effectiveDefaultProfile}`;

  const statusDescription =
    preset === 'disabled'
      ? '等价于方案 A。OpenClaw 不再主动触发浏览器控制。'
      : preset === 'managed'
        ? advancedKeys.length > 0
          ? '默认 profile 已锁定为 openclaw，但检测到其他 browser.* 高级字段；因此它不一定完全等价于最小托管预设，请同时结合这些高级项判断真实运行行为。'
          : '等价于方案 B。会显式锁定到 OpenClaw 托管的独立 profile，避免依赖上游默认值。'
        : '当前配置偏离了这两个安全预设，请确认这就是你想要的浏览器接管方式。';

  return (
    <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 overflow-hidden transition-all hover:shadow-md">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-4 px-5 py-4 text-left hover:bg-gray-50/50 dark:hover:bg-gray-700/20 transition-colors"
      >
        <div className={`p-2 rounded-lg transition-colors ${expanded ? 'bg-violet-100 dark:bg-violet-900/30 text-violet-600' : 'bg-gray-100 dark:bg-gray-800 text-gray-500'}`}>
          <Monitor size={18} />
        </div>
        <div className="flex-1">
          <span className="text-sm font-bold text-gray-900 dark:text-white block">浏览器控制</span>
          <span className="text-[10px] text-gray-400 mt-0.5 block leading-relaxed">
            围绕“彻底禁用浏览器控制 / 显式锁定托管 openclaw profile”两个安全方案做可视化管控。
          </span>
          <span className="text-[10px] text-gray-400 mt-1 block">2 个核心开关 + 2 个安全预设</span>
        </div>
        {expanded ? <ChevronDown size={16} className="text-gray-400" /> : <ChevronRight size={16} className="text-gray-400" />}
      </button>

      {expanded && (
        <div className="px-5 pb-6 pt-2 border-t border-gray-50 dark:border-gray-800/50 space-y-5 animate-in slide-in-from-top-2 duration-200">
          <div className={`rounded-xl border p-4 ${statusTone}`}>
            <div className="flex items-center gap-2">
              {preset === 'disabled' ? <EyeOff size={16} /> : preset === 'managed' ? <CheckCircle size={16} /> : <AlertTriangle size={16} />}
              <span className="text-xs font-semibold">{statusTitle}</span>
            </div>
            <p className="mt-2 text-[11px] leading-relaxed opacity-90">{statusDescription}</p>
            {enabled && !rawDefaultProfile && (
              <p className="mt-2 text-[11px] leading-relaxed opacity-90">
                当前配置未显式写入 <code className="mx-0.5 rounded border border-current/20 bg-white/60 px-1 py-0.5 font-mono dark:bg-black/10">browser.defaultProfile</code>，
                但根据上游代码实际会默认使用 <code className="mx-0.5 rounded border border-current/20 bg-white/60 px-1 py-0.5 font-mono dark:bg-black/10">openclaw</code>。
                点击下方“托管浏览器”预设可把这个默认值固化进配置，避免后续版本/文档漂移。
              </p>
            )}
            {advancedKeys.length > 0 && (
              <p className="mt-2 text-[11px] leading-relaxed opacity-90">
                检测到其他 <code className="mx-0.5 rounded border border-current/20 bg-white/60 px-1 py-0.5 font-mono dark:bg-black/10">browser.*</code> 高级字段：
                {' '}
                <span className="font-mono">{advancedKeys.join(', ')}</span>。本区只会调整
                {' '}
                <code className="mx-0.5 rounded border border-current/20 bg-white/60 px-1 py-0.5 font-mono dark:bg-black/10">browser.enabled</code>
                和
                {' '}
                <code className="mx-0.5 rounded border border-current/20 bg-white/60 px-1 py-0.5 font-mono dark:bg-black/10">browser.defaultProfile</code>，不会覆盖这些高级项。
              </p>
            )}
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <button
              type="button"
              onClick={() => applyPreset('disabled')}
              className={`rounded-xl border p-4 text-left transition-all ${
                preset === 'disabled'
                  ? 'border-violet-300 bg-violet-50 dark:border-violet-700 dark:bg-violet-950/20 shadow-sm'
                  : 'border-gray-200 dark:border-gray-700 hover:border-violet-200 hover:bg-gray-50 dark:hover:bg-gray-900/40'
              }`}
            >
              <div className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                <EyeOff size={16} className="text-gray-500 dark:text-gray-300" />
                方案 A：彻底禁用浏览器控制
              </div>
              <p className="mt-2 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
                适合你根本不想让 OpenClaw 碰浏览器的场景。保存后核心行为等价于：
              </p>
              <pre className="mt-2 rounded-lg border border-gray-100 dark:border-gray-800 bg-gray-50 dark:bg-gray-900/60 p-3 text-[11px] text-gray-700 dark:text-gray-300 overflow-x-auto font-mono">{`{\n  "browser": {\n    "enabled": false\n  }\n}`}</pre>
            </button>

            <button
              type="button"
              onClick={() => applyPreset('managed')}
              className={`rounded-xl border p-4 text-left transition-all ${
                preset === 'managed'
                  ? 'border-violet-300 bg-violet-50 dark:border-violet-700 dark:bg-violet-950/20 shadow-sm'
                  : 'border-gray-200 dark:border-gray-700 hover:border-violet-200 hover:bg-gray-50 dark:hover:bg-gray-900/40'
              }`}
            >
              <div className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                <Monitor size={16} className="text-violet-600 dark:text-violet-400" />
                方案 B：托管浏览器（推荐）
              </div>
              <p className="mt-2 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
                显式锁定到 OpenClaw 托管的 <span className="font-mono">openclaw</span> profile，使用独立 user-data-dir，
                避免把运行行为建立在上游默认值之上。
              </p>
              <pre className="mt-2 rounded-lg border border-gray-100 dark:border-gray-800 bg-gray-50 dark:bg-gray-900/60 p-3 text-[11px] text-gray-700 dark:text-gray-300 overflow-x-auto font-mono">{`{\n  "browser": {\n    "enabled": true,\n    "defaultProfile": "openclaw"\n  }\n}`}</pre>
            </button>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
            <div>
              <div className="flex items-center justify-between mb-1.5">
                <label className="text-xs font-semibold text-gray-700 dark:text-gray-300">启用浏览器控制</label>
                <code className="text-[9px] text-gray-400 font-mono bg-gray-50 dark:bg-gray-900 px-1.5 py-0.5 rounded border border-gray-100 dark:border-gray-800">browser.enabled</code>
              </div>
              <div className="flex items-center gap-3 p-3 rounded-lg border border-gray-100 dark:border-gray-800 bg-gray-50/30 dark:bg-gray-900/30">
                <button
                  onClick={toggleEnabled}
                  className={`relative w-9 h-5 rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-violet-500 ${enabled ? 'bg-violet-600' : 'bg-gray-300 dark:bg-gray-600'}`}
                >
                  <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow-sm transition-transform ${enabled ? 'translate-x-4' : ''}`} />
                </button>
                <span className={`text-xs font-medium ${enabled ? 'text-violet-600 dark:text-violet-400' : 'text-gray-500'}`}>
                  {enabled ? '已启用' : '已禁用'}
                </span>
              </div>
              <p className="mt-1.5 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
                关闭后等价于方案 A，并会移除当前显式的 <span className="font-mono">browser.defaultProfile</span>；后续再次开启时，会自动回到更安全的 <span className="font-mono">openclaw</span>。
              </p>
            </div>

            <div>
              <div className="flex items-center justify-between mb-1.5">
                <label className="text-xs font-semibold text-gray-700 dark:text-gray-300">默认浏览器 Profile</label>
                <code className="text-[9px] text-gray-400 font-mono bg-gray-50 dark:bg-gray-900 px-1.5 py-0.5 rounded border border-gray-100 dark:border-gray-800">browser.defaultProfile</code>
              </div>
              <div className="space-y-2">
                <div className="relative">
                  <select
                    value={profileMode === 'custom' ? 'custom' : profileMode}
                    onChange={e => setProfileMode(e.target.value as Exclude<BrowserProfileMode, 'custom'>)}
                    disabled={!enabled}
                    className="w-full px-3.5 py-2.5 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500 appearance-none cursor-pointer disabled:opacity-60 disabled:cursor-not-allowed"
                  >
                    {profileMode === 'custom' && (
                      <option value="custom" disabled>
                        custom（当前：{effectiveDefaultProfile}）
                      </option>
                    )}
                    <option value="openclaw">openclaw（托管独立 profile）</option>
                    <option value="chrome">chrome（扩展 relay / 系统 Chromium 标签页）</option>
                  </select>
                  <ChevronDown size={14} className="absolute right-3.5 top-1/2 -translate-y-1/2 text-gray-400 pointer-events-none" />
                </div>
              </div>
              {!enabled ? (
                <p className="mt-1.5 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
                  先开启浏览器控制，再决定默认 profile。若只想要最稳妥方案，直接点击上面的“托管浏览器（推荐）”即可。
                </p>
              ) : profileMode === 'openclaw' ? (
                <p className="mt-1.5 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
                  这就是方案 B：默认使用 OpenClaw 托管浏览器，尽量避免影响系统浏览器的日常个人登录态。
                </p>
              ) : profileMode === 'chrome' ? (
                <p className="mt-1.5 text-[11px] leading-relaxed text-amber-600 dark:text-amber-400">
                  <span className="font-semibold">谨慎使用：</span>这会走 Chrome 扩展 relay。只有你手动附加（badge 为 ON）的系统 Chromium 标签页会被控制，但不建议挂在日常个人 profile 上。
                </p>
              ) : (
                <p className="mt-1.5 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
                  当前是自定义 profile：<span className="font-mono">{effectiveDefaultProfile}</span>。本区会保留它，但这里只提供切回内建
                  <span className="font-mono"> openclaw </span>
                  或
                  <span className="font-mono"> chrome </span>
                  的可视化入口；如需继续维护自定义
                  <span className="font-mono"> browser.profiles.* </span>
                  请继续使用高级 JSON / 配置文件。
                </p>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function CfgNumberInput({
  value,
  placeholder,
  label,
  integer,
  min,
  max,
  onCommit,
}: {
  value: any;
  placeholder?: string;
  label: string;
  integer?: boolean;
  min?: number;
  max?: number;
  onCommit: (next: number | undefined) => void;
}) {
  const [draft, setDraft] = useState(value === undefined || value === null ? '' : String(value));
  const [focused, setFocused] = useState(false);

  useEffect(() => {
    if (!focused) setDraft(value === undefined || value === null ? '' : String(value));
  }, [focused, value]);

  const commit = (raw: string) => {
    const trimmed = raw.trim();
    if (!trimmed) {
      onCommit(undefined);
      return;
    }
    const parsed = Number(trimmed);
    const error = validateNumericFieldValue(parsed, { label, integer, min, max });
    if (Number.isFinite(parsed) && !error) onCommit(parsed);
  };

  return (
    <input
      type="text"
      inputMode="decimal"
      value={draft}
      onFocus={() => setFocused(true)}
      onBlur={() => {
        setFocused(false);
        commit(draft);
      }}
      onChange={e => {
        const raw = e.target.value;
        if (!/^-?(?:\d+)?(?:\.\d*)?$/.test(raw)) return;
        setDraft(raw);
        const trimmed = raw.trim();
        if (!trimmed) {
          onCommit(undefined);
          return;
        }
        if (trimmed !== '-' && trimmed !== '.' && trimmed !== '-.' && !trimmed.endsWith('.')) {
          commit(trimmed);
        }
      }}
      placeholder={placeholder}
      className="w-full px-3.5 py-2.5 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500 transition-all placeholder:text-gray-400"
    />
  );
}

function SoftwareEnvironment({ envInfo, onRefresh }: { envInfo: any; onRefresh: () => void }) {
  const [installing, setInstalling] = useState<string | null>(null);
  const [installMsg, setInstallMsg] = useState('');
  const [swData, setSwData] = useState<any[]>([]);

  useEffect(() => {
    api.getSoftwareList().then(r => { if (r.ok) setSwData(r.software || []); }).catch(() => {});
  }, []);

  // Merge backend software data with envInfo for display
  const getSw = (id: string) => swData.find(s => s.id === id);

  const softwareList = [
    { id: 'nodejs', name: 'Node.js', value: envInfo.software?.node, required: true, category: 'runtime', installable: true },
    { id: 'npm', name: 'npm', value: envInfo.software?.npm, required: false, installable: false, category: 'runtime' },
    { id: 'docker', name: 'Docker', value: envInfo.software?.docker, required: false, category: 'runtime', installable: true },
    { id: 'git', name: 'Git', value: envInfo.software?.git, required: false, category: 'runtime', installable: true },
    { id: 'python', name: 'Python 3', value: envInfo.software?.python, required: false, category: 'runtime', installable: true },
    { id: 'openclaw', name: 'OpenClaw', value: envInfo.software?.openclaw, required: true, category: 'service', installable: true },
    { id: 'napcat', name: getSw('napcat')?.name || 'NapCat (QQ个人号)', value: getSw('napcat')?.version || null, required: false, category: 'container', installable: true, status: getSw('napcat')?.status },
    { id: 'wechat', name: getSw('wechat')?.name || '微信机器人', value: getSw('wechat')?.version || null, required: false, category: 'container', installable: true, status: getSw('wechat')?.status },
  ];

  const handleInstall = async (id: string) => {
    setInstalling(id);
    setInstallMsg('');
    try {
      const r = await api.installSoftware(id);
      if (r.ok) {
        setInstallMsg(`✅ ${id} 安装任务已创建，请在消息中心查看进度`);
      } else {
        setInstallMsg(`❌ ${r.error || '安装失败'}`);
      }
    } catch (err: any) {
      setInstallMsg(`❌ ${err.message || '请求失败'}`);
    } finally {
      setInstalling(null);
      setTimeout(() => { setInstallMsg(''); onRefresh(); }, 5000);
    }
  };

  const categories = [
    { key: 'runtime', label: '运行时环境', icon: Terminal },
    { key: 'service', label: '核心服务', icon: Brain },
    { key: 'container', label: '容器组件', icon: Box },
  ];

  return (
    <>
      {installMsg && (
        <div className={`px-4 py-3 rounded-xl text-sm font-medium flex items-center gap-2 ${installMsg.includes('❌') ? 'bg-red-50 dark:bg-red-900/30 text-red-600' : 'bg-emerald-50 dark:bg-emerald-900/30 text-emerald-600'}`}>
          {installMsg}
        </div>
      )}
      {categories.map(cat => {
        const items = softwareList.filter(s => s.category === cat.key);
        if (items.length === 0) return null;
        return (
          <div key={cat.key} className="page-modern-panel p-6 space-y-4">
            <h3 className="text-sm font-bold text-gray-900 dark:text-white flex items-center gap-2">
              <cat.icon size={16} className="text-blue-500" /> {cat.label}
            </h3>
            <div className="space-y-3">
              {items.map(sw => {
                const pluginMissing = sw.status === 'plugin_missing';
                const installed = pluginMissing || (sw.value && !sw.value.includes('not installed') && !sw.value.includes('not found')) || sw.status === 'running' || sw.status === 'exited';
                const isRunning = sw.status === 'running';
                const installable = sw.installable !== false && !installed;
                return (
                  <div key={sw.id} className="flex items-center gap-4 px-4 py-3 rounded-xl bg-[linear-gradient(145deg,rgba(255,255,255,0.74),rgba(239,246,255,0.56))] dark:bg-[linear-gradient(145deg,rgba(12,24,42,0.8),rgba(30,64,175,0.08))] border border-blue-100/70 dark:border-blue-800/20 hover:border-blue-200 dark:hover:border-blue-800/40 transition-colors backdrop-blur-xl">
                    <div className={`w-8 h-8 rounded-lg flex items-center justify-center shrink-0 ${pluginMissing ? 'bg-amber-100 dark:bg-amber-900/30 text-amber-600' : installed ? (isRunning ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-600' : 'bg-blue-100 dark:bg-blue-900/30 text-blue-600') : 'bg-gray-200 dark:bg-gray-700 text-gray-400'}`}>
                      {installed && !pluginMissing ? <CheckCircle size={16} /> : <AlertTriangle size={16} />}
                    </div>
                    <div className="w-32 shrink-0">
                      <span className="text-sm font-bold text-gray-900 dark:text-white">{sw.name}</span>
                      {sw.required && <span className="block text-[10px] text-amber-600 dark:text-amber-500 font-medium">必需组件</span>}
                    </div>
                    <span className="text-xs text-gray-600 dark:text-gray-400 font-mono flex-1 truncate bg-white dark:bg-gray-800 px-2 py-1 rounded border border-gray-100 dark:border-gray-700">
                      {pluginMissing ? '已安装 NapCat，但缺少 QQ 个人号插件' : (sw.value || (installed ? 'Docker 容器' : '未安装'))}
                    </span>
                    {installed ? (
                      <span className={`text-xs font-medium px-2.5 py-1 rounded-lg whitespace-nowrap ${pluginMissing ? 'bg-amber-50 dark:bg-amber-900/20 text-amber-600 dark:text-amber-400' : isRunning ? 'bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600 dark:text-emerald-400' : 'bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400'}`}>
                        {pluginMissing ? '缺少插件' : (isRunning ? '运行中' : (sw.status === 'exited' ? '已停止' : '已安装'))}
                      </span>
                    ) : installable ? (
                      <button
                        onClick={() => handleInstall(sw.id)}
                        disabled={installing === sw.id}
                        className="page-modern-accent px-3 py-1.5 text-xs font-medium disabled:opacity-50 whitespace-nowrap"
                      >
                        {installing === sw.id ? <RefreshCw size={12} className="animate-spin" /> : <Package size={12} />}
                        {installing === sw.id ? '安装中...' : '一键安装'}
                      </button>
                    ) : (
                      <span className="text-xs font-medium px-2.5 py-1 rounded-lg bg-gray-100 dark:bg-gray-700 text-gray-500 whitespace-nowrap">
                        未安装
                      </span>
                    )}
                  </div>
                );
              })}
            </div>
          </div>
        );
      })}
    </>
  );
}

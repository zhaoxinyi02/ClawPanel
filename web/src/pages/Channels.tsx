import { useEffect, useState, useCallback, useRef } from 'react';
import { useNavigate, useOutletContext, useSearchParams } from 'react-router-dom';
import QRCode from 'qrcode';
import { api } from '../lib/api';
import { Radio, Wifi, WifiOff, QrCode, Key, Zap, UserCheck, Check, X, Power, Loader2, RefreshCw, LogOut, Sparkles, Download, Package, Wrench, Search, Copy, CheckCircle, AlertTriangle, AlertCircle, Trash2, ChevronDown } from 'lucide-react';
import InfoTooltip from '../components/InfoTooltip';
import { useI18n } from '../i18n';

type ChannelFieldSection = 'default' | 'access' | 'conversation' | 'advanced';

type ChannelConfigField = {
  key: string;
  label: string;
  type: 'text' | 'password' | 'toggle' | 'number' | 'select' | 'textarea';
  options?: string[];
  placeholder?: string;
  help?: string;
  defaultValue?: string | number | boolean;
  section?: ChannelFieldSection;
  rows?: number;
  valueFormat?: 'stringArray';
};

type ChannelDef = {
  id: string; label: string; description: string; type: 'builtin' | 'plugin';
  configFields: ChannelConfigField[];
  loginMethods?: ('qrcode' | 'quick' | 'password')[];
};

type ChannelCatalogItem = {
  id: string;
  label: string;
  description: string;
  type: 'builtin' | 'plugin';
  bundled?: boolean;
  channels?: string[];
  envVars?: string[];
  configSchema?: Record<string, any>;
  uiHints?: Record<string, any>;
};

type FeishuDMDiagnosis = {
  configuredDmScope?: string;
  effectiveDmScope: string;
  recommendedDmScope: string;
  defaultAgent: string;
  scannedAgentIds?: string[];
  accountCount: number;
  accountIds?: string[];
  defaultAccount?: string;
  dmPolicy?: string;
  threadSession?: boolean;
  unsupportedChannelDmScope?: string;
  sessionFilePath?: string;
  sessionIndexExists?: boolean;
  feishuSessionCount?: number;
  feishuSessionKeys?: string[];
  hasSharedMainSessionKey?: boolean;
  mainSessionKey?: string;
  credentialsDir?: string;
  pairingStorePath?: string;
  pendingPairingCount?: number;
  authorizedSenderCount?: number;
  authorizedSenders?: FeishuAuthorizedSenderBucket[];
};

type FeishuAuthorizedSenderBucket = {
  accountId: string;
  accountConfigured?: boolean;
  senderCount?: number;
  senderIds?: string[];
  sourceFiles?: string[];
};

type OpenClawPairingRequest = {
  id: string;
  code: string;
  createdAt: string;
  lastSeenAt?: string;
  meta?: Record<string, string>;
};

const PAIRING_CAPABLE_CHANNELS = new Set([
  'telegram',
  'feishu',
  'wecom',
  'whatsapp',
  'signal',
  'discord',
  'slack',
  'line',
  'matrix',
  'zalo',
  'zalouser',
  'nextcloud-talk',
  'synology-chat',
  'googlechat',
  'bluebubbles',
  'imessage',
]);

function isPlainObject(value: any): value is Record<string, any> {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

function deepClone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value ?? {}));
}

function getNestedValue(raw: any, path: string): any {
  return path.split('.').reduce<any>((acc, key) => (isPlainObject(acc) ? acc[key] : undefined), raw);
}

function setNestedValue(raw: Record<string, any>, path: string, value: any) {
  const keys = path.split('.');
  let cur: Record<string, any> = raw;
  for (let i = 0; i < keys.length - 1; i++) {
    const key = keys[i];
    if (!isPlainObject(cur[key])) cur[key] = {};
    cur = cur[key];
  }
  cur[keys[keys.length - 1]] = value;
}

function deleteNestedValue(raw: Record<string, any>, path: string) {
  const keys = path.split('.');
  let cur: Record<string, any> | undefined = raw;
  for (let i = 0; i < keys.length - 1; i++) {
    const key = keys[i];
    if (!isPlainObject(cur?.[key])) return;
    cur = cur[key];
  }
  if (!cur) return;
  delete cur[keys[keys.length - 1]];
}

function listFeishuRawAccountIDs(cfg: any): string[] {
  const accounts = isPlainObject(cfg?.accounts) ? cfg.accounts : {};
  return Object.keys(accounts).map(id => id.trim()).filter(Boolean).sort((a, b) => a.localeCompare(b));
}

function findFeishuMirroredDefaultAccount(cfg: any): string {
  const topAppId = String(cfg?.appId || '').trim();
  const topAppSecret = String(cfg?.appSecret || '').trim();
  if (!topAppId || !topAppSecret) return '';
  const accounts = isPlainObject(cfg?.accounts) ? cfg.accounts : {};
  let matchedAccount = '';
  for (const accountId of listFeishuRawAccountIDs(cfg)) {
    const entry = isPlainObject(accounts[accountId]) ? accounts[accountId] : {};
    if (String(entry.appId || '').trim() !== topAppId) continue;
    if (String(entry.appSecret || '').trim() !== topAppSecret) continue;
    if (matchedAccount) return '';
    matchedAccount = accountId;
  }
  return matchedAccount;
}

function findFeishuEnabledDefaultAccount(cfg: any): string {
  let matchedAccount = '';
  for (const accountId of listFeishuRunnableAccountIDs(cfg)) {
    const entry = getFeishuAccountEntry(cfg, accountId);
    const parsedEnabled = parseFeishuEnabledValue(entry.enabled);
    const enabled = typeof parsedEnabled === 'boolean' ? parsedEnabled : hasFeishuRunnableCredentials(entry);
    if (!enabled) continue;
    if (matchedAccount) return '';
    matchedAccount = accountId;
  }
  return matchedAccount;
}

function pickFeishuDefaultAccount(cfg: any): string {
  const explicit = String(cfg?.defaultAccount || '').trim();
  const ids = listFeishuRawAccountIDs(cfg);
  const runnableIds = listFeishuRunnableAccountIDs(cfg);
  if (explicit) {
    const hasTopLevelSeed = !!String(cfg?.appId || '').trim() || !!String(cfg?.appSecret || '').trim();
    if (ids.includes(explicit)) {
      if (runnableIds.length === 0 || hasFeishuRunnableCredentials(getFeishuAccountEntry(cfg, explicit))) {
        return explicit;
      }
    } else if (runnableIds.length === 0 && hasTopLevelSeed) {
      return explicit;
    }
  }
  const mirrored = findFeishuMirroredDefaultAccount(cfg);
  if (mirrored) return mirrored;
  const enabled = findFeishuEnabledDefaultAccount(cfg);
  if (enabled) return enabled;
  if (runnableIds.includes('default')) return 'default';
  if (runnableIds.length > 0) return runnableIds[0];
  if (ids.includes('default')) return 'default';
  return ids[0] || 'default';
}

function getFeishuAccountEntry(cfg: any, accountId: string): Record<string, any> {
  return isPlainObject(cfg?.accounts?.[accountId]) ? cfg.accounts[accountId] : {};
}

function hasFeishuRunnableCredentials(entry: Record<string, any>): boolean {
  return !!String(entry.appId || '').trim() && !!String(entry.appSecret || '').trim();
}

function listFeishuRunnableAccountIDs(cfg: any): string[] {
  return listFeishuRawAccountIDs(cfg).filter(accountId => hasFeishuRunnableCredentials(getFeishuAccountEntry(cfg, accountId)));
}

function parseFeishuEnabledValue(value: any): boolean | undefined {
  if (typeof value === 'boolean') return value;
  if (typeof value === 'number') {
    if (value === 1) return true;
    if (value === 0) return false;
    return undefined;
  }
  if (typeof value !== 'string') return undefined;
  const normalized = value.trim().toLowerCase();
  if (['1', 't', 'true'].includes(normalized)) return true;
  if (['0', 'f', 'false'].includes(normalized)) return false;
  return undefined;
}

function isFeishuAccountEnabled(cfg: any, accountId: string): boolean {
  const defaultAccount = pickFeishuDefaultAccount(cfg);
  if (!accountId) return false;
  const entry = getFeishuAccountEntry(cfg, accountId);
  if (accountId === defaultAccount) return hasFeishuRunnableCredentials(entry);
  const parsedEnabled = parseFeishuEnabledValue(entry.enabled);
  if (typeof parsedEnabled === 'boolean') return parsedEnabled;
  return hasFeishuRunnableCredentials(entry);
}

function listFeishuAccountIDs(cfg: any): string[] {
  const ids = listFeishuRawAccountIDs(cfg);
  const defaultAccount = pickFeishuDefaultAccount(cfg);
  return ids.sort((a, b) => {
    if (a === defaultAccount) return -1;
    if (b === defaultAccount) return 1;
    const aEnabled = isFeishuAccountEnabled(cfg, a);
    const bEnabled = isFeishuAccountEnabled(cfg, b);
    if (aEnabled !== bEnabled) return aEnabled ? -1 : 1;
    return a.localeCompare(b);
  });
}

function hasFeishuAdvancedAccounts(cfg: any): boolean {
  return listFeishuAccountIDs(cfg).length > 0;
}

function countEnabledFeishuAccounts(cfg: any): number {
  return listFeishuAccountIDs(cfg).filter(accountId => isFeishuAccountEnabled(cfg, accountId)).length;
}

function ensureFeishuAccountEntry(draft: any, accountId: string): Record<string, any> {
  if (!isPlainObject(draft.accounts)) draft.accounts = {};
  if (!isPlainObject(draft.accounts[accountId])) draft.accounts[accountId] = {};
  return draft.accounts[accountId];
}

function syncFeishuTopLevelMirror(draft: any) {
  const defaultAccount = pickFeishuDefaultAccount(draft);
  if (!defaultAccount) {
    delete draft.appId;
    delete draft.appSecret;
    delete draft.defaultAccount;
    return;
  }
  draft.defaultAccount = defaultAccount;
  const entry = ensureFeishuAccountEntry(draft, defaultAccount);
  entry.enabled = true;
  const appId = String(entry.appId || '').trim();
  const appSecret = String(entry.appSecret || '').trim();
  if (appId) draft.appId = appId;
  else delete draft.appId;
  if (appSecret) draft.appSecret = appSecret;
  else delete draft.appSecret;
}

function applyFeishuDefaultOnlyMode(draft: any) {
  const defaultAccount = pickFeishuDefaultAccount(draft);
  if (!defaultAccount) return;
  const defaultEntry = ensureFeishuAccountEntry(draft, defaultAccount);
  if (!defaultEntry.appId && draft.appId) defaultEntry.appId = draft.appId;
  if (!defaultEntry.appSecret && draft.appSecret) defaultEntry.appSecret = draft.appSecret;
  for (const accountId of listFeishuAccountIDs(draft)) {
    const entry = ensureFeishuAccountEntry(draft, accountId);
    entry.enabled = accountId === defaultAccount;
  }
  syncFeishuTopLevelMirror(draft);
}

function applyFeishuMultiAccountMode(draft: any) {
  const defaultAccount = pickFeishuDefaultAccount(draft);
  if (!defaultAccount) return;
  const defaultEntry = ensureFeishuAccountEntry(draft, defaultAccount);
  if (!defaultEntry.appId && draft.appId) defaultEntry.appId = draft.appId;
  if (!defaultEntry.appSecret && draft.appSecret) defaultEntry.appSecret = draft.appSecret;
  defaultEntry.enabled = true;
  syncFeishuTopLevelMirror(draft);
}

function resolveFeishuSimpleCredentials(cfg: any): { appId: string; appSecret: string } {
  const appId = String(cfg?.appId || '').trim();
  const appSecret = String(cfg?.appSecret || '').trim();
  if (appId || appSecret) return { appId, appSecret };
  const defaultAccount = pickFeishuDefaultAccount(cfg);
  const entry = getFeishuAccountEntry(cfg, defaultAccount);
  return {
    appId: String(entry.appId || '').trim(),
    appSecret: String(entry.appSecret || '').trim(),
  };
}

function formatCommaList(value: any): string {
  if (Array.isArray(value)) {
    return value.map(item => String(item || '').trim()).filter(Boolean).join(', ');
  }
  return String(value || '').trim();
}

function parseDelimitedList(value: string): string[] {
  return value
    .split(/[\n,，]+/)
    .map(item => item.trim())
    .filter(Boolean)
    .filter((item, index, arr) => arr.indexOf(item) === index);
}

function formatChannelFieldDefaultValue(value: string | number | boolean | undefined): string {
  if (value === undefined) return '';
  if (value === true) return '开启';
  if (value === false) return '关闭';
  return String(value);
}

const FEISHU_FIELD_SECTIONS: Record<Exclude<ChannelFieldSection, 'default'>, { title: string; description: string }> = {
  access: {
    title: '接入与准入策略',
    description: '优先确定域名、群聊/私聊准入和 @ 触发规则，再决定是否启用群白名单。',
  },
  conversation: {
    title: '对话与输出体验',
    description: '控制回复形态、话题拆分、页脚信息与名称解析等会话体验。',
  },
  advanced: {
    title: '高级兼容参数',
    description: '补齐高频兼容字段；更深层能力继续通过 Raw JSON 或插件原生配置扩展。',
  },
};

const QQBOT_PRIMARY_FIELDS = new Set(['appId', 'clientSecret', 'appSecret']);
const CHANNEL_UNSELECTED_ID = '__none__';
const QQBOT_ADVANCED_FIELD_HELP: Record<string, string> = {
  allowFrom: '可选。限制允许触发机器人的用户 ID，支持英文逗号、中文逗号或换行分隔。',
  clientSecretFile: '可选。App Secret 文件路径，通常仅在密钥文件托管场景使用。',
  defaultAccount: '可选。默认账号标识，多账号路由时用于兜底匹配。',
  enabled: '开关当前 QQ 官方机器人通道。',
  name: '可选。面板中的显示名称，不影响 QQ 开放平台实际配置。',
  systemPrompt: '可选。为该通道覆盖默认系统提示词。',
  upgradeMode: '可选。升级/兼容模式，通常保持默认。',
  upgradeUrl: '可选。升级资源地址，通常无需填写。',
  voiceDirectUploadFormats: '可选。语音直传格式白名单，支持英文逗号、中文逗号或换行分隔。',
};

function humanizeChannelFieldKey(raw: string): string {
  const leaf = raw.split('.').pop() || raw;
  return leaf
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .replace(/[_-]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/\b\w/g, ch => ch.toUpperCase());
}

function inferChannelFieldType(key: string, schema: Record<string, any>, uiHint: Record<string, any>): ChannelConfigField['type'] | '' {
  const type = schema?.type;
  if (type === 'boolean') return 'toggle';
  if (type === 'integer' || type === 'number') return 'number';
  if (Array.isArray(schema?.enum) && schema.enum.length > 0) return 'select';
  if (type === 'array' && schema?.items?.type === 'string') return 'textarea';
  if (type !== 'string') return '';
  if (uiHint?.sensitive) return 'password';
  if (/(token|secret|password|key)$/i.test(key)) return 'password';
  if (/(prompt|instructions)$/i.test(key)) return 'textarea';
  return 'text';
}

function buildDynamicChannelFields(
  schema: Record<string, any> | undefined,
  uiHints: Record<string, any> | undefined,
  prefix = '',
  depth = 0,
): ChannelConfigField[] {
  const properties = isPlainObject(schema?.properties) ? schema.properties : {};
  const hints = isPlainObject(uiHints) ? uiHints : {};
  const fields: ChannelConfigField[] = [];
  for (const [propKey, rawPropSchema] of Object.entries(properties)) {
    if (!isPlainObject(rawPropSchema)) continue;
    if (!prefix && propKey === 'enabled') continue;
    const fieldKey = prefix ? `${prefix}.${propKey}` : propKey;
    const uiHint = isPlainObject(hints[fieldKey]) ? hints[fieldKey] : {};
    const propType = String(rawPropSchema.type || '').trim();
    if (propType === 'object') {
      if (depth >= 1) continue;
      if (!isPlainObject(rawPropSchema.properties) || Object.keys(rawPropSchema.properties).length === 0) continue;
      fields.push(...buildDynamicChannelFields(rawPropSchema, uiHints, fieldKey, depth + 1));
      continue;
    }
    const fieldType = inferChannelFieldType(fieldKey, rawPropSchema, uiHint);
    if (!fieldType) continue;
    const field: ChannelConfigField = {
      key: fieldKey,
      label: String(uiHint.label || humanizeChannelFieldKey(fieldKey)),
      type: fieldType,
      section: uiHint.advanced ? 'advanced' : 'default',
    };
    if (Array.isArray(rawPropSchema.enum) && rawPropSchema.enum.length > 0) {
      field.options = rawPropSchema.enum.map((item: any) => String(item));
    }
    if (uiHint.placeholder) field.placeholder = String(uiHint.placeholder);
    if (uiHint.help) field.help = String(uiHint.help);
    if (rawPropSchema.default !== undefined) field.defaultValue = rawPropSchema.default;
    if (fieldType === 'textarea') {
      field.rows = /prompt|instructions/i.test(fieldKey) ? 4 : 3;
      if (propType === 'array' && rawPropSchema?.items?.type === 'string') {
        field.valueFormat = 'stringArray';
        if (!field.help) field.help = '支持英文逗号、中文逗号或换行分隔';
      }
    }
    fields.push(field);
  }
  return fields;
}

function mergeChannelDefs(defaultDefs: ChannelDef[], catalogItems: ChannelCatalogItem[]): ChannelDef[] {
  const merged = new Map<string, ChannelDef>();
  defaultDefs.forEach(def => {
    merged.set(def.id, {
      ...def,
      configFields: def.configFields.map(field => ({ ...field })),
      loginMethods: def.loginMethods ? [...def.loginMethods] : undefined,
    });
  });
  catalogItems.forEach(item => {
    const dynamicFields = buildDynamicChannelFields(item.configSchema, item.uiHints);
    const existing = merged.get(item.id);
    if (existing) {
      const seen = new Set(existing.configFields.map(field => field.key));
      dynamicFields.forEach(field => {
        const nextField = { ...field };
        if (item.id === 'qqbot' && !QQBOT_PRIMARY_FIELDS.has(nextField.key)) {
          nextField.section = 'advanced';
          if (!nextField.help && QQBOT_ADVANCED_FIELD_HELP[nextField.key]) {
            nextField.help = QQBOT_ADVANCED_FIELD_HELP[nextField.key];
          }
        }
        if (!seen.has(nextField.key)) existing.configFields.push(nextField);
      });
      existing.label = existing.label || item.label || item.id;
      existing.description = existing.description || item.description || '';
      existing.type = existing.type || item.type;
      return;
    }
    merged.set(item.id, {
      id: item.id,
      label: item.label || item.id,
      description: item.description || '',
      type: item.type,
      configFields: dynamicFields,
      loginMethods: item.id === 'openclaw-weixin' ? ['qrcode'] : undefined,
    });
  });
  return Array.from(merged.values());
}

const DEFAULT_CHANNEL_DEFS: ChannelDef[] = [
  { id: 'qq', label: 'QQ (NapCat)', description: 'QQ个人号，NapCat OneBot11协议', type: 'plugin',
    loginMethods: ['qrcode', 'quick', 'password'],
    configFields: [
      { key: 'wsUrl', label: 'WebSocket 地址', type: 'text', placeholder: 'ws://127.0.0.1:3001' },
      { key: 'accessToken', label: 'Access Token', type: 'password' },
      { key: 'ownerQQ', label: '主人QQ号', type: 'number', help: '接收通知的QQ号' },
      { key: 'rateLimit.wakeProbability', label: '唤醒概率 (%)', type: 'number', help: '群聊中Bot回复的概率，0-100' },
      { key: 'rateLimit.minIntervalSec', label: '最小发送间隔 (秒)', type: 'number' },
      { key: 'rateLimit.wakeTrigger.keywords', label: '唤醒触发词', type: 'text', help: '逗号分隔关键词' },
      { key: 'pokeReplyText', label: '戳一戳回复内容', type: 'text', placeholder: '别戳我啦~', help: '自定义戳一戳回复文本' },
      { key: 'autoApprove.group.enabled', label: '自动同意加群', type: 'toggle' },
      { key: 'autoApprove.friend.enabled', label: '自动同意好友', type: 'toggle' },
      { key: 'notifications.antiRecall', label: '防撤回通知', type: 'toggle', help: '撤回消息时发送通知' },
      { key: 'notifications.memberChange', label: '成员变动通知', type: 'toggle', help: '群成员加入/退出通知' },
      { key: 'notifications.adminChange', label: '管理员变动通知', type: 'toggle', help: '管理员设置/取消通知' },
      { key: 'notifications.banNotice', label: '禁言通知', type: 'toggle', help: '禁言/解禁通知' },
      { key: 'notifications.pokeReply', label: '戳一戳回复', type: 'toggle', help: '收到戳一戳时自动回复' },
      { key: 'notifications.honorNotice', label: '荣誉通知', type: 'toggle', help: '群荣誉变动通知' },
      { key: 'notifications.fileUpload', label: '文件上传通知', type: 'toggle', help: '群文件上传通知' },
      { key: 'welcome.enabled', label: '入群欢迎', type: 'toggle', help: '新成员入群时发送欢迎消息' },
      { key: 'welcome.template', label: '欢迎模板', type: 'text', placeholder: '欢迎 {nickname} 加入本群！' },
    ] },
  { id: 'whatsapp', label: 'WhatsApp', description: 'Baileys QR扫码配对', type: 'builtin', loginMethods: ['qrcode'],
    configFields: [
      { key: 'dmPolicy', label: 'DM策略', type: 'select', options: ['pairing','open','allowlist'] },
      { key: 'reactionLevel', label: 'Reaction Level', type: 'select', options: ['off', 'ack', 'minimal', 'extensive'], help: '控制 WhatsApp 预确认和 agent 主动 reaction 的等级。' },
      { key: 'ackReaction.emoji', label: 'Ack Reaction Emoji', type: 'text', placeholder: '👀', help: '收到消息后立刻发送的预确认 emoji。' },
      { key: 'ackReaction.direct', label: '私聊 Ack Reaction', type: 'toggle', help: '是否在私聊中发送 ack reaction。' },
      { key: 'ackReaction.group', label: '群聊 Ack Reaction', type: 'select', options: ['always', 'mentions', 'never'], help: '群聊里何时发送 ack reaction。' },
    ] },
  { id: 'telegram', label: 'Telegram', description: 'Telegram Bot 内置通道', type: 'builtin',
    configFields: [
      { key: 'botToken', label: 'Bot Token', type: 'password', placeholder: '123456:ABC-DEF...', help: '从 @BotFather 获取的 Telegram Bot Token' },
      { key: 'dmPolicy', label: '私聊准入策略', type: 'select', options: ['pairing', 'open', 'allowlist'], help: 'pairing = 首次私聊需配对批准；open = 所有私聊直接可用；allowlist = 仅白名单', defaultValue: 'open' },
      { key: 'allowFrom', label: '私聊白名单', type: 'textarea', rows: 3, placeholder: '123456789, 987654321', help: '仅 dmPolicy=allowlist 时生效；支持英文逗号、中文逗号或换行分隔' },
      { key: 'groupPolicy', label: '群聊准入策略', type: 'select', options: ['allowlist', 'open'], help: 'allowlist = 仅允许白名单中的发送者；open = 群内任何人都可触发（通常仍需提及）', defaultValue: 'allowlist' },
      { key: 'groupAllowFrom', label: '群聊白名单', type: 'textarea', rows: 3, placeholder: '123456789, 987654321', help: '仅 groupPolicy=allowlist 时生效；支持英文逗号、中文逗号或换行分隔' },
      { key: 'reactionNotifications', label: 'Reaction Notifications', type: 'select', options: ['off', 'own', 'all'], help: '控制哪些 Telegram reaction 会转成系统事件。' },
      { key: 'reactionLevel', label: 'Reaction Level', type: 'select', options: ['off', 'ack', 'minimal', 'extensive'], help: '控制 agent 在 Telegram 里使用 reaction 的范围。' },
      { key: 'ackReaction', label: 'Ack Reaction Emoji', type: 'text', placeholder: '👀', help: '处理消息时先发送的确认 emoji；留空可禁用。' },
    ] },
  { id: 'discord', label: 'Discord', description: 'Discord Bot API + Gateway', type: 'builtin',
    configFields: [
      { key: 'token', label: 'Bot Token', type: 'password' },
      { key: 'applicationId', label: 'Application ID', type: 'text' },
      { key: 'guildIds', label: 'Guild IDs', type: 'text', help: '逗号分隔' },
    ] },
  { id: 'irc', label: 'IRC', description: '经典IRC服务器', type: 'builtin',
    configFields: [
      { key: 'server', label: '服务器', type: 'text', placeholder: 'irc.libera.chat' },
      { key: 'nick', label: '昵称', type: 'text' },
      { key: 'channels', label: '频道', type: 'text', help: '逗号分隔' },
    ] },
  { id: 'slack', label: 'Slack', description: 'Bolt SDK，工作区应用', type: 'builtin',
    configFields: [
      { key: 'appToken', label: 'App Token', type: 'password', placeholder: 'xapp-...' },
      { key: 'botToken', label: 'Bot Token', type: 'password', placeholder: 'xoxb-...' },
    ] },
  { id: 'signal', label: 'Signal', description: 'signal-cli REST API', type: 'builtin',
    configFields: [
      { key: 'apiUrl', label: 'REST API URL', type: 'text', placeholder: 'http://signal-cli:8080' },
      { key: 'phoneNumber', label: '手机号', type: 'text' },
      { key: 'reactionNotifications', label: 'Reaction Notifications', type: 'select', options: ['off', 'own', 'all', 'allowlist'], help: '控制哪些 Signal reaction 会变成系统事件。' },
      { key: 'reactionLevel', label: 'Reaction Level', type: 'select', options: ['off', 'ack', 'minimal', 'extensive'], help: '控制 Signal 的 ack / agent reaction 能力。' },
    ] },
  { id: 'feishu', label: '飞书 / Lark', description: 'OpenClaw 内置飞书通道（2026.4.x 起）', type: 'builtin',
    configFields: [
      { key: 'appId', label: 'App ID', type: 'text', help: '飞书 / Lark 应用 App ID', section: 'access' },
      { key: 'appSecret', label: 'App Secret', type: 'password', help: '飞书 / Lark 应用 App Secret', section: 'access' },
      { key: 'domain', label: '站点域（Domain）', type: 'select', options: ['feishu', 'lark'], help: '国际版 Lark 场景可切到 lark；不确定时保持 feishu', defaultValue: 'feishu', section: 'access' },
      { key: 'requireMention', label: '群聊是否必须 @', type: 'select', options: ['true', 'false'], help: '仅接受 true / false；open 属于 groupPolicy，不属于本字段', defaultValue: true, section: 'access' },
      { key: 'groupPolicy', label: '群组准入策略', type: 'select', options: ['open', 'allowlist', 'closed'], help: 'open = 所有群可用；allowlist = 仅白名单；closed = 禁止群聊', defaultValue: 'open', section: 'access' },
      { key: 'dmPolicy', label: '私聊准入策略', type: 'select', options: ['pairing', 'open', 'allowlist'], help: 'pairing = 需先配对；open = 所有私聊可用；allowlist = 仅白名单', defaultValue: 'open', section: 'access' },
      { key: 'groupAllowFrom', label: '群聊白名单', type: 'textarea', placeholder: 'oc_xxx, oc_yyy', help: '支持英文逗号、中文逗号或换行分隔；仅 groupPolicy=allowlist 时生效，保存时会写成数组', section: 'access', rows: 3 },
      { key: 'streaming', label: '流式卡片输出', type: 'toggle', help: '开启后回复以流式卡片形式呈现', section: 'conversation' },
      { key: 'threadSession', label: '话题独立上下文', type: 'toggle', help: '每个话题拥有独立会话并可并行', section: 'conversation' },
      { key: 'footer.elapsed', label: '显示耗时页脚', type: 'toggle', section: 'conversation' },
      { key: 'footer.status', label: '显示状态页脚', type: 'toggle', section: 'conversation' },
      { key: 'replyInThread', label: '话题内回复', type: 'toggle', help: '优先在话题内回复', section: 'conversation' },
      { key: 'typingIndicator', label: '输入中提示', type: 'toggle', section: 'conversation' },
      { key: 'resolveSenderNames', label: '解析发送者名称', type: 'toggle', section: 'conversation' },
      { key: 'dynamicAgentCreation', label: '动态创建 Agent', type: 'toggle', section: 'conversation' },
      { key: 'connectionMode', label: '连接模式', type: 'text', placeholder: 'websocket', defaultValue: 'websocket', section: 'advanced' },
      { key: 'historyLimit', label: '历史消息回放上限', type: 'number', placeholder: '300', defaultValue: 300, section: 'advanced' },
      { key: 'mediaMaxMb', label: '媒体大小上限（MB）', type: 'number', placeholder: '5', defaultValue: 5, section: 'advanced' },
    ] },
  { id: 'googlechat', label: 'Google Chat', description: 'Google Chat API Webhook', type: 'builtin',
    configFields: [
      { key: 'serviceAccountKey', label: '服务账号 JSON', type: 'text' },
      { key: 'webhookUrl', label: 'Webhook URL', type: 'text' },
    ] },
  { id: 'bluebubbles', label: 'BlueBubbles (iMessage)', description: 'macOS iMessage', type: 'builtin',
    configFields: [
      { key: 'serverUrl', label: '服务器URL', type: 'text' },
      { key: 'password', label: '密码', type: 'password' },
    ] },
  { id: 'imessage', label: 'iMessage', description: 'OpenClaw 内置 iMessage 通道（通常依赖 macOS 主机环境）', type: 'builtin', configFields: [] },
  { id: 'line', label: 'LINE', description: 'LINE Messaging API（内置）', type: 'builtin',
    configFields: [
      { key: 'channelAccessToken', label: 'Channel Access Token', type: 'password' },
      { key: 'channelSecret', label: 'Channel Secret', type: 'password' },
    ] },
  { id: 'matrix', label: 'Matrix', description: 'Matrix 协议（内置）', type: 'builtin',
    configFields: [
      { key: 'homeserverUrl', label: 'Homeserver URL', type: 'text' },
      { key: 'accessToken', label: 'Access Token', type: 'password' },
    ] },
  { id: 'mattermost', label: 'Mattermost', description: 'Mattermost Bot API + WebSocket（内置）', type: 'builtin',
    configFields: [
      { key: 'url', label: '服务器URL', type: 'text' },
      { key: 'token', label: 'Bot Token', type: 'password' },
    ] },
  { id: 'msteams', label: 'Microsoft Teams', description: 'Bot Framework（内置）', type: 'builtin',
    configFields: [
      { key: 'appId', label: 'App ID', type: 'text' },
      { key: 'appPassword', label: 'App Password', type: 'password' },
    ] },
  { id: 'nextcloud-talk', label: 'Nextcloud Talk', description: 'OpenClaw 内置 Nextcloud Talk 通道', type: 'builtin', configFields: [] },
  { id: 'nostr', label: 'Nostr', description: 'OpenClaw 内置 Nostr 通道', type: 'builtin', configFields: [] },
  { id: 'qa-channel', label: 'QA Channel', description: 'OpenClaw 内置 QA 调试通道', type: 'builtin', configFields: [] },
  { id: 'qqbot', label: 'QQ 官方机器人', description: 'QQ 开放平台官方 Bot API（内置）', type: 'builtin',
    configFields: [
      { key: 'appId', label: 'App ID', type: 'text', help: 'QQ 开放平台应用 ID（新版内置通道只需填写 appId 和 appSecret）' },
      { key: 'clientSecret', label: 'App Secret', type: 'password', help: 'QQ 官方机器人 App Secret' },
    ] },
  { id: 'synology-chat', label: 'Synology Chat', description: 'OpenClaw 内置 Synology Chat 通道', type: 'builtin', configFields: [] },
  { id: 'tlon', label: 'Tlon', description: 'OpenClaw 内置 Tlon 通道', type: 'builtin', configFields: [] },
  { id: 'twitch', label: 'Twitch', description: 'Twitch Chat via IRC（内置）', type: 'builtin',
    configFields: [
      { key: 'username', label: '用户名', type: 'text' },
      { key: 'oauthToken', label: 'OAuth Token', type: 'password' },
      { key: 'channels', label: '频道', type: 'text', help: '逗号分隔' },
    ] },
  { id: 'zalo', label: 'Zalo', description: 'OpenClaw 内置 Zalo 通道', type: 'builtin', configFields: [] },
  { id: 'webchat', label: 'WebChat', description: 'Gateway WebChat UI (内置)', type: 'builtin', configFields: [] },
  // Plugin channels
  { id: 'dingtalk', label: '钉钉', description: '钉钉机器人 (插件)', type: 'plugin',
    configFields: [
      { key: 'clientId', label: 'Client ID', type: 'text', help: '钉钉应用 Client ID' },
      { key: 'clientSecret', label: 'Client Secret', type: 'password', help: '钉钉应用 Client Secret' },
    ] },
  { id: 'wecom', label: '企业微信（智能机器人）', description: '企业微信智能机器人，支持 URL 回调与长连接两种接入方式', type: 'plugin',
    configFields: [
      { key: 'connectionMode', label: '连接方式', type: 'select', options: ['callback', 'long-polling'], help: 'callback = URL 回调；long-polling = 长连接', defaultValue: 'callback' },
      { key: 'token', label: 'Token', type: 'password', help: '企业微信回调配置中的 Token' },
      { key: 'encodingAESKey', label: 'EncodingAESKey', type: 'password', help: '43 位字符' },
      { key: 'webhookPath', label: 'Webhook Path', type: 'text', help: '如 /wecom' },
      { key: 'botId', label: 'Bot ID', type: 'text', help: '长连接模式下使用的 Bot ID' },
      { key: 'secret', label: 'Secret', type: 'password', help: '长连接模式下使用的 Secret' },
    ] },
  { id: 'wecom-app', label: '企业微信（自建应用）', description: '企业微信自建应用，支持更完整 API 与微信入口', type: 'plugin',
    configFields: [
      { key: 'token', label: 'Token', type: 'password', help: '企业微信回调配置中的 Token' },
      { key: 'encodingAESKey', label: 'EncodingAESKey', type: 'password', help: '43 位字符' },
      { key: 'corpId', label: 'Corp ID', type: 'text', help: '企业 ID' },
      { key: 'corpSecret', label: 'Corp Secret', type: 'password', help: '应用 Secret' },
      { key: 'agentId', label: 'Agent ID', type: 'text', help: '应用 Agent ID' },
    ] },
  { id: 'openclaw-weixin', label: '微信（ClawBot）', description: '腾讯官方 WeChat ClawBot 插件，当前重点支持微信私聊', type: 'plugin', loginMethods: ['qrcode'],
    configFields: [
      { key: 'name', label: '账号显示名', type: 'text', help: '可选。作为当前默认账号的显示名称；真正登录凭证由扫码写入本地。' },
      { key: 'baseUrl', label: 'Base URL', type: 'text', placeholder: 'https://ilinkai.weixin.qq.com', help: '默认可保持官方地址，仅在腾讯侧给出其他入口时调整。' },
      { key: 'cdnBaseUrl', label: 'CDN Base URL', type: 'text', placeholder: 'https://novac2c.cdn.weixin.qq.com/c2c', help: '媒体上传/下载 CDN 地址；通常保持默认即可。' },
      { key: 'routeTag', label: 'Route Tag', type: 'number', help: '可选。多入口或灰度路由场景下才需要。' },
    ] },
];

const CHANNEL_REQUIRED_FIELDS: Record<string, string[]> = {
  telegram: ['botToken'],
  discord: ['token', 'applicationId'],
  irc: ['server', 'nick', 'channels'],
  slack: ['appToken', 'botToken'],
  signal: ['apiUrl', 'phoneNumber'],
  googlechat: ['serviceAccountKey', 'webhookUrl'],
  bluebubbles: ['serverUrl', 'password'],
  feishu: ['appId', 'appSecret'],
  qqbot: ['appId', 'clientSecret'],
  dingtalk: ['clientId', 'clientSecret'],
  'wecom': ['token', 'encodingAESKey', 'webhookPath'],
  'wecom-app': ['token', 'encodingAESKey', 'corpId', 'corpSecret', 'agentId'],
  msteams: ['appId', 'appPassword'],
  mattermost: ['url', 'token'],
  line: ['channelAccessToken', 'channelSecret'],
  matrix: ['homeserverUrl', 'accessToken'],
  twitch: ['username', 'oauthToken', 'channels'],
};
// 飞书官方版当前仅认可 openclaw-lark，旧 ID 仅做历史兼容清理。
const FEISHU_OFFICIAL_IDS = ['openclaw-lark'] as const;
// 所有当前有效的飞书插件 ID（含社区版）。
const FEISHU_ALL_IDS = ['openclaw-lark', 'feishu'] as const;
const WECOM_BOT_PLUGIN_IDS = ['wecom-openclaw-plugin', 'wecom'] as const;
const QQBOT_PLUGIN_IDS = ['qqbot', 'qqbot-community'] as const;
type QQBotVariant = 'builtin' | 'community';

function getEnabledPluginEntry(entries: Record<string, any>, ids: readonly string[]): string | null {
  for (const id of ids) {
    if (entries[id]?.enabled) return id;
  }
  return null;
}

function isQQBotPluginInstalled(installedPlugins: any[]) {
  return installedPlugins.some((p: any) => (QQBOT_PLUGIN_IDS as readonly string[]).includes(p.id));
}

function getActiveQQBotVariant(ocConfig: any): QQBotVariant {
  const entries = ocConfig?.plugins?.entries || {};
  if (entries['qqbot-community']?.enabled) return 'community';
  if (entries['qqbot']?.enabled) return 'builtin';
  if (entries['qqbot-community']) return 'community';
  return 'builtin';
}

function getWecomConnectionMode(cfg: any): 'callback' | 'long-polling' {
	const mode = String(cfg?.connectionMode || '').trim().toLowerCase();
	if (mode === 'long-polling' || mode === 'long_polling' || mode === 'longpolling') return 'long-polling';
	if (mode === 'callback') return 'callback';
	if (String(cfg?.botId || '').trim() && String(cfg?.secret || '').trim()) return 'long-polling';
	return 'callback';
}

// 飞书版本：读取当前启用的变体
function getActiveFeishuVariant(ocConfig: any): 'official' | 'clawteam' | null {
  if (isPlainObject(ocConfig?.channels?.feishu)) return 'official';
  const entries = ocConfig?.plugins?.entries || {};
  if (getEnabledPluginEntry(entries, FEISHU_OFFICIAL_IDS)) return 'official';
  if (entries['feishu']?.enabled) return 'official';
  return null;
}

// 飞书版本：获取当前活跃的 plugin entry ID
function getFeishuPluginEntryId(ocConfig: any): string {
  const entries = ocConfig?.plugins?.entries || {};
  for (const id of FEISHU_ALL_IDS) {
    if (entries[id]?.enabled) return id;
  }
  // 未启用时返回有 entry 的第一个
  for (const id of FEISHU_ALL_IDS) {
    if (entries[id]) return id;
  }
  return 'feishu';
}

function isQQPluginInstalled(installedPlugins: any[]) {
  return installedPlugins.some((p: any) => p.id === 'qq');
}

function isQQActuallyInstalled(installedPlugins: any[], qqChannelState: any) {
  return !!(qqChannelState?.pluginInstalled || isQQPluginInstalled(installedPlugins));
}

function getWecomAppVirtualConfig(ocConfig: any): Record<string, any> {
  const channels = isPlainObject(ocConfig?.channels) ? ocConfig.channels : {};
  const wecom = isPlainObject(channels.wecom) ? channels.wecom : {};
  const agent = isPlainObject(wecom.agent) ? { ...wecom.agent } : {};
  if (agent.token === undefined && wecom.token !== undefined) {
    agent.token = wecom.token;
  }
  if (agent.encodingAESKey === undefined) {
    if (wecom.encodingAESKey !== undefined) agent.encodingAESKey = wecom.encodingAESKey;
    else if (wecom.encodingAesKey !== undefined) agent.encodingAESKey = wecom.encodingAesKey;
  }
  if (agent.encodingAESKey === undefined && agent.encodingAesKey !== undefined) {
    agent.encodingAESKey = agent.encodingAesKey;
  }
  const virtual = isPlainObject(channels['wecom-app']) ? channels['wecom-app'] : {};
  if (virtual.enabled !== undefined) agent.enabled = virtual.enabled;
  else if (agent.enabled === undefined) agent.enabled = false;
  return agent;
}

function isWecomAppEnabled(ocConfig: any): boolean {
  return !!getWecomAppVirtualConfig(ocConfig).enabled;
}

function isWecomBuiltinEnabled(ocConfig: any): boolean {
  const entries = ocConfig?.plugins?.entries || {};
  const wecomEnabled = !!ocConfig?.channels?.wecom?.enabled || !!getEnabledPluginEntry(entries, WECOM_BOT_PLUGIN_IDS);
  return wecomEnabled && !isWecomAppEnabled(ocConfig);
}
// Determine channel status: 'enabled' (green), 'configured' (red/orange), 'unconfigured' (gray)
function getChannelStatus(
  ch: ChannelDef,
  ocConfig: any,
  installedPlugins: any[],
  qqChannelState: any,
): 'enabled' | 'configured' | 'unconfigured' {
  // wecom-app is backed by channels.wecom.agent
  const chConf = ch.id === 'wecom-app'
    ? getWecomAppVirtualConfig(ocConfig)
    : (ocConfig?.channels?.[ch.id] || {});
  const pluginConf = ocConfig?.plugins?.entries?.[ch.id] || {};
  const pluginInstalled = ch.id === 'qq'
    ? isQQActuallyInstalled(installedPlugins, qqChannelState)
    : ch.id === 'qqbot'
      ? isQQBotPluginInstalled(installedPlugins)
    : ch.type === 'plugin'
      ? (
          ch.id === 'feishu'
            ? installedPlugins.some((p: any) => (FEISHU_ALL_IDS as readonly string[]).includes(p.id))
            : ch.id === 'wecom'
              ? installedPlugins.some((p: any) => (WECOM_BOT_PLUGIN_IDS as readonly string[]).includes(p.id))
              : ch.id === 'wecom-app'
                ? installedPlugins.some((p: any) => p.id === 'wecom' || p.id === 'wecom-app')
                : installedPlugins.some((p: any) => p.id === ch.id)
        )
      : true;
  // 飞书特殊处理：任一变体 enabled 即视为 enabled
  const configuredEnabled = ch.id === 'feishu'
    ? (pluginConf.enabled || !!getEnabledPluginEntry(ocConfig?.plugins?.entries || {}, FEISHU_OFFICIAL_IDS) || chConf.enabled)
    : ch.id === 'qqbot'
      ? (chConf.enabled || !!getEnabledPluginEntry(ocConfig?.plugins?.entries || {}, QQBOT_PLUGIN_IDS))
    : ch.id === 'wecom-app'
      ? isWecomAppEnabled(ocConfig)
      : ch.id === 'wecom'
        ? isWecomBuiltinEnabled(ocConfig)
        : (chConf.enabled || pluginConf.enabled);
  const isEnabled = configuredEnabled && pluginInstalled;
  // Check if any config field has a value
  const hasConfig = ch.configFields.some(f => {
    const v = getNestedValue(chConf, f.key);
    return v !== undefined && v !== null && v !== '';
  }) || (
    ch.id === 'feishu' && (
      !!String(chConf?.appId || '').trim()
      || !!String(chConf?.appSecret || '').trim()
      || hasFeishuAdvancedAccounts(chConf)
    )
  );
  if (isEnabled) return 'enabled';
  if (hasConfig) return 'configured';
  return 'unconfigured';
}

function statusDot(s: 'enabled' | 'configured' | 'unconfigured') {
  if (s === 'enabled') return 'bg-emerald-500';
  if (s === 'configured') return 'bg-red-400';
  return 'bg-gray-300 dark:bg-gray-600';
}

// statusLabel is now inside the component to access i18n

export default function Channels() {
  const { t } = useI18n();
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const modern = uiMode === 'modern';
  const [searchParams, setSearchParams] = useSearchParams();

  const statusLabel = (s: 'enabled' | 'configured' | 'unconfigured') => {
    if (s === 'enabled') return t.channels.statusEnabled;
    if (s === 'configured') return t.channels.statusConfigured;
    return t.channels.statusUnconfigured;
  };

  const [status, setStatus] = useState<any>(null);
  const [channelDefs, setChannelDefs] = useState<ChannelDef[]>(DEFAULT_CHANNEL_DEFS);
  const [selectedChannel, setSelectedChannel] = useState(CHANNEL_UNSELECTED_ID);
  const [ocConfig, setOcConfig] = useState<any>({});
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState('');
  const [requests, setRequests] = useState<any[]>([]);
  // QQ Login state
  const [loginModal, setLoginModal] = useState<'qrcode' | 'quick' | 'password' | null>(null);
  const [loginChannelId, setLoginChannelId] = useState<string>('');
  const [qrImg, setQrImg] = useState('');
  const [qrRenderImg, setQrRenderImg] = useState('');
  const [qrLoading, setQrLoading] = useState(false);
  const [quickList, setQuickList] = useState<string[]>([]);
  const [loginUin, setLoginUin] = useState('');
  const [loginPwd, setLoginPwd] = useState('');
  const [loginLoading, setLoginLoading] = useState(false);
  const [loginMsg, setLoginMsg] = useState('');
  const [softwareList, setSoftwareList] = useState<any[]>([]);
  const [serverPlatform, setServerPlatform] = useState<string>('');
  const [installingSw, setInstallingSw] = useState<string | null>(null);
  const [napcatStatus, setNapcatStatus] = useState<any>(null);
  const [reconnectLogs, setReconnectLogs] = useState<any[]>([]);
  const [showReconnectLogs, setShowReconnectLogs] = useState(false);
  const [reconnecting, setReconnecting] = useState(false);
  const [diagnosing, setDiagnosing] = useState(false);
  const [diagnoseResult, setDiagnoseResult] = useState<any>(null);
  const [restarting, setRestarting] = useState(false);
  const [installedPlugins, setInstalledPlugins] = useState<any[]>([]);
  const [installingChannelPlugin, setInstallingChannelPlugin] = useState<string | null>(null);
  const [openClawWeixinStatus, setOpenClawWeixinStatus] = useState<any>(null);
  const [openClawWeixinSessionKey, setOpenClawWeixinSessionKey] = useState('');
  const [loggingOutWeixinAccount, setLoggingOutWeixinAccount] = useState<string | null>(null);
  const [qqChannelState, setQQChannelState] = useState<any>(null);
  const [pairingRequests, setPairingRequests] = useState<OpenClawPairingRequest[]>([]);
  const [loadingPairingRequests, setLoadingPairingRequests] = useState(false);
  const [approvingPairingCode, setApprovingPairingCode] = useState('');
  const [channelDrafts, setChannelDrafts] = useState<Record<string, any>>({});
  const [channelFieldTextDrafts, setChannelFieldTextDrafts] = useState<Record<string, string>>({});
  const [qqbotAdvancedOpen, setQqbotAdvancedOpen] = useState(false);
  const [switchingQQBotVariant, setSwitchingQQBotVariant] = useState(false);
  const [feishuAdvancedAccounts, setFeishuAdvancedAccounts] = useState(false);
  const [feishuActiveAccountId, setFeishuActiveAccountId] = useState('default');
  const [feishuNewAccountId, setFeishuNewAccountId] = useState('');
  const [feishuDmDiagnosis, setFeishuDmDiagnosis] = useState<FeishuDMDiagnosis | null>(null);
  const [loadingFeishuDmDiagnosis, setLoadingFeishuDmDiagnosis] = useState(false);
  const qrPollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const feishuAccountModeInitializedRef = useRef(false);
  const navigate = useNavigate();

  const normalizeChannelQuery = (value: string | null) => {
    if (!value) return '';
    const normalized = value.trim().toLowerCase();
    if (normalized === 'napcat') return 'qq';
    return channelDefs.some(channel => channel.id === normalized) ? normalized : '';
  };

  const syncFeishuUiState = useCallback((config: any) => {
    const feishuCfg = isPlainObject(config?.channels?.feishu) ? config.channels.feishu : {};
    const hasAccounts = hasFeishuAdvancedAccounts(feishuCfg);
    const defaultAccount = pickFeishuDefaultAccount(feishuCfg);
    const accountIDs = listFeishuAccountIDs(feishuCfg);
    const shouldShowMultiAccountEditor = accountIDs.length > 1 && countEnabledFeishuAccounts(feishuCfg) > 1;
    setFeishuAdvancedAccounts(prev => {
      if (!feishuAccountModeInitializedRef.current) {
        feishuAccountModeInitializedRef.current = true;
        return shouldShowMultiAccountEditor;
      }
      if (!hasAccounts) return false;
      return prev;
    });
    setFeishuActiveAccountId(accountIDs.includes(defaultAccount) ? defaultAccount : (defaultAccount || accountIDs[0] || 'default'));
    setFeishuNewAccountId('');
  }, []);

  const updateChannelDraft = useCallback((channelId: string, mutate: (draft: any) => void) => {
    setChannelDrafts((prev: Record<string, any>) => {
      const next = { ...prev };
      const base = isPlainObject(prev[channelId])
        ? prev[channelId]
        : (isPlainObject(ocConfig?.channels?.[channelId]) ? ocConfig.channels[channelId] : {});
      const current = deepClone(base);
      mutate(current);
      next[channelId] = current;
      return next;
    });
  }, [ocConfig]);

  const updateFeishuDraft = useCallback((mutate: (draft: any) => void) => {
    updateChannelDraft('feishu', mutate);
  }, [updateChannelDraft]);

  const loadNapcatStatus = () => {
    api.napcatStatus().then(r => { if (r.ok) setNapcatStatus(r.status); }).catch(() => {});
  };

  const loadReconnectLogs = () => {
    api.napcatReconnectLogs().then(r => { if (r.ok) setReconnectLogs(r.logs || []); }).catch(() => {});
  };

  const handleReconnect = async () => {
    if (!confirm('确定要触发 NapCat 重连？将重启容器。')) return;
    setReconnecting(true);
    try {
      const r = await api.napcatReconnect();
      if (r.ok) setMsg('重连请求已发送，请等待...');
      else setMsg(r.error || '重连失败');
    } catch (err) { setMsg('重连失败: ' + String(err)); }
    finally { setReconnecting(false); setTimeout(() => { setMsg(''); loadNapcatStatus(); }, 10000); }
  };

  const loadSoftware = () => {
    api.getSoftwareList().then(r => { if (r.ok) { setSoftwareList(r.software || []); if (r.platform) setServerPlatform(r.platform); } }).catch(() => {});
  };

  const handleInstallContainer = async (id: string) => {
    setInstallingSw(id);
    try {
      const r = await api.installSoftware(id);
      if (r.ok) setMsg(`✅ ${id} 安装任务已创建，请在消息中心查看进度`);
      else setMsg(`❌ ${r.error || '安装失败'}`);
    } catch { setMsg('❌ 安装请求失败'); }
    finally { setInstallingSw(null); setTimeout(() => { setMsg(''); loadSoftware(); loadInstalledPlugins(); loadQQChannelState(); reload(); }, 5000); }
  };

  const handleSetupQQChannel = async () => {
    setInstallingSw('napcat');
    try {
      const r = await api.setupQQChannel();
      if (r.ok) {
        setMsg('✅ QQ (NapCat) 安装任务已创建，请在消息中心查看进度');
        setTimeout(() => {
          setMsg('');
          loadSoftware();
          loadInstalledPlugins();
          loadQQChannelState();
          reload();
          loadNapcatStatus();
        }, 5000);
      } else {
        setMsg(`❌ ${r.error || '安装失败'}`);
      }
    } catch {
      setMsg('❌ 安装请求失败');
    } finally {
      setInstallingSw(null);
    }
  };

  const handleInstallChannelPlugin = async (pluginId: string) => {
    setInstallingChannelPlugin(pluginId);
    try {
      const r = await api.installPlugin(pluginId);
      if (r.ok) {
        setMsg(r.message || `✅ ${pluginId} 安装任务已创建，请在消息中心查看进度`);
        setTimeout(() => {
          setMsg('');
          loadInstalledPlugins();
          loadQQChannelState();
          reload();
        }, 5000);
      } else {
        setMsg(`❌ ${r.error || '安装失败'}`);
      }
    } catch {
      setMsg('❌ 安装请求失败');
    } finally {
      setInstallingChannelPlugin(null);
    }
  };

  const isContainerInstalled = (id: string) => {
    const sw = softwareList.find(s => s.id === id);
    return sw?.installed || false;
  };

  const loadInstalledPlugins = () => {
    api.getInstalledPlugins().then((r: any) => { if (r.ok) setInstalledPlugins(r.plugins || []); }).catch(() => {});
  };

  const loadQQChannelState = () => {
    api.getQQChannelState().then((r: any) => { if (r.ok) setQQChannelState(r.state || null); }).catch(() => {});
  };

  const loadOpenClawWeixinStatus = () => {
    api.getOpenClawWeixinStatus().then((r: any) => { if (r.ok) setOpenClawWeixinStatus(r); }).catch(() => {});
  };

  const loadChannelCatalog = useCallback(() => {
    api.getChannelCatalog()
      .then((r: any) => {
        if (!r.ok) return;
        const items = (Array.isArray(r.channels) ? (r.channels as ChannelCatalogItem[]) : [])
          .filter(item => item?.id !== 'qqbot-community' && item?.id !== 'openai' && item?.id !== 'minimax');
        setChannelDefs(mergeChannelDefs(DEFAULT_CHANNEL_DEFS, items));
      })
      .catch(() => {});
  }, []);

  const loadFeishuDmDiagnosis = useCallback(async () => {
    setLoadingFeishuDmDiagnosis(true);
    try {
      const r = await api.getFeishuDMDiagnosis();
      if (!r.ok) return;
      const diagnosis = r.diagnosis as FeishuDMDiagnosis;
      setFeishuDmDiagnosis(diagnosis);
    } catch {
      // noop
    } finally {
      setLoadingFeishuDmDiagnosis(false);
    }
  }, []);

  const isPluginInstalled = (channelId: string) => {
    // 飞书特殊处理：任一版本已安装即视为已安装
    if (channelId === 'feishu') {
      return installedPlugins.some((p: any) => (FEISHU_ALL_IDS as readonly string[]).includes(p.id));
    }
    if (channelId === 'qqbot') {
      return isQQBotPluginInstalled(installedPlugins);
    }
    if (channelId === 'wecom') {
      return installedPlugins.some((p: any) => (WECOM_BOT_PLUGIN_IDS as readonly string[]).includes(p.id));
    }
    // 企业微信自建应用：@sunnoy/wecom 插件 id 为 wecom
    if (channelId === 'wecom-app') {
      return installedPlugins.some((p: any) => p.id === 'wecom' || p.id === 'wecom-app');
    }
    // Check if plugin extension is installed (in extensions dir or plugins.installs)
    return installedPlugins.some((p: any) => p.id === channelId);
  };

  const validateChannelBeforeEnable = (channelId: string) => {
    if (channelId === 'wecom') {
      const cfg = getEffectiveChannelConfig(channelId);
      const mode = getWecomConnectionMode(cfg);
      const required = mode === 'long-polling'
        ? [
            ['botId', 'Bot ID'],
            ['secret', 'Secret'],
          ]
        : [
            ['token', 'Token'],
            ['encodingAESKey', 'EncodingAESKey'],
            ['webhookPath', 'Webhook Path'],
          ];
      const missing = required
        .filter(([key]) => !String(getNestedValue(cfg, key) ?? '').trim())
        .map(([, label]) => label);
      if (missing.length) {
        return `企业微信（智能机器人）当前连接方式缺少必填项：${missing.join('、')}`;
      }
      return '';
    }
    const requiredFields = CHANNEL_REQUIRED_FIELDS[channelId] || [];
    if (!requiredFields.length) return '';
    const cfg = getEffectiveChannelConfig(channelId);
    const missingLabels = requiredFields
      .filter(key => {
        const value = key.split('.').reduce((obj: any, part: string) => obj?.[part], cfg);
        return !String(value ?? '').trim();
      })
      .map(key => channelDefs.find(ch => ch.id === channelId)?.configFields.find(field => field.key === key)?.label || key);
    if (missingLabels.length) {
      const channelLabel = channelDefs.find(ch => ch.id === channelId)?.label || channelId;
      if (channelId === 'dingtalk') {
        return '钉钉需要先填写 Client ID 和 Client Secret 才能启用';
      }
      return `${channelLabel} 需要先填写：${missingLabels.join('、')}`;
    }
    return '';
  };

  const reload = () => {
    loadChannelCatalog();
    api.getStatus().then(r => { if (r.ok) setStatus(r); });
    api.getOpenClawConfig().then(r => {
      if (!r.ok) return;
      const nextConfig = r.config || {};
      setOcConfig(nextConfig);
      setChannelDrafts({});
      setChannelFieldTextDrafts({});
      syncFeishuUiState(nextConfig);
    });
    loadFeishuDmDiagnosis();
    api.getRequests().then(r => { if (r.ok) setRequests(r.requests || []); });
    loadOpenClawWeixinStatus();
  };

  useEffect(() => {
    // Batch all initial loads in parallel for faster page load
    Promise.all([
      api.getStatus().then(r => { if (r.ok) setStatus(r); }).catch(() => {}),
      api.getChannelCatalog().then((r: any) => {
        if (!r.ok) return;
        const items = Array.isArray(r.channels) ? (r.channels as ChannelCatalogItem[]) : [];
        setChannelDefs(mergeChannelDefs(DEFAULT_CHANNEL_DEFS, items));
      }).catch(() => {}),
      api.getOpenClawConfig().then(r => {
        if (!r.ok) return;
        const nextConfig = r.config || {};
        setOcConfig(nextConfig);
        setChannelDrafts({});
        setChannelFieldTextDrafts({});
        syncFeishuUiState(nextConfig);
      }).catch(() => {}),
      loadFeishuDmDiagnosis().catch(() => {}),
      api.getRequests().then(r => { if (r.ok) setRequests(r.requests || []); }).catch(() => {}),
      api.getSoftwareList().then(r => { if (r.ok) { setSoftwareList(r.software || []); if (r.platform) setServerPlatform(r.platform); } }).catch(() => {}),
      api.napcatStatus().then(r => { if (r.ok) setNapcatStatus(r.status); }).catch(() => {}),
      api.getInstalledPlugins().then((r: any) => { if (r.ok) setInstalledPlugins(r.plugins || []); }).catch(() => {}),
      api.getQQChannelState().then((r: any) => { if (r.ok) setQQChannelState(r.state || null); }).catch(() => {}),
      api.getOpenClawWeixinStatus().then((r: any) => { if (r.ok) setOpenClawWeixinStatus(r); }).catch(() => {}),
    ]).catch(() => {});
  }, []);
  // 自动选择第一个已启用的渠道（而非硬编码 QQ）
  useEffect(() => {
    if (selectedChannel) return; // 用户已手动选择
      const firstEnabled = channelDefs.find(ch => {
      const chConf = ocConfig?.channels?.[ch.id] || {};
      const pluginConf = ocConfig?.plugins?.entries?.[ch.id] || {};
      if (ch.id === 'feishu') {
        return chConf.enabled || pluginConf.enabled || !!getEnabledPluginEntry(ocConfig?.plugins?.entries || {}, FEISHU_OFFICIAL_IDS);
      }
      if (ch.id === 'qqbot') {
        return chConf.enabled || pluginConf.enabled || !!getEnabledPluginEntry(ocConfig?.plugins?.entries || {}, QQBOT_PLUGIN_IDS);
      }
      if (ch.id === 'wecom-app') return isWecomAppEnabled(ocConfig);
      if (ch.id === 'wecom') return isWecomBuiltinEnabled(ocConfig);
      return chConf.enabled || pluginConf.enabled;
    });
    if (firstEnabled) setSelectedChannel(firstEnabled.id);
    else setSelectedChannel(channelDefs.some(ch => ch.id === 'feishu') ? 'feishu' : (channelDefs[0]?.id || ''));
  }, [channelDefs, ocConfig, selectedChannel]);
  useEffect(() => {
    if (selectedChannel === 'feishu') syncFeishuUiState(ocConfig);
  }, [selectedChannel, syncFeishuUiState]);
  useEffect(() => {
    if (selectedChannel !== 'qqbot') setQqbotAdvancedOpen(false);
  }, [selectedChannel]);
  useEffect(() => {
    const queryChannel = normalizeChannelQuery(searchParams.get('channel'));
    if (!queryChannel) return;
    setSelectedChannel(prev => prev === queryChannel ? prev : queryChannel);
  }, [channelDefs, searchParams]);
  // 自动选择第一个已启用的渠道（而非硬编码 QQ）
  useEffect(() => {
    const queryChannel = normalizeChannelQuery(searchParams.get('channel'));
    if (queryChannel || selectedChannel) return;
    const firstEnabled = channelDefs.find(ch => {
      const chConf = ocConfig?.channels?.[ch.id] || {};
      const pluginConf = ocConfig?.plugins?.entries?.[ch.id] || {};
      if (ch.id === 'feishu') {
        return chConf.enabled || pluginConf.enabled || !!getEnabledPluginEntry(ocConfig?.plugins?.entries || {}, FEISHU_OFFICIAL_IDS);
      }
      if (ch.id === 'wecom-app') return isWecomAppEnabled(ocConfig);
      if (ch.id === 'wecom') return isWecomBuiltinEnabled(ocConfig);
      return chConf.enabled || pluginConf.enabled;
    });
    if (firstEnabled) setSelectedChannel(firstEnabled.id);
    else setSelectedChannel(channelDefs.some(ch => ch.id === 'feishu') ? 'feishu' : (channelDefs[0]?.id || ''));
  }, [channelDefs, ocConfig, selectedChannel]);
  useEffect(() => {
    const timer = setInterval(loadNapcatStatus, 30000);
    return () => clearInterval(timer);
  }, []);

  const ocChannels = ocConfig?.channels || {};
  const ocPlugins = ocConfig?.plugins?.entries || {};
  const getEffectiveChannelConfig = (channelId: string) => {
    if (isPlainObject(channelDrafts[channelId])) return channelDrafts[channelId];
    // wecom-app is backed by channels.wecom.agent in openclaw.json
    if (channelId === 'wecom-app') {
      return getWecomAppVirtualConfig(ocConfig);
    }
    if (channelId === 'qqbot' && isPlainObject(ocChannels[channelId])) {
      const cfg = { ...ocChannels[channelId] } as any;
      if (!String(cfg.clientSecret || '').trim() && String(cfg.appSecret || '').trim()) {
        cfg.clientSecret = cfg.appSecret;
      }
      return cfg;
    }
    if (channelId === 'wecom' && isPlainObject(ocChannels[channelId])) {
      const cfg = { ...ocChannels[channelId] } as any;
      if (cfg.encodingAESKey === undefined && cfg.encodingAesKey !== undefined) {
        cfg.encodingAESKey = cfg.encodingAesKey;
      }
      cfg.connectionMode = getWecomConnectionMode(cfg);
      return cfg;
    }
    if (isPlainObject(ocChannels[channelId])) return ocChannels[channelId];
    return {};
  };
  const currentFeishuConfig = getEffectiveChannelConfig('feishu');
  const currentFeishuVariant = getActiveFeishuVariant(ocConfig);
  const currentFeishuRequireMentionRaw = currentFeishuConfig?.requireMention;
  const currentFeishuRequireMentionLiteral = typeof currentFeishuRequireMentionRaw === 'string'
    ? currentFeishuRequireMentionRaw.trim()
    : '';
  const currentFeishuRequireMentionValue = currentFeishuRequireMentionRaw === true
    ? 'true'
    : currentFeishuRequireMentionRaw === false
      ? 'false'
      : currentFeishuRequireMentionLiteral.toLowerCase() === 'true'
        ? 'true'
        : currentFeishuRequireMentionLiteral.toLowerCase() === 'false'
          ? 'false'
          : '';
  const currentFeishuRequireMentionInvalid = currentFeishuRequireMentionRaw !== undefined
    && currentFeishuRequireMentionRaw !== null
    && !(typeof currentFeishuRequireMentionRaw === 'string' && currentFeishuRequireMentionLiteral === '')
    && currentFeishuRequireMentionValue === '';
  const currentFeishuRequireMentionHelp = currentFeishuVariant === 'official'
    ? '官方版当前按布尔开关处理：true = 群聊必须 @ 机器人；false = 允许不 @。open 属于 groupPolicy，不属于本字段。'
    : currentFeishuVariant === 'clawteam'
      ? 'ClawTeam 版在面板中同样按布尔开关处理：true = 群聊必须 @ 机器人；false = 放宽触发。open 属于 groupPolicy，不属于本字段。'
      : '按布尔开关处理：true = 群聊必须 @ 机器人；false = 允许不 @。open 属于 groupPolicy，不属于本字段。';
  const currentFeishuAccounts = listFeishuAccountIDs(currentFeishuConfig);
  const currentFeishuDefaultAccount = pickFeishuDefaultAccount(currentFeishuConfig);
  const currentFeishuEditingAccountId = currentFeishuAccounts.includes(feishuActiveAccountId)
    ? feishuActiveAccountId
    : (currentFeishuDefaultAccount || currentFeishuAccounts[0] || 'default');
  const currentFeishuSimpleCredentials = resolveFeishuSimpleCredentials(currentFeishuConfig);
  const currentFeishuAccountConfig = getFeishuAccountEntry(currentFeishuConfig, currentFeishuEditingAccountId);
  const currentFeishuHasStoredAccounts = hasFeishuAdvancedAccounts(currentFeishuConfig);
  const currentFeishuRunnableAccounts = listFeishuRunnableAccountIDs(currentFeishuConfig);
  const currentFeishuEnabledCount = countEnabledFeishuAccounts(currentFeishuConfig);
  const currentFeishuVariantHint = currentFeishuVariant === 'official'
    ? '官方版仍在快速迭代；面板目前优先暴露确认过的共享字段，高级账号结构是否被完整消费取决于插件版本。'
    : currentFeishuVariant === 'clawteam'
      ? 'ClawTeam 版字段相对明确；这里仍统一写入 channels.feishu 共享配置。'
      : '未检测到活动变体时，也会先写入共享的 channels.feishu 配置。';
  const currentFeishuAccountBoundaryHint = currentFeishuVariant === 'official'
    ? '当前已检测到官方插件；面板统一写入 channels.feishu。defaultAccount/accounts 是当前面板的高级账号模型，但官方插件是否完整消费这套结构尚未完全证实，请以插件版本和实测为准。至少需要确保默认账号具备完整凭证，顶层 appId/appSecret 会镜像这个账号。'
    : '面板统一写入 channels.feishu；默认账号会同步镜像到顶层 appId/appSecret，便于单账号路径与高级账号路径共存。';
  const currentFeishuGroupPolicy = String(currentFeishuConfig.groupPolicy || '').trim();
  const currentFeishuGroupAllowFrom = formatCommaList(currentFeishuConfig.groupAllowFrom);
  const hasFeishuGroupAllowlistConflict = currentFeishuGroupPolicy !== 'allowlist' && !!currentFeishuGroupAllowFrom;
  const currentConfiguredFeishuDmScope = String(feishuDmDiagnosis?.configuredDmScope || '').trim();
  const currentEffectiveFeishuDmScope = String(feishuDmDiagnosis?.effectiveDmScope || 'main').trim() || 'main';
  const feishuAuthorizedSenders = Array.isArray(feishuDmDiagnosis?.authorizedSenders) ? feishuDmDiagnosis.authorizedSenders : [];
  const currentWecomConfig = getEffectiveChannelConfig('wecom');
  const currentWecomMode = getWecomConnectionMode(currentWecomConfig);
  const currentWecomEntryId = getEnabledPluginEntry(ocPlugins, WECOM_BOT_PLUGIN_IDS) || (ocPlugins['wecom-openclaw-plugin'] ? 'wecom-openclaw-plugin' : (ocPlugins['wecom'] ? 'wecom' : 'wecom-openclaw-plugin'));
  const currentQQBotVariant = getActiveQQBotVariant(ocConfig);
  const currentQQBotPluginEntryId = currentQQBotVariant === 'community' ? 'qqbot-community' : 'qqbot';

  const handleWecomModeChange = (mode: 'callback' | 'long-polling') => {
    updateChannelDraft('wecom', draft => {
      draft.connectionMode = mode;
    });
  };

  const handleToggleFeishuAdvancedAccounts = (enabled: boolean) => {
    setFeishuAdvancedAccounts(enabled);
    updateFeishuDraft(draft => {
      const seededAccount = pickFeishuDefaultAccount(draft) || 'default';
      const entry = ensureFeishuAccountEntry(draft, seededAccount);
      if (draft.appId && !entry.appId) entry.appId = draft.appId;
      if (draft.appSecret && !entry.appSecret) entry.appSecret = draft.appSecret;
      draft.defaultAccount = seededAccount;
      if (enabled) applyFeishuMultiAccountMode(draft);
      else applyFeishuDefaultOnlyMode(draft);
    });
    setFeishuActiveAccountId(currentFeishuDefaultAccount || 'default');
  };

  const handleFeishuSimpleFieldChange = (key: 'appId' | 'appSecret' | 'botName', value: string) => {
    updateFeishuDraft(draft => {
      const defaultAccount = pickFeishuDefaultAccount(draft) || 'default';
      draft.defaultAccount = defaultAccount;
      const entry = ensureFeishuAccountEntry(draft, defaultAccount);
      if (value) entry[key] = value;
      else delete entry[key];
      if (key === 'appId' || key === 'appSecret') {
        if (value) draft[key] = value;
        else delete draft[key];
      }
      if (feishuAdvancedAccounts) applyFeishuMultiAccountMode(draft);
      else applyFeishuDefaultOnlyMode(draft);
    });
  };

  const handleFeishuDefaultAccountChange = (accountId: string) => {
    if (!hasFeishuRunnableCredentials(getFeishuAccountEntry(currentFeishuConfig, accountId)) && currentFeishuRunnableAccounts.some(id => id !== accountId)) {
      setMsg('请先为该账号填写完整 App ID / App Secret，再设为默认账号。');
      setTimeout(() => setMsg(''), 4000);
      return;
    }
    updateFeishuDraft(draft => {
      if (accountId) draft.defaultAccount = accountId;
      else delete draft.defaultAccount;
      ensureFeishuAccountEntry(draft, accountId);
      if (feishuAdvancedAccounts) applyFeishuMultiAccountMode(draft);
      else applyFeishuDefaultOnlyMode(draft);
    });
    setFeishuActiveAccountId(accountId);
  };

  const handleFeishuAccountFieldChange = (accountId: string, key: 'appId' | 'appSecret' | 'botName', value: string) => {
    updateFeishuDraft(draft => {
      const entry = ensureFeishuAccountEntry(draft, accountId);
      if (value) entry[key] = value;
      else delete entry[key];
      if (feishuAdvancedAccounts) applyFeishuMultiAccountMode(draft);
      else applyFeishuDefaultOnlyMode(draft);
    });
  };

  const handleFeishuAccountEnabledChange = (accountId: string, enabled: boolean) => {
    if (accountId === currentFeishuDefaultAccount && !enabled) {
      setMsg('默认账号必须保持启用状态。');
      setTimeout(() => setMsg(''), 3000);
      return;
    }
    updateFeishuDraft(draft => {
      const entry = ensureFeishuAccountEntry(draft, accountId);
      entry.enabled = enabled;
      applyFeishuMultiAccountMode(draft);
    });
  };

  const handleAddFeishuAccount = () => {
    const nextID = feishuNewAccountId.trim();
    if (!nextID) {
      setMsg('请先输入 Account ID');
      setTimeout(() => setMsg(''), 3000);
      return;
    }
    if (!/^[A-Za-z0-9._-]+$/.test(nextID)) {
      setMsg('Account ID 仅支持字母、数字、点、下划线和中划线');
      setTimeout(() => setMsg(''), 4000);
      return;
    }
    if (currentFeishuAccounts.includes(nextID)) {
      setMsg(`Account ID 已存在：${nextID}`);
      setTimeout(() => setMsg(''), 3000);
      return;
    }
    updateFeishuDraft(draft => {
      const hasDefault = !!String(draft.defaultAccount || '').trim();
      const entry = ensureFeishuAccountEntry(draft, nextID);
      if (!hasDefault) {
        draft.defaultAccount = nextID;
        entry.enabled = true;
      } else if (typeof entry.enabled !== 'boolean') {
        entry.enabled = false;
      }
      applyFeishuMultiAccountMode(draft);
    });
    setFeishuAdvancedAccounts(true);
    setFeishuActiveAccountId(nextID);
    setFeishuNewAccountId('');
  };

  const handleRemoveFeishuAccount = (accountId: string) => {
    if (currentFeishuAccounts.length <= 1) {
      setMsg('至少保留一个账号；如果只需要单机器人，请切回单账号模式。');
      setTimeout(() => setMsg(''), 4000);
      return;
    }
    const remaining = currentFeishuAccounts.filter(id => id !== accountId);
    updateFeishuDraft(draft => {
      if (!isPlainObject(draft.accounts)) return;
      delete draft.accounts[accountId];
      let nextDefault = String(draft.defaultAccount || '').trim();
      if (!nextDefault || nextDefault === accountId || !isPlainObject(draft.accounts?.[nextDefault])) {
        const rest = listFeishuAccountIDs(draft);
        nextDefault = rest.find(id => isFeishuAccountEnabled(draft, id)) || rest[0] || '';
      }
      if (nextDefault) draft.defaultAccount = nextDefault;
      else delete draft.defaultAccount;
      if (feishuAdvancedAccounts) applyFeishuMultiAccountMode(draft);
      else applyFeishuDefaultOnlyMode(draft);
    });
    if (feishuActiveAccountId === accountId) {
      const nextPreferred = remaining.find(id => isFeishuAccountEnabled(currentFeishuConfig, id)) || remaining[0] || 'default';
      setFeishuActiveAccountId(nextPreferred);
    }
  };

  // Get the merged config for the current channel (supports nested keys like notifications.antiRecall)
  const getFieldValue = (channelId: string, key: string) => {
    const chConf = getEffectiveChannelConfig(channelId);
    const fieldDef = channelDefs.find(ch => ch.id === channelId)?.configFields.find(field => field.key === key);
    if (channelId === 'qq') {
      if (key === 'rateLimit.wakeProbability') {
        const nested = chConf?.rateLimit?.wakeProbability;
        return nested ?? chConf?.wakeProbability;
      }
      if (key === 'rateLimit.minIntervalSec') {
        const nested = chConf?.rateLimit?.minIntervalSec;
        if (nested !== undefined && nested !== null) return nested;
        const legacyMs = chConf?.minSendIntervalMs;
        if (legacyMs !== undefined && legacyMs !== null) return Math.round(Number(legacyMs) / 1000);
      }
      if (key === 'rateLimit.wakeTrigger.keywords') {
        const nested = chConf?.rateLimit?.wakeTrigger?.keywords;
        if (Array.isArray(nested)) return nested.join(',');
        const legacy = chConf?.wakeTrigger;
        if (typeof legacy === 'string') return legacy;
      }
    }
    if (channelId === 'feishu' && key === 'groupAllowFrom') {
      return formatCommaList(chConf?.groupAllowFrom);
    }
    if (fieldDef?.valueFormat === 'stringArray') {
      return formatCommaList(key.split('.').reduce((o: any, k: string) => o?.[k], chConf));
    }
    return key.split('.').reduce((o: any, k: string) => o?.[k], chConf);
  };

  const handleFieldDraftChange = (channelId: string, field: ChannelConfigField, rawValue: string) => {
    if (field.type === 'textarea') {
      const fieldDraftKey = `${channelId}:${field.key}`;
      setChannelFieldTextDrafts(prev => {
        const next = { ...prev };
        if (rawValue) next[fieldDraftKey] = rawValue;
        else delete next[fieldDraftKey];
        return next;
      });
    }
    updateChannelDraft(channelId, draft => {
      const trimmed = rawValue.trim();
      if (!trimmed) {
        deleteNestedValue(draft, field.key);
        return;
      }

      if (field.type === 'number') {
        const parsed = Number(trimmed);
        if (!Number.isFinite(parsed)) return;
        setNestedValue(draft, field.key, parsed);
        return;
      }

      if (channelId === 'qq' && field.key === 'rateLimit.wakeTrigger.keywords') {
        setNestedValue(draft, field.key, parseDelimitedList(rawValue));
        return;
      }

      if (channelId === 'feishu' && field.key === 'requireMention') {
        setNestedValue(draft, field.key, trimmed === 'true');
        return;
      }

      if (channelId === 'feishu' && field.key === 'groupAllowFrom') {
        setNestedValue(draft, field.key, parseDelimitedList(rawValue));
        return;
      }

      if (field.valueFormat === 'stringArray') {
        setNestedValue(draft, field.key, parseDelimitedList(rawValue));
        return;
      }

      setNestedValue(draft, field.key, rawValue);
    });
  };

  const isChannelEnabled = (channelId: string) => {
    if (channelId === 'feishu') {
      return ocPlugins[channelId]?.enabled || !!getEnabledPluginEntry(ocPlugins, FEISHU_OFFICIAL_IDS) || ocChannels[channelId]?.enabled || false;
    }
    if (channelId === 'qqbot') {
      return ocChannels[channelId]?.enabled || !!getEnabledPluginEntry(ocPlugins, QQBOT_PLUGIN_IDS) || false;
    }
    if (channelId === 'wecom-app') return isWecomAppEnabled(ocConfig);
    if (channelId === 'wecom') return isWecomBuiltinEnabled(ocConfig);
    return ocChannels[channelId]?.enabled || ocPlugins[channelId]?.enabled || false;
  };

  const currentDef = selectedChannel === CHANNEL_UNSELECTED_ID ? undefined : channelDefs.find(c => c.id === selectedChannel);
  const qqbotPrimaryFields = currentDef?.id === 'qqbot'
    ? currentDef.configFields.filter(field => field.key === 'appId' || field.key === 'clientSecret')
    : [];
  const qqbotAdvancedFields = currentDef?.id === 'qqbot'
    ? currentDef.configFields.filter(field => field.key !== 'appId' && field.key !== 'clientSecret')
    : [];
  const currentLoginChannelId = loginChannelId || currentDef?.id || '';
  const currentLoginChannel = channelDefs.find(channel => channel.id === currentLoginChannelId) || currentDef;
  const openClawWeixinAccounts = Array.isArray(openClawWeixinStatus?.accounts) ? openClawWeixinStatus.accounts : [];
  const currentChannelSupportsPairing = !!currentDef && currentDef.id !== 'openclaw-weixin' && (
    PAIRING_CAPABLE_CHANNELS.has(currentDef.id) ||
    currentDef.configFields.some(field => field.key === 'dmPolicy')
  );
  const currentChannelPairingEnabled = !!currentDef && currentChannelSupportsPairing && String(getEffectiveChannelConfig(currentDef.id)?.dmPolicy || '').trim() === 'pairing';

  const resolvePairingAccountId = useCallback((channelId: string) => {
    const cfg = getEffectiveChannelConfig(channelId);
    if (channelId === 'feishu') {
      return feishuAdvancedAccounts ? pickFeishuDefaultAccount(cfg) : pickFeishuDefaultAccount(cfg);
    }
    return String(cfg?.defaultAccount || '').trim();
  }, [feishuAdvancedAccounts, channelDrafts, ocConfig]);

  const loadPairingRequests = useCallback(async (channelId: string, accountId?: string) => {
    if (!channelId) {
      setPairingRequests([]);
      return;
    }
    setLoadingPairingRequests(true);
    try {
      const r = await api.getOpenClawPairingRequests(channelId, accountId);
      if (r?.ok) {
        setPairingRequests(Array.isArray(r.requests) ? r.requests : []);
      } else {
        setPairingRequests([]);
      }
    } catch {
      setPairingRequests([]);
    } finally {
      setLoadingPairingRequests(false);
    }
  }, []);
  useEffect(() => {
    if (!currentDef || !currentChannelPairingEnabled) {
      setPairingRequests([]);
      return;
    }
    loadPairingRequests(currentDef.id, resolvePairingAccountId(currentDef.id) || undefined);
  }, [currentDef, currentChannelPairingEnabled, loadPairingRequests, resolvePairingAccountId]);

  const syncSelectedChannel = (channelId: string) => {
    setSelectedChannel(channelId);
    const next = new URLSearchParams(searchParams);
    next.set('channel', channelId);
    setSearchParams(next, { replace: true });
  };

  const WECOM_MUTEX: Record<string, string> = { 'wecom': 'wecom-app', 'wecom-app': 'wecom' };

  const handleToggleEnabled = async (channelId: string) => {
    const newEnabled = !isChannelEnabled(channelId);
    if (newEnabled) {
      const validationError = validateChannelBeforeEnable(channelId);
      if (validationError) {
        setMsg(`❌ ${validationError}`);
        setTimeout(() => setMsg(''), 5000);
        return;
      }
    }
    try {
      // 企微互斥：开启其中一个时自动关闭另一个
      const mutexId = WECOM_MUTEX[channelId];
      if (newEnabled && mutexId && isChannelEnabled(mutexId)) {
        await api.toggleChannel(mutexId, false);
        await api.updatePlugin(currentWecomEntryId, { enabled: false });
      }
      const r = await api.toggleChannel(channelId, newEnabled);
      if (r.ok) {
        setMsg(r.message || (newEnabled ? t.channels.channelEnabled : t.channels.channelDisabled));
        if (channelId === 'qq' && !newEnabled) {
          setMsg(t.channels.qqClosing);
        }
      } else {
        setMsg(r.error || t.common.operationFailed);
      }
      reload();
      setTimeout(() => setMsg(''), 5000);
    } catch (err) { setMsg(t.common.operationFailed + ': ' + String(err)); setTimeout(() => setMsg(''), 3000); }
  };

  const handleSave = async () => {
    if (!currentDef) return;
    setSaving(true); setMsg('');
    try {
      const chData: any = deepClone(getEffectiveChannelConfig(currentDef.id));
      if (currentDef.id === 'feishu' && currentFeishuRequireMentionInvalid) {
        throw new Error(`requireMention 仅支持 true/false，当前值为 ${JSON.stringify(currentFeishuRequireMentionRaw)}`);
      }
      const enabledState = isChannelEnabled(currentDef.id);
      if (currentDef.id === 'feishu' && String(chData.groupPolicy || '').trim() !== 'allowlist') {
        delete chData.groupAllowFrom;
      }
      const r = await api.updateChannel(currentDef.id, chData);
      if (!r.ok) throw new Error(r.error || t.channels.saveFailed);
      // 企微互斥：保存并启用时自动关闭另一个
      const mutexId = WECOM_MUTEX[currentDef.id];
      if (enabledState && mutexId && isChannelEnabled(mutexId)) {
        await api.toggleChannel(mutexId, false);
        await api.updatePlugin(currentWecomEntryId, { enabled: false });
      }
      // 飞书特殊处理：保存时操作当前活跃变体的 plugin entry
      if (currentDef.id === 'feishu') {
        const entryId = getFeishuPluginEntryId(ocConfig);
        await api.updatePlugin(entryId, { enabled: enabledState });
      } else if (currentDef.id === 'qqbot') {
        await api.updatePlugin(currentQQBotPluginEntryId, { enabled: enabledState });
      } else if (currentDef.id === 'wecom') {
        await api.updatePlugin(currentWecomEntryId, { enabled: enabledState });
      } else if (currentDef.id === 'wecom-app') {
        // wecom-app 复用 wecom 插件，操作 wecom entry 而非创建无效的 wecom-app entry
        await api.updatePlugin(currentWecomEntryId, { enabled: enabledState });
      } else if (currentDef.type === 'plugin') {
        await api.updatePlugin(currentDef.id, { enabled: enabledState });
      }
      setMsg(r.message || t.channels.saveSuccess);
      reload();
      setTimeout(() => setMsg(''), 5000);
    } catch (err) { setMsg(t.channels.saveFailed + ': ' + String(err)); }
    finally { setSaving(false); }
  };

  const handleToggleField = (channelId: string, key: string) => {
    updateChannelDraft(channelId, draft => {
      setNestedValue(draft, key, !getNestedValue(draft, key));
    });
  };

  // 飞书版本切换
  const handleSwitchFeishuVariant = async (variant: 'official' | 'clawteam') => {
    try {
      const r = await api.switchFeishuVariant(variant);
      if (r.ok) {
        setMsg(r.message || '飞书版本已切换');
      } else {
        setMsg(r.error || '切换失败');
      }
      reload();
      setTimeout(() => setMsg(''), 5000);
    } catch (err) { setMsg('切换失败: ' + String(err)); setTimeout(() => setMsg(''), 3000); }
  };
  const handleSwitchQQBotVariant = async (variant: QQBotVariant) => {
    if (variant === currentQQBotVariant || switchingQQBotVariant) return;
    setSwitchingQQBotVariant(true);
    try {
      const r = await api.switchQQBotVariant(variant);
      if (r?.ok) {
        setMsg(r.message || 'QQ å®˜æ–¹æœºå™¨äººç‰ˆæœ¬å·²åˆ‡æ¢');
      } else {
        setMsg(r?.error || 'QQ å®˜æ–¹æœºå™¨äººç‰ˆæœ¬åˆ‡æ¢å¤±è´¥');
      }
      reload();
      setTimeout(() => setMsg(''), 5000);
    } catch (err) {
      setMsg('QQ å®˜æ–¹æœºå™¨äººç‰ˆæœ¬åˆ‡æ¢å¤±è´¥: ' + String(err));
      setTimeout(() => setMsg(''), 5000);
    } finally {
      setSwitchingQQBotVariant(false);
    }
  };
  // === QQ Login handlers ===
  const handleQRLogin = async (channelId = currentDef?.id || selectedChannel) => {
    setLoginChannelId(channelId);
    setLoginModal('qrcode');
    setQrImg('');
    setQrLoading(true);
    setLoginMsg('');
    try {
      if (channelId === 'openclaw-weixin') {
        const r = await api.startOpenClawWeixinQRCode({
          force: true,
          sessionKey: openClawWeixinSessionKey || undefined,
        });
        if (r.ok && r.qrcodeUrl) {
          setQrImg(r.qrcodeUrl);
          setOpenClawWeixinSessionKey(String(r.sessionKey || ''));
          startQrPolling('openclaw-weixin', String(r.sessionKey || ''));
        } else {
          setLoginMsg(r.message || r.error || '获取微信二维码失败');
        }
        return;
      }

      const r = await api.napcatGetQRCode();
      if (r.ok && r.data?.qrcode) {
        setQrImg(r.data.qrcode);
        startQrPolling('qq');
      } else if (r.message?.includes('Logined') || r.data?.message?.includes('Logined')) {
        setLoginMsg('QQ 已登录，无需重复登录');
      } else {
        setLoginMsg(r.message || r.data?.message || r.error || '获取二维码失败');
      }
    } catch (err) {
      setLoginMsg((channelId === 'openclaw-weixin' ? '获取微信二维码失败: ' : '获取二维码失败: ') + String(err));
    } finally {
      setQrLoading(false);
    }
  };

  const handleRefreshQR = async () => {
    setQrLoading(true); setLoginMsg('');
    try {
      if (currentLoginChannelId === 'openclaw-weixin') {
        const r = await api.startOpenClawWeixinQRCode({
          force: true,
          sessionKey: openClawWeixinSessionKey || undefined,
        });
        if (r.ok && r.qrcodeUrl) {
          setQrImg(r.qrcodeUrl);
          setOpenClawWeixinSessionKey(String(r.sessionKey || ''));
          startQrPolling('openclaw-weixin', String(r.sessionKey || ''));
        } else {
          setLoginMsg(r.message || r.error || '刷新失败');
        }
        return;
      }

      const r = await api.napcatRefreshQRCode();
      if (r.ok && r.data?.qrcode) {
        setQrImg(r.data.qrcode);
        startQrPolling('qq');
      } else {
        setLoginMsg(r.data?.message || '刷新失败');
      }
    } catch { setLoginMsg('刷新失败'); }
    finally { setQrLoading(false); }
  };

  const handleQuickLoginOpen = async () => {
    setLoginModal('quick'); setQuickList([]); setLoginLoading(true); setLoginMsg('');
    try {
      const r = await api.napcatQuickLoginList();
      if (r.ok && r.data) {
        const list = Array.isArray(r.data) ? r.data : (r.data.QuickLoginList || r.data.quickLoginList || []);
        setQuickList(list.map((item: any) => typeof item === 'string' ? item : item.uin || String(item)));
        if (list.length === 0) {
          if (r.message?.includes('Logined') || r.data?.message?.includes('Logined')) {
            setLoginMsg('QQ 已登录，无需重复登录');
          } else {
            setLoginMsg('没有可用的快速登录账号，请先使用扫码登录一次');
          }
        }
      } else { setLoginMsg(r.message || r.error || '获取快速登录列表失败'); }
    } catch (err) { setLoginMsg('获取快速登录列表失败: ' + String(err)); }
    finally { setLoginLoading(false); }
  };

  const handleQuickLogin = async (uin: string) => {
    setLoginLoading(true); setLoginMsg('');
    try {
      const r = await api.napcatQuickLogin(uin);
      if (r.ok && (r.code === 0 || r.message?.includes('Logined'))) { setLoginMsg('登录成功！'); reload(); setTimeout(() => setLoginModal(null), 1500); }
      else { setLoginMsg(r.message || r.data?.message || r.error || '快速登录失败'); }
    } catch (err) { setLoginMsg('快速登录失败: ' + String(err)); }
    finally { setLoginLoading(false); }
  };

  const handlePasswordLoginOpen = () => {
    setLoginModal('password'); setLoginUin(''); setLoginPwd(''); setLoginMsg('');
  };

  const handlePasswordLogin = async () => {
    if (!loginUin || !loginPwd) { setLoginMsg('请输入QQ号和密码'); return; }
    setLoginLoading(true); setLoginMsg('');
    try {
      const r = await api.napcatPasswordLogin(loginUin, loginPwd);
      if (r.ok && (r.code === 0 || r.message?.includes('Logined'))) { setLoginMsg('登录成功！'); reload(); setTimeout(() => setLoginModal(null), 1500); }
      else { setLoginMsg(r.message || r.data?.message || r.error || '账密登录失败'); }
    } catch (err) { setLoginMsg('账密登录失败: ' + String(err)); }
    finally { setLoginLoading(false); }
  };

  const handleQQLogout = async () => {
    if (!confirm('确定要退出当前QQ登录？将重启 NapCat 容器，需要重新扫码登录。')) return;
    setLoginLoading(true); setLoginMsg('');
    try {
      const r = await api.napcatLogout();
      if (r.ok) {
        setMsg('QQ 正在退出登录，NapCat 容器重启中，请等待约 30 秒后重新扫码...');
        setTimeout(() => { reload(); setMsg(''); }, 15000);
      } else {
        setMsg(r.error || '退出登录失败');
      }
    } catch (err) { setMsg('退出登录失败: ' + String(err)); }
    finally { setLoginLoading(false); setTimeout(() => setMsg(''), 15000); }
  };

  const handleRestartNapcat = async () => {
    if (!confirm('确定要重启 NapCat 容器？重启期间 QQ 将暂时离线。')) return;
    setRestarting(true);
    try {
      const r = await api.napcatRestart();
      if (r.ok) {
        setMsg('NapCat 容器正在重启，请等待约 30 秒...');
        setTimeout(() => { reload(); setMsg(''); setRestarting(false); loadNapcatStatus(); }, 15000);
      } else {
        setMsg(r.error || '重启失败');
        setRestarting(false);
      }
    } catch (err) { setMsg('重启失败: ' + String(err)); setRestarting(false); }
  };

  const handleDeleteQQChannel = async () => {
    if (!confirm('确定要一键删除 QQ 通道吗？这会卸载 NapCat、卸载 qq 插件，并清空 openclaw.json 中所有 QQ 相关配置与登录数据。')) return;
    try {
      const r = await api.deleteQQChannel();
      if (r.ok) {
        setMsg(r.message || 'QQ 通道删除任务已创建，请在消息中心查看进度');
        setTimeout(() => {
          reload();
          loadSoftware();
          loadInstalledPlugins();
          loadQQChannelState();
          loadNapcatStatus();
          setMsg('');
        }, 5000);
      } else {
        setMsg(r.error || '删除 QQ 通道失败');
      }
    } catch (err) {
      setMsg('删除 QQ 通道失败: ' + String(err));
    }
  };

  const handleOpenClawWeixinLogout = async (accountId: string) => {
    if (!confirm(`确定要退出微信账号 ${accountId} 吗？退出后需要重新扫码登录。`)) return;
    setLoggingOutWeixinAccount(accountId);
    try {
      const r = await api.logoutOpenClawWeixin(accountId);
      if (r.ok) {
        setMsg(r.message || '微信账号已退出');
        loadOpenClawWeixinStatus();
        reload();
      } else {
        setMsg(r.error || '退出登录失败');
      }
    } catch (err) {
      setMsg('退出登录失败: ' + String(err));
    } finally {
      setLoggingOutWeixinAccount(null);
      setTimeout(() => setMsg(''), 5000);
    }
  };

  const handleDiagnose = async (repair: boolean) => {
    setDiagnosing(true);
    setDiagnoseResult(null);
    try {
      const r = await api.napcatDiagnose(repair);
      if (r.ok) {
        setDiagnoseResult(r);
        loadNapcatStatus();
      } else {
        setMsg(r.error || '诊断失败');
      }
    } catch (err) { setMsg('诊断失败: ' + String(err)); }
    finally { setDiagnosing(false); }
  };

  // QR code polling: check login status every 3s after QR is shown
  const startQrPolling = (channelId: string, sessionKey?: string) => {
    stopQrPolling();
    qrPollRef.current = setInterval(async () => {
      try {
        if (channelId === 'openclaw-weixin') {
          const activeSessionKey = sessionKey || openClawWeixinSessionKey;
          if (!activeSessionKey) return;
          const r = await api.waitOpenClawWeixinQRCode(activeSessionKey, 30000);
          if (!r.ok) {
            setLoginMsg(r.error || '微信登录状态检查失败');
            return;
          }
          if (r.connected) {
            stopQrPolling();
            setLoginMsg(`✅ ${r.message || '微信连接成功'}`);
            loadOpenClawWeixinStatus();
            reload();
            setTimeout(() => {
              setLoginModal(null);
              setOpenClawWeixinSessionKey('');
            }, 1500);
            return;
          }
          if (r.status === 'scaned') {
            setLoginMsg('已扫码，请在微信里确认登录');
            return;
          }
          if (r.status === 'expired') {
            stopQrPolling();
            setLoginMsg('二维码已过期，请刷新');
            return;
          }
          if (r.message) setLoginMsg(r.message);
          return;
        }

        const r = await api.napcatLoginStatus();
        if (r.ok && r.data?.isLogin) {
          stopQrPolling();
          setLoginMsg('✅ 登录成功！');
          reload();
          loadNapcatStatus();
          setTimeout(() => setLoginModal(null), 1500);
        }
      } catch {}
    }, 3000);
  };

  const stopQrPolling = () => {
    if (qrPollRef.current) {
      clearInterval(qrPollRef.current);
      qrPollRef.current = null;
    }
  };

  // Cleanup polling on unmount or modal close
  useEffect(() => {
    if (!loginModal) stopQrPolling();
    if (!loginModal) {
      setLoginChannelId('');
      setOpenClawWeixinSessionKey('');
      setQrRenderImg('');
    }
    return () => stopQrPolling();
  }, [loginModal]);

  useEffect(() => {
    let cancelled = false;

    const renderQRCode = async () => {
      if (!qrImg) {
        setQrRenderImg('');
        return;
      }
      if (currentLoginChannelId !== 'openclaw-weixin') {
        setQrRenderImg(qrImg);
        return;
      }
      try {
        const dataUrl = await QRCode.toDataURL(qrImg, {
          width: 192,
          margin: 1,
        });
        if (!cancelled) setQrRenderImg(dataUrl);
      } catch {
        if (!cancelled) {
          setQrRenderImg('');
          setLoginMsg('微信二维码生成失败，请刷新后重试');
        }
      }
    };

    renderQRCode();
    return () => {
      cancelled = true;
    };
  }, [qrImg, currentLoginChannelId]);

  const handleApprove = async (flag: string) => {
    await api.approveRequest(flag);
    setRequests(prev => prev.filter(r => r.flag !== flag));
  };
  const handleReject = async (flag: string) => {
    await api.rejectRequest(flag);
    setRequests(prev => prev.filter(r => r.flag !== flag));
  };
  const handleApprovePairingCode = async (channelId: string, code: string, accountId?: string) => {
    setApprovingPairingCode(code);
    try {
      const r = await api.approveOpenClawPairingRequest({ channelId, code, accountId });
      if (r?.ok) {
        setMsg(`已批准 ${channelDefs.find(ch => ch.id === channelId)?.label || channelId} 配对请求 ${code}`);
        await loadPairingRequests(channelId, accountId);
      } else {
        setMsg(r?.error || '批准 pairing code 失败');
      }
    } catch (err) {
      setMsg('批准 pairing code 失败: ' + String(err));
    } finally {
      setApprovingPairingCode('');
      setTimeout(() => setMsg(''), 5000);
    }
  };

  // Sort channels: enabled first, then configured, then unconfigured
  const sortedBuiltin = channelDefs.filter(c => c.type === 'builtin').sort((a, b) => {
    const order = { enabled: 0, configured: 1, unconfigured: 2 };
    return order[getChannelStatus(a, ocConfig, installedPlugins, qqChannelState)] - order[getChannelStatus(b, ocConfig, installedPlugins, qqChannelState)];
  });
  const sortedPlugin = channelDefs.filter(c => c.type === 'plugin').sort((a, b) => {
    const order = { enabled: 0, configured: 1, unconfigured: 2 };
    return order[getChannelStatus(a, ocConfig, installedPlugins, qqChannelState)] - order[getChannelStatus(b, ocConfig, installedPlugins, qqChannelState)];
  });

  const currentFeishuAllowlistEntries = parseDelimitedList(currentFeishuGroupAllowFrom);

  const renderConfigField = (channelId: string, field: ChannelConfigField) => {
    if (channelId === 'feishu' && field.key === 'groupAllowFrom' && currentFeishuGroupPolicy !== 'allowlist' && currentFeishuAllowlistEntries.length === 0) {
      return null;
    }

    const rawCurrentVal = getFieldValue(channelId, field.key);
    const fieldOptions = channelId === 'feishu' && field.key === 'requireMention'
      ? ['true', 'false']
      : field.options;
    const fieldHelp = channelId === 'feishu' && field.key === 'requireMention'
      ? currentFeishuRequireMentionHelp
      : (field.help || (channelId === 'qqbot' ? QQBOT_ADVANCED_FIELD_HELP[field.key] : ''));
    const currentVal = channelId === 'feishu' && field.key === 'requireMention'
      ? currentFeishuRequireMentionValue
      : rawCurrentVal;
    const isFullWidth =
      field.type === 'textarea'
      || field.key === 'webhookUrl'
      || field.key === 'token'
      || field.key === 'accessToken'
      || field.key === 'appSecret';
    const isCompactToggle = channelId === 'feishu' && field.type === 'toggle';
    const hasExplicitValue = currentVal !== undefined && currentVal !== null && currentVal !== '';
    const defaultHint = !hasExplicitValue && field.defaultValue !== undefined
      ? formatChannelFieldDefaultValue(field.defaultValue)
      : '';
    const groupAllowPreview = channelId === 'feishu' && field.key === 'groupAllowFrom'
      ? parseDelimitedList(String(currentVal || ''))
      : [];
    const textDraftKey = `${channelId}:${field.key}`;
    const textareaValue = field.type === 'textarea'
      ? (channelFieldTextDrafts[textDraftKey] ?? String(currentVal ?? ''))
      : '';

    return (
      <div key={field.key} className={isFullWidth ? 'md:col-span-2' : ''}>
        {field.type !== 'toggle' && (
          <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300 mb-1.5">
            {field.label}
          </label>
        )}

        {field.type === 'toggle' ? (
          <div
            className={`rounded-lg border transition-colors ${
              isCompactToggle
                ? 'flex items-start justify-between gap-4 px-4 py-3 border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 h-full'
                : 'flex items-center gap-3 p-3 border-gray-100 dark:border-gray-800 bg-gray-50/50 dark:bg-gray-900/30'
            }`}
          >
            {isCompactToggle && (
              <div className="min-w-0 flex-1">
                <div className="text-sm font-semibold text-gray-900 dark:text-white">{field.label}</div>
                {field.help && (
                  <p className="mt-1 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">{field.help}</p>
                )}
              </div>
            )}
            <button
              type="button"
              onClick={() => handleToggleField(channelId, field.key)}
              className={`relative shrink-0 w-9 h-5 rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-violet-500 ${currentVal ? 'bg-violet-600' : 'bg-gray-300 dark:bg-gray-600'}`}
            >
              <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow-sm transition-transform ${currentVal ? 'translate-x-4' : ''}`} />
            </button>
            <span className={`text-xs ${currentVal ? 'text-violet-600 dark:text-violet-400 font-medium' : 'text-gray-500'}`}>
              {currentVal ? t.channels.opened : t.channels.closed}
            </span>
          </div>
        ) : field.type === 'select' ? (
          <div className="relative">
            <select
              name={field.key}
              value={currentVal ?? ''}
              onChange={e => handleFieldDraftChange(channelId, field, e.target.value)}
              className={`w-full px-3.5 py-2 text-sm border rounded-lg bg-white dark:bg-gray-900 transition-all focus:ring-2 focus:ring-violet-100 dark:focus:ring-violet-900/30 focus:border-violet-500 outline-none
                ${hasExplicitValue
                  ? 'border-gray-300 dark:border-gray-700 text-gray-900 dark:text-gray-100'
                  : 'border-gray-200 dark:border-gray-800 text-gray-400'}`}
            >
              <option value="">{defaultHint ? t.channels.notConfiguredWithDefault.replace('{value}', defaultHint) : t.common.notConfigured}</option>
              {fieldOptions?.map(opt => (
                <option key={opt} value={opt}>{opt}</option>
              ))}
            </select>
          </div>
        ) : field.type === 'textarea' ? (
          <textarea
            name={field.key}
            rows={field.rows || 3}
            value={textareaValue}
            onChange={e => handleFieldDraftChange(channelId, field, e.target.value)}
            placeholder={field.placeholder || t.common.notConfigured}
            className={`w-full px-3.5 py-2 text-sm border rounded-lg bg-white dark:bg-gray-900 transition-all focus:ring-2 focus:ring-violet-100 dark:focus:ring-violet-900/30 focus:border-violet-500 outline-none resize-y
              ${hasExplicitValue
                ? 'border-gray-300 dark:border-gray-700 text-gray-900 dark:text-gray-100'
                : 'border-gray-200 dark:border-gray-800 text-gray-400'}`}
          />
        ) : (
          <div className="relative">
            <input
              name={field.key}
              type={field.type === 'password' ? 'password' : field.type === 'number' ? 'number' : 'text'}
              value={currentVal ?? ''}
              onChange={e => handleFieldDraftChange(channelId, field, e.target.value)}
              placeholder={field.placeholder || t.common.notConfigured}
              className={`w-full px-3.5 py-2 text-sm border rounded-lg bg-white dark:bg-gray-900 transition-all focus:ring-2 focus:ring-violet-100 dark:focus:ring-violet-900/30 focus:border-violet-500 outline-none
                ${hasExplicitValue
                  ? 'border-gray-300 dark:border-gray-700 text-gray-900 dark:text-gray-100'
                  : 'border-gray-200 dark:border-gray-800 text-gray-400'}`}
            />
            {hasExplicitValue && (
              <div className="absolute right-3 top-1/2 -translate-y-1/2 text-emerald-500">
                <Check size={14} strokeWidth={3} />
              </div>
            )}
          </div>
        )}

        {field.type !== 'toggle' && fieldHelp && (
          <p className="mt-1.5 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">{fieldHelp}</p>
        )}

        {defaultHint && (
          <p className="mt-1 text-[11px] text-gray-400">
            {t.channels.defaultWhenUnset.replace('{value}', defaultHint)}
          </p>
        )}
        {channelId === 'feishu' && field.key === 'groupAllowFrom' && (
          <div className="mt-2 space-y-2">
            <p className="text-[11px] text-gray-500">
              当前将保存为数组 {groupAllowPreview.length > 0 ? `（${groupAllowPreview.length} 个群 ID）` : '（当前为空）'}。
            </p>
            {groupAllowPreview.length > 0 && (
              <div className="flex flex-wrap gap-2">
                {groupAllowPreview.map(groupId => (
                  <span key={groupId} className="px-2 py-1 rounded-full bg-violet-50 dark:bg-violet-900/20 text-[11px] text-violet-700 dark:text-violet-300 border border-violet-100 dark:border-violet-800/40 font-mono">
                    {groupId}
                  </span>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    );
  };

  return (
    <div className={`space-y-4 ${modern ? 'page-modern' : ''}`}>
      <div>
        <h2 className={`${modern ? 'page-modern-title text-xl' : 'text-lg font-bold'}`}>{t.channels.title}</h2>
        <p className={`${modern ? 'page-modern-subtitle text-xs mt-0.5' : 'text-xs text-gray-500 mt-0.5'}`}>{t.channels.subtitle} — <span className="inline-flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-emerald-500 inline-block" />{t.channels.statusEnabled}</span> <span className="inline-flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-red-400 inline-block" />{t.channels.statusConfigured}</span> <span className="inline-flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-gray-300 inline-block" />{t.channels.statusUnconfigured}</span></p>
      </div>

      {msg && (
        <div className={`px-3 py-2 rounded-lg text-xs ${msg.includes('失败') ? 'bg-red-50 dark:bg-red-950 text-red-600' : 'bg-emerald-50 dark:bg-emerald-950 text-emerald-600'}`}>
          {msg}
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
        {/* Channel selector */}
        <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 flex flex-col max-h-[75vh] overflow-hidden">
          <div className="p-3 border-b border-gray-100 dark:border-gray-700/50 bg-gray-50/50 dark:bg-gray-800/50">
            <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider px-1">{t.channels.channelList}</h3>
          </div>
          <div className="flex-1 overflow-y-auto p-2 space-y-4">
            <div>
              <h3 className="text-[10px] font-semibold text-gray-400 mb-2 px-2 uppercase tracking-wide">{t.channels.builtinChannels}</h3>
              <div className="space-y-1">
                {sortedBuiltin.map(ch => {
                  const st = getChannelStatus(ch, ocConfig, installedPlugins, qqChannelState);
                  return (
                    <button key={ch.id} onClick={() => syncSelectedChannel(ch.id)}
                      className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-left text-sm transition-all duration-200 group ${
                        selectedChannel === ch.id
                          ? 'border border-blue-100/80 bg-[linear-gradient(135deg,rgba(37,99,235,0.12),rgba(14,165,233,0.08))] dark:bg-[linear-gradient(135deg,rgba(37,99,235,0.2),rgba(14,165,233,0.12))] dark:border-blue-800/40 text-blue-700 dark:text-blue-300 shadow-sm'
                          : 'text-gray-600 dark:text-gray-400 hover:bg-white/70 dark:hover:bg-gray-700/50 hover:border-blue-100/70'
                      }`}>
                      <div className={`p-1.5 rounded-xl transition-colors border ${selectedChannel === ch.id ? 'bg-blue-50/90 dark:bg-blue-900/30 border-blue-100 dark:border-blue-800/40 text-blue-600' : 'bg-gray-100 dark:bg-gray-700 border-transparent text-gray-500 group-hover:bg-white group-hover:border-blue-100/70 group-hover:shadow-sm'}`}>
                        <Radio size={14} />
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="text-xs font-semibold truncate">{ch.label}</div>
                        <div className="text-[10px] text-gray-400 truncate opacity-80">{ch.description}</div>
                      </div>
                      <span className={`w-2 h-2 rounded-full shrink-0 ${statusDot(st)} ring-2 ring-white dark:ring-gray-800`} title={statusLabel(st)} />
                    </button>
                  );
                })}
              </div>
            </div>
            
            {sortedPlugin.length > 0 && (
              <div>
                <h3 className="text-[10px] font-semibold text-gray-400 mb-2 px-2 uppercase tracking-wide">{t.channels.pluginChannels}</h3>
                <div className="space-y-1">
                  {sortedPlugin.map(ch => {
                    const st = getChannelStatus(ch, ocConfig, installedPlugins, qqChannelState);
                    return (
                      <button key={ch.id} onClick={() => syncSelectedChannel(ch.id)}
                        className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-left text-sm transition-all duration-200 group ${
                          selectedChannel === ch.id
                            ? 'border border-blue-100/80 bg-[linear-gradient(135deg,rgba(37,99,235,0.12),rgba(14,165,233,0.08))] dark:bg-[linear-gradient(135deg,rgba(37,99,235,0.2),rgba(14,165,233,0.12))] dark:border-blue-800/40 text-blue-700 dark:text-blue-300 shadow-sm'
                            : 'text-gray-600 dark:text-gray-400 hover:bg-white/70 dark:hover:bg-gray-700/50 hover:border-blue-100/70'
                        }`}>
                        <div className={`p-1.5 rounded-xl transition-colors border ${selectedChannel === ch.id ? 'bg-blue-50/90 dark:bg-blue-900/30 border-blue-100 dark:border-blue-800/40 text-blue-600' : 'bg-gray-100 dark:bg-gray-700 border-transparent text-gray-500 group-hover:bg-white group-hover:border-blue-100/70 group-hover:shadow-sm'}`}>
                          <Radio size={14} />
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="text-xs font-semibold truncate">{ch.label}</div>
                          <div className="text-[10px] text-gray-400 truncate opacity-80">{ch.description}</div>
                        </div>
                        <span className={`w-2 h-2 rounded-full shrink-0 ${statusDot(st)} ring-2 ring-white dark:ring-gray-800`} title={statusLabel(st)} />
                      </button>
                    );
                  })}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Channel config */}
        <div className="lg:col-span-3 space-y-6">
          {!currentDef && (
            <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 p-10 md:p-14">
              <div className="mx-auto max-w-xl text-center space-y-4">
                <div className="mx-auto w-14 h-14 rounded-2xl bg-blue-50 dark:bg-blue-900/20 border border-blue-100 dark:border-blue-800/40 text-blue-500 flex items-center justify-center">
                  <Radio size={24} />
                </div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white">请选择左侧通道开始配置</h3>
                <p className="text-sm text-gray-500 leading-relaxed">
                  这里会显示对应通道的连接状态、登录入口和参数配置。建议先配置基础凭证，再按需展开高级选项。
                </p>
              </div>
            </div>
          )}

          {/* QQ plugin not installed overlay */}
          {currentDef && currentDef.id === 'qq' && !isQQActuallyInstalled(installedPlugins, qqChannelState) && (
            <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-8 text-center space-y-4">
              <div className="w-16 h-16 mx-auto rounded-2xl bg-amber-50 dark:bg-amber-900/20 flex items-center justify-center">
                <AlertTriangle size={32} className="text-amber-500" />
              </div>
              <div>
                <h3 className="text-lg font-bold text-gray-900 dark:text-white">QQ 个人号插件未安装</h3>
                <p className="text-sm text-gray-500 mt-1">
                  安装 QQ (NapCat) 前会先安装 QQ 个人号插件。当前未检测到插件，请重新执行 NapCat 安装；若仍失败，请检查加速源或手动前往插件中心安装 `qq` 插件。
                </p>
              </div>
              <div className="flex items-center justify-center gap-3 flex-wrap">
                <button
                  onClick={handleSetupQQChannel}
                  disabled={installingSw !== null}
                  className={`${modern ? 'page-modern-accent px-6 py-3 text-sm' : 'inline-flex items-center gap-2 px-6 py-3 text-sm font-medium rounded-xl bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50 transition-all shadow-lg shadow-violet-200 dark:shadow-none hover:shadow-xl'}`}
                >
                  {installingSw ? <Loader2 size={16} className="animate-spin" /> : <Download size={16} />}
                  {installingSw ? '安装中...' : '重新安装 QQ (NapCat)'}
                </button>
                <button onClick={() => navigate('/plugins')} className={`${modern ? 'page-modern-action px-6 py-3 text-sm' : 'inline-flex items-center gap-2 px-6 py-3 text-sm font-medium rounded-xl bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-100 hover:bg-gray-200 dark:hover:bg-gray-600 transition-all'}`}>
                  <Package size={16} />
                  前往插件中心
                </button>
              </div>
            </div>
          )}

          {/* QQ NapCat not installed overlay */}
          {currentDef && currentDef.id === 'qq' && isQQActuallyInstalled(installedPlugins, qqChannelState) && !(qqChannelState?.napcatInstalled || isContainerInstalled('napcat')) && softwareList.length > 0 && (
            <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-8 text-center space-y-4">
              <div className="w-16 h-16 mx-auto rounded-2xl bg-gray-100 dark:bg-gray-700 flex items-center justify-center">
                <Package size={32} className="text-gray-400" />
              </div>
              <div>
                <h3 className="text-lg font-bold text-gray-900 dark:text-white">NapCat (QQ个人号) 未安装</h3>
                <p className="text-sm text-gray-500 mt-1">
                  {serverPlatform === 'windows'
                    ? '需要安装 NapCat Shell 才能使用 QQ 个人号通道。安装后将自动配置 OneBot11 WebSocket 协议。'
                    : '需要安装 NapCat Docker 容器才能使用 QQ 个人号通道。安装后将自动配置 OneBot11 WebSocket 协议。'}
                </p>
              </div>
              {serverPlatform === 'windows' ? (
                <button
                  onClick={handleSetupQQChannel}
                  disabled={installingSw !== null}
                  className={`${modern ? 'page-modern-accent px-6 py-3 text-sm' : 'inline-flex items-center gap-2 px-6 py-3 text-sm font-medium rounded-xl bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50 transition-all shadow-lg shadow-violet-200 dark:shadow-none hover:shadow-xl'}`}
                >
                  {installingSw ? <Loader2 size={16} className="animate-spin" /> : <Download size={16} />}
                  {installingSw ? '安装中...' : '一键安装 NapCat Shell'}
                </button>
              ) : !isContainerInstalled('docker') ? (
                <div className="text-sm text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/20 rounded-lg px-4 py-2 inline-block">
                  需要先安装 Docker，请前往 系统配置 → 运行环境 安装
                </div>
              ) : (
                <button
                  onClick={handleSetupQQChannel}
                  disabled={installingSw !== null}
                  className={`${modern ? 'page-modern-accent px-6 py-3 text-sm' : 'inline-flex items-center gap-2 px-6 py-3 text-sm font-medium rounded-xl bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50 transition-all shadow-lg shadow-violet-200 dark:shadow-none hover:shadow-xl'}`}
                >
                  {installingSw ? <Loader2 size={16} className="animate-spin" /> : <Download size={16} />}
                  {installingSw ? '安装中...' : '一键安装 NapCat Docker'}
                </button>
              )}
              <p className="text-[11px] text-gray-400">安装进度可在右上角铃铛中的消息中心实时查看</p>
            </div>
          )}

          {/* Plugin channel not installed overlay (feishu, qqbot, dingtalk, wecom, etc.) */}
          {currentDef && currentDef.type === 'plugin' && currentDef.id !== 'qq' && !isPluginInstalled(currentDef.id) && (
            <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-8 text-center space-y-4">
              <div className="w-16 h-16 mx-auto rounded-2xl bg-gray-100 dark:bg-gray-700 flex items-center justify-center">
                <Package size={32} className="text-gray-400" />
              </div>
              <div>
                <h3 className="text-lg font-bold text-gray-900 dark:text-white">{currentDef.label} 插件未安装</h3>
                <p className="text-sm text-gray-500 mt-1">
                  需要先安装 {currentDef.label} 插件才能配置此通道。这里会直接调用官方插件安装命令，无需跳转插件中心。
                </p>
              </div>
              <div className="flex items-center justify-center gap-3 flex-wrap">
                <button
                  onClick={() => handleInstallChannelPlugin(currentDef.id)}
                  disabled={installingChannelPlugin !== null}
                  className={`${modern ? 'page-modern-accent px-6 py-3 text-sm' : 'inline-flex items-center gap-2 px-6 py-3 text-sm font-medium rounded-xl bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50 transition-all shadow-lg shadow-violet-200 dark:shadow-none hover:shadow-xl'}`}
                >
                  {installingChannelPlugin === currentDef.id ? <Loader2 size={16} className="animate-spin" /> : <Download size={16} />}
                  {installingChannelPlugin === currentDef.id ? '安装中...' : `一键安装 ${currentDef.label}`}
                </button>
                <button onClick={() => navigate('/plugins')} className={`${modern ? 'page-modern-action px-6 py-3 text-sm' : 'inline-flex items-center gap-2 px-6 py-3 text-sm font-medium rounded-xl bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-100 hover:bg-gray-200 dark:hover:bg-gray-600 transition-all'}`}>
                  <Package size={16} />
                  查看插件中心
                </button>
              </div>
            </div>
          )}

          {currentDef && !(
            (currentDef.id === 'qq' && (!isQQActuallyInstalled(installedPlugins, qqChannelState) || (!(qqChannelState?.napcatInstalled || isContainerInstalled('napcat')) && softwareList.length > 0))) ||
            (currentDef.type === 'plugin' && currentDef.id !== 'qq' && !isPluginInstalled(currentDef.id))
          ) && (
            <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 p-6 space-y-6">
              <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between pb-4 border-b border-gray-100 dark:border-gray-800">
                <div className="flex items-center gap-4">
                  <div className={`p-2.5 rounded-xl ${isChannelEnabled(currentDef.id) ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-600' : 'bg-gray-100 dark:bg-gray-800 text-gray-500'}`}>
                    <Power size={20} />
                  </div>
                  <div>
                    <div className="flex items-center gap-3">
                      <h3 className="font-bold text-base text-gray-900 dark:text-white">{currentDef.label} {t.channels.config}</h3>
                      <span className={`px-2 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-wider ${
                        getChannelStatus(currentDef, ocConfig, installedPlugins, qqChannelState) === 'enabled' ? 'bg-emerald-100 dark:bg-emerald-900/40 text-emerald-700 dark:text-emerald-400' :
                        getChannelStatus(currentDef, ocConfig, installedPlugins, qqChannelState) === 'configured' ? 'bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-400' :
                        'bg-gray-100 dark:bg-gray-800 text-gray-500'
                      }`}>{statusLabel(getChannelStatus(currentDef, ocConfig, installedPlugins, qqChannelState))}</span>
                    </div>
                    <p className="text-xs text-gray-500 mt-1">{currentDef.description}</p>
                  </div>
                </div>
                <div className="flex flex-wrap items-center gap-3 xl:justify-end">
                  {/* Enable/Disable toggle switch */}
                  <div className="flex items-center gap-2 bg-gray-50 dark:bg-gray-900/50 px-3 py-1.5 rounded-lg border border-gray-100 dark:border-gray-800">
                    <span className={`text-[11px] font-medium ${isChannelEnabled(currentDef.id) ? 'text-gray-900 dark:text-gray-100' : 'text-gray-400'}`}>
                      {isChannelEnabled(currentDef.id) ? t.channels.enabledState : t.channels.disabledState}
                    </span>
                      <button onClick={() => handleToggleEnabled(currentDef.id)}
                        className={`relative w-9 h-5 rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-violet-500 ${isChannelEnabled(currentDef.id) ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-gray-600'}`}>
                      <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow-sm transition-transform ${isChannelEnabled(currentDef.id) ? 'translate-x-4' : ''}`} />
                    </button>
                  </div>
                  
                  {currentDef.loginMethods && currentDef.loginMethods.length > 0 && (
                    <div className="flex items-center gap-2 border-l border-gray-200 dark:border-gray-700 pl-3 ml-1">
                      {currentDef.loginMethods.includes('qrcode') && (
                        <button onClick={() => handleQRLogin(currentDef.id)} className={`${modern ? 'page-modern-accent px-3 py-1.5 text-xs' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/50 transition-colors'}`}>
                          <QrCode size={14} />{t.channels.qrLogin}
                        </button>
                      )}
                      {currentDef.loginMethods.includes('quick') && (
                        <button onClick={handleQuickLoginOpen} className={`${modern ? 'page-modern-success px-3 py-1.5 text-xs' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-emerald-50 dark:bg-emerald-900/30 text-emerald-600 dark:text-emerald-400 hover:bg-emerald-100 dark:hover:bg-emerald-900/50 transition-colors'}`}>
                          <Zap size={14} />{t.channels.quickLogin}
                        </button>
                      )}
                      {currentDef.loginMethods.includes('password') && (
                        <button onClick={handlePasswordLoginOpen} className={`${modern ? 'page-modern-warn px-3 py-1.5 text-xs' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-amber-50 dark:bg-amber-900/30 text-amber-600 dark:text-amber-400 hover:bg-amber-100 dark:hover:bg-amber-900/50 transition-colors'}`}>
                          <Key size={14} />{t.channels.passwordLogin}
                        </button>
                      )}
                      {currentDef.id === 'qq' && (
                        <>
                          <button onClick={handleQQLogout} className={`${modern ? 'page-modern-danger px-3 py-1.5 text-xs' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-red-50 dark:bg-red-900/30 text-red-600 dark:text-red-400 hover:bg-red-100 dark:hover:bg-red-900/50 transition-colors'}`}>
                            <LogOut size={14} />{t.channels.logoutQQ}
                          </button>
                          <button onClick={handleRestartNapcat} disabled={restarting} className={`${modern ? 'page-modern-warn px-3 py-1.5 text-xs' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-orange-50 dark:bg-orange-900/30 text-orange-600 dark:text-orange-400 hover:bg-orange-100 dark:hover:bg-orange-900/50 disabled:opacity-50 transition-colors'}`}>
                            {restarting ? <Loader2 size={14} className="animate-spin" /> : <RefreshCw size={14} />}
                            {restarting ? t.channels.restartingNapcat : t.channels.restartNapcat}
                          </button>
                          <button onClick={handleDeleteQQChannel} className={`${modern ? 'page-modern-danger px-3 py-1.5 text-xs' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-red-50 dark:bg-red-900/30 text-red-600 dark:text-red-400 hover:bg-red-100 dark:hover:bg-red-900/50 transition-colors'}`}>
                            <Trash2 size={14} />{t.channels.deleteQQChannel}
                          </button>
                        </>
                      )}
                    </div>
                  )}
                </div>
              </div>

              {/* NapCat Connection Status */}
              {currentDef.id === 'qq' && napcatStatus && (
                <div className={`rounded-xl border p-4 mb-2 ${
                  napcatStatus.status === 'online' ? 'bg-emerald-50/50 dark:bg-emerald-900/10 border-emerald-200 dark:border-emerald-800/30' :
                  napcatStatus.status === 'reconnecting' ? 'bg-amber-50/50 dark:bg-amber-900/10 border-amber-200 dark:border-amber-800/30' :
                  napcatStatus.status === 'login_expired' ? 'bg-orange-50/50 dark:bg-orange-900/10 border-orange-200 dark:border-orange-800/30' :
                  'bg-red-50/50 dark:bg-red-900/10 border-red-200 dark:border-red-800/30'
                }`}>
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <div className={`w-2.5 h-2.5 rounded-full ${
                        napcatStatus.status === 'online' ? 'bg-emerald-500 animate-pulse' :
                        napcatStatus.status === 'reconnecting' ? 'bg-amber-500 animate-pulse' :
                        napcatStatus.status === 'login_expired' ? 'bg-orange-500' :
                        'bg-red-500'
                      }`} />
                      <div>
                        <span className="text-sm font-semibold text-gray-900 dark:text-white">
                          {napcatStatus.status === 'online' ? t.channels.napcatStatusOnline :
                           napcatStatus.status === 'reconnecting' ? t.channels.napcatStatusReconnecting :
                           napcatStatus.status === 'login_expired' ? t.channels.napcatStatusLoginExpired :
                           napcatStatus.status === 'stopped' ? t.channels.napcatStatusStopped :
                           t.channels.napcatStatusOffline}
                        </span>
                        {napcatStatus.qqId && (
                          <span className="ml-2 text-xs text-gray-500">
                            {napcatStatus.qqNickname || napcatStatus.qqId} ({napcatStatus.qqId})
                          </span>
                        )}
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      {napcatStatus.autoReconnect && (
                        <span className="text-[10px] font-medium px-2 py-0.5 rounded-full bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400">
                          {t.channels.autoReconnect} {napcatStatus.reconnectCount > 0 ? `(${napcatStatus.reconnectCount}/${napcatStatus.maxReconnect})` : t.channels.autoReconnectEnabled}
                        </span>
                      )}
                      {(napcatStatus.status === 'offline' || napcatStatus.status === 'stopped') && (
                        <button onClick={handleReconnect} disabled={reconnecting}
                          className={`${modern ? 'page-modern-accent px-2.5 py-1 text-[11px]' : 'flex items-center gap-1 px-2.5 py-1 text-[11px] font-medium rounded-lg bg-violet-50 dark:bg-violet-900/30 text-violet-600 dark:text-violet-400 hover:bg-violet-100 dark:hover:bg-violet-900/50 disabled:opacity-50 transition-colors'}`}>
                          <RefreshCw size={11} className={reconnecting ? 'animate-spin' : ''} />
                          {reconnecting ? t.channels.reconnecting : t.channels.manualReconnect}
                        </button>
                      )}
                      <button onClick={() => { setShowReconnectLogs(!showReconnectLogs); if (!showReconnectLogs) loadReconnectLogs(); }}
                        className="text-[11px] text-gray-500 hover:text-gray-700 dark:hover:text-gray-300 transition-colors underline">
                        {showReconnectLogs ? t.channels.hideReconnectLogs : t.channels.showReconnectLogs}
                      </button>
                    </div>
                  </div>
                  <div className="flex items-center gap-4 mt-2 text-[11px] text-gray-500">
                    <span>容器: {napcatStatus.containerRunning ? '✅ 运行中' : '❌ 未运行'}</span>
                    <span>WS: {napcatStatus.wsConnected ? '✅ 已连接' : '❌ 未连接'}</span>
                    <span>HTTP: {napcatStatus.httpAvailable ? '✅ 可用' : '❌ 不可用'}</span>
                    <span>QQ: {napcatStatus.qqLoggedIn ? '✅ 已登录' : '❌ 未登录'}</span>
                  </div>
                  {showReconnectLogs && reconnectLogs.length > 0 && (
                    <div className="mt-3 border-t border-gray-200 dark:border-gray-700 pt-3 space-y-1.5 max-h-40 overflow-y-auto">
                      <p className="text-[10px] font-bold text-gray-400 uppercase tracking-wider">重连日志</p>
                      {reconnectLogs.slice(-10).reverse().map((log: any, i: number) => (
                        <div key={i} className="flex items-center gap-2 text-[11px]">
                          <span className={log.success ? 'text-emerald-500' : 'text-red-500'}>{log.success ? '✅' : '❌'}</span>
                          <span className="text-gray-400 font-mono">{new Date(log.time).toLocaleTimeString()}</span>
                          <span className="text-gray-600 dark:text-gray-400">{log.reason}</span>
                          {log.detail && <span className="text-gray-400 truncate">— {log.detail}</span>}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {/* NapCat Diagnose & Repair */}
              {currentDef.id === 'qq' && (qqChannelState?.napcatInstalled || isContainerInstalled('napcat')) && (
                <div className="rounded-xl border border-gray-100 dark:border-gray-700/50 p-4 space-y-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Wrench size={16} className="text-violet-500" />
                      <span className="text-sm font-bold text-gray-900 dark:text-white">NapCat 诊断修复</span>
                    </div>
                    <div className="flex items-center gap-2">
                       <button onClick={() => handleDiagnose(false)} disabled={diagnosing}
                         className={`${modern ? 'page-modern-action px-3 py-1.5 text-xs' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/50 disabled:opacity-50 transition-colors'}`}>
                        {diagnosing ? <Loader2 size={12} className="animate-spin" /> : <Search size={12} />}
                        {diagnosing ? '诊断中...' : '检测状态'}
                      </button>
                       <button onClick={() => handleDiagnose(true)} disabled={diagnosing}
                         className={`${modern ? 'page-modern-accent px-3 py-1.5 text-xs' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50 transition-all shadow-sm'}`}>
                        {diagnosing ? <Loader2 size={12} className="animate-spin" /> : <Wrench size={12} />}
                        {diagnosing ? '修复中...' : '诊断并修复'}
                      </button>
                    </div>
                  </div>
                  {diagnoseResult && (
                    <div className="space-y-2">
                      <div className={`px-3 py-2 rounded-lg text-xs font-medium ${
                        diagnoseResult.summary?.includes('✅') ? 'bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600' :
                        diagnoseResult.summary?.includes('🔧') ? 'bg-blue-50 dark:bg-blue-900/20 text-blue-600' :
                        'bg-red-50 dark:bg-red-900/20 text-red-600'
                      }`}>
                        {diagnoseResult.summary}
                      </div>
                      <div className="space-y-1 max-h-60 overflow-y-auto">
                        {diagnoseResult.steps?.map((step: any, i: number) => (
                          <div key={i} className="flex items-start gap-2 text-[11px] px-2 py-1.5 rounded-lg bg-gray-50 dark:bg-gray-900/30">
                            <span className="shrink-0 mt-0.5">
                              {step.status === 'ok' ? <CheckCircle size={13} className="text-emerald-500" /> :
                               step.status === 'fixed' ? <CheckCircle size={13} className="text-blue-500" /> :
                               step.status === 'warning' ? <AlertTriangle size={13} className="text-amber-500" /> :
                               step.status === 'error' ? <AlertCircle size={13} className="text-red-500" /> :
                               <Check size={13} className="text-gray-400" />}
                            </span>
                            <div className="min-w-0 flex-1">
                              <div className="flex items-center gap-2">
                                <span className="font-semibold text-gray-700 dark:text-gray-300">{step.step}</span>
                                <span className={`text-[10px] px-1.5 py-0.5 rounded-full font-bold uppercase ${
                                  step.status === 'ok' ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-600' :
                                  step.status === 'fixed' ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-600' :
                                  step.status === 'warning' ? 'bg-amber-100 dark:bg-amber-900/30 text-amber-600' :
                                  step.status === 'error' ? 'bg-red-100 dark:bg-red-900/30 text-red-600' :
                                  'bg-gray-100 dark:bg-gray-800 text-gray-500'
                                }`}>{step.status}</span>
                              </div>
                              <p className="text-gray-600 dark:text-gray-400">{step.message}</p>
                              {step.detail && <p className="text-gray-400 text-[10px] mt-0.5">{step.detail}</p>}
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              )}

              {/* Restarting overlay */}
              {currentDef.id === 'qq' && restarting && (
                <div className="rounded-xl border-2 border-dashed border-orange-200 dark:border-orange-800 bg-orange-50/50 dark:bg-orange-900/10 p-6 flex flex-col items-center justify-center gap-3">
                  <Loader2 size={32} className="animate-spin text-orange-500" />
                  <p className="text-sm font-medium text-orange-600 dark:text-orange-400">NapCat 正在重启中，请稍候...</p>
                  <p className="text-[11px] text-gray-400">重启期间 QQ 将暂时离线，通常需要 15-30 秒</p>
                </div>
              )}

              {/* 飞书内置通道提示 */}
              {currentDef.id === 'feishu' && (
                <div className="rounded-xl border border-violet-200 dark:border-violet-800 bg-violet-50/50 dark:bg-violet-900/10 p-4 space-y-3">
                  <div className="text-sm font-semibold text-gray-900 dark:text-white">当前飞书接入模式</div>
                  <div className="rounded-lg border border-violet-200/70 dark:border-violet-700/50 bg-white/70 dark:bg-slate-900/40 px-4 py-3 space-y-1.5">
                    <div className="text-sm font-medium text-gray-900 dark:text-white">OpenClaw 内置飞书通道</div>
                    <div className="text-[11px] text-gray-500 leading-relaxed">
                      本机当前 OpenClaw 版本已直接内置 <span className="font-mono">feishu</span> 通道，默认不再需要额外安装飞书插件。
                      面板仍会兼容旧的历史配置，但新部署建议直接按内置通道方式管理。
                    </div>
                  </div>
                </div>
              )}

              {currentDef.id === 'feishu' && (
                <div className="rounded-xl border border-gray-200 dark:border-gray-700 p-4 space-y-4">
                  <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
                    <div className="space-y-1.5">
                      <div className="text-sm font-semibold text-gray-900 dark:text-white">账号配置</div>
                      <p className="text-xs text-gray-500 leading-relaxed">
                        飞书始终使用同一套 <span className="font-mono">defaultAccount + accounts + 顶层镜像</span> 配置；
                        这里切换的是运行方式，不是底层 schema。
                      </p>
                      <p className="text-xs text-gray-500 leading-relaxed">
                        保存时会自动把默认账号同步到顶层 <span className="mx-1 font-mono">appId/appSecret</span>，
                        并强制默认账号保持启用；切回仅默认账号时，其他账号只会被标记为 <span className="font-mono">enabled=false</span>，不会删除。
                      </p>
                    </div>
                    <div className="inline-flex rounded-xl border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/40 p-1">
                      <button
                        type="button"
                        onClick={() => handleToggleFeishuAdvancedAccounts(false)}
                        className={`px-3.5 py-2 text-xs font-medium rounded-lg transition-colors ${
                          !feishuAdvancedAccounts
                            ? 'bg-white dark:bg-gray-800 text-violet-700 dark:text-violet-300 shadow-sm'
                            : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
                        }`}
                      >
                        仅默认账号
                      </button>
                      <button
                        type="button"
                        onClick={() => handleToggleFeishuAdvancedAccounts(true)}
                        className={`px-3.5 py-2 text-xs font-medium rounded-lg transition-colors ${
                          feishuAdvancedAccounts
                            ? 'bg-white dark:bg-gray-800 text-violet-700 dark:text-violet-300 shadow-sm'
                            : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
                        }`}
                      >
                        多账号并行
                      </button>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-3">
                    <div className="rounded-lg border border-gray-100 dark:border-gray-700 bg-gray-50/70 dark:bg-gray-900/30 px-3 py-2.5">
                      <div className="text-[11px] text-gray-500">运行方式</div>
                      <div className="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
                        {feishuAdvancedAccounts ? '多账号并行' : '仅默认账号'}
                      </div>
                    </div>
                    <div className="rounded-lg border border-gray-100 dark:border-gray-700 bg-gray-50/70 dark:bg-gray-900/30 px-3 py-2.5">
                      <div className="text-[11px] text-gray-500">默认账号</div>
                      <div className="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
                        {currentFeishuDefaultAccount || '未设置'}
                      </div>
                    </div>
                    <div className="rounded-lg border border-gray-100 dark:border-gray-700 bg-gray-50/70 dark:bg-gray-900/30 px-3 py-2.5">
                      <div className="text-[11px] text-gray-500">顶层镜像来源</div>
                      <div className="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
                        {currentFeishuDefaultAccount || '未设置'}
                      </div>
                    </div>
                    <div className="rounded-lg border border-gray-100 dark:border-gray-700 bg-gray-50/70 dark:bg-gray-900/30 px-3 py-2.5">
                      <div className="text-[11px] text-gray-500">已启用账号数</div>
                      <div className="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
                        {currentFeishuEnabledCount} / {Math.max(currentFeishuAccounts.length, currentFeishuDefaultAccount ? 1 : 0)}
                      </div>
                    </div>
                  </div>

                  <div className="rounded-xl border border-sky-100 bg-sky-50 px-4 py-3 text-[12px] leading-relaxed text-sky-700 dark:border-sky-900/40 dark:bg-sky-950/20 dark:text-sky-200">
                    {currentFeishuAccountBoundaryHint}
                  </div>

                  {!feishuAdvancedAccounts ? (
                    <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                      <div>
                        <h4 className="text-sm font-semibold text-gray-900 dark:text-white">QQBot 通道版本</h4>
                        <p className="text-xs text-gray-500 mt-1 leading-relaxed">
                          {currentFeishuHasStoredAccounts
                            ? `当前仍保留 ${currentFeishuAccounts.length} 个账号；仅默认账号视图只会编辑默认账号 ${currentFeishuDefaultAccount || 'default'}，其他账号会继续保留，但保存时会自动标记为 enabled=false。`
                            : '维护一套共享 App ID / App Secret 即可，适合只接一个飞书机器人。'}
                        </p>
                        <p className="text-[11px] text-gray-500 mt-1">{currentFeishuVariantHint}</p>
                      </div>
                      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                        <div>
                          <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300 mb-1.5">机器人名称</label>
                          <input
                            value={String(getFeishuAccountEntry(currentFeishuConfig, currentFeishuDefaultAccount).botName || '')}
                            onChange={e => handleFeishuSimpleFieldChange('botName', e.target.value)}
                            placeholder="例如 主机器人"
                            className="w-full px-3.5 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                          />
                        </div>
                        <div>
                          <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300 mb-1.5">App ID</label>
                          <input
                            value={currentFeishuSimpleCredentials.appId}
                            onChange={e => handleFeishuSimpleFieldChange('appId', e.target.value)}
                            placeholder="cli_xxx"
                            className="w-full px-3.5 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                          />
                        </div>
                        <div>
                          <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300 mb-1.5">App Secret</label>
                          <input
                            type="password"
                            value={currentFeishuSimpleCredentials.appSecret}
                            onChange={e => handleFeishuSimpleFieldChange('appSecret', e.target.value)}
                            placeholder="请输入 App Secret"
                            className="w-full px-3.5 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                          />
                        </div>
                      </div>
                      <p className="text-[11px] text-gray-500 leading-relaxed">
                        如果后续需要给不同飞书机器人分配不同 Agent，再切到“多账号并行”并手动启用需要参与运行的账号。
                      </p>
                    </div>
                  ) : (
                    <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                      <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
                        <div>
                          <h4 className="text-sm font-semibold text-gray-900 dark:text-white">账号管理</h4>
                          <p className="text-xs text-gray-500 mt-1 leading-relaxed">
                            多账号并行时，每个 Account 都可以单独控制 <span className="font-mono">enabled</span>；默认账号会命中 Agent 路由里留空的 <span className="font-mono">accountId</span>。
                          </p>
                          <p className="text-[11px] text-gray-500 mt-1">{currentFeishuVariantHint}</p>
                        </div>
                        <div className="flex flex-col sm:flex-row gap-2">
                          <input
                            value={feishuNewAccountId}
                            onChange={e => setFeishuNewAccountId(e.target.value)}
                            placeholder="新 Account ID，例如 backup"
                            className="px-3 py-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                          />
                          <button
                            type="button"
                            onClick={handleAddFeishuAccount}
                            className="px-3 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700"
                          >
                            添加账号
                          </button>
                        </div>
                      </div>

                      <div className="grid grid-cols-1 md:grid-cols-[240px_minmax(0,1fr)] gap-4">
                        <div className="space-y-3">
                          <div className="rounded-lg border border-gray-100 dark:border-gray-700 bg-gray-50/70 dark:bg-gray-900/30 px-3 py-3">
                            <div className="text-xs font-semibold text-gray-900 dark:text-white">默认账号</div>
                            <div className="mt-1 text-sm font-semibold text-violet-700 dark:text-violet-300">
                              {currentFeishuDefaultAccount || '未设置'}
                            </div>
                            <p className="mt-1 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
                              顶层 <span className="font-mono">appId/appSecret</span> 会始终镜像这个账号；
                              Agent 页面会读取 <span className="font-mono">defaultAccount</span> 来补全留空的 <span className="font-mono">accountId</span>。
                            </p>
                          </div>
                          <div className="space-y-2">
                            {currentFeishuAccounts.map(accountId => {
                              const accountEntry = getFeishuAccountEntry(currentFeishuConfig, accountId);
                              const hasCredentials = !!String(accountEntry.appId || '').trim() && !!String(accountEntry.appSecret || '').trim();
                              const enabled = isFeishuAccountEnabled(currentFeishuConfig, accountId);
                              return (
                                <button
                                  key={accountId}
                                  type="button"
                                  onClick={() => setFeishuActiveAccountId(accountId)}
                                  className={`w-full flex items-center justify-between gap-3 px-3 py-2.5 rounded-lg border text-left text-sm transition-colors ${
                                    currentFeishuEditingAccountId === accountId
                                      ? 'border-violet-500 bg-violet-50 dark:bg-violet-900/20 text-violet-700 dark:text-violet-300'
                                      : 'border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 text-gray-700 dark:text-gray-200 hover:border-violet-300'
                                  }`}
                                >
                                  <div className="min-w-0">
                                    <div className="truncate font-medium">{accountId}</div>
                                    <div className="mt-0.5 text-[11px] text-gray-500 dark:text-gray-400">
                                      {String(accountEntry.botName || '').trim() || (hasCredentials ? '已填写凭证' : '待填写凭证')}
                                    </div>
                                  </div>
                                  <div className="shrink-0 flex flex-wrap items-center justify-end gap-1">
                                    {enabled && (
                                      <span className="text-[10px] px-2 py-0.5 rounded-full bg-emerald-100 dark:bg-emerald-900/40 text-emerald-600 dark:text-emerald-300">启用</span>
                                    )}
                                    {accountId === currentFeishuDefaultAccount && (
                                      <span className="text-[10px] px-2 py-0.5 rounded-full bg-violet-100 dark:bg-violet-900/40 text-violet-600 dark:text-violet-300">默认</span>
                                    )}
                                  </div>
                                </button>
                              );
                            })}
                          </div>
                        </div>

                        <div className="rounded-lg border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                            <div>
                              <div className="text-sm font-semibold text-gray-900 dark:text-white">当前账号：{currentFeishuEditingAccountId}</div>
                              <p className="text-xs text-gray-500 mt-1 leading-relaxed">
                                保存时，如果它是默认账号，会自动同步到顶层 <span className="font-mono">appId/appSecret</span>；默认账号不能被禁用。
                              </p>
                            </div>
                            <div className="flex flex-wrap items-center gap-2">
                              {currentFeishuEditingAccountId !== currentFeishuDefaultAccount && (
                                <button
                                  type="button"
                                  onClick={() => handleFeishuDefaultAccountChange(currentFeishuEditingAccountId)}
                                  disabled={!hasFeishuRunnableCredentials(currentFeishuAccountConfig) && currentFeishuRunnableAccounts.some(id => id !== currentFeishuEditingAccountId)}
                                  className="px-3 py-2 text-xs rounded-lg border border-violet-200 text-violet-700 hover:bg-violet-50 disabled:opacity-50 disabled:cursor-not-allowed dark:border-violet-900/40 dark:text-violet-300 dark:hover:bg-violet-900/20"
                                >
                                  设为默认
                                </button>
                              )}
                              <button
                                type="button"
                                onClick={() => handleRemoveFeishuAccount(currentFeishuEditingAccountId)}
                                className="px-3 py-2 text-xs rounded-lg border border-red-200 text-red-600 hover:bg-red-50 dark:border-red-900/40 dark:text-red-300 dark:hover:bg-red-900/20"
                              >
                                删除当前账号
                              </button>
                            </div>
                          </div>

                          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div>
                              <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300 mb-1.5">机器人名称</label>
                              <input
                                value={String(currentFeishuAccountConfig.botName || '')}
                                onChange={e => handleFeishuAccountFieldChange(currentFeishuEditingAccountId, 'botName', e.target.value)}
                                placeholder="例如 备用机器人"
                                className="w-full px-3.5 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                              />
                            </div>
                            <div className="flex items-end">
                              <label className="w-full rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50/70 dark:bg-gray-900/30 px-3.5 py-3 flex items-center justify-between gap-3">
                                <div>
                                  <div className="text-xs font-semibold text-gray-700 dark:text-gray-300">参与运行</div>
                                  <div className="mt-1 text-[11px] text-gray-500 dark:text-gray-400">
                                    {currentFeishuEditingAccountId === currentFeishuDefaultAccount ? '默认账号固定启用' : '关闭后账号仍保留，但不会参与多账号并行'}
                                  </div>
                                </div>
                                <input
                                  type="checkbox"
                                  className="h-4 w-4 accent-violet-600"
                                  checked={isFeishuAccountEnabled(currentFeishuConfig, currentFeishuEditingAccountId)}
                                  disabled={currentFeishuEditingAccountId === currentFeishuDefaultAccount}
                                  onChange={e => handleFeishuAccountEnabledChange(currentFeishuEditingAccountId, e.target.checked)}
                                />
                              </label>
                            </div>
                            <div>
                              <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300 mb-1.5">App ID</label>
                              <input
                                value={String(currentFeishuAccountConfig.appId || '')}
                                onChange={e => handleFeishuAccountFieldChange(currentFeishuEditingAccountId, 'appId', e.target.value)}
                                placeholder="cli_xxx"
                                className="w-full px-3.5 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                              />
                            </div>
                            <div>
                              <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300 mb-1.5">App Secret</label>
                              <input
                                type="password"
                                value={String(currentFeishuAccountConfig.appSecret || '')}
                                onChange={e => handleFeishuAccountFieldChange(currentFeishuEditingAccountId, 'appSecret', e.target.value)}
                                placeholder="请输入 App Secret"
                                className="w-full px-3.5 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900"
                              />
                            </div>
                          </div>

                          <div className="rounded-lg border border-sky-100 dark:border-sky-900/40 bg-sky-50/60 dark:bg-sky-950/20 px-3 py-3 text-[11px] leading-relaxed text-sky-800 dark:text-sky-200">
                            未填写 <span className="font-mono">accountId</span> 的 Agent 路由会命中默认账号。
                            切回“仅默认账号”时，已有账号数据会继续保留，不会被删除，只会统一切成 <span className="font-mono">enabled=false</span>。
                          </div>
                        </div>
                      </div>
                    </div>
                  )}
                </div>
              )}

              {currentDef.id === 'feishu' && (
                <div className="rounded-xl border border-sky-200 dark:border-sky-800/40 bg-sky-50/50 dark:bg-sky-950/10 p-4 space-y-4">
                  <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
                    <div className="space-y-1.5">
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">私聊上下文隔离诊断</h4>
                      <p className="text-xs text-gray-600 dark:text-gray-300 leading-relaxed">
                        飞书私聊的上下文隔离由全局配置控制，如果未写入则运行时等价于"main"模式。
                        <InfoTooltip content={<>真正生效的字段是顶层 <code className="font-mono text-[11px]">session.dmScope</code>，而非 <code className="font-mono text-[11px]">channels.feishu.dmScope</code>（后者已弃用）。</>} />
                      </p>
                    </div>
                    <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 xl:w-[540px]">
                      <div className="rounded-lg border border-white/80 dark:border-slate-800 bg-white/80 dark:bg-slate-900/50 px-3 py-2.5">
                        <div className="text-[11px] text-gray-500">配置文件</div>
                        <div className="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
                          {currentConfiguredFeishuDmScope || '未设置'}
                        </div>
                      </div>
                      <div className="rounded-lg border border-white/80 dark:border-slate-800 bg-white/80 dark:bg-slate-900/50 px-3 py-2.5">
                        <div className="text-[11px] text-gray-500">当前生效</div>
                        <div className="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
                          {currentEffectiveFeishuDmScope}
                        </div>
                      </div>
                      <div className="rounded-lg border border-white/80 dark:border-slate-800 bg-white/80 dark:bg-slate-900/50 px-3 py-2.5">
                        <div className="text-[11px] text-gray-500">推荐值</div>
                        <div className="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
                          {feishuDmDiagnosis?.recommendedDmScope || 'per-account-channel-peer'}
                        </div>
                      </div>
                    </div>
                  </div>

                  <div className="rounded-xl border border-sky-100 dark:border-sky-900/40 bg-white/75 dark:bg-slate-900/40 px-4 py-3 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                    <div className="text-[12px] leading-relaxed text-sky-900 dark:text-sky-100">
                      <div className="font-semibold">配置入口已收敛到系统设置</div>
                      <div className="mt-1 text-sky-800/90 dark:text-sky-100/85">
                        私聊隔离是全局配置，这里仅保留诊断视图。
                        真正的编辑入口请前往 <span className="font-medium">系统配置 &gt; 通用配置 &gt; 私聊上下文隔离</span>。
                        <InfoTooltip content={<>对应字段 <code className="font-mono text-[11px]">session.dmScope</code>，是全局配置而非通道级配置。</>} />
                      </div>
                    </div>
                    <button
                      type="button"
                      onClick={() => navigate('/config?tab=general')}
                      className={`${modern ? 'page-modern-accent px-4 py-2 text-xs font-medium' : 'inline-flex items-center gap-2 px-4 py-2 text-xs font-medium rounded-lg bg-sky-600 text-white hover:bg-sky-700 transition-colors shadow-sm'}`}
                    >
                      前往系统设置
                    </button>
                  </div>

                  {feishuDmDiagnosis?.unsupportedChannelDmScope && (
                    <div className="rounded-lg border border-amber-200 dark:border-amber-800/40 bg-amber-50/80 dark:bg-amber-900/10 px-4 py-3 text-xs text-amber-700 dark:text-amber-300 leading-relaxed flex items-center gap-1.5 flex-wrap">
                      检测到已弃用的通道级隔离配置（值：{feishuDmDiagnosis.unsupportedChannelDmScope}），请改用系统设置中的全局私聊隔离。
                      <InfoTooltip content={<>检测到 <code className="font-mono text-[11px]">channels.feishu.dmScope = {feishuDmDiagnosis.unsupportedChannelDmScope}</code>，该字段不是当前 OpenClaw 的有效 schema，请改用 <code className="font-mono text-[11px]">session.dmScope</code>。</>} />
                    </div>
                  )}

                  <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3">
                    <div className="rounded-lg border border-white/80 dark:border-slate-800 bg-white/80 dark:bg-slate-900/50 p-3">
                      <div className="text-xs font-semibold text-gray-900 dark:text-white">账号视角</div>
                      <p className="mt-1 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
                        已检测到账户 {feishuDmDiagnosis?.accountCount || 0} 个
                        {feishuDmDiagnosis?.defaultAccount ? `，默认账号为 ${feishuDmDiagnosis.defaultAccount}` : ''}。
                      </p>
                      {!!feishuDmDiagnosis?.accountIds?.length && (
                        <div className="mt-2 flex flex-wrap gap-2">
                          {feishuDmDiagnosis.accountIds.map(accountId => (
                            <span key={accountId} className="px-2 py-1 rounded-full border border-sky-100 dark:border-sky-900/40 bg-sky-50 dark:bg-sky-900/20 text-[11px] text-sky-700 dark:text-sky-300 font-mono">
                              {accountId}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>
                    <div className="rounded-lg border border-white/80 dark:border-slate-800 bg-white/80 dark:bg-slate-900/50 p-3">
                      <div className="text-xs font-semibold text-gray-900 dark:text-white">运行中会话</div>
                      {loadingFeishuDmDiagnosis ? (
                        <div className="mt-2 inline-flex items-center gap-2 text-[11px] text-gray-500">
                          <Loader2 size={13} className="animate-spin" />
                          正在读取当前会话索引…
                        </div>
                      ) : feishuDmDiagnosis?.sessionIndexExists ? (
                        <>
                          <p className="mt-1 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
                            已在
                            <span className="mx-1 font-mono">{feishuDmDiagnosis.scannedAgentIds?.join(', ') || feishuDmDiagnosis.defaultAgent}</span>
                            的会话索引中检测到
                            <span className="mx-1 font-mono">{feishuDmDiagnosis.feishuSessionCount || 0}</span> 个飞书会话。
                          </p>
                          {!!feishuDmDiagnosis?.feishuSessionKeys?.length && (
                            <div className="mt-2 space-y-1">
                              {feishuDmDiagnosis.feishuSessionKeys.slice(0, 3).map(sessionKey => (
                                <div key={sessionKey} className="rounded-md bg-gray-50 dark:bg-slate-950/40 px-2.5 py-1.5 text-[11px] text-gray-600 dark:text-gray-300 font-mono break-all">
                                  {sessionKey}
                                </div>
                              ))}
                            </div>
                          )}
                        </>
                      ) : (
                        <p className="mt-1 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
                          当前尚未读取到 <span className="font-mono">sessions.json</span>；保存后可通过新私聊或重置会话来观察分桶结果。
                        </p>
                      )}
                    </div>
                    <div className="rounded-lg border border-white/80 dark:border-slate-800 bg-white/80 dark:bg-slate-900/50 p-3">
                      <div className="text-xs font-semibold text-gray-900 dark:text-white">落地提示</div>
                      <p className="mt-1 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
                        {feishuDmDiagnosis?.hasSharedMainSessionKey
                          ? <>当前仍存在共享主会话键 <span className="font-mono">{feishuDmDiagnosis.mainSessionKey}</span>。写入新值后，新的私聊消息会按新键创建；旧会话如需立即拆分，需重置对应会话或重启后观察。</>
                          : <>当前未检测到共享主会话键。若已写入推荐值，新的飞书私聊应按账号 / 渠道 / 对端拆分。</>}
                      </p>
                    </div>
                  </div>

                  <div className="rounded-xl border border-sky-100 dark:border-sky-900/40 bg-white/75 dark:bg-slate-900/40 p-4 space-y-3">
                    <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
                      <div className="space-y-1.5">
                        <h4 className="text-sm font-semibold text-gray-900 dark:text-white">配对授权状态</h4>
                        <p className="text-xs text-gray-600 dark:text-gray-300 leading-relaxed">
                          <span className="font-mono">openclaw pairing list feishu</span> 只显示待审批请求；
                          已批准的用户会落到 <span className="font-mono">credentials/feishu-*-allowFrom.json</span>。
                        </p>
                      </div>
                      <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 xl:w-[360px]">
                        <div className="rounded-lg border border-white/80 dark:border-slate-800 bg-white/80 dark:bg-slate-900/50 px-3 py-2.5">
                          <div className="text-[11px] text-gray-500">待审批请求</div>
                          <div className="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
                            {feishuDmDiagnosis?.pendingPairingCount || 0}
                          </div>
                        </div>
                        <div className="rounded-lg border border-white/80 dark:border-slate-800 bg-white/80 dark:bg-slate-900/50 px-3 py-2.5">
                          <div className="text-[11px] text-gray-500">已授权 OpenID</div>
                          <div className="mt-1 text-sm font-semibold text-gray-900 dark:text-white">
                            {feishuDmDiagnosis?.authorizedSenderCount || 0}
                          </div>
                        </div>
                      </div>
                    </div>

                    {loadingFeishuDmDiagnosis ? (
                      <div className="inline-flex items-center gap-2 text-[11px] text-gray-500">
                        <Loader2 size={13} className="animate-spin" />
                        正在读取飞书授权名单…
                      </div>
                    ) : feishuAuthorizedSenders.length > 0 ? (
                      <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
                        {feishuAuthorizedSenders.map(bucket => (
                          <div key={bucket.accountId} className="rounded-lg border border-white/80 dark:border-slate-800 bg-slate-50/70 dark:bg-slate-950/30 p-3 space-y-2">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="text-xs font-semibold text-gray-900 dark:text-white">账号</span>
                              <span className="px-2 py-1 rounded-full border border-sky-100 dark:border-sky-900/40 bg-sky-50 dark:bg-sky-900/20 text-[11px] text-sky-700 dark:text-sky-300 font-mono">
                                {bucket.accountId}
                              </span>
                              {!bucket.accountConfigured && (
                                <span className="px-2 py-1 rounded-full border border-amber-200 dark:border-amber-800/40 bg-amber-50 dark:bg-amber-900/20 text-[11px] text-amber-700 dark:text-amber-300">
                                  配置中未声明
                                </span>
                              )}
                              <span className="text-[11px] text-gray-500">
                                {bucket.senderCount || 0} 个已授权 OpenID
                              </span>
                            </div>
                            {Array.isArray(bucket.senderIds) && bucket.senderIds.length > 0 ? (
                              <div className="space-y-1">
                                {bucket.senderIds.map(senderId => (
                                  <div key={senderId} className="rounded-md bg-white dark:bg-slate-900 px-2.5 py-1.5 text-[11px] text-gray-700 dark:text-gray-300 font-mono break-all">
                                    {senderId}
                                  </div>
                                ))}
                              </div>
                            ) : (
                              <div className="text-[11px] text-gray-500 dark:text-gray-400">
                                当前未检测到该账号的已授权 OpenID。
                              </div>
                            )}
                            {Array.isArray(bucket.sourceFiles) && bucket.sourceFiles.length > 0 && (
                              <div className="rounded-md bg-slate-100/80 dark:bg-slate-900/60 px-2.5 py-2 text-[11px] text-gray-500 dark:text-gray-400">
                                来源文件：
                                <div className="mt-1 space-y-1">
                                  {bucket.sourceFiles.map(sourceFile => (
                                    <div key={sourceFile} className="font-mono break-all text-gray-600 dark:text-gray-300">
                                      {sourceFile}
                                    </div>
                                  ))}
                                </div>
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className="rounded-lg border border-dashed border-slate-200 dark:border-slate-800 px-3 py-3 text-[11px] leading-relaxed text-gray-500 dark:text-gray-400">
                        当前未检测到飞书已授权 OpenID。首次私聊触发配对后，审批成功的用户会显示在这里。
                      </div>
                    )}
                  </div>
                </div>
              )}

              {currentDef.id === 'feishu' && hasFeishuGroupAllowlistConflict && (
                <div className="rounded-lg border border-amber-200 dark:border-amber-800/40 bg-amber-50/70 dark:bg-amber-900/10 px-4 py-3 text-xs text-amber-700 dark:text-amber-300 leading-relaxed">
                  当前 <span className="font-mono">groupPolicy</span> 为 <span className="font-mono">{currentFeishuGroupPolicy || '未配置（默认 open）'}</span>，
                  <span className="mx-1 font-mono">groupAllowFrom</span>
                  仅在 <span className="font-mono">allowlist</span> 模式下生效。若保持当前策略并保存，白名单会被自动清理。
                </div>
              )}

              {currentDef.id === 'telegram' && String(getEffectiveChannelConfig('telegram')?.dmPolicy || 'pairing') === 'pairing' && (
                <div className="rounded-lg border border-amber-200 dark:border-amber-800/40 bg-amber-50/80 dark:bg-amber-900/10 px-4 py-3 text-xs text-amber-700 dark:text-amber-300 leading-relaxed space-y-1.5">
                  <div className="font-semibold text-amber-900 dark:text-amber-100">Telegram 当前处于配对模式</div>
                  <div>首次私聊机器人时，OpenClaw 会返回 pairing code；现在可以直接在上方“统一配对审批”里查看并批准。</div>
                  <div>如果需要排查底层状态，仍可在服务器执行 <span className="font-mono">openclaw pairing list telegram</span> 与 <span className="font-mono">openclaw pairing approve telegram &lt;code&gt;</span>。</div>
                  <div>如果你不想走配对流程，可把“私聊准入策略”改成 <span className="font-mono">open</span>。</div>
                </div>
              )}

              {currentDef.id === 'openclaw-weixin' && (
                <div className="rounded-lg border border-blue-200 dark:border-blue-800/40 bg-blue-50/80 dark:bg-blue-900/10 px-4 py-3 text-xs text-blue-700 dark:text-blue-300 leading-relaxed space-y-1.5">
                  <div className="font-semibold text-blue-900 dark:text-blue-100">腾讯官方微信通道接入步骤</div>
                  <div>现在可以直接在面板里生成登录二维码；底层仍然使用 OpenClaw 官方微信通道的扫码登录流程。</div>
                  <div>建议顺序：先安装插件并启用，再点击上方“二维码登录”完成扫码，确认后面板会自动刷新账号状态。</div>
                  <div>如果你准备接多个微信账号，建议再执行 <span className="font-mono">openclaw config set session.dmScope per-account-channel-peer</span>，避免不同账号私聊上下文串线。</div>
                </div>
              )}

              {currentDef.id === 'openclaw-weixin' && (
                <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">{'\u5fae\u4fe1\u767b\u5f55\u72b6\u6001'}</h4>
                      <p className="mt-1 text-xs text-gray-500 dark:text-gray-400 leading-relaxed">
                        {'\u5728\u8fd9\u91cc\u5237\u65b0\u626b\u7801\u4e0e\u8d26\u53f7\u72b6\u6001\uff0c\u627e\u4e0d\u5230\u4e8c\u7ef4\u7801\u65f6\u53ef\u4ee5\u91cd\u65b0\u751f\u6210\u3002'}
                      </p>
                    </div>
                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        onClick={() => loadOpenClawWeixinStatus()}
                        className={`${modern ? 'page-modern-action px-3 py-1.5 text-xs' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-200 hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors'}`}
                      >
                        <RefreshCw size={12} />
                        刷新状态
                      </button>
                      <button
                        type="button"
                        onClick={() => handleQRLogin('openclaw-weixin')}
                        className={`${modern ? 'page-modern-accent px-3 py-1.5 text-xs' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg bg-blue-600 text-white hover:bg-blue-700 transition-colors'}`}
                      >
                        <QrCode size={12} />
                        新增登录
                      </button>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 xl:grid-cols-4 gap-3">
                    <div className="rounded-lg border border-gray-100 dark:border-gray-800 bg-gray-50/60 dark:bg-gray-900/30 px-3 py-3">
                      <div className="text-[11px] text-gray-500 dark:text-gray-400">插件安装</div>
                      <div className={`mt-1 text-sm font-semibold ${openClawWeixinStatus?.pluginInstalled ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'}`}>
                        {openClawWeixinStatus?.pluginInstalled ? '已安装' : '未安装'}
                      </div>
                    </div>
                    <div className="rounded-lg border border-gray-100 dark:border-gray-800 bg-gray-50/60 dark:bg-gray-900/30 px-3 py-3">
                      <div className="text-[11px] text-gray-500 dark:text-gray-400">插件启用</div>
                      <div className={`mt-1 text-sm font-semibold ${openClawWeixinStatus?.enabled ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-600 dark:text-amber-400'}`}>
                        {openClawWeixinStatus?.enabled ? '已启用' : '未启用'}
                      </div>
                    </div>
                    <div className="rounded-lg border border-gray-100 dark:border-gray-800 bg-gray-50/60 dark:bg-gray-900/30 px-3 py-3">
                      <div className="text-[11px] text-gray-500 dark:text-gray-400">已登录账号</div>
                      <div className="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{openClawWeixinAccounts.length}</div>
                    </div>
                    <div className="rounded-lg border border-gray-100 dark:border-gray-800 bg-gray-50/60 dark:bg-gray-900/30 px-3 py-3">
                      <div className="text-[11px] text-gray-500 dark:text-gray-400">待处理登录</div>
                      <div className="mt-1 text-sm font-semibold text-gray-900 dark:text-white">{Number(openClawWeixinStatus?.pendingLogins || 0)}</div>
                    </div>
                  </div>

                  {openClawWeixinAccounts.length > 0 ? (
                    <div className="space-y-3">
                      {openClawWeixinAccounts.map((account: any) => (
                        <div key={account.accountId} className="rounded-xl border border-gray-100 dark:border-gray-800 bg-white dark:bg-gray-900/40 px-4 py-4">
                          <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                            <div className="min-w-0">
                              <div className="flex flex-wrap items-center gap-2">
                                <div className="text-sm font-semibold text-gray-900 dark:text-white">
                                  {account.name || account.accountId}
                                </div>
                                <span className={`px-2 py-0.5 rounded-full text-[10px] font-semibold ${account.configured ? 'bg-emerald-100 dark:bg-emerald-900/40 text-emerald-700 dark:text-emerald-300' : 'bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-300'}`}>
                                  {account.configured ? '已登录' : '未完成'}
                                </span>
                                <span className={`px-2 py-0.5 rounded-full text-[10px] font-semibold ${account.enabled ? 'bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-300' : 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-300'}`}>
                                  {account.enabled ? '启用中' : '已禁用'}
                                </span>
                              </div>
                              <div className="mt-2 grid grid-cols-1 md:grid-cols-2 gap-2 text-xs text-gray-500 dark:text-gray-400">
                                <div>账号 ID：<span className="font-mono text-gray-700 dark:text-gray-200">{account.accountId}</span></div>
                                <div>用户 ID：<span className="font-mono text-gray-700 dark:text-gray-200">{account.userId || '-'}</span></div>
                                <div className="md:col-span-2">Base URL：<span className="font-mono text-gray-700 dark:text-gray-200 break-all">{account.baseUrl || '-'}</span></div>
                                <div className="md:col-span-2">CDN：<span className="font-mono text-gray-700 dark:text-gray-200 break-all">{account.cdnBaseUrl || '-'}</span></div>
                                <div>最近保存：<span className="font-mono text-gray-700 dark:text-gray-200">{account.savedAt || '-'}</span></div>
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              <button
                                type="button"
                                onClick={() => handleQRLogin('openclaw-weixin')}
                                className={`${modern ? 'page-modern-accent px-3 py-1.5 text-xs' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/50 transition-colors'}`}
                              >
                                <RefreshCw size={12} />
                                重新扫码
                              </button>
                              <button
                                type="button"
                                onClick={() => handleOpenClawWeixinLogout(String(account.accountId || ''))}
                                disabled={loggingOutWeixinAccount === account.accountId}
                                className={`${modern ? 'page-modern-danger px-3 py-1.5 text-xs' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg bg-red-50 dark:bg-red-900/30 text-red-600 dark:text-red-400 hover:bg-red-100 dark:hover:bg-red-900/50 disabled:opacity-50 transition-colors'}`}
                              >
                                {loggingOutWeixinAccount === account.accountId ? <Loader2 size={12} className="animate-spin" /> : <LogOut size={12} />}
                                退出登录
                              </button>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="rounded-lg border border-dashed border-gray-200 dark:border-gray-700 px-4 py-5 text-xs text-gray-500 dark:text-gray-400">
                      当前还没有已登录的微信账号。点击上方“新增登录”即可在面板里生成二维码并扫码接入。
                    </div>
                  )}
                </div>
              )}

              {currentDef && currentChannelPairingEnabled && (
                <div className="rounded-xl border border-amber-200 dark:border-amber-800/40 bg-amber-50/60 dark:bg-amber-900/10 p-4 space-y-4">
                  <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                    <div>
                      <h4 className="text-sm font-semibold text-amber-900 dark:text-amber-100">统一配对审批</h4>
                      <p className="mt-1 text-xs leading-relaxed text-amber-700 dark:text-amber-300">
                        当前通道启用了 <span className="font-mono">dmPolicy=pairing</span>。用户首次私聊时会先拿到 pairing code，需要在这里批准后才能继续对话。
                        {resolvePairingAccountId(currentDef.id) ? <> 当前按账号 <span className="font-mono">{resolvePairingAccountId(currentDef.id)}</span> 过滤。</> : null}
                      </p>
                    </div>
                    <button
                      type="button"
                      onClick={() => loadPairingRequests(currentDef.id, resolvePairingAccountId(currentDef.id) || undefined)}
                      disabled={loadingPairingRequests}
                      className={`${modern ? 'page-modern-action px-3 py-1.5 text-xs' : 'inline-flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg bg-white/80 dark:bg-gray-900/40 text-amber-800 dark:text-amber-200 border border-amber-200/80 dark:border-amber-800/40 hover:bg-white dark:hover:bg-gray-900 disabled:opacity-50 transition-colors'}`}
                    >
                      {loadingPairingRequests ? <Loader2 size={12} className="animate-spin" /> : <RefreshCw size={12} />}
                      刷新待审批
                    </button>
                  </div>

                  {pairingRequests.length > 0 ? (
                    <div className="space-y-3">
                      {pairingRequests.map(request => (
                        <div key={`${request.code}:${request.id}`} className="rounded-xl border border-amber-200/80 dark:border-amber-800/30 bg-white/80 dark:bg-gray-950/20 px-4 py-4">
                          <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
                            <div className="min-w-0">
                              <div className="flex flex-wrap items-center gap-2">
                                <div className="text-sm font-semibold text-gray-900 dark:text-white">
                                  配对码 <span className="font-mono">{request.code}</span>
                                </div>
                                <span className="px-2 py-0.5 rounded-full bg-amber-100 dark:bg-amber-900/30 text-[10px] font-semibold text-amber-700 dark:text-amber-300">
                                  待审批
                                </span>
                              </div>
                              <div className="mt-2 grid grid-cols-1 md:grid-cols-2 gap-2 text-xs text-gray-500 dark:text-gray-400">
                                <div>发送者：<span className="font-mono text-gray-700 dark:text-gray-200 break-all">{request.id}</span></div>
                                <div>创建时间：<span className="font-mono text-gray-700 dark:text-gray-200">{request.createdAt || '-'}</span></div>
                                {request.lastSeenAt && (
                                  <div>最近出现：<span className="font-mono text-gray-700 dark:text-gray-200">{request.lastSeenAt}</span></div>
                                )}
                                {Object.entries(request.meta || {}).map(([key, value]) => (
                                  <div key={key}>
                                    {key}：<span className="font-mono text-gray-700 dark:text-gray-200 break-all">{String(value || '-')}</span>
                                  </div>
                                ))}
                              </div>
                            </div>
                            <button
                              type="button"
                              onClick={() => handleApprovePairingCode(currentDef.id, request.code, resolvePairingAccountId(currentDef.id) || undefined)}
                              disabled={approvingPairingCode === request.code}
                              className={`${modern ? 'page-modern-accent px-3 py-1.5 text-xs' : 'inline-flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg bg-amber-600 text-white hover:bg-amber-700 disabled:opacity-50 transition-colors'}`}
                            >
                              {approvingPairingCode === request.code ? <Loader2 size={12} className="animate-spin" /> : <CheckCircle size={12} />}
                              批准配对
                            </button>
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="rounded-lg border border-dashed border-amber-200/80 dark:border-amber-800/30 px-4 py-5 text-xs text-amber-700 dark:text-amber-300">
                      {loadingPairingRequests ? '正在加载待审批 pairing 请求…' : '当前没有待审批的 pairing code。'}
                    </div>
                  )}
                </div>
              )}

              {currentDef.id === 'feishu' && String(getEffectiveChannelConfig('feishu')?.dmPolicy || 'pairing') === 'pairing' && (
                <div className="rounded-lg border border-amber-200 dark:border-amber-800/40 bg-amber-50/80 dark:bg-amber-900/10 px-4 py-3 text-xs text-amber-700 dark:text-amber-300 leading-relaxed space-y-1.5">
                  <div className="font-semibold text-amber-900 dark:text-amber-100">飞书当前处于配对模式</div>
                  <div>首次私聊机器人时，OpenClaw 会返回 pairing code。Lite 版保存有凭证时会优先写成免配对模式；如果你看到这里，重新保存一次配置通常就会改成免配对。</div>
                  <div>现在可以直接在上方“统一配对审批”中查看并批准请求；只有在需要排查底层文件时才需要回 CLI。</div>
                </div>
              )}

              {(currentDef.id === 'wecom' || currentDef.id === 'wecom-app') && (
                <div className="rounded-lg border border-blue-200 dark:border-blue-800/40 bg-blue-50/80 dark:bg-blue-900/10 px-4 py-3 text-xs text-blue-700 dark:text-blue-300 leading-relaxed">
                  <span className="font-semibold">企业微信两种模式互斥：</span>智能机器人与自建应用共用同一 channel，启用其中一个会自动关闭另一个。
                </div>
              )}

              {currentDef.id === 'wecom' && String(getEffectiveChannelConfig('wecom')?.dmPolicy || 'pairing') === 'pairing' && (
                <div className="rounded-lg border border-amber-200 dark:border-amber-800/40 bg-amber-50/80 dark:bg-amber-900/10 px-4 py-3 text-xs text-amber-700 dark:text-amber-300 leading-relaxed space-y-1.5">
                  <div className="font-semibold text-amber-900 dark:text-amber-100">企业微信当前处于配对模式</div>
                  <div>Lite 版保存 Bot ID 和 Secret 时会优先写成免配对模式；如果你看到这里，重新保存一次配置通常就会改成免配对。</div>
                  <div>现在可以直接在上方“统一配对审批”中查看并批准请求；只有在需要排查底层文件时才需要回 CLI。</div>
                </div>
              )}

              {currentDef.id === 'wecom' && (
                <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                  <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900 dark:text-white">连接方式</h4>
                      <p className="mt-1 text-xs text-gray-500 dark:text-gray-400 leading-relaxed">
                        企业微信智能机器人支持两种接入方式：
                        <span className="font-medium"> 使用 URL 回调 </span>
                        与
                        <span className="font-medium"> 使用长连接 </span>。
                        现在可在同一页面切换，已填写的另一种方式配置会继续保留。
                      </p>
                      <a
                        href="https://open.work.weixin.qq.com/help2/pc/cat?doc_id=21661"
                        target="_blank"
                        rel="noreferrer"
                        className="mt-2 inline-flex text-xs text-sky-600 hover:text-sky-700 dark:text-sky-400 dark:hover:text-sky-300"
                      >
                        查看企业微信官方配置指引
                      </a>
                    </div>
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 xl:w-[620px]">
                      <button
                        type="button"
                        onClick={() => handleWecomModeChange('long-polling')}
                        className={`rounded-xl border p-4 text-left transition ${currentWecomMode === 'long-polling' ? 'border-sky-500 bg-sky-50/80 dark:bg-sky-950/20 shadow-sm' : 'border-gray-200 dark:border-gray-700 hover:border-sky-300 dark:hover:border-sky-700 bg-white dark:bg-gray-900'}`}
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div>
                            <div className="text-sm font-semibold text-gray-900 dark:text-white">使用长连接</div>
                            <div className="mt-1 text-xs leading-relaxed text-gray-500 dark:text-gray-400">无需公网域名或固定回调 URL，配置 Bot ID 和 Secret 即可建立长连接。</div>
                          </div>
                          <div className={`mt-1 h-4 w-4 rounded-full border ${currentWecomMode === 'long-polling' ? 'border-sky-500 bg-sky-500' : 'border-gray-300 dark:border-gray-600'}`}>
                            <div className={`m-[3px] h-2 w-2 rounded-full bg-white ${currentWecomMode === 'long-polling' ? 'opacity-100' : 'opacity-0'}`} />
                          </div>
                        </div>
                      </button>
                      <button
                        type="button"
                        onClick={() => handleWecomModeChange('callback')}
                        className={`rounded-xl border p-4 text-left transition ${currentWecomMode === 'callback' ? 'border-sky-500 bg-sky-50/80 dark:bg-sky-950/20 shadow-sm' : 'border-gray-200 dark:border-gray-700 hover:border-sky-300 dark:hover:border-sky-700 bg-white dark:bg-gray-900'}`}
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div>
                            <div className="text-sm font-semibold text-gray-900 dark:text-white">使用 URL 回调</div>
                            <div className="mt-1 text-xs leading-relaxed text-gray-500 dark:text-gray-400">通过回调 URL 接收消息并返回结果，适合已有域名、反向代理或公网入口的部署方式。</div>
                          </div>
                          <div className={`mt-1 h-4 w-4 rounded-full border ${currentWecomMode === 'callback' ? 'border-sky-500 bg-sky-500' : 'border-gray-300 dark:border-gray-600'}`}>
                            <div className={`m-[3px] h-2 w-2 rounded-full bg-white ${currentWecomMode === 'callback' ? 'opacity-100' : 'opacity-0'}`} />
                          </div>
                        </div>
                      </button>
                    </div>
                  </div>

                  <div className="rounded-lg border border-sky-100 dark:border-sky-900/40 bg-sky-50/70 dark:bg-sky-950/10 px-4 py-3 text-xs text-sky-700 dark:text-sky-300 leading-relaxed">
                    {currentWecomMode === 'long-polling'
                      ? <>当前为 <span className="font-semibold">长连接</span> 模式。保存时会写入 <span className="font-mono">botId</span> 和 <span className="font-mono">secret</span>，并优先按免配对模式处理。</>
                      : <>当前为 <span className="font-semibold">URL 回调</span> 模式。保存时会写入 <span className="font-mono">token</span>、<span className="font-mono">encodingAESKey</span> 和 <span className="font-mono">webhookPath</span>。</>}
                  </div>
                </div>
              )}

              <form
                id="channel-config-form"
                className={currentDef.id === 'feishu' || currentDef.id === 'wecom' || currentDef.id === 'qqbot' ? 'space-y-4' : 'grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-5'}
                onSubmit={e => { e.preventDefault(); handleSave(); }}
              >
                {currentDef.id === 'feishu'
                  ? (Object.entries(FEISHU_FIELD_SECTIONS) as Array<[Exclude<ChannelFieldSection, 'default'>, { title: string; description: string }]>).map(([sectionKey, sectionMeta]) => {
                      const fields = currentDef.configFields.filter(field => field.section === sectionKey);
                      const visibleFields = fields.filter(field => !(field.key === 'groupAllowFrom' && currentFeishuGroupPolicy !== 'allowlist' && currentFeishuAllowlistEntries.length === 0));
                      if (visibleFields.length === 0) return null;
                      return (
                        <div key={sectionKey} className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                          <div>
                            <h4 className="text-sm font-semibold text-gray-900 dark:text-white">{sectionMeta.title}</h4>
                            <p className="text-xs text-gray-500 mt-1 leading-relaxed">{sectionMeta.description}</p>
                          </div>
                          {sectionKey === 'access' && currentFeishuRequireMentionInvalid && (
                            <div className="rounded-lg border border-amber-200 dark:border-amber-800/40 bg-amber-50/80 dark:bg-amber-900/10 px-4 py-3 text-xs text-amber-700 dark:text-amber-300 leading-relaxed">
                              检测到当前配置中的 <span className="font-mono">requireMention</span> 为非法值 <span className="font-mono">{JSON.stringify(currentFeishuRequireMentionRaw)}</span>。
                              面板现在只接受 <span className="font-mono">true</span> / <span className="font-mono">false</span>；
                              如需开放群聊，请改用 <span className="font-mono">groupPolicy=open</span>。
                            </div>
                          )}
                          <div className="grid grid-cols-1 md:grid-cols-2 gap-x-5 gap-y-4">
                            {visibleFields.map(field => renderConfigField(currentDef.id, field))}
                          </div>
                        </div>
                      );
                    })
                  : currentDef.id === 'wecom'
                    ? (
                      <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                        <div>
                          <h4 className="text-sm font-semibold text-gray-900 dark:text-white">API 配置</h4>
                          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400 leading-relaxed">
                            {currentWecomMode === 'long-polling'
                              ? '使用 SDK 启动长连接，配置 Bot ID 和 Secret 即可连接企业微信智能机器人。'
                              : '使用企业微信回调地址接收消息并返回结果，需要填写 Token、EncodingAESKey 与 Webhook Path。'}
                          </p>
                        </div>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-x-5 gap-y-4">
                          {currentDef.configFields
                            .filter(field => field.key !== 'connectionMode')
                            .filter(field => currentWecomMode === 'long-polling'
                              ? ['botId', 'secret'].includes(field.key)
                              : ['token', 'encodingAESKey', 'webhookPath'].includes(field.key))
                            .map(field => renderConfigField(currentDef.id, field))}
                        </div>
                      </div>
                    )
                  : currentDef.id === 'qqbot'
                    ? (
                      <div className="space-y-4">
                        <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4">
                          <div className="flex items-center justify-between gap-4 flex-wrap">
                            <div>
                              <h4 className="text-sm font-semibold text-gray-900 dark:text-white">{'QQBot \u901a\u9053\u7248\u672c'}</h4>
                              <p className="mt-1 text-xs text-gray-500 dark:text-gray-400 leading-relaxed">{'\u53ef\u5728\u5185\u7f6e\u7248\u4e0e\u793e\u533a\u7248\u4e4b\u95f4\u5207\u6362\uff0c\u5207\u6362\u540e\u4f1a\u81ea\u52a8\u542f\u7528\u5f53\u524d\u7248\u672c\u5e76\u5173\u95ed\u53e6\u4e00\u4e2a\u3002'}</p>
                            </div>
                            <div className="inline-flex rounded-lg border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 p-1">
                              <button
                                type="button"
                                onClick={() => handleSwitchQQBotVariant('builtin')}
                                disabled={switchingQQBotVariant}
                                className={`px-3 py-1.5 text-xs font-medium rounded-md transition ${currentQQBotVariant === 'builtin'
                                  ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm'
                                  : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}`}
                              >
                                {'\u5185\u7f6e\u7248'}
                              </button>
                              <button
                                type="button"
                                onClick={() => handleSwitchQQBotVariant('community')}
                                disabled={switchingQQBotVariant}
                                className={`px-3 py-1.5 text-xs font-medium rounded-md transition ${currentQQBotVariant === 'community'
                                  ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm'
                                  : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200'}`}
                              >
                                {'\u793e\u533a\u7248'}
                              </button>
                            </div>
                          </div>
                        </div>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-5">
                          {qqbotPrimaryFields.map(field => renderConfigField(currentDef.id, field))}
                        </div>
                        <div className="rounded-xl border border-blue-100 dark:border-blue-900/40 bg-blue-50/60 dark:bg-blue-950/20 px-4 py-3 text-xs text-blue-700 dark:text-blue-300 leading-relaxed">
                          {'\u5b98\u65b9 QQBot \u65e5\u5e38\u53ea\u9700\u8981\u914d\u7f6e '}
                          <span className="font-mono">App ID</span>
                          {' \u548c '}
                          <span className="font-mono">App Secret</span>
                          {' \u5373\u53ef\u8fd0\u884c\uff1b\u5176\u4f59\u517c\u5bb9\u5b57\u6bb5\u8bf7\u6309\u9700\u5728\u9ad8\u7ea7\u9009\u9879\u4e2d\u5f00\u542f\u3002'}
                        </div>
                        {qqbotAdvancedFields.length > 0 && (
                          <div className="rounded-xl border border-gray-100 dark:border-gray-700 p-4 space-y-4">
                            <button
                              type="button"
                              onClick={() => setQqbotAdvancedOpen(prev => !prev)}
                              className="w-full flex items-center justify-between gap-3 text-left"
                            >
                              <div>
                                <h4 className="text-sm font-semibold text-gray-900 dark:text-white">高级配置（兼容与定制）</h4>
                                <p className="mt-1 text-xs text-gray-500 dark:text-gray-400 leading-relaxed">
                                  仅在你明确需要路由控制、兼容老配置或调试场景时再修改。
                                </p>
                              </div>
                              <ChevronDown size={16} className={`text-gray-400 transition-transform ${qqbotAdvancedOpen ? 'rotate-180' : ''}`} />
                            </button>
                            {qqbotAdvancedOpen && (
                              <div className="grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-5 pt-2 border-t border-gray-100 dark:border-gray-700">
                                {qqbotAdvancedFields.map(field => renderConfigField(currentDef.id, field))}
                              </div>
                            )}
                          </div>
                        )}
                      </div>
                    )
                  : currentDef.configFields.map(field => renderConfigField(currentDef.id, field))}
               </form>

              {currentDef.configFields.length === 0 && (
                <div className="py-12 flex flex-col items-center justify-center text-gray-400 border-2 border-dashed border-gray-100 dark:border-gray-800 rounded-xl">
                  <Sparkles size={32} className="mb-2 opacity-20" />
                  <p className="text-sm">{t.channels.noConfigNeeded}</p>
                </div>
              )}

              <div className="flex items-center justify-end pt-4 border-t border-gray-50 dark:border-gray-800">
                <button onClick={handleSave} disabled={saving}
                  className={`${modern ? 'page-modern-accent px-5 py-2.5 text-sm' : 'flex items-center gap-2 px-5 py-2.5 text-sm font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50 shadow-sm shadow-violet-200 dark:shadow-none transition-all hover:shadow-md hover:shadow-violet-200 dark:hover:shadow-none'}`}>
                  {saving ? <Loader2 size={16} className="animate-spin" /> : <Check size={16} />}
                  {saving ? t.channels.saving : t.channels.saveConfig}
                </button>
              </div>
            </div>
          )}

          {/* QQ Requests — only when QQ selected */}
          {selectedChannel === 'qq' && requests.length > 0 && (
            <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 p-5 space-y-4">
              <div className="flex items-center gap-3 pb-2 border-b border-gray-50 dark:border-gray-800">
                <div className="p-1.5 rounded-lg bg-violet-100 dark:bg-violet-900/30 text-violet-600">
                  <UserCheck size={18} />
                </div>
                <h3 className="font-bold text-sm text-gray-900 dark:text-white">{t.channels.pendingRequests}</h3>
                <span className="bg-red-100 text-red-600 text-xs font-bold px-2 py-0.5 rounded-full">{requests.length}</span>
              </div>
              <div className="space-y-2.5">
                {requests.map((r: any) => (
                  <div key={r.flag} className="flex items-center gap-4 px-4 py-3 rounded-xl bg-gray-50 dark:bg-gray-900/30 border border-gray-100 dark:border-gray-800 text-sm">
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 mb-0.5">
                        <span className={`text-[10px] font-bold px-1.5 py-0.5 rounded ${r.type === 'group' ? 'bg-indigo-100 text-indigo-700' : 'bg-pink-100 text-pink-700'}`}>
                          {r.type === 'group' ? t.channels.groupRequest : t.channels.friendRequest}
                        </span>
                        <span className="font-mono text-gray-900 dark:text-gray-100 font-medium">{r.userId || r.groupId || ''}</span>
                      </div>
                      {r.comment && <div className="text-gray-500 text-xs truncate">{t.channels.comment}: "{r.comment}"</div>}
                    </div>
                    <div className="flex items-center gap-2">
                      <button onClick={() => handleApprove(r.flag)} className={`${modern ? 'page-modern-success p-2' : 'p-2 rounded-lg bg-emerald-50 dark:bg-emerald-900/30 text-emerald-600 hover:bg-emerald-100 dark:hover:bg-emerald-900/50 transition-colors'}`} title={t.channels.approve}>
                        <Check size={16} />
                      </button>
                      <button onClick={() => handleReject(r.flag)} className={`${modern ? 'page-modern-danger p-2' : 'p-2 rounded-lg bg-red-50 dark:bg-red-900/30 text-red-500 hover:bg-red-100 dark:hover:bg-red-900/50 transition-colors'}`} title={t.channels.reject}>
                        <X size={16} />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Channel Login Modal */}
      {loginModal && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4" onClick={() => setLoginModal(null)}>
          <div className="bg-white dark:bg-gray-900 rounded-2xl shadow-2xl max-w-md w-full overflow-hidden transform transition-all border border-gray-100 dark:border-gray-800" onClick={e => e.stopPropagation()}>
            <div className="px-5 py-4 border-b border-gray-100 dark:border-gray-800 flex items-center justify-between bg-gray-50/50 dark:bg-gray-900">
              <h3 className="text-sm font-bold text-gray-900 dark:text-white flex items-center gap-2">
                {loginModal === 'qrcode' && <><QrCode size={18} className="text-blue-500" /> {currentLoginChannel?.label || '渠道'} {t.channels.qrLogin}</>}
                {loginModal === 'quick' && <><Zap size={18} className="text-emerald-500" /> {currentLoginChannel?.label || 'QQ'} {t.channels.quickLogin}</>}
                {loginModal === 'password' && <><Key size={18} className="text-amber-500" /> {currentLoginChannel?.label || 'QQ'} {t.channels.passwordLogin}</>}
              </h3>
              <button onClick={() => setLoginModal(null)} className={`${modern ? 'page-modern-action p-1.5' : 'p-1 rounded-md text-gray-400 hover:text-gray-600 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors'}`}>
                <X size={18} />
              </button>
            </div>
            <div className="p-6 space-y-4">
              {loginMsg && (
                <div className={`px-4 py-3 rounded-xl text-sm font-medium flex items-center gap-2 ${loginMsg.includes(t.common.success) || loginMsg.includes('success') ? 'bg-emerald-50 dark:bg-emerald-900/30 text-emerald-600' : 'bg-red-50 dark:bg-red-900/30 text-red-600'}`}>
                   {loginMsg.includes(t.common.success) || loginMsg.includes('success') ? <Check size={16} /> : <X size={16} />}
                  {loginMsg}
                </div>
              )}

              {/* QR Code Login */}
              {loginModal === 'qrcode' && (
                <div className="flex flex-col items-center gap-3">
                  {qrLoading ? (
                    <div className="w-48 h-48 flex items-center justify-center bg-gray-50 dark:bg-gray-800 rounded-lg">
                      <Loader2 size={24} className="animate-spin text-gray-400" />
                    </div>
                  ) : qrRenderImg ? (
                    <img src={qrRenderImg.startsWith('data:') || qrRenderImg.startsWith('http') ? qrRenderImg : `data:image/png;base64,${qrRenderImg}`} alt="QR Code" className="w-48 h-48 rounded-lg border border-gray-200 dark:border-gray-700" />
                  ) : (
                    <div className="w-48 h-48 flex items-center justify-center bg-gray-50 dark:bg-gray-800 rounded-lg text-xs text-gray-400">
                      {t.channels.cannotLoadQR}
                    </div>
                  )}
                  <p className="text-xs text-gray-500">
                    {currentLoginChannelId === 'openclaw-weixin' ? '使用微信扫描二维码，并在手机上确认登录。' : t.channels.scanQR}
                  </p>
                  <button onClick={handleRefreshQR} disabled={qrLoading}
                    className={`${modern ? 'page-modern-accent px-3 py-1.5 text-xs' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50'}`}>
                    <RefreshCw size={12} className={qrLoading ? 'animate-spin' : ''} />{t.channels.refreshQR}
                  </button>
                </div>
              )}

              {/* Quick Login */}
              {loginModal === 'quick' && (
                <div className="space-y-2">
                  {loginLoading ? (
                    <div className="flex items-center justify-center py-8"><Loader2 size={20} className="animate-spin text-gray-400" /></div>
                  ) : quickList.length === 0 ? (
                    <p className="text-xs text-gray-400 text-center py-8">{t.channels.noQuickAccounts}</p>
                  ) : (
                    <>
                      <p className="text-xs text-gray-500">{t.channels.selectQuickAccount}</p>
                      {quickList.map(uin => (
                        <button key={uin} onClick={() => handleQuickLogin(uin)}
                          className={`${modern ? 'w-full flex items-center gap-3 px-4 py-3 rounded-xl border border-emerald-100/70 bg-[linear-gradient(135deg,rgba(16,185,129,0.08),rgba(255,255,255,0.72))] dark:bg-[linear-gradient(135deg,rgba(16,185,129,0.16),rgba(15,23,42,0.8))] dark:border-emerald-800/30 text-sm transition-all hover:shadow-sm hover:border-emerald-200/80' : 'w-full flex items-center gap-3 px-4 py-3 rounded-lg bg-gray-50 dark:bg-gray-800 hover:bg-emerald-50 dark:hover:bg-emerald-950 text-sm transition-colors'}`}>
                          <Zap size={14} className="text-emerald-500" />
                          <span className="font-mono">{uin}</span>
                        </button>
                      ))}
                    </>
                  )}
                </div>
              )}

              {/* Password Login */}
              {loginModal === 'password' && (
                <div className="space-y-3">
                  <div>
                    <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">{t.channels.qqNumberLabel}</label>
                    <input type="text" value={loginUin} onChange={e => setLoginUin(e.target.value)}
                      placeholder={t.channels.qqNumberPlaceholder} className="w-full px-3 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-transparent" />
                  </div>
                  <div>
                    <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">{t.channels.passwordLabel}</label>
                    <input type="password" value={loginPwd} onChange={e => setLoginPwd(e.target.value)}
                      placeholder={t.channels.passwordPlaceholder} className="w-full px-3 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-transparent" />
                  </div>
                  <button onClick={handlePasswordLogin} disabled={loginLoading || !loginUin || !loginPwd}
                    className={`${modern ? 'page-modern-warn w-full px-3 py-2 text-xs' : 'w-full flex items-center justify-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg bg-amber-600 text-white hover:bg-amber-700 disabled:opacity-50'}`}>
                    {loginLoading ? <Loader2 size={12} className="animate-spin" /> : <Key size={12} />}
                    {loginLoading ? t.login.loggingIn : t.login.loginButton}
                  </button>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

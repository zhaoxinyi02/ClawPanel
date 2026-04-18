import { useEffect, useState } from 'react';
import { useNavigate, useOutletContext } from 'react-router-dom';
import { api } from '../lib/api';
import { RefreshCw, Save, Play, Wrench, Activity, Check, Radio, ChevronDown } from 'lucide-react';
import { useI18n } from '../i18n';
import ConfigFieldRenderer from '../components/ConfigFieldRenderer';

// ============================================================================
// Types
// ============================================================================

type PlatformFieldSection = 'bot' | 'platform' | 'advanced';

type PlatformConfigField = {
  key: string;
  label: string;
  type: 'text' | 'password' | 'toggle' | 'number' | 'select' | 'textarea';
  options?: string[];
  valueFormat?: 'stringArray';
  placeholder?: string;
  help?: string;
  defaultValue?: string | number | boolean;
  section?: PlatformFieldSection;
  rows?: number;
  envVar?: boolean; // if true, belongs to environment vars; otherwise config.yaml
};

interface PlatformStatus {
  id: string;
  label: string;
  configured: boolean;
  enabled: boolean;
  runtimeStatus?: string;
  lastEvidence?: string;
  lastError?: string;
  detail?: string;
}

interface PlatformDetail {
  status: PlatformStatus;
  config: Record<string, any>;
  environment: Record<string, string>;
  fields?: PlatformConfigField[];
}

interface PlatformsResponse {
  platforms?: PlatformStatus[];
  configuredCount?: number;
  enabledCount?: number;
  detailsById?: Record<string, PlatformDetail>;
}

// ============================================================================
// Platform Field Catalogs
// ============================================================================

const TELEGRAM_FIELDS: PlatformConfigField[] = [
  // Bot Configuration (ENV)
  {
    key: 'TELEGRAM_BOT_TOKEN',
    label: 'Bot Token',
    type: 'password',
    section: 'bot',
    envVar: true,
    help: 'Token from @BotFather, e.g. 123456789:ABCdefGHIjkl...',
    placeholder: '123456789:ABCdefGHIjklMNOpqrsTUVwxyz',
  },
  {
    key: 'TELEGRAM_ALLOWED_USERS',
    label: 'Allowed Users',
    type: 'text',
    section: 'bot',
    envVar: true,
    help: 'Comma-separated Telegram user IDs. If empty, all users are denied by default.',
    placeholder: '123456789, 987654321',
  },
  {
    key: 'TELEGRAM_HOME_CHANNEL',
    label: 'Home Channel ID',
    type: 'text',
    section: 'bot',
    envVar: true,
    help: 'Numeric chat ID for cron job delivery. Use @username for channels.',
    placeholder: '-1001234567890',
  },
  {
    key: 'TELEGRAM_HOME_CHANNEL_NAME',
    label: 'Home Channel Name',
    type: 'text',
    section: 'bot',
    envVar: true,
    help: 'Display name for the home channel.',
    placeholder: 'My Channel',
  },
  // Platform Settings (config.yaml)
  {
    key: 'require_mention',
    label: 'Require @mention',
    type: 'toggle',
    section: 'platform',
    envVar: false,
    help: 'Bot only responds when @mentioned in groups. DMs are always allowed.',
  },
  {
    key: 'mention_patterns',
    label: 'Mention Patterns',
    type: 'textarea',
    section: 'platform',
    envVar: false,
    rows: 3,
    help: 'Regex wake-word patterns (one per line or JSON array). Used when require_mention is on.',
    placeholder: '^\\s*hermes\\b\n^\\s*hey hermes\\b',
  },
  {
    key: 'free_response_chats',
    label: 'Free Response Chats',
    type: 'textarea',
    section: 'platform',
    envVar: false,
    rows: 3,
    help: 'Chat IDs where the bot responds without needing a @mention (one per line or JSON array).',
    placeholder: '-1001234567890\n-1009876543210',
  },
  {
    key: 'ignored_threads',
    label: 'Ignored Threads',
    type: 'textarea',
    section: 'platform',
    envVar: false,
    rows: 3,
    help: 'Forum topic/thread IDs where the bot never responds (one per line or JSON array).',
    placeholder: '31\n42',
  },
  {
    key: 'reactions',
    label: 'Enable Reactions',
    type: 'toggle',
    section: 'platform',
    envVar: false,
    help: 'Show emoji reactions on messages while the agent is thinking.',
  },
  {
    key: 'reply_to_mode',
    label: 'Reply Mode',
    type: 'select',
    options: ['first', 'all', 'off'],
    section: 'platform',
    envVar: false,
    help: 'Whether to reply to message threads. "first" replies to the first message in a thread.',
  },
  {
    key: 'disable_link_previews',
    label: 'Disable Link Previews',
    type: 'toggle',
    section: 'platform',
    envVar: false,
    help: 'Suppress Telegram link previews for URLs in bot messages.',
  },
  {
    key: 'webhook_mode',
    label: 'Webhook Mode',
    type: 'toggle',
    section: 'advanced',
    envVar: false,
    help: 'Use webhook instead of long polling. Requires TELEGRAM_WEBHOOK_URL env var.',
  },
  {
    key: 'TELEGRAM_WEBHOOK_URL',
    label: 'Webhook URL',
    type: 'text',
    section: 'advanced',
    envVar: true,
    help: 'Public HTTPS URL for webhook mode. Enables webhook instead of polling.',
    placeholder: 'https://your-domain.com/webhook/telegram',
  },
  {
    key: 'TELEGRAM_WEBHOOK_PORT',
    label: 'Webhook Port',
    type: 'number',
    section: 'advanced',
    envVar: true,
    help: 'Local listen port for webhook server (default: 8443).',
    placeholder: '8443',
  },
  {
    key: 'TELEGRAM_WEBHOOK_SECRET',
    label: 'Webhook Secret',
    type: 'password',
    section: 'advanced',
    envVar: true,
    help: 'Secret token for verifying that updates come from Telegram.',
    placeholder: 'your-webhook-secret',
  },
  {
    key: 'TELEGRAM_PROXY',
    label: 'Proxy URL',
    type: 'text',
    section: 'advanced',
    envVar: true,
    help: 'Proxy URL (http://, https://, socks5://). Overrides HTTPS_PROXY.',
    placeholder: 'socks5://127.0.0.1:1080',
  },
  {
    key: 'TELEGRAM_REQUIRE_MENTION',
    label: 'Require Mention (Legacy)',
    type: 'toggle',
    section: 'advanced',
    envVar: true,
    help: 'Legacy env var equivalent of require_mention config option.',
  },
  {
    key: 'TELEGRAM_REPLY_TO_MODE',
    label: 'Reply Mode (Legacy)',
    type: 'select',
    options: ['first', 'all', 'off'],
    section: 'advanced',
    envVar: true,
    help: 'Legacy env var equivalent of reply_to_mode config option.',
  },
  {
    key: 'TELEGRAM_FALLBACK_IPS',
    label: 'Fallback IPs',
    type: 'textarea',
    section: 'advanced',
    envVar: true,
    rows: 3,
    help: 'Comma-separated fallback IPs for restricted networks.',
    placeholder: '149.154.167.220\n149.154.164.220',
  },
];

const QQBOT_FIELDS: PlatformConfigField[] = [
  {
    key: 'QQ_APP_ID',
    label: 'App ID',
    type: 'text',
    section: 'bot',
    envVar: true,
    help: 'QQ Bot App ID from q.qq.com.',
    placeholder: '1024xxxxxx',
  },
  {
    key: 'QQ_CLIENT_SECRET',
    label: 'App Secret',
    type: 'password',
    section: 'bot',
    envVar: true,
    help: 'QQ Bot App Secret from q.qq.com.',
    placeholder: 'your-app-secret',
  },
  {
    key: 'QQ_HOME_CHANNEL',
    label: 'Home Channel',
    type: 'text',
    section: 'bot',
    envVar: true,
    help: 'OpenID used for cron delivery and notifications.',
    placeholder: 'user_or_group_openid',
  },
  {
    key: 'QQ_HOME_CHANNEL_NAME',
    label: 'Home Channel Name',
    type: 'text',
    section: 'bot',
    envVar: true,
    help: 'Display name for the home target.',
    placeholder: 'Home',
  },
  {
    key: 'extra.markdown_support',
    label: 'Markdown Support',
    type: 'toggle',
    section: 'platform',
    envVar: false,
    help: 'Enable QQ markdown message format.',
  },
  {
    key: 'extra.dm_policy',
    label: 'DM Policy',
    type: 'select',
    options: ['open', 'allowlist', 'disabled'],
    section: 'platform',
    envVar: false,
    help: 'Direct-message access policy.',
  },
  {
    key: 'QQ_ALLOWED_USERS',
    label: 'DM Allowed Users',
    type: 'textarea',
    section: 'platform',
    envVar: true,
    rows: 3,
    help: 'Comma or newline separated QQ user OpenIDs allowed to DM the bot.',
    placeholder: 'user_openid_1\nuser_openid_2',
  },
  {
    key: 'extra.group_policy',
    label: 'Group Policy',
    type: 'select',
    options: ['open', 'allowlist', 'disabled'],
    section: 'platform',
    envVar: false,
    help: 'Group @-message access policy.',
  },
  {
    key: 'QQ_GROUP_ALLOWED_USERS',
    label: 'Group Allowlist',
    type: 'textarea',
    section: 'platform',
    envVar: true,
    rows: 3,
    help: 'Comma or newline separated group OpenIDs allowed to mention the bot.',
    placeholder: 'group_openid_1\ngroup_openid_2',
  },
  {
    key: 'QQ_ALLOW_ALL_USERS',
    label: 'Allow All Users',
    type: 'toggle',
    section: 'advanced',
    envVar: true,
    help: 'Allow all QQ users to DM the bot. Overrides DM allowlist.',
  },
  {
    key: 'QQ_STT_API_KEY',
    label: 'STT API Key',
    type: 'password',
    section: 'advanced',
    envVar: true,
    help: 'Optional voice-to-text API key when QQ built-in ASR is unavailable.',
    placeholder: 'stt-api-key',
  },
  {
    key: 'QQ_STT_BASE_URL',
    label: 'STT Base URL',
    type: 'text',
    section: 'advanced',
    envVar: true,
    help: 'Optional OpenAI-compatible STT endpoint.',
    placeholder: 'https://open.bigmodel.cn/api/coding/paas/v4',
  },
  {
    key: 'QQ_STT_MODEL',
    label: 'STT Model',
    type: 'text',
    section: 'advanced',
    envVar: true,
    help: 'Voice transcription model name.',
    placeholder: 'glm-asr',
  },
];

const DISCORD_FIELDS: PlatformConfigField[] = [
  { key: 'DISCORD_BOT_TOKEN', label: 'Bot Token', type: 'password', section: 'bot', envVar: true, help: 'Discord Bot Token。' },
  { key: 'DISCORD_ALLOWED_USERS', label: '私聊白名单', type: 'textarea', rows: 3, section: 'platform', envVar: true, valueFormat: 'stringArray', help: '允许与 Bot 私聊的用户 ID，支持逗号或换行分隔。' },
  { key: 'DISCORD_HOME_CHANNEL', label: 'Home Channel', type: 'text', section: 'platform', envVar: true, help: '用于通知和定时任务投递的频道 ID。' },
];

const SLACK_FIELDS: PlatformConfigField[] = [
  { key: 'SLACK_APP_TOKEN', label: 'App Token', type: 'password', section: 'bot', envVar: true, help: 'Slack App Level Token，通常以 xapp- 开头。' },
  { key: 'SLACK_BOT_TOKEN', label: 'Bot Token', type: 'password', section: 'bot', envVar: true, help: 'Slack Bot User OAuth Token，通常以 xoxb- 开头。' },
  { key: 'SLACK_ALLOWED_USERS', label: '私聊白名单', type: 'textarea', rows: 3, section: 'platform', envVar: true, valueFormat: 'stringArray', help: '允许触发 Bot 的 Slack 用户 ID。' },
  { key: 'SLACK_HOME_CHANNEL', label: 'Home Channel', type: 'text', section: 'platform', envVar: true, help: '通知与 cron 消息投递频道。' },
];

const SIGNAL_FIELDS: PlatformConfigField[] = [
  { key: 'SIGNAL_ALLOWED_USERS', label: '私聊白名单', type: 'textarea', rows: 3, section: 'platform', envVar: true, valueFormat: 'stringArray', help: '允许触发 Bot 的 Signal 用户。' },
  { key: 'SIGNAL_HOME_CONTACT', label: 'Home Contact', type: 'text', section: 'platform', envVar: true, help: '用于通知和定时消息投递的 Signal 联系人。' },
];

const WHATSAPP_FIELDS: PlatformConfigField[] = [
  { key: 'WHATSAPP_ALLOWED_USERS', label: '私聊白名单', type: 'textarea', rows: 3, section: 'platform', envVar: true, valueFormat: 'stringArray', help: '允许触发 Bot 的 WhatsApp 用户。' },
  { key: 'WHATSAPP_HOME_CONTACT', label: 'Home Contact', type: 'text', section: 'platform', envVar: true, help: '用于通知和定时消息投递的联系人。' },
];

const FEISHU_FIELDS: PlatformConfigField[] = [
  { key: 'FEISHU_APP_ID', label: 'App ID', type: 'text', section: 'bot', envVar: true, help: '飞书 / Lark 应用 App ID。' },
  { key: 'FEISHU_APP_SECRET', label: 'App Secret', type: 'password', section: 'bot', envVar: true, help: '飞书 / Lark 应用 App Secret。' },
];

const WECOM_FIELDS: PlatformConfigField[] = [
  { key: 'WECOM_BOT_ID', label: 'Bot ID', type: 'text', section: 'bot', envVar: true, help: '企业微信长连接模式 Bot ID。' },
  { key: 'WECOM_SECRET', label: 'Secret', type: 'password', section: 'bot', envVar: true, help: '企业微信 Secret。' },
  { key: 'WECOM_ALLOWED_USERS', label: '私聊白名单', type: 'textarea', rows: 3, section: 'platform', envVar: true, valueFormat: 'stringArray', help: '允许触发 Bot 的企业微信用户。' },
  { key: 'WECOM_HOME_CHANNEL', label: 'Home Channel', type: 'text', section: 'platform', envVar: true, help: '通知和 cron 投递目标。' },
  { key: 'WECOM_HOME_CHANNEL_NAME', label: 'Home Channel Name', type: 'text', section: 'platform', envVar: true, help: 'Home Channel 显示名称。' },
];

const WEIXIN_FIELDS: PlatformConfigField[] = [
  { key: 'WEIXIN_ACCOUNT_ID', label: 'Account ID', type: 'text', section: 'bot', envVar: true, help: '微信接入账户 ID。' },
  { key: 'WEIXIN_TOKEN', label: 'Token', type: 'password', section: 'bot', envVar: true, help: '微信接入 Token。' },
  { key: 'WEIXIN_ALLOWED_USERS', label: '私聊白名单', type: 'textarea', rows: 3, section: 'platform', envVar: true, valueFormat: 'stringArray', help: '允许触发 Bot 的微信用户。' },
  { key: 'WEIXIN_HOME_CHANNEL', label: 'Home Channel', type: 'text', section: 'platform', envVar: true, help: '通知和 cron 投递目标。' },
  { key: 'WEIXIN_HOME_CHANNEL_NAME', label: 'Home Channel Name', type: 'text', section: 'platform', envVar: true, help: 'Home Channel 显示名称。' },
];

const DINGTALK_FIELDS: PlatformConfigField[] = [
  { key: 'DINGTALK_APP_KEY', label: 'App Key', type: 'text', section: 'bot', envVar: true, help: '钉钉应用 App Key。' },
  { key: 'DINGTALK_APP_SECRET', label: 'App Secret', type: 'password', section: 'bot', envVar: true, help: '钉钉应用 App Secret。' },
];

const MATRIX_FIELDS: PlatformConfigField[] = [
  { key: 'MATRIX_HOMESERVER_URL', label: 'Homeserver URL', type: 'text', section: 'bot', envVar: true, help: 'Matrix homeserver 地址。' },
  { key: 'MATRIX_ACCESS_TOKEN', label: 'Access Token', type: 'password', section: 'bot', envVar: true, help: 'Matrix Access Token。' },
];

const WEBHOOK_FIELDS: PlatformConfigField[] = [
  { key: 'WEBHOOK_PORT', label: 'Webhook Port', type: 'number', section: 'advanced', envVar: true, help: 'Webhook 监听端口。' },
  { key: 'WEBHOOK_SECRET', label: 'Webhook Secret', type: 'password', section: 'advanced', envVar: true, help: 'Webhook 签名 Secret。' },
];

const SMS_FIELDS: PlatformConfigField[] = [
  { key: 'TWILIO_ACCOUNT_SID', label: 'Twilio Account SID', type: 'text', section: 'bot', envVar: true, help: 'Twilio Account SID。' },
  { key: 'TWILIO_AUTH_TOKEN', label: 'Twilio Auth Token', type: 'password', section: 'bot', envVar: true, help: 'Twilio Auth Token。' },
  { key: 'TWILIO_FROM_NUMBER', label: 'From Number', type: 'text', section: 'platform', envVar: true, help: 'Twilio 发信号码。' },
];

const PLATFORM_CATALOG: Record<string, PlatformConfigField[]> = {
  telegram: TELEGRAM_FIELDS,
  qqbot: QQBOT_FIELDS,
  discord: DISCORD_FIELDS,
  slack: SLACK_FIELDS,
  signal: SIGNAL_FIELDS,
  whatsapp: WHATSAPP_FIELDS,
  feishu: FEISHU_FIELDS,
  wecom: WECOM_FIELDS,
  weixin: WEIXIN_FIELDS,
  dingtalk: DINGTALK_FIELDS,
  matrix: MATRIX_FIELDS,
  webhook: WEBHOOK_FIELDS,
  sms: SMS_FIELDS,
};

const SECTION_META: Record<PlatformFieldSection, { title: string; titleZh: string }> = {
  bot: {
    title: 'Bot Configuration',
    titleZh: '机器人配置',
  },
  platform: {
    title: 'Platform Settings',
    titleZh: '平台设置',
  },
  advanced: {
    title: 'Advanced',
    titleZh: '高级选项',
  },
};

// ============================================================================
// Helpers
// ============================================================================

function parseDelimitedList(value: string | undefined | null): string[] {
  if (!value) return [];
  return value.split(/[\n,]+/).map(s => s.trim()).filter(Boolean);
}

function formatDelimitedList(values: string[] | undefined | null): string {
  if (!values || !Array.isArray(values)) return '';
  return values.join(', ');
}

function getPlatformStatusKey(platform: PlatformStatus): 'enabled' | 'configured' | 'unconfigured' {
  if (platform.enabled || platform.runtimeStatus === 'healthy' || platform.runtimeStatus === 'warning' || platform.runtimeStatus === 'error') return 'enabled';
  if (platform.configured) return 'configured';
  return 'unconfigured';
}

function statusDotClass(status: 'enabled' | 'configured' | 'unconfigured') {
  if (status === 'enabled') return 'bg-emerald-500';
  if (status === 'configured') return 'bg-red-400';
  return 'bg-gray-300';
}

function getNestedValue(obj: Record<string, any> | null, key: string): any {
  if (!obj) return undefined;
  return key.split('.').reduce((o: any, k) => o?.[k], obj);
}

function setNestedValue(obj: Record<string, any>, key: string, value: any): void {
  const parts = key.split('.');
  let current = obj;
  for (let i = 0; i < parts.length - 1; i++) {
    const p = parts[i];
    if (!(p in current)) current[p] = {};
    current = current[p];
  }
  const last = parts[parts.length - 1];
  if (value === undefined || value === null || value === '') {
    delete current[last];
  } else {
    current[last] = value;
  }
}

function deleteNestedValue(obj: Record<string, any>, key: string): void {
  const parts = key.split('.');
  let current = obj;
  for (let i = 0; i < parts.length - 1; i++) {
    const p = parts[i];
    if (!(p in current)) return;
    current = current[p];
  }
  const last = parts[parts.length - 1];
  delete current[last];
}

function isPlainObject(value: any): boolean {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function envToText(env: Record<string, string>) {
  return Object.entries(env || {}).map(([k, v]) => `${k}=${v}`).join('\n');
}

function textToEnv(text: string) {
  const result: Record<string, string> = {};
  for (const line of text.split('\n')) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const idx = trimmed.indexOf('=');
    if (idx === -1) continue;
    const key = trimmed.slice(0, idx).trim();
    const value = trimmed.slice(idx + 1).trim();
    if (key) result[key] = value;
  }
  return result;
}

// ============================================================================
// Component
// ============================================================================

export default function HermesPlatforms() {
  const { locale } = useI18n();
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const navigate = useNavigate();
  const modern = uiMode === 'modern';

  const [platforms, setPlatforms] = useState<PlatformStatus[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [detail, setDetail] = useState<PlatformDetail | null>(null);
  const [detailCache, setDetailCache] = useState<Record<string, PlatformDetail>>({});

  // Structured drafts
  const [envDrafts, setEnvDrafts] = useState<Record<string, string>>({});
  const [configDrafts, setConfigDrafts] = useState<Record<string, any>>({});

  const [enabled, setEnabled] = useState(false);
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');
  const [actionRunning, setActionRunning] = useState(false);
  const [advancedOpen, setAdvancedOpen] = useState(false);

  const platformFields = (PLATFORM_CATALOG[selectedId] && PLATFORM_CATALOG[selectedId].length > 0)
    ? PLATFORM_CATALOG[selectedId]
    : (detail?.fields || []);

  const applyDetail = (platform: PlatformDetail | null) => {
    setDetail(platform);
    setEnabled(Boolean(platform?.status?.enabled));

    const envMap: Record<string, string> = {};
    for (const [k, v] of Object.entries(platform?.environment || {})) {
      envMap[k] = String(v ?? '');
    }
    setEnvDrafts(envMap);
    setConfigDrafts({ ...(platform?.config || {}) });
  };

  const loadPlatforms = async () => {
    setLoading(true);
    setErr('');
    try {
      const r = await api.getHermesPlatforms();
      if (r.ok) {
        const platformPayload = (r.platforms || {}) as PlatformsResponse;
        const next = platformPayload.platforms || [];
        const nextSelectedId = selectedId || next[0]?.id || '';
        setPlatforms(next);
        setSelectedId(prev => prev || nextSelectedId);

        const embeddedDetails = platformPayload.detailsById || {};
        if (Object.keys(embeddedDetails).length > 0) {
          setDetailCache(prev => ({ ...prev, ...embeddedDetails }));
          if (nextSelectedId && embeddedDetails[nextSelectedId]) {
            applyDetail(embeddedDetails[nextSelectedId]);
          }
        }
      }
    } catch {
      setErr(locale === 'zh-CN' ? '加载 Hermes 平台失败' : 'Failed to load Hermes platforms');
    } finally {
      setLoading(false);
    }
  };

  const loadDetail = async (id: string) => {
    if (!id) return;
    if (detailCache[id]) {
      applyDetail(detailCache[id]);
      return;
    }
    setDetail(null);
    setDetailLoading(true);
    try {
      const r = await api.getHermesPlatformDetail(id);
      if (r.ok) {
        const platform = r.platform || null;
        if (platform) {
          setDetailCache(prev => ({ ...prev, [id]: platform }));
        }
        applyDetail(platform);
      }
    } catch {
      setErr(locale === 'zh-CN' ? '加载 Hermes 平台详情失败' : 'Failed to load Hermes platform detail');
    } finally {
      setDetailLoading(false);
    }
  };

  useEffect(() => { loadPlatforms(); }, []);
  useEffect(() => { if (selectedId) loadDetail(selectedId); }, [selectedId]);
  useEffect(() => { setAdvancedOpen(false); }, [selectedId]);

  // --------------------------------------------------------------------------
  // Field value accessors
  // --------------------------------------------------------------------------

  const getEnvValue = (key: string): string => {
    return envDrafts[key] ?? '';
  };

  const getConfigValue = (key: string): any => {
    return getNestedValue(configDrafts, key);
  };

  const handleEnvChange = (key: string, value: string) => {
    setEnvDrafts(prev => {
      const next = { ...prev };
      if (value.trim()) {
        next[key] = value;
      } else {
        delete next[key];
      }
      return next;
    });
  };

  const handleConfigChange = (key: string, value: any) => {
    setConfigDrafts(prev => {
      const next = { ...prev };
      setNestedValue(next, key, value);
      return next;
    });
  };

  const handleToggle = (key: string) => {
    handleConfigChange(key, !getConfigValue(key));
  };

  // --------------------------------------------------------------------------
  // Save
  // --------------------------------------------------------------------------

  const save = async () => {
    if (!selectedId) return;
    setSaving(true);
    setErr('');
    setMsg('');
    try {
      const env = { ...envDrafts };
      const r = await api.updateHermesPlatformDetail(selectedId, { enabled, config: configDrafts, env });
      if (r.ok) {
        setMsg(locale === 'zh-CN' ? '平台配置已保存' : 'Platform configuration saved');
        setDetailCache(prev => {
          const next = { ...prev };
          delete next[selectedId];
          return next;
        });
        await loadPlatforms();
        await loadDetail(selectedId);
      } else {
        setErr(r.error || 'Save failed');
      }
    } catch (e) {
      setErr(locale === 'zh-CN' ? `保存失败: ${String(e)}` : `Save failed: ${String(e)}`);
    } finally {
      setSaving(false);
    }
  };

  // --------------------------------------------------------------------------
  // Actions
  // --------------------------------------------------------------------------

  const runAction = async (action: string) => {
    setActionRunning(true);
    setMsg('');
    setErr('');
    try {
      const r = await api.runHermesAction(action);
      if (r?.ok) {
        setMsg(locale === 'zh-CN' ? `已触发动作 ${action}` : `Triggered action ${action}`);
      } else {
        setErr(r?.error || 'Action failed');
      }
    } catch {
      setErr(locale === 'zh-CN' ? '执行 Hermes 平台动作失败' : 'Failed to run Hermes platform action');
    } finally {
      setActionRunning(false);
    }
  };

  // --------------------------------------------------------------------------
  // Field renderer
  // --------------------------------------------------------------------------

  const renderField = (field: PlatformConfigField) => {
    const isEnv = field.envVar !== false;
    const currentVal = isEnv ? getEnvValue(field.key) : getConfigValue(field.key);
    const hasExplicitValue = currentVal !== undefined && currentVal !== null && currentVal !== '';

    const handleChange = (rawValue: string) => {
      if (isEnv) {
        if (field.valueFormat === 'stringArray') {
          handleEnvChange(field.key, parseDelimitedList(rawValue).join(','));
        } else {
          handleEnvChange(field.key, rawValue);
        }
      } else {
        if (field.type === 'number') {
          const parsed = Number(rawValue);
          handleConfigChange(field.key, Number.isFinite(parsed) ? parsed : rawValue);
        } else if (field.valueFormat === 'stringArray') {
          handleConfigChange(field.key, parseDelimitedList(rawValue));
        } else {
          handleConfigChange(field.key, rawValue);
        }
      }
    };

    const textDraftKey = `${field.key}`;
    const textareaValue = field.type === 'textarea'
      ? (field.valueFormat === 'stringArray'
        ? (Array.isArray(currentVal) ? currentVal.join('\n') : String(currentVal ?? '').replace(/,/g, '\n'))
        : currentVal ?? '')
      : '';

    const showCheck = hasExplicitValue && field.type !== 'toggle';

    return (
      <ConfigFieldRenderer
        key={field.key}
        field={field}
        value={currentVal}
        textareaValue={textareaValue}
        hasExplicitValue={showCheck}
        onChange={handleChange}
        onToggle={() => handleToggle(field.key)}
        fieldHelp={field.help}
        emptyOptionLabel={field.placeholder || (locale === 'zh-CN' ? '未配置' : 'Not configured')}
        accent="violet"
      />
    );
  };

  const botFields = platformFields.filter(f => f.section === 'bot');
  const platformFields_ = platformFields.filter(f => f.section === 'platform');
  const advancedFields = platformFields.filter(f => f.section === 'advanced');

  const mainFieldGroups: { section: PlatformFieldSection; fields: PlatformConfigField[] }[] = [
    { section: 'bot' as PlatformFieldSection, fields: botFields },
    { section: 'platform' as PlatformFieldSection, fields: platformFields_ },
  ].filter(g => g.fields.length > 0);

  const sortedPlatforms = [...platforms].sort((a, b) => {
    const order = { enabled: 0, configured: 1, unconfigured: 2 };
    return order[getPlatformStatusKey(a)] - order[getPlatformStatusKey(b)] || a.label.localeCompare(b.label);
  });

  // --------------------------------------------------------------------------
  // Render
  // --------------------------------------------------------------------------

  return (
    <div className={`space-y-6 ${modern ? 'page-modern' : ''}`}>
      <div className={`${modern ? 'page-modern-header' : 'flex items-center justify-between'}`}>
        <div>
          <h2 className={`${modern ? 'page-modern-title text-xl' : 'text-xl font-bold text-gray-900 dark:text-white'}`}>
            {locale === 'zh-CN' ? 'Hermes 平台管理' : 'Hermes Platforms'}
          </h2>
          <p className={`${modern ? 'page-modern-subtitle mt-1 text-sm' : 'text-sm text-gray-500 mt-1'}`}>
            {locale === 'zh-CN'
              ? '配置 Hermes 消息平台的机器人凭证和运行参数。'
              : 'Configure Hermes messaging platform bot credentials and runtime parameters.'}
          </p>
          <p className="text-xs text-gray-500 mt-1.5 inline-flex items-center gap-3 flex-wrap">
            <span className="inline-flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-emerald-500 inline-block" />{locale === 'zh-CN' ? '已启用/运行态' : 'Enabled / active'}</span>
            <span className="inline-flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-red-400 inline-block" />{locale === 'zh-CN' ? '已配置未启用' : 'Configured'}</span>
            <span className="inline-flex items-center gap-1"><span className="w-1.5 h-1.5 rounded-full bg-gray-300 inline-block" />{locale === 'zh-CN' ? '未配置' : 'Unconfigured'}</span>
          </p>
        </div>
        <button onClick={loadPlatforms} className={`${modern ? 'page-modern-action px-3 py-2 text-xs' : 'px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-800'} inline-flex items-center gap-2`}>
          <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          {locale === 'zh-CN' ? '刷新' : 'Refresh'}
        </button>
      </div>

      {msg && <div className="rounded-2xl border border-emerald-100 bg-emerald-50/80 px-4 py-3 text-sm text-emerald-700 dark:border-emerald-900/30 dark:bg-emerald-900/10 dark:text-emerald-300">{msg}</div>}
      {err && <div className="rounded-2xl border border-red-100 bg-red-50/80 px-4 py-3 text-sm text-red-700 dark:border-red-900/30 dark:bg-red-900/10 dark:text-red-300">{err}</div>}

      <div className="grid grid-cols-1 xl:grid-cols-[320px_1fr] gap-6">
        {/* Platform list */}
        <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 flex flex-col max-h-[75vh] overflow-hidden">
          <div className="p-3 border-b border-gray-100 dark:border-gray-700/50 bg-gray-50/50 dark:bg-gray-800/50">
            <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider px-1">{locale === 'zh-CN' ? '平台列表' : 'Platform List'}</h3>
          </div>
          <div className="flex-1 overflow-y-auto p-2 space-y-1">
          {sortedPlatforms.map(platform => {
            const status = getPlatformStatusKey(platform);
            return (
            <button
              key={platform.id}
              onClick={() => setSelectedId(platform.id)}
              className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-left text-sm transition-all duration-200 group ${selectedId === platform.id ? 'border border-blue-100/80 bg-[linear-gradient(135deg,rgba(37,99,235,0.12),rgba(14,165,233,0.08))] dark:bg-[linear-gradient(135deg,rgba(37,99,235,0.2),rgba(14,165,233,0.12))] dark:border-blue-800/40 text-blue-700 dark:text-blue-300 shadow-sm' : 'text-gray-600 dark:text-gray-400 hover:bg-white/70 dark:hover:bg-gray-700/50 hover:border-blue-100/70'}`}
            >
              <div className={`p-1.5 rounded-xl transition-colors border ${selectedId === platform.id ? 'bg-blue-50/90 dark:bg-blue-900/30 border-blue-100 dark:border-blue-800/40 text-blue-600' : 'bg-gray-100 dark:bg-gray-700 border-transparent text-gray-500 group-hover:bg-white group-hover:border-blue-100/70 group-hover:shadow-sm'}`}>
                <Radio size={14} />
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-xs font-semibold truncate text-gray-900 dark:text-white">{platform.label}</div>
                <div className="text-[10px] text-gray-400 truncate opacity-80">{platform.detail || '-'}</div>
              </div>
              <span className={`w-2 h-2 rounded-full shrink-0 ${statusDotClass(status)} ring-2 ring-white dark:ring-gray-800`} />
            </button>
          );})}
          </div>
        </div>

        {/* Platform detail / form */}
        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-5 space-y-6`}>
          {!detail && !detailLoading ? (
            <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 p-10 md:p-14">
              <div className="mx-auto max-w-xl text-center space-y-4">
                <div className="mx-auto w-14 h-14 rounded-2xl bg-blue-50 dark:bg-blue-900/20 border border-blue-100 dark:border-blue-800/40 text-blue-500 flex items-center justify-center">
                  <Radio size={24} />
                </div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white">{locale === 'zh-CN' ? '请选择左侧平台开始配置' : 'Select a platform to configure'}</h3>
                <p className="text-sm text-gray-500 leading-relaxed">
                  {locale === 'zh-CN' ? '这里会显示对应平台的启用状态、运行线索和机器人参数。建议先配置基础凭证，再按需展开高级选项。' : 'This area shows platform status, runtime hints, and bot parameters. Configure basic credentials first, then expand advanced options as needed.'}
                </p>
              </div>
            </div>
          ) : detailLoading && !detail ? (
            <div className="space-y-6 animate-pulse">
              <div className="flex items-center justify-between gap-3">
                <div className="space-y-2 flex-1">
                  <div className="h-6 w-40 rounded bg-gray-200 dark:bg-gray-700" />
                  <div className="h-4 w-3/4 rounded bg-gray-100 dark:bg-gray-800" />
                </div>
                <div className="flex gap-2">
                  <div className="h-9 w-20 rounded-lg bg-gray-100 dark:bg-gray-800" />
                  <div className="h-9 w-24 rounded-lg bg-gray-100 dark:bg-gray-800" />
                  <div className="h-9 w-20 rounded-lg bg-gray-100 dark:bg-gray-800" />
                </div>
              </div>
              <div className="h-5 w-36 rounded bg-gray-100 dark:bg-gray-800" />
              <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                <div className="h-20 rounded-xl bg-gray-100 dark:bg-gray-800" />
                <div className="h-20 rounded-xl bg-gray-100 dark:bg-gray-800" />
              </div>
              {[1, 2].map(section => (
                <div key={section} className="space-y-4">
                  <div className="h-5 w-28 rounded bg-gray-200 dark:bg-gray-700" />
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div className="h-20 rounded-xl bg-gray-100 dark:bg-gray-800" />
                    <div className="h-20 rounded-xl bg-gray-100 dark:bg-gray-800" />
                    <div className="h-20 rounded-xl bg-gray-100 dark:bg-gray-800 md:col-span-2" />
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <>
              {/* Header */}
              <div className="flex items-center justify-between gap-3">
                <div>
                  <div className="text-lg font-semibold text-gray-900 dark:text-white">{detail!.status.label}</div>
                  <div className="text-xs text-gray-500 mt-1">{detail!.status.detail || '-'}</div>
                </div>
                <div className="flex items-center gap-2 flex-wrap">
                  <button onClick={() => navigate('/hermes/health')} className={`${modern ? 'page-modern-action px-3 py-2 text-xs' : 'px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-800'}`}>
                    {locale === 'zh-CN' ? '查看健康' : 'Health'}
                  </button>
                  <button
                    onClick={() => runAction(enabled ? 'gateway-restart' : 'gateway-start')}
                    disabled={actionRunning}
                    className={`${modern ? 'page-modern-action px-3 py-2 text-xs disabled:opacity-50' : 'px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-800 disabled:opacity-50'} inline-flex items-center gap-2`}
                  >
                    {enabled ? <Activity size={14} /> : <Play size={14} />}
                    {enabled ? (locale === 'zh-CN' ? '重启网关' : 'Restart') : (locale === 'zh-CN' ? '启动网关' : 'Start')}
                  </button>
                  <button
                    onClick={() => runAction('doctor')}
                    disabled={actionRunning}
                    className={`${modern ? 'page-modern-action px-3 py-2 text-xs disabled:opacity-50' : 'px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-800 disabled:opacity-50'} inline-flex items-center gap-2`}
                  >
                    <Wrench size={14} />
                    Doctor
                  </button>
                  <button
                    onClick={save}
                    disabled={saving}
                    className={`${modern ? 'page-modern-accent px-4 py-2 text-xs disabled:opacity-50' : 'px-4 py-2 text-xs rounded-lg bg-blue-600 text-white disabled:opacity-50'} inline-flex items-center gap-2`}
                  >
                    <Save size={14} />
                    {locale === 'zh-CN' ? '保存' : 'Save'}
                  </button>
                </div>
              </div>

              {/* Enable toggle */}
              <label className="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 cursor-pointer">
                <input
                  type="checkbox"
                  checked={enabled}
                  onChange={e => setEnabled(e.target.checked)}
                  className="w-4 h-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                />
                {locale === 'zh-CN' ? '启用该平台' : 'Enable this platform'}
              </label>

              {/* Runtime info */}
              {(detail!.status.lastEvidence || detail!.status.lastError) && (
                <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                  <div className="rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 px-4 py-3 text-sm">
                    <div className="text-[11px] uppercase tracking-wider text-gray-500">{locale === 'zh-CN' ? '最近线索' : 'Last Evidence'}</div>
                    <div className="mt-1 text-gray-800 dark:text-gray-100">{detail!.status.lastEvidence || '-'}</div>
                  </div>
                  <div className="rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 px-4 py-3 text-sm">
                    <div className="text-[11px] uppercase tracking-wider text-gray-500">{locale === 'zh-CN' ? '最近错误' : 'Last Error'}</div>
                    <div className="mt-1 text-red-600 dark:text-red-300">{detail!.status.lastError || '-'}</div>
                  </div>
                </div>
              )}

              {/* Structured form fields */}
              {mainFieldGroups.map(({ section, fields }) => (
                <div key={section} className="space-y-4">
                  <div className="border-b border-gray-100 dark:border-gray-700/50 pb-2">
                    <h3 className="text-sm font-semibold text-gray-800 dark:text-gray-200">
                      {locale === 'zh-CN' ? SECTION_META[section].titleZh : SECTION_META[section].title}
                    </h3>
                  </div>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {fields.map(renderField)}
                  </div>
                </div>
              ))}

              {advancedFields.length > 0 && (
                <div className="rounded-xl border border-gray-100 dark:border-gray-700/50 overflow-hidden">
                  <button
                    type="button"
                    onClick={() => setAdvancedOpen(prev => !prev)}
                    className="w-full flex items-center justify-between px-4 py-3 text-left bg-gray-50/60 dark:bg-gray-900/30 hover:bg-gray-100/70 dark:hover:bg-gray-900/50"
                  >
                    <div>
                      <div className="text-sm font-semibold text-gray-900 dark:text-white">{locale === 'zh-CN' ? '高级配置' : 'Advanced Configuration'}</div>
                      <div className="text-[11px] text-gray-500 mt-0.5">{locale === 'zh-CN' ? '按需展开 webhook、STT、兼容性等选项。' : 'Expand optional webhook, STT, and compatibility settings.'}</div>
                    </div>
                    <ChevronDown size={16} className={`text-gray-400 transition-transform ${advancedOpen ? 'rotate-180' : ''}`} />
                  </button>
                  {advancedOpen && (
                    <div className="p-4 grid grid-cols-1 md:grid-cols-2 gap-4 border-t border-gray-100 dark:border-gray-700/50">
                      {advancedFields.map(renderField)}
                    </div>
                  )}
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  );
}

import { memo, useEffect, useState } from 'react';
import { useOutletContext } from 'react-router-dom';
import { api } from '../lib/api';
import {
  Clock, Plus, Play, Pause, Trash2, Edit3, RefreshCw,
  CheckCircle2, XCircle, AlertCircle, ChevronDown, ChevronRight,
  Timer, CalendarClock,
} from 'lucide-react';
import { useI18n } from '../i18n';
import MobileActionTray from '../components/MobileActionTray';

interface CronDelivery {
  mode?: 'none' | 'announce' | 'webhook';
  channel?: string;
  to?: string;
  accountId?: string;
  bestEffort?: boolean;
  // Legacy webhook field kept for read compatibility; canonical field is "to".
  url?: string;
}

interface CronJob {
  id: string;
  name: string;
  description?: string;
  enabled: boolean;
  /** agentId: which agent handles this job (new field; old jobs omit it and used sessionTarget instead) */
  agentId?: string;
  schedule: { kind: string; expr?: string; every?: string; everyMs?: number; at?: string; atMs?: number; tz?: string };
  /** sessionTarget: 'main' | 'isolated' — which session scope to use (new semantic); legacy jobs store the agentId here */
  sessionTarget: string;
  wakeMode: string;
  payload: {
    kind: string;
    text?: string;
    message?: string;
    deliver?: boolean;
    channel?: string;
    to?: string;
    model?: string;
    thinking?: string;
    lightContext?: boolean;
    toolsAllow?: string[];
  };
  /** Top-level delivery config (canonical format, replaces legacy payload.deliver) */
  delivery?: CronDelivery;
  state: {
    nextRunAtMs?: number; lastRunAtMs?: number; lastStatus?: string; lastError?: string; lastDurationMs?: number;
    lastRunStatus?: string; lastDeliveryStatus?: string; lastDeliveryError?: string;
    consecutiveErrors?: number;
  };
  createdAtMs: number;
}

/** Resolve the effective agentId from a job (handles legacy format where sessionTarget was the agentId) */
function resolveAgentId(job: CronJob, fallback: string): string {
  if (job.agentId != null) return job.agentId;
  // Legacy: if sessionTarget is not a recognised session-mode keyword, it was the agentId
  if (job.sessionTarget !== 'main' && job.sessionTarget !== 'isolated') return job.sessionTarget;
  return fallback;
}

/** Resolve the effective session mode from a job */
function resolveSessionMode(job: CronJob): string {
  // T1: prioritize sessionTarget over agentId presence
  if (job.sessionTarget === 'main' || job.sessionTarget === 'isolated') return job.sessionTarget;
  if (job.agentId != null) return 'isolated'; // has agentId but no valid sessionTarget → infer isolated
  return 'main'; // both missing → default main
}

/** Resolve the effective delivery mode from a job (handles legacy payload.deliver) */
function resolveDelivery(job: CronJob): CronDelivery {
  if (job.delivery?.mode) {
    if (job.delivery.mode === 'webhook') {
      const target = (job.delivery.to || job.delivery.url || '').trim();
      return target ? { ...job.delivery, mode: 'webhook', to: target } : { ...job.delivery, mode: 'webhook' };
    }
    return job.delivery;
  }
  // Legacy fallback: payload.deliver boolean → announce/none
  if (job.payload.deliver === true) {
    return {
      mode: 'announce',
      channel: job.payload.channel,
      to: job.payload.to,
    };
  }
  return { mode: 'none' };
}

/** Resolve the effective delivery status label */
function resolveDeliveryStatus(job: CronJob): string | undefined {
  const delivery = resolveDelivery(job);
  return job.state.lastDeliveryStatus || (delivery.mode === 'none' ? 'not-requested' : undefined);
}

// T7: Parse well-known error codes from error strings
const KNOWN_ERROR_CODES: Record<string, { zh: string; en: string }> = {
  '99992361': { zh: 'open_id \u8de8\u5e94\u7528\uff0c\u8bf7\u68c0\u67e5 accountId \u662f\u5426\u6b63\u786e', en: 'open_id cross app - check accountId' },
  '99991400': { zh: '\u8bf7\u6c42\u53c2\u6570\u9519\u8bef', en: 'Invalid request parameters' },
  '99991401': { zh: '\u8ba4\u8bc1\u5931\u8d25\uff0c\u68c0\u67e5 token', en: 'Auth failed - check token' },
  '99991672': { zh: '\u6743\u9650\u4e0d\u8db3\uff0c\u68c0\u67e5\u5e94\u7528 scope', en: 'Insufficient scope/permission' },
  '230001': { zh: '\u673a\u5668\u4eba\u4e0d\u5728\u7fa4\u5185', en: 'Bot not in chat group' },
};

function parseErrorHint(error: string, locale: string): string | null {
  for (const [code, hint] of Object.entries(KNOWN_ERROR_CODES)) {
    if (error.includes(code)) return locale === 'zh-CN' ? hint.zh : hint.en;
  }
  return null;
}

type ScheduleKind = 'cron' | 'every' | 'at';

function CronJobsPage() {
  const { t, locale } = useI18n();
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const modern = uiMode === 'modern';
  const [jobs, setJobs] = useState<CronJob[]>([]);
  const [loading, setLoading] = useState(true);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [msg, setMsg] = useState('');
  const [agentOptions, setAgentOptions] = useState<string[]>([]);
  const [defaultAgent, setDefaultAgent] = useState('main');

  // New job form — core
  const [newName, setNewName] = useState('');
  const [newMessage, setNewMessage] = useState('');

  // New job form — delivery (replaces legacy newDeliver checkbox)
  const [newDeliveryMode, setNewDeliveryMode] = useState<'none' | 'announce' | 'webhook'>('announce');
  const [newDeliveryAccountId, setNewDeliveryAccountId] = useState('');
  const [newWebhookUrl, setNewWebhookUrl] = useState('');

  // New job form — agent + session target (separated)
  const [newAgentId, setNewAgentId] = useState('');
  const [newSessionMode, setNewSessionMode] = useState<'main' | 'isolated'>('isolated');
  const [newModelOverride, setNewModelOverride] = useState('');
  const [newThinkingLevel, setNewThinkingLevel] = useState('');
  const [newLightContext, setNewLightContext] = useState(false);
  const [newToolsAllow, setNewToolsAllow] = useState('');

  // T4: Feishu account options for delivery accountId dropdown
  const [feishuAccounts, setFeishuAccounts] = useState<string[]>([]);

  // New job form — schedule
  const [newScheduleKind, setNewScheduleKind] = useState<ScheduleKind>('cron');
  const [newCron, setNewCron] = useState('0 9 * * *');
  const [newEveryMin, setNewEveryMin] = useState(60);
  const [newAtDateTime, setNewAtDateTime] = useState('');

  useEffect(() => {
    loadJobs();
    loadAgents();
    loadFeishuAccounts();
  }, []);

  const loadJobs = async () => {
    setLoading(true);
    try {
      const r = await api.getCronJobs();
      if (r.ok && r.jobs) {
        setJobs(r.jobs);
      } else {
        setJobs([]);
      }
    } catch { setJobs([]); }
    finally { setLoading(false); }
  };

  const loadAgents = async () => {
    try {
      const r = await api.getAgentsConfig();
      if (r.ok) {
        const list = (r.agents?.list || []).map((x: any) => String(x.id || '').trim()).filter(Boolean);
        const configuredDefaultRaw = String(r.agents?.default || '').trim();
        const fallbackDefault = list[0] || 'main';
        const effectiveDefault = configuredDefaultRaw && list.includes(configuredDefaultRaw) ? configuredDefaultRaw : fallbackDefault;
        const uniq = Array.from(new Set<string>(list.length > 0 ? list : [effectiveDefault]));
        setAgentOptions(uniq);
        setDefaultAgent(effectiveDefault);
        if (!newAgentId) {
          setNewAgentId(effectiveDefault);
        }
      }
    } catch {
      setAgentOptions(['main']);
      setDefaultAgent('main');
      if (!newAgentId) setNewAgentId('main');
    }
  };

  // T4: load Feishu account IDs for delivery accountId dropdown
  const loadFeishuAccounts = async () => {
    try {
      const r = await api.getOpenClawConfig();
      if (r.ok && r.config) {
        const feishu = r.config?.channels?.feishu;
        if (feishu?.accounts && typeof feishu.accounts === 'object') {
          setFeishuAccounts(Object.keys(feishu.accounts));
        } else if (feishu?.defaultAccount) {
          setFeishuAccounts([feishu.defaultAccount]);
        }
      }
    } catch { /* ignore — feishu accounts are optional enhancement */ }
  };

  const toggleJob = async (id: string) => {
    const job = jobs.find(j => j.id === id);
    if (!job) return;
    const updated = jobs.map(j => j.id === id ? { ...j, enabled: !j.enabled } : j);
    setJobs(updated);
    try {
      await api.updateCronJobs(updated);
      setMsg(`${job.name} ${!job.enabled ? t.common.enabled : t.common.paused}`);
      setTimeout(() => setMsg(''), 2000);
    } catch {
      setJobs(jobs);
      setMsg(t.common.operationFailed);
      setTimeout(() => setMsg(''), 2000);
    }
  };

  const deleteJob = async (id: string) => {
    if (!confirm(t.cron.deleteConfirm)) return;
    const updated = jobs.filter(j => j.id !== id);
    setJobs(updated);
    try {
      await api.updateCronJobs(updated);
      setMsg(t.cron.deleted);
      setTimeout(() => setMsg(''), 2000);
    } catch {
      loadJobs();
      setMsg(t.cron.deleteFailed);
      setTimeout(() => setMsg(''), 2000);
    }
  };

  const buildSchedule = (): CronJob['schedule'] => {
    if (newScheduleKind === 'cron') return { kind: 'cron', expr: newCron };
    if (newScheduleKind === 'every') return { kind: 'every', everyMs: Math.max(1, newEveryMin) * 60000 };
    // T9: 'at' uses ISO 8601 string instead of atMs
    const iso = newAtDateTime ? new Date(newAtDateTime).toISOString() : new Date(Date.now() + 3600000).toISOString();
    return { kind: 'at', at: iso };
  };

  const createJob = async () => {
    if (!newName.trim() || !newMessage.trim()) {
      setMsg(t.cron.fillRequired);
      setTimeout(() => setMsg(''), 2000);
      return;
    }
    if (newScheduleKind === 'at' && !newAtDateTime) {
      setMsg(locale === 'zh-CN' ? '请填写执行时间' : 'Please set execution time');
      setTimeout(() => setMsg(''), 2000);
      return;
    }
    if (newSessionMode === 'main' && newDeliveryMode === 'announce') {
      setMsg(locale === 'zh-CN' ? '主会话不支持发送到通道，请改为 Webhook 或不投递' : 'Main session does not support channel announce; use Webhook or None');
      setTimeout(() => setMsg(''), 2000);
      return;
    }
    if (newDeliveryMode === 'webhook' && !newWebhookUrl.trim()) {
      setMsg(locale === 'zh-CN' ? 'Webhook 模式必须填写目标 URL' : 'Webhook mode requires a target URL');
      setTimeout(() => setMsg(''), 2000);
      return;
    }
    const agentId = newAgentId || defaultAgent || 'main';
    // T6: payload.kind is determined by sessionTarget
    const payloadKind = newSessionMode === 'main' ? 'systemEvent' : 'agentTurn';
    const payload: CronJob['payload'] = payloadKind === 'agentTurn'
      ? {
          kind: 'agentTurn',
          message: newMessage.trim(),
          ...(newModelOverride.trim() ? { model: newModelOverride.trim() } : {}),
          ...(newThinkingLevel.trim() ? { thinking: newThinkingLevel.trim() } : {}),
          ...(newLightContext ? { lightContext: true } : {}),
          ...(newToolsAllow.trim()
            ? {
                toolsAllow: newToolsAllow
                  .split(/[\n,，]+/)
                  .map(item => item.trim())
                  .filter(Boolean)
                  .filter((item, index, arr) => arr.indexOf(item) === index),
              }
            : {}),
        }
      : { kind: 'systemEvent', text: newMessage.trim() };
    // T5: canonical delivery at top level
    const delivery: CronDelivery = newDeliveryMode === 'announce'
      ? { mode: 'announce', ...(newDeliveryAccountId ? { accountId: newDeliveryAccountId } : {}) }
      : newDeliveryMode === 'webhook'
        ? { mode: 'webhook', ...(newWebhookUrl ? { to: newWebhookUrl.trim() } : {}) }
        : { mode: 'none' };
    const job: CronJob = {
      id: 'cron_' + Date.now(),
      name: newName.trim(),
      enabled: true,
      agentId,
      schedule: buildSchedule(),
      sessionTarget: newSessionMode,
      wakeMode: 'now',
      payload,
      delivery,
      state: {},
      createdAtMs: Date.now(),
    };
    const updated = [...jobs, job];
    setJobs(updated);
    try {
      await api.updateCronJobs(updated);
      setMsg(t.cron.createSuccess);
      setShowCreate(false);
      setNewName('');
      setNewMessage('');
      setNewAgentId(defaultAgent || 'main');
      setNewSessionMode('isolated');
      setNewModelOverride('');
      setNewThinkingLevel('');
      setNewLightContext(false);
      setNewToolsAllow('');
      setNewDeliveryMode('announce');
      setNewDeliveryAccountId('');
      setNewWebhookUrl('');
      setNewScheduleKind('cron');
      setNewCron('0 9 * * *');
      setNewEveryMin(60);
      setNewAtDateTime('');
      setTimeout(() => setMsg(''), 2000);
    } catch {
      loadJobs();
      setMsg(t.cron.createFailed);
      setTimeout(() => setMsg(''), 2000);
    }
  };

  const formatSchedule = (s: CronJob['schedule']) => {
    if (s.kind === 'cron') return `Cron: ${s.expr}${s.tz ? ` (${s.tz})` : ''}`;
    if (s.kind === 'every') {
      // Support both everyMs (number) and every (string like "10m")
      if (s.everyMs) return t.cron.everyMinutes.replace('{n}', String(Math.round(s.everyMs / 60000)));
      if (s.every) return `${t.cron.scheduleKindEvery}: ${s.every}`;
      return t.cron.scheduleKindEvery;
    }
    if (s.kind === 'at') {
      // T9: support both ISO at (canonical) and atMs (legacy)
      if (s.at) return `${t.cron.oneTime}: ${new Date(s.at).toLocaleString()}`;
      if (s.atMs) return `${t.cron.oneTime}: ${new Date(s.atMs).toLocaleString()}`;
      return t.cron.oneTime;
    }
    return JSON.stringify(s);
  };

  const statusIcon = (s?: string) => {
    if (s === 'ok') return <CheckCircle2 size={13} className="text-emerald-500" />;
    if (s === 'error') return <XCircle size={13} className="text-red-500" />;
    if (s === 'skipped') return <AlertCircle size={13} className="text-amber-500" />;
    return <Clock size={13} className="text-gray-400" />;
  };

  const inputCls = `w-full px-3.5 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none transition-all ${modern ? 'focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500' : 'focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500'}`;

  // Schedule kind tab labels + icons
  const scheduleKinds: { kind: ScheduleKind; label: string; icon: React.ReactNode }[] = [
    { kind: 'cron', label: t.cron.scheduleKindCron, icon: <Clock size={13} /> },
    { kind: 'every', label: t.cron.scheduleKindEvery, icon: <Timer size={13} /> },
    { kind: 'at', label: t.cron.scheduleKindAt, icon: <CalendarClock size={13} /> },
  ];

  return (
    <div className={`space-y-6 ${modern ? 'page-modern' : ''}`}>
      <div className={`${modern ? 'page-modern-header' : 'flex items-center justify-between'}`}>
        <div>
          <h2 className={`${modern ? 'page-modern-title' : 'text-xl font-bold text-gray-900 dark:text-white tracking-tight'}`}>{t.cron.title}</h2>
          <p className={`${modern ? 'page-modern-subtitle' : 'text-sm text-gray-500 mt-1'}`}>{t.cron.subtitle}</p>
        </div>
        <MobileActionTray label={locale === 'zh-CN' ? '更多操作' : 'Actions'}>
          <button onClick={loadJobs} className={`${modern ? 'page-modern-action' : 'flex items-center gap-2 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 transition-colors shadow-sm'}`}>
            <RefreshCw size={14} />{t.cron.refreshList}
          </button>
          <button onClick={() => setShowCreate(!showCreate)}
            className={`${modern ? 'page-modern-accent' : 'flex items-center gap-2 px-4 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 shadow-sm shadow-violet-200 dark:shadow-none transition-all hover:shadow-md hover:shadow-violet-200 dark:hover:shadow-none'}`}>
            <Plus size={14} />{t.cron.newJob}
          </button>
        </MobileActionTray>
      </div>

      {msg && (
        <div className={`px-4 py-3 rounded-xl text-sm font-medium flex items-center gap-2 ${msg.includes('失败') || msg.includes('failed') || msg.includes('Failed') ? 'bg-red-50 dark:bg-red-900/30 text-red-600' : 'bg-emerald-50 dark:bg-emerald-900/30 text-emerald-600'}`}>
          {msg.includes('失败') || msg.includes('failed') || msg.includes('Failed') ? <XCircle size={16} /> : <CheckCircle2 size={16} />}
          {msg}
        </div>
      )}

      {/* Create form */}
      {showCreate && (
        <div className={`${modern ? 'page-modern-panel p-6 space-y-5 animate-in fade-in slide-in-from-top-4 duration-200' : 'bg-white dark:bg-gray-800 rounded-xl shadow-lg border border-violet-100 dark:border-violet-900/30 p-6 space-y-5 animate-in fade-in slide-in-from-top-4 duration-200'}`}>
          <div className="flex items-center gap-2 pb-4 border-b border-gray-100 dark:border-gray-700/50">
            <div className="p-1.5 rounded-lg bg-violet-100 dark:bg-violet-900/30 text-violet-600">
              <Plus size={16} />
            </div>
            <h3 className="font-bold text-gray-900 dark:text-white">{t.cron.createJob}</h3>
          </div>

          {/* Row 1: name */}
          <div className="space-y-1.5">
            <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300">{t.cron.jobName}</label>
            <input value={newName} onChange={e => setNewName(e.target.value)} placeholder={t.cron.jobNamePlaceholder}
              className={inputCls} />
          </div>

          {/* Row 2: agent + session mode (T6: linked constraints) */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
            <div className="space-y-1.5">
              <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300">{t.cron.agentId}</label>
              <select value={newAgentId} onChange={e => {
                setNewAgentId(e.target.value);
                // T6: non-default agent must use isolated
                if (e.target.value !== defaultAgent) setNewSessionMode('isolated');
              }}
                className={inputCls + ' font-mono'}>
                {agentOptions.map(id => <option key={id} value={id}>{id}</option>)}
              </select>
            </div>
            <div className="space-y-1.5">
              <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300">{t.cron.sessionMode}</label>
              <div className="flex gap-2">
                {(['main', 'isolated'] as const).map(mode => {
                  // T6: non-default agent → disable main
                  const disabled = mode === 'main' && newAgentId !== '' && newAgentId !== defaultAgent;
                  return (
                    <button key={mode} type="button"
                      disabled={disabled}
                      onClick={() => {
                        setNewSessionMode(mode);
                        if (mode === 'main' && newDeliveryMode === 'announce') {
                          setNewDeliveryMode('none');
                        }
                      }}
                      className={`flex-1 py-2.5 text-xs font-semibold rounded-lg border transition-all ${
                        disabled ? 'opacity-40 cursor-not-allowed bg-gray-100 dark:bg-gray-900 text-gray-400 border-gray-200 dark:border-gray-700' :
                        newSessionMode === mode
                          ? modern
                            ? 'bg-blue-600 text-white border-blue-600 shadow-sm'
                            : 'bg-violet-600 text-white border-violet-600 shadow-sm'
                          : 'bg-white dark:bg-gray-900 text-gray-600 dark:text-gray-400 border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
                      }`}>
                      {mode === 'main' ? t.cron.sessionModeMain : t.cron.sessionModeIsolated}
                    </button>
                  );
                })}
              </div>
              <p className="text-[10px] text-gray-400">
                {newSessionMode === 'isolated'
                  ? (locale === 'zh-CN' ? '每次创建独立会话 → agentTurn' : 'Fresh isolated session → agentTurn')
                  : (locale === 'zh-CN' ? '复用主会话上下文 → systemEvent' : 'Reuses main session → systemEvent')}
              </p>
            </div>
          </div>

          {/* Row 3: schedule kind tabs */}
          <div className="space-y-3">
            <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300">{t.cron.scheduleKind}</label>
            <div className="flex gap-2">
              {scheduleKinds.map(({ kind, label, icon }) => (
                <button key={kind} type="button"
                  onClick={() => setNewScheduleKind(kind)}
                  className={`flex items-center gap-1.5 px-3.5 py-2 text-xs font-semibold rounded-lg border transition-all ${
                    newScheduleKind === kind
                      ? modern
                        ? 'bg-blue-600 text-white border-blue-600 shadow-sm'
                        : 'bg-violet-600 text-white border-violet-600 shadow-sm'
                      : 'bg-white dark:bg-gray-900 text-gray-600 dark:text-gray-400 border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
                  }`}>
                  {icon}{label}
                </button>
              ))}
            </div>

            {/* Schedule-specific inputs */}
            {newScheduleKind === 'cron' && (
              <div className="space-y-1.5">
                <div className="relative">
                  <input value={newCron} onChange={e => setNewCron(e.target.value)} placeholder="0 9 * * *"
                    className={inputCls + ' font-mono pr-9'} />
                  <div className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400">
                    <Clock size={14} />
                  </div>
                </div>
                <p className="text-[10px] text-gray-400">{t.cron.cronHelp}</p>
              </div>
            )}

            {newScheduleKind === 'every' && (
              <div className="space-y-1.5">
                <label className="block text-[10px] font-medium text-gray-500">{t.cron.intervalMinutes}</label>
                <div className="flex items-center gap-3">
                  <input type="number" min={1} value={newEveryMin}
                    onChange={e => setNewEveryMin(Math.max(1, parseInt(e.target.value) || 1))}
                    className={inputCls + ' font-mono w-32'} />
                  <span className="text-sm text-gray-500">{locale === 'zh-CN' ? '分钟' : 'minutes'}</span>
                  <span className="text-xs text-gray-400 font-mono bg-gray-50 dark:bg-gray-900 px-2 py-1 rounded border border-gray-100 dark:border-gray-800">
                    everyMs: {newEveryMin * 60000}
                  </span>
                </div>
              </div>
            )}

            {newScheduleKind === 'at' && (
              <div className="space-y-1.5">
                <label className="block text-[10px] font-medium text-gray-500">{t.cron.atDateTime}</label>
                <input type="datetime-local" value={newAtDateTime}
                  onChange={e => setNewAtDateTime(e.target.value)}
                  className={inputCls + ' font-mono'} />
                <p className="text-[10px] text-gray-400">
                  {locale === 'zh-CN'
                    ? '一次性执行，到期运行后任务会自动删除；若要保留，请在 Advanced JSON 中设置 deleteAfterRun=false。'
                    : 'Runs once at the specified time, then is automatically deleted; set deleteAfterRun=false in Advanced JSON to retain it.'}
                </p>
              </div>
            )}
          </div>

          {/* Row 4: message */}
          <div className="space-y-1.5">
            <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300">{t.cron.messageContent}</label>
            <textarea value={newMessage} onChange={e => setNewMessage(e.target.value)} placeholder={t.cron.messagePlaceholder}
              rows={3} className={inputCls + ' resize-none'} />
          </div>

          {newSessionMode === 'isolated' && (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
              <div className="space-y-1.5">
                <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300">
                  {locale === 'zh-CN' ? '模型覆盖' : 'Model Override'}
                </label>
                <input
                  value={newModelOverride}
                  onChange={e => setNewModelOverride(e.target.value)}
                  placeholder="openai/gpt-5.4 或 opus"
                  className={inputCls + ' font-mono'}
                />
                <p className="text-[10px] text-gray-400">
                  {locale === 'zh-CN' ? '对应 OpenClaw cron add --model；若模型不在允许列表，运行时会回退。' : 'Maps to openclaw cron add --model; runtime falls back if the model is not allowed.'}
                </p>
              </div>
              <div className="space-y-1.5">
                <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300">
                  {locale === 'zh-CN' ? 'Thinking 等级' : 'Thinking Level'}
                </label>
                <select value={newThinkingLevel} onChange={e => setNewThinkingLevel(e.target.value)} className={inputCls}>
                  <option value="">{locale === 'zh-CN' ? '跟随默认' : 'Use default'}</option>
                  {['off', 'minimal', 'low', 'medium', 'high', 'xhigh'].map(level => (
                    <option key={level} value={level}>{level}</option>
                  ))}
                </select>
                <p className="text-[10px] text-gray-400">
                  {locale === 'zh-CN' ? '对应 OpenClaw cron add --thinking。' : 'Maps to openclaw cron add --thinking.'}
                </p>
              </div>
              <div className="space-y-1.5">
                <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300">
                  {locale === 'zh-CN' ? '工具白名单' : 'Tool Allow-List'}
                </label>
                <input
                  value={newToolsAllow}
                  onChange={e => setNewToolsAllow(e.target.value)}
                  placeholder="exec, read, write"
                  className={inputCls + ' font-mono'}
                />
                <p className="text-[10px] text-gray-400">
                  {locale === 'zh-CN' ? '对应 payload.toolsAllow / CLI 的 --tools；支持逗号分隔。' : 'Maps to payload.toolsAllow / CLI --tools; comma-separated.'}
                </p>
              </div>
              <div className="space-y-1.5">
                <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300">
                  {locale === 'zh-CN' ? '轻量上下文' : 'Light Context'}
                </label>
                <div className="flex items-center gap-3 p-3 rounded-lg border border-gray-100 dark:border-gray-800 bg-gray-50/30 dark:bg-gray-900/30">
                  <button
                    type="button"
                    onClick={() => setNewLightContext(prev => !prev)}
                    className={`relative w-9 h-5 rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-blue-500 ${newLightContext ? 'bg-blue-600' : 'bg-gray-300 dark:bg-gray-600'}`}
                  >
                    <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow-sm transition-transform ${newLightContext ? 'translate-x-4' : ''}`} />
                  </button>
                  <span className={`text-xs font-medium ${newLightContext ? 'text-blue-600 dark:text-blue-400' : 'text-gray-500'}`}>
                    {newLightContext ? (locale === 'zh-CN' ? '已启用' : 'Enabled') : (locale === 'zh-CN' ? '已禁用' : 'Disabled')}
                  </span>
                </div>
                <p className="text-[10px] text-gray-400">
                  {locale === 'zh-CN' ? '对应 OpenClaw cron add --light-context；跳过 workspace bootstrap 文件注入。' : 'Maps to openclaw cron add --light-context; skips workspace bootstrap file injection.'}
                </p>
              </div>
            </div>
          )}

          {/* Row 5: delivery mode + account + actions */}
          <div className="space-y-3 pt-2">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-1.5">
                <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300">
                  {locale === 'zh-CN' ? '投递方式' : 'Delivery Mode'}
                </label>
                <div className="flex gap-2">
                  {(['announce', 'webhook', 'none'] as const).map(mode => {
                    // OpenClaw constraint: sessionTarget=main only supports webhook/none delivery.
                    // Keep announce visible for clarity, but prevent invalid selection.
                    const disabled = mode === 'announce' && newSessionMode === 'main';
                    return (
                    <button key={mode} type="button"
                      disabled={disabled}
                      onClick={() => setNewDeliveryMode(mode)}
                      className={`flex-1 py-2 text-xs font-semibold rounded-lg border transition-all ${
                        disabled
                          ? 'opacity-40 cursor-not-allowed bg-gray-100 dark:bg-gray-900 text-gray-400 border-gray-200 dark:border-gray-700'
                          : newDeliveryMode === mode
                          ? modern ? 'bg-blue-600 text-white border-blue-600 shadow-sm' : 'bg-violet-600 text-white border-violet-600 shadow-sm'
                          : 'bg-white dark:bg-gray-900 text-gray-600 dark:text-gray-400 border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
                      }`}>
                      {mode === 'announce' ? (locale === 'zh-CN' ? '发送到通道' : 'Announce') : mode === 'webhook' ? 'Webhook' : (locale === 'zh-CN' ? '不投递' : 'None')}
                    </button>
                    );
                  })}
                </div>
                {newSessionMode === 'main' && (
                  <p className="text-[10px] text-amber-500">
                    {locale === 'zh-CN'
                      ? '⚠️ 主会话只支持 Webhook 或不投递，发送到通道仅限独立会话'
                      : '⚠️ Main session supports only Webhook or None; Announce is isolated-session only'}
                  </p>
                )}
              </div>
              {newDeliveryMode === 'announce' && newSessionMode !== 'main' && (
                <div className="space-y-1.5">
                  <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300">
                    {locale === 'zh-CN' ? '飞书账号 (accountId)' : 'Feishu Account (accountId)'}
                  </label>
                  {feishuAccounts.length > 0 ? (
                    <select value={newDeliveryAccountId} onChange={e => setNewDeliveryAccountId(e.target.value)}
                      className={inputCls + ' font-mono'}>
                      <option value="">{locale === 'zh-CN' ? '-- 默认账号 --' : '-- Default Account --'}</option>
                      {feishuAccounts.map(acct => <option key={acct} value={acct}>{acct}</option>)}
                    </select>
                  ) : (
                    <input value={newDeliveryAccountId} onChange={e => setNewDeliveryAccountId(e.target.value)}
                      placeholder={locale === 'zh-CN' ? '留空使用默认账号' : 'Leave empty for default account'}
                      className={inputCls + ' font-mono'} />
                  )}
                  <p className="text-[10px] text-amber-500">
                    {locale === 'zh-CN' ? '⚠️ 多账号场景必须指定，否则可能 open_id cross app' : '⚠️ Required for multi-account to avoid open_id cross app'}
                  </p>
                </div>
              )}
              {/* T8: Webhook URL input */}
              {newDeliveryMode === 'webhook' && (
                <div className="space-y-1.5">
                  <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300">
                    Webhook URL
                  </label>
                  <input value={newWebhookUrl} onChange={e => setNewWebhookUrl(e.target.value)}
                    placeholder="https://example.com/webhook"
                    type="url"
                    className={inputCls + ' font-mono'} />
                  <p className="text-[10px] text-gray-500">
                    {locale === 'zh-CN' ? '任务完成后将结果 POST 到此 URL' : 'Task result will be POSTed to this URL on completion'}
                  </p>
                </div>
              )}
            </div>
            <div className="flex items-center justify-end gap-3 pt-1">
              <button onClick={() => setShowCreate(false)} className="px-4 py-2 text-sm font-medium text-gray-600 dark:text-gray-300 bg-gray-100 dark:bg-gray-800 hover:bg-gray-200 dark:hover:bg-gray-700 rounded-lg transition-colors">{t.common.cancel}</button>
              <button onClick={createJob} className={`${modern ? 'page-modern-accent px-6 py-2 text-sm' : 'px-6 py-2 text-sm font-medium bg-violet-600 text-white hover:bg-violet-700 rounded-lg shadow-sm shadow-violet-200 dark:shadow-none transition-all hover:shadow-md hover:shadow-violet-200 dark:hover:shadow-none'}`}>{t.cron.createNow}</button>
            </div>
          </div>
        </div>
      )}

      {/* Jobs list */}
      {loading ? (
        <div className="flex flex-col items-center justify-center py-16 text-gray-400 gap-3">
          <RefreshCw size={32} className="animate-spin text-violet-500/50" />
          <p className="text-sm">{t.cron.loadingJobs}</p>
        </div>
      ) : jobs.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-gray-400 border-2 border-dashed border-gray-100 dark:border-gray-800 rounded-xl">
          <Clock size={32} className="opacity-20 mb-2" />
          <h3 className="font-semibold text-sm text-gray-500">{t.cron.noJobs}</h3>
          <p className="text-xs text-gray-400 mt-1">{t.cron.noJobsHint}</p>
        </div>
      ) : (
        <div className="grid gap-3">
          {jobs.map(job => {
            const effectiveAgentId = resolveAgentId(job, defaultAgent);
            const effectiveSessionMode = resolveSessionMode(job);
            return (
              <div key={job.id} className={`${modern ? 'page-modern-panel hover:shadow-md transition-all group overflow-hidden' : 'bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 hover:shadow-md transition-all group overflow-hidden'}`}>
                <div className="flex items-center gap-4 p-4 cursor-pointer" onClick={() => setExpandedId(expandedId === job.id ? null : job.id)}>
                  <div className={`p-2.5 rounded-xl shrink-0 transition-colors border ${job.enabled ? 'bg-blue-100/80 dark:bg-blue-900/20 border-blue-100 dark:border-blue-800/30 text-blue-600 dark:text-blue-300' : 'bg-gray-100 dark:bg-gray-800 border-transparent text-gray-400'}`}>
                    <Clock size={20} />
                  </div>

                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-3 mb-1">
                      <span className="text-base font-bold text-gray-900 dark:text-white truncate">{job.name}</span>
                      <span className={`text-[10px] px-2 py-0.5 rounded-full font-bold uppercase tracking-wider ${job.enabled ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-400' : 'bg-gray-100 dark:bg-gray-700 text-gray-500'}`}>
                        {job.enabled ? t.common.running : t.common.paused}
                      </span>
                    </div>
                    <div className="flex items-center gap-4 text-xs text-gray-500 flex-wrap">
                      <span className="font-mono bg-gray-50 dark:bg-gray-900/50 px-1.5 py-0.5 rounded border border-gray-100 dark:border-gray-800">{formatSchedule(job.schedule)}</span>
                      <span className="font-mono text-blue-500/80">@{effectiveAgentId}</span>
                      {job.state.lastRunAtMs && (
                        <span className="flex items-center gap-1.5">
                          {statusIcon(job.state.lastStatus)}
                          {t.cron.lastRun}: {new Date(job.state.lastRunAtMs).toLocaleString()}
                        </span>
                      )}
                      {/* T3: delivery status badge in list */}
                      {(() => {
                        const dStatus = resolveDeliveryStatus(job);
                        if (!dStatus || dStatus === 'not-requested') return null;
                        const colors = dStatus === 'delivered'
                          ? 'bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600 dark:text-emerald-400 border-emerald-100 dark:border-emerald-800/30'
                          : dStatus === 'not-delivered'
                          ? 'bg-red-50 dark:bg-red-900/20 text-red-500 dark:text-red-400 border-red-100 dark:border-red-800/30'
                          : 'bg-gray-50 dark:bg-gray-900/50 text-gray-500 border-gray-100 dark:border-gray-800';
                        return (
                          <span className={`text-[10px] px-1.5 py-0.5 rounded border font-mono ${colors}`}>
                            {locale === 'zh-CN' ? '\u6295\u9012' : 'delivery'}: {dStatus}
                          </span>
                        );
                      })()}
                      {/* T3: delivery accountId hint when present */}
                      {resolveDelivery(job).accountId && (
                        <span className="text-[10px] text-amber-500 font-mono">
                          [{resolveDelivery(job).accountId}]
                        </span>
                      )}
                      {(() => {
                        const d = resolveDelivery(job);
                        if (d.mode !== 'webhook') return null;
                        const target = (d.to || d.url || '').trim();
                        if (!target) {
                          return (
                            <span className="text-[10px] px-1.5 py-0.5 rounded border font-mono bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400 border-blue-100 dark:border-blue-800/30">
                              webhook
                            </span>
                          );
                        }
                        return (
                          <span className="text-[10px] px-1.5 py-0.5 rounded border font-mono bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400 border-blue-100 dark:border-blue-800/30 max-w-[360px] truncate">
                            webhook: {target}
                          </span>
                        );
                      })()}
                    </div>
                  </div>

                  <div className="flex items-center gap-2 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity" onClick={e => e.stopPropagation()}>
                    <button onClick={() => toggleJob(job.id)}
                      className={`p-2 rounded-lg transition-colors ${job.enabled ? 'text-amber-500 hover:bg-amber-50 dark:hover:bg-amber-900/20' : 'text-emerald-500 hover:bg-emerald-50 dark:hover:bg-emerald-900/20'}`}
                      title={job.enabled ? t.common.paused : t.common.enabled}>
                      {job.enabled ? <Pause size={16} /> : <Play size={16} />}
                    </button>
                    <button onClick={() => deleteJob(job.id)} className="p-2 rounded-lg text-red-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors" title={t.common.delete}>
                      <Trash2 size={16} />
                    </button>
                  </div>

                  <ChevronDown size={16} className={`text-gray-300 transition-transform duration-200 ${expandedId === job.id ? 'rotate-180 text-gray-500' : ''}`} />
                </div>

                {expandedId === job.id && (
                  <div className="px-6 pb-6 pt-2 border-t border-gray-50 dark:border-gray-800 bg-gray-50/30 dark:bg-gray-900/20">
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-y-4 gap-x-8 text-sm">
                      <div className="space-y-1">
                        <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">{t.cron.agentId}</span>
                        <div className="font-mono text-blue-600 dark:text-blue-400">{effectiveAgentId}</div>
                      </div>
                      <div className="space-y-1">
                        <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">{t.cron.sessionMode}</span>
                        <div className="font-mono text-gray-700 dark:text-gray-300">{effectiveSessionMode}</div>
                      </div>
                      <div className="space-y-1">
                        <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">{t.cron.wakeMode}</span>
                        <div className="font-mono text-gray-700 dark:text-gray-300">{job.wakeMode}</div>
                      </div>
                      <div className="space-y-1">
                        <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">{t.cron.jobType}</span>
                        <div className="font-mono text-gray-700 dark:text-gray-300">
                          {job.payload.kind}
                          {/* T6: show constraint hint */}
                          <span className="ml-1 text-[10px] text-gray-400">
                            ({effectiveSessionMode === 'isolated' ? 'agentTurn' : 'systemEvent'})
                          </span>
                        </div>
                      </div>
                      {job.payload.model && (
                        <div className="space-y-1">
                          <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
                            {locale === 'zh-CN' ? '模型覆盖' : 'Model Override'}
                          </span>
                          <div className="font-mono text-gray-700 dark:text-gray-300 break-all">{job.payload.model}</div>
                        </div>
                      )}
                      {job.payload.thinking && (
                        <div className="space-y-1">
                          <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
                            {locale === 'zh-CN' ? 'Thinking' : 'Thinking'}
                          </span>
                          <div className="font-mono text-gray-700 dark:text-gray-300">{job.payload.thinking}</div>
                        </div>
                      )}
                      {typeof job.payload.lightContext === 'boolean' && (
                        <div className="space-y-1">
                          <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
                            {locale === 'zh-CN' ? '轻量上下文' : 'Light Context'}
                          </span>
                          <div className="font-mono text-gray-700 dark:text-gray-300">{job.payload.lightContext ? 'true' : 'false'}</div>
                        </div>
                      )}
                      {Array.isArray(job.payload.toolsAllow) && job.payload.toolsAllow.length > 0 && (
                        <div className="md:col-span-2 space-y-1">
                          <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
                            {locale === 'zh-CN' ? '工具白名单' : 'Tool Allow-List'}
                          </span>
                          <div className="font-mono text-gray-700 dark:text-gray-300 break-all">{job.payload.toolsAllow.join(', ')}</div>
                        </div>
                      )}

                      {/* T2: Delivery info (canonical + legacy fallback) */}
                      {(() => {
                        const d = resolveDelivery(job);
                        const dStatus = resolveDeliveryStatus(job);
                        return (
                          <>
                            <div className="space-y-1">
                              <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
                                {locale === 'zh-CN' ? '\u6295\u9012\u65b9\u5f0f' : 'Delivery Mode'}
                              </span>
                              <div className={`font-medium ${d.mode === 'announce' ? 'text-emerald-600' : d.mode === 'webhook' ? 'text-blue-600 dark:text-blue-400' : 'text-gray-500'}`}>
                                {d.mode === 'announce'
                                  ? (locale === 'zh-CN' ? '\u53d1\u9001\u5230\u901a\u9053' : 'Announce')
                                  : d.mode === 'webhook'
                                    ? 'Webhook'
                                    : (locale === 'zh-CN' ? '\u4e0d\u6295\u9012' : 'None')}
                              </div>
                            </div>
                            {d.mode === 'webhook' && (d.to || d.url) && (
                              <div className="md:col-span-2 space-y-1">
                                <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
                                  {locale === 'zh-CN' ? 'Webhook 目标' : 'Webhook Target'}
                                </span>
                                <div className="font-mono text-blue-600 dark:text-blue-400 break-all">
                                  {(d.to || d.url) as string}
                                </div>
                              </div>
                            )}
                            {d.accountId && (
                              <div className="space-y-1">
                                <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
                                  {locale === 'zh-CN' ? '\u98de\u4e66\u8d26\u53f7' : 'Feishu Account'}
                                </span>
                                <div className="font-mono text-amber-600 dark:text-amber-400">{d.accountId}</div>
                              </div>
                            )}
                            {dStatus && (
                              <div className="space-y-1">
                                <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
                                  {locale === 'zh-CN' ? '\u6295\u9012\u72b6\u6001' : 'Delivery Status'}
                                </span>
                                <div className={`font-mono ${dStatus === 'delivered' ? 'text-emerald-600' : dStatus === 'not-delivered' ? 'text-red-500' : 'text-gray-500'}`}>
                                  {dStatus}
                                </div>
                              </div>
                            )}
                          </>
                        );
                      })()}

                      {/* T7: Execution error with parsed hint */}
                      {job.state.lastError && (() => {
                        const hint = parseErrorHint(job.state.lastError, locale);
                        return (
                          <div className="md:col-span-2 bg-red-50 dark:bg-red-900/20 p-3 rounded-lg border border-red-100 dark:border-red-900/30">
                            <span className="text-xs font-bold text-red-500 uppercase tracking-wider block mb-1">{t.cron.execError}</span>
                            <div className="text-xs text-red-600 dark:text-red-400 font-mono break-all">{job.state.lastError}</div>
                            {hint && <div className="mt-1 text-xs text-red-700 dark:text-red-300 bg-red-100 dark:bg-red-900/40 px-2 py-1 rounded">\u2139\uFE0F {hint}</div>}
                          </div>
                        );
                      })()}

                      {/* T7: Delivery error with parsed hint */}
                      {job.state.lastDeliveryError && (() => {
                        const hint = parseErrorHint(job.state.lastDeliveryError, locale);
                        return (
                          <div className="md:col-span-2 bg-amber-50 dark:bg-amber-900/20 p-3 rounded-lg border border-amber-100 dark:border-amber-900/30">
                            <span className="text-xs font-bold text-amber-500 uppercase tracking-wider block mb-1">
                              {locale === 'zh-CN' ? '\u6295\u9012\u9519\u8bef' : 'Delivery Error'}
                            </span>
                            <div className="text-xs text-amber-600 dark:text-amber-400 font-mono break-all">{job.state.lastDeliveryError}</div>
                            {hint && <div className="mt-1 text-xs text-amber-700 dark:text-amber-300 bg-amber-100 dark:bg-amber-900/40 px-2 py-1 rounded">\u2139\uFE0F {hint}</div>}
                          </div>
                        );
                      })()}

                      {(job.payload.text || job.payload.message) && (
                        <div className="md:col-span-2 space-y-1.5">
                          <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">{t.cron.messageContent}</span>
                          <div className={`${modern ? 'p-3 rounded-xl bg-white/75 dark:bg-slate-900/55 border border-blue-100/70 dark:border-slate-700/70 text-xs font-mono text-gray-600 dark:text-gray-300 whitespace-pre-wrap shadow-sm backdrop-blur-xl' : 'p-3 rounded-lg bg-white dark:bg-gray-900 border border-gray-100 dark:border-gray-800 text-xs font-mono text-gray-600 dark:text-gray-300 whitespace-pre-wrap shadow-sm'}`}>
                            {job.payload.text || job.payload.message}
                          </div>
                        </div>
                      )}
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

export default memo(CronJobsPage);

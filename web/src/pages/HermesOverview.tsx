import { useEffect, useState } from 'react';
import { useNavigate, useOutletContext } from 'react-router-dom';
import { api } from '../lib/api';
import { Brain, Download, ExternalLink, RefreshCw, Terminal, Wrench, CheckCircle2, AlertTriangle, Bell, Radio, MessageSquare, GitBranch, Settings } from 'lucide-react';
import { useI18n } from '../i18n';

interface HermesStatus {
  installed?: boolean;
  configured?: boolean;
  running?: boolean;
  gatewayRunning?: boolean;
  version?: string;
  binaryPath?: string;
  homeDir?: string;
  configPath?: string;
  envPath?: string;
  stateDir?: string;
  pythonVersion?: string;
  docsUrl?: string;
  repoUrl?: string;
}

interface HermesOverviewData {
  status?: HermesStatus;
  warnings?: string[];
  platforms?: { configuredCount?: number; enabledCount?: number; platforms?: Array<{ id: string; label: string; runtimeStatus?: string }> };
  storage?: { conversationCount?: number; sessionArtifactCount?: number; previewSessions?: Array<{ title: string }> };
  doctor?: { updatedAt?: string; taskStatus?: string };
  actions?: Array<{ id: string; label: string; command: string }>;
}

export default function HermesOverview() {
  const { locale } = useI18n();
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const navigate = useNavigate();
  const modern = uiMode === 'modern';
  const [status, setStatus] = useState<HermesStatus | null>(null);
  const [overview, setOverview] = useState<HermesOverviewData | null>(null);
  const [loading, setLoading] = useState(true);
  const [storageLoading, setStorageLoading] = useState(false);
  const [installing, setInstalling] = useState(false);
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');

  const loadStatus = async () => {
    setLoading(true);
    setErr('');
    try {
      const overviewRes = await api.getHermesOverview();
      if (overviewRes.ok) {
        setOverview(overviewRes.overview || {});
        setStatus(overviewRes.overview?.status || {});
        setStorageLoading(true);
        void api.getHermesStorage()
          .then(storageRes => {
            if (storageRes?.ok) {
              setOverview(prev => ({ ...(prev || {}), storage: storageRes.storage || {} }));
            }
          })
          .catch(() => {})
          .finally(() => setStorageLoading(false));
      }
    } catch {
      setErr(locale === 'zh-CN' ? '加载 Hermes 状态失败' : 'Failed to load Hermes status');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { loadStatus(); }, []);

  const handleInstall = async () => {
    setInstalling(true);
    setMsg('');
    setErr('');
    try {
      const r = await api.installSoftware('hermes');
      if (r?.ok) {
        setMsg(locale === 'zh-CN' ? 'Hermes 安装任务已创建，请在消息中心或后台任务里查看进度。' : 'Hermes install task created. Check Message Center or Tasks for progress.');
      } else {
        setErr(r?.error || (locale === 'zh-CN' ? '安装 Hermes 失败' : 'Failed to install Hermes'));
      }
    } catch {
      setErr(locale === 'zh-CN' ? '安装 Hermes 失败' : 'Failed to install Hermes');
    } finally {
      setInstalling(false);
    }
  };

  const open = (url?: string) => {
    if (!url) return;
    window.open(url, '_blank', 'noopener,noreferrer');
  };

  const cards = [
    {
      label: locale === 'zh-CN' ? '安装状态' : 'Install Status',
      value: status?.installed ? (locale === 'zh-CN' ? '已安装' : 'Installed') : (locale === 'zh-CN' ? '未安装' : 'Not Installed'),
      tone: status?.installed ? 'emerald' : 'amber',
    },
    {
      label: locale === 'zh-CN' ? '配置状态' : 'Config Status',
      value: status?.configured ? (locale === 'zh-CN' ? '已配置' : 'Configured') : (locale === 'zh-CN' ? '未配置' : 'Not Configured'),
      tone: status?.configured ? 'emerald' : 'slate',
    },
    {
      label: locale === 'zh-CN' ? '运行状态' : 'Runtime',
      value: status?.gatewayRunning ? (locale === 'zh-CN' ? 'Gateway 运行中' : 'Gateway Running') : status?.running ? (locale === 'zh-CN' ? 'Hermes 进程运行中' : 'Hermes Running') : (locale === 'zh-CN' ? '未运行' : 'Stopped'),
      tone: status?.running ? 'blue' : 'slate',
    },
    {
      label: locale === 'zh-CN' ? 'Python' : 'Python',
      value: status?.pythonVersion || (locale === 'zh-CN' ? '未检测到' : 'Not detected'),
      tone: status?.pythonVersion ? 'violet' : 'slate',
    },
    {
      label: locale === 'zh-CN' ? '已启用平台' : 'Enabled Platforms',
      value: String(overview?.platforms?.enabledCount ?? 0),
      tone: (overview?.platforms?.enabledCount ?? 0) > 0 ? 'blue' : 'slate',
    },
    {
      label: locale === 'zh-CN' ? '会话数' : 'Sessions',
      value: storageLoading
        ? (locale === 'zh-CN' ? '加载中...' : 'Loading...')
        : String(overview?.storage?.conversationCount ?? overview?.storage?.sessionArtifactCount ?? 0),
      tone: (overview?.storage?.conversationCount ?? overview?.storage?.sessionArtifactCount ?? 0) > 0 ? 'emerald' : 'slate',
    },
  ];

  const cardToneClass = (tone: string) => {
    switch (tone) {
      case 'emerald':
        return 'border-emerald-100 dark:border-emerald-900/30';
      case 'amber':
        return 'border-amber-100 dark:border-amber-900/30';
      case 'blue':
        return 'border-blue-100 dark:border-blue-900/30';
      case 'violet':
        return 'border-violet-100 dark:border-violet-900/30';
      default:
        return 'border-gray-100 dark:border-gray-700/50';
    }
  };
  const runtimeTone = !status?.installed ? 'amber' : status?.gatewayRunning || status?.running ? 'emerald' : status?.configured ? 'amber' : 'slate';
  const runtimeToneClass = runtimeTone === 'emerald'
    ? 'border-emerald-200/60 dark:border-emerald-800/40 bg-[linear-gradient(135deg,rgba(255,255,255,0.72),rgba(236,253,245,0.78))] dark:bg-[linear-gradient(135deg,rgba(6,78,59,0.26),rgba(15,23,42,0.78))]'
    : runtimeTone === 'amber'
      ? 'border-amber-200/70 dark:border-amber-800/40 bg-[linear-gradient(135deg,rgba(255,255,255,0.72),rgba(255,251,235,0.86))] dark:bg-[linear-gradient(135deg,rgba(120,53,15,0.24),rgba(15,23,42,0.82))]'
      : 'border-slate-200/70 dark:border-slate-700/40 bg-[linear-gradient(135deg,rgba(255,255,255,0.72),rgba(241,245,249,0.82))] dark:bg-[linear-gradient(135deg,rgba(30,41,59,0.4),rgba(15,23,42,0.82))]';

  return (
    <div className={`space-y-6 ${modern ? 'page-modern' : ''}`}>
      <div className={`${modern ? 'page-modern-header' : 'flex items-center justify-between'}`}>
        <div>
          <h2 className={`${modern ? 'page-modern-title text-xl' : 'text-xl font-bold text-gray-900 dark:text-white'}`}>Hermes</h2>
          <p className={`${modern ? 'page-modern-subtitle mt-1 text-sm' : 'text-sm text-gray-500 mt-1'}`}>
            {locale === 'zh-CN'
              ? 'Hermes 作为与 OpenClaw 并列的独立 Agent 控制台接入，状态、平台、日志、任务、动作与 Profiles 都可以在这里统一管理。'
              : 'Hermes is integrated as an independent agent console alongside OpenClaw, with status, platforms, logs, tasks, actions, and profiles managed here.'}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={loadStatus} className={`${modern ? 'page-modern-action px-3 py-2 text-xs' : 'px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-800'}`}>
            <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          </button>
          <button onClick={handleInstall} disabled={installing} className={`${modern ? 'page-modern-accent px-4 py-2 text-xs disabled:opacity-50' : 'px-4 py-2 text-xs rounded-lg bg-blue-600 text-white disabled:opacity-50'} inline-flex items-center gap-2`}>
            {installing ? <RefreshCw size={14} className="animate-spin" /> : <Download size={14} />}
            {locale === 'zh-CN' ? '安装 Hermes' : 'Install Hermes'}
          </button>
        </div>
      </div>

      <div className={`flex items-center justify-between gap-3 rounded-[28px] border px-5 py-4 backdrop-blur-xl shadow-[0_16px_36px_rgba(15,23,42,0.06)] ${runtimeToneClass}`}>
        <div className="flex items-center gap-3">
          <span className="flex h-3 w-3 relative">
            <span className={`absolute inline-flex h-full w-full animate-ping rounded-full opacity-70 ${runtimeTone === 'emerald' ? 'bg-emerald-400' : runtimeTone === 'amber' ? 'bg-amber-400' : 'bg-slate-400'}`} />
            <span className={`relative inline-flex h-3 w-3 rounded-full ${runtimeTone === 'emerald' ? 'bg-emerald-500' : runtimeTone === 'amber' ? 'bg-amber-500' : 'bg-slate-500'}`} />
          </span>
          <div>
            <div className="text-sm font-semibold text-slate-900 dark:text-white">
              {locale === 'zh-CN'
                ? (status?.gatewayRunning ? 'Hermes Gateway 正在运行' : status?.running ? 'Hermes 进程正在运行' : status?.installed ? 'Hermes 已安装，等待运行' : 'Hermes 尚未安装')
                : (status?.gatewayRunning ? 'Hermes gateway is running' : status?.running ? 'Hermes process is running' : status?.installed ? 'Hermes is installed and waiting to run' : 'Hermes is not installed yet')}
            </div>
            <div className="mt-1 text-xs text-slate-500 dark:text-slate-300">
              {locale === 'zh-CN'
                ? `Version ${status?.version || '-'} · Doctor ${overview?.doctor?.taskStatus || '-'}`
                : `Version ${status?.version || '-'} · Doctor ${overview?.doctor?.taskStatus || '-'}`}
            </div>
          </div>
        </div>
        <div className="hidden md:flex items-center gap-2 text-xs text-slate-500 dark:text-slate-300">
          <span>{locale === 'zh-CN' ? '平台' : 'Platforms'} {overview?.platforms?.enabledCount ?? 0}</span>
          <span>·</span>
          <span>{locale === 'zh-CN' ? '动作' : 'Actions'} {(overview?.actions || []).length}</span>
          <span>·</span>
          <span>{locale === 'zh-CN' ? '会话' : 'Sessions'} {storageLoading ? '...' : (overview?.storage?.conversationCount ?? overview?.storage?.sessionArtifactCount ?? 0)}</span>
        </div>
      </div>

      {msg && (
        <div className="rounded-2xl border border-emerald-100 bg-emerald-50/80 px-4 py-3 text-sm text-emerald-700 dark:border-emerald-900/30 dark:bg-emerald-900/10 dark:text-emerald-300">
          {msg}
        </div>
      )}
      {err && (
        <div className="rounded-2xl border border-red-100 bg-red-50/80 px-4 py-3 text-sm text-red-700 dark:border-red-900/30 dark:bg-red-900/10 dark:text-red-300">
          {err}
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-6 gap-4">
        {cards.map(card => (
          <div key={card.label} className={`${modern ? 'page-modern-card' : 'bg-white dark:bg-gray-800'} rounded-2xl border p-4 ${cardToneClass(card.tone)}`}>
            <div className="text-xs font-semibold uppercase tracking-wider text-gray-500">{card.label}</div>
            <div className="mt-3 text-lg font-bold text-gray-900 dark:text-white">{card.value}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-5 xl:col-span-2 space-y-4`}>
          <div className="flex items-center gap-2 text-gray-900 dark:text-white font-semibold">
            <Brain size={18} className="text-blue-500" />
            {locale === 'zh-CN' ? '当前运行时信息' : 'Runtime Details'}
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
            {[
              { label: 'Version', value: status?.version || '-' },
              { label: 'Binary', value: status?.binaryPath || '-' },
              { label: 'Home', value: status?.homeDir || '-' },
              { label: 'Config', value: status?.configPath || '-' },
              { label: 'Env', value: status?.envPath || '-' },
              { label: 'State', value: status?.stateDir || '-' },
            ].map(item => (
              <div key={item.label} className="rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 px-4 py-3">
                <div className="text-[11px] uppercase tracking-wider text-gray-500">{item.label}</div>
                <div className="mt-1 font-mono break-all text-gray-800 dark:text-gray-100">{item.value}</div>
              </div>
            ))}
          </div>
        </div>

        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-5 space-y-4`}>
          <div className="flex items-center gap-2 text-gray-900 dark:text-white font-semibold">
            <Wrench size={18} className="text-blue-500" />
            {locale === 'zh-CN' ? '管理入口' : 'Management Links'}
          </div>
          <button onClick={() => navigate('/hermes/health')} className="w-full inline-flex items-center justify-between rounded-xl border border-gray-100 dark:border-gray-700/50 px-4 py-3 text-sm hover:bg-gray-50 dark:hover:bg-gray-900/40">
            <span>{locale === 'zh-CN' ? '健康与检查' : 'Health & Checks'}</span>
            <Bell size={14} />
          </button>
          <button onClick={() => navigate('/hermes/platforms')} className="w-full inline-flex items-center justify-between rounded-xl border border-gray-100 dark:border-gray-700/50 px-4 py-3 text-sm hover:bg-gray-50 dark:hover:bg-gray-900/40">
            <span>{locale === 'zh-CN' ? '平台管理' : 'Platforms'}</span>
            <Radio size={14} />
          </button>
          <button onClick={() => navigate('/hermes/logs')} className="w-full inline-flex items-center justify-between rounded-xl border border-gray-100 dark:border-gray-700/50 px-4 py-3 text-sm hover:bg-gray-50 dark:hover:bg-gray-900/40">
            <span>{locale === 'zh-CN' ? '日志与诊断' : 'Logs & Diagnostics'}</span>
            <Terminal size={14} />
          </button>
          <button onClick={() => navigate('/hermes/actions')} className="w-full inline-flex items-center justify-between rounded-xl border border-gray-100 dark:border-gray-700/50 px-4 py-3 text-sm hover:bg-gray-50 dark:hover:bg-gray-900/40">
            <span>{locale === 'zh-CN' ? '动作中心' : 'Actions'}</span>
            <Wrench size={14} />
          </button>
          <button onClick={() => navigate('/hermes/tasks')} className="w-full inline-flex items-center justify-between rounded-xl border border-gray-100 dark:border-gray-700/50 px-4 py-3 text-sm hover:bg-gray-50 dark:hover:bg-gray-900/40">
            <span>{locale === 'zh-CN' ? '任务与账本' : 'Tasks & Ledger'}</span>
            <Bell size={14} />
          </button>
          <button onClick={() => navigate('/hermes/sessions')} className="w-full inline-flex items-center justify-between rounded-xl border border-gray-100 dark:border-gray-700/50 px-4 py-3 text-sm hover:bg-gray-50 dark:hover:bg-gray-900/40">
            <span>{locale === 'zh-CN' ? '会话与 Usage' : 'Sessions & Usage'}</span>
            <MessageSquare size={14} />
          </button>
          <button onClick={() => navigate('/hermes/personality')} className="w-full inline-flex items-center justify-between rounded-xl border border-gray-100 dark:border-gray-700/50 px-4 py-3 text-sm hover:bg-gray-50 dark:hover:bg-gray-900/40">
            <span>{locale === 'zh-CN' ? '人格与路由' : 'Personality & Routing'}</span>
            <GitBranch size={14} />
          </button>
          <button onClick={() => navigate('/hermes/profiles')} className="w-full inline-flex items-center justify-between rounded-xl border border-gray-100 dark:border-gray-700/50 px-4 py-3 text-sm hover:bg-gray-50 dark:hover:bg-gray-900/40">
            <span>{locale === 'zh-CN' ? 'Profiles 编辑' : 'Profiles Editor'}</span>
            <Settings size={14} />
          </button>
          <button onClick={() => navigate('/hermes/config')} className="w-full inline-flex items-center justify-between rounded-xl border border-gray-100 dark:border-gray-700/50 px-4 py-3 text-sm hover:bg-gray-50 dark:hover:bg-gray-900/40">
            <span>{locale === 'zh-CN' ? '结构化配置' : 'Structured Config'}</span>
            <Settings size={14} />
          </button>
          <button onClick={() => open(status?.docsUrl)} className="w-full inline-flex items-center justify-between rounded-xl border border-gray-100 dark:border-gray-700/50 px-4 py-3 text-sm hover:bg-gray-50 dark:hover:bg-gray-900/40">
            <span>{locale === 'zh-CN' ? '官方文档' : 'Documentation'}</span>
            <ExternalLink size={14} />
          </button>
          <button onClick={() => open(status?.repoUrl)} className="w-full inline-flex items-center justify-between rounded-xl border border-gray-100 dark:border-gray-700/50 px-4 py-3 text-sm hover:bg-gray-50 dark:hover:bg-gray-900/40">
            <span>{locale === 'zh-CN' ? 'GitHub 仓库' : 'GitHub Repository'}</span>
            <ExternalLink size={14} />
          </button>
          <div className="rounded-xl border border-blue-100 bg-blue-50/70 px-4 py-3 text-xs leading-6 text-blue-700 dark:border-blue-900/30 dark:bg-blue-900/10 dark:text-blue-300">
            {locale === 'zh-CN'
              ? 'Hermes 与 OpenClaw 现在是并列视图；切到 Hermes 后，常见的状态、运行、日志、任务和配置操作都可以在这一套面板里完成，不会影响现有 OpenClaw 配置。'
              : 'Hermes and OpenClaw now live as peer views. Once switched to Hermes, common status, runtime, logs, tasks, and configuration work can all stay inside this board without affecting OpenClaw.'}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-5 space-y-4`}>
          <div className="text-gray-900 dark:text-white font-semibold">{locale === 'zh-CN' ? '当前告警' : 'Current Warnings'}</div>
          <div className="space-y-3">
            {(overview?.warnings || []).length === 0 ? (
              <div className="text-sm text-gray-500">{locale === 'zh-CN' ? '暂无 Hermes 告警' : 'No Hermes warnings'}</div>
            ) : (overview?.warnings || []).map((warning, index) => (
              <div key={`${warning}-${index}`} className="rounded-xl border border-amber-100 dark:border-amber-900/30 bg-amber-50/70 dark:bg-amber-900/10 px-4 py-3 text-sm text-amber-800 dark:text-amber-200">
                {warning}
              </div>
            ))}
          </div>
        </div>

        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-5 space-y-4`}>
          <div className="text-gray-900 dark:text-white font-semibold">{locale === 'zh-CN' ? '最近活动摘要' : 'Recent Summary'}</div>
          <div className="space-y-3 text-sm text-gray-700 dark:text-gray-300">
            <div>{locale === 'zh-CN' ? 'Doctor 状态：' : 'Doctor Status: '}{overview?.doctor?.taskStatus || '-'}</div>
            <div>{locale === 'zh-CN' ? 'Doctor 时间：' : 'Doctor Updated: '}{overview?.doctor?.updatedAt || '-'}</div>
            <div>{locale === 'zh-CN' ? '样例会话：' : 'Sample Session: '}{storageLoading ? (locale === 'zh-CN' ? '加载中...' : 'Loading...') : (overview?.storage?.previewSessions?.[0]?.title || '-')}</div>
            <div>{locale === 'zh-CN' ? '可执行动作：' : 'Available Actions: '}{(overview?.actions || []).map(item => item.label).slice(0, 3).join(' / ') || '-'}</div>
          </div>
        </div>
      </div>

      <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-5 space-y-4`}>
        <div className="flex items-center gap-2 text-gray-900 dark:text-white font-semibold">
          <Terminal size={18} className="text-blue-500" />
          {locale === 'zh-CN' ? '推荐命令' : 'Recommended Commands'}
        </div>
        <pre className="rounded-2xl bg-gray-950 text-gray-100 p-4 overflow-x-auto text-xs leading-6 font-mono">{`hermes
hermes setup
hermes model
hermes gateway
hermes doctor
hermes claw migrate`}</pre>
        <div className="flex items-start gap-3 rounded-xl border border-amber-100 bg-amber-50/70 px-4 py-3 text-sm text-amber-800 dark:border-amber-900/30 dark:bg-amber-900/10 dark:text-amber-200">
          {status?.installed ? <CheckCircle2 size={18} className="mt-0.5 shrink-0" /> : <AlertTriangle size={18} className="mt-0.5 shrink-0" />}
          <div>
            {locale === 'zh-CN'
              ? '如果 Hermes 已安装，建议先执行 `hermes setup` 完成模型、工具和消息网关初始化；如果你之前主要在用 OpenClaw，也可以用 `hermes claw migrate` 迁移已有数据。'
              : 'If Hermes is installed, start with `hermes setup` to initialize models, tools, and messaging. If you are coming from OpenClaw, `hermes claw migrate` is the recommended migration path.'}
          </div>
        </div>
      </div>
    </div>
  );
}

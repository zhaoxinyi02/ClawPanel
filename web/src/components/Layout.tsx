import { memo, useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { Outlet, NavLink, useLocation, useNavigate } from 'react-router-dom';
import {
  LayoutDashboard, ScrollText, Radio, Sparkles, Clock, Settings,
  Moon, Sun, LogOut, Menu, FolderOpen, Languages, MessageSquare,
  RotateCw, RefreshCw, Power, Puzzle, Bot, Search, Bell, ChevronDown, GitBranch, Network, Activity,
} from 'lucide-react';
import { useI18n } from '../i18n';
import AIAssistant from './AIAssistant';
import MessageCenter, { TaskInfo } from './MessageCenter';
import { api } from '../lib/api';
import { resolveOpenClawRuntime } from '../lib/openclawRuntime';

interface Props { onLogout: () => void; napcatStatus: any; wechatStatus?: any; openclawStatus?: any; processStatus?: any; wsMessages?: any[]; }

const DISPLAY_CHANNEL_IDS = new Set([
  'qq', 'wechat', 'whatsapp', 'telegram', 'discord', 'irc', 'slack', 'signal', 'googlechat',
  'bluebubbles', 'imessage', 'webchat', 'feishu', 'qqbot', 'dingtalk', 'wecom', 'wecom-app',
  'msteams', 'mattermost', 'line', 'matrix', 'nextcloud-talk', 'nostr', 'qa-channel',
  'synology-chat', 'tlon', 'twitch', 'voice-call', 'zalo', 'zalouser', 'openclaw-weixin',
]);

function mapWorkflowRunToTask(run: any): TaskInfo {
  let status: TaskInfo['status'] = 'pending';
  if (run?.status === 'completed') status = 'success';
  else if (run?.status === 'failed') status = 'failed';
  else if (run?.status === 'cancelled') status = 'canceled';
  else if (run?.status === 'running' || run?.status === 'paused' || run?.status === 'waiting_for_user' || run?.status === 'waiting_for_approval') status = 'running';

  const steps = Array.isArray(run?.steps) ? run.steps : [];
  const finished = steps.filter((step: any) => step?.status === 'completed' || step?.status === 'skipped').length;
  const progress = run?.status === 'completed' ? 100 : steps.length > 0 ? Math.round((finished / steps.length) * 100) : 0;

  return {
    id: `workflow-${run.id}`,
    name: `${run.name || '工作流'} ${run.shortId || ''}`.trim(),
    type: 'workflow_run',
    status,
    progress,
    error: run?.status === 'failed' ? run?.lastMessage : undefined,
    createdAt: new Date(run?.createdAt || Date.now()).toISOString(),
    updatedAt: new Date(run?.updatedAt || Date.now()).toISOString(),
    logCount: run?.lastMessage ? 1 : 0,
    log: run?.lastMessage ? [run.lastMessage] : [],
  };
}

function mergeTasks(base: TaskInfo[], extra: TaskInfo[]) {
  const merged = new Map<string, TaskInfo>();
  [...extra, ...base].forEach(task => {
    merged.set(task.id, task);
  });
  return Array.from(merged.values()).sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime());
}

function filterVisibleTasks(tasks: TaskInfo[]) {
  return tasks.filter(task => !(task.type === 'workflow_run' && task.status === 'canceled'));
}

function LayoutShell({ onLogout, napcatStatus, wechatStatus, openclawStatus, processStatus, wsMessages }: Props) {
  const { t, locale, setLocale } = useI18n();
  const navigate = useNavigate();
  const location = useLocation();
  const enableAgents = import.meta.env.VITE_FEATURE_AGENTS !== 'false';
  const reducedPerfMode = useMemo(() => {
    const path = location.pathname;
    return [
      '/agents',
      '/monitor',
      '/channels',
      '/skills',
      '/plugins',
      '/workflows',
      '/workspace',
      '/config',
      '/sessions',
      '/logs',
      '/cron',
    ].some(prefix => path === prefix || path.startsWith(`${prefix}/`));
  }, [location.pathname]);
  const [tasks, setTasks] = useState<TaskInfo[]>([]);
  const [taskLogs, setTaskLogs] = useState<Record<string, string[]>>({});
  const [searchQuery, setSearchQuery] = useState('');
  const [searchOpen, setSearchOpen] = useState(false);
  const [profileOpen, setProfileOpen] = useState(false);
  const searchRef = useRef<HTMLDivElement | null>(null);
  const profileRef = useRef<HTMLDivElement | null>(null);

  const loadTasks = useCallback(async () => {
    try {
      const [taskRes, workflowRes] = await Promise.all([
        api.getPanelTasks(),
        api.getWorkflowRuns(),
      ]);
      const taskItems = taskRes?.ok ? (taskRes.tasks || []) : [];
      const workflowItems = workflowRes?.ok ? (workflowRes.runs || []).map(mapWorkflowRunToTask) : [];
      setTasks(filterVisibleTasks(mergeTasks(taskItems, workflowItems)));
    } catch {}
  }, []);

  useEffect(() => { loadTasks(); }, [loadTasks]);

  useEffect(() => {
	if (!tasks.some(task => task.status === 'running' || task.status === 'pending')) return;
	const timer = setInterval(() => {
	  loadTasks();
	}, 2500);
	return () => clearInterval(timer);
  }, [tasks, loadTasks]);

  // Listen for WebSocket task events
  useEffect(() => {
    if (!wsMessages || wsMessages.length === 0) return;
    const last = wsMessages[wsMessages.length - 1];
    if (last?.type === 'task_update') {
      setTasks(prev => {
        if (last.task?.type === 'workflow_run' && last.task?.status === 'canceled') {
          return prev.filter(t => t.id !== last.task.id);
        }
        const idx = prev.findIndex(t => t.id === last.task.id);
        if (idx >= 0) { const n = [...prev]; n[idx] = { ...n[idx], ...last.task }; return n; }
        return [last.task, ...prev];
      });
    } else if (last?.type === 'task_log') {
      setTaskLogs(prev => ({
        ...prev,
        [last.taskId]: [...(prev[last.taskId] || []), last.line],
      }));
    }
  }, [wsMessages]);

  const navItems = useMemo(() => [
    { to: '/', icon: LayoutDashboard, label: t.nav.dashboard },
    { to: '/chat', icon: MessageSquare, label: t.nav.panelChat },
    { to: '/logs', icon: ScrollText, label: t.nav.activityLog },
    { to: '/channels', icon: Radio, label: t.nav.channels },
    { to: '/skills', icon: Sparkles, label: t.nav.skills },
    { to: '/plugins', icon: Puzzle, label: locale === 'zh-CN' ? '插件中心' : 'Plugins' },
    ...(enableAgents ? [{ to: '/agents', icon: Bot, label: locale === 'zh-CN' ? '智能体' : 'Agents' }] : []),
    ...(enableAgents ? [{ to: '/monitor', icon: Network, label: locale === 'zh-CN' ? '编排监控' : 'Monitor' }] : []),
    { to: '/workflows', icon: GitBranch, label: locale === 'zh-CN' ? '工作流' : 'Workflows' },
    { to: '/cron', icon: Clock, label: t.nav.cronJobs },
    { to: '/tasks', icon: Activity, label: locale === 'zh-CN' ? '后台任务' : 'Tasks' },
    { to: '/sessions', icon: MessageSquare, label: '会话管理' },
    { to: '/workspace', icon: FolderOpen, label: t.nav.workspace },
    { to: '/config', icon: Settings, label: t.nav.systemConfig },
  ], [enableAgents, locale, t]);

  const mobileNavItems = useMemo(() => [
    { to: '/', icon: LayoutDashboard, label: t.nav.dashboard },
    { to: '/chat', icon: MessageSquare, label: t.nav.panelChat },
    { to: '/channels', icon: Radio, label: t.nav.channels },
    ...(enableAgents ? [{ to: '/agents', icon: Bot, label: locale === 'zh-CN' ? '智能体' : 'Agents' }] : [{ to: '/plugins', icon: Puzzle, label: locale === 'zh-CN' ? '插件' : 'Plugins' }]),
    { to: '/workflows', icon: GitBranch, label: locale === 'zh-CN' ? '工作流' : 'Flows' },
    { to: '/config', icon: Settings, label: t.nav.systemConfig },
  ], [enableAgents, locale, t]);

  const [dark, setDark] = useState(() => {
    const s = localStorage.getItem('theme');
    if (s === 'dark' || (!s && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
      document.documentElement.classList.add('dark');
      return true;
    }
    return false;
  });
  const [open, setOpen] = useState(false);
  const outletContext = useMemo(() => ({ uiMode: 'modern' as const }), []);

  useEffect(() => {
    document.body.dataset.uiMode = 'modern';
    return () => {
      delete document.body.dataset.uiMode;
    };
  }, []);

  useEffect(() => {
    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target as Node;
      if (searchRef.current && !searchRef.current.contains(target)) {
        setSearchOpen(false);
      }
      if (profileRef.current && !profileRef.current.contains(target)) {
        setProfileOpen(false);
      }
    };

    document.addEventListener('mousedown', handlePointerDown);
    return () => document.removeEventListener('mousedown', handlePointerDown);
  }, []);

  const toggleDark = () => {
    setDark(d => {
      const n = !d;
      localStorage.setItem('theme', n ? 'dark' : 'light');
      document.documentElement.classList.toggle('dark', n);
      return n;
    });
  };

  const toggleLocale = () => {
    setLocale(locale === 'zh-CN' ? 'en' : 'zh-CN');
  };

  const commandItems = useMemo(() => [
    { label: '仪表盘', keywords: ['dashboard', 'home', '首页', '仪表盘'], path: '/' },
    { label: locale === 'zh-CN' ? '面板聊天' : 'Panel Chat', keywords: ['chat', 'panel chat', '对话', '聊天', '本地聊天'], path: '/chat' },
    { label: '活动日志', keywords: ['log', 'logs', '日志', '活动日志'], path: '/logs' },
    { label: '通道配置 - QQ个人号', keywords: ['qq', 'napcat', 'qq个人号', 'qq personal'], path: '/channels?channel=qq' },
    { label: '通道配置 - 飞书', keywords: ['feishu', 'lark', '飞书'], path: '/channels?channel=feishu' },
    { label: '通道配置 - QQ官方机器人', keywords: ['qqbot', 'qq官方', 'qq 官方机器人'], path: '/channels?channel=qqbot' },
    { label: '通道配置 - Matrix', keywords: ['matrix'], path: '/channels?channel=matrix' },
    { label: '通道配置 - Mattermost', keywords: ['mattermost'], path: '/channels?channel=mattermost' },
    { label: '通道配置 - LINE', keywords: ['line'], path: '/channels?channel=line' },
    { label: '通道配置 - Microsoft Teams', keywords: ['msteams', 'teams'], path: '/channels?channel=msteams' },
    { label: '通道配置 - Twitch', keywords: ['twitch'], path: '/channels?channel=twitch' },
    { label: '通道配置 - WhatsApp', keywords: ['whatsapp'], path: '/channels?channel=whatsapp' },
    { label: '技能中心', keywords: ['skills', 'skill', '技能'], path: '/skills' },
    { label: '插件中心', keywords: ['plugins', 'plugin', '插件'], path: '/plugins' },
    ...(enableAgents ? [{ label: locale === 'zh-CN' ? '智能体' : 'Agents', keywords: ['agent', 'agents', '智能体'], path: '/agents' }] : []),
    ...(enableAgents ? [{ label: locale === 'zh-CN' ? '编排监控' : 'Monitor', keywords: ['monitor', 'topology', '监控', '编排', '拓扑'], path: '/monitor' }] : []),
    { label: locale === 'zh-CN' ? '工作流中心' : 'Workflow Center', keywords: ['workflow', 'workflows', '流程', '工作流'], path: '/workflows' },
    { label: '定时任务', keywords: ['cron', 'jobs', '定时任务'], path: '/cron' },
    { label: locale === 'zh-CN' ? '后台任务' : 'Background Tasks', keywords: ['tasks', 'background tasks', '任务账本', '后台任务'], path: '/tasks' },
    { label: '会话管理', keywords: ['session', 'sessions', '会话'], path: '/sessions' },
    { label: '工作区', keywords: ['workspace', '工作区', '文件'], path: '/workspace' },
    { label: '系统配置', keywords: ['config', 'settings', '系统配置'], path: '/config' },
  ], [enableAgents, locale]);

  const searchResults = useMemo(() => searchQuery.trim()
    ? commandItems.filter(item => {
        const q = searchQuery.toLowerCase();
        return item.label.toLowerCase().includes(q) || item.keywords.some(k => k.toLowerCase().includes(q));
      }).slice(0, 8)
    : commandItems.slice(0, 6), [commandItems, searchQuery]);

  const handleSearchGo = (path: string) => {
    navigate(path);
    setSearchQuery('');
    setSearchOpen(false);
    setOpen(false);
    setProfileOpen(false);
  };

  // Build channel list from enabledChannels returned by /api/status
  const connectedChannels = useMemo(() => {
    const enabledChannels: { id: string; label: string }[] = (openclawStatus?.enabledChannels || [])
      .map((ch: { id: string; label: string }) => ({
        ...ch,
        id: ch.id === 'qqbot-community' ? 'qqbot' : ch.id,
      }))
      .filter((ch: { id: string }) => DISPLAY_CHANNEL_IDS.has(ch.id));
    const channels: { label: string; detail: string; connected: boolean }[] = [];

    for (const ch of enabledChannels) {
      if (ch.id === 'qq') {
        const connected = napcatStatus?.connected;
        channels.push({
          label: 'QQ',
          detail: connected ? `${napcatStatus.nickname || 'QQ'} (${napcatStatus.selfId || ''})` : t.common.notLoggedIn,
          connected: !!connected,
        });
      } else if (ch.id === 'wechat') {
        channels.push({
          label: locale === 'zh-CN' ? '微信' : 'WeChat',
          detail: wechatStatus?.loggedIn ? (wechatStatus.name || t.common.connected) : t.common.notLoggedIn,
          connected: !!wechatStatus?.loggedIn,
        });
      } else if (ch.id === 'wecom') {
        channels.push({
          label: locale === 'zh-CN' ? '企业微信（机器人）' : 'WeCom Bot',
          detail: t.common.enabled,
          connected: true,
        });
      } else if (ch.id === 'wecom-app') {
        channels.push({
          label: locale === 'zh-CN' ? '企业微信（自建应用）' : 'WeCom App',
          detail: t.common.enabled,
          connected: true,
        });
      } else {
        channels.push({ label: ch.label, detail: t.common.enabled, connected: true });
      }
    }

    // Sort: connected channels first, then alphabetical; limit to 5
    channels.sort((a, b) => (a.connected === b.connected ? a.label.localeCompare(b.label) : a.connected ? -1 : 1));
    return channels.slice(0, 5);
  }, [locale, napcatStatus, openclawStatus, t, wechatStatus]);
  const totalEnabledChannels = (openclawStatus?.enabledChannels || []).length;
  const runtime = useMemo(() => resolveOpenClawRuntime(openclawStatus, processStatus), [openclawStatus, processStatus]);
  const openClawRestartHint = processStatus?.managedExternally
    ? (locale === 'zh-CN' ? '当前 OpenClaw 由外部进程管理，请改用“网关”按钮或在外部环境中重启。' : 'OpenClaw is managed externally. Use “Gateway” or restart it outside the panel.')
    : processStatus?.daemonized
      ? (locale === 'zh-CN' ? '当前 OpenClaw 以 daemon 模式运行，请改用“网关”按钮重启。' : 'OpenClaw is running in daemon mode. Use “Gateway” to restart it.')
      : '';
  const openClawRestartDisabled = !!openClawRestartHint;

  return (
    <div className="flex h-screen overflow-hidden ui-modern-shell" data-ui-perf={reducedPerfMode ? 'reduced' : undefined}>
      {open && <div className="fixed inset-0 z-40 bg-slate-950/42 backdrop-blur-sm lg:hidden" onClick={() => setOpen(false)} />}
      <aside className={`fixed inset-y-0 left-0 z-50 flex w-[88vw] max-w-[320px] flex-col ui-modern-sidebar transition-transform duration-300 lg:static lg:w-64 lg:max-w-none lg:translate-x-0 ${open ? 'translate-x-0' : '-translate-x-full'}`}>
        {/* Brand */}
        <div className="px-4 py-4 border-b border-slate-200/70">
          <div className="flex items-center gap-3">
            <div className="flex items-center justify-center w-10 h-10 rounded-2xl border border-slate-200/70 bg-white/90 shadow-[0_10px_24px_rgba(15,23,42,0.06)] dark:border-slate-700/70 dark:bg-slate-900/90">
              <img src="/logo.jpg" alt="ClawPanel" className="w-8 h-8 rounded-xl object-cover" />
            </div>
            <div>
              <h1 className="font-bold text-sm tracking-tight text-gray-900 dark:text-white">ClawPanel</h1>
              <p className="text-[10px] font-medium -mt-0.5 text-slate-500">{t.nav.subtitle}</p>
            </div>
          </div>
        </div>

        {/* Connected channel indicators — only show if any connected */}
        {connectedChannels.length > 0 && (
          <div className="px-4 py-3 border-b space-y-1.5 border-slate-200/70">
            <div className="text-[10px] font-semibold uppercase tracking-wider mb-1.5 text-slate-400">{t.nav.runningStatus}</div>
            {connectedChannels.map(ch => (
              <div key={ch.label} className="flex items-center gap-2 text-xs">
                <span className={`relative flex h-2 w-2 shrink-0`}>
                  <span className={`animate-ping absolute inline-flex h-full w-full rounded-full opacity-75 ${ch.connected ? 'bg-emerald-400' : 'bg-amber-400'}`}></span>
                  <span className={`relative inline-flex rounded-full h-2 w-2 ${ch.connected ? 'bg-emerald-500' : 'bg-amber-500'}`}></span>
                </span>
                <div className="min-w-0 flex-1">
                  <span className="text-gray-600 dark:text-gray-300 font-medium block truncate">{ch.label}</span>
                  <span className="text-[10px] text-gray-400 block truncate">{ch.detail}</span>
                </div>
              </div>
            ))}
            {totalEnabledChannels > connectedChannels.length && (
              <div className="text-[10px] text-slate-400 pl-4">+{totalEnabledChannels - connectedChannels.length} {locale === 'zh-CN' ? '个通道' : 'more'}</div>
            )}
          </div>
        )}

        {/* Navigation */}
        <nav className="flex-1 space-y-1 overflow-y-auto p-3 ui-modern-scrollbar">
          {navItems.map(({ to, icon: Icon, label }) => (
            <NavLink key={to} to={to} end={to === '/'} onClick={() => setOpen(false)}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3.5 py-2.5 rounded-xl text-[13px] transition-all duration-200 group border ${isActive ? 'ui-modern-nav-link active font-semibold translate-x-0.5' : 'ui-modern-nav-link border-transparent hover:-translate-y-0.5 hover:translate-x-0.5 hover:border-blue-100/80 hover:text-slate-900'}`
              }>
              <Icon size={18} className="shrink-0 transition-transform duration-200 group-hover:scale-105 group-hover:-rotate-3" />
              <span className="transition-all duration-200">{label}</span>
            </NavLink>
          ))}
        </nav>

        {/* Footer */}
        <div className="space-y-0.5 border-t border-slate-200/70 p-2 pb-[max(0.5rem,env(safe-area-inset-bottom))] lg:pb-2">
          <button onClick={toggleLocale} className="flex items-center gap-2.5 px-3 py-2 rounded-lg text-[13px] text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-800 w-full">
            <Languages size={16} />{locale === 'zh-CN' ? 'English' : '中文（简体）'}
          </button>
          {/* Quick actions */}
          <div className="flex items-center gap-1 px-1 py-1">
            <button
              onClick={async () => {
                if (openClawRestartDisabled) {
                  window.alert(openClawRestartHint);
                  return;
                }
                if (!confirm(locale === 'zh-CN' ? '确定重启 OpenClaw？' : 'Restart OpenClaw?')) return;
                try {
                  const r = await api.restartProcess();
                  if (!r?.ok) window.alert(r?.error || (locale === 'zh-CN' ? '重启 OpenClaw 失败' : 'Failed to restart OpenClaw'));
                } catch {
                  window.alert(locale === 'zh-CN' ? '重启 OpenClaw 失败' : 'Failed to restart OpenClaw');
                }
              }}
              aria-disabled={openClawRestartDisabled}
              className={`flex-1 flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-lg text-[11px] font-medium transition-colors ${
                openClawRestartDisabled
                  ? 'text-gray-400 dark:text-gray-500 bg-gray-50 dark:bg-gray-800/60'
                  : 'text-amber-600 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-950/30'
              }`}
              title={openClawRestartHint || (locale === 'zh-CN' ? '重启 OpenClaw' : 'Restart OpenClaw')}
            >
              <RotateCw size={13} /><span>OpenClaw</span>
            </button>
            <button
              onClick={async () => {
                try {
                  const r = await api.restartGateway();
                  if (!r?.ok) window.alert(r?.error || (locale === 'zh-CN' ? '重启网关失败' : 'Failed to restart gateway'));
                } catch {
                  window.alert(locale === 'zh-CN' ? '重启网关失败' : 'Failed to restart gateway');
                }
              }}
              className="flex-1 flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-lg text-[11px] font-medium text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-950/30 transition-colors"
              title={locale === 'zh-CN' ? '重启网关' : 'Restart Gateway'}
            >
              <RefreshCw size={13} /><span>{locale === 'zh-CN' ? '网关' : 'Gateway'}</span>
            </button>
            <button
              onClick={async () => { if (!confirm(locale === 'zh-CN' ? '确定重启 ClawPanel？页面将短暂断开。' : 'Restart ClawPanel? Page will briefly disconnect.')) return; try { await api.restartPanel(); setTimeout(() => window.location.reload(), 3000); } catch {} }}
              className="flex-1 flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-lg text-[11px] font-medium text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-950/30 transition-colors"
              title={locale === 'zh-CN' ? '重启面板' : 'Restart Panel'}
            >
              <Power size={13} /><span>{locale === 'zh-CN' ? '面板' : 'Panel'}</span>
            </button>
          </div>
          <button onClick={onLogout} className="flex items-center gap-2.5 px-3 py-2 rounded-lg text-[13px] text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-950/30 w-full">
            <LogOut size={16} />{t.nav.logout}
          </button>
        </div>
      </aside>
      <main className="flex-1 flex flex-col overflow-hidden bg-transparent">
          <header className="relative z-[160] hidden lg:flex items-center justify-between px-6 py-4 border-b border-slate-200/70 dark:border-slate-800/70 bg-[rgba(255,255,255,0.58)] dark:bg-[rgba(8,18,33,0.82)] backdrop-blur-xl">
            <div className="flex items-center gap-3 min-w-0">
              <button onClick={() => setOpen(true)} className="inline-flex items-center justify-center w-10 h-10 rounded-xl border border-slate-200 bg-white/90 text-slate-500 hover:text-slate-700 hover:bg-white transition-colors lg:hidden">
                <Menu size={18} />
              </button>
              <div ref={searchRef} className="relative z-[120] hidden xl:flex flex-col min-w-[360px] max-w-[520px]">
                <div className="flex items-center gap-3 px-4 py-3 rounded-2xl ui-modern-panel">
                <Search size={16} className="text-slate-400 dark:text-slate-500" />
                <input value={searchQuery} onChange={(e) => { setSearchQuery(e.target.value); setSearchOpen(true); }} onFocus={() => setSearchOpen(true)} onKeyDown={(e) => { if (e.key === 'Enter' && searchResults[0]) handleSearchGo(searchResults[0].path); if (e.key === 'Escape') setSearchOpen(false); }} placeholder={locale === 'zh-CN' ? '搜索页面、功能或通道...' : 'Search pages, features, or channels...'} className="w-full bg-transparent outline-none text-sm text-slate-700 dark:text-slate-100 placeholder:text-slate-500 dark:placeholder:text-slate-500" />
                </div>
                {searchOpen && (
                  <div className="absolute top-full left-0 right-0 mt-3 rounded-[24px] border border-blue-100/80 bg-[linear-gradient(145deg,rgba(255,255,255,0.98),rgba(239,246,255,0.92))] shadow-[0_24px_60px_rgba(15,23,42,0.14)] backdrop-blur-xl overflow-hidden z-[140] dark:border-blue-800/30 dark:bg-[linear-gradient(145deg,rgba(12,24,42,0.98),rgba(30,64,175,0.16))]">
                    {searchResults.length === 0 ? <div className="px-4 py-3 text-sm text-slate-500 dark:text-slate-300">未找到匹配页面</div> : searchResults.map(item => (
                      <button key={item.path + item.label} onClick={() => handleSearchGo(item.path)} className="w-full text-left px-4 py-3 hover:bg-blue-50/70 dark:hover:bg-blue-900/20 transition-colors border-b last:border-b-0 border-blue-100/60 dark:border-slate-700/70">
                        <div className="text-sm font-medium text-slate-800 dark:text-slate-100">{item.label}</div>
                        <div className="text-[11px] text-slate-500 dark:text-slate-400">{item.path}</div>
                      </button>
                    ))}
                  </div>
                )}
              </div>
            </div>
            <div className="flex items-center gap-3">
              <button onClick={toggleDark} className="w-10 h-10 rounded-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 text-slate-500 dark:text-slate-300 hover:text-slate-700 dark:hover:text-white transition-colors inline-flex items-center justify-center">
                {dark ? <Sun size={17} /> : <Moon size={17} />}
              </button>
              <MessageCenter tasks={tasks} taskLogs={taskLogs} onRefresh={loadTasks} mode="icon" />
              <div ref={profileRef} className="relative">
                <button onClick={() => setProfileOpen(v => !v)} className="flex items-center gap-3 rounded-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 pl-2 pr-3 py-1.5 hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors">
                  <img src="/logo.jpg" alt="avatar" className="w-8 h-8 rounded-full object-cover" />
                    <div className="text-right leading-tight">
                      <div className="text-xs font-semibold text-slate-800 dark:text-slate-100">Admin</div>
                      <div className="text-[10px] text-slate-400 dark:text-slate-500">ClawPanel</div>
                    </div>
                  <ChevronDown size={14} className="text-slate-400" />
                </button>
                {profileOpen && (
                  <div className="absolute top-full right-0 mt-2 w-44 rounded-2xl ui-modern-card p-2 z-50">
                    <button onClick={() => { setProfileOpen(false); onLogout(); }} className="w-full flex items-center gap-2 px-3 py-2 rounded-xl text-sm text-red-600 hover:bg-red-50 transition-colors">
                      <LogOut size={15} /> 退出登录
                    </button>
                  </div>
                )}
              </div>
            </div>
          </header>
        <header className="lg:hidden shrink-0 border-b border-blue-100/70 bg-[linear-gradient(180deg,rgba(255,255,255,0.86),rgba(239,246,255,0.74))] px-3 pt-[max(0.75rem,env(safe-area-inset-top))] pb-3 backdrop-blur-2xl dark:border-blue-400/15 dark:bg-[linear-gradient(180deg,rgba(7,17,31,0.94),rgba(11,26,46,0.88))]">
          <div className="flex items-center gap-2.5">
            <button onClick={() => setOpen(true)} className="page-modern-action h-11 w-11 rounded-2xl p-0">
              <Menu size={19} />
            </button>
            <div className="flex min-w-0 flex-1 items-center gap-2.5 rounded-2xl border border-blue-100/70 bg-white/60 px-3 py-2.5 shadow-[0_14px_30px_rgba(15,23,42,0.05)] backdrop-blur-xl dark:border-blue-400/15 dark:bg-slate-900/50">
              <img src="/logo.jpg" alt="ClawPanel" className="h-9 w-9 rounded-xl object-cover shadow-sm" />
              <div className="min-w-0">
                <div className="truncate text-sm font-bold tracking-tight text-slate-900 dark:text-white">ClawPanel</div>
                <div className="truncate text-[11px] text-slate-500 dark:text-slate-400">{locale === 'zh-CN' ? '移动端控制台' : 'Mobile Console'}</div>
              </div>
            </div>
            <button onClick={toggleDark} className="page-modern-action h-11 w-11 rounded-2xl p-0">
              {dark ? <Sun size={17} /> : <Moon size={17} />}
            </button>
            <MessageCenter tasks={tasks} taskLogs={taskLogs} onRefresh={loadTasks} mode="icon" />
          </div>
          <div ref={searchRef} className="relative mt-3">
            <div className="flex items-center gap-3 rounded-2xl border border-blue-100/70 bg-white/64 px-4 py-3 shadow-[0_14px_32px_rgba(15,23,42,0.05)] backdrop-blur-xl dark:border-blue-400/15 dark:bg-slate-900/48">
              <Search size={16} className="text-slate-400 dark:text-slate-500" />
              <input value={searchQuery} onChange={(e) => { setSearchQuery(e.target.value); setSearchOpen(true); }} onFocus={() => setSearchOpen(true)} onKeyDown={(e) => { if (e.key === 'Enter' && searchResults[0]) handleSearchGo(searchResults[0].path); if (e.key === 'Escape') setSearchOpen(false); }} placeholder={locale === 'zh-CN' ? '搜索页面、功能或通道...' : 'Search pages, features, or channels...'} className="w-full bg-transparent text-sm text-slate-700 outline-none placeholder:text-slate-500 dark:text-slate-100 dark:placeholder:text-slate-500" />
            </div>
            {searchOpen && (
              <div className="absolute left-0 right-0 top-full z-[140] mt-3 overflow-hidden rounded-[24px] border border-blue-100/80 bg-[linear-gradient(145deg,rgba(255,255,255,0.98),rgba(239,246,255,0.92))] shadow-[0_24px_60px_rgba(15,23,42,0.14)] backdrop-blur-xl dark:border-blue-800/30 dark:bg-[linear-gradient(145deg,rgba(12,24,42,0.98),rgba(30,64,175,0.16))]">
                {searchResults.length === 0 ? <div className="px-4 py-3 text-sm text-slate-500 dark:text-slate-300">未找到匹配页面</div> : searchResults.map(item => (
                  <button key={item.path + item.label} onClick={() => handleSearchGo(item.path)} className="w-full border-b border-blue-100/60 px-4 py-3 text-left transition-colors last:border-b-0 hover:bg-blue-50/70 dark:border-slate-700/70 dark:hover:bg-blue-900/20">
                    <div className="text-sm font-medium text-slate-800 dark:text-slate-100">{item.label}</div>
                    <div className="text-[11px] text-slate-500 dark:text-slate-400">{item.path}</div>
                  </button>
                ))}
              </div>
            )}
          </div>
        </header>
        {openclawStatus?.configured && !runtime.healthy && (
          <div className="px-3 pt-3 sm:px-4 lg:px-6 xl:px-7">
            <div className={`rounded-[24px] border px-4 py-3 shadow-[0_16px_34px_rgba(15,23,42,0.06)] backdrop-blur-xl ${runtime.state === 'offline' ? 'border-red-200/80 dark:border-red-900/40 bg-[linear-gradient(135deg,rgba(254,242,242,0.96),rgba(255,237,213,0.88))] dark:bg-[linear-gradient(135deg,rgba(127,29,29,0.24),rgba(120,53,15,0.18))]' : 'border-amber-200/80 dark:border-amber-900/40 bg-[linear-gradient(135deg,rgba(255,251,235,0.96),rgba(254,249,195,0.86))] dark:bg-[linear-gradient(135deg,rgba(120,53,15,0.22),rgba(113,63,18,0.16))]'}`}>
              <div className="flex items-start gap-3">
                <div className={`mt-0.5 rounded-2xl p-2 ${runtime.state === 'offline' ? 'bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-300' : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'}`}>
                  <Bell size={16} />
                </div>
                <div className="min-w-0 flex-1">
                  <div className={`text-sm font-semibold ${runtime.state === 'offline' ? 'text-red-900 dark:text-red-100' : 'text-amber-900 dark:text-amber-100'}`}>{runtime.title}</div>
                  <div className={`mt-1 text-xs leading-5 ${runtime.state === 'offline' ? 'text-red-700 dark:text-red-200/90' : 'text-amber-800 dark:text-amber-200/90'}`}>{runtime.message}</div>
                </div>
              </div>
            </div>
          </div>
        )}
        <div className="ui-modern-content flex-1 overflow-y-auto ui-modern-scrollbar p-3 pb-24 sm:p-4 sm:pb-28 lg:p-6 lg:pb-6 xl:p-7"><Outlet context={outletContext} /></div>
      </main>
      <nav className="fixed inset-x-0 bottom-0 z-40 border-t border-blue-100/70 bg-[linear-gradient(180deg,rgba(255,255,255,0.9),rgba(239,246,255,0.84))] px-3 pb-[max(0.6rem,env(safe-area-inset-bottom))] pt-2 backdrop-blur-2xl dark:border-blue-400/15 dark:bg-[linear-gradient(180deg,rgba(7,17,31,0.96),rgba(11,26,46,0.92))] lg:hidden">
        <div className="grid grid-cols-5 gap-2">
          {mobileNavItems.map(({ to, icon: Icon, label }) => {
            const active = location.pathname === to || (to !== '/' && location.pathname.startsWith(to));
            return (
              <button key={to} onClick={() => handleSearchGo(to)} className={`flex min-h-[62px] flex-col items-center justify-center gap-1 rounded-2xl border text-[11px] font-medium transition-all ${active ? 'border-blue-200/80 bg-[linear-gradient(135deg,rgba(59,130,246,0.18),rgba(14,165,233,0.12))] text-blue-700 shadow-[0_12px_24px_rgba(37,99,235,0.12)] dark:border-blue-400/20 dark:bg-[linear-gradient(135deg,rgba(37,99,235,0.24),rgba(14,165,233,0.12))] dark:text-blue-100' : 'border-transparent bg-white/40 text-slate-500 dark:bg-slate-900/28 dark:text-slate-400'}`}>
                <Icon size={17} />
                <span className="truncate px-1">{label}</span>
              </button>
            );
          })}
          <button onClick={() => setOpen(true)} className="flex min-h-[62px] flex-col items-center justify-center gap-1 rounded-2xl bg-white/40 text-[11px] font-medium text-slate-500 dark:bg-slate-900/28 dark:text-slate-400">
            <Menu size={17} />
            <span>{locale === 'zh-CN' ? '更多' : 'More'}</span>
          </button>
        </div>
      </nav>
      <AIAssistant />
    </div>
  );
}

export default memo(LayoutShell);

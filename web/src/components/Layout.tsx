import { useState, useEffect, useCallback, useRef } from 'react';
import { Outlet, NavLink, useNavigate } from 'react-router-dom';
import {
  LayoutDashboard, ScrollText, Radio, Sparkles, Clock, Settings,
  Moon, Sun, LogOut, Menu, FolderOpen, Languages, MessageSquare,
  RotateCw, RefreshCw, Power, Puzzle, Bot, Search, Bell, ChevronDown,
} from 'lucide-react';
import { useI18n } from '../i18n';
import AIAssistant from './AIAssistant';
import MessageCenter, { TaskInfo } from './MessageCenter';
import { api } from '../lib/api';

interface Props { onLogout: () => void; napcatStatus: any; wechatStatus?: any; openclawStatus?: any; processStatus?: any; wsMessages?: any[]; }

export default function Layout({ onLogout, napcatStatus, wechatStatus, openclawStatus, processStatus, wsMessages }: Props) {
  const { t, locale, setLocale } = useI18n();
  const navigate = useNavigate();
  const enableAgents = import.meta.env.VITE_FEATURE_AGENTS !== 'false';
  const [tasks, setTasks] = useState<TaskInfo[]>([]);
  const [taskLogs, setTaskLogs] = useState<Record<string, string[]>>({});
  const [searchQuery, setSearchQuery] = useState('');
  const [searchOpen, setSearchOpen] = useState(false);
  const [profileOpen, setProfileOpen] = useState(false);
  const searchRef = useRef<HTMLDivElement | null>(null);
  const profileRef = useRef<HTMLDivElement | null>(null);

  const loadTasks = useCallback(async () => {
    try { const r = await api.getTasks(); if (r.ok) setTasks(r.tasks || []); } catch {}
  }, []);

  useEffect(() => { loadTasks(); }, [loadTasks]);

  // Listen for WebSocket task events
  useEffect(() => {
    if (!wsMessages || wsMessages.length === 0) return;
    const last = wsMessages[wsMessages.length - 1];
    if (last?.type === 'task_update') {
      setTasks(prev => {
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

  const navItems = [
    { to: '/', icon: LayoutDashboard, label: t.nav.dashboard },
    { to: '/logs', icon: ScrollText, label: t.nav.activityLog },
    { to: '/channels', icon: Radio, label: t.nav.channels },
    { to: '/skills', icon: Sparkles, label: t.nav.skills },
    { to: '/plugins', icon: Puzzle, label: locale === 'zh-CN' ? '插件中心' : 'Plugins' },
    ...(enableAgents ? [{ to: '/agents', icon: Bot, label: locale === 'zh-CN' ? '智能体' : 'Agents' }] : []),
    { to: '/cron', icon: Clock, label: t.nav.cronJobs },
    { to: '/sessions', icon: MessageSquare, label: '会话管理' },
    { to: '/workspace', icon: FolderOpen, label: t.nav.workspace },
    { to: '/config', icon: Settings, label: t.nav.systemConfig },
  ];

  const [dark, setDark] = useState(() => {
    const s = localStorage.getItem('theme');
    if (s === 'dark' || (!s && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
      document.documentElement.classList.add('dark');
      return true;
    }
    return false;
  });
  const [open, setOpen] = useState(false);

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

  const commandItems = [
    { label: '仪表盘', keywords: ['dashboard', 'home', '首页', '仪表盘'], path: '/' },
    { label: '活动日志', keywords: ['log', 'logs', '日志', '活动日志'], path: '/logs' },
    { label: '通道配置 - QQ个人号', keywords: ['qq', 'napcat', 'qq个人号', 'qq personal'], path: '/channels?channel=qq' },
    { label: '通道配置 - 飞书', keywords: ['feishu', 'lark', '飞书'], path: '/channels?channel=feishu' },
    { label: '通道配置 - QQ官方机器人', keywords: ['qqbot', 'qq官方', 'qq 官方机器人'], path: '/channels?channel=qqbot' },
    { label: '技能中心', keywords: ['skills', 'skill', '技能'], path: '/skills' },
    { label: '插件中心', keywords: ['plugins', 'plugin', '插件'], path: '/plugins' },
    ...(enableAgents ? [{ label: locale === 'zh-CN' ? '智能体' : 'Agents', keywords: ['agent', 'agents', '智能体'], path: '/agents' }] : []),
    { label: '定时任务', keywords: ['cron', 'jobs', '定时任务'], path: '/cron' },
    { label: '会话管理', keywords: ['session', 'sessions', '会话'], path: '/sessions' },
    { label: '工作区', keywords: ['workspace', '工作区', '文件'], path: '/workspace' },
    { label: '系统配置', keywords: ['config', 'settings', '系统配置'], path: '/config' },
  ];

  const searchResults = searchQuery.trim()
    ? commandItems.filter(item => {
        const q = searchQuery.toLowerCase();
        return item.label.toLowerCase().includes(q) || item.keywords.some(k => k.toLowerCase().includes(q));
      }).slice(0, 8)
    : commandItems.slice(0, 6);

  const handleSearchGo = (path: string) => {
    navigate(path);
    setSearchQuery('');
    setSearchOpen(false);
    setOpen(false);
    setProfileOpen(false);
  };

  // Build channel list from enabledChannels returned by /api/status
  const enabledChannels: { id: string; label: string }[] = openclawStatus?.enabledChannels || [];
  const connectedChannels: { label: string; detail: string; connected: boolean }[] = [];
  const restartViaGateway = !!(processStatus?.managedExternally || processStatus?.daemonized || (openclawStatus?.configured && !processStatus?.running));
  const openClawRestartHint = processStatus?.managedExternally
    ? (locale === 'zh-CN' ? '当前 OpenClaw 由外部进程管理，点击将自动改为重启网关。' : 'OpenClaw is managed externally. Click to restart gateway instead.')
    : processStatus?.daemonized
      ? (locale === 'zh-CN' ? '当前 OpenClaw 以 daemon 模式运行，点击将自动改为重启网关。' : 'OpenClaw is running in daemon mode. Click to restart gateway instead.')
      : '';
  for (const ch of enabledChannels) {
    if (ch.id === 'qq') {
      const connected = napcatStatus?.connected;
      connectedChannels.push({
        label: 'QQ',
        detail: connected ? `${napcatStatus.nickname || 'QQ'} (${napcatStatus.selfId || ''})` : t.common.notLoggedIn,
        connected: !!connected,
      });
    } else if (ch.id === 'wechat') {
      connectedChannels.push({
        label: locale === 'zh-CN' ? '微信' : 'WeChat',
        detail: wechatStatus?.loggedIn ? (wechatStatus.name || t.common.connected) : t.common.notLoggedIn,
        connected: !!wechatStatus?.loggedIn,
      });
    } else {
      connectedChannels.push({ label: ch.label, detail: t.common.enabled, connected: true });
    }
  }

  return (
    <div className="flex h-screen overflow-hidden ui-modern-shell">
      {open && <div className="fixed inset-0 bg-black/50 z-40 lg:hidden" onClick={() => setOpen(false)} />}
      <aside className={`fixed lg:static inset-y-0 left-0 z-50 w-64 ui-modern-sidebar flex flex-col transition-transform lg:translate-x-0 ${open ? 'translate-x-0' : '-translate-x-full'}`}>
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
          </div>
        )}

        {/* Navigation */}
        <nav className="flex-1 p-3 space-y-1 overflow-y-auto ui-modern-scrollbar">
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
        <div className="p-2 border-t space-y-0.5 border-slate-200/70">
          <button onClick={toggleLocale} className="flex items-center gap-2.5 px-3 py-2 rounded-lg text-[13px] text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-800 w-full">
            <Languages size={16} />{locale === 'zh-CN' ? 'English' : '中文（简体）'}
          </button>
          {/* Quick actions */}
          <div className="flex items-center gap-1 px-1 py-1">
            <button
              onClick={async () => {
                if (!confirm(locale === 'zh-CN' ? '确定重启 OpenClaw？' : 'Restart OpenClaw?')) return;
                try {
                  if (restartViaGateway) {
                    const r = await api.restartGateway();
                    if (!r?.ok) {
                      window.alert(r?.error || (locale === 'zh-CN' ? '重启网关失败' : 'Failed to restart gateway'));
                      return;
                    }
                    window.alert(r?.message || (locale === 'zh-CN' ? '已改为重启网关' : 'Restarted via gateway'));
                    return;
                  }

                  const r = await api.restartProcess();
                  if (!r?.ok) window.alert(r?.error || (locale === 'zh-CN' ? '重启 OpenClaw 失败' : 'Failed to restart OpenClaw'));
                } catch {
                  window.alert(restartViaGateway
                    ? (locale === 'zh-CN' ? '重启网关失败' : 'Failed to restart gateway')
                    : (locale === 'zh-CN' ? '重启 OpenClaw 失败' : 'Failed to restart OpenClaw'));
                }
              }}
              className="flex-1 flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-lg text-[11px] font-medium text-amber-600 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-950/30 transition-colors"
              title={openClawRestartHint || (locale === 'zh-CN' ? '重启 OpenClaw' : 'Restart OpenClaw')}
            >
              <RotateCw size={13} /><span>OpenClaw</span>
            </button>
            <button
              onClick={async () => {
                try {
                  const r = await api.restartGateway();
                  if (!r?.ok) {
                    window.alert(r?.error || (locale === 'zh-CN' ? '重启网关失败' : 'Failed to restart gateway'));
                    return;
                  }
                  window.alert(r?.message || (locale === 'zh-CN' ? '网关重启请求已发送' : 'Gateway restart requested'));
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
        <header className="lg:hidden flex items-center gap-3 p-3 border-b border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
          <button onClick={() => setOpen(true)}><Menu size={20} /></button>
          <div className="flex items-center gap-2">
            <img src="/logo.jpg" alt="ClawPanel" className="w-7 h-7 rounded-lg shadow-sm object-cover" />
            <span className="font-bold text-sm text-gray-900 dark:text-white">ClawPanel</span>
          </div>
        </header>
        <div className="flex-1 overflow-y-auto ui-modern-scrollbar p-4 lg:p-6 xl:p-7"><Outlet context={{ uiMode: 'modern' }} /></div>
      </main>
      <AIAssistant />
    </div>
  );
}

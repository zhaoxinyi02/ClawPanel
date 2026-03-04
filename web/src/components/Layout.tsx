import { useState, useEffect, useCallback } from 'react';
import { Outlet, NavLink } from 'react-router-dom';
import {
  LayoutDashboard, ScrollText, Radio, Sparkles, Clock, Settings,
  Moon, Sun, LogOut, Menu, FolderOpen, Languages, MessageSquare,
  RotateCw, RefreshCw, Power, Puzzle, Bot,
} from 'lucide-react';
import { useI18n } from '../i18n';
import AIAssistant from './AIAssistant';
import MessageCenter, { TaskInfo } from './MessageCenter';
import { api } from '../lib/api';

interface Props { onLogout: () => void; napcatStatus: any; wechatStatus?: any; openclawStatus?: any; wsMessages?: any[]; }

export default function Layout({ onLogout, napcatStatus, wechatStatus, openclawStatus, wsMessages }: Props) {
  const { t, locale, setLocale } = useI18n();
  const enableAgents = import.meta.env.VITE_FEATURE_AGENTS !== 'false';
  const [tasks, setTasks] = useState<TaskInfo[]>([]);
  const [taskLogs, setTaskLogs] = useState<Record<string, string[]>>({});

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
    ...(enableAgents ? [{ to: '/agents', icon: Bot, label: locale === 'zh-CN' ? 'Agents' : 'Agents' }] : []),
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

  // Build channel list from enabledChannels returned by /api/status
  const enabledChannels: { id: string; label: string }[] = openclawStatus?.enabledChannels || [];
  const connectedChannels: { label: string; detail: string; connected: boolean }[] = [];
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
    <div className="flex h-screen overflow-hidden">
      {open && <div className="fixed inset-0 bg-black/50 z-40 lg:hidden" onClick={() => setOpen(false)} />}
      <aside className={`fixed lg:static inset-y-0 left-0 z-50 w-56 bg-white dark:bg-gray-900 border-r border-gray-200 dark:border-gray-800 flex flex-col transition-transform lg:translate-x-0 ${open ? 'translate-x-0' : '-translate-x-full'}`}>
        {/* Brand */}
        <div className="px-4 py-3.5 border-b border-gray-200 dark:border-gray-800">
          <div className="flex items-center gap-3">
            <img src="/logo.jpg" alt="ClawPanel" className="w-8 h-8 rounded-xl shadow-sm object-cover" />
            <div>
              <h1 className="font-bold text-sm tracking-tight text-gray-900 dark:text-white">ClawPanel</h1>
              <p className="text-[10px] text-gray-500 font-medium -mt-0.5">{t.nav.subtitle}</p>
            </div>
          </div>
        </div>

        {/* Connected channel indicators — only show if any connected */}
        {connectedChannels.length > 0 && (
          <div className="px-4 py-3 border-b border-gray-100 dark:border-gray-800/50 space-y-1.5">
            <div className="text-[10px] font-semibold text-gray-400 uppercase tracking-wider mb-1.5">{t.nav.runningStatus}</div>
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
        <nav className="flex-1 p-3 space-y-1 overflow-y-auto">
          {navItems.map(({ to, icon: Icon, label }) => (
            <NavLink key={to} to={to} end={to === '/'} onClick={() => setOpen(false)}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3.5 py-2.5 rounded-xl text-[13px] transition-all duration-200 group ${
                  isActive 
                  ? 'bg-violet-600 text-white shadow-md shadow-violet-200 dark:shadow-none font-medium' 
                  : 'text-gray-600 dark:text-gray-400 hover:bg-violet-50 dark:hover:bg-violet-900/20 hover:text-violet-600 dark:hover:text-violet-300'
                }`
              }>
              <Icon size={18} className="shrink-0" />
              <span>{label}</span>
            </NavLink>
          ))}
        </nav>

        {/* Footer */}
        <div className="p-2 border-t border-gray-200 dark:border-gray-800 space-y-0.5">
          <MessageCenter tasks={tasks} taskLogs={taskLogs} onRefresh={loadTasks} />
          <button onClick={toggleLocale} className="flex items-center gap-2.5 px-3 py-2 rounded-lg text-[13px] text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-800 w-full">
            <Languages size={16} />{locale === 'zh-CN' ? 'English' : '中文（简体）'}
          </button>
          <button onClick={toggleDark} className="flex items-center gap-2.5 px-3 py-2 rounded-lg text-[13px] text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-800 w-full">
            {dark ? <Sun size={16} /> : <Moon size={16} />}{dark ? t.nav.lightMode : t.nav.darkMode}
          </button>
          {/* Quick actions */}
          <div className="flex items-center gap-1 px-1 py-1">
            <button
              onClick={async () => { if (!confirm(locale === 'zh-CN' ? '确定重启 OpenClaw？' : 'Restart OpenClaw?')) return; try { await api.restartProcess(); } catch {} }}
              className="flex-1 flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-lg text-[11px] font-medium text-amber-600 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-950/30 transition-colors"
              title={locale === 'zh-CN' ? '重启 OpenClaw' : 'Restart OpenClaw'}
            >
              <RotateCw size={13} /><span>OpenClaw</span>
            </button>
            <button
              onClick={async () => { try { await api.restartGateway(); } catch {} }}
              className="flex-1 flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-lg text-[11px] font-medium text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-950/30 transition-colors"
              title={locale === 'zh-CN' ? '重启网关' : 'Restart Gateway'}
            >
              <RefreshCw size={13} /><span>{locale === 'zh-CN' ? '网关' : 'Gateway'}</span>
            </button>
            <button
              onClick={async () => { if (!confirm(locale === 'zh-CN' ? '确定重启 ClawPanel？页面将短暂断开。' : 'Restart ClawPanel? Page will briefly disconnect.')) return; try { await api.restartPanel(); setTimeout(() => window.location.reload(), 3000); } catch {} }}
              className="flex-1 flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-lg text-[11px] font-medium text-violet-600 dark:text-violet-400 hover:bg-violet-50 dark:hover:bg-violet-950/30 transition-colors"
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
      <main className="flex-1 flex flex-col overflow-hidden bg-gray-50/50 dark:bg-gray-950">
        <header className="lg:hidden flex items-center gap-3 p-3 border-b border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
          <button onClick={() => setOpen(true)}><Menu size={20} /></button>
          <div className="flex items-center gap-2">
            <img src="/logo.jpg" alt="ClawPanel" className="w-7 h-7 rounded-lg shadow-sm object-cover" />
            <span className="font-bold text-sm text-gray-900 dark:text-white">ClawPanel</span>
          </div>
        </header>
        <div className="flex-1 overflow-y-auto p-4 lg:p-6"><Outlet /></div>
      </main>
      <AIAssistant />
    </div>
  );
}

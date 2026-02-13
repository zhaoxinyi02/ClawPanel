import { useState } from 'react';
import { Outlet, NavLink } from 'react-router-dom';
import {
  LayoutDashboard, ScrollText, Radio, Sparkles, Clock, Settings,
  Moon, Sun, LogOut, Menu, FolderOpen, UserCheck, Cat,
} from 'lucide-react';

const navItems = [
  { to: '/', icon: LayoutDashboard, label: '仪表盘', group: 'main' },
  { to: '/logs', icon: ScrollText, label: '活动日志', group: 'main' },
  { to: '/channels', icon: Radio, label: '通道管理', group: 'main' },
  { to: '/skills', icon: Sparkles, label: '技能中心', group: 'main' },
  { to: '/cron', icon: Clock, label: '定时任务', group: 'main' },
  { to: '/config', icon: Settings, label: '系统配置', group: 'main' },
  { to: '/workspace', icon: FolderOpen, label: '工作区', group: 'extra' },
  { to: '/requests', icon: UserCheck, label: '审核', group: 'extra' },
];

interface Props { onLogout: () => void; napcatStatus: any; wechatStatus?: any; }

export default function Layout({ onLogout, napcatStatus, wechatStatus }: Props) {
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

  const mainNav = navItems.filter(n => n.group === 'main');
  const extraNav = navItems.filter(n => n.group === 'extra');

  return (
    <div className="flex h-screen overflow-hidden">
      {open && <div className="fixed inset-0 bg-black/50 z-40 lg:hidden" onClick={() => setOpen(false)} />}
      <aside className={`fixed lg:static inset-y-0 left-0 z-50 w-56 bg-white dark:bg-gray-900 border-r border-gray-200 dark:border-gray-800 flex flex-col transition-transform lg:translate-x-0 ${open ? 'translate-x-0' : '-translate-x-full'}`}>
        {/* Brand */}
        <div className="px-4 py-3.5 border-b border-gray-200 dark:border-gray-800">
          <div className="flex items-center gap-2">
            <div className="w-7 h-7 rounded-lg bg-gradient-to-br from-violet-500 to-indigo-600 flex items-center justify-center">
              <Cat size={15} className="text-white" />
            </div>
            <div>
              <h1 className="font-bold text-sm tracking-tight">ClawPanel</h1>
              <p className="text-[10px] text-gray-400 -mt-0.5">OpenClaw 管理面板</p>
            </div>
          </div>
        </div>

        {/* Channel status indicators */}
        <div className="px-3 py-2 border-b border-gray-100 dark:border-gray-800/50 space-y-1">
          <div className="flex items-center gap-2 text-[11px]">
            <span className={`w-1.5 h-1.5 rounded-full ${napcatStatus?.connected ? 'bg-emerald-500' : 'bg-red-500'}`} />
            <span className="text-gray-500 dark:text-gray-400 truncate">
              {napcatStatus?.connected
              ? `QQ: ${napcatStatus.nickname || 'QQ'}${napcatStatus.selfId ? ` (${napcatStatus.selfId})` : ''}`
              : 'QQ 未连接'}
            </span>
          </div>
          <div className="flex items-center gap-2 text-[11px]">
            <span className={`w-1.5 h-1.5 rounded-full ${wechatStatus?.connected ? 'bg-emerald-500' : 'bg-red-500'}`} />
            <span className="text-gray-500 dark:text-gray-400 truncate">
              {wechatStatus?.connected ? `微信: ${wechatStatus.name || '已连接'}` : '微信 未连接'}
            </span>
          </div>
        </div>

        {/* Main navigation */}
        <nav className="flex-1 p-2 space-y-0.5 overflow-y-auto">
          {mainNav.map(({ to, icon: Icon, label }) => (
            <NavLink key={to} to={to} end={to === '/'} onClick={() => setOpen(false)}
              className={({ isActive }) =>
                `flex items-center gap-2.5 px-3 py-2 rounded-lg text-[13px] transition-colors ${isActive ? 'bg-violet-50 dark:bg-violet-950/50 text-violet-700 dark:text-violet-300 font-medium' : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-800'}`
              }>
              <Icon size={16} />{label}
            </NavLink>
          ))}
          {extraNav.length > 0 && (
            <>
              <div className="pt-2 pb-1 px-3">
                <span className="text-[10px] font-medium text-gray-400 uppercase tracking-wider">其他</span>
              </div>
              {extraNav.map(({ to, icon: Icon, label }) => (
                <NavLink key={to} to={to} onClick={() => setOpen(false)}
                  className={({ isActive }) =>
                    `flex items-center gap-2.5 px-3 py-2 rounded-lg text-[13px] transition-colors ${isActive ? 'bg-violet-50 dark:bg-violet-950/50 text-violet-700 dark:text-violet-300 font-medium' : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-800'}`
                  }>
                  <Icon size={16} />{label}
                </NavLink>
              ))}
            </>
          )}
        </nav>

        {/* Footer */}
        <div className="p-2 border-t border-gray-200 dark:border-gray-800 space-y-0.5">
          <button onClick={toggleDark} className="flex items-center gap-2.5 px-3 py-2 rounded-lg text-[13px] text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-800 w-full">
            {dark ? <Sun size={16} /> : <Moon size={16} />}{dark ? '浅色模式' : '深色模式'}
          </button>
          <button onClick={onLogout} className="flex items-center gap-2.5 px-3 py-2 rounded-lg text-[13px] text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-950/30 w-full">
            <LogOut size={16} />退出登录
          </button>
        </div>
      </aside>
      <main className="flex-1 flex flex-col overflow-hidden bg-gray-50/50 dark:bg-gray-950">
        <header className="lg:hidden flex items-center gap-3 p-3 border-b border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
          <button onClick={() => setOpen(true)}><Menu size={20} /></button>
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 rounded-md bg-gradient-to-br from-violet-500 to-indigo-600 flex items-center justify-center">
              <Cat size={13} className="text-white" />
            </div>
            <span className="font-bold text-sm">ClawPanel</span>
          </div>
        </header>
        <div className="flex-1 overflow-y-auto p-4 lg:p-6"><Outlet /></div>
      </main>
    </div>
  );
}

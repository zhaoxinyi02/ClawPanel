import { useState } from 'react';
import { Outlet, NavLink } from 'react-router-dom';
import { LayoutDashboard, Bot, Settings, UserCheck, Cpu, Moon, Sun, LogOut, Menu, LogIn, MessageCircle } from 'lucide-react';

const navItems = [
  { to: '/', icon: LayoutDashboard, label: '仪表盘' },
  { to: '/openclaw', icon: Cpu, label: 'OpenClaw' },
  { to: '/qqbot', icon: Bot, label: 'QQ Bot' },
  { to: '/qqlogin', icon: LogIn, label: 'QQ 登录' },
  { to: '/wechatlogin', icon: MessageCircle, label: '微信登录' },
  { to: '/requests', icon: UserCheck, label: '审核' },
  { to: '/settings', icon: Settings, label: '设置' },
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

  return (
    <div className="flex h-screen overflow-hidden">
      {open && <div className="fixed inset-0 bg-black/50 z-40 lg:hidden" onClick={() => setOpen(false)} />}
      <aside className={`fixed lg:static inset-y-0 left-0 z-50 w-60 bg-white dark:bg-gray-900 border-r border-gray-200 dark:border-gray-800 flex flex-col transition-transform lg:translate-x-0 ${open ? 'translate-x-0' : '-translate-x-full'}`}>
        <div className="p-4 border-b border-gray-200 dark:border-gray-800">
          <h1 className="font-bold text-sm">OpenClaw Manager</h1>
          <div className="flex items-center gap-2 text-xs mt-1.5">
            <span className={`w-2 h-2 rounded-full ${napcatStatus?.connected ? 'bg-emerald-500' : 'bg-red-500'}`} />
            <span className="text-gray-500 dark:text-gray-400">
              {napcatStatus?.connected
              ? `QQ: ${napcatStatus.nickname || 'QQ'}${napcatStatus.selfId ? ` (${napcatStatus.selfId})` : ''}`
              : 'QQ 未连接'}
            </span>
          </div>
          <div className="flex items-center gap-2 text-xs mt-1">
            <span className={`w-2 h-2 rounded-full ${wechatStatus?.connected ? 'bg-emerald-500' : 'bg-red-500'}`} />
            <span className="text-gray-500 dark:text-gray-400">
              {wechatStatus?.connected
              ? `微信: ${wechatStatus.name || '已连接'}`
              : '微信 未连接'}
            </span>
          </div>
        </div>
        <nav className="flex-1 p-2 space-y-0.5 overflow-y-auto">
          {navItems.map(({ to, icon: Icon, label }) => (
            <NavLink key={to} to={to} end={to === '/'} onClick={() => setOpen(false)}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${isActive ? 'bg-indigo-50 dark:bg-indigo-950 text-indigo-700 dark:text-indigo-300 font-medium' : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800'}`
              }>
              <Icon size={17} />{label}
            </NavLink>
          ))}
        </nav>
        <div className="p-2 border-t border-gray-200 dark:border-gray-800 space-y-0.5">
          <button onClick={toggleDark} className="flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 w-full">
            {dark ? <Sun size={17} /> : <Moon size={17} />}{dark ? '浅色' : '深色'}
          </button>
          <button onClick={onLogout} className="flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-950/30 w-full">
            <LogOut size={17} />退出
          </button>
        </div>
      </aside>
      <main className="flex-1 flex flex-col overflow-hidden">
        <header className="lg:hidden flex items-center gap-3 p-3 border-b border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
          <button onClick={() => setOpen(true)}><Menu size={20} /></button>
          <span className="font-bold text-sm">OpenClaw QQ Manager</span>
        </header>
        <div className="flex-1 overflow-y-auto p-4 lg:p-6"><Outlet /></div>
      </main>
    </div>
  );
}

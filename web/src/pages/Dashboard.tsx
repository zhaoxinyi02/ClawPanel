import { useEffect, useState, useRef } from 'react';
import { api } from '../lib/api';
import {
  Wifi, WifiOff, Users, MessageCircle, Cpu, Clock, RefreshCw,
  Search, ChevronDown, ChevronRight, ArrowDown, Activity,
  MemoryStick, Radio, Sparkles, TrendingUp,
} from 'lucide-react';
import type { LogEntry } from '../hooks/useWebSocket';

interface DashboardProps {
  ws: {
    events: any[];
    logEntries: LogEntry[];
    napcatStatus: any;
    wechatStatus: any;
    clearEvents: () => void;
    refreshLog: () => void;
  };
}

export default function Dashboard({ ws }: DashboardProps) {
  const [status, setStatus] = useState<any>(null);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [autoScroll, setAutoScroll] = useState(true);
  const logRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    api.getStatus().then(r => { if (r.ok) setStatus(r); });
    const t = setInterval(() => { api.getStatus().then(r => { if (r.ok) setStatus(r); }); }, 10000);
    return () => clearInterval(t);
  }, []);

  useEffect(() => {
    if (autoScroll && logRef.current) logRef.current.scrollTop = 0;
  }, [ws.logEntries.length, autoScroll]);

  const nc = status?.napcat || {};
  const wc = status?.wechat || {};
  const oc = status?.openclaw || {};
  const adm = status?.admin || {};

  // Message stats
  const now = Date.now();
  const todayStart = new Date(); todayStart.setHours(0,0,0,0);
  const todayLogs = ws.logEntries.filter(e => e.time >= todayStart.getTime());
  const qqCount = todayLogs.filter(e => e.source === 'qq').length;
  const botCount = todayLogs.filter(e => e.source === 'openclaw').length;

  // Active channels count
  const activeChannels = [nc.connected, wc.loggedIn].filter(Boolean).length;

  return (
    <div className="space-y-4 h-full flex flex-col">
      {/* Header */}
      <div className="shrink-0">
        <h2 className="text-lg font-bold">仪表盘</h2>
        <p className="text-xs text-gray-500 mt-0.5">OpenClaw 运行状态总览</p>
      </div>

      {/* Status cards */}
      <div className="grid grid-cols-2 lg:grid-cols-3 xl:grid-cols-6 gap-3 shrink-0">
        <StatCard icon={Radio} label="活跃通道" value={`${activeChannels} 个`}
          sub={activeChannels > 0 ? [nc.connected && 'QQ', wc.loggedIn && '微信'].filter(Boolean).join(', ') : '无通道连接'}
          color={activeChannels > 0 ? 'text-emerald-600' : 'text-red-500'} />
        <StatCard icon={Cpu} label="AI 模型" value={oc.currentModel ? shortenModel(oc.currentModel) : '未设置'}
          sub={oc.currentModel || ''} color={oc.currentModel ? 'text-violet-600' : 'text-amber-500'} />
        <StatCard icon={Clock} label="运行时间" value={formatUptime(adm.uptime || 0)}
          color="text-blue-600" />
        <StatCard icon={MemoryStick} label="内存占用" value={`${adm.memoryMB || 0} MB`}
          color="text-cyan-600" />
        <StatCard icon={TrendingUp} label="今日消息" value={`${todayLogs.length}`}
          sub={`收 ${qqCount} / 发 ${botCount}`} color="text-amber-600" />
        <StatCard icon={Users} label="QQ 群/好友" value={`${nc.groupCount || 0} / ${nc.friendCount || 0}`}
          color="text-indigo-600" />
      </div>

      {/* Channel status detail */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-3 shrink-0">
        <ChannelCard
          name="QQ (NapCat)"
          connected={nc.connected}
          details={nc.connected ? [
            { label: '昵称', value: nc.nickname || '-' },
            { label: 'QQ号', value: nc.selfId || '-' },
            { label: '群数', value: String(nc.groupCount || 0) },
            { label: '好友数', value: String(nc.friendCount || 0) },
          ] : []}
        />
        <ChannelCard
          name="微信"
          connected={wc.loggedIn}
          details={wc.loggedIn ? [
            { label: '用户', value: wc.name || '-' },
            { label: '状态', value: '已登录' },
          ] : [{ label: '容器', value: wc.connected ? '已连接' : '未连接' }]}
        />
      </div>

      {/* Recent activity */}
      <div className="card flex-1 flex flex-col min-h-0">
        <div className="flex items-center justify-between px-4 pt-3 pb-2 shrink-0">
          <div className="flex items-center gap-2">
            <Activity size={14} className="text-violet-500" />
            <h3 className="font-semibold text-sm">最近活动</h3>
            <span className="text-[10px] text-gray-400 tabular-nums">{ws.logEntries.length} 条</span>
          </div>
          <div className="flex items-center gap-1.5">
            <button onClick={() => setAutoScroll(!autoScroll)}
              className={`p-1.5 rounded transition-colors ${autoScroll ? 'bg-violet-100 dark:bg-violet-900 text-violet-600' : 'text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800'}`}>
              <ArrowDown size={13} />
            </button>
            <button onClick={ws.refreshLog} className="p-1.5 rounded hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-400">
              <RefreshCw size={13} />
            </button>
          </div>
        </div>
        <div ref={logRef} className="flex-1 overflow-y-auto min-h-0 px-2 pb-2">
          {ws.logEntries.length === 0 && <p className="text-gray-400 py-8 text-center text-xs">暂无活动</p>}
          {ws.logEntries.slice(0, 50).map((entry) => (
            <div key={entry.id} className="group">
              <div
                className={`flex items-start gap-2 py-1.5 px-2 rounded cursor-pointer transition-colors text-xs
                  ${expandedId === entry.id ? 'bg-gray-50 dark:bg-gray-800/70' : 'hover:bg-gray-50 dark:hover:bg-gray-800/30'}`}
                onClick={() => setExpandedId(expandedId === entry.id ? null : entry.id)}
              >
                {entry.detail ? (
                  expandedId === entry.id
                    ? <ChevronDown size={12} className="shrink-0 mt-0.5 text-gray-400" />
                    : <ChevronRight size={12} className="shrink-0 mt-0.5 text-gray-400" />
                ) : <span className="w-3 shrink-0" />}
                <span className="text-gray-400 shrink-0 tabular-nums text-[11px]">{formatLogTime(entry.time)}</span>
                <span className={`shrink-0 px-1.5 py-0.5 rounded text-[10px] font-medium leading-none ${sourceColor(entry.source)}`}>
                  {sourceLabel(entry.source)}
                </span>
                <span className={`break-all leading-relaxed ${entry.source === 'openclaw' ? 'text-purple-700 dark:text-purple-300' : 'text-gray-700 dark:text-gray-300'}`}>
                  {entry.summary}
                </span>
              </div>
              {expandedId === entry.id && entry.detail && (
                <div className="ml-8 mr-2 mb-1 px-3 py-2 rounded bg-gray-100 dark:bg-gray-800 text-[11px] text-gray-600 dark:text-gray-400 whitespace-pre-wrap break-all max-h-40 overflow-y-auto">
                  {entry.detail}
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function ChannelCard({ name, connected, details }: { name: string; connected: boolean; details: { label: string; value: string }[] }) {
  return (
    <div className="card p-3">
      <div className="flex items-center gap-2 mb-2">
        {connected ? <Wifi size={14} className="text-emerald-500" /> : <WifiOff size={14} className="text-red-400" />}
        <span className="text-xs font-semibold">{name}</span>
        <span className={`text-[10px] px-1.5 py-0.5 rounded font-medium ${connected ? 'bg-emerald-50 dark:bg-emerald-950 text-emerald-600' : 'bg-red-50 dark:bg-red-950 text-red-500'}`}>
          {connected ? '已连接' : '未连接'}
        </span>
      </div>
      {details.length > 0 && (
        <div className="grid grid-cols-2 gap-x-4 gap-y-1">
          {details.map(d => (
            <div key={d.label} className="flex items-center gap-1.5 text-[11px]">
              <span className="text-gray-400">{d.label}:</span>
              <span className="text-gray-700 dark:text-gray-300 truncate">{d.value}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function StatCard({ icon: Icon, label, value, color, sub }: { icon: any; label: string; value: string; color: string; sub?: string }) {
  return (
    <div className="card p-3">
      <div className="flex items-center gap-2.5">
        <div className="w-8 h-8 rounded-lg bg-gray-50 dark:bg-gray-800 flex items-center justify-center shrink-0">
          <Icon size={16} className={color} />
        </div>
        <div className="min-w-0">
          <p className="text-[10px] text-gray-500 leading-tight">{label}</p>
          <p className="text-xs font-semibold truncate leading-tight">{value}</p>
          {sub && <p className="text-[10px] text-gray-400 truncate leading-tight">{sub}</p>}
        </div>
      </div>
    </div>
  );
}

function sourceColor(s: string) {
  switch (s) {
    case 'qq': return 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300';
    case 'wechat': return 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300';
    case 'system': return 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300';
    case 'openclaw': return 'bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300';
    default: return 'bg-gray-100 text-gray-600';
  }
}

function sourceLabel(s: string) {
  switch (s) {
    case 'qq': return 'QQ';
    case 'wechat': return '微信';
    case 'system': return '系统';
    case 'openclaw': return 'Bot';
    default: return s;
  }
}

function shortenModel(m: string) {
  if (m.length > 20) {
    const parts = m.split('/');
    return parts.length > 1 ? parts[parts.length - 1] : m.slice(0, 20) + '...';
  }
  return m;
}

function formatLogTime(ts: number) {
  const d = new Date(ts);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function formatUptime(s: number) {
  if (s < 60) return `${Math.floor(s)}秒`;
  if (s < 3600) return `${Math.floor(s / 60)}分`;
  if (s < 86400) return `${Math.floor(s / 3600)}时${Math.floor((s % 3600) / 60)}分`;
  return `${Math.floor(s / 86400)}天${Math.floor((s % 86400) / 3600)}时`;
}

import { useEffect, useState, useRef } from 'react';
import { api } from '../lib/api';
import {
  Wifi, Users, Cpu, Clock, RefreshCw,
  ChevronDown, ChevronRight, ArrowDown, Activity,
  MemoryStick, Radio, TrendingUp,
} from 'lucide-react';
import type { LogEntry } from '../hooks/useWebSocket';
import { useI18n } from '../i18n';

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
  const { t, locale } = useI18n();
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

  const todayStart = new Date(); todayStart.setHours(0,0,0,0);
  const todayLogs = ws.logEntries.filter(e => e.time >= todayStart.getTime());
  const qqCount = todayLogs.filter(e => e.source === 'qq').length;
  const botCount = todayLogs.filter(e => e.source === 'openclaw').length;

  // Build connected channels dynamically from enabledChannels returned by /api/status
  const enabledChannels: { id: string; label: string; type: string }[] = oc.enabledChannels || [];
  const connectedChannels: { name: string; status: string; details: { label: string; value: string }[] }[] = [];

  for (const ch of enabledChannels) {
    if (ch.id === 'qq') {
      // QQ (NapCat) — has detailed status from OneBot client
      connectedChannels.push({
        name: ch.label,
        status: nc.connected ? t.common.connected : t.common.notLoggedIn,
        details: [
          { label: t.dashboard.nickname, value: nc.nickname || '-' },
          { label: t.dashboard.qqNumber, value: nc.selfId || '-' },
          { label: t.dashboard.groups, value: String(nc.groupCount || 0) },
          { label: t.dashboard.friends, value: String(nc.friendCount || 0) },
        ],
      });
    } else if (ch.id === 'wechat') {
      // WeChat — has detailed status from wechat client
      connectedChannels.push({
        name: ch.label,
        status: wc.loggedIn ? t.common.connected : t.common.notLoggedIn,
        details: [
          { label: t.dashboard.user, value: wc.name || '-' },
          { label: t.common.status, value: wc.loggedIn ? t.dashboard.loggedIn : t.common.notLoggedIn },
        ],
      });
    } else {
      // All other channels (feishu, qqbot, dingtalk, etc.) — enabled in config
      connectedChannels.push({
        name: ch.label,
        status: t.common.enabled,
        details: [
          { label: t.dashboard.channelType, value: ch.type === 'plugin' ? t.dashboard.pluginChannel : t.dashboard.builtinChannel },
          { label: t.common.status, value: t.dashboard.managedByGateway },
        ],
      });
    }
  }

  return (
    <div className="space-y-6 h-full flex flex-col p-2">
      {/* Header */}
      <div className="shrink-0 flex items-center justify-between">
        <div>
          <h2 className="text-xl font-bold text-gray-900 dark:text-white tracking-tight">{t.dashboard.title}</h2>
          <p className="text-sm text-gray-500 mt-1">{t.dashboard.subtitle}</p>
        </div>
        <div className="flex items-center gap-2">
          <span className="flex h-2.5 w-2.5 relative">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
            <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-emerald-500"></span>
          </span>
          <span className="text-xs font-medium text-emerald-600 dark:text-emerald-400">{t.dashboard.systemNormal}</span>
        </div>
      </div>

      {/* Status cards */}
      <div className="grid grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-4 shrink-0">
        <StatCard icon={Radio} label={t.dashboard.activeChannels} value={`${connectedChannels.length}`} unit={t.dashboard.channelUnit || undefined}
          sub={connectedChannels.length > 0 ? connectedChannels.map(c => c.name).join(', ') : t.dashboard.noChannels}
          color="text-emerald-600" bg="bg-emerald-50 dark:bg-emerald-900/20" />
        <StatCard icon={Cpu} label={t.dashboard.aiModel} value={oc.currentModel ? shortenModel(oc.currentModel) : t.dashboard.notSet}
          sub={oc.currentModel || ''} color="text-violet-600" bg="bg-violet-50 dark:bg-violet-900/20" />
        <StatCard icon={Clock} label={t.dashboard.uptime} value={formatUptime(adm.uptime || 0, t).split(/(\d+)/)[1]} unit={formatUptime(adm.uptime || 0, t).split(/(\d+)/)[2]}
          color="text-blue-600" bg="bg-blue-50 dark:bg-blue-900/20" />
        <StatCard icon={MemoryStick} label={t.dashboard.memory} value={`${adm.memoryMB || 0}`} unit="MB"
          color="text-cyan-600" bg="bg-cyan-50 dark:bg-cyan-900/20" />
        <StatCard icon={TrendingUp} label={t.dashboard.todayMessages} value={`${todayLogs.length}`} unit={t.dashboard.msgUnit || undefined}
          sub={`${t.dashboard.received} ${qqCount} / ${t.dashboard.sent} ${botCount}`} color="text-amber-600" bg="bg-amber-50 dark:bg-amber-900/20" />
      </div>

      {/* Connected channel cards — only show connected */}
      {connectedChannels.length > 0 && (
        <div className="shrink-0">
          <h3 className="text-sm font-semibold text-gray-500 mb-3 px-1">{t.dashboard.connectedChannels}</h3>
          <div className={`grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4`}>
            {connectedChannels.map(ch => (
              <div key={ch.name} className="bg-white dark:bg-gray-800 rounded-xl p-4 shadow-sm border border-gray-100 dark:border-gray-700/50 hover:shadow-md transition-shadow">
                <div className="flex items-center justify-between mb-3">
                  <div className="flex items-center gap-2">
                    <div className={`p-1.5 rounded-lg ${ch.status === t.common.connected ? 'bg-emerald-50 text-emerald-600' : ch.status === t.common.enabled ? 'bg-blue-50 text-blue-600' : 'bg-amber-50 text-amber-600'}`}>
                      <Wifi size={16} />
                    </div>
                    <span className="font-semibold text-sm text-gray-900 dark:text-gray-100">{ch.name}</span>
                  </div>
                  <span className={`text-[10px] px-2 py-0.5 rounded-full font-medium ${ch.status === t.common.connected ? 'bg-emerald-50 dark:bg-emerald-900/30 text-emerald-600 dark:text-emerald-400' : ch.status === t.common.enabled ? 'bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400' : 'bg-amber-50 dark:bg-amber-900/30 text-amber-600 dark:text-amber-400'}`}>{ch.status}</span>
                </div>
                <div className="grid grid-cols-2 gap-y-2 gap-x-4">
                  {ch.details.map(d => (
                    <div key={d.label} className="flex flex-col">
                      <span className="text-[10px] text-gray-400 font-medium uppercase tracking-wider">{d.label}</span>
                      <span className="text-xs font-medium text-gray-700 dark:text-gray-300 truncate">{d.value}</span>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Recent activity */}
      <div className="bg-white dark:bg-gray-800 rounded-2xl shadow-sm border border-gray-100 dark:border-gray-700/50 flex-1 flex flex-col min-h-0 overflow-hidden">
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-50 dark:border-gray-700/50 shrink-0 bg-gray-50/30 dark:bg-gray-800/50">
          <div className="flex items-center gap-2.5">
            <div className="p-1.5 rounded-lg bg-violet-100 dark:bg-violet-900/30 text-violet-600">
              <Activity size={16} />
            </div>
            <div>
              <h3 className="font-bold text-sm text-gray-900 dark:text-white">{t.dashboard.recentActivity}</h3>
              <p className="text-[10px] text-gray-500">{t.dashboard.realtimeLog} ({ws.logEntries.length})</p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <button onClick={() => setAutoScroll(!autoScroll)}
              title={autoScroll ? t.dashboard.pauseScroll : t.dashboard.resumeScroll}
              className={`p-2 rounded-lg transition-all ${autoScroll ? 'bg-violet-50 text-violet-600 shadow-sm ring-1 ring-violet-100' : 'text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700'}`}>
              <ArrowDown size={14} />
            </button>
            <button onClick={ws.refreshLog} title={t.dashboard.refreshLog} className="p-2 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-400 transition-colors">
              <RefreshCw size={14} />
            </button>
          </div>
        </div>
        <div ref={logRef} className="flex-1 overflow-y-auto min-h-0 p-2 space-y-0.5">
          {ws.logEntries.length === 0 && (
            <div className="h-full flex flex-col items-center justify-center text-gray-400 gap-2">
              <Clock size={32} className="opacity-20" />
              <p className="text-xs">{t.dashboard.noActivity}</p>
            </div>
          )}
          {ws.logEntries.slice(0, 100).map((entry) => (
            <div key={entry.id} className="group">
              <div
                className={`flex items-start gap-3 py-2 px-3 rounded-lg cursor-pointer transition-all duration-200 text-xs border border-transparent
                  ${expandedId === entry.id ? 'bg-gray-50 dark:bg-gray-700/30 border-gray-100 dark:border-gray-700' : 'hover:bg-gray-50 dark:hover:bg-gray-700/20'}`}
                onClick={() => setExpandedId(expandedId === entry.id ? null : entry.id)}
              >
                <span className="text-gray-400 shrink-0 font-mono text-[10px] pt-0.5 opacity-70 group-hover:opacity-100 transition-opacity">{formatLogTime(entry.time)}</span>
                
                <span className={`shrink-0 px-2 py-0.5 rounded-md text-[10px] font-bold tracking-wide uppercase ${sourceColor(entry.source)}`}>
                  {sourceLabel(entry.source)}
                </span>
                
                <div className="flex-1 min-w-0">
                  <p className={`truncate font-medium leading-relaxed ${entry.source === 'openclaw' ? 'text-violet-700 dark:text-violet-300' : 'text-gray-700 dark:text-gray-300'}`}>
                    {entry.summary}
                  </p>
                </div>

                {entry.detail ? (
                  <ChevronDown size={12} className={`shrink-0 text-gray-300 transition-transform duration-200 ${expandedId === entry.id ? 'rotate-180 text-gray-500' : ''}`} />
                ) : <span className="w-3" />}
              </div>
              {expandedId === entry.id && entry.detail && (
                <div className="ml-12 mr-3 mb-2 mt-1 px-4 py-3 rounded-lg bg-gray-50 dark:bg-gray-900/50 border border-gray-100 dark:border-gray-800 text-[11px] font-mono text-gray-600 dark:text-gray-400 whitespace-pre-wrap break-all shadow-inner max-h-60 overflow-y-auto">
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

function StatCard({ icon: Icon, label, value, unit, color, bg, sub }: { icon: any; label: string; value: string; unit?: string; color: string; bg: string; sub?: string }) {
  return (
    <div className="bg-white dark:bg-gray-800 rounded-xl p-4 shadow-sm border border-gray-100 dark:border-gray-700/50 flex flex-col justify-between hover:shadow-md transition-shadow group h-24">
      <div className="flex items-start justify-between">
        <div className={`p-2 rounded-lg ${bg} ${color} transition-transform group-hover:scale-105`}>
          <Icon size={18} />
        </div>
        {sub && <span className="text-[10px] font-medium px-1.5 py-0.5 rounded-full bg-gray-50 dark:bg-gray-700 text-gray-500 max-w-[80px] truncate">{sub}</span>}
      </div>
      <div>
        <p className="text-[10px] font-medium text-gray-400 uppercase tracking-wider mb-0.5">{label}</p>
        <div className="flex items-baseline gap-1">
          <span className="text-lg font-bold text-gray-900 dark:text-white tracking-tight">{value}</span>
          {unit && <span className="text-[10px] text-gray-400 font-medium">{unit}</span>}
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
    case 'wechat': return 'WeChat';
    case 'system': return 'SYS';
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
  const now = new Date();
  const isToday = d.getFullYear() === now.getFullYear() && d.getMonth() === now.getMonth() && d.getDate() === now.getDate();
  const time = `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
  if (isToday) return time;
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${time}`;
}

function formatUptime(s: number, t: any) {
  if (s < 60) return `${Math.floor(s)}${t.dashboard.seconds}`;
  if (s < 3600) return `${Math.floor(s / 60)}${t.dashboard.minutes}`;
  if (s < 86400) return `${Math.floor(s / 3600)}${t.dashboard.hours}${Math.floor((s % 3600) / 60)}${t.dashboard.minutes}`;
  return `${Math.floor(s / 86400)}${t.dashboard.days}${Math.floor((s % 86400) / 3600)}${t.dashboard.hours}`;
}

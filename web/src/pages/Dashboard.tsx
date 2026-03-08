import { useEffect, useState, useRef } from 'react';
import { useOutletContext } from 'react-router-dom';
import { api } from '../lib/api';
import {
  Wifi, Users, Cpu, Clock, RefreshCw,
  ChevronDown, ChevronRight, ArrowDown, Activity,
  MemoryStick, Radio, TrendingUp, AlertTriangle, Download, Brain, Loader2,
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
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const modern = uiMode === 'modern';
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
  const gateway = status?.gateway || {};
  const proc = status?.process || {};
  const adm = status?.admin || {};

  const todayStart = new Date(); todayStart.setHours(0,0,0,0);
  const filteredLogs = ws.logEntries.filter(e => !isNoiseEvent(e));
  const todayLogs = filteredLogs.filter(e => e.time >= todayStart.getTime());
  const messageLogs = todayLogs.filter(e => e.source !== 'system');
  const inboundCount = messageLogs.filter(e => e.source === 'qq' || e.source === 'wechat').length;
  const botCount = messageLogs.filter(e => e.source === 'openclaw').length;

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

  const [installingOC, setInstallingOC] = useState(false);
  const handleInstallOpenClaw = async () => {
    setInstallingOC(true);
    try {
      const r = await api.installSoftware('openclaw');
      if (!r.ok) console.error(r.error);
    } catch {}
    finally { setInstallingOC(false); }
  };

  return (
    <div className={`space-y-6 h-full flex flex-col ${modern ? 'p-0' : 'p-2'}`}>
      {/* OpenClaw not installed banner */}
      {status && !oc.configured && (
        <div className="shrink-0 rounded-[28px] border border-amber-200/70 dark:border-amber-700/40 bg-[linear-gradient(135deg,rgba(255,251,235,0.82),rgba(219,234,254,0.56))] dark:bg-[linear-gradient(135deg,rgba(120,53,15,0.22),rgba(30,64,175,0.18))] backdrop-blur-xl p-6 flex flex-col items-center gap-4 text-center shadow-[0_18px_40px_rgba(15,23,42,0.06)]">
          <div className="w-14 h-14 rounded-2xl bg-white/70 dark:bg-slate-900/60 border border-white/70 dark:border-slate-700/60 flex items-center justify-center shadow-sm">
            <Brain size={28} className="text-amber-600 dark:text-amber-400" />
          </div>
          <div>
            <h3 className="text-lg font-bold text-gray-900 dark:text-white">OpenClaw 尚未安装</h3>
            <p className="text-sm text-gray-500 mt-1 max-w-md mx-auto">
              ClawPanel 需要 OpenClaw AI 引擎才能正常工作。安装后即可配置模型、管理技能和连接通道。
            </p>
          </div>
          <button onClick={handleInstallOpenClaw} disabled={installingOC}
            className={`${modern ? 'page-modern-accent px-6 py-3 text-sm' : 'inline-flex items-center gap-2 px-6 py-3 text-sm font-medium rounded-xl bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50 transition-all shadow-lg shadow-violet-200 dark:shadow-none'}`}>
            {installingOC ? <Loader2 size={16} className="animate-spin" /> : <Download size={16} />}
            {installingOC ? '安装中...' : '一键安装 OpenClaw'}
          </button>
          <p className="text-[11px] text-gray-400">安装进度可在右上角铃铛中的消息中心实时查看</p>
        </div>
      )}

      {/* Header */}
      <div className={`shrink-0 flex items-center justify-between ${modern ? 'px-1' : ''}`}>
        <div>
          <h2 className={`font-bold tracking-tight ${modern ? 'text-2xl text-slate-900 dark:text-white' : 'text-xl text-gray-900 dark:text-white'}`}>{t.dashboard.title}</h2>
          <p className={`text-sm mt-1 ${modern ? 'text-slate-500' : 'text-gray-500'}`}>{t.dashboard.subtitle}</p>
        </div>
        <div className={`flex items-center gap-2 ${modern ? 'px-4 py-2 rounded-2xl border border-emerald-200/60 dark:border-emerald-800/40 bg-[linear-gradient(135deg,rgba(255,255,255,0.72),rgba(236,253,245,0.78))] dark:bg-[linear-gradient(135deg,rgba(6,78,59,0.26),rgba(15,23,42,0.78))] backdrop-blur-xl shadow-[0_14px_30px_rgba(15,23,42,0.05)]' : ''}`}>
          <span className="flex h-2.5 w-2.5 relative">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
            <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-emerald-500"></span>
          </span>
          <span className="text-xs font-medium text-emerald-600 dark:text-emerald-400">{t.dashboard.systemNormal}</span>
        </div>
      </div>

      {/* Status cards */}
      <div className={`grid ${modern ? 'grid-cols-1 md:grid-cols-2 xl:grid-cols-4 2xl:grid-cols-6' : 'grid-cols-2 lg:grid-cols-3 xl:grid-cols-5'} gap-4 shrink-0`}>
        <StatCard icon={Brain} label="OpenClaw" value={proc.running ? '运行中' : '未运行'}
          sub={proc.running ? (proc.managedExternally ? '外部管理' : `PID ${proc.pid || '-'}`) : '需要启动'}
          color={proc.running ? 'text-emerald-600' : 'text-red-600'}
          bg={proc.running ? 'bg-emerald-50 dark:bg-emerald-900/20' : 'bg-red-50 dark:bg-red-900/20'} modern={modern} />
        <StatCard icon={Wifi} label="网关" value={gateway.running ? '在线' : '离线'}
          sub={gateway.running ? '消息链路可用' : '建议重启网关'}
          color={gateway.running ? 'text-blue-600' : 'text-amber-600'}
          bg={gateway.running ? 'bg-blue-50 dark:bg-blue-900/20' : 'bg-amber-50 dark:bg-amber-900/20'} modern={modern} />
        <StatCard icon={Radio} label={t.dashboard.activeChannels} value={`${connectedChannels.length}`} unit={t.dashboard.channelUnit || undefined}
          sub={connectedChannels.length > 0 ? connectedChannels.map(c => c.name).join(', ') : t.dashboard.noChannels}
          color="text-emerald-600" bg="bg-emerald-50 dark:bg-emerald-900/20" modern={modern} />
        <StatCard icon={Cpu} label={t.dashboard.aiModel} value={oc.currentModel ? shortenModel(oc.currentModel) : t.dashboard.notSet}
          sub={oc.currentModel || ''} color="text-violet-600" bg="bg-violet-50 dark:bg-violet-900/20" modern={modern} />
        <StatCard icon={Clock} label={t.dashboard.uptime} value={formatUptime(adm.uptime || 0, t).split(/(\d+)/)[1]} unit={formatUptime(adm.uptime || 0, t).split(/(\d+)/)[2]}
          color="text-blue-600" bg="bg-blue-50 dark:bg-blue-900/20" modern={modern} />
        <StatCard icon={MemoryStick} label={t.dashboard.memory} value={`${adm.memoryMB || 0}`} unit="MB"
          color="text-cyan-600" bg="bg-cyan-50 dark:bg-cyan-900/20" modern={modern} />
        <StatCard icon={TrendingUp} label={t.dashboard.todayMessages} value={`${messageLogs.length}`} unit={t.dashboard.msgUnit || undefined}
          sub={`${t.dashboard.received} ${inboundCount} / ${t.dashboard.sent} ${botCount}`} color="text-amber-600" bg="bg-amber-50 dark:bg-amber-900/20" modern={modern} />
        {modern && (
          <StatCard icon={Users} label={t.dashboard.connectedChannels} value={`${connectedChannels.filter(c => c.status === t.common.connected).length}`} unit={t.dashboard.channelUnit || undefined}
            sub={connectedChannels.length > 0 ? 'Live status' : t.dashboard.noChannels} color="text-indigo-600" bg="bg-indigo-50 dark:bg-indigo-900/20" modern={modern} />
        )}
      </div>

      {/* Connected channel cards — only show connected */}
      {connectedChannels.length > 0 && (
        <div className="shrink-0">
          <h3 className="text-sm font-semibold text-gray-500 mb-3 px-1">{t.dashboard.connectedChannels}</h3>
          <div className={`grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4`}>
            {connectedChannels.map(ch => (
              <div key={ch.name} className={`${modern ? 'relative overflow-hidden rounded-3xl p-5 border border-white/60 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.82),rgba(239,246,255,0.66))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.86),rgba(15,118,110,0.14))] backdrop-blur-xl shadow-[0_18px_40px_rgba(15,23,42,0.06)]' : 'bg-white dark:bg-gray-800 rounded-xl p-4 shadow-sm border border-gray-100 dark:border-gray-700/50'} hover:shadow-md transition-shadow`}>
                {modern && <div className="pointer-events-none absolute inset-x-5 top-0 h-px bg-gradient-to-r from-transparent via-white/90 to-transparent dark:via-slate-200/20" />}
                <div className="flex items-center justify-between mb-3">
                  <div className="flex items-center gap-2">
                    <div className={`p-1.5 rounded-xl border ${ch.status === t.common.connected ? 'bg-emerald-50/80 border-emerald-100 text-emerald-600 dark:bg-emerald-900/20 dark:border-emerald-800/40' : ch.status === t.common.enabled ? 'bg-blue-50/80 border-blue-100 text-blue-600 dark:bg-blue-900/20 dark:border-blue-800/40' : 'bg-amber-50/80 border-amber-100 text-amber-600 dark:bg-amber-900/20 dark:border-amber-800/40'}`}>
                      <Wifi size={16} />
                    </div>
                    <span className="font-semibold text-sm text-gray-900 dark:text-gray-100 tracking-tight">{ch.name}</span>
                  </div>
                  <span className={`text-[10px] px-2.5 py-1 rounded-full font-semibold border ${ch.status === t.common.connected ? 'bg-emerald-50/90 border-emerald-100 text-emerald-600 dark:bg-emerald-900/25 dark:border-emerald-800/40 dark:text-emerald-400' : ch.status === t.common.enabled ? 'bg-blue-50/90 border-blue-100 text-blue-600 dark:bg-blue-900/25 dark:border-blue-800/40 dark:text-blue-400' : 'bg-amber-50/90 border-amber-100 text-amber-600 dark:bg-amber-900/25 dark:border-amber-800/40 dark:text-amber-400'}`}>{ch.status}</span>
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
      <div className={`${modern ? 'rounded-[28px] border border-white/60 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.84),rgba(239,246,255,0.62))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.9),rgba(30,64,175,0.12))] backdrop-blur-xl shadow-[0_22px_48px_rgba(15,23,42,0.06)]' : 'bg-white dark:bg-gray-800 shadow-sm border border-gray-100 dark:border-gray-700/50'} flex-1 flex flex-col min-h-0 overflow-hidden`}>
        <div className={`flex items-center justify-between px-5 py-4 shrink-0 ${modern ? 'border-b border-slate-200/60 dark:border-slate-700/50 bg-white/30 dark:bg-slate-900/20 backdrop-blur-xl' : 'border-b border-gray-50 dark:border-gray-700/50 bg-gray-50/30 dark:bg-gray-800/50'}`}>
          <div className="flex items-center gap-2.5">
            <div className="p-1.5 rounded-xl border border-blue-100/80 dark:border-blue-800/40 bg-blue-50/80 dark:bg-blue-900/20 text-blue-600 dark:text-blue-300 shadow-sm">
              <Activity size={16} />
            </div>
            <div>
              <h3 className="font-bold text-sm text-gray-900 dark:text-white">{t.dashboard.recentActivity}</h3>
              <div className="flex items-center gap-2 mt-1">
                <p className="text-[10px] text-gray-500">{t.dashboard.realtimeLog}</p>
                <span className="px-2 py-0.5 rounded-full text-[10px] font-semibold border border-blue-100/80 bg-blue-50/80 text-blue-600 dark:border-blue-800/40 dark:bg-blue-900/20 dark:text-blue-300">{filteredLogs.length}</span>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <button onClick={() => setAutoScroll(!autoScroll)}
              title={autoScroll ? t.dashboard.pauseScroll : t.dashboard.resumeScroll}
              className={`p-2 rounded-xl transition-all ${autoScroll ? 'bg-blue-50/80 text-blue-600 shadow-sm ring-1 ring-blue-100 dark:bg-blue-900/20 dark:ring-blue-800/40 dark:text-blue-300' : 'text-gray-400 hover:bg-white/70 dark:hover:bg-slate-800/70'}`}>
              <ArrowDown size={14} />
            </button>
              <button onClick={ws.refreshLog} title={t.dashboard.refreshLog} className="p-2 rounded-xl hover:bg-white/70 dark:hover:bg-slate-800/70 text-gray-400 transition-colors">
                <RefreshCw size={14} />
              </button>
          </div>
        </div>
        <div ref={logRef} className="flex-1 overflow-y-auto min-h-0 p-2 space-y-0.5">
          {filteredLogs.length === 0 && (
            <div className="h-full flex flex-col items-center justify-center text-gray-400 gap-2">
              <Clock size={32} className="opacity-20" />
              <p className="text-xs">{t.dashboard.noActivity}</p>
            </div>
          )}
          {filteredLogs.slice(0, 100).map((entry) => {
            const detailText = (entry.detail || entry.summary || '').trim();
            const canExpand = detailText.length > 0;
            return (
              <div key={entry.id} className="group">
                <div
                  className={`flex items-start gap-3 py-2.5 px-3.5 rounded-xl cursor-pointer transition-all duration-200 text-xs border border-transparent
                    ${expandedId === entry.id ? 'bg-white/72 dark:bg-slate-800/55 border-blue-100/70 dark:border-slate-700/70 shadow-sm shadow-blue-100/30 dark:shadow-none' : 'hover:bg-white/68 dark:hover:bg-slate-800/35 hover:border-blue-100/50 dark:hover:border-slate-700/50'}`}
                  onClick={() => setExpandedId(expandedId === entry.id ? null : entry.id)}
                >
                  <span className="text-gray-400 shrink-0 font-mono text-[10px] pt-0.5 opacity-70 group-hover:opacity-100 transition-opacity">{formatLogTime(entry.time)}</span>

                  <span className={`shrink-0 px-2.5 py-1 rounded-full text-[10px] font-bold tracking-wide uppercase border ${sourceColor(entry.source)}`}>
                    {sourceLabel(entry.source)}
                  </span>

                  <div className="flex-1 min-w-0">
                    <p className={`truncate font-medium leading-relaxed ${entry.source === 'openclaw' ? 'text-blue-700 dark:text-blue-300' : 'text-gray-700 dark:text-gray-300'}`}>
                      {entry.summary}
                    </p>
                  </div>

                  {canExpand ? (
                    <ChevronDown size={12} className={`shrink-0 text-gray-300 transition-transform duration-200 ${expandedId === entry.id ? 'rotate-180 text-gray-500' : ''}`} />
                  ) : <span className="w-3" />}
                </div>
                {expandedId === entry.id && canExpand && (
                  <div className="ml-12 mr-3 mb-2 mt-1 px-4 py-3 rounded-xl bg-white/65 dark:bg-slate-900/55 border border-blue-100/70 dark:border-slate-700/70 text-[11px] font-mono text-gray-600 dark:text-gray-400 whitespace-pre-wrap break-all shadow-inner max-h-60 overflow-y-auto backdrop-blur-xl">
                    {detailText}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function StatCard({ icon: Icon, label, value, unit, color, bg, sub, modern }: { icon: any; label: string; value: string; unit?: string; color: string; bg: string; sub?: string; modern?: boolean }) {
  return (
    <div className={`${modern ? 'relative overflow-hidden rounded-[26px] p-5 h-[132px] border border-white/65 dark:border-slate-700/55 bg-[linear-gradient(145deg,rgba(255,255,255,0.84),rgba(239,246,255,0.62))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.88),rgba(30,64,175,0.12))] backdrop-blur-xl shadow-[0_18px_40px_rgba(15,23,42,0.06)]' : 'bg-white dark:bg-gray-800 rounded-xl p-4 shadow-sm border border-gray-100 dark:border-gray-700/50 h-24'} flex flex-col justify-between hover:shadow-md transition-shadow group`}>
      {modern && <div className="pointer-events-none absolute inset-x-5 top-0 h-px bg-gradient-to-r from-transparent via-white/95 to-transparent dark:via-slate-200/20" />}
      <div className="flex items-start justify-between">
        <div className={`p-2 rounded-xl border border-white/70 dark:border-slate-700/60 ${bg} ${color} transition-transform group-hover:scale-105 shadow-sm`}>
          <Icon size={18} />
        </div>
        {sub && <span className={`text-[10px] font-semibold px-2.5 py-1 rounded-full max-w-[124px] truncate border ${modern ? 'bg-white/75 text-slate-500 border-white/75 dark:bg-slate-900/50 dark:border-slate-700/60' : 'bg-gray-50 dark:bg-gray-700 text-gray-500'}`}>{sub}</span>}
      </div>
      <div>
        <p className={`text-[10px] font-medium uppercase tracking-wider mb-0.5 ${modern ? 'text-slate-400' : 'text-gray-400'}`}>{label}</p>
        <div className="flex items-baseline gap-1">
          <span className={`${modern ? 'text-[28px] leading-none' : 'text-lg'} font-bold text-gray-900 dark:text-white tracking-tight`}>{value}</span>
          {unit && <span className={`font-medium ${modern ? 'text-xs text-slate-400' : 'text-[10px] text-gray-400'}`}>{unit}</span>}
        </div>
      </div>
    </div>
  );
}

function sourceColor(s: string) {
  switch (s) {
    case 'qq': return 'bg-blue-100/90 border-blue-100 text-blue-700 dark:bg-blue-900/25 dark:border-blue-800/40 dark:text-blue-300';
    case 'wechat': return 'bg-emerald-100/90 border-emerald-100 text-emerald-700 dark:bg-emerald-900/25 dark:border-emerald-800/40 dark:text-emerald-300';
    case 'system': return 'bg-slate-100/90 border-slate-200 text-slate-700 dark:bg-slate-800/70 dark:border-slate-700 dark:text-slate-300';
    case 'openclaw': return 'bg-sky-100/90 border-sky-100 text-sky-700 dark:bg-sky-900/25 dark:border-sky-800/40 dark:text-sky-300';
    default: return 'bg-slate-100/90 border-slate-200 text-slate-600';
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

function isNoiseEvent(entry: { source: string; type: string; summary: string }) {
  return entry.source === 'qq' && entry.type === 'notice.notify';
}

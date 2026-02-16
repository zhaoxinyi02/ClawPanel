import { useEffect, useState, useRef } from 'react';
import { api } from '../lib/api';
import {
  Search, ChevronDown, ChevronRight, Trash2, ArrowDown, RefreshCw,
  Download, Filter,
} from 'lucide-react';
import type { LogEntry } from '../hooks/useWebSocket';
import { useI18n } from '../i18n';

interface Props {
  ws: {
    events: any[];
    logEntries: LogEntry[];
    napcatStatus: any;
    wechatStatus: any;
    clearEvents: () => void;
    refreshLog: () => void;
  };
}

export default function ActivityLog({ ws }: Props) {
  const { t } = useI18n();
  const [search, setSearch] = useState('');
  const [sourceFilter, setSourceFilter] = useState<string>('');
  const [typeFilter, setTypeFilter] = useState<string>('');
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [autoScroll, setAutoScroll] = useState(true);
  const logRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (autoScroll && logRef.current) {
      logRef.current.scrollTop = 0;
    }
  }, [ws.logEntries.length, autoScroll]);

  const filteredLog = ws.logEntries.filter(e => {
    if (sourceFilter && e.source !== sourceFilter) return false;
    if (typeFilter) {
      const isMedia = e.summary.includes('[图片]') || e.summary.includes('[动画表情]') || e.summary.includes('[视频]') || e.summary.includes('[语音]');
      const isSticker = e.summary.includes('[动画表情]') || e.summary.includes('[QQ表情');
      if (typeFilter === 'media' && !isMedia) return false;
      if (typeFilter === 'sticker' && !isSticker) return false;
      if (typeFilter === 'text' && isMedia) return false;
    }
    if (search) {
      const q = search.toLowerCase();
      return e.summary.toLowerCase().includes(q) || (e.detail || '').toLowerCase().includes(q);
    }
    return true;
  });

  const sourceCounts = ws.logEntries.reduce((acc, e) => {
    acc[e.source] = (acc[e.source] || 0) + 1;
    return acc;
  }, {} as Record<string, number>);

  const handleExport = () => {
    const lines = filteredLog.map(e => {
      const t = new Date(e.time).toLocaleString();
      return `[${t}] [${e.source}] ${e.summary}${e.detail ? '\n  ' + e.detail : ''}`;
    });
    const blob = new Blob([lines.join('\n')], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `clawpanel-log-${new Date().toISOString().slice(0, 10)}.txt`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="h-full flex flex-col space-y-4">
      <div className="flex items-center justify-between shrink-0">
        <div>
          <h2 className="text-xl font-bold text-gray-900 dark:text-white tracking-tight">{t.activityLog.title}</h2>
          <p className="text-sm text-gray-500 mt-1">{t.activityLog.subtitle}</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={handleExport} className="flex items-center gap-2 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 transition-colors shadow-sm">
            <Download size={14} />{t.activityLog.exportLog}
          </button>
        </div>
      </div>

      <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 flex-1 flex flex-col min-h-0 overflow-hidden">
        {/* Toolbar */}
        <div className="flex flex-col gap-3 px-5 py-4 border-b border-gray-100 dark:border-gray-800 bg-gray-50/30 dark:bg-gray-800/50 shrink-0">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2 text-sm text-gray-500">
              <span className="font-semibold text-gray-900 dark:text-white tabular-nums">{filteredLog.length}</span> {t.common.records}
            </div>
            <div className="flex items-center gap-2">
              <button onClick={() => setAutoScroll(!autoScroll)}
                className={`p-2 rounded-lg transition-all ${autoScroll ? 'bg-violet-50 text-violet-600 shadow-sm ring-1 ring-violet-100' : 'text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700'}`}
                title={autoScroll ? t.dashboard.pauseScroll : t.dashboard.resumeScroll}>
                <ArrowDown size={14} />
              </button>
              <button onClick={ws.refreshLog} className="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-400 transition-colors" title={t.common.refresh}>
                <RefreshCw size={14} />
              </button>
              <button onClick={ws.clearEvents} className="p-2 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/20 text-gray-400 hover:text-red-500 transition-colors" title={t.activityLog.clear}>
                <Trash2 size={14} />
              </button>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-3">
            <div className="relative flex-1 min-w-[200px] max-w-md">
              <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input 
                value={search} 
                onChange={e => setSearch(e.target.value)} 
placeholder={t.activityLog.searchPlaceholder}
                className="w-full pl-9 pr-4 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500 transition-all" 
              />
            </div>
            
            <div className="h-4 w-px bg-gray-200 dark:bg-gray-700 hidden sm:block"></div>
            
            <div className="flex items-center gap-1.5 overflow-x-auto pb-1 sm:pb-0 scrollbar-hide">
              {[
                { key: '', label: t.common.all, count: ws.logEntries.length },
                { key: 'qq', label: 'QQ', count: sourceCounts['qq'] || 0 },
                { key: 'openclaw', label: t.activityLog.botReply, count: sourceCounts['openclaw'] || 0 },
                { key: 'wechat', label: 'WeChat', count: sourceCounts['wechat'] || 0 },
                { key: 'system', label: t.dashboard.sourceSystem, count: sourceCounts['system'] || 0 },
              ].map(f => (
                <button key={f.key} onClick={() => setSourceFilter(f.key)}
                  className={`px-2.5 py-1.5 text-xs rounded-lg transition-all whitespace-nowrap flex items-center gap-1.5 ${
                    sourceFilter === f.key 
                    ? 'bg-violet-100 dark:bg-violet-900/40 text-violet-700 dark:text-violet-300 font-semibold shadow-sm' 
                    : 'text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700/50 hover:text-gray-700 dark:hover:text-gray-300'
                  }`}>
                  {f.label}
                  {f.count > 0 && <span className={`px-1.5 py-0.5 rounded-full text-[10px] ${sourceFilter === f.key ? 'bg-white/50 text-violet-800' : 'bg-gray-100 dark:bg-gray-700 text-gray-500'}`}>{f.count}</span>}
                </button>
              ))}
            </div>

            <div className="h-4 w-px bg-gray-200 dark:bg-gray-700 hidden sm:block"></div>

            <div className="flex items-center gap-1.5">
              <Filter size={14} className="text-gray-400" />
              {[
                { key: '', label: t.activityLog.allTypes },
                { key: 'text', label: t.activityLog.text },
                { key: 'media', label: t.activityLog.media },
                { key: 'sticker', label: t.activityLog.sticker },
              ].map(f => (
                <button key={f.key} onClick={() => setTypeFilter(f.key)}
                  className={`px-2.5 py-1.5 text-xs rounded-lg transition-all whitespace-nowrap ${
                    typeFilter === f.key 
                    ? 'bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-300 font-semibold shadow-sm' 
                    : 'text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700/50 hover:text-gray-700 dark:hover:text-gray-300'
                  }`}>
                  {f.label}
                </button>
              ))}
            </div>
          </div>
        </div>

        {/* Log entries */}
        <div ref={logRef} className="flex-1 overflow-y-auto min-h-0 p-2 space-y-0.5">
          {filteredLog.length === 0 && (
            <div className="h-full flex flex-col items-center justify-center text-gray-400 gap-3">
              <div className="p-4 rounded-full bg-gray-50 dark:bg-gray-800/50">
                <Search size={24} className="opacity-20" />
              </div>
              <p className="text-sm">{t.activityLog.noMatch}</p>
            </div>
          )}
          {filteredLog.slice(0, 500).map((entry) => (
            <div key={entry.id} className="group">
              <div
                className={`flex items-start gap-3 py-2 px-3 rounded-lg cursor-pointer transition-all duration-200 text-xs border border-transparent
                  ${expandedId === entry.id ? 'bg-gray-50 dark:bg-gray-800 border-gray-100 dark:border-gray-700 shadow-sm' : 'hover:bg-gray-50 dark:hover:bg-gray-800/50'}`}
                onClick={() => setExpandedId(expandedId === entry.id ? null : entry.id)}
              >
                <span className="text-gray-400 shrink-0 font-mono text-[10px] pt-0.5 opacity-70 group-hover:opacity-100 transition-opacity">{formatLogTime(entry.time)}</span>
                
                <span className={`shrink-0 px-2 py-0.5 rounded-md text-[10px] font-bold tracking-wide uppercase ${sourceColor(entry.source)}`}>
                  {sourceLabel(entry.source)}
                </span>
                
                <div className="flex-1 min-w-0">
                  <p className={`break-all font-medium leading-relaxed ${entry.source === 'openclaw' ? 'text-violet-700 dark:text-violet-300' : 'text-gray-700 dark:text-gray-300'}`}>
                    {entry.summary}
                  </p>
                </div>

                {entry.detail ? (
                  <ChevronDown size={14} className={`shrink-0 text-gray-300 transition-transform duration-200 ${expandedId === entry.id ? 'rotate-180 text-gray-500' : ''}`} />
                ) : <span className="w-3.5" />}
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

function formatLogTime(ts: number) {
  const d = new Date(ts);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

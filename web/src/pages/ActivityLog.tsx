import { useEffect, useState, useRef } from 'react';
import { api } from '../lib/api';
import {
  Search, ChevronDown, ChevronRight, Trash2, ArrowDown, RefreshCw,
  Download, Filter,
} from 'lucide-react';
import type { LogEntry } from '../hooks/useWebSocket';

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
    <div className="h-full flex flex-col">
      <div className="flex items-center justify-between mb-4 shrink-0">
        <div>
          <h2 className="text-lg font-bold">活动日志</h2>
          <p className="text-xs text-gray-500 mt-0.5">实时查看所有通道的消息收发记录</p>
        </div>
        <div className="flex items-center gap-1.5">
          <button onClick={handleExport} className="flex items-center gap-1.5 px-2.5 py-1.5 text-xs rounded-lg border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800 text-gray-600 dark:text-gray-400">
            <Download size={13} />导出
          </button>
        </div>
      </div>

      <div className="card flex-1 flex flex-col min-h-0">
        {/* Toolbar */}
        <div className="flex items-center justify-between px-4 pt-3 pb-2 shrink-0">
          <div className="flex items-center gap-3">
            <span className="text-[11px] text-gray-400 tabular-nums">{filteredLog.length} 条记录</span>
          </div>
          <div className="flex items-center gap-1.5">
            <button onClick={() => setAutoScroll(!autoScroll)}
              className={`p-1.5 rounded transition-colors ${autoScroll ? 'bg-violet-100 dark:bg-violet-900 text-violet-600' : 'text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800'}`}
              title={autoScroll ? '自动滚动: 开' : '自动滚动: 关'}>
              <ArrowDown size={13} />
            </button>
            <button onClick={ws.refreshLog} className="p-1.5 rounded hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-400" title="刷新">
              <RefreshCw size={13} />
            </button>
            <button onClick={ws.clearEvents} className="p-1.5 rounded hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-400" title="清空">
              <Trash2 size={13} />
            </button>
          </div>
        </div>

        {/* Filters */}
        <div className="flex items-center gap-2 px-4 pb-2 shrink-0 flex-wrap">
          <div className="relative flex-1 max-w-xs min-w-[180px]">
            <Search size={13} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-gray-400" />
            <input value={search} onChange={e => setSearch(e.target.value)} placeholder="搜索日志内容..."
              className="w-full pl-8 pr-3 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-transparent" />
          </div>
          {/* Source filter */}
          <div className="flex gap-1">
            {[
              { key: '', label: '全部', count: ws.logEntries.length },
              { key: 'qq', label: 'QQ', count: sourceCounts['qq'] || 0 },
              { key: 'openclaw', label: 'Bot回复', count: sourceCounts['openclaw'] || 0 },
              { key: 'wechat', label: '微信', count: sourceCounts['wechat'] || 0 },
              { key: 'system', label: '系统', count: sourceCounts['system'] || 0 },
            ].map(f => (
              <button key={f.key} onClick={() => setSourceFilter(f.key)}
                className={`px-2 py-1 text-[10px] rounded-md transition-colors whitespace-nowrap ${sourceFilter === f.key ? 'bg-violet-100 dark:bg-violet-900 text-violet-700 dark:text-violet-300 font-medium' : 'text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800'}`}>
                {f.label}{f.count > 0 ? ` (${f.count})` : ''}
              </button>
            ))}
          </div>
          {/* Type filter */}
          <div className="flex gap-1 ml-1">
            <Filter size={12} className="text-gray-400 mt-0.5" />
            {[
              { key: '', label: '全部类型' },
              { key: 'text', label: '文本' },
              { key: 'media', label: '媒体' },
              { key: 'sticker', label: '表情' },
            ].map(f => (
              <button key={f.key} onClick={() => setTypeFilter(f.key)}
                className={`px-2 py-1 text-[10px] rounded-md transition-colors whitespace-nowrap ${typeFilter === f.key ? 'bg-amber-100 dark:bg-amber-900 text-amber-700 dark:text-amber-300 font-medium' : 'text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800'}`}>
                {f.label}
              </button>
            ))}
          </div>
        </div>

        {/* Log entries */}
        <div ref={logRef} className="flex-1 overflow-y-auto min-h-0 px-2 pb-2">
          {filteredLog.length === 0 && <p className="text-gray-400 py-8 text-center text-xs">暂无日志</p>}
          {filteredLog.slice(0, 500).map((entry) => (
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

function formatLogTime(ts: number) {
  const d = new Date(ts);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

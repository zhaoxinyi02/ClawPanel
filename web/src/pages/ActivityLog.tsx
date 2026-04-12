import { memo, useEffect, useMemo, useRef, useState } from 'react';
import { useOutletContext } from 'react-router-dom';
import { Search, Trash2, RefreshCw, Download, MessageSquare, Bell, Bot, User, Sparkles } from 'lucide-react';
import { api } from '../lib/api';
import type { LogEntry } from '../hooks/useWebSocket';
import { useI18n } from '../i18n';

interface Props {
  logEntries: LogEntry[];
  clearEvents: () => void;
  refreshLog: () => void;
}

interface SessionItem {
  agentId?: string;
  key: string;
  sessionId: string;
  chatType?: string;
  lastChannel?: string;
  lastTo?: string;
  updatedAt: number;
  originLabel?: string;
  originProvider?: string;
  originFrom?: string;
  messageCount?: number;
}

interface SessionMessage {
  id?: string;
  role?: string;
  content?: string;
  timestamp?: string;
}

function ActivityLogPage({ logEntries, clearEvents, refreshLog }: Props) {
  const { t } = useI18n();
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const modern = uiMode === 'modern';
  const [search, setSearch] = useState('');
  const [sourceFilter, setSourceFilter] = useState('');
  const [sessions, setSessions] = useState<SessionItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedKey, setSelectedKey] = useState('');
  const [detailLoading, setDetailLoading] = useState(false);
  const [messages, setMessages] = useState<SessionMessage[]>([]);
  const [showLowValueSources, setShowLowValueSources] = useState(false);
  const messagePaneRef = useRef<HTMLDivElement | null>(null);
  const shouldStickToBottomRef = useRef(true);

  const systemEntries = useMemo(
    () => logEntries.filter(entry => entry.source === 'system' || entry.source === 'workflow').slice(0, 24),
    [logEntries],
  );

  const loadSessions = async () => {
    setLoading(true);
    try {
      const r = await api.getSessions('all');
      if (r.ok && Array.isArray(r.sessions)) {
        const deduped = dedupeSessions(r.sessions as SessionItem[]);
        setSessions(deduped);
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadSessions();
  }, []);

  const searchedSessions = useMemo(() => sessions.filter((session) => {
    const channel = normalizeChannel(session);
    if (!search.trim()) return true;
    const q = search.toLowerCase();
    return [
      session.originLabel,
      session.originFrom,
      session.lastTo,
      session.sessionId,
      session.agentId,
      session.key,
      sourceLabel(channel),
    ].some(value => String(value || '').toLowerCase().includes(q));
  }), [sessions, search]);

  const filteredSessions = useMemo(() => searchedSessions.filter((session) => {
    const channel = normalizeChannel(session);
    if (!sourceFilter) return true;
    return channel === sourceFilter;
  }), [searchedSessions, sourceFilter]);

  useEffect(() => {
    if (!filteredSessions.length) {
      setSelectedKey('');
      setMessages([]);
      return;
    }
    if (!selectedKey || !filteredSessions.some(item => sessionIdentity(item) === selectedKey)) {
      setSelectedKey(sessionIdentity(filteredSessions[0]));
    }
  }, [filteredSessions, selectedKey]);

  const selectedSession = filteredSessions.find(item => sessionIdentity(item) === selectedKey) || null;

  useEffect(() => {
    if (!selectedSession) {
      setMessages([]);
      return;
    }
    let cancelled = false;
    const loadDetail = async () => {
      setDetailLoading(true);
      try {
        const r = await api.getSessionDetail(selectedSession.sessionId, selectedSession.agentId || 'main');
        if (!cancelled && r.ok) {
          setMessages(Array.isArray(r.messages) ? r.messages : []);
        }
      } finally {
        if (!cancelled) setDetailLoading(false);
      }
    };
    void loadDetail();
    return () => {
      cancelled = true;
    };
  }, [selectedSession]);

  useEffect(() => {
    const el = messagePaneRef.current;
    if (!el) return;
    if (shouldStickToBottomRef.current) {
      el.scrollTop = el.scrollHeight;
    }
  }, [messages, selectedSession]);

  const rawChannelCounts = useMemo(() => searchedSessions.reduce((acc, session) => {
    const channel = normalizeChannel(session);
    acc[channel] = (acc[channel] || 0) + 1;
    return acc;
  }, {} as Record<string, number>), [searchedSessions]);
  const allSourceFilters = useMemo(() => Object.keys(rawChannelCounts).sort(), [rawChannelCounts]);
  const highValueSourceFilters = useMemo(
    () => allSourceFilters.filter(channel => !isLowValueSource(channel)),
    [allSourceFilters],
  );
  const lowValueSourceFilters = useMemo(
    () => allSourceFilters.filter(channel => isLowValueSource(channel)),
    [allSourceFilters],
  );
  const displayChannelCounts = useMemo(
    () => allSourceFilters
      .filter(channel => showLowValueSources || !isLowValueSource(channel))
      .reduce((acc, channel) => {
        acc[channel] = rawChannelCounts[channel] || 0;
        return acc;
      }, {} as Record<string, number>),
    [allSourceFilters, rawChannelCounts, showLowValueSources],
  );
  const channelCounts = displayChannelCounts;

  useEffect(() => {
    if (!sourceFilter) return;
    if (channelCounts[sourceFilter]) return;
    if (isLowValueSource(sourceFilter) && !showLowValueSources) {
      setSourceFilter('');
      return;
    }
    if (!rawChannelCounts[sourceFilter]) {
      setSourceFilter('');
    }
  }, [channelCounts, rawChannelCounts, showLowValueSources, sourceFilter]);

  const messageStats = useMemo(() => messages.reduce((acc, item) => {
    const role = normalizeRole(item.role);
    acc[role] = (acc[role] || 0) + 1;
    return acc;
  }, {} as Record<string, number>), [messages]);

  const handleExport = () => {
    const payload = {
      session: selectedSession,
      messages,
      systemEntries,
    };
    const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `clawpanel-activity-${new Date().toISOString().slice(0, 10)}.json`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className={`h-full flex flex-col space-y-4 ${modern ? 'page-modern' : ''}`}>
      <div className={`${modern ? 'page-modern-header shrink-0' : 'flex items-center justify-between shrink-0'}`}>
        <div>
          <h2 className={`${modern ? 'page-modern-title' : 'text-xl font-bold text-gray-900 dark:text-white tracking-tight'}`}>{t.activityLog.title}</h2>
          <p className={`${modern ? 'page-modern-subtitle' : 'text-sm text-gray-500 mt-1'}`}>直接读取 OpenClaw 会话，统一展示各通道的用户消息、智能体回复和系统事件。</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={handleExport} className={`${modern ? 'page-modern-accent' : 'flex items-center gap-2 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700'} `}>
            <Download size={14} />导出当前视图
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-3 shrink-0">
        {[
          { label: '会话数', value: filteredSessions.length, icon: MessageSquare },
          { label: '通道数', value: Object.keys(channelCounts).length, icon: Sparkles },
          { label: '当前用户消息', value: messageStats.user || 0, icon: User },
          { label: '当前智能体回复', value: messageStats.assistant || 0, icon: Bot },
        ].map(card => (
          <div key={card.label} className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/60 rounded-2xl'} px-4 py-4`}>
            <div className="flex items-center justify-between">
              <div>
                <div className="text-xs text-gray-500 dark:text-gray-400">{card.label}</div>
                <div className="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{card.value}</div>
              </div>
              <card.icon size={18} className="text-sky-500" />
            </div>
          </div>
        ))}
      </div>

      <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 rounded-2xl shadow-sm border border-gray-100 dark:border-gray-700/50'} flex-1 flex flex-col min-h-0 overflow-hidden`}>
        <div className="px-5 py-4 border-b border-gray-100 dark:border-gray-800 bg-white/90 dark:bg-slate-950/50 shrink-0 space-y-3">
          <div className="flex flex-wrap items-center gap-2 justify-between">
            <div className="relative flex-1 min-w-[220px] max-w-md">
              <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input value={search} onChange={e => setSearch(e.target.value)} placeholder="搜索通道、用户、会话或智能体..." className="w-full pl-9 pr-4 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-sky-500/20 focus:border-sky-500 transition-all" />
            </div>
            <div className="flex items-center gap-2">
              {lowValueSourceFilters.length > 0 && (
                <button
                  onClick={() => setShowLowValueSources(prev => !prev)}
                  className="px-2.5 py-1.5 rounded-lg text-[11px] border border-gray-200 dark:border-gray-700 text-gray-500 hover:text-gray-700 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700/50 transition-colors"
                >
                  {showLowValueSources ? '收起低价值分类' : `展开低价值分类 (${lowValueSourceFilters.length})`}
                </button>
              )}
              <button onClick={() => { void loadSessions(); refreshLog(); }} className="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-400 transition-colors" title={t.common.refresh}><RefreshCw size={14} /></button>
              <button onClick={clearEvents} className="p-2 rounded-lg hover:bg-red-50 dark:hover:bg-red-900/20 text-gray-400 hover:text-red-500 transition-colors" title={t.activityLog.clear}><Trash2 size={14} /></button>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2 overflow-x-auto pb-1">
            {[{ key: '', label: '全部会话' }, ...Object.keys(channelCounts).sort().map(key => ({ key, label: sourceLabel(key) }))].map(item => (
              <button key={item.key} onClick={() => setSourceFilter(item.key)} className={`px-3 py-1.5 text-xs rounded-lg transition-all whitespace-nowrap flex items-center gap-1.5 ${sourceFilter === item.key ? 'bg-sky-100 dark:bg-sky-900/40 text-sky-700 dark:text-sky-300 font-semibold shadow-sm' : 'text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700/50 hover:text-gray-700 dark:hover:text-gray-300'}`}>
                {item.label}
                <span className={`px-1.5 py-0.5 rounded-full text-[10px] ${sourceFilter === item.key ? 'bg-white/50 text-sky-800' : 'bg-gray-100 dark:bg-gray-700 text-gray-500'}`}>{item.key ? channelCounts[item.key] || 0 : searchedSessions.length}</span>
              </button>
            ))}
          </div>
        </div>

        <div className="flex-1 min-h-0 grid grid-cols-1 xl:grid-cols-[340px_minmax(0,1fr)_320px]">
          <div className="border-r border-gray-100 dark:border-gray-800 min-h-0 overflow-y-auto p-3 space-y-2 bg-slate-50/70 dark:bg-slate-950/30">
            {loading && filteredSessions.length === 0 ? (
              <div className="h-full flex items-center justify-center text-sm text-gray-400">正在读取会话...</div>
            ) : filteredSessions.length === 0 ? (
              <div className="h-full flex flex-col items-center justify-center text-gray-400 gap-3 py-16">
                <MessageSquare size={24} className="opacity-20" />
                <p className="text-sm">没有匹配的会话</p>
              </div>
            ) : filteredSessions.map(session => {
              const active = sessionIdentity(session) === selectedKey;
              const channel = normalizeChannel(session);
              return (
                <button key={sessionIdentity(session)} onClick={() => setSelectedKey(sessionIdentity(session))} className={`w-full text-left rounded-2xl border px-3 py-3 transition-all ${active ? 'border-sky-200 bg-white shadow-sm dark:border-sky-800 dark:bg-slate-900/80' : 'border-transparent bg-white/80 hover:border-slate-200 hover:bg-white dark:bg-slate-900/30 dark:hover:border-slate-800 dark:hover:bg-slate-900/55'}`}>
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className={`rounded-md px-2 py-0.5 text-[10px] font-bold uppercase ${sourceColor(channel)}`}>{sourceLabel(channel)}</span>
                        {session.agentId && <span className="text-[10px] text-slate-400">{session.agentId}</span>}
                      </div>
                      <div className="mt-2 truncate text-sm font-semibold text-slate-900 dark:text-white">{sessionTitle(session)}</div>
                      <div className="mt-1 line-clamp-2 text-xs leading-5 text-slate-500 dark:text-slate-400">{session.lastTo || session.originFrom || session.sessionId}</div>
                    </div>
                    <span className="shrink-0 text-[11px] text-gray-400">{formatLogTime(session.updatedAt)}</span>
                  </div>
                  <div className="mt-2 flex flex-wrap gap-2 text-[10px] text-slate-400">
                    <span>{session.chatType === 'group' ? '群聊' : session.chatType === 'direct' ? '私聊' : '普通会话'}</span>
                    <span>{session.messageCount || 0} 条</span>
                  </div>
                </button>
              );
            })}
          </div>

          <div className="min-h-0 flex flex-col bg-white dark:bg-slate-950 border-r border-gray-100 dark:border-gray-800">
            {!selectedSession ? (
              <div className="h-full flex items-center justify-center text-sm text-gray-400">选择左侧会话查看消息流</div>
            ) : (
              <>
                <div className="border-b border-gray-100 dark:border-gray-800 px-6 py-4 bg-white dark:bg-slate-950 shrink-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className={`rounded-md px-2 py-0.5 text-[10px] font-bold uppercase ${sourceColor(normalizeChannel(selectedSession))}`}>{sourceLabel(normalizeChannel(selectedSession))}</span>
                    <span className="text-sm font-semibold text-slate-900 dark:text-white">{sessionTitle(selectedSession)}</span>
                    <span className="text-xs text-slate-400">最近更新 {formatLogTime(selectedSession.updatedAt)}</span>
                  </div>
                  <div className="mt-2 flex flex-wrap gap-3 text-[11px] text-slate-500 dark:text-slate-400">
                    <span>智能体 {selectedSession.agentId || 'main'}</span>
                    <span>会话类型 {selectedSession.chatType === 'group' ? '群聊' : selectedSession.chatType === 'direct' ? '私聊' : '普通'}</span>
                    <span>通道目标 {shortMeta(selectedSession.lastTo || '-')}</span>
                  </div>
                </div>

                <div
                  ref={messagePaneRef}
                  onScroll={(e) => {
                    const el = e.currentTarget;
                    shouldStickToBottomRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 80;
                  }}
                  className="flex-1 overflow-y-auto px-6 py-5 space-y-3"
                >
                  {detailLoading ? (
                    <div className="h-full flex items-center justify-center text-sm text-gray-400">正在读取消息...</div>
                  ) : messages.length === 0 ? (
                    <div className="h-full flex items-center justify-center text-sm text-gray-400">暂无用户或智能体消息</div>
                  ) : messages.map((entry, idx) => {
                    const role = normalizeRole(entry.role);
                    const isUser = role === 'user';
                    return (
                      <div key={entry.id || idx} className={`flex ${isUser ? 'justify-end' : 'justify-start'}`}>
                        <div className={`max-w-[78%] rounded-2xl border px-4 py-3 ${messageTone(role)}`}>
                          <div className="mb-1 flex flex-wrap items-center gap-2 text-[10px] opacity-75">
                            <span className="font-semibold">{roleLabel(role)}</span>
                            <span>{formatTimestamp(entry.timestamp)}</span>
                          </div>
                          <div className="whitespace-pre-wrap break-all text-sm leading-6">{entry.content || '空消息'}</div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </>
            )}
          </div>

          <div className="min-h-0 overflow-y-auto p-4 bg-slate-50/70 dark:bg-slate-950/30 space-y-3">
            <div className="rounded-2xl border border-gray-100 dark:border-gray-800 bg-white dark:bg-slate-900 px-4 py-4">
              <div className="flex items-center gap-2 text-sm font-semibold text-slate-900 dark:text-white">
                <Bell size={15} className="text-amber-500" />系统事件
              </div>
              <div className="mt-3 space-y-2">
                {systemEntries.length === 0 ? (
                  <div className="text-xs text-gray-400">暂无系统事件</div>
                ) : systemEntries.map(entry => (
                  <div key={entry.id} className="rounded-xl border border-gray-100 dark:border-gray-800 bg-slate-50/80 dark:bg-slate-950 px-3 py-3">
                    <div className="flex items-center justify-between gap-2">
                      <span className={`rounded-md px-2 py-0.5 text-[10px] font-bold ${sourceColor(entry.source)}`}>{sourceLabel(entry.source)}</span>
                      <span className="text-[10px] text-gray-400">{formatLogTime(entry.time)}</span>
                    </div>
                    <div className="mt-2 text-xs leading-5 text-slate-700 dark:text-slate-300">{entry.summary}</div>
                  </div>
                ))}
              </div>
            </div>

            {selectedSession && (
              <div className="rounded-2xl border border-gray-100 dark:border-gray-800 bg-white dark:bg-slate-900 px-4 py-4 space-y-2 text-xs text-slate-500 dark:text-slate-400">
                <div className="font-semibold text-slate-900 dark:text-white">当前会话摘要</div>
                <div>来源标签：{selectedSession.originLabel || '未记录'}</div>
                <div>来源 ID：{selectedSession.originFrom || '未记录'}</div>
                <div>投递目标：{selectedSession.lastTo || '未记录'}</div>
                <div>最后通道：{normalizeChannel(selectedSession)}</div>
                <div>消息数量：{selectedSession.messageCount || messages.length || 0}</div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

function dedupeSessions(items: SessionItem[]) {
  const byIdentity = new Map<string, SessionItem>();
  items.forEach((item) => {
    const key = sessionIdentity(item);
    const current = byIdentity.get(key);
    if (!current || (item.updatedAt || 0) > (current.updatedAt || 0)) {
      byIdentity.set(key, item);
    }
  });
  return Array.from(byIdentity.values()).sort((a, b) => (b.updatedAt || 0) - (a.updatedAt || 0));
}

function sessionIdentity(session: SessionItem) {
  return `${session.agentId || 'main'}::${session.sessionId}`;
}

function normalizeChannel(session: SessionItem) {
  const raw = String(session.lastChannel || session.originProvider || '').trim().toLowerCase();
  if (!raw) return 'agent';
  if (raw === 'openclaw-weixin' || raw === 'weixin') return 'wechat';
  if (raw === 'qqbot-community') return 'qqbot';
  if (raw === 'heartbeat' || raw.startsWith('heartbeat:')) return 'heartbeat';
  if (raw === 'line') return 'line';
  return raw;
}

function isLowValueSource(source: string) {
  return source === 'agent' || source === 'heartbeat';
}

function sessionTitle(session: SessionItem) {
  return session.originLabel || session.originFrom || session.lastTo || session.sessionId;
}

function normalizeRole(role?: string) {
  if (role === 'assistant') return 'assistant';
  if (role === 'system') return 'system';
  return 'user';
}

function sourceColor(source: string) {
  switch (source) {
    case 'qq': return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300';
    case 'qqbot': return 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300';
    case 'wecom': return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300';
    case 'feishu': return 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300';
    case 'telegram': return 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300';
    case 'discord': return 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-300';
    case 'slack': return 'bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-300';
    case 'line': return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300';
    case 'matrix': return 'bg-lime-100 text-lime-700 dark:bg-lime-900/30 dark:text-lime-300';
    case 'mattermost': return 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300';
    case 'msteams': return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300';
    case 'twitch': return 'bg-fuchsia-100 text-fuchsia-700 dark:bg-fuchsia-900/30 dark:text-fuchsia-300';
    case 'whatsapp': return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300';
    case 'bluebubbles': return 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300';
    case 'imessage': return 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300';
    case 'googlechat': return 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300';
    case 'nextcloud-talk': return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300';
    case 'nostr': return 'bg-stone-100 text-stone-700 dark:bg-stone-900/30 dark:text-stone-300';
    case 'qa-channel': return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300';
    case 'synology-chat': return 'bg-teal-100 text-teal-700 dark:bg-teal-900/30 dark:text-teal-300';
    case 'tlon': return 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300';
    case 'voice-call': return 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300';
    case 'zalo': return 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300';
    case 'zalouser': return 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300';
    case 'wechat': return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300';
    case 'signal': return 'bg-gray-200 text-gray-700 dark:bg-gray-800 dark:text-gray-300';
    case 'heartbeat': return 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300';
    case 'system': return 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300';
    case 'workflow': return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300';
    case 'agent': return 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300';
    default: return 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-300';
  }
}

function sourceLabel(source: string) {
  switch (source) {
    case 'qq': return 'QQ NapCat';
    case 'qqbot': return 'QQ 机器人';
    case 'wecom': return '企业微信';
    case 'feishu': return '飞书';
    case 'telegram': return 'Telegram';
    case 'discord': return 'Discord';
    case 'slack': return 'Slack';
    case 'line': return 'LINE';
    case 'matrix': return 'Matrix';
    case 'mattermost': return 'Mattermost';
    case 'msteams': return 'Microsoft Teams';
    case 'twitch': return 'Twitch';
    case 'whatsapp': return 'WhatsApp';
    case 'bluebubbles': return 'BlueBubbles';
    case 'imessage': return 'iMessage';
    case 'googlechat': return 'Google Chat';
    case 'nextcloud-talk': return 'Nextcloud Talk';
    case 'nostr': return 'Nostr';
    case 'qa-channel': return 'QA Channel';
    case 'synology-chat': return 'Synology Chat';
    case 'tlon': return 'Tlon';
    case 'voice-call': return 'Voice Call';
    case 'zalo': return 'Zalo';
    case 'zalouser': return 'Zalo User';
    case 'wechat': return '微信';
    case 'signal': return 'Signal';
    case 'heartbeat': return 'Heartbeat';
    case 'workflow': return '工作流';
    case 'system': return '系统';
    case 'agent': return '普通会话';
    default: return source || '未知';
  }
}

function roleLabel(role: string) {
  if (role === 'assistant') return '智能体';
  if (role === 'system') return '系统';
  return '用户';
}

function messageTone(role: string) {
  if (role === 'user') return 'border-blue-100 bg-blue-50 text-blue-950 dark:border-blue-900/30 dark:bg-blue-950/20 dark:text-blue-50';
  if (role === 'assistant') return 'border-violet-100 bg-white text-slate-900 dark:border-violet-900/30 dark:bg-slate-900 dark:text-slate-50';
  return 'border-amber-100 bg-amber-50 text-amber-950 dark:border-amber-900/30 dark:bg-amber-950/20 dark:text-amber-50';
}

function shortMeta(value: string) {
  if (!value) return '-';
  return value.length > 28 ? `${value.slice(0, 28)}...` : value;
}

function formatTimestamp(ts?: string) {
  if (!ts) return '-';
  const d = new Date(ts);
  if (Number.isNaN(d.getTime())) return ts;
  return d.toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour: '2-digit', minute: '2-digit', second: '2-digit', month: '2-digit', day: '2-digit' });
}

function formatLogTime(ts: number) {
  if (!ts) return '-';
  const d = new Date(ts);
  const parts = new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).formatToParts(d);
  const get = (type: string) => parts.find((part) => part.type === type)?.value || '00';
  return `${get('month')}-${get('day')} ${get('hour')}:${get('minute')}`;
}

export default memo(ActivityLogPage);

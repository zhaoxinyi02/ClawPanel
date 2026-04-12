import { memo, useEffect, useRef, useState } from 'react';
import { useOutletContext } from 'react-router-dom';
import { api } from '../lib/api';
import { MessageSquare, Trash2, ChevronLeft, Clock, User, Bot, Loader2, RefreshCw, Search, Hash, CircleAlert } from 'lucide-react';
import MobileActionTray from '../components/MobileActionTray';

interface SessionInfo {
  agentId?: string;
  key: string;
  sessionId: string;
  chatType: string;
  lastChannel: string;
  lastTo: string;
  updatedAt: number;
  originLabel: string;
  originProvider: string;
  originFrom: string;
  messageCount: number;
}

interface ChatMessage {
  id: string;
  role: string;
  content: string;
  timestamp: string;
}

interface UsageTotals {
  input: number;
  output: number;
  cacheRead: number;
  cacheWrite: number;
  totalTokens: number;
  totalCost: number;
  requests: number;
  sessions: number;
}

interface UsageSummary {
  today: UsageTotals;
  last7d: UsageTotals;
  last30d: UsageTotals;
}

interface AgentUsageSummary {
  agentId: string;
  today: UsageTotals;
  last7d: UsageTotals;
  last30d: UsageTotals;
}

type MessageSide = 'user' | 'assistant';

function getMessageSide(role?: string): MessageSide {
  const r = (role || '').toLowerCase();
  if (r === 'user' || r === 'human') return 'user';
  return 'assistant';
}

function formatSessionRequestError(error?: string, fallback = '操作失败，请稍后重试'): string {
  if (error === 'ambiguous session') return '会话标识存在歧义，请先切换到具体智能体后再试。';
  if (error === 'not found') return '目标会话不存在或已被删除，请刷新列表后重试。';
  return error ? `请求失败：${error}` : fallback;
}

function SessionsPage() {
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const modern = uiMode === 'modern';
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [agentOptions, setAgentOptions] = useState<string[]>([]);
  const [selectedAgent, setSelectedAgent] = useState('all');
  const [defaultAgent, setDefaultAgent] = useState('main');
  const [loading, setLoading] = useState(true);
  const [listError, setListError] = useState('');
  const [selected, setSelected] = useState<SessionInfo | null>(null);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [msgLoading, setMsgLoading] = useState(false);
  const [messageError, setMessageError] = useState('');
  const [deleteError, setDeleteError] = useState('');
  const [search, setSearch] = useState('');
  const [usage, setUsage] = useState<UsageSummary | null>(null);
  const [agentUsage, setAgentUsage] = useState<AgentUsageSummary[]>([]);
  const [usageLoading, setUsageLoading] = useState(true);
  const sessionsRequestRef = useRef(0);
  const messageRequestRef = useRef(0);
  const selectedRef = useRef<SessionInfo | null>(null);

  const getSessionIdentity = (session: SessionInfo, selectedAgentValue = selectedAgent) => {
    const effectiveAgent = session.agentId || (selectedAgentValue === 'all' ? defaultAgent : selectedAgentValue) || defaultAgent || 'main';
    const sessionToken = session.sessionId || session.key || session.originFrom || session.lastTo || `${session.lastChannel || 'unknown'}:${session.chatType || 'session'}`;
    return `${effectiveAgent}:${sessionToken}`;
  };

  const invalidateMessageRequest = () => {
    messageRequestRef.current += 1;
  };

  const invalidateSessionsRequest = () => {
    sessionsRequestRef.current += 1;
  };

  const setSelectedSession = (session: SessionInfo | null) => {
    selectedRef.current = session;
    setSelected(session);
  };

  const clearSelectedSession = () => {
    invalidateMessageRequest();
    setSelectedSession(null);
    setMessages([]);
    setMessageError('');
  };

  const loadSessions = async () => {
    const requestId = sessionsRequestRef.current + 1;
    sessionsRequestRef.current = requestId;
    if (!selectedAgent) {
      setSessions([]);
      setLoading(false);
      setListError('');
      clearSelectedSession();
      return;
    }
    setLoading(true);
    setListError('');
    try {
      const r = await api.getSessions(selectedAgent);
      if (sessionsRequestRef.current !== requestId) return;
      if (!r.ok || r.error) {
        setSessions([]);
        clearSelectedSession();
        setListError(formatSessionRequestError(r.error, '加载会话列表失败，请稍后重试。'));
      } else {
        const nextSessions = r.sessions || [];
        setSessions(nextSessions);
        const currentSelected = selectedRef.current;
        if (currentSelected) {
          const selectedIdentity = getSessionIdentity(currentSelected);
          const stillExists = nextSessions.some((session: SessionInfo) => getSessionIdentity(session) === selectedIdentity);
          if (!stillExists) clearSelectedSession();
        }
      }
    } catch {
      if (sessionsRequestRef.current !== requestId) return;
      setSessions([]);
      clearSelectedSession();
      setListError('加载会话列表失败，请稍后重试。');
    }
    finally {
      if (sessionsRequestRef.current === requestId) setLoading(false);
    }
  };

  const loadUsage = async () => {
    if (!selectedAgent) {
      setUsage(null);
      setAgentUsage([]);
      setUsageLoading(false);
      return;
    }
    setUsageLoading(true);
    try {
      const r = await api.getSessionUsage(selectedAgent);
      if (r.ok) {
        setUsage(r.summary || null);
        setAgentUsage(r.agents || []);
      }
    } catch {
      setUsage(null);
      setAgentUsage([]);
    } finally {
      setUsageLoading(false);
    }
  };

  const loadAgents = async () => {
    try {
      const r = await api.getAgentsConfig();
      if (r.ok) {
        const list = (r.agents?.list || []).map((x: any) => String(x.id || '').trim()).filter(Boolean);
        const configuredDefaultRaw = String(r.agents?.default || '').trim();
        const fallbackDefault = list[0] || 'main';
        const effectiveDefault = configuredDefaultRaw && list.includes(configuredDefaultRaw) ? configuredDefaultRaw : fallbackDefault;
        const uniq = Array.from(new Set<string>(list.length > 0 ? list : [effectiveDefault]));
        setAgentOptions(uniq);
        setDefaultAgent(effectiveDefault);
        setSelectedAgent(prev => {
          if (prev === 'all') return prev;
          if (prev && uniq.includes(prev)) return prev;
          return effectiveDefault;
        });
      } else {
        setAgentOptions(['main']);
        setDefaultAgent('main');
        setSelectedAgent(prev => prev || 'main');
      }
    } catch {
      setAgentOptions(['main']);
      setDefaultAgent('main');
      setSelectedAgent(prev => prev || 'main');
    }
  };

  useEffect(() => {
    loadAgents();
  }, []);

  useEffect(() => {
    loadSessions();
    loadUsage();
    clearSelectedSession();
    setDeleteError('');
  }, [selectedAgent]);

  const handleRefresh = () => {
    loadSessions();
    loadUsage();
  };

  const loadMessages = async (s: SessionInfo) => {
    const requestId = messageRequestRef.current + 1;
    messageRequestRef.current = requestId;
    setSelectedSession(s);
    setMsgLoading(true);
    setMessages([]);
    setMessageError('');
    try {
      const targetAgent = s.agentId || (selectedAgent === 'all' ? defaultAgent : selectedAgent);
      const r = await api.getSessionDetail(s.sessionId, targetAgent);
      if (messageRequestRef.current !== requestId) return;
      if (!r.ok || r.error) {
        setMessageError(formatSessionRequestError(r.error, '加载消息失败，请稍后重试。'));
      } else {
        setMessages(r.messages || []);
      }
    } catch {
      if (messageRequestRef.current !== requestId) return;
      setMessageError('加载消息失败，请稍后重试。');
    }
    finally {
      if (messageRequestRef.current === requestId) setMsgLoading(false);
    }
  };

  const handleDelete = async (s: SessionInfo) => {
    if (!confirm(`确定删除会话 "${s.originLabel || s.key}"？此操作不可恢复。`)) return;
    setDeleteError('');
    try {
      const targetAgent = s.agentId || (selectedAgent === 'all' ? defaultAgent : selectedAgent);
      const r = await api.deleteSession(s.sessionId, targetAgent);
      if (r.ok) {
        invalidateSessionsRequest();
        const targetIdentity = getSessionIdentity(s);
        setSessions(prev => prev.filter(x => getSessionIdentity(x) !== targetIdentity));
        if (selectedRef.current && getSessionIdentity(selectedRef.current) === targetIdentity) {
          clearSelectedSession();
        }
        loadUsage();
      } else {
        setDeleteError(formatSessionRequestError(r.error, '删除会话失败，请稍后重试。'));
      }
    } catch {
      setDeleteError('删除会话失败，请稍后重试。');
    }
  };

  const formatTime = (ms: number) => {
    if (!ms) return '-';
    const d = new Date(ms);
    const now = new Date();
    const diff = now.getTime() - ms;
    if (diff < 60000) return '刚刚';
    if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`;
    if (d.toDateString() === new Date(now.getTime() - 86400000).toDateString()) return '昨天';
    return `${d.getMonth() + 1}/${d.getDate()} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
  };

  const formatTokens = (value: number) => {
    if (!value) return '0';
    if (value >= 1000000) return `${(value / 1000000).toFixed(1)}M`;
    if (value >= 1000) return `${(value / 1000).toFixed(1)}k`;
    return String(value);
  };

  const formatCost = (value: number) => {
    if (!value) return '$0';
    return `$${value.toFixed(value >= 1 ? 2 : 4)}`;
  };

  const channelBadge = (ch: string) => {
    const colors: Record<string, string> = {
      qq: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
      wechat: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400',
    };
    return colors[ch] || 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400';
  };

  const filtered = sessions.filter(s => {
    if (!search) return true;
    const q = search.toLowerCase();
    return (s.originLabel || '').toLowerCase().includes(q) ||
           (s.key || '').toLowerCase().includes(q) ||
           (s.lastChannel || '').toLowerCase().includes(q);
  });

  return (
    <div className={`space-y-4 ${modern ? 'page-modern' : ''}`}>
      <div className={`${modern ? 'page-modern-header' : 'flex items-center justify-between'}`}>
        <div>
          <h2 className={`${modern ? 'page-modern-title text-xl' : 'text-lg font-bold text-gray-900 dark:text-white'}`}>会话管理</h2>
          <p className={`${modern ? 'page-modern-subtitle text-xs mt-0.5' : 'text-xs text-gray-500 mt-0.5'}`}>管理 OpenClaw 的对话会话，查看聊天记录</p>
        </div>
        <MobileActionTray label="会话筛选与操作">
          <select
            value={selectedAgent}
            onChange={e => setSelectedAgent(e.target.value)}
            className={`${modern ? 'page-modern-control min-w-[132px] text-xs font-medium' : 'px-2.5 py-1.5 text-xs rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-600 dark:text-gray-300'}`}
          >
            <option value="all">all</option>
            {agentOptions.map(id => (
              <option key={id} value={id}>{id}</option>
            ))}
          </select>
          <button onClick={handleRefresh} className={`${modern ? 'page-modern-accent px-3.5 py-2 text-xs font-medium' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors'}`}>
            <RefreshCw size={14} /> 刷新
          </button>
        </MobileActionTray>
      </div>

      {deleteError && (
        <div className={`${modern ? 'page-modern-panel border border-red-100/80 dark:border-red-900/40 text-red-600 dark:text-red-300 px-4 py-3 text-xs flex items-start gap-2' : 'rounded-xl border border-red-100 dark:border-red-900/40 bg-red-50/80 dark:bg-red-900/10 text-red-600 dark:text-red-300 px-4 py-3 text-xs flex items-start gap-2'}`}>
          <CircleAlert size={14} className="mt-0.5 shrink-0" />
          <span>{deleteError}</span>
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {([
          { key: 'today' as const, label: '今日用量' },
          { key: 'last7d' as const, label: '近 7 日' },
          { key: 'last30d' as const, label: '近 30 日' },
        ]).map(({ key, label }) => {
          const data = usage?.[key];
          return (
            <div key={key} className={`${modern ? 'page-modern-card' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-xl p-4 shadow-sm`}>
              <div className="flex items-center justify-between">
                <span className="text-xs font-bold uppercase tracking-wider text-gray-500">{label}</span>
                {usageLoading && <Loader2 size={14} className="animate-spin text-gray-400" />}
              </div>
              <div className="mt-3 text-2xl font-bold text-gray-900 dark:text-white">
                {usageLoading || !data ? '--' : formatTokens(data.totalTokens)}
              </div>
              <div className="mt-1 text-[11px] text-gray-500">
                {usageLoading || !data ? '统计中...' : `请求 ${data.requests} · 会话 ${data.sessions} · Cost ${formatCost(data.totalCost)}`}
              </div>
              <div className="mt-2 text-[11px] text-gray-500">
                {usageLoading || !data ? '--' : `输入 ${formatTokens(data.input)} / 输出 ${formatTokens(data.output)} / 缓存 ${formatTokens(data.cacheRead + data.cacheWrite)}`}
              </div>
            </div>
          );
        })}
      </div>

      {agentUsage.length > 1 && (
        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-xl shadow-sm overflow-hidden`}>
          <div className="px-4 py-3 border-b border-gray-100 dark:border-gray-700/50 flex items-center justify-between">
            <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider">Agent 用量分布</h3>
            <span className="text-[10px] text-gray-400 font-medium">{agentUsage.length} 个 agent</span>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead className="bg-gray-50 dark:bg-gray-900/40 text-gray-500">
                <tr>
                  <th className="px-4 py-2 text-left">Agent</th>
                  <th className="px-4 py-2 text-right">今日</th>
                  <th className="px-4 py-2 text-right">近 7 日</th>
                  <th className="px-4 py-2 text-right">近 30 日</th>
                  <th className="px-4 py-2 text-right">近 30 日请求</th>
                </tr>
              </thead>
              <tbody>
                {agentUsage.map(item => (
                  <tr key={item.agentId} className="border-t border-gray-100 dark:border-gray-700/40">
                    <td className="px-4 py-2.5 font-mono text-gray-900 dark:text-gray-100">{item.agentId}</td>
                    <td className="px-4 py-2.5 text-right text-gray-700 dark:text-gray-300">{formatTokens(item.today.totalTokens)}</td>
                    <td className="px-4 py-2.5 text-right text-gray-700 dark:text-gray-300">{formatTokens(item.last7d.totalTokens)}</td>
                    <td className="px-4 py-2.5 text-right text-gray-700 dark:text-gray-300">{formatTokens(item.last30d.totalTokens)}</td>
                    <td className="px-4 py-2.5 text-right text-gray-500">{item.last30d.requests}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Session list */}
        <div className={`${modern ? 'page-modern-panel flex flex-col max-h-[75vh] overflow-hidden' : 'bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 flex flex-col max-h-[75vh] overflow-hidden'}`}>
          <div className="p-3 border-b border-gray-100 dark:border-gray-700/50 space-y-2">
            <div className="flex items-center justify-between">
              <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider px-1">会话列表</h3>
              <span className="text-[10px] text-gray-400 font-medium">{filtered.length} 个会话</span>
            </div>
            <div className={`${modern ? 'relative rounded-xl border border-blue-100/70 dark:border-blue-800/20 bg-[linear-gradient(145deg,rgba(255,255,255,0.76),rgba(239,246,255,0.62))] dark:bg-[linear-gradient(145deg,rgba(10,20,36,0.82),rgba(30,64,175,0.1))] shadow-sm backdrop-blur-xl' : 'relative'}`}>
              <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-gray-400" />
              <input value={search} onChange={e => setSearch(e.target.value)} placeholder="搜索会话..."
                className={`w-full pl-8 pr-3 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900 focus:outline-none transition-all ${modern ? 'border-transparent bg-transparent dark:bg-transparent focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500' : 'focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500'}`} />
            </div>
            {listError && (
              <div className={`${modern ? 'rounded-xl border border-red-100/80 dark:border-red-900/40 text-red-600 dark:text-red-300 px-3 py-2 text-[11px] flex items-start gap-2 bg-red-50/60 dark:bg-red-900/10' : 'rounded-lg border border-red-100 dark:border-red-900/40 bg-red-50/80 dark:bg-red-900/10 text-red-600 dark:text-red-300 px-3 py-2 text-[11px] flex items-start gap-2'}`}>
                <CircleAlert size={13} className="mt-0.5 shrink-0" />
                <span>{listError}</span>
              </div>
            )}
          </div>
          <div className="flex-1 overflow-y-auto p-2 space-y-1">
            {loading ? (
              <div className="flex items-center justify-center py-12 text-gray-400">
                <Loader2 size={20} className="animate-spin" />
              </div>
            ) : filtered.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 text-gray-400">
                <MessageSquare size={24} className="mb-2 opacity-30" />
                <p className="text-xs">{search ? '无匹配会话' : '暂无会话'}</p>
              </div>
            ) : filtered.map(s => {
              const sessionIdentity = getSessionIdentity(s);
              const isSelected = selected ? getSessionIdentity(selected) === sessionIdentity : false;

              return (
              <div key={sessionIdentity} role="button" tabIndex={0} onClick={() => loadMessages(s)} onKeyDown={(event) => {
                if (event.target !== event.currentTarget) return;
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  loadMessages(s);
                }
              }}
                className={`w-full text-left px-3 py-2.5 rounded-xl transition-all duration-200 group border border-transparent ${
                  isSelected
                    ? 'bg-[linear-gradient(145deg,rgba(255,255,255,0.82),rgba(219,234,254,0.68))] dark:bg-[linear-gradient(145deg,rgba(30,64,175,0.2),rgba(10,20,36,0.82))] border-blue-100/80 dark:border-blue-800/40 shadow-sm shadow-blue-100/40 dark:shadow-none'
                    : 'hover:bg-white/70 dark:hover:bg-gray-700/50 hover:border-blue-100/70 dark:hover:border-blue-800/30'
                }`}>
                <div className="flex items-start gap-2.5">
                  <div className={`p-1.5 rounded-xl mt-0.5 shrink-0 border ${isSelected ? 'bg-blue-100/80 dark:bg-blue-900/30 border-blue-100 dark:border-blue-800/40 text-blue-600 dark:text-blue-300' : 'bg-gray-100 dark:bg-gray-700 border-transparent text-gray-500'}`}>
                    <MessageSquare size={14} />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="text-xs font-semibold text-gray-900 dark:text-white truncate">
                        {s.originLabel || s.key}
                      </span>
                      {s.lastChannel && (
                        <span className={`text-[9px] font-bold px-1.5 py-0.5 rounded shrink-0 ${channelBadge(s.lastChannel)}`}>
                          {s.lastChannel.toUpperCase()}
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-2 mt-0.5">
                      {s.agentId && (
                        <span className="text-[10px] px-1.5 py-0.5 rounded bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300 font-mono">
                          {s.agentId}
                        </span>
                      )}
                      <span className="text-[10px] text-gray-400 flex items-center gap-1">
                        <Clock size={10} /> {formatTime(s.updatedAt)}
                      </span>
                      <span className="text-[10px] text-gray-400 flex items-center gap-1">
                        <Hash size={10} /> {s.messageCount} 条
                      </span>
                    </div>
                  </div>
                  <button
                    onClick={e => { e.stopPropagation(); handleDelete(s); }}
                    onKeyDown={e => { e.stopPropagation(); }}
                    aria-label={`删除会话 ${s.originLabel || s.key || s.sessionId}`}
                    className={`${modern ? 'page-modern-danger p-1.5 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 focus-visible:opacity-100 transition-all shrink-0' : 'p-1 rounded opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 focus-visible:opacity-100 text-gray-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 transition-all shrink-0'}`}>
                    <Trash2 size={12} />
                  </button>
                </div>
              </div>
            );
            })}
          </div>
        </div>

        {/* Chat detail */}
        <div className={`${modern ? 'page-modern-panel lg:col-span-2 flex flex-col max-h-[75vh] overflow-hidden' : 'lg:col-span-2 bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 flex flex-col max-h-[75vh] overflow-hidden'}`}>
          {!selected ? (
            <div className="flex-1 flex flex-col items-center justify-center text-gray-400 p-8">
              <MessageSquare size={40} className="mb-3 opacity-20" />
              <p className="text-sm font-medium">选择一个会话查看聊天记录</p>
              <p className="text-xs mt-1 opacity-60">点击左侧会话列表中的任意会话</p>
            </div>
          ) : (
            <>
              <div className={`${modern ? 'px-5 py-3 border-b border-blue-100/60 dark:border-slate-700/50 flex items-center gap-3 bg-[linear-gradient(145deg,rgba(255,255,255,0.34),rgba(239,246,255,0.22))] dark:bg-[linear-gradient(145deg,rgba(10,20,36,0.36),rgba(30,64,175,0.08))] backdrop-blur-xl shrink-0' : 'px-5 py-3 border-b border-gray-100 dark:border-gray-700/50 flex items-center gap-3 bg-gray-50/50 dark:bg-gray-800/50 shrink-0'}`}>
                <button onClick={() => { clearSelectedSession(); }} className={`${modern ? 'page-modern-action p-1.5 lg:hidden' : 'lg:hidden p-1 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-700'}`}>
                  <ChevronLeft size={18} />
                </button>
                <div className="p-2 rounded-xl border border-blue-100/80 dark:border-blue-800/40 bg-blue-100/80 dark:bg-blue-900/20 text-blue-600 dark:text-blue-300">
                  <MessageSquare size={18} />
                </div>
                <div className="min-w-0 flex-1">
                  <h3 className="text-sm font-bold text-gray-900 dark:text-white truncate">{selected.originLabel || selected.key}</h3>
                  <div className="flex items-center gap-3 text-[10px] text-gray-400 mt-0.5 flex-wrap">
                    {selected.lastChannel && <span className={`font-bold px-1.5 py-0.5 rounded ${channelBadge(selected.lastChannel)}`}>{selected.lastChannel.toUpperCase()}</span>}
                    <span>{selected.chatType === 'direct' ? '私聊' : selected.chatType === 'group' ? '群聊' : selected.chatType || '未知'}</span>
                    <span>{selected.messageCount} 条消息</span>
                    <span>更新于 {formatTime(selected.updatedAt)}</span>
                  </div>
                </div>
              </div>
              <div className="flex-1 overflow-y-auto p-4 space-y-3">
                {msgLoading ? (
                  <div className="flex items-center justify-center py-12 text-gray-400">
                    <Loader2 size={20} className="animate-spin" />
                  </div>
                ) : messageError ? (
                  <div className="flex flex-col items-center justify-center py-12 text-red-500 dark:text-red-300">
                    <CircleAlert size={24} className="mb-2 opacity-80" />
                    <p className="text-xs font-medium">{messageError}</p>
                    <button
                      onClick={() => loadMessages(selected)}
                      className={`${modern ? 'page-modern-action mt-3 px-3 py-1.5 text-xs font-medium' : 'mt-3 px-3 py-1.5 text-xs font-medium rounded-lg border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800 text-gray-600 dark:text-gray-300'}`}
                    >
                      重试
                    </button>
                  </div>
                ) : messages.length === 0 ? (
                  <div className="flex flex-col items-center justify-center py-12 text-gray-400">
                    <MessageSquare size={24} className="mb-2 opacity-30" />
                    <p className="text-xs">暂无消息记录</p>
                  </div>
                ) : messages.map((m, i) => {
                  const side = getMessageSide(m.role);
                  const isUser = side === 'user';
                  return (
                    <div key={m.id || i} className={`flex gap-3 ${isUser ? 'flex-row-reverse' : ''}`}>
                      <div className={`w-8 h-8 rounded-xl flex items-center justify-center shrink-0 border shadow-sm ${
                        isUser ? 'bg-blue-100/80 dark:bg-blue-900/20 border-blue-100 dark:border-blue-800/30 text-blue-600' : 'bg-white/85 dark:bg-slate-800/75 border-blue-100/70 dark:border-slate-700/60 text-blue-500'
                      }`}>
                        {isUser ? <User size={16} /> : <Bot size={16} />}
                      </div>
                      <div className={`max-w-[75%] flex flex-col ${isUser ? 'items-end' : 'items-start'}`}>
                        <div className={`inline-block px-3.5 py-2.5 rounded-2xl text-sm leading-relaxed whitespace-pre-wrap break-words ${
                          isUser
                            ? 'bg-[linear-gradient(135deg,rgba(37,99,235,0.92),rgba(14,165,233,0.8))] text-white rounded-tr-md shadow-sm'
                            : 'bg-white/80 dark:bg-slate-800/70 text-gray-900 dark:text-gray-100 rounded-tl-md border border-blue-100/60 dark:border-slate-700/60 backdrop-blur-xl'
                        }`}>
                          {m.content}
                        </div>
                        {m.timestamp && (
                          <p className={`text-[10px] text-gray-400 mt-1 px-1 ${isUser ? 'text-right' : 'text-left'}`}>
                            {new Date(m.timestamp).toLocaleString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                          </p>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

export default memo(SessionsPage);

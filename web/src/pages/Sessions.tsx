import { useEffect, useState } from 'react';
import { api } from '../lib/api';
import { MessageSquare, Trash2, ChevronLeft, Clock, User, Bot, Loader2, RefreshCw, Search, Hash } from 'lucide-react';

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

export default function Sessions() {
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [agentOptions, setAgentOptions] = useState<string[]>(['main']);
  const [selectedAgent, setSelectedAgent] = useState('main');
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<SessionInfo | null>(null);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [msgLoading, setMsgLoading] = useState(false);
  const [search, setSearch] = useState('');

  const loadSessions = async () => {
    setLoading(true);
    try {
      const r = await api.getSessions(selectedAgent);
      if (r.ok) setSessions(r.sessions || []);
    } catch {}
    finally { setLoading(false); }
  };

  const loadAgents = async () => {
    try {
      const r = await api.getAgentsConfig();
      if (r.ok) {
        const list = (r.agents?.list || []).map((x: any) => x.id).filter(Boolean);
        const uniq = Array.from(new Set(['main', ...list]));
        setAgentOptions(uniq);
      }
    } catch {}
  };

  useEffect(() => {
    loadAgents();
  }, []);

  useEffect(() => {
    loadSessions();
    setSelected(null);
    setMessages([]);
  }, [selectedAgent]);

  const loadMessages = async (s: SessionInfo) => {
    setSelected(s);
    setMsgLoading(true);
    setMessages([]);
    try {
      const targetAgent = s.agentId || selectedAgent;
      const r = await api.getSessionDetail(s.sessionId, targetAgent);
      if (r.ok) setMessages(r.messages || []);
    } catch {}
    finally { setMsgLoading(false); }
  };

  const handleDelete = async (s: SessionInfo) => {
    if (!confirm(`确定删除会话 "${s.originLabel || s.key}"？此操作不可恢复。`)) return;
    try {
      const targetAgent = s.agentId || selectedAgent;
      const r = await api.deleteSession(s.sessionId, targetAgent);
      if (r.ok) {
        setSessions(prev => prev.filter(x => !(x.sessionId === s.sessionId && (x.agentId || selectedAgent) === targetAgent)));
        if (selected?.sessionId === s.sessionId && (selected?.agentId || selectedAgent) === targetAgent) {
          setSelected(null);
          setMessages([]);
        }
      }
    } catch {}
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
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-gray-900 dark:text-white">会话管理</h2>
          <p className="text-xs text-gray-500 mt-0.5">管理 OpenClaw 的对话会话，查看聊天记录</p>
        </div>
        <div className="flex items-center gap-2">
          <select
            value={selectedAgent}
            onChange={e => setSelectedAgent(e.target.value)}
            className="px-2.5 py-1.5 text-xs rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-600 dark:text-gray-300"
          >
            <option value="all">all</option>
            {agentOptions.map(id => (
              <option key={id} value={id}>{id}</option>
            ))}
          </select>
          <button onClick={loadSessions} className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors">
            <RefreshCw size={14} /> 刷新
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Session list */}
        <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 flex flex-col max-h-[75vh] overflow-hidden">
          <div className="p-3 border-b border-gray-100 dark:border-gray-700/50 space-y-2">
            <div className="flex items-center justify-between">
              <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider px-1">会话列表</h3>
              <span className="text-[10px] text-gray-400 font-medium">{filtered.length} 个会话</span>
            </div>
            <div className="relative">
              <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-gray-400" />
              <input value={search} onChange={e => setSearch(e.target.value)} placeholder="搜索会话..."
                className="w-full pl-8 pr-3 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500 transition-all" />
            </div>
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
            ) : filtered.map(s => (
              <button key={s.sessionId} onClick={() => loadMessages(s)}
                className={`w-full text-left px-3 py-2.5 rounded-lg transition-all duration-200 group ${
                  selected?.sessionId === s.sessionId
                    ? 'bg-violet-50 dark:bg-violet-900/20 ring-1 ring-violet-100 dark:ring-violet-800'
                    : 'hover:bg-gray-50 dark:hover:bg-gray-700/50'
                }`}>
                <div className="flex items-start gap-2.5">
                  <div className={`p-1.5 rounded-md mt-0.5 shrink-0 ${selected?.sessionId === s.sessionId ? 'bg-violet-100 dark:bg-violet-900/40 text-violet-600' : 'bg-gray-100 dark:bg-gray-700 text-gray-500'}`}>
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
                        <span className="text-[10px] px-1.5 py-0.5 rounded bg-violet-50 text-violet-700 dark:bg-violet-900/20 dark:text-violet-300 font-mono">
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
                  <button onClick={e => { e.stopPropagation(); handleDelete(s); }}
                    className="p-1 rounded opacity-0 group-hover:opacity-100 text-gray-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 transition-all shrink-0">
                    <Trash2 size={12} />
                  </button>
                </div>
              </button>
            ))}
          </div>
        </div>

        {/* Chat detail */}
        <div className="lg:col-span-2 bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 flex flex-col max-h-[75vh] overflow-hidden">
          {!selected ? (
            <div className="flex-1 flex flex-col items-center justify-center text-gray-400 p-8">
              <MessageSquare size={40} className="mb-3 opacity-20" />
              <p className="text-sm font-medium">选择一个会话查看聊天记录</p>
              <p className="text-xs mt-1 opacity-60">点击左侧会话列表中的任意会话</p>
            </div>
          ) : (
            <>
              <div className="px-5 py-3 border-b border-gray-100 dark:border-gray-700/50 flex items-center gap-3 bg-gray-50/50 dark:bg-gray-800/50 shrink-0">
                <button onClick={() => { setSelected(null); setMessages([]); }} className="lg:hidden p-1 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-700">
                  <ChevronLeft size={18} />
                </button>
                <div className="p-2 rounded-lg bg-violet-100 dark:bg-violet-900/30 text-violet-600">
                  <MessageSquare size={18} />
                </div>
                <div className="min-w-0 flex-1">
                  <h3 className="text-sm font-bold text-gray-900 dark:text-white truncate">{selected.originLabel || selected.key}</h3>
                  <div className="flex items-center gap-3 text-[10px] text-gray-400 mt-0.5">
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
                ) : messages.length === 0 ? (
                  <div className="flex flex-col items-center justify-center py-12 text-gray-400">
                    <MessageSquare size={24} className="mb-2 opacity-30" />
                    <p className="text-xs">暂无消息记录</p>
                  </div>
                ) : messages.map((m, i) => (
                  <div key={m.id || i} className={`flex gap-3 ${m.role === 'assistant' ? '' : 'flex-row-reverse'}`}>
                    <div className={`w-8 h-8 rounded-lg flex items-center justify-center shrink-0 ${
                      m.role === 'assistant' ? 'bg-violet-100 dark:bg-violet-900/30 text-violet-600' : 'bg-blue-100 dark:bg-blue-900/30 text-blue-600'
                    }`}>
                      {m.role === 'assistant' ? <Bot size={16} /> : <User size={16} />}
                    </div>
                    <div className={`max-w-[75%] ${m.role === 'assistant' ? '' : 'text-right'}`}>
                      <div className={`inline-block px-3.5 py-2.5 rounded-2xl text-sm leading-relaxed whitespace-pre-wrap break-words ${
                        m.role === 'assistant'
                          ? 'bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-gray-100 rounded-tl-md'
                          : 'bg-violet-600 text-white rounded-tr-md'
                      }`}>
                        {m.content}
                      </div>
                      {m.timestamp && (
                        <p className={`text-[10px] text-gray-400 mt-1 ${m.role === 'assistant' ? '' : 'text-right'}`}>
                          {new Date(m.timestamp).toLocaleString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                        </p>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

import { useEffect, useState } from 'react';
import { useOutletContext } from 'react-router-dom';
import { api } from '../lib/api';
import { RefreshCw, MessageSquare, Database } from 'lucide-react';
import { useI18n } from '../i18n';

interface HermesSession {
  id: string;
  title: string;
  path: string;
  updatedAt?: string;
  messageCount: number;
  recentMessages?: Array<{ role: string; content: string; timestamp?: string }>;
}

interface HermesUsage {
  inputTokens?: number;
  outputTokens?: number;
  totalTokens?: number;
  rows?: number;
}

export default function HermesSessions() {
  const { locale } = useI18n();
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const modern = uiMode === 'modern';
  const [sessions, setSessions] = useState<HermesSession[]>([]);
  const [selected, setSelected] = useState<HermesSession | null>(null);
  const [messages, setMessages] = useState<Array<{ role: string; content: string; timestamp?: string }>>([]);
  const [usage, setUsage] = useState<HermesUsage | null>(null);
  const [dbInfo, setDbInfo] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [msgLoading, setMsgLoading] = useState(false);
  const [err, setErr] = useState('');

  const load = async () => {
    setLoading(true);
    setErr('');
    try {
      const [sessionsRes, usageRes] = await Promise.all([
        api.getHermesSessions(100),
        api.getHermesUsage(),
      ]);
      if (sessionsRes.ok) {
        const next = sessionsRes.sessions || [];
        setSessions(next);
        setSelected(prev => prev || next[0] || null);
      }
      if (usageRes.ok) {
        setUsage(usageRes.usage || null);
        setDbInfo(usageRes.db || null);
      }
    } catch {
      setErr(locale === 'zh-CN' ? '加载 Hermes 会话失败' : 'Failed to load Hermes sessions');
    } finally {
      setLoading(false);
    }
  };

  const loadDetail = async (id: string) => {
    if (!id) return;
    setMsgLoading(true);
    try {
      const r = await api.getHermesSessionDetail(id, 200);
      if (r.ok) setMessages(r.messages || []);
    } finally {
      setMsgLoading(false);
    }
  };

  useEffect(() => { load(); }, []);
  useEffect(() => { if (selected?.id) loadDetail(selected.id); }, [selected?.id]);

  return (
    <div className={`space-y-6 ${modern ? 'page-modern' : ''}`}>
      <div className={`${modern ? 'page-modern-header' : 'flex items-center justify-between'}`}>
        <div>
          <h2 className={`${modern ? 'page-modern-title text-xl' : 'text-xl font-bold text-gray-900 dark:text-white'}`}>{locale === 'zh-CN' ? 'Hermes 会话' : 'Hermes Sessions'}</h2>
          <p className={`${modern ? 'page-modern-subtitle mt-1 text-sm' : 'text-sm text-gray-500 mt-1'}`}>
            {locale === 'zh-CN'
              ? '查看 Hermes 会话、最近消息以及从 state.db 推导出的 usage 信息。'
              : 'Inspect Hermes sessions, recent messages, and usage inferred from state.db.'}
          </p>
        </div>
        <button onClick={load} className={`${modern ? 'page-modern-action px-3 py-2 text-xs' : 'px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-800'} inline-flex items-center gap-2`}>
          <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          {locale === 'zh-CN' ? '刷新' : 'Refresh'}
        </button>
      </div>

      {err && <div className="rounded-2xl border border-red-100 bg-red-50/80 px-4 py-3 text-sm text-red-700 dark:border-red-900/30 dark:bg-red-900/10 dark:text-red-300">{err}</div>}

      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        {[
          { label: locale === 'zh-CN' ? '会话数' : 'Sessions', value: sessions.length },
          { label: locale === 'zh-CN' ? '输入 Tokens' : 'Input Tokens', value: usage?.inputTokens ?? 0 },
          { label: locale === 'zh-CN' ? '输出 Tokens' : 'Output Tokens', value: usage?.outputTokens ?? 0 },
          { label: locale === 'zh-CN' ? '总 Tokens' : 'Total Tokens', value: usage?.totalTokens ?? 0 },
        ].map(card => (
          <div key={card.label} className={`${modern ? 'page-modern-card' : 'bg-white dark:bg-gray-800'} rounded-2xl border border-gray-100 dark:border-gray-700/50 p-4`}>
            <div className="text-xs font-semibold uppercase tracking-wider text-gray-500">{card.label}</div>
            <div className="mt-3 text-lg font-bold text-gray-900 dark:text-white">{card.value}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[360px_1fr] gap-6">
        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-4 space-y-3`}>
          {sessions.map(session => (
            <button key={session.id} onClick={() => setSelected(session)} className={`w-full text-left rounded-xl border px-4 py-3 transition-colors ${selected?.id === session.id ? 'border-blue-300 bg-blue-50/70 dark:border-blue-700 dark:bg-blue-900/20' : 'border-gray-100 dark:border-gray-700/50 hover:bg-gray-50 dark:hover:bg-gray-900/40'}`}>
              <div className="flex items-start gap-3">
                <MessageSquare size={16} className="mt-0.5 text-blue-500 shrink-0" />
                <div className="min-w-0 flex-1">
                  <div className="font-medium text-gray-900 dark:text-white truncate">{session.title}</div>
                  <div className="mt-1 text-xs text-gray-500 truncate">{session.path}</div>
                  <div className="mt-2 text-[11px] text-gray-400">{session.messageCount} msgs</div>
                </div>
              </div>
            </button>
          ))}
        </div>

        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-5 space-y-4`}>
          <div className="flex items-center gap-2 text-gray-900 dark:text-white font-semibold">
            <Database size={18} className="text-blue-500" />
            {locale === 'zh-CN' ? '会话详情与 DB 摘要' : 'Session Detail & DB Summary'}
          </div>
          {selected ? (
            <>
              <div className="rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 px-4 py-3 text-sm">
                <div className="font-medium text-gray-900 dark:text-white">{selected.title}</div>
                <div className="mt-1 font-mono text-xs text-gray-500 break-all">{selected.path}</div>
              </div>
              <div className="space-y-3">
                {(msgLoading ? [] : messages).map((message, index) => (
                  <div key={`${message.timestamp || index}-${index}`} className="rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 px-4 py-3">
                    <div className="text-[11px] uppercase tracking-wider text-gray-500">{message.role}</div>
                    <div className="mt-1 text-sm text-gray-800 dark:text-gray-100 whitespace-pre-wrap break-words">{message.content}</div>
                  </div>
                ))}
              </div>
              <div className="rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                {dbInfo?.path ? `DB: ${dbInfo.path}` : 'No DB metadata'}
              </div>
              {dbInfo?.tables?.length > 0 && (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  {dbInfo.tables.map((table: any) => (
                    <div key={table.name} className="rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 px-4 py-3 text-xs">
                      <div className="font-medium text-gray-900 dark:text-white">{table.name}</div>
                      <div className="mt-1 text-gray-500">{table.rowCount} rows</div>
                    </div>
                  ))}
                </div>
              )}
              <div className="rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                {(selected.recentMessages || []).length > 0 && !msgLoading ? (
                  <div>{locale === 'zh-CN' ? '会话预览缓存已命中，可与详情消息对照查看。' : 'Session preview cache is available for quick comparison.'}</div>
                ) : (
                  <div>{locale === 'zh-CN' ? '暂无预览缓存' : 'No preview cache'}</div>
                )}
              </div>
            </>
          ) : (
            <div className="text-sm text-gray-500">{locale === 'zh-CN' ? '请选择一个会话' : 'Select a session'}</div>
          )}
        </div>
      </div>
    </div>
  );
}

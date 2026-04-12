import { useEffect, useRef, useState, useCallback } from 'react';
import { api } from '../lib/api';
import { DEMO_LOG_ENTRIES, DEMO_NAPCAT_STATUS, DEMO_WECHAT_STATUS } from '../lib/mockApi';

const IS_DEMO = import.meta.env.VITE_DEMO === 'true';

export interface LogEntry {
  id: string;
  time: number;
  source: 'qq' | 'wechat' | 'wecom' | 'feishu' | 'system' | 'openclaw' | 'workflow';
  type: string;
  summary: string;
  detail?: string;
}

// Convert demo log format to LogEntry format
function demoToLogEntry(d: any): LogEntry {
  return { id: d.id, time: d.time, source: d.source, type: d.type, summary: d.content, detail: d.content };
}

export function useWebSocket() {
  const wsRef = useRef<WebSocket | null>(null);
  const [logEntries, setLogEntries] = useState<LogEntry[]>([]);
  const [napcatStatus, setNapcatStatus] = useState<any>(IS_DEMO ? DEMO_NAPCAT_STATUS : { connected: false });
  const [wechatStatus, setWechatStatus] = useState<any>(IS_DEMO ? DEMO_WECHAT_STATUS : { connected: false });
  const [openclawStatus, setOpenclawStatus] = useState<any>({});
  const [processStatus, setProcessStatus] = useState<any>({});
  const [wsMessages, setWsMessages] = useState<any[]>([]);

  // Fetch initial status and event log from API
  useEffect(() => {
    const fetchStatus = () => {
      if (document.hidden) return;
      api.getStatus().then(r => {
        if (r.ok && r.napcat) setNapcatStatus(r.napcat);
        if (r.ok && r.wechat) setWechatStatus(r.wechat);
        if (r.ok && r.openclaw) setOpenclawStatus(r.openclaw);
        if (r.ok && r.process) setProcessStatus(r.process);
      }).catch(() => {});
    };
    fetchStatus();
    const t = setInterval(fetchStatus, 8000);
    const onVisible = () => { if (!document.hidden) fetchStatus(); };
    document.addEventListener('visibilitychange', onVisible);
    return () => { clearInterval(t); document.removeEventListener('visibilitychange', onVisible); };
  }, []);

  // Load persisted event log on mount (or demo data)
  useEffect(() => {
    if (IS_DEMO) {
      setLogEntries(DEMO_LOG_ENTRIES.map(demoToLogEntry));
      return;
    }
    api.getEvents({ limit: 200 }).then(r => {
      if (r.ok && (r.events || r.entries)) setLogEntries(r.events || r.entries);
    }).catch(() => {});
  }, []);

  // Fallback polling: merge latest events in case websocket misses a push
  useEffect(() => {
    if (IS_DEMO) return;

    const poll = () => {
      if (document.hidden) return;
      api.getEvents({ limit: 80 }).then(r => {
        if (!r.ok || !(r.events || r.entries)) return;
        const incoming: LogEntry[] = (r.events || r.entries) as LogEntry[];

        setLogEntries(prev => {
          const idSet = new Set(prev.map(item => String(item.id)));
          const additions = incoming.filter(item => !idSet.has(String(item.id)));
          if (additions.length === 0) return prev;
          const merged = [...additions, ...prev];
          merged.sort((a, b) => b.time - a.time);
          return merged.slice(0, 500);
        });
      }).catch(() => {});
    };

    const t = setInterval(poll, 3000);
    return () => clearInterval(t);
  }, []);

  // Demo mode: simulate periodic new log entries
  useEffect(() => {
    if (!IS_DEMO) return;
    const msgs = [
      { source: 'qq', type: 'text', content: '帮我查一下明天的天气预报' },
      { source: 'openclaw', type: 'text', content: '明天北京天气多云，气温18-26°C，东南风3级，空气质量良好。' },
      { source: 'qq', type: 'text', content: '推荐几本好书' },
      { source: 'openclaw', type: 'text', content: '推荐：《人类简史》《三体》《思考，快与慢》《原则》《黑天鹅》' },
      { source: 'system', type: 'text', content: '[System] Skill "web-search" executed (245ms)' },
      { source: 'qq', type: 'text', content: '今天有什么新闻？' },
      { source: 'openclaw', type: 'text', content: '今日热点：1. AI技术突破 2. 新能源汽车销量创新高 3. 国际科技峰会召开' },
    ];
    let idx = 0;
    const t = setInterval(() => {
      const m = msgs[idx % msgs.length];
      const entry: LogEntry = {
        id: `demo_${Date.now()}_${idx}`,
        time: Date.now(),
        source: m.source as any,
        type: m.type,
        summary: m.content,
        detail: m.content,
      };
      setLogEntries(prev => [entry, ...prev].slice(0, 500));
      idx++;
    }, 8000);
    return () => clearInterval(t);
  }, []);

  useEffect(() => {
    if (IS_DEMO) return;
    let unmounted = false;
    let reconnectAttempts = 0;
    const MAX_RECONNECT = 20;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

    const connect = () => {
      if (unmounted) return;
      const token = localStorage.getItem('admin-token');
      if (!token) return;
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const url = `${protocol}//${window.location.host}/ws?token=${token}`;
      const ws = new WebSocket(url);
      wsRef.current = ws;

      ws.onopen = () => { reconnectAttempts = 0; };

      ws.onmessage = (e) => {
        try {
          const msg = JSON.parse(e.data);
          if (msg.type === 'log-entry') {
            setLogEntries(prev => {
              const next = [msg.data, ...prev];
              return next.length > 500 ? next.slice(0, 500) : next;
            });
          } else if (msg.type === 'napcat-status') {
            setNapcatStatus(msg.data);
          } else if (msg.type === 'wechat-status') {
            setWechatStatus(msg.data);
          } else if (msg.type === 'task_update' || msg.type === 'task_log') {
            setWsMessages(prev => [...prev.slice(-100), msg]);
          }
        } catch {}
      };

      ws.onclose = () => {
        if (unmounted) return;
        wsRef.current = null;
        if (!localStorage.getItem('admin-token')) return;
        if (reconnectAttempts < MAX_RECONNECT) {
          const delay = Math.min(3000 * Math.pow(2, reconnectAttempts), 30000);
          reconnectAttempts++;
          reconnectTimer = setTimeout(connect, delay);
        }
      };
    };

    connect();
    return () => {
      unmounted = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      wsRef.current?.close();
    };
  }, []);

  const clearEvents = useCallback(() => {
    setLogEntries([]);
    api.clearEvents().catch(() => {});
  }, []);
  const refreshLog = useCallback(() => {
    if (IS_DEMO) {
      setLogEntries(DEMO_LOG_ENTRIES.map(demoToLogEntry));
      return;
    }
    api.getEvents({ limit: 200 }).then(r => {
      if (r.ok && (r.events || r.entries)) setLogEntries(r.events || r.entries);
    }).catch(() => {});
  }, []);

  return { logEntries, napcatStatus, wechatStatus, openclawStatus, processStatus, wsMessages, clearEvents, refreshLog };
}

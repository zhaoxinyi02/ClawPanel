import { useEffect, useRef, useState, useCallback } from 'react';
import { api } from '../lib/api';
import { DEMO_LOG_ENTRIES, DEMO_NAPCAT_STATUS, DEMO_WECHAT_STATUS } from '../lib/mockApi';

const IS_DEMO = import.meta.env.VITE_DEMO === 'true';

export interface LogEntry {
  id: string;
  time: number;
  source: 'qq' | 'wechat' | 'system' | 'openclaw';
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
  const [events, setEvents] = useState<any[]>([]);
  const [logEntries, setLogEntries] = useState<LogEntry[]>([]);
  const [napcatStatus, setNapcatStatus] = useState<any>(IS_DEMO ? DEMO_NAPCAT_STATUS : { connected: false });
  const [wechatStatus, setWechatStatus] = useState<any>(IS_DEMO ? DEMO_WECHAT_STATUS : { connected: false });
  const [openclawStatus, setOpenclawStatus] = useState<any>({});

  // Fetch initial status and event log from API
  useEffect(() => {
    const fetchStatus = () => {
      api.getStatus().then(r => {
        if (r.ok && r.napcat) setNapcatStatus(r.napcat);
        if (r.ok && r.wechat) setWechatStatus(r.wechat);
        if (r.ok && r.openclaw) setOpenclawStatus(r.openclaw);
      }).catch(() => {});
    };
    fetchStatus();
    const t = setInterval(fetchStatus, 8000);
    return () => clearInterval(t);
  }, []);

  // Load persisted event log on mount (or demo data)
  useEffect(() => {
    if (IS_DEMO) {
      setLogEntries(DEMO_LOG_ENTRIES.map(demoToLogEntry));
      return;
    }
    api.getEvents({ limit: 200 }).then(r => {
      if (r.ok && r.entries) setLogEntries(r.entries);
    }).catch(() => {});
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
    if (IS_DEMO) return; // Skip real WebSocket in demo mode
    const token = localStorage.getItem('admin-token');
    if (!token) return;
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${window.location.host}/ws?token=${token}`;
    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data);
        if (msg.type === 'event' || msg.type === 'wechat-event') {
          setEvents(prev => { const next = [...prev, { ...msg.data, _source: msg.type === 'wechat-event' ? 'wechat' : 'qq' }]; return next.length > 200 ? next.slice(-200) : next; });
        } else if (msg.type === 'log-entry') {
          setLogEntries(prev => {
            const next = [msg.data, ...prev];
            return next.length > 500 ? next.slice(0, 500) : next;
          });
        } else if (msg.type === 'napcat-status') {
          setNapcatStatus(msg.data);
        } else if (msg.type === 'wechat-status') {
          setWechatStatus(msg.data);
        }
      } catch {}
    };

    ws.onclose = () => {
      setTimeout(() => { if (localStorage.getItem('admin-token')) window.location.reload(); }, 5000);
    };

    return () => { ws.close(); };
  }, []);

  const clearEvents = useCallback(() => setEvents([]), []);
  const refreshLog = useCallback(() => {
    if (IS_DEMO) {
      setLogEntries(DEMO_LOG_ENTRIES.map(demoToLogEntry));
      return;
    }
    api.getEvents({ limit: 200 }).then(r => {
      if (r.ok && r.entries) setLogEntries(r.entries);
    }).catch(() => {});
  }, []);

  return { events, logEntries, napcatStatus, wechatStatus, openclawStatus, clearEvents, refreshLog };
}

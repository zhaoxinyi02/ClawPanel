import { useState, useRef, useEffect, useCallback } from 'react';
import { MessageCircle, Send, Loader2, Bot, Settings, ChevronDown, Minimize2, Trash2, GripHorizontal, Maximize2 } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { api } from '../lib/api';

interface ChatMessage {
  role: 'user' | 'assistant';
  content: string;
  time: number;
}

function normalizeProviderModels(value: any): any[] {
  if (Array.isArray(value)) return value;
  return [];
}

function toModelOptionValue(pid: string, mid: string) {
  return JSON.stringify({ pid, mid });
}

function fromModelOptionValue(value: string): { pid: string; mid: string } | null {
  if (!value) return null;
  try {
    const parsed = JSON.parse(value);
    if (parsed && typeof parsed.pid === 'string' && typeof parsed.mid === 'string') {
      return parsed;
    }
  } catch {}
  return null;
}

const MIN_W = 380, MIN_H = 420, DEFAULT_W = 480, DEFAULT_H = 620;

export default function AIAssistant() {
  const [open, setOpen] = useState(false);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [providerId, setProviderId] = useState('');
  const [modelId, setModelId] = useState('');
  const [providers, setProviders] = useState<Record<string, any>>({});
  const [primaryModel, setPrimaryModel] = useState('');
  const [isMaximized, setIsMaximized] = useState(false);
  const [isMobile, setIsMobile] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const savedPosSize = useRef({ x: 0, y: 0, w: DEFAULT_W, h: DEFAULT_H });

  // Position & size state
  const [pos, setPos] = useState({ x: -1, y: -1 }); // -1 = not initialized
  const [size, setSize] = useState({ w: DEFAULT_W, h: DEFAULT_H });
  const dragging = useRef(false);
  const resizing = useRef(false);
  const dragOffset = useRef({ x: 0, y: 0 });
  const resizeStart = useRef({ x: 0, y: 0, w: 0, h: 0 });

  const loadModelConfig = useCallback(async () => {
    try {
      const r = await api.getOpenClawConfig();
      if (!r.ok) return null;

      const nextProviders = r.config?.models?.providers || {};
      const nextPrimary = r.config?.agents?.defaults?.model?.primary || '';
      setProviders(nextProviders);
      setPrimaryModel(nextPrimary);

      if (providerId && modelId) {
        const models = normalizeProviderModels(nextProviders?.[providerId]?.models);
        const exists = models.some((m: any) => (typeof m === 'string' ? m : m?.id) === modelId);
        if (!exists) {
          setProviderId('');
          setModelId('');
        }
      }
      return { providers: nextProviders, primary: nextPrimary };
    } catch {}
    return null;
  }, [providerId, modelId]);

  useEffect(() => {
    const syncViewport = () => {
      setIsMobile(window.innerWidth < 768);
    };
    syncViewport();
    window.addEventListener('resize', syncViewport);
    return () => window.removeEventListener('resize', syncViewport);
  }, []);

  // Initialize position to bottom-right
  useEffect(() => {
    if (open && pos.x === -1) {
      setPos({ x: window.innerWidth - size.w - 24, y: window.innerHeight - size.h - 24 });
    }
  }, [open, pos.x, size.w]);

  useEffect(() => {
    if (!open || !isMobile) return;
    const width = Math.max(window.innerWidth - 16, 320);
    const height = Math.max(window.innerHeight - 16, 480);
    setPos({ x: 8, y: 8 });
    setSize({ w: width, h: height });
    setIsMaximized(false);
  }, [open, isMobile]);

  // Load model config
  useEffect(() => {
    if (open) {
      loadModelConfig();
    }
  }, [open, loadModelConfig]);

  // Auto-scroll
  useEffect(() => {
    if (scrollRef.current) scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
  }, [messages, loading]);

  // Focus input when opened
  useEffect(() => {
    if (open && inputRef.current) setTimeout(() => inputRef.current?.focus(), 100);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      setShowSettings(false);
      setOpen(false);
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [open]);

  // Drag handlers
  const onDragStart = useCallback((e: React.MouseEvent) => {
    if ((e.target as HTMLElement).closest('button, select, input, textarea, a')) return;
    dragging.current = true;
    dragOffset.current = { x: e.clientX - pos.x, y: e.clientY - pos.y };
    e.preventDefault();
  }, [pos]);

  // Resize handlers
  const onResizeStart = useCallback((e: React.MouseEvent) => {
    resizing.current = true;
    resizeStart.current = { x: e.clientX, y: e.clientY, w: size.w, h: size.h };
    e.preventDefault();
    e.stopPropagation();
  }, [size]);

  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      if (isMobile) return;
      if (dragging.current) {
        const nx = Math.max(0, Math.min(window.innerWidth - size.w, e.clientX - dragOffset.current.x));
        const ny = Math.max(0, Math.min(window.innerHeight - size.h, e.clientY - dragOffset.current.y));
        setPos({ x: nx, y: ny });
      }
      if (resizing.current) {
        const dw = e.clientX - resizeStart.current.x;
        const dh = e.clientY - resizeStart.current.y;
        setSize({
          w: Math.max(MIN_W, resizeStart.current.w + dw),
          h: Math.max(MIN_H, resizeStart.current.h + dh),
        });
      }
    };
    const onUp = () => { dragging.current = false; resizing.current = false; };
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
    return () => { window.removeEventListener('mousemove', onMove); window.removeEventListener('mouseup', onUp); };
  }, [size, isMobile]);

  const sendMessage = async () => {
    const text = input.trim();
    if (!text || loading) return;
    setInput('');
    const userMsg: ChatMessage = { role: 'user', content: text, time: Date.now() };
    setMessages(prev => [...prev, userMsg]);
    setLoading(true);

    try {
      let effectiveProviderId = providerId;
      let effectiveModelId = modelId;
      const latest = await loadModelConfig();

      if (latest && effectiveProviderId && effectiveModelId) {
        const models = normalizeProviderModels(latest.providers?.[effectiveProviderId]?.models);
        const exists = models.some((m: any) => (typeof m === 'string' ? m : m?.id) === effectiveModelId);
        if (!exists) {
          effectiveProviderId = '';
          effectiveModelId = '';
        }
      }

      const chatHistory = [...messages, userMsg].slice(-20).map(m => ({ role: m.role, content: m.content }));
      const r = await api.aiChat(chatHistory, effectiveProviderId || undefined, effectiveModelId || undefined);
      if (r.ok && r.reply) {
        setMessages(prev => [...prev, { role: 'assistant', content: r.reply, time: Date.now() }]);
      } else {
        setMessages(prev => [...prev, { role: 'assistant', content: `⚠️ ${r.error || '请求失败，请检查模型配置'}`, time: Date.now() }]);
      }
    } catch (err: any) {
      setMessages(prev => [...prev, { role: 'assistant', content: `⚠️ 网络错误: ${err.message || '请稍后重试'}`, time: Date.now() }]);
    } finally {
      setLoading(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendMessage(); }
  };

  // All available models for dropdown
  const allModels: { pid: string; mid: string; label: string }[] = [];
  for (const [pid, prov] of Object.entries(providers) as [string, any][]) {
    for (const m of normalizeProviderModels(prov?.models)) {
      const mid = typeof m === 'string' ? m : m.id;
      if (!mid || typeof mid !== 'string') continue;
      allModels.push({ pid, mid, label: `${pid}/${mid}` });
    }
  }

  const currentModel = providerId && modelId ? `${providerId}/${modelId}` : primaryModel || '未配置';

  return (
    <>
      {/* Floating bubble */}
      {!open && (
        <button onClick={() => setOpen(true)}
          className={`fixed z-50 flex items-center justify-center rounded-full border border-blue-200/70 bg-[linear-gradient(145deg,rgba(255,255,255,0.24),rgba(255,255,255,0.08)),linear-gradient(135deg,rgba(37,99,235,0.96),rgba(14,165,233,0.86))] text-white shadow-[0_18px_40px_rgba(37,99,235,0.34)] backdrop-blur-2xl transition-all duration-300 hover:scale-110 hover:shadow-[0_22px_48px_rgba(37,99,235,0.42)] dark:border-blue-400/20 dark:bg-[linear-gradient(145deg,rgba(255,255,255,0.05),rgba(255,255,255,0.02)),linear-gradient(135deg,rgba(29,78,216,0.88),rgba(8,145,178,0.74))] dark:shadow-[0_22px_52px_rgba(2,6,23,0.55)] group ${isMobile ? 'bottom-24 right-4 h-12 w-12' : 'bottom-6 right-6 h-14 w-14'}`}
          title="AI 助手">
          <Bot size={isMobile ? 20 : 24} className="group-hover:scale-110 transition-transform" />
          <span className="absolute -right-1 -top-1 h-3.5 w-3.5 rounded-full border-2 border-white bg-emerald-400 animate-pulse dark:border-slate-950" />
        </button>
      )}

      {/* Chat panel — draggable & resizable */}
      {open && (
        <div ref={panelRef}
          className="ui-modern-panel fixed z-50 flex flex-col overflow-hidden rounded-[28px] border border-blue-100/70 shadow-[0_28px_70px_rgba(15,23,42,0.18)] before:pointer-events-none before:absolute before:inset-0 before:bg-[linear-gradient(180deg,rgba(255,255,255,0.26),transparent_24%)] before:content-[''] dark:border-blue-400/15 dark:shadow-[0_34px_80px_rgba(2,6,23,0.55)]"
          style={isMobile ? { left: 8, top: 8, width: 'calc(100vw - 16px)', height: 'calc(100dvh - 16px)', borderRadius: 24 } : { left: pos.x, top: pos.y, width: size.w, height: size.h }}>

          {/* Header — drag handle */}
          <div className={`relative flex shrink-0 select-none items-center justify-between border-b border-blue-100/70 bg-[linear-gradient(145deg,rgba(255,255,255,0.12),rgba(255,255,255,0.04)),linear-gradient(135deg,rgba(37,99,235,0.94),rgba(14,165,233,0.82))] px-4 py-3 text-white dark:border-blue-400/15 dark:bg-[linear-gradient(145deg,rgba(255,255,255,0.04),rgba(255,255,255,0.01)),linear-gradient(135deg,rgba(15,23,42,0.92),rgba(30,64,175,0.72))] ${isMobile ? '' : 'cursor-move'}`}
            onMouseDown={onDragStart}>
            <div className="flex items-center gap-2.5">
              <div className="flex h-8 w-8 items-center justify-center rounded-full border border-white/20 bg-white/15 backdrop-blur-xl">
                <Bot size={18} />
              </div>
              <div>
                <h3 className="text-sm font-bold flex items-center gap-1.5">AI 助手 <GripHorizontal size={12} className="opacity-50" /></h3>
                <p className="text-[10px] text-white/70 truncate" style={{ maxWidth: size.w - 200 }}>模型: {currentModel}</p>
              </div>
            </div>
            <div className="flex items-center gap-1">
              <button onClick={() => setShowSettings(!showSettings)} className="rounded-xl border border-white/10 bg-white/10 p-1.5 transition-colors hover:bg-white/20" title="设置">
                <Settings size={14} />
              </button>
              <button onClick={() => setMessages([])} className="rounded-xl border border-white/10 bg-white/10 p-1.5 transition-colors hover:bg-white/20" title="清空对话">
                <Trash2 size={14} />
              </button>
              {!isMobile && <button onClick={() => {
                if (isMaximized) {
                  setPos({ x: savedPosSize.current.x, y: savedPosSize.current.y });
                  setSize({ w: savedPosSize.current.w, h: savedPosSize.current.h });
                  setIsMaximized(false);
                } else {
                  savedPosSize.current = { x: pos.x, y: pos.y, w: size.w, h: size.h };
                  const maxW = Math.min(900, window.innerWidth - 32);
                  const maxH = Math.min(900, window.innerHeight - 32);
                  setSize({ w: maxW, h: maxH });
                  setPos({ x: (window.innerWidth - maxW) / 2, y: (window.innerHeight - maxH) / 2 });
                  setIsMaximized(true);
                }
              }} className="rounded-xl border border-white/10 bg-white/10 p-1.5 transition-colors hover:bg-white/20" title={isMaximized ? '还原大小' : '最大化'}>
                {isMaximized ? <Minimize2 size={14} /> : <Maximize2 size={14} />}
              </button>}
              <button onClick={() => {
                setShowSettings(false);
                setOpen(false);
              }} className="rounded-xl border border-white/10 bg-white/10 p-1.5 transition-colors hover:bg-white/20" title="最小化（Esc）">
                <Minimize2 size={14} />
              </button>
            </div>
          </div>

          {/* Settings panel */}
          {showSettings && (
            <div className="shrink-0 space-y-2 border-b border-blue-100/70 bg-white/45 px-4 py-3 backdrop-blur-2xl dark:border-blue-400/15 dark:bg-slate-950/30">
              <label className="text-[10px] font-bold text-gray-500 uppercase tracking-wider">模型选择</label>
              <div className="relative">
                <select value={providerId && modelId ? toModelOptionValue(providerId, modelId) : ''}
                  onChange={e => {
                    const next = fromModelOptionValue(e.target.value);
                    if (!next) { setProviderId(''); setModelId(''); return; }
                    setProviderId(next.pid);
                    setModelId(next.mid);
                  }}
                  className="page-modern-control w-full cursor-pointer appearance-none px-3 py-2 text-xs">
                  <option value="">使用主模型 ({primaryModel || '未配置'})</option>
                  {allModels.map(m => <option key={`${m.pid}:${m.mid}`} value={toModelOptionValue(m.pid, m.mid)}>{m.label}</option>)}
                </select>
                <ChevronDown size={12} className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 pointer-events-none" />
              </div>
              <p className="text-[10px] text-gray-400">在「设置 → 模型配置」中添加更多模型</p>
            </div>
          )}

          {/* Messages */}
          <div ref={scrollRef} className="ui-modern-scrollbar flex-1 min-h-0 space-y-4 overflow-y-auto bg-[radial-gradient(circle_at_top,rgba(59,130,246,0.08),transparent_28%),linear-gradient(180deg,rgba(255,255,255,0.18),transparent_42%)] p-4 scroll-smooth dark:bg-[radial-gradient(circle_at_top,rgba(59,130,246,0.12),transparent_30%),linear-gradient(180deg,rgba(255,255,255,0.03),transparent_36%)]">
            {messages.length === 0 && (
              <div className="flex flex-col items-center justify-center h-full text-gray-400 gap-3">
                <div className="flex h-16 w-16 items-center justify-center rounded-full border border-blue-100/80 bg-[linear-gradient(135deg,rgba(255,255,255,0.88),rgba(219,234,254,0.76))] text-blue-500 shadow-[0_14px_30px_rgba(59,130,246,0.16)] dark:border-blue-400/15 dark:bg-[linear-gradient(135deg,rgba(14,28,48,0.76),rgba(18,39,66,0.58))] dark:text-blue-200">
                  <Bot size={28} className="text-current" />
                </div>
                <div className="text-center">
                  <p className="text-sm font-medium text-gray-500 dark:text-gray-400">需要帮助？</p>
                  <p className="text-xs text-gray-400 mt-1">可以直接询问管理后台相关问题</p>
                </div>
                <div className="flex flex-wrap gap-1.5 justify-center mt-2">
                  {['如何配置模型？', '技能怎么启用？', '怎么添加QQ通道？', '你使用的是什么模型？'].map(q => (
                    <button key={q} onClick={() => { setInput(q); }}
                      className="page-modern-action px-2.5 py-1.5 text-[10px]">
                      {q}
                    </button>
                  ))}
                </div>
              </div>
            )}

            {messages.map((msg, i) => (
              <div key={i} className={`flex gap-2.5 ${msg.role === 'user' ? 'flex-row-reverse' : ''}`}>
                <div className={`mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-full border ${msg.role === 'user' ? 'border-blue-200/80 bg-[linear-gradient(135deg,rgba(59,130,246,0.18),rgba(14,165,233,0.14))] text-blue-600 dark:border-blue-400/20 dark:bg-[linear-gradient(135deg,rgba(30,64,175,0.42),rgba(14,116,144,0.28))] dark:text-blue-200' : 'border-slate-200/80 bg-white/75 text-slate-500 dark:border-blue-400/15 dark:bg-slate-900/50 dark:text-slate-300'}`}>
                  {msg.role === 'user' ? <MessageCircle size={14} /> : <Bot size={14} />}
                </div>
                {msg.role === 'user' ? (
                  <div className="max-w-[85%] whitespace-pre-wrap break-words rounded-2xl rounded-tr-sm border border-blue-300/20 bg-[linear-gradient(135deg,rgba(37,99,235,0.96),rgba(14,165,233,0.82))] px-3.5 py-2.5 text-sm leading-relaxed text-white shadow-[0_14px_30px_rgba(37,99,235,0.22)] dark:border-blue-300/10 dark:shadow-[0_16px_36px_rgba(2,6,23,0.34)]">
                    {msg.content}
                  </div>
                ) : (
                  <div className="ui-modern-card ai-markdown max-w-[85%] rounded-2xl rounded-tl-sm border border-blue-100/70 px-3.5 py-2.5 text-gray-700 dark:border-blue-400/15 dark:text-gray-200">
                    <ReactMarkdown remarkPlugins={[remarkGfm]}
                      components={{
                        p: ({ children }) => <p className="text-sm leading-relaxed mb-2 last:mb-0">{children}</p>,
                        h1: ({ children }) => <h1 className="text-base font-bold mb-2 mt-3 first:mt-0">{children}</h1>,
                        h2: ({ children }) => <h2 className="text-sm font-bold mb-1.5 mt-2.5 first:mt-0">{children}</h2>,
                        h3: ({ children }) => <h3 className="text-sm font-semibold mb-1 mt-2 first:mt-0">{children}</h3>,
                        ul: ({ children }) => <ul className="list-disc list-inside text-sm space-y-0.5 mb-2 ml-1">{children}</ul>,
                        ol: ({ children }) => <ol className="list-decimal list-inside text-sm space-y-0.5 mb-2 ml-1">{children}</ol>,
                        li: ({ children }) => <li className="leading-relaxed">{children}</li>,
                        code: ({ className, children, ...props }) => {
                          const isBlock = className?.includes('language-');
                          return isBlock ? (
                            <pre className="my-2 overflow-x-auto rounded-xl border border-slate-800/70 bg-slate-950/95 p-3 text-xs leading-relaxed text-gray-100 shadow-inner"><code>{children}</code></pre>
                          ) : (
                            <code className="rounded-md bg-blue-50 px-1.5 py-0.5 text-xs font-mono text-blue-700 dark:bg-slate-800 dark:text-blue-200" {...props}>{children}</code>
                          );
                        },
                        pre: ({ children }) => <>{children}</>,
                        a: ({ href, children }) => <a href={href} target="_blank" rel="noopener noreferrer" className="text-blue-600 underline hover:text-blue-700 dark:text-blue-300">{children}</a>,
                        blockquote: ({ children }) => <blockquote className="my-2 border-l-[3px] border-blue-300 pl-3 text-sm italic text-gray-500 dark:border-blue-600 dark:text-gray-400">{children}</blockquote>,
                        table: ({ children }) => <div className="overflow-x-auto my-2"><table className="text-xs border-collapse w-full">{children}</table></div>,
                        th: ({ children }) => <th className="border border-blue-100 px-2 py-1 text-left font-semibold bg-blue-50/70 dark:border-blue-400/15 dark:bg-slate-900/40">{children}</th>,
                        td: ({ children }) => <td className="border border-blue-100 px-2 py-1 dark:border-blue-400/15">{children}</td>,
                        strong: ({ children }) => <strong className="font-bold">{children}</strong>,
                        em: ({ children }) => <em className="italic">{children}</em>,
                        hr: () => <hr className="my-3 border-blue-100 dark:border-blue-400/15" />,
                      }}>
                      {msg.content}
                    </ReactMarkdown>
                  </div>
                )}
              </div>
            ))}

            {loading && (
              <div className="flex gap-2.5">
                <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full border border-slate-200/80 bg-white/75 text-slate-500 dark:border-blue-400/15 dark:bg-slate-900/50 dark:text-slate-300">
                  <Bot size={14} />
                </div>
                <div className="ui-modern-card rounded-2xl rounded-tl-sm border border-blue-100/70 px-3.5 py-2.5 dark:border-blue-400/15">
                  <div className="flex items-center gap-2">
                    <Loader2 size={14} className="animate-spin text-blue-500" />
                    <span className="text-sm text-gray-400">思考中...</span>
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* Input */}
          <div className="shrink-0 border-t border-blue-100/70 bg-white/55 px-3 py-3 backdrop-blur-2xl dark:border-blue-400/15 dark:bg-slate-950/34">
            <div className="flex items-end gap-2">
              <textarea ref={inputRef} value={input} onChange={e => setInput(e.target.value)} onKeyDown={handleKeyDown}
                placeholder="输入问题... (Enter 发送, Shift+Enter 换行)"
                rows={1}
                className="page-modern-control max-h-24 flex-1 resize-none px-3.5 py-2.5 text-sm leading-relaxed"
                style={{ minHeight: '40px' }} />
              <button onClick={sendMessage} disabled={!input.trim() || loading}
                className="page-modern-accent shrink-0 rounded-xl p-2.5 disabled:cursor-not-allowed disabled:opacity-40">
                {loading ? <Loader2 size={18} className="animate-spin" /> : <Send size={18} />}
              </button>
            </div>
          </div>

          {/* Resize handle — bottom-right corner */}
          {!isMobile && <div className="absolute bottom-0 right-0 z-10 h-4 w-4 cursor-se-resize group" onMouseDown={onResizeStart}>
            <svg viewBox="0 0 16 16" className="h-full w-full text-slate-300 transition-colors group-hover:text-blue-400 dark:text-slate-600 dark:group-hover:text-blue-300">
              <path d="M14 14L14 8M14 14L8 14M10 14L14 10" stroke="currentColor" strokeWidth="1.5" fill="none" strokeLinecap="round" />
            </svg>
          </div>}
        </div>
      )}
    </>
  );
}

import { useState, useEffect, useRef } from 'react';
import { Bell, X, CheckCircle, AlertTriangle, Loader2, ChevronDown, ChevronUp, Terminal } from 'lucide-react';
import { useI18n } from '../i18n';

export interface TaskInfo {
  id: string;
  name: string;
  type: string;
  status: 'pending' | 'running' | 'success' | 'failed' | 'canceled';
  progress: number;
  error?: string;
  createdAt: string;
  updatedAt: string;
  logCount?: number;
  log?: string[];
}

interface Props {
  tasks: TaskInfo[];
  taskLogs: Record<string, string[]>;
  onRefresh: () => void;
  mode?: 'sidebar' | 'icon';
}

export default function MessageCenter({ tasks, taskLogs, onRefresh, mode = 'sidebar' }: Props) {
  const { t, locale } = useI18n();
  const [open, setOpen] = useState(false);
  const [expandedTask, setExpandedTask] = useState<string | null>(null);
  const logEndRef = useRef<HTMLDivElement>(null);

  const runningCount = tasks.filter(t => t.status === 'running').length;
  const hasNew = tasks.some(t => t.status === 'running' || (t.status === 'success' && Date.now() - new Date(t.updatedAt).getTime() < 10000));

  useEffect(() => {
    if (logEndRef.current && expandedTask) {
      logEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [taskLogs, expandedTask]);

  // Auto-open when a new task starts
  useEffect(() => {
    if (runningCount > 0) setOpen(true);
  }, [runningCount]);

  const statusIcon = (status: string) => {
    switch (status) {
      case 'running': return <Loader2 size={14} className="animate-spin text-blue-500" />;
      case 'success': return <CheckCircle size={14} className="text-emerald-500" />;
      case 'failed': return <AlertTriangle size={14} className="text-red-500" />;
      default: return <Loader2 size={14} className="text-gray-400" />;
    }
  };

  const statusLabel = (status: string) => {
    switch (status) {
      case 'pending': return t.messageCenter.statusPending;
      case 'running': return t.messageCenter.statusRunning;
      case 'success': return t.messageCenter.statusSuccess;
      case 'failed': return t.messageCenter.statusFailed;
      case 'canceled': return t.messageCenter.statusCanceled;
      default: return status;
    }
  };

  const statusColor = (status: string) => {
    switch (status) {
      case 'running': return 'text-blue-600 bg-blue-50 dark:bg-blue-900/30 dark:text-blue-400';
      case 'success': return 'text-emerald-600 bg-emerald-50 dark:bg-emerald-900/30 dark:text-emerald-400';
      case 'failed': return 'text-red-600 bg-red-50 dark:bg-red-900/30 dark:text-red-400';
      default: return 'text-gray-500 bg-gray-50 dark:bg-gray-800 dark:text-gray-400';
    }
  };

  const indicatorColor = tasks.some(t => t.status === 'failed')
    ? 'bg-red-500'
    : runningCount > 0
      ? 'bg-amber-400'
      : hasNew
        ? 'bg-emerald-500'
        : '';

  return (
    <div className="relative">
      {/* Toggle Button */}
      <button
        onClick={() => { setOpen(!open); if (!open) onRefresh(); }}
        className={mode === 'icon'
          ? 'relative inline-flex items-center justify-center w-10 h-10 rounded-full bg-white border border-slate-200 text-slate-500 hover:text-slate-700 transition-colors'
          : 'flex items-center gap-2 px-3 py-2 rounded-xl text-[13px] text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-800 border border-transparent w-full relative transition-all'}
      >
        <Bell size={16} />
        {mode !== 'icon' && <span>{t.messageCenter.title}</span>}
        {indicatorColor && <span className={`absolute ${mode === 'icon' ? 'top-2 right-2' : 'top-1 right-1'} w-2.5 h-2.5 rounded-full ${indicatorColor}`}></span>}
      </button>

      {/* Panel */}
      {open && (
        <div className={`${mode === 'icon' ? 'absolute top-full right-0 mt-3' : 'absolute bottom-full left-0 mb-2'} w-[420px] max-h-[520px] bg-white dark:bg-gray-900 rounded-xl shadow-2xl border border-gray-200 dark:border-gray-700 overflow-hidden z-50 animate-in fade-in slide-in-from-bottom-2 duration-200`}>
          {/* Header */}
          <div className="flex items-center justify-between px-4 py-3 border-b border-gray-100 dark:border-gray-800 bg-gray-50/50 dark:bg-gray-800/50">
            <div className="flex items-center gap-2">
              <Terminal size={16} className="text-violet-500" />
              <span className="text-sm font-bold text-gray-900 dark:text-white">{t.messageCenter.title}</span>
              {runningCount > 0 && (
                <span className="text-[10px] font-medium px-1.5 py-0.5 rounded-full bg-blue-100 dark:bg-blue-900/40 text-blue-600 dark:text-blue-400">
                  {runningCount} {t.messageCenter.statusRunning}
                </span>
              )}
            </div>
            <button onClick={() => setOpen(false)} className="p-1 hover:bg-gray-200 dark:hover:bg-gray-700 rounded-lg transition-colors">
              <X size={14} className="text-gray-400" />
            </button>
          </div>

          {/* Task List */}
          <div className="overflow-y-auto max-h-[460px] divide-y divide-gray-100 dark:divide-gray-800">
            {tasks.length === 0 ? (
              <div className="py-12 text-center text-gray-400 text-sm">{t.messageCenter.noTasks}</div>
            ) : (
              tasks.map(task => {
                const logs = taskLogs[task.id] || task.log || [];
                const isExpanded = expandedTask === task.id;
                return (
                  <div key={task.id} className="group">
                    <button
                      onClick={() => setExpandedTask(isExpanded ? null : task.id)}
                      className="w-full px-4 py-3 flex items-center gap-3 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors text-left"
                    >
                      {statusIcon(task.status)}
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-medium text-gray-900 dark:text-white truncate">{task.name}</span>
                          <span className={`text-[10px] font-medium px-1.5 py-0.5 rounded-full ${statusColor(task.status)}`}>
                            {statusLabel(task.status)}
                          </span>
                        </div>
                        {task.status === 'running' && (
                          <div className="mt-1.5 w-full bg-gray-200 dark:bg-gray-700 rounded-full h-1.5">
                            <div className="bg-blue-500 h-1.5 rounded-full transition-all duration-500 animate-pulse" style={{ width: `${Math.max(task.progress, 10)}%` }}></div>
                          </div>
                        )}
                        {task.error && <p className="text-[11px] text-red-500 mt-0.5 truncate">{task.error}</p>}
                        <p className="text-[10px] text-gray-400 mt-0.5">
                          {new Date(task.createdAt).toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                          {logs.length > 0 && ` · ${t.messageCenter.logLines.replace('{n}', String(logs.length))}`}
                        </p>
                      </div>
                      {isExpanded ? <ChevronUp size={14} className="text-gray-400 shrink-0" /> : <ChevronDown size={14} className="text-gray-400 shrink-0" />}
                    </button>

                    {/* Log Panel */}
                    {isExpanded && logs.length > 0 && (
                      <div className="bg-gray-950 text-gray-300 text-[11px] font-mono leading-relaxed max-h-[200px] overflow-y-auto px-4 py-2 border-t border-gray-800">
                        {logs.map((line, i) => (
                          <div key={i} className="whitespace-pre-wrap break-all py-px hover:bg-gray-900/50">
                            <span className="text-gray-600 select-none mr-2">{String(i + 1).padStart(3)}</span>
                            <span className={line.startsWith('✅') ? 'text-emerald-400' : line.startsWith('❌') ? 'text-red-400' : line.startsWith('⚠️') ? 'text-amber-400' : line.startsWith('📦') || line.startsWith('📥') || line.startsWith('🔧') ? 'text-blue-400' : ''}>{line}</span>
                          </div>
                        ))}
                        <div ref={logEndRef} />
                      </div>
                    )}
                  </div>
                );
              })
            )}
          </div>
        </div>
      )}
    </div>
  );
}

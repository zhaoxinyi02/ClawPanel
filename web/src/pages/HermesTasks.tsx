import { useEffect, useMemo, useState } from 'react';
import { useOutletContext, useSearchParams } from 'react-router-dom';
import { api } from '../lib/api';
import { RefreshCw, AlertTriangle, CheckCircle2, Loader2, Clock3, ArrowRight, ListTodo, Sparkles, TerminalSquare } from 'lucide-react';
import { useI18n } from '../i18n';

interface HermesTask {
  id: string;
  name: string;
  type: string;
  status: 'pending' | 'running' | 'success' | 'failed' | 'canceled';
  progress: number;
  error?: string;
  createdAt: string;
  updatedAt: string;
  log?: string[];
}

interface HermesTaskSummary {
  total?: number;
  byStatus?: Record<string, number>;
}

function statusTone(status: string) {
  if (status === 'running') return 'text-blue-700 bg-blue-50 dark:bg-blue-900/20 dark:text-blue-200';
  if (status === 'pending') return 'text-amber-700 bg-amber-50 dark:bg-amber-900/20 dark:text-amber-200';
  if (status === 'success') return 'text-emerald-700 bg-emerald-50 dark:bg-emerald-900/20 dark:text-emerald-200';
  if (status === 'failed') return 'text-red-700 bg-red-50 dark:bg-red-900/20 dark:text-red-200';
  return 'text-gray-700 bg-gray-100 dark:bg-gray-800 dark:text-gray-300';
}

export default function HermesTasks() {
  const { locale } = useI18n();
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const [searchParams, setSearchParams] = useSearchParams();
  const modern = uiMode === 'modern';
  const [tasks, setTasks] = useState<HermesTask[]>([]);
  const [summary, setSummary] = useState<HermesTaskSummary>({});
  const [selectedId, setSelectedId] = useState(searchParams.get('task') || '');
  const [detail, setDetail] = useState<HermesTask | null>(null);
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [err, setErr] = useState('');

  const load = async () => {
    setLoading(true);
    setErr('');
    try {
      const res = await api.getHermesTasks();
      if (!res?.ok) {
        setErr(res?.error || (locale === 'zh-CN' ? '加载 Hermes 任务失败' : 'Failed to load Hermes tasks'));
        return;
      }
      const nextTasks = Array.isArray(res.tasks) ? res.tasks : [];
      setTasks(nextTasks);
      setSummary(res.summary || {});
      setSelectedId(prev => prev || nextTasks[0]?.id || '');
    } catch {
      setErr(locale === 'zh-CN' ? '加载 Hermes 任务失败' : 'Failed to load Hermes tasks');
    } finally {
      setLoading(false);
    }
  };

  const loadDetail = async (taskId: string) => {
    if (!taskId) return;
    setDetailLoading(true);
    try {
      const res = await api.getHermesTaskDetail(taskId);
      if (res?.ok) setDetail(res.task || null);
    } finally {
      setDetailLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  useEffect(() => {
    if (!tasks.some(task => task.status === 'running' || task.status === 'pending')) return;
    const timer = setInterval(() => {
      void load();
    }, 2500);
    return () => clearInterval(timer);
  }, [tasks]);

  useEffect(() => {
    if (!selectedId) return;
    searchParams.set('task', selectedId);
    setSearchParams(searchParams, { replace: true });
    void loadDetail(selectedId);
  }, [selectedId]);

  const selectedTask = useMemo(
    () => tasks.find(task => task.id === selectedId) || detail,
    [tasks, selectedId, detail],
  );
  const progressWidth = Math.min(100, Math.max(6, selectedTask?.progress || 0));

  return (
    <div className={`space-y-6 ${modern ? 'page-modern' : ''}`}>
      <div className={`${modern ? 'page-modern-header' : 'flex items-center justify-between'}`}>
        <div>
          <h2 className={`${modern ? 'page-modern-title text-xl' : 'text-xl font-bold text-gray-900 dark:text-white'}`}>
            {locale === 'zh-CN' ? 'Hermes 任务账本' : 'Hermes Tasks'}
          </h2>
          <p className={`${modern ? 'page-modern-subtitle mt-1 text-sm' : 'mt-1 text-sm text-gray-500'}`}>
            {locale === 'zh-CN'
              ? '集中查看 Hermes Doctor、Update、Gateway 等动作对应的后台任务、进度和日志。'
              : 'Track Hermes background tasks, progress, and logs for doctor, update, and gateway actions.'}
          </p>
        </div>
        <button
          onClick={() => void load()}
          className={`${modern ? 'page-modern-action px-3 py-2 text-xs' : 'rounded-lg bg-gray-100 px-3 py-2 text-xs dark:bg-gray-800'} inline-flex items-center gap-2`}
        >
          <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          {locale === 'zh-CN' ? '刷新' : 'Refresh'}
        </button>
      </div>

      {err && <div className="rounded-2xl border border-red-100 bg-red-50/80 px-4 py-3 text-sm text-red-700 dark:border-red-900/30 dark:bg-red-900/10 dark:text-red-300">{err}</div>}

      <div className={`${modern ? 'rounded-[30px] border border-white/60 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.9),rgba(239,246,255,0.68))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.9),rgba(30,64,175,0.12))] backdrop-blur-xl shadow-[0_24px_54px_rgba(15,23,42,0.08)]' : 'rounded-2xl border border-gray-100 bg-white dark:bg-gray-800 dark:border-gray-700/50'} p-5`}>
        <div className="grid grid-cols-1 gap-5 xl:grid-cols-[1.15fr_0.85fr]">
          <div className="space-y-3">
            <div className="inline-flex items-center gap-2 rounded-full border border-blue-100/70 bg-white/80 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.18em] text-blue-700 dark:border-blue-400/15 dark:bg-slate-900/50 dark:text-blue-200">
              <Sparkles size={13} />
              {locale === 'zh-CN' ? 'Hermes 任务流' : 'Hermes Task Flow'}
            </div>
            <div className="text-lg font-semibold text-slate-900 dark:text-white">
              {locale === 'zh-CN'
                ? '像 OpenClaw 一样，把后台执行链条直接摊开看'
                : 'Expose the full background execution chain like OpenClaw does'}
            </div>
            <div className="text-sm leading-6 text-slate-600 dark:text-slate-300">
              {locale === 'zh-CN'
                ? '左侧看任务队列，右侧看进度、错误和日志。适合确认某个动作到底跑到了哪一步。'
                : 'Track queue state on the left and progress, errors, and logs on the right to see exactly where an action is in flight.'}
            </div>
          </div>
          <div className="rounded-[24px] border border-white/70 bg-white/80 p-4 shadow-[0_14px_34px_rgba(15,23,42,0.06)] dark:border-slate-700/50 dark:bg-slate-900/45">
            <div className="flex items-center justify-between gap-3">
              <div>
                <div className="text-xs font-semibold uppercase tracking-[0.18em] text-slate-400">
                  {locale === 'zh-CN' ? '当前选中任务' : 'Selected Task'}
                </div>
                <div className="mt-2 text-base font-semibold text-slate-900 dark:text-white">
                  {selectedTask?.name || (locale === 'zh-CN' ? '等待选择任务' : 'Waiting for selection')}
                </div>
              </div>
              {selectedTask && <span className={`rounded-full px-3 py-1 text-[10px] font-semibold uppercase ${statusTone(selectedTask.status)}`}>{selectedTask.status}</span>}
            </div>
            <div className="mt-3 text-sm text-slate-500 dark:text-slate-400">
              {selectedTask ? `${selectedTask.type} · ${selectedTask.id}` : (locale === 'zh-CN' ? '从左侧选择一条任务查看细节。' : 'Select a task on the left to inspect details.')}
            </div>
            {selectedTask && (
              <div className="mt-4">
                <div className="mb-2 flex items-center justify-between text-xs text-slate-500">
                  <span>{locale === 'zh-CN' ? '执行进度' : 'Execution Progress'}</span>
                  <span>{selectedTask.progress}%</span>
                </div>
                <div className="h-2 rounded-full bg-slate-100 dark:bg-slate-800">
                  <div className="h-2 rounded-full bg-blue-500 transition-all" style={{ width: `${progressWidth}%` }} />
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-5 gap-4">
        {[
          { label: locale === 'zh-CN' ? '总任务' : 'Total', value: summary.total || tasks.length },
          { label: locale === 'zh-CN' ? '待处理' : 'Pending', value: summary.byStatus?.pending || 0 },
          { label: locale === 'zh-CN' ? '运行中' : 'Running', value: summary.byStatus?.running || 0 },
          { label: locale === 'zh-CN' ? '成功' : 'Success', value: summary.byStatus?.success || 0 },
          { label: locale === 'zh-CN' ? '失败' : 'Failed', value: summary.byStatus?.failed || 0 },
        ].map(card => (
          <div key={card.label} className={`${modern ? 'page-modern-card' : 'bg-white dark:bg-gray-800'} rounded-2xl border border-gray-100 p-4 dark:border-gray-700/50`}>
            <div className="text-xs font-semibold uppercase tracking-wider text-gray-500">{card.label}</div>
            <div className="mt-3 text-lg font-bold text-gray-900 dark:text-white">{card.value}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[360px_1fr] gap-6">
        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-4 space-y-3`}>
          <div className="flex items-center justify-between gap-3 px-1">
            <div className="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
              <ListTodo size={16} className="text-blue-500" />
              {locale === 'zh-CN' ? '任务队列' : 'Task Queue'}
            </div>
            <div className="text-xs text-gray-500">{tasks.length}</div>
          </div>
          {tasks.map(task => (
            <button
              key={task.id}
              onClick={() => setSelectedId(task.id)}
              className={`w-full rounded-xl border px-4 py-3 text-left transition-colors ${
                selectedId === task.id
                  ? 'border-blue-300 bg-blue-50/70 dark:border-blue-700 dark:bg-blue-900/20'
                  : 'border-gray-100 hover:bg-gray-50 dark:border-gray-700/50 dark:hover:bg-gray-900/40'
              }`}
            >
              <div className="flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <div className="truncate font-medium text-gray-900 dark:text-white">{task.name}</div>
                  <div className="mt-1 text-xs text-gray-500">{task.type}</div>
                </div>
                <span className={`rounded-full px-2.5 py-1 text-[10px] font-semibold uppercase ${statusTone(task.status)}`}>
                  {task.status}
                </span>
              </div>
              <div className="mt-3 flex items-center justify-between gap-3 text-xs text-gray-500">
                <span className="inline-flex items-center gap-1">
                  <Clock3 size={12} />
                  {task.updatedAt}
                </span>
                <span className="inline-flex items-center gap-1 text-blue-600 dark:text-blue-300">
                  {locale === 'zh-CN' ? '查看' : 'Open'}
                  <ArrowRight size={12} />
                </span>
              </div>
            </button>
          ))}
          {tasks.length === 0 && !loading && (
            <div className="rounded-xl border border-dashed border-gray-200 px-4 py-6 text-sm text-gray-500 dark:border-gray-700/50 dark:text-gray-400">
              {locale === 'zh-CN' ? '当前没有 Hermes 任务' : 'No Hermes tasks'}
            </div>
          )}
        </div>

        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-5 space-y-4`}>
          {!selectedTask ? (
            <div className="text-sm text-gray-500">{locale === 'zh-CN' ? '请选择一条任务' : 'Select a task'}</div>
          ) : (
            <>
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2 text-gray-900 dark:text-white font-semibold">
                    {selectedTask.status === 'running' || detailLoading ? <Loader2 size={17} className="animate-spin text-blue-500" /> : selectedTask.status === 'failed' ? <AlertTriangle size={17} className="text-red-500" /> : <CheckCircle2 size={17} className="text-emerald-500" />}
                    <span className="truncate">{selectedTask.name}</span>
                  </div>
                  <div className="mt-1 text-xs text-gray-500">{selectedTask.id} · {selectedTask.type}</div>
                </div>
                <span className={`rounded-full px-2.5 py-1 text-[10px] font-semibold uppercase ${statusTone(selectedTask.status)}`}>
                  {selectedTask.status}
                </span>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-sm">
                <div className="rounded-xl border border-gray-100 bg-gray-50/70 px-4 py-3 dark:border-gray-700/50 dark:bg-gray-900/40">
                  <div className="text-[11px] uppercase tracking-wider text-gray-500">Progress</div>
                  <div className="mt-1 text-gray-900 dark:text-white">{selectedTask.progress}%</div>
                  <div className="mt-3 h-2 rounded-full bg-slate-100 dark:bg-slate-800">
                    <div className="h-2 rounded-full bg-blue-500 transition-all" style={{ width: `${progressWidth}%` }} />
                  </div>
                </div>
                <div className="rounded-xl border border-gray-100 bg-gray-50/70 px-4 py-3 dark:border-gray-700/50 dark:bg-gray-900/40">
                  <div className="text-[11px] uppercase tracking-wider text-gray-500">Updated</div>
                  <div className="mt-1 text-gray-900 dark:text-white break-all">{selectedTask.updatedAt}</div>
                </div>
              </div>

              {selectedTask.error && (
                <div className="rounded-xl border border-red-100 bg-red-50/70 px-4 py-3 text-sm text-red-700 dark:border-red-900/30 dark:bg-red-900/10 dark:text-red-300">
                  {selectedTask.error}
                </div>
              )}

              <div className="rounded-2xl bg-gray-950 px-4 py-4 font-mono text-xs leading-6 text-gray-200">
                <div className="mb-3 flex items-center gap-2 text-gray-400">
                  <TerminalSquare size={14} />
                  <span>{locale === 'zh-CN' ? '任务日志' : 'Task Log'}</span>
                </div>
                {(selectedTask.log || []).length === 0 ? (
                  <div className="text-gray-500">{locale === 'zh-CN' ? '当前没有可显示日志' : 'No task log available'}</div>
                ) : (
                  (selectedTask.log || []).map((line, index) => (
                    <div key={`${index}-${line.slice(0, 24)}`} className="whitespace-pre-wrap break-all py-px">
                      <span className="mr-3 text-gray-600">{String(index + 1).padStart(3, '0')}</span>
                      <span>{line}</span>
                    </div>
                  ))
                )}
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

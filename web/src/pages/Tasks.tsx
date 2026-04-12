import { useEffect, useMemo, useState } from 'react';
import { useOutletContext } from 'react-router-dom';
import { Activity, AlertTriangle, Clock3, Loader2, RefreshCw } from 'lucide-react';
import { api } from '../lib/api';

type OpenClawTask = {
  taskId: string;
  runtime: string;
  task: string;
  label?: string;
  status: string;
  deliveryStatus: string;
  createdAt: number;
  startedAt?: number;
  endedAt?: number;
  lastEventAt?: number;
  progressSummary?: string;
  terminalSummary?: string;
  error?: string;
  agentId?: string;
  childSessionKey?: string;
  runId?: string;
};

type TaskPressure = {
  total?: number;
  active?: number;
  failures?: number;
  visible?: number;
  byStatus?: Record<string, number>;
  byRuntime?: Record<string, number>;
};

function formatTime(ms?: number) {
  if (!ms) return '-';
  return new Date(ms).toLocaleString();
}

function runtimeLabel(runtime: string) {
  switch (runtime) {
    case 'subagent': return 'Subagent';
    case 'acp': return 'ACP';
    case 'cli': return 'CLI';
    case 'cron': return 'Cron';
    default: return runtime || '-';
  }
}

function statusTone(status: string) {
  if (status === 'running') return 'text-blue-600 bg-blue-50 dark:bg-blue-900/20 dark:text-blue-300';
  if (status === 'queued') return 'text-amber-700 bg-amber-50 dark:bg-amber-900/20 dark:text-amber-300';
  if (status === 'succeeded') return 'text-emerald-600 bg-emerald-50 dark:bg-emerald-900/20 dark:text-emerald-300';
  if (status === 'failed' || status === 'timed_out' || status === 'lost') return 'text-red-600 bg-red-50 dark:bg-red-900/20 dark:text-red-300';
  return 'text-gray-600 bg-gray-100 dark:bg-gray-800 dark:text-gray-300';
}

export default function TasksPage() {
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const modern = uiMode === 'modern';
  const [loading, setLoading] = useState(true);
  const [tasks, setTasks] = useState<OpenClawTask[]>([]);
  const [allTasks, setAllTasks] = useState<OpenClawTask[]>([]);
  const [taskPressure, setTaskPressure] = useState<TaskPressure>({});
  const [error, setError] = useState('');

  const load = async () => {
    setLoading(true);
    setError('');
    try {
      const res = await api.getOpenClawTasks();
      if (!res?.ok) {
        setError(res?.error || '加载任务账本失败');
        return;
      }
      setTasks(Array.isArray(res.tasks) ? res.tasks : []);
      setAllTasks(Array.isArray(res.allTasks) ? res.allTasks : []);
      setTaskPressure(res.taskPressure || {});
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
    const timer = setInterval(load, 10000);
    return () => clearInterval(timer);
  }, []);

  const queued = Number(taskPressure.byStatus?.queued || 0);
  const running = Number(taskPressure.byStatus?.running || 0);
  const failures = Number(taskPressure.failures || 0);
  const runtimeSummary = useMemo(() => {
    const entries = Object.entries(taskPressure.byRuntime || {}).filter(([, count]) => Number(count) > 0);
    if (entries.length === 0) return '暂无后台任务';
    return entries.map(([key, count]) => `${runtimeLabel(key)} ${count}`).join(' · ');
  }, [taskPressure.byRuntime]);

  return (
    <div className={`space-y-6 ${modern ? 'p-0' : 'p-2'}`}>
      <div className="flex items-center justify-between">
        <div>
          <h2 className={`font-bold tracking-tight ${modern ? 'text-2xl text-slate-900 dark:text-white' : 'text-xl text-gray-900 dark:text-white'}`}>后台任务</h2>
          <p className={`text-sm mt-1 ${modern ? 'text-slate-500' : 'text-gray-500'}`}>这里展示的是 OpenClaw 自己的任务账本，不是面板安装任务。</p>
        </div>
        <button
          onClick={load}
          className={`${modern ? 'page-modern-action px-3 py-2 text-xs' : 'px-3 py-2 rounded-lg bg-gray-100 dark:bg-gray-800 text-xs text-gray-700 dark:text-gray-200 hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors'} inline-flex items-center gap-2`}
        >
          <RefreshCw size={14} />
          刷新
        </button>
      </div>

      <div className={`grid gap-4 ${modern ? 'grid-cols-1 md:grid-cols-2 xl:grid-cols-4' : 'grid-cols-2 xl:grid-cols-4'}`}>
        <TaskStatCard modern={modern} icon={Clock3} label="活动任务" value={queued + running} sub={`${queued} 排队 · ${running} 运行中`} />
        <TaskStatCard modern={modern} icon={AlertTriangle} label="异常任务" value={failures} sub={failures > 0 ? '需要人工关注' : '暂无异常'} danger={failures > 0} />
        <TaskStatCard modern={modern} icon={Activity} label="可见任务" value={Number(taskPressure.visible || 0)} sub="按 OpenClaw 状态卡规则裁剪" />
        <TaskStatCard modern={modern} icon={RefreshCw} label="运行时分布" value={Number(taskPressure.total || 0)} sub={runtimeSummary} />
      </div>

      {error && (
        <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800/40 dark:bg-red-900/10 dark:text-red-300">
          {error}
        </div>
      )}

      <div className={`${modern ? 'rounded-[28px] border border-white/60 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.84),rgba(248,250,252,0.7))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.92),rgba(51,65,85,0.22))] backdrop-blur-xl shadow-[0_22px_48px_rgba(15,23,42,0.06)]' : 'bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50'} overflow-hidden`}>
        <div className="px-5 py-4 border-b border-slate-200/60 dark:border-slate-700/50 flex items-center justify-between">
          <div>
            <h3 className="text-sm font-bold text-gray-900 dark:text-white">当前可见任务</h3>
            <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">有活动任务时优先展示活动项，否则只展示最近终态任务。</p>
          </div>
          {loading && <Loader2 size={16} className="animate-spin text-gray-400" />}
        </div>
        <div className="divide-y divide-slate-200/60 dark:divide-slate-700/50">
          {tasks.length === 0 && !loading && (
            <div className="px-5 py-10 text-center text-sm text-gray-500 dark:text-gray-400">当前没有可见的 OpenClaw 后台任务。</div>
          )}
          {tasks.map(task => {
            const title = task.label || task.task || task.taskId;
            const detail = task.error || task.progressSummary || task.terminalSummary || '无附加详情';
            return (
              <div key={task.taskId} className="px-5 py-4 space-y-3">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="text-sm font-semibold text-gray-900 dark:text-white truncate">{title}</div>
                    <div className="mt-1 text-xs text-gray-500 dark:text-gray-400 break-all">{detail}</div>
                  </div>
                  <span className={`shrink-0 rounded-full px-2.5 py-1 text-[10px] font-semibold ${statusTone(task.status)}`}>{task.status}</span>
                </div>
                <div className="grid grid-cols-2 xl:grid-cols-4 gap-3 text-xs">
                  <MetaItem label="Task ID" value={task.taskId} mono />
                  <MetaItem label="Runtime" value={runtimeLabel(task.runtime)} />
                  <MetaItem label="Delivery" value={task.deliveryStatus || '-'} />
                  <MetaItem label="Agent" value={task.agentId || '-'} />
                  <MetaItem label="Created" value={formatTime(task.createdAt)} />
                  <MetaItem label="Last Event" value={formatTime(task.lastEventAt)} />
                  <MetaItem label="Child Session" value={task.childSessionKey || '-'} mono />
                  <MetaItem label="Run ID" value={task.runId || '-'} mono />
                </div>
              </div>
            );
          })}
        </div>
      </div>

      <details className={`${modern ? 'rounded-[28px] border border-white/60 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.84),rgba(248,250,252,0.7))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.92),rgba(51,65,85,0.22))] backdrop-blur-xl shadow-[0_22px_48px_rgba(15,23,42,0.06)]' : 'bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50'} overflow-hidden`}>
        <summary className="px-5 py-4 cursor-pointer text-sm font-semibold text-gray-900 dark:text-white">完整账本（最近 {allTasks.length} 条）</summary>
        <div className="border-t border-slate-200/60 dark:border-slate-700/50 px-5 py-4">
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="text-left text-gray-500 dark:text-gray-400">
                  <th className="py-2 pr-4">Task</th>
                  <th className="py-2 pr-4">Status</th>
                  <th className="py-2 pr-4">Runtime</th>
                  <th className="py-2 pr-4">Created</th>
                  <th className="py-2 pr-4">Detail</th>
                </tr>
              </thead>
              <tbody>
                {allTasks.map(task => (
                  <tr key={`all-${task.taskId}`} className="border-t border-slate-100 dark:border-slate-800">
                    <td className="py-2 pr-4 font-mono text-[11px] text-gray-700 dark:text-gray-300">{task.taskId}</td>
                    <td className="py-2 pr-4">{task.status}</td>
                    <td className="py-2 pr-4">{runtimeLabel(task.runtime)}</td>
                    <td className="py-2 pr-4">{formatTime(task.createdAt)}</td>
                    <td className="py-2 pr-4 text-gray-500 dark:text-gray-400">{task.error || task.progressSummary || task.terminalSummary || task.label || task.task}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </details>
    </div>
  );
}

function TaskStatCard({ modern, icon: Icon, label, value, sub, danger }: { modern: boolean; icon: any; label: string; value: number; sub: string; danger?: boolean }) {
  return (
    <div className={`${modern ? 'rounded-[28px] border border-white/60 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.84),rgba(248,250,252,0.7))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.92),rgba(51,65,85,0.22))] backdrop-blur-xl shadow-[0_22px_48px_rgba(15,23,42,0.06)]' : 'bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50'} p-5`}>
      <div className="flex items-center justify-between">
        <div className={`rounded-2xl p-2 ${danger ? 'bg-red-50 text-red-600 dark:bg-red-900/20 dark:text-red-300' : 'bg-blue-50 text-blue-600 dark:bg-blue-900/20 dark:text-blue-300'}`}>
          <Icon size={16} />
        </div>
        <div className={`text-2xl font-bold ${danger ? 'text-red-600 dark:text-red-300' : 'text-gray-900 dark:text-white'}`}>{value}</div>
      </div>
      <div className="mt-4 text-sm font-semibold text-gray-900 dark:text-white">{label}</div>
      <div className="mt-1 text-xs text-gray-500 dark:text-gray-400">{sub}</div>
    </div>
  );
}

function MetaItem({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="space-y-1">
      <div className="text-[10px] uppercase tracking-wider text-gray-400">{label}</div>
      <div className={`${mono ? 'font-mono text-[11px]' : ''} text-gray-700 dark:text-gray-300 break-all`}>{value}</div>
    </div>
  );
}

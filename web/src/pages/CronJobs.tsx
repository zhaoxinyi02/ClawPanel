import { useEffect, useState } from 'react';
import { api } from '../lib/api';
import {
  Clock, Plus, Play, Pause, Trash2, Edit3, RefreshCw,
  CheckCircle2, XCircle, AlertCircle, ChevronDown, ChevronRight,
} from 'lucide-react';

interface CronJob {
  id: string;
  name: string;
  description?: string;
  enabled: boolean;
  schedule: { kind: string; expr?: string; everyMs?: number; atMs?: number; tz?: string };
  sessionTarget: string;
  wakeMode: string;
  payload: { kind: string; text?: string; message?: string; deliver?: boolean; channel?: string; to?: string };
  state: { nextRunAtMs?: number; lastRunAtMs?: number; lastStatus?: string; lastError?: string; lastDurationMs?: number };
  createdAtMs: number;
}

export default function CronJobs() {
  const [jobs, setJobs] = useState<CronJob[]>([]);
  const [loading, setLoading] = useState(true);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [msg, setMsg] = useState('');

  // New job form
  const [newName, setNewName] = useState('');
  const [newCron, setNewCron] = useState('0 9 * * *');
  const [newMessage, setNewMessage] = useState('');
  const [newDeliver, setNewDeliver] = useState(true);

  useEffect(() => { loadJobs(); }, []);

  const loadJobs = async () => {
    setLoading(true);
    try {
      const r = await api.getOpenClawConfig();
      if (r.ok && r.config?.cron?.jobs) {
        setJobs(r.config.cron.jobs);
      } else {
        setJobs([]);
      }
    } catch { setJobs([]); }
    finally { setLoading(false); }
  };

  const toggleJob = async (id: string) => {
    const job = jobs.find(j => j.id === id);
    if (!job) return;
    const updated = jobs.map(j => j.id === id ? { ...j, enabled: !j.enabled } : j);
    setJobs(updated);
    try {
      const r = await api.getOpenClawConfig();
      if (r.ok) {
        const config = r.config || {};
        if (!config.cron) config.cron = {};
        config.cron.jobs = updated;
        await api.updateOpenClawConfig(config);
        setMsg(`${job.name} 已${!job.enabled ? '启用' : '暂停'}`);
        setTimeout(() => setMsg(''), 2000);
      }
    } catch {
      setJobs(jobs);
      setMsg('操作失败');
      setTimeout(() => setMsg(''), 2000);
    }
  };

  const deleteJob = async (id: string) => {
    if (!confirm('确定删除此定时任务？')) return;
    const updated = jobs.filter(j => j.id !== id);
    setJobs(updated);
    try {
      const r = await api.getOpenClawConfig();
      if (r.ok) {
        const config = r.config || {};
        if (!config.cron) config.cron = {};
        config.cron.jobs = updated;
        await api.updateOpenClawConfig(config);
        setMsg('已删除');
        setTimeout(() => setMsg(''), 2000);
      }
    } catch {
      loadJobs();
      setMsg('删除失败');
      setTimeout(() => setMsg(''), 2000);
    }
  };

  const createJob = async () => {
    if (!newName.trim() || !newMessage.trim()) {
      setMsg('请填写任务名称和消息内容');
      setTimeout(() => setMsg(''), 2000);
      return;
    }
    const job: CronJob = {
      id: 'cron_' + Date.now(),
      name: newName.trim(),
      enabled: true,
      schedule: { kind: 'cron', expr: newCron },
      sessionTarget: 'main',
      wakeMode: 'now',
      payload: { kind: 'agentTurn', message: newMessage.trim(), deliver: newDeliver },
      state: {},
      createdAtMs: Date.now(),
    };
    const updated = [...jobs, job];
    setJobs(updated);
    try {
      const r = await api.getOpenClawConfig();
      if (r.ok) {
        const config = r.config || {};
        if (!config.cron) config.cron = {};
        config.cron.jobs = updated;
        await api.updateOpenClawConfig(config);
        setMsg('创建成功');
        setShowCreate(false);
        setNewName('');
        setNewMessage('');
        setTimeout(() => setMsg(''), 2000);
      }
    } catch {
      loadJobs();
      setMsg('创建失败');
      setTimeout(() => setMsg(''), 2000);
    }
  };

  const formatSchedule = (s: CronJob['schedule']) => {
    if (s.kind === 'cron') return `Cron: ${s.expr}${s.tz ? ` (${s.tz})` : ''}`;
    if (s.kind === 'every') return `每 ${Math.round((s.everyMs || 0) / 60000)} 分钟`;
    if (s.kind === 'at') return `一次性: ${new Date(s.atMs || 0).toLocaleString()}`;
    return JSON.stringify(s);
  };

  const statusIcon = (s?: string) => {
    if (s === 'ok') return <CheckCircle2 size={13} className="text-emerald-500" />;
    if (s === 'error') return <XCircle size={13} className="text-red-500" />;
    if (s === 'skipped') return <AlertCircle size={13} className="text-amber-500" />;
    return <Clock size={13} className="text-gray-400" />;
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold">定时任务</h2>
          <p className="text-xs text-gray-500 mt-0.5">创建和管理 OpenClaw 的定时执行任务</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={loadJobs} className="flex items-center gap-1.5 px-2.5 py-1.5 text-xs rounded-lg border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800 text-gray-600 dark:text-gray-400">
            <RefreshCw size={13} />刷新
          </button>
          <button onClick={() => setShowCreate(!showCreate)}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700">
            <Plus size={13} />新建任务
          </button>
        </div>
      </div>

      {msg && (
        <div className={`px-3 py-2 rounded-lg text-xs ${msg.includes('失败') ? 'bg-red-50 dark:bg-red-950 text-red-600' : 'bg-emerald-50 dark:bg-emerald-950 text-emerald-600'}`}>
          {msg}
        </div>
      )}

      {/* Create form */}
      {showCreate && (
        <div className="card p-4 space-y-3 border-2 border-violet-200 dark:border-violet-800">
          <h3 className="font-semibold text-sm">新建定时任务</h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">任务名称</label>
              <input value={newName} onChange={e => setNewName(e.target.value)} placeholder="如: 每日早报"
                className="w-full px-3 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-transparent" />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">Cron 表达式</label>
              <input value={newCron} onChange={e => setNewCron(e.target.value)} placeholder="0 9 * * *"
                className="w-full px-3 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-transparent font-mono" />
              <p className="text-[10px] text-gray-400 mt-0.5">分 时 日 月 周 (如 0 9 * * * = 每天9点)</p>
            </div>
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1">消息内容</label>
            <textarea value={newMessage} onChange={e => setNewMessage(e.target.value)} placeholder="发送给 Agent 的消息..."
              rows={3} className="w-full px-3 py-2 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-transparent resize-none" />
          </div>
          <div className="flex items-center gap-4">
            <label className="flex items-center gap-2 text-xs">
              <input type="checkbox" checked={newDeliver} onChange={e => setNewDeliver(e.target.checked)} className="rounded" />
              发送结果到通道
            </label>
          </div>
          <div className="flex gap-2">
            <button onClick={createJob} className="px-4 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700">创建</button>
            <button onClick={() => setShowCreate(false)} className="px-4 py-2 text-xs rounded-lg border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800">取消</button>
          </div>
        </div>
      )}

      {/* Jobs list */}
      {loading ? (
        <div className="text-center py-12 text-gray-400 text-xs">加载中...</div>
      ) : jobs.length === 0 ? (
        <div className="card p-8 text-center">
          <Clock size={32} className="mx-auto text-gray-300 dark:text-gray-600 mb-3" />
          <h3 className="font-semibold text-sm mb-1">暂无定时任务</h3>
          <p className="text-xs text-gray-500">点击"新建任务"创建你的第一个定时任务</p>
        </div>
      ) : (
        <div className="space-y-2">
          {jobs.map(job => (
            <div key={job.id} className="card">
              <div className="flex items-center gap-3 px-4 py-3 cursor-pointer" onClick={() => setExpandedId(expandedId === job.id ? null : job.id)}>
                {expandedId === job.id ? <ChevronDown size={14} className="text-gray-400" /> : <ChevronRight size={14} className="text-gray-400" />}
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium">{job.name}</span>
                    <span className={`text-[10px] px-1.5 py-0.5 rounded font-medium ${job.enabled ? 'bg-emerald-50 dark:bg-emerald-950 text-emerald-600' : 'bg-gray-100 dark:bg-gray-800 text-gray-500'}`}>
                      {job.enabled ? '运行中' : '已暂停'}
                    </span>
                  </div>
                  <div className="flex items-center gap-3 text-[11px] text-gray-500 mt-0.5">
                    <span>{formatSchedule(job.schedule)}</span>
                    {job.state.lastRunAtMs && (
                      <span className="flex items-center gap-1">
                        {statusIcon(job.state.lastStatus)}
                        上次: {new Date(job.state.lastRunAtMs).toLocaleString()}
                      </span>
                    )}
                  </div>
                </div>
                <div className="flex items-center gap-1 shrink-0" onClick={e => e.stopPropagation()}>
                  <button onClick={() => toggleJob(job.id)} className={`p-1.5 rounded hover:bg-gray-100 dark:hover:bg-gray-800 ${job.enabled ? 'text-emerald-500' : 'text-gray-400'}`} title={job.enabled ? '暂停' : '启用'}>
                    {job.enabled ? <Pause size={14} /> : <Play size={14} />}
                  </button>
                  <button onClick={() => deleteJob(job.id)} className="p-1.5 rounded hover:bg-gray-100 dark:hover:bg-gray-800 text-red-400" title="删除">
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>
              {expandedId === job.id && (
                <div className="px-4 pb-3 pt-0 border-t border-gray-100 dark:border-gray-800 mt-0">
                  <div className="grid grid-cols-2 gap-3 text-xs pt-3">
                    <div><span className="text-gray-400">会话目标:</span> <span className="ml-1">{job.sessionTarget}</span></div>
                    <div><span className="text-gray-400">唤醒模式:</span> <span className="ml-1">{job.wakeMode}</span></div>
                    <div><span className="text-gray-400">类型:</span> <span className="ml-1">{job.payload.kind}</span></div>
                    {job.payload.deliver !== undefined && <div><span className="text-gray-400">发送结果:</span> <span className="ml-1">{job.payload.deliver ? '是' : '否'}</span></div>}
                    {job.state.lastError && <div className="col-span-2 text-red-500"><span className="text-gray-400">错误:</span> {job.state.lastError}</div>}
                    {(job.payload.text || job.payload.message) && (
                      <div className="col-span-2">
                        <span className="text-gray-400">消息:</span>
                        <pre className="mt-1 p-2 rounded bg-gray-50 dark:bg-gray-800 text-[11px] whitespace-pre-wrap">{job.payload.text || job.payload.message}</pre>
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

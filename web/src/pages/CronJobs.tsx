import { useEffect, useState } from 'react';
import { api } from '../lib/api';
import {
  Clock, Plus, Play, Pause, Trash2, Edit3, RefreshCw,
  CheckCircle2, XCircle, AlertCircle, ChevronDown, ChevronRight,
} from 'lucide-react';
import { useI18n } from '../i18n';

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
  const { t } = useI18n();
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
      const r = await api.getCronJobs();
      if (r.ok && r.jobs) {
        setJobs(r.jobs);
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
      await api.updateCronJobs(updated);
      setMsg(`${job.name} ${!job.enabled ? t.common.enabled : t.common.paused}`);
      setTimeout(() => setMsg(''), 2000);
    } catch {
      setJobs(jobs);
      setMsg(t.common.operationFailed);
      setTimeout(() => setMsg(''), 2000);
    }
  };

  const deleteJob = async (id: string) => {
    if (!confirm(t.cron.deleteConfirm)) return;
    const updated = jobs.filter(j => j.id !== id);
    setJobs(updated);
    try {
      await api.updateCronJobs(updated);
      setMsg(t.cron.deleted);
      setTimeout(() => setMsg(''), 2000);
    } catch {
      loadJobs();
      setMsg(t.cron.deleteFailed);
      setTimeout(() => setMsg(''), 2000);
    }
  };

  const createJob = async () => {
    if (!newName.trim() || !newMessage.trim()) {
      setMsg(t.cron.fillRequired);
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
      await api.updateCronJobs(updated);
      setMsg(t.cron.createSuccess);
      setShowCreate(false);
      setNewName('');
      setNewMessage('');
      setTimeout(() => setMsg(''), 2000);
    } catch {
      loadJobs();
      setMsg(t.cron.createFailed);
      setTimeout(() => setMsg(''), 2000);
    }
  };

  const formatSchedule = (s: CronJob['schedule']) => {
    if (s.kind === 'cron') return `Cron: ${s.expr}${s.tz ? ` (${s.tz})` : ''}`;
    if (s.kind === 'every') return t.cron.everyMinutes.replace('{n}', String(Math.round((s.everyMs || 0) / 60000)));
    if (s.kind === 'at') return `${t.cron.oneTime}: ${new Date(s.atMs || 0).toLocaleString()}`;
    return JSON.stringify(s);
  };

  const statusIcon = (s?: string) => {
    if (s === 'ok') return <CheckCircle2 size={13} className="text-emerald-500" />;
    if (s === 'error') return <XCircle size={13} className="text-red-500" />;
    if (s === 'skipped') return <AlertCircle size={13} className="text-amber-500" />;
    return <Clock size={13} className="text-gray-400" />;
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-bold text-gray-900 dark:text-white tracking-tight">{t.cron.title}</h2>
          <p className="text-sm text-gray-500 mt-1">{t.cron.subtitle}</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={loadJobs} className="flex items-center gap-2 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 transition-colors shadow-sm">
            <RefreshCw size={14} />{t.cron.refreshList}
          </button>
          <button onClick={() => setShowCreate(!showCreate)}
            className="flex items-center gap-2 px-4 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 shadow-sm shadow-violet-200 dark:shadow-none transition-all hover:shadow-md hover:shadow-violet-200 dark:hover:shadow-none">
            <Plus size={14} />{t.cron.newJob}
          </button>
        </div>
      </div>

      {msg && (
        <div className={`px-4 py-3 rounded-xl text-sm font-medium flex items-center gap-2 ${msg.includes('失败') ? 'bg-red-50 dark:bg-red-900/30 text-red-600' : 'bg-emerald-50 dark:bg-emerald-900/30 text-emerald-600'}`}>
          {msg.includes('失败') ? <XCircle size={16} /> : <CheckCircle2 size={16} />}
          {msg}
        </div>
      )}

      {/* Create form */}
      {showCreate && (
        <div className="bg-white dark:bg-gray-800 rounded-xl shadow-lg border border-violet-100 dark:border-violet-900/30 p-6 space-y-5 animate-in fade-in slide-in-from-top-4 duration-200">
          <div className="flex items-center gap-2 pb-4 border-b border-gray-100 dark:border-gray-700/50">
            <div className="p-1.5 rounded-lg bg-violet-100 dark:bg-violet-900/30 text-violet-600">
              <Plus size={16} />
            </div>
            <h3 className="font-bold text-gray-900 dark:text-white">{t.cron.createJob}</h3>
          </div>
          
          <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
            <div className="space-y-1.5">
              <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300">{t.cron.jobName}</label>
              <input value={newName} onChange={e => setNewName(e.target.value)} placeholder={t.cron.jobNamePlaceholder}
                className="w-full px-3.5 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500 transition-all" />
            </div>
            <div className="space-y-1.5">
              <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300">{t.cron.cronExpr}</label>
              <div className="relative">
                <input value={newCron} onChange={e => setNewCron(e.target.value)} placeholder="0 9 * * *"
                  className="w-full px-3.5 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500 transition-all font-mono" />
                <div className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400">
                  <Clock size={14} />
                </div>
              </div>
              <p className="text-[10px] text-gray-400">{t.cron.cronHelp}</p>
            </div>
          </div>
          
          <div className="space-y-1.5">
            <label className="block text-xs font-semibold text-gray-700 dark:text-gray-300">{t.cron.messageContent}</label>
            <textarea value={newMessage} onChange={e => setNewMessage(e.target.value)} placeholder={t.cron.messagePlaceholder}
              rows={3} className="w-full px-3.5 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500 transition-all resize-none" />
          </div>
          
          <div className="flex items-center justify-between pt-2">
            <label className="flex items-center gap-2.5 cursor-pointer group">
              <input type="checkbox" checked={newDeliver} onChange={e => setNewDeliver(e.target.checked)} 
                className="w-4 h-4 text-violet-600 rounded border-gray-300 focus:ring-violet-500 transition-colors" />
              <span className="text-sm text-gray-600 dark:text-gray-400 group-hover:text-gray-900 dark:group-hover:text-gray-200 transition-colors">{t.cron.deliverToChannel}</span>
            </label>
            <div className="flex gap-3">
              <button onClick={() => setShowCreate(false)} className="px-4 py-2 text-sm font-medium text-gray-600 dark:text-gray-300 bg-gray-100 dark:bg-gray-800 hover:bg-gray-200 dark:hover:bg-gray-700 rounded-lg transition-colors">{t.common.cancel}</button>
              <button onClick={createJob} className="px-6 py-2 text-sm font-medium bg-violet-600 text-white hover:bg-violet-700 rounded-lg shadow-sm shadow-violet-200 dark:shadow-none transition-all hover:shadow-md hover:shadow-violet-200 dark:hover:shadow-none">{t.cron.createNow}</button>
            </div>
          </div>
        </div>
      )}

      {/* Jobs list */}
      {loading ? (
        <div className="flex flex-col items-center justify-center py-16 text-gray-400 gap-3">
          <RefreshCw size={32} className="animate-spin text-violet-500/50" />
          <p className="text-sm">{t.cron.loadingJobs}</p>
        </div>
      ) : jobs.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-gray-400 border-2 border-dashed border-gray-100 dark:border-gray-800 rounded-xl">
          <Clock size={32} className="opacity-20 mb-2" />
          <h3 className="font-semibold text-sm text-gray-500">{t.cron.noJobs}</h3>
          <p className="text-xs text-gray-400 mt-1">{t.cron.noJobsHint}</p>
        </div>
      ) : (
        <div className="grid gap-3">
          {jobs.map(job => (
            <div key={job.id} className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 hover:shadow-md transition-all group overflow-hidden">
              <div className="flex items-center gap-4 p-4 cursor-pointer" onClick={() => setExpandedId(expandedId === job.id ? null : job.id)}>
                <div className={`p-2.5 rounded-xl shrink-0 transition-colors ${job.enabled ? 'bg-violet-100 dark:bg-violet-900/30 text-violet-600' : 'bg-gray-100 dark:bg-gray-800 text-gray-400'}`}>
                  <Clock size={20} />
                </div>
                
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-3 mb-1">
                    <span className="text-base font-bold text-gray-900 dark:text-white truncate">{job.name}</span>
                    <span className={`text-[10px] px-2 py-0.5 rounded-full font-bold uppercase tracking-wider ${job.enabled ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-400' : 'bg-gray-100 dark:bg-gray-700 text-gray-500'}`}>
                      {job.enabled ? t.common.running : t.common.paused}
                    </span>
                  </div>
                  <div className="flex items-center gap-4 text-xs text-gray-500">
                    <span className="font-mono bg-gray-50 dark:bg-gray-900/50 px-1.5 py-0.5 rounded border border-gray-100 dark:border-gray-800">{formatSchedule(job.schedule)}</span>
                    {job.state.lastRunAtMs && (
                      <span className="flex items-center gap-1.5">
                        {statusIcon(job.state.lastStatus)}
                        {t.cron.lastRun}: {new Date(job.state.lastRunAtMs).toLocaleString()}
                      </span>
                    )}
                  </div>
                </div>
                
                <div className="flex items-center gap-2 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity" onClick={e => e.stopPropagation()}>
                  <button onClick={() => toggleJob(job.id)} 
                    className={`p-2 rounded-lg transition-colors ${job.enabled ? 'text-amber-500 hover:bg-amber-50 dark:hover:bg-amber-900/20' : 'text-emerald-500 hover:bg-emerald-50 dark:hover:bg-emerald-900/20'}`} 
                    title={job.enabled ? t.common.paused : t.common.enabled}>
                    {job.enabled ? <Pause size={16} /> : <Play size={16} />}
                  </button>
                  <button onClick={() => deleteJob(job.id)} className="p-2 rounded-lg text-red-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors" title={t.common.delete}>
                    <Trash2 size={16} />
                  </button>
                </div>
                
                <ChevronDown size={16} className={`text-gray-300 transition-transform duration-200 ${expandedId === job.id ? 'rotate-180 text-gray-500' : ''}`} />
              </div>
              
              {expandedId === job.id && (
                <div className="px-6 pb-6 pt-2 border-t border-gray-50 dark:border-gray-800 bg-gray-50/30 dark:bg-gray-900/20">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-y-4 gap-x-8 text-sm">
                    <div className="space-y-1">
                      <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">{t.cron.sessionTarget}</span>
                      <div className="font-mono text-gray-700 dark:text-gray-300">{job.sessionTarget}</div>
                    </div>
                    <div className="space-y-1">
                      <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">{t.cron.wakeMode}</span>
                      <div className="font-mono text-gray-700 dark:text-gray-300">{job.wakeMode}</div>
                    </div>
                    <div className="space-y-1">
                      <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">{t.cron.jobType}</span>
                      <div className="font-mono text-gray-700 dark:text-gray-300">{job.payload.kind}</div>
                    </div>
                    {job.payload.deliver !== undefined && (
                      <div className="space-y-1">
                        <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">{t.cron.resultPush}</span>
                        <div className={`font-medium ${job.payload.deliver ? 'text-emerald-600' : 'text-gray-500'}`}>{job.payload.deliver ? t.common.on : t.common.off}</div>
                      </div>
                    )}
                    
                    {job.state.lastError && (
                      <div className="md:col-span-2 bg-red-50 dark:bg-red-900/20 p-3 rounded-lg border border-red-100 dark:border-red-900/30">
                        <span className="text-xs font-bold text-red-500 uppercase tracking-wider block mb-1">{t.cron.execError}</span>
                        <div className="text-xs text-red-600 dark:text-red-400 font-mono break-all">{job.state.lastError}</div>
                      </div>
                    )}
                    
                    {(job.payload.text || job.payload.message) && (
                      <div className="md:col-span-2 space-y-1.5">
                        <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">{t.cron.messageContent}</span>
                        <div className="p-3 rounded-lg bg-white dark:bg-gray-900 border border-gray-100 dark:border-gray-800 text-xs font-mono text-gray-600 dark:text-gray-300 whitespace-pre-wrap shadow-sm">
                          {job.payload.text || job.payload.message}
                        </div>
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

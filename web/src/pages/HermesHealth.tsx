import { useEffect, useState } from 'react';
import { useNavigate, useOutletContext } from 'react-router-dom';
import { api } from '../lib/api';
import { AlertTriangle, CheckCircle2, RefreshCw, Wrench, Stethoscope } from 'lucide-react';
import { useI18n } from '../i18n';

interface HermesIssue {
  id: string;
  severity: string;
  component: string;
  title: string;
  description: string;
  fixable?: boolean;
}

interface HermesHealthCheck {
  id: string;
  title: string;
  status: string;
  detail?: string;
  fixHint?: string;
}

interface HermesDoctorSnapshot {
  updatedAt?: string;
  fixApplied?: boolean;
  taskId?: string;
  taskStatus?: string;
  error?: string;
  rawLines?: string[];
  statusLines?: string[];
  platforms?: {
    configuredCount?: number;
    enabledCount?: number;
    platforms?: Array<{ id: string; label: string; runtimeStatus?: string; lastError?: string }>;
  };
}

export default function HermesHealth() {
  const { locale } = useI18n();
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const navigate = useNavigate();
  const modern = uiMode === 'modern';
  const [loading, setLoading] = useState(true);
  const [doctorRunning, setDoctorRunning] = useState(false);
  const [issues, setIssues] = useState<HermesIssue[]>([]);
  const [checks, setChecks] = useState<HermesHealthCheck[]>([]);
  const [doctor, setDoctor] = useState<HermesDoctorSnapshot | null>(null);
  const [checkedCount, setCheckedCount] = useState(0);
  const [problems, setProblems] = useState(0);
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');

  const load = async () => {
    setLoading(true);
    setErr('');
    try {
      const [healthRes, checkRes, doctorRes] = await Promise.all([
        api.getHermesHealth(),
        api.checkHermes(),
        api.getHermesDoctorSnapshot(),
      ]);
      if (healthRes.ok) setChecks(healthRes.health?.checks || []);
      if (checkRes.ok) {
        setIssues(checkRes.issues || []);
        setCheckedCount(checkRes.checked || 0);
        setProblems(checkRes.problems || 0);
      }
      if (doctorRes.ok) setDoctor(doctorRes.snapshot || null);
    } catch {
      setErr(locale === 'zh-CN' ? '加载 Hermes 健康信息失败' : 'Failed to load Hermes health data');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const runDoctor = async (fix = false) => {
    setDoctorRunning(true);
    setMsg('');
    setErr('');
    try {
      const r = await api.runHermesDoctor(fix);
      if (r?.ok) {
        setMsg(fix
          ? (locale === 'zh-CN' ? '已触发 Hermes doctor --fix，请稍后刷新查看结果。' : 'Triggered Hermes doctor --fix. Refresh shortly to see results.')
          : (locale === 'zh-CN' ? '已触发 Hermes doctor，请稍后刷新查看结果。' : 'Triggered Hermes doctor. Refresh shortly to see results.'));
      } else {
        setErr(r?.error || (locale === 'zh-CN' ? '执行 Hermes doctor 失败' : 'Failed to run Hermes doctor'));
      }
    } catch {
      setErr(locale === 'zh-CN' ? '执行 Hermes doctor 失败' : 'Failed to run Hermes doctor');
    } finally {
      setDoctorRunning(false);
    }
  };

  const fixIssue = async (issueId: string) => {
    setMsg('');
    setErr('');
    try {
      const r = await api.fixHermes([issueId]);
      if (r?.ok) {
        setMsg(locale === 'zh-CN' ? `已尝试修复 ${issueId}` : `Attempted fix: ${issueId}`);
        await load();
      } else {
        setErr(r?.error || (locale === 'zh-CN' ? 'Hermes 修复失败' : 'Hermes fix failed'));
      }
    } catch {
      setErr(locale === 'zh-CN' ? 'Hermes 修复失败' : 'Hermes fix failed');
    }
  };

  const toneClass = (status: string) => {
    switch (status) {
      case 'healthy':
        return 'border-emerald-100 dark:border-emerald-900/30';
      case 'warning':
        return 'border-amber-100 dark:border-amber-900/30';
      case 'error':
        return 'border-red-100 dark:border-red-900/30';
      default:
        return 'border-gray-100 dark:border-gray-700/50';
    }
  };

  return (
    <div className={`space-y-6 ${modern ? 'page-modern' : ''}`}>
      <div className={`${modern ? 'page-modern-header' : 'flex items-center justify-between'}`}>
        <div>
          <h2 className={`${modern ? 'page-modern-title text-xl' : 'text-xl font-bold text-gray-900 dark:text-white'}`}>{locale === 'zh-CN' ? 'Hermes 健康与检查' : 'Hermes Health & Checks'}</h2>
          <p className={`${modern ? 'page-modern-subtitle mt-1 text-sm' : 'text-sm text-gray-500 mt-1'}`}>
            {locale === 'zh-CN'
              ? '集中查看 Hermes 的结构化健康快照、检查项和 doctor 输出。'
              : 'Review Hermes health snapshot, structured checks, and doctor output.'}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={() => runDoctor(false)} disabled={doctorRunning} className={`${modern ? 'page-modern-action px-3 py-2 text-xs disabled:opacity-50' : 'px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-800 disabled:opacity-50'} inline-flex items-center gap-2`}>
            <Stethoscope size={14} />
            {locale === 'zh-CN' ? '运行 Doctor' : 'Run Doctor'}
          </button>
          <button onClick={() => runDoctor(true)} disabled={doctorRunning} className={`${modern ? 'page-modern-accent px-3 py-2 text-xs disabled:opacity-50' : 'px-3 py-2 text-xs rounded-lg bg-blue-600 text-white disabled:opacity-50'} inline-flex items-center gap-2`}>
            <Wrench size={14} />
            {locale === 'zh-CN' ? 'Doctor + Fix' : 'Doctor + Fix'}
          </button>
          <button onClick={load} className={`${modern ? 'page-modern-action px-3 py-2 text-xs' : 'px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-800'} inline-flex items-center gap-2`}>
            <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
            {locale === 'zh-CN' ? '刷新' : 'Refresh'}
          </button>
        </div>
      </div>

      {msg && <div className="rounded-2xl border border-emerald-100 bg-emerald-50/80 px-4 py-3 text-sm text-emerald-700 dark:border-emerald-900/30 dark:bg-emerald-900/10 dark:text-emerald-300">{msg}</div>}
      {err && <div className="rounded-2xl border border-red-100 bg-red-50/80 px-4 py-3 text-sm text-red-700 dark:border-red-900/30 dark:bg-red-900/10 dark:text-red-300">{err}</div>}

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {[
          { label: locale === 'zh-CN' ? '检查项总数' : 'Checks', value: String(checkedCount) },
          { label: locale === 'zh-CN' ? '告警 / 错误' : 'Problems', value: String(problems) },
          { label: locale === 'zh-CN' ? 'Doctor 时间' : 'Doctor Updated', value: doctor?.updatedAt || '-' },
        ].map(card => (
          <div key={card.label} className={`${modern ? 'page-modern-card' : 'bg-white dark:bg-gray-800'} rounded-2xl border border-gray-100 dark:border-gray-700/50 p-4`}>
            <div className="text-xs font-semibold uppercase tracking-wider text-gray-500">{card.label}</div>
            <div className="mt-3 text-lg font-bold text-gray-900 dark:text-white break-all">{card.value}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-5 space-y-4`}>
          <div className="text-gray-900 dark:text-white font-semibold">{locale === 'zh-CN' ? '健康快照' : 'Health Snapshot'}</div>
          <div className="space-y-3">
            {checks.map(check => (
              <div key={check.id} className={`rounded-xl border p-4 ${toneClass(check.status)}`}>
                <div className="flex items-center gap-2">
                  {check.status === 'healthy' ? <CheckCircle2 size={16} className="text-emerald-500" /> : <AlertTriangle size={16} className={check.status === 'error' ? 'text-red-500' : 'text-amber-500'} />}
                  <div className="font-medium text-gray-900 dark:text-white">{check.title}</div>
                </div>
                {check.detail && <div className="mt-2 text-sm text-gray-600 dark:text-gray-300">{check.detail}</div>}
                {check.fixHint && <div className="mt-2 text-xs text-gray-500">{check.fixHint}</div>}
              </div>
            ))}
          </div>
        </div>

        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-5 space-y-4`}>
          <div className="text-gray-900 dark:text-white font-semibold">{locale === 'zh-CN' ? '结构化检查项' : 'Structured Issues'}</div>
          <div className="space-y-3">
            {issues.map(issue => (
              <div key={issue.id} className="rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 p-4">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <div className="font-medium text-gray-900 dark:text-white">{issue.title}</div>
                    <div className="mt-1 text-xs text-gray-500">{issue.id}</div>
                  </div>
                  {issue.fixable && (
                    <button onClick={() => fixIssue(issue.id)} className={`${modern ? 'page-modern-action px-3 py-2 text-xs' : 'px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-800'} shrink-0`}>
                      {locale === 'zh-CN' ? '尝试修复' : 'Try Fix'}
                    </button>
                  )}
                </div>
                <div className="mt-2 text-sm text-gray-600 dark:text-gray-300">{issue.description}</div>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-5 space-y-4`}>
          <div className="flex items-center justify-between gap-3">
            <div className="text-gray-900 dark:text-white font-semibold">{locale === 'zh-CN' ? '平台运行摘要' : 'Platform Runtime Summary'}</div>
            <button onClick={() => navigate('/hermes/platforms')} className={`${modern ? 'page-modern-action px-3 py-2 text-xs' : 'px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-800'}`}>
              {locale === 'zh-CN' ? '打开平台页' : 'Open Platforms'}
            </button>
          </div>
          <div className="space-y-3">
            {(doctor?.platforms?.platforms || []).length === 0 ? (
              <div className="text-sm text-gray-500">{locale === 'zh-CN' ? '暂无平台快照' : 'No platform snapshot yet'}</div>
            ) : (doctor?.platforms?.platforms || []).map(platform => (
              <div key={platform.id} className="rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 px-4 py-3">
                <div className="flex items-center justify-between gap-3">
                  <div className="font-medium text-gray-900 dark:text-white">{platform.label}</div>
                  <div className="text-[10px] uppercase tracking-wider text-gray-500">{platform.runtimeStatus || 'unknown'}</div>
                </div>
                {platform.lastError && <div className="mt-2 text-sm text-red-600 dark:text-red-300">{platform.lastError}</div>}
              </div>
            ))}
          </div>
        </div>

        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-5 space-y-4`}>
          <div className="text-gray-900 dark:text-white font-semibold">{locale === 'zh-CN' ? 'Hermes Status 线索' : 'Hermes Status Lines'}</div>
          <pre className="rounded-2xl bg-gray-950 text-gray-100 p-4 overflow-x-auto text-xs leading-6 font-mono">{(doctor?.statusLines || []).join('\n') || 'No status lines yet.'}</pre>
        </div>
      </div>

      <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-5 space-y-4`}>
        <div className="text-gray-900 dark:text-white font-semibold">{locale === 'zh-CN' ? 'Doctor 输出' : 'Doctor Output'}</div>
        <pre className="rounded-2xl bg-gray-950 text-gray-100 p-4 overflow-x-auto text-xs leading-6 font-mono">{(doctor?.rawLines || doctor?.statusLines || []).join('\n') || 'No output yet.'}</pre>
      </div>
    </div>
  );
}

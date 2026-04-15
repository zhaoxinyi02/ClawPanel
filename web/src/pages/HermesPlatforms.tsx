import { useEffect, useState } from 'react';
import { useNavigate, useOutletContext } from 'react-router-dom';
import { api } from '../lib/api';
import { RefreshCw, Save, Play, Wrench, Activity } from 'lucide-react';
import { useI18n } from '../i18n';

interface PlatformStatus {
  id: string;
  label: string;
  configured: boolean;
  enabled: boolean;
  runtimeStatus?: string;
  lastEvidence?: string;
  lastError?: string;
  detail?: string;
}

interface PlatformDetail {
  status: PlatformStatus;
  config: Record<string, any>;
  environment: Record<string, string>;
}

function envToText(env: Record<string, string>) {
  return Object.entries(env || {}).map(([k, v]) => `${k}=${v}`).join('\n');
}

function textToEnv(text: string) {
  const result: Record<string, string> = {};
  for (const line of text.split('\n')) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const idx = trimmed.indexOf('=');
    if (idx === -1) continue;
    const key = trimmed.slice(0, idx).trim();
    const value = trimmed.slice(idx + 1).trim();
    if (key) result[key] = value;
  }
  return result;
}

export default function HermesPlatforms() {
  const { locale } = useI18n();
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const navigate = useNavigate();
  const modern = uiMode === 'modern';
  const [platforms, setPlatforms] = useState<PlatformStatus[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [detail, setDetail] = useState<PlatformDetail | null>(null);
  const [configText, setConfigText] = useState('{}');
  const [envText, setEnvText] = useState('');
  const [enabled, setEnabled] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');
  const [actionRunning, setActionRunning] = useState(false);

  const loadPlatforms = async () => {
    setLoading(true);
    setErr('');
    try {
      const r = await api.getHermesPlatforms();
      if (r.ok) {
        const next = r.platforms?.platforms || [];
        setPlatforms(next);
        setSelectedId(prev => prev || next[0]?.id || '');
      }
    } catch {
      setErr(locale === 'zh-CN' ? '加载 Hermes 平台失败' : 'Failed to load Hermes platforms');
    } finally {
      setLoading(false);
    }
  };

  const loadDetail = async (id: string) => {
    if (!id) return;
    try {
      const r = await api.getHermesPlatformDetail(id);
      if (r.ok) {
        setDetail(r.platform || null);
        setEnabled(Boolean(r.platform?.status?.enabled));
        setConfigText(JSON.stringify(r.platform?.config || {}, null, 2));
        setEnvText(envToText(r.platform?.environment || {}));
      }
    } catch {
      setErr(locale === 'zh-CN' ? '加载 Hermes 平台详情失败' : 'Failed to load Hermes platform detail');
    }
  };

  useEffect(() => { loadPlatforms(); }, []);
  useEffect(() => { if (selectedId) loadDetail(selectedId); }, [selectedId]);

  const save = async () => {
    if (!selectedId) return;
    setSaving(true);
    setErr('');
    setMsg('');
    try {
      const parsed = JSON.parse(configText || '{}');
      const env = textToEnv(envText);
      const r = await api.updateHermesPlatformDetail(selectedId, { enabled, config: parsed, env });
      if (r.ok) {
        setMsg(locale === 'zh-CN' ? '平台配置已保存' : 'Platform configuration saved');
        await loadPlatforms();
        await loadDetail(selectedId);
      } else {
        setErr(r.error || 'Save failed');
      }
    } catch (e) {
      setErr(locale === 'zh-CN' ? `配置格式错误: ${String(e)}` : `Invalid config format: ${String(e)}`);
    } finally {
      setSaving(false);
    }
  };

  const runAction = async (action: string) => {
    setActionRunning(true);
    setMsg('');
    setErr('');
    try {
      const r = await api.runHermesAction(action);
      if (r?.ok) {
        setMsg(locale === 'zh-CN' ? `已触发动作 ${action}` : `Triggered action ${action}`);
      } else {
        setErr(r?.error || 'Action failed');
      }
    } catch {
      setErr(locale === 'zh-CN' ? '执行 Hermes 平台动作失败' : 'Failed to run Hermes platform action');
    } finally {
      setActionRunning(false);
    }
  };

  return (
    <div className={`space-y-6 ${modern ? 'page-modern' : ''}`}>
      <div className={`${modern ? 'page-modern-header' : 'flex items-center justify-between'}`}>
        <div>
          <h2 className={`${modern ? 'page-modern-title text-xl' : 'text-xl font-bold text-gray-900 dark:text-white'}`}>{locale === 'zh-CN' ? 'Hermes 平台管理' : 'Hermes Platforms'}</h2>
          <p className={`${modern ? 'page-modern-subtitle mt-1 text-sm' : 'text-sm text-gray-500 mt-1'}`}>
            {locale === 'zh-CN'
              ? '查看各平台的配置、启用状态和运行线索，并做最小配置编辑。'
              : 'Review platform state, config, and runtime evidence, then apply minimal updates.'}
          </p>
        </div>
        <button onClick={loadPlatforms} className={`${modern ? 'page-modern-action px-3 py-2 text-xs' : 'px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-800'} inline-flex items-center gap-2`}>
          <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          {locale === 'zh-CN' ? '刷新' : 'Refresh'}
        </button>
      </div>

      {msg && <div className="rounded-2xl border border-emerald-100 bg-emerald-50/80 px-4 py-3 text-sm text-emerald-700 dark:border-emerald-900/30 dark:bg-emerald-900/10 dark:text-emerald-300">{msg}</div>}
      {err && <div className="rounded-2xl border border-red-100 bg-red-50/80 px-4 py-3 text-sm text-red-700 dark:border-red-900/30 dark:bg-red-900/10 dark:text-red-300">{err}</div>}

      <div className="grid grid-cols-1 xl:grid-cols-[360px_1fr] gap-6">
        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-4 space-y-3`}>
          {platforms.map(platform => (
            <button key={platform.id} onClick={() => setSelectedId(platform.id)} className={`w-full text-left rounded-xl border px-4 py-3 transition-colors ${selectedId === platform.id ? 'border-blue-300 bg-blue-50/70 dark:border-blue-700 dark:bg-blue-900/20' : 'border-gray-100 dark:border-gray-700/50 hover:bg-gray-50 dark:hover:bg-gray-900/40'}`}>
              <div className="flex items-center justify-between gap-3">
                <div className="font-medium text-gray-900 dark:text-white">{platform.label}</div>
                <div className="text-[10px] uppercase tracking-wider text-gray-500">{platform.runtimeStatus || 'unknown'}</div>
              </div>
              <div className="mt-1 text-xs text-gray-500">{platform.detail || '-'}</div>
            </button>
          ))}
        </div>

        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-5 space-y-4`}>
          {!detail ? (
            <div className="text-sm text-gray-500">{locale === 'zh-CN' ? '请选择一个平台' : 'Select a platform'}</div>
          ) : (
            <>
              <div className="flex items-center justify-between gap-3">
                <div>
                  <div className="text-lg font-semibold text-gray-900 dark:text-white">{detail.status.label}</div>
                  <div className="text-xs text-gray-500 mt-1">{detail.status.detail || '-'}</div>
                </div>
                <div className="flex items-center gap-2">
                  <button onClick={() => navigate('/hermes/health')} className={`${modern ? 'page-modern-action px-3 py-2 text-xs' : 'px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-800'}`}>
                    {locale === 'zh-CN' ? '查看健康' : 'Health'}
                  </button>
                  <button onClick={() => runAction(enabled ? 'gateway-restart' : 'gateway-start')} disabled={actionRunning} className={`${modern ? 'page-modern-action px-3 py-2 text-xs disabled:opacity-50' : 'px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-800 disabled:opacity-50'} inline-flex items-center gap-2`}>
                    {enabled ? <Activity size={14} /> : <Play size={14} />}
                    {enabled ? (locale === 'zh-CN' ? '重启网关' : 'Restart') : (locale === 'zh-CN' ? '启动网关' : 'Start')}
                  </button>
                  <button onClick={() => runAction('doctor')} disabled={actionRunning} className={`${modern ? 'page-modern-action px-3 py-2 text-xs disabled:opacity-50' : 'px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-800 disabled:opacity-50'} inline-flex items-center gap-2`}>
                    <Wrench size={14} />
                    Doctor
                  </button>
                  <button onClick={save} disabled={saving} className={`${modern ? 'page-modern-accent px-4 py-2 text-xs disabled:opacity-50' : 'px-4 py-2 text-xs rounded-lg bg-blue-600 text-white disabled:opacity-50'} inline-flex items-center gap-2`}>
                    <Save size={14} />
                    {locale === 'zh-CN' ? '保存' : 'Save'}
                  </button>
                </div>
              </div>

              <label className="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                <input type="checkbox" checked={enabled} onChange={e => setEnabled(e.target.checked)} />
                {locale === 'zh-CN' ? '启用该平台' : 'Enable this platform'}
              </label>

              {(detail.status.lastEvidence || detail.status.lastError) && (
                <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                  <div className="rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 px-4 py-3 text-sm">
                    <div className="text-[11px] uppercase tracking-wider text-gray-500">{locale === 'zh-CN' ? '最近线索' : 'Last Evidence'}</div>
                    <div className="mt-1 text-gray-800 dark:text-gray-100">{detail.status.lastEvidence || '-'}</div>
                  </div>
                  <div className="rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 px-4 py-3 text-sm">
                    <div className="text-[11px] uppercase tracking-wider text-gray-500">{locale === 'zh-CN' ? '最近错误' : 'Last Error'}</div>
                    <div className="mt-1 text-red-600 dark:text-red-300">{detail.status.lastError || '-'}</div>
                  </div>
                </div>
              )}

              <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <div className="text-xs font-semibold uppercase tracking-wider text-gray-500">Config JSON</div>
                  <textarea value={configText} onChange={e => setConfigText(e.target.value)} className="w-full min-h-[320px] rounded-2xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 p-4 text-xs font-mono text-gray-800 dark:text-gray-100 outline-none" />
                </div>
                <div className="space-y-2">
                  <div className="text-xs font-semibold uppercase tracking-wider text-gray-500">ENV</div>
                  <textarea value={envText} onChange={e => setEnvText(e.target.value)} className="w-full min-h-[320px] rounded-2xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 p-4 text-xs font-mono text-gray-800 dark:text-gray-100 outline-none" />
                </div>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

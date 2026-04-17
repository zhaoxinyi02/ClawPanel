import { useEffect, useMemo, useState } from 'react';
import { useOutletContext } from 'react-router-dom';
import { api } from '../lib/api';
import { RefreshCw, Save, FileStack, Plus, Sparkles, FileCode2, Clock3, Braces, ScrollText } from 'lucide-react';
import { useI18n } from '../i18n';

interface HermesProfileFile {
  name: string;
  path: string;
  exists?: boolean;
  size?: number;
  modifiedAt?: string;
  content?: string;
}

function defaultProfileContent(name: string) {
  if (name.endsWith('.yaml') || name.endsWith('.yml')) {
    return 'name: profile\nsystem: concise\n';
  }
  return '# profile\n\nDescribe the profile here.\n';
}

function formatBytes(size?: number) {
  if (!size || size <= 0) return '-';
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

export default function HermesProfiles() {
  const { locale } = useI18n();
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const modern = uiMode === 'modern';
  const [profiles, setProfiles] = useState<HermesProfileFile[]>([]);
  const [selectedName, setSelectedName] = useState('');
  const [content, setContent] = useState('');
  const [newProfileName, setNewProfileName] = useState('new-profile.yaml');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState('');
  const [msg, setMsg] = useState('');

  const load = async () => {
    setLoading(true);
    setErr('');
    try {
      const res = await api.getHermesProfiles();
      if (!res?.ok) {
        setErr(res?.error || (locale === 'zh-CN' ? '加载 Hermes Profiles 失败' : 'Failed to load Hermes profiles'));
        return;
      }
      const nextProfiles = Array.isArray(res.profiles) ? res.profiles : [];
      setProfiles(nextProfiles);
      setSelectedName(prev => prev || nextProfiles[0]?.name || '');
    } catch {
      setErr(locale === 'zh-CN' ? '加载 Hermes Profiles 失败' : 'Failed to load Hermes profiles');
    } finally {
      setLoading(false);
    }
  };

  const loadDetail = async (name: string) => {
    if (!name) return;
    try {
      const res = await api.getHermesProfileDetail(name);
      if (res?.ok) setContent(res.profile?.content || '');
    } catch {
      setErr(locale === 'zh-CN' ? '加载 Profile 详情失败' : 'Failed to load profile detail');
    }
  };

  useEffect(() => {
    void load();
  }, []);

  useEffect(() => {
    if (!selectedName) return;
    void loadDetail(selectedName);
  }, [selectedName]);

  const selectedProfile = useMemo(
    () => profiles.find(profile => profile.name === selectedName) || null,
    [profiles, selectedName],
  );
  const profileType = selectedName.endsWith('.md') ? 'Markdown' : selectedName.endsWith('.yaml') || selectedName.endsWith('.yml') ? 'YAML' : 'Text';

  const saveProfile = async () => {
    if (!selectedName) return;
    setSaving(true);
    setErr('');
    setMsg('');
    try {
      const res = await api.updateHermesProfileDetail(selectedName, content);
      if (!res?.ok) {
        setErr(res?.error || (locale === 'zh-CN' ? '保存 Profile 失败' : 'Failed to save profile'));
        return;
      }
      setMsg(locale === 'zh-CN' ? 'Profile 已保存' : 'Profile saved');
      await load();
      await loadDetail(selectedName);
    } catch {
      setErr(locale === 'zh-CN' ? '保存 Profile 失败' : 'Failed to save profile');
    } finally {
      setSaving(false);
    }
  };

  const createProfile = async () => {
    const name = newProfileName.trim();
    if (!name) return;
    setSaving(true);
    setErr('');
    setMsg('');
    try {
      const initial = defaultProfileContent(name);
      const res = await api.updateHermesProfileDetail(name, initial);
      if (!res?.ok) {
        setErr(res?.error || (locale === 'zh-CN' ? '创建 Profile 失败' : 'Failed to create profile'));
        return;
      }
      setSelectedName(name);
      setContent(initial);
      setMsg(locale === 'zh-CN' ? '新 Profile 已创建' : 'New profile created');
      await load();
    } catch {
      setErr(locale === 'zh-CN' ? '创建 Profile 失败' : 'Failed to create profile');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className={`space-y-6 ${modern ? 'page-modern' : ''}`}>
      <div className={`${modern ? 'page-modern-header' : 'flex items-center justify-between'}`}>
        <div>
          <h2 className={`${modern ? 'page-modern-title text-xl' : 'text-xl font-bold text-gray-900 dark:text-white'}`}>
            {locale === 'zh-CN' ? 'Hermes Profiles' : 'Hermes Profiles'}
          </h2>
          <p className={`${modern ? 'page-modern-subtitle mt-1 text-sm' : 'mt-1 text-sm text-gray-500'}`}>
            {locale === 'zh-CN'
              ? '把 Hermes 的 Profiles 单独作为文件资源管理，便于维护多套 persona、策略和路由目标。'
              : 'Manage Hermes profiles as standalone files for persona, policy, and routing targets.'}
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

      {msg && <div className="rounded-2xl border border-emerald-100 bg-emerald-50/80 px-4 py-3 text-sm text-emerald-700 dark:border-emerald-900/30 dark:bg-emerald-900/10 dark:text-emerald-300">{msg}</div>}
      {err && <div className="rounded-2xl border border-red-100 bg-red-50/80 px-4 py-3 text-sm text-red-700 dark:border-red-900/30 dark:bg-red-900/10 dark:text-red-300">{err}</div>}

      <div className={`${modern ? 'rounded-[30px] border border-white/60 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.9),rgba(239,246,255,0.68))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.9),rgba(30,64,175,0.12))] backdrop-blur-xl shadow-[0_24px_54px_rgba(15,23,42,0.08)]' : 'rounded-2xl border border-gray-100 bg-white dark:bg-gray-800 dark:border-gray-700/50'} p-5`}>
        <div className="grid grid-cols-1 gap-5 xl:grid-cols-[1.15fr_0.85fr]">
          <div className="space-y-3">
            <div className="inline-flex items-center gap-2 rounded-full border border-blue-100/70 bg-white/80 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.18em] text-blue-700 dark:border-blue-400/15 dark:bg-slate-900/50 dark:text-blue-200">
              <Sparkles size={13} />
              {locale === 'zh-CN' ? 'Profiles 资产区' : 'Profile Assets'}
            </div>
            <div className="text-lg font-semibold text-slate-900 dark:text-white">
              {locale === 'zh-CN'
                ? '像管理配置文件一样管理 Hermes 的 persona 和策略模板'
                : 'Manage Hermes persona and policy templates like first-class config assets'}
            </div>
            <div className="text-sm leading-6 text-slate-600 dark:text-slate-300">
              {locale === 'zh-CN'
                ? '左侧看文件集合，右侧直接编辑正文。这样切 persona、路由目标和策略模板时会比纯文本堆叠更直观。'
                : 'Browse the file set on the left and edit content on the right for a more visual workflow across persona, routing, and policy templates.'}
            </div>
          </div>
          <div className="rounded-[24px] border border-white/70 bg-white/80 p-4 shadow-[0_14px_34px_rgba(15,23,42,0.06)] dark:border-slate-700/50 dark:bg-slate-900/45">
            <div className="flex items-center justify-between gap-3">
              <div>
                <div className="text-xs font-semibold uppercase tracking-[0.18em] text-slate-400">
                  {locale === 'zh-CN' ? '当前选中文件' : 'Selected Profile'}
                </div>
                <div className="mt-2 text-base font-semibold text-slate-900 dark:text-white">
                  {selectedProfile?.name || (locale === 'zh-CN' ? '等待选择 Profile' : 'Waiting for selection')}
                </div>
              </div>
              <span className="rounded-full bg-blue-50 px-3 py-1 text-[10px] font-semibold uppercase text-blue-700 dark:bg-blue-900/20 dark:text-blue-200">
                {selectedName ? profileType : '-'}
              </span>
            </div>
            <div className="mt-3 text-sm text-slate-500 dark:text-slate-400">
              {selectedProfile?.path || (locale === 'zh-CN' ? '选择左侧文件后可在这里查看上下文。' : 'Select a file on the left to inspect context here.')}
            </div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        {[
          { label: locale === 'zh-CN' ? 'Profile 数量' : 'Profiles', value: profiles.length },
          { label: locale === 'zh-CN' ? '当前文件' : 'Selected', value: selectedProfile?.name || '-' },
          { label: locale === 'zh-CN' ? '文件大小' : 'Size', value: formatBytes(selectedProfile?.size) },
          { label: locale === 'zh-CN' ? '最近修改' : 'Modified', value: selectedProfile?.modifiedAt || '-' },
        ].map(card => (
          <div key={card.label} className={`${modern ? 'page-modern-card' : 'bg-white dark:bg-gray-800'} rounded-2xl border border-gray-100 p-4 dark:border-gray-700/50`}>
            <div className="text-xs font-semibold uppercase tracking-wider text-gray-500">{card.label}</div>
            <div className="mt-3 text-lg font-bold text-gray-900 dark:text-white break-all">{card.value}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[320px_1fr] gap-6">
        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-4 space-y-4`}>
          <div className="space-y-2">
            <div className="text-sm font-semibold text-gray-900 dark:text-white">{locale === 'zh-CN' ? '新建 Profile' : 'New Profile'}</div>
            <div className="flex items-center gap-2">
              <input
                value={newProfileName}
                onChange={e => setNewProfileName(e.target.value)}
                className="flex-1 rounded-xl border border-gray-100 bg-gray-50/70 px-3 py-2 text-sm outline-none dark:border-gray-700/50 dark:bg-gray-900/40 dark:text-gray-100"
                placeholder="new-profile.yaml"
              />
              <button
                onClick={() => void createProfile()}
                disabled={saving}
                className={`${modern ? 'page-modern-action px-3 py-2 text-xs disabled:opacity-50' : 'rounded-lg bg-gray-100 px-3 py-2 text-xs disabled:opacity-50 dark:bg-gray-800'} inline-flex items-center gap-2`}
              >
                <Plus size={14} />
                {locale === 'zh-CN' ? '新建' : 'New'}
              </button>
            </div>
          </div>

          <div className="space-y-2">
            {profiles.map(profile => (
              <button
                key={profile.name}
                onClick={() => setSelectedName(profile.name)}
                className={`w-full rounded-xl border px-4 py-3 text-left transition-colors ${
                  selectedName === profile.name
                    ? 'border-blue-300 bg-blue-50/70 dark:border-blue-700 dark:bg-blue-900/20'
                    : 'border-gray-100 hover:bg-gray-50 dark:border-gray-700/50 dark:hover:bg-gray-900/40'
                }`}
              >
                <div className="flex items-start gap-3">
                  <FileStack size={16} className="mt-0.5 shrink-0 text-blue-500" />
                  <div className="min-w-0 flex-1">
                    <div className="truncate font-medium text-gray-900 dark:text-white">{profile.name}</div>
                    <div className="mt-1 truncate text-xs text-gray-500">{profile.path}</div>
                    <div className="mt-2 flex items-center gap-2 text-[11px] text-gray-400">
                      <span>{formatBytes(profile.size)}</span>
                      <span>·</span>
                      <span className="inline-flex items-center gap-1">
                        <Clock3 size={11} />
                        {profile.modifiedAt || '-'}
                      </span>
                    </div>
                  </div>
                </div>
              </button>
            ))}
            {profiles.length === 0 && !loading && (
              <div className="rounded-xl border border-dashed border-gray-200 px-4 py-6 text-sm text-gray-500 dark:border-gray-700/50 dark:text-gray-400">
                {locale === 'zh-CN' ? '当前还没有 Profile 文件' : 'No profile files yet'}
              </div>
            )}
          </div>
        </div>

        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-5 space-y-4`}>
          {!selectedName ? (
            <div className="text-sm text-gray-500">{locale === 'zh-CN' ? '请选择一个 Profile' : 'Select a profile'}</div>
          ) : (
            <>
              <div className="flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <div className="font-semibold text-gray-900 dark:text-white truncate">{selectedName}</div>
                  <div className="mt-1 text-xs text-gray-500 break-all">{selectedProfile?.path || '-'}</div>
                </div>
                <button
                  onClick={() => void saveProfile()}
                  disabled={saving}
                  className={`${modern ? 'page-modern-accent px-4 py-2 text-xs disabled:opacity-50' : 'rounded-lg bg-blue-600 px-4 py-2 text-xs text-white disabled:opacity-50'} inline-flex items-center gap-2`}
                >
                  <Save size={14} />
                  {locale === 'zh-CN' ? '保存' : 'Save'}
                </button>
              </div>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
                <div className="rounded-xl border border-gray-100 bg-gray-50/70 px-4 py-3 dark:border-gray-700/50 dark:bg-gray-900/40">
                  <div className="text-[11px] uppercase tracking-wider text-gray-500">{locale === 'zh-CN' ? '格式' : 'Format'}</div>
                  <div className="mt-2 flex items-center gap-2 text-sm font-medium text-gray-900 dark:text-white">
                    <Braces size={14} className="text-blue-500" />
                    {profileType}
                  </div>
                </div>
                <div className="rounded-xl border border-gray-100 bg-gray-50/70 px-4 py-3 dark:border-gray-700/50 dark:bg-gray-900/40">
                  <div className="text-[11px] uppercase tracking-wider text-gray-500">{locale === 'zh-CN' ? '字节大小' : 'Size'}</div>
                  <div className="mt-2 flex items-center gap-2 text-sm font-medium text-gray-900 dark:text-white">
                    <FileCode2 size={14} className="text-emerald-500" />
                    {formatBytes(selectedProfile?.size)}
                  </div>
                </div>
                <div className="rounded-xl border border-gray-100 bg-gray-50/70 px-4 py-3 dark:border-gray-700/50 dark:bg-gray-900/40">
                  <div className="text-[11px] uppercase tracking-wider text-gray-500">{locale === 'zh-CN' ? '最近修改' : 'Modified'}</div>
                  <div className="mt-2 flex items-center gap-2 text-sm font-medium text-gray-900 dark:text-white">
                    <ScrollText size={14} className="text-violet-500" />
                    {selectedProfile?.modifiedAt || '-'}
                  </div>
                </div>
              </div>
              <textarea
                value={content}
                onChange={e => setContent(e.target.value)}
                className="min-h-[560px] w-full rounded-2xl border border-gray-100 bg-gray-50/70 p-4 font-mono text-sm text-gray-800 outline-none dark:border-gray-700/50 dark:bg-gray-900/40 dark:text-gray-100"
              />
            </>
          )}
        </div>
      </div>
    </div>
  );
}

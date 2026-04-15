import { useEffect, useState } from 'react';
import { useOutletContext } from 'react-router-dom';
import { api } from '../lib/api';
import { RefreshCw, Save, Wand2, GitBranch } from 'lucide-react';
import { useI18n } from '../i18n';

interface HermesProfileFile {
  name: string;
  path: string;
  content?: string;
}

export default function HermesPersonality() {
  const { locale } = useI18n();
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const modern = uiMode === 'modern';
  const [soulContent, setSoulContent] = useState('');
  const [profiles, setProfiles] = useState<HermesProfileFile[]>([]);
  const [selectedProfile, setSelectedProfile] = useState('');
  const [profileContent, setProfileContent] = useState('');
  const [routingText, setRoutingText] = useState('{\n  "defaultProfile": "default",\n  "rules": []\n}');
  const [previewForm, setPreviewForm] = useState({ platform: 'telegram', chatType: 'direct', chatId: 'demo-chat', userId: '', message: '' });
  const [preview, setPreview] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');
  const [newProfileName, setNewProfileName] = useState('new-profile.yaml');

  const load = async () => {
    setLoading(true);
    setErr('');
    try {
      const [personalityRes, profilesRes, routingRes] = await Promise.all([
        api.getHermesPersonality(),
        api.getHermesProfiles(),
        api.getHermesRouting(),
      ]);
      if (personalityRes.ok) setSoulContent(personalityRes.personality?.soulContent || '');
      if (profilesRes.ok) {
        const next = profilesRes.profiles || [];
        setProfiles(next);
        setSelectedProfile(prev => prev || next[0]?.name || '');
      }
      if (routingRes.ok) setRoutingText(JSON.stringify(routingRes.routing || { defaultProfile: 'default', rules: [] }, null, 2));
    } catch {
      setErr(locale === 'zh-CN' ? '加载 Hermes personality / routing 失败' : 'Failed to load Hermes personality / routing');
    } finally {
      setLoading(false);
    }
  };

  const loadProfileDetail = async (name: string) => {
    if (!name) return;
    try {
      const r = await api.getHermesProfileDetail(name);
      if (r.ok) setProfileContent(r.profile?.content || '');
    } catch {
      setErr(locale === 'zh-CN' ? '加载 profile 详情失败' : 'Failed to load profile detail');
    }
  };

  useEffect(() => { load(); }, []);
  useEffect(() => { if (selectedProfile) loadProfileDetail(selectedProfile); }, [selectedProfile]);

  const saveSoul = async () => {
    setSaving(true);
    setErr('');
    setMsg('');
    try {
      const r = await api.updateHermesPersonality(soulContent);
      if (r.ok) setMsg(locale === 'zh-CN' ? 'SOUL.md 已保存' : 'SOUL.md saved');
      else setErr(r.error || 'Save failed');
    } catch {
      setErr(locale === 'zh-CN' ? '保存 SOUL.md 失败' : 'Failed to save SOUL.md');
    } finally {
      setSaving(false);
    }
  };

  const saveProfile = async () => {
    if (!selectedProfile) return;
    setSaving(true);
    setErr('');
    setMsg('');
    try {
      const r = await api.updateHermesProfileDetail(selectedProfile, profileContent);
      if (r.ok) {
        setMsg(locale === 'zh-CN' ? 'Profile 已保存' : 'Profile saved');
        await load();
      }
      else setErr(r.error || 'Save failed');
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
      const r = await api.updateHermesProfileDetail(name, '# profile\n');
      if (r.ok) {
        setSelectedProfile(name);
        setProfileContent('# profile\n');
        setMsg(locale === 'zh-CN' ? '已创建新 Profile' : 'New profile created');
        await load();
      } else {
        setErr(r.error || 'Create failed');
      }
    } catch {
      setErr(locale === 'zh-CN' ? '创建 Profile 失败' : 'Failed to create profile');
    } finally {
      setSaving(false);
    }
  };

  const saveRouting = async () => {
    setSaving(true);
    setErr('');
    setMsg('');
    try {
      const parsed = JSON.parse(routingText || '{}');
      const r = await api.updateHermesRouting(parsed);
      if (r.ok) {
        setMsg(locale === 'zh-CN' ? 'Routing 已保存' : 'Routing saved');
        await runPreview();
      }
      else setErr(r.error || 'Save failed');
    } catch (e) {
      setErr(locale === 'zh-CN' ? `Routing JSON 格式错误: ${String(e)}` : `Invalid routing JSON: ${String(e)}`);
    } finally {
      setSaving(false);
    }
  };

  const runPreview = async () => {
    setErr('');
    try {
      const r = await api.previewHermesRouting(previewForm);
      if (r.ok) {
        setPreview(r.preview || null);
        setMsg(locale === 'zh-CN' ? 'Routing Preview 已刷新' : 'Routing preview refreshed');
      }
      else setErr(r.error || 'Preview failed');
    } catch {
      setErr(locale === 'zh-CN' ? '运行 preview 失败' : 'Failed to run preview');
    }
  };

  return (
    <div className={`space-y-6 ${modern ? 'page-modern' : ''}`}>
      <div className={`${modern ? 'page-modern-header' : 'flex items-center justify-between'}`}>
        <div>
          <h2 className={`${modern ? 'page-modern-title text-xl' : 'text-xl font-bold text-gray-900 dark:text-white'}`}>{locale === 'zh-CN' ? 'Hermes 人格与路由' : 'Hermes Personality & Routing'}</h2>
          <p className={`${modern ? 'page-modern-subtitle mt-1 text-sm' : 'text-sm text-gray-500 mt-1'}`}>
            {locale === 'zh-CN'
              ? '管理 SOUL、Profiles 以及 Hermes 的会话路由规则骨架。'
              : 'Manage SOUL, profiles, and Hermes session routing rules.'}
          </p>
        </div>
        <button onClick={load} className={`${modern ? 'page-modern-action px-3 py-2 text-xs' : 'px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-800'} inline-flex items-center gap-2`}>
          <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          {locale === 'zh-CN' ? '刷新' : 'Refresh'}
        </button>
      </div>

      {msg && <div className="rounded-2xl border border-emerald-100 bg-emerald-50/80 px-4 py-3 text-sm text-emerald-700 dark:border-emerald-900/30 dark:bg-emerald-900/10 dark:text-emerald-300">{msg}</div>}
      {err && <div className="rounded-2xl border border-red-100 bg-red-50/80 px-4 py-3 text-sm text-red-700 dark:border-red-900/30 dark:bg-red-900/10 dark:text-red-300">{err}</div>}

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-5 space-y-4`}>
          <div className="flex items-center gap-2 font-semibold text-gray-900 dark:text-white">
            <Wand2 size={18} className="text-blue-500" />
            SOUL.md
          </div>
          <textarea value={soulContent} onChange={e => setSoulContent(e.target.value)} className="w-full min-h-[260px] rounded-2xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 p-4 text-sm text-gray-800 dark:text-gray-100 outline-none" />
          <button onClick={saveSoul} disabled={saving} className={`${modern ? 'page-modern-accent px-4 py-2 text-xs disabled:opacity-50' : 'px-4 py-2 text-xs rounded-lg bg-blue-600 text-white disabled:opacity-50'} inline-flex items-center gap-2`}>
            <Save size={14} />
            {locale === 'zh-CN' ? '保存 SOUL' : 'Save SOUL'}
          </button>
        </div>

        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-5 space-y-4`}>
          <div className="flex items-center gap-2 font-semibold text-gray-900 dark:text-white">
            <Wand2 size={18} className="text-blue-500" />
            Profiles
          </div>
          <div className="flex items-center gap-2">
            <input value={newProfileName} onChange={e => setNewProfileName(e.target.value)} placeholder="new-profile.yaml" className="flex-1 rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 px-3 py-2 text-sm outline-none" />
            <button onClick={createProfile} disabled={saving} className={`${modern ? 'page-modern-action px-3 py-2 text-xs disabled:opacity-50' : 'px-3 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-800 disabled:opacity-50'}`}>
              {locale === 'zh-CN' ? '新建' : 'New'}
            </button>
          </div>
          <div className="flex gap-2 flex-wrap">
            {profiles.map(profile => (
              <button key={profile.name} onClick={() => setSelectedProfile(profile.name)} className={`rounded-xl px-3 py-2 text-xs border ${selectedProfile === profile.name ? 'border-blue-300 bg-blue-50/70 dark:border-blue-700 dark:bg-blue-900/20' : 'border-gray-100 dark:border-gray-700/50 hover:bg-gray-50 dark:hover:bg-gray-900/40'}`}>
                {profile.name}
              </button>
            ))}
          </div>
          <textarea value={profileContent} onChange={e => setProfileContent(e.target.value)} className="w-full min-h-[220px] rounded-2xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 p-4 text-sm font-mono text-gray-800 dark:text-gray-100 outline-none" />
          <button onClick={saveProfile} disabled={saving || !selectedProfile} className={`${modern ? 'page-modern-accent px-4 py-2 text-xs disabled:opacity-50' : 'px-4 py-2 text-xs rounded-lg bg-blue-600 text-white disabled:opacity-50'} inline-flex items-center gap-2`}>
            <Save size={14} />
            {locale === 'zh-CN' ? '保存 Profile' : 'Save Profile'}
          </button>
        </div>
      </div>

      <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-5 space-y-4`}>
        <div className="flex items-center gap-2 font-semibold text-gray-900 dark:text-white">
          <GitBranch size={18} className="text-blue-500" />
          Routing JSON
        </div>
        <textarea value={routingText} onChange={e => setRoutingText(e.target.value)} className="w-full min-h-[260px] rounded-2xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 p-4 text-sm font-mono text-gray-800 dark:text-gray-100 outline-none" />
        <div className="flex items-center gap-2">
          <button onClick={saveRouting} disabled={saving} className={`${modern ? 'page-modern-accent px-4 py-2 text-xs disabled:opacity-50' : 'px-4 py-2 text-xs rounded-lg bg-blue-600 text-white disabled:opacity-50'} inline-flex items-center gap-2`}>
            <Save size={14} />
            {locale === 'zh-CN' ? '保存 Routing' : 'Save Routing'}
          </button>
          <button onClick={runPreview} className={`${modern ? 'page-modern-action px-4 py-2 text-xs' : 'px-4 py-2 text-xs rounded-lg bg-gray-100 dark:bg-gray-800'} inline-flex items-center gap-2`}>
            <GitBranch size={14} />
            {locale === 'zh-CN' ? '运行 Preview' : 'Run Preview'}
          </button>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="space-y-2">
            <input value={previewForm.platform} onChange={e => setPreviewForm(prev => ({ ...prev, platform: e.target.value }))} placeholder="platform" className="w-full rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 px-3 py-2 text-sm outline-none" />
            <input value={previewForm.chatType} onChange={e => setPreviewForm(prev => ({ ...prev, chatType: e.target.value }))} placeholder="chatType" className="w-full rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 px-3 py-2 text-sm outline-none" />
            <input value={previewForm.chatId} onChange={e => setPreviewForm(prev => ({ ...prev, chatId: e.target.value }))} placeholder="chatId" className="w-full rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 px-3 py-2 text-sm outline-none" />
            <input value={previewForm.userId} onChange={e => setPreviewForm(prev => ({ ...prev, userId: e.target.value }))} placeholder="userId" className="w-full rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 px-3 py-2 text-sm outline-none" />
            <textarea value={previewForm.message} onChange={e => setPreviewForm(prev => ({ ...prev, message: e.target.value }))} placeholder="message" className="w-full min-h-[120px] rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 px-3 py-2 text-sm outline-none" />
          </div>
          <pre className="rounded-2xl bg-gray-950 text-gray-100 p-4 overflow-x-auto text-xs leading-6 font-mono">{preview ? JSON.stringify(preview, null, 2) : 'No preview yet.'}</pre>
        </div>
        <div className="rounded-xl border border-gray-100 dark:border-gray-700/50 bg-gray-50/70 dark:bg-gray-900/40 px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
          {locale === 'zh-CN'
            ? '建议先在 Routing 中写最少规则，再用 Preview 验证 sessionKey、Profile 和 homeTarget 是否符合预期。'
            : 'Start with minimal routing rules, then use Preview to verify sessionKey, profile, and homeTarget.'}
        </div>
      </div>
    </div>
  );
}

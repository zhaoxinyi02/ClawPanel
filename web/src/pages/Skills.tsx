import { useEffect, useState } from 'react';
import { api } from '../lib/api';
import {
  Sparkles, Search, ToggleLeft, ToggleRight, Download,
  RefreshCw, Package, Globe, Check, Loader2, ExternalLink, X, Key, FolderOpen,
} from 'lucide-react';
import { useI18n } from '../i18n';

interface SkillEntry {
  id: string;
  name: string;
  description?: string;
  enabled: boolean;
  source: string;
  version?: string;
  installedAt?: string;
  metadata?: any;
  requires?: { env?: string[]; bins?: string[] };
  path?: string;
}

const CLAWHUB_CATALOG: { id: string; name: string; description: string; descriptionZh?: string; version: string; category: string }[] = [
  { id: 'feishu', name: '飞书 / Lark', description: 'Feishu/Lark bot channel via WebSocket', descriptionZh: '飞书机器人通道插件，支持 WebSocket 连接', version: '1.1.0', category: '通道' },
  { id: 'qqbot', name: 'QQ 官方机器人', description: 'QQ Official Bot API plugin', descriptionZh: 'QQ开放平台官方Bot API插件', version: '1.2.3', category: '通道' },
  { id: 'dingtalk', name: '钉钉', description: 'DingTalk robot channel plugin', descriptionZh: '钉钉机器人通道插件', version: '0.2.0', category: '通道' },
  { id: 'wecom', name: '企业微信', description: 'WeCom (WeChat Work) app message plugin', descriptionZh: '企业微信应用消息通道插件', version: '2026.1.30', category: '通道' },
  { id: 'msteams', name: 'Microsoft Teams', description: 'Bot Framework enterprise channel plugin', descriptionZh: 'Bot Framework 企业通道插件', version: '0.3.0', category: '通道' },
  { id: 'mattermost', name: 'Mattermost', description: 'Mattermost Bot API + WebSocket plugin', descriptionZh: 'Mattermost Bot API + WebSocket 插件', version: '0.2.0', category: '通道' },
  { id: 'line', name: 'LINE', description: 'LINE Messaging API channel plugin', descriptionZh: 'LINE Messaging API 通道插件', version: '0.1.0', category: '通道' },
  { id: 'matrix', name: 'Matrix', description: 'Matrix protocol channel plugin', descriptionZh: 'Matrix 协议通道插件', version: '0.1.0', category: '通道' },
  { id: 'nextcloud-talk', name: 'Nextcloud Talk', description: 'Nextcloud Talk self-hosted chat plugin', descriptionZh: 'Nextcloud Talk 自托管聊天插件', version: '0.1.0', category: '通道' },
  { id: 'nostr', name: 'Nostr', description: 'Decentralized NIP-04 DM plugin', descriptionZh: '去中心化 NIP-04 DM 插件', version: '0.1.0', category: '通道' },
  { id: 'twitch', name: 'Twitch', description: 'Twitch Chat via IRC plugin', descriptionZh: 'Twitch Chat via IRC 插件', version: '0.1.0', category: '通道' },
  { id: 'tlon', name: 'Tlon', description: 'Urbit-based messenger plugin', descriptionZh: 'Urbit-based messenger 插件', version: '0.1.0', category: '通道' },
  { id: 'zalo', name: 'Zalo', description: 'Zalo Bot API plugin', descriptionZh: 'Zalo Bot API 插件', version: '0.1.0', category: '通道' },
];

export default function Skills() {
  const { t } = useI18n();
  const [skills, setSkills] = useState<SkillEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState<'all' | 'enabled' | 'disabled'>('all');
  const [tab, setTab] = useState<'installed' | 'clawhub'>('installed');
  const [msg, setMsg] = useState('');
  const [installing, setInstalling] = useState('');
  const [syncing, setSyncing] = useState(false);
  const [configSkill, setConfigSkill] = useState<SkillEntry | null>(null);

  useEffect(() => { loadSkills(); }, []);

  const loadSkills = async () => {
    setLoading(true);
    try {
      const r = await api.getSkills();
      if (r.ok) setSkills(r.skills || []);
    } catch (err) {
      console.error('Failed to load skills:', err);
    } finally { setLoading(false); }
  };

  const toggleSkill = async (id: string) => {
    const skill = skills.find(s => s.id === id);
    if (!skill) return;
    const newEnabled = !skill.enabled;
    setSkills(prev => prev.map(s => s.id === id ? { ...s, enabled: newEnabled } : s));
    try {
      await api.updatePlugin(id, { enabled: newEnabled });
      setMsg(`${skill.name} ${newEnabled ? t.common.enabled : t.common.disabled}`);
      setTimeout(() => setMsg(''), 2000);
    } catch {
      setSkills(prev => prev.map(s => s.id === id ? { ...s, enabled: !newEnabled } : s));
      setMsg(t.common.operationFailed);
      setTimeout(() => setMsg(''), 2000);
    }
  };

  const installedIds = new Set(skills.map(s => s.id));
  const hubFiltered = CLAWHUB_CATALOG.filter(s => {
    if (!search) return true;
    const q = search.toLowerCase();
    return s.id.includes(q) || s.name.toLowerCase().includes(q) || s.description.toLowerCase().includes(q);
  });

  const handleSyncClawHub = async () => {
    setSyncing(true);
    try {
      const r = await api.syncClawHub();
      if (r.ok && r.skills) {
        setMsg(t.skills.syncSuccess.replace('{n}', String(r.skills.length)));
      } else {
        setMsg(t.skills.syncDone);
      }
    } catch { setMsg(t.skills.syncFailed); }
    finally { setSyncing(false); setTimeout(() => setMsg(''), 3000); }
  };

  const handleInstallHint = (id: string) => {
    setInstalling(id);
    setMsg(t.skills.installHint.replace('{id}', id));
    setTimeout(() => setInstalling(''), 3000);
  };

  const filtered = skills.filter(s => {
    if (filter === 'enabled' && !s.enabled) return false;
    if (filter === 'disabled' && s.enabled) return false;
    if (search) {
      const q = search.toLowerCase();
      return s.id.toLowerCase().includes(q) || s.name.toLowerCase().includes(q) || (s.description || '').toLowerCase().includes(q);
    }
    return true;
  });

  const getSourceBadge = (source: string) => {
    const badges: Record<string, { label: string; color: string }> = {
      installed: { label: t.skills.srcInstalled, color: 'bg-emerald-100 dark:bg-emerald-950 text-emerald-700 dark:text-emerald-300' },
      'config-ext': { label: t.skills.srcDevExt, color: 'bg-blue-100 dark:bg-blue-950 text-blue-700 dark:text-blue-300' },
      skill: { label: t.skills.srcSkill, color: 'bg-purple-100 dark:bg-purple-950 text-purple-700 dark:text-purple-300' },
      'app-skill': { label: t.skills.srcAppSkill, color: 'bg-indigo-100 dark:bg-indigo-950 text-indigo-700 dark:text-indigo-300' },
      workspace: { label: t.skills.srcWorkspace, color: 'bg-amber-100 dark:bg-amber-950 text-amber-700 dark:text-amber-300' },
      script: { label: t.skills.srcScript, color: 'bg-pink-100 dark:bg-pink-950 text-pink-700 dark:text-pink-300' },
      config: { label: t.skills.srcConfig, color: 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400' },
    };
    const badge = badges[source] || { label: source, color: 'bg-gray-100 dark:bg-gray-800 text-gray-600' };
    return <span className={`px-2 py-0.5 rounded-full text-[10px] font-medium ${badge.color}`}>{badge.label}</span>;
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-bold text-gray-900 dark:text-white tracking-tight">{t.skills.title}</h2>
          <p className="text-sm text-gray-500 mt-1">{t.skills.subtitle}</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={loadSkills} className="flex items-center gap-2 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 transition-colors shadow-sm">
            <RefreshCw size={14} />{t.skills.refreshList}
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-6 border-b border-gray-200 dark:border-gray-800">
        <button onClick={() => setTab('installed')}
          className={`pb-3 text-sm font-medium border-b-2 transition-all flex items-center gap-2 ${tab === 'installed' ? 'border-violet-600 text-violet-700 dark:text-violet-400' : 'border-transparent text-gray-500 hover:text-gray-700 dark:hover:text-gray-300'}`}>
          <Package size={16} />{t.skills.installedTab} <span className="bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 text-xs px-1.5 py-0.5 rounded-full">{skills.length}</span>
        </button>
        <button onClick={() => setTab('clawhub')}
          className={`pb-3 text-sm font-medium border-b-2 transition-all flex items-center gap-2 ${tab === 'clawhub' ? 'border-violet-600 text-violet-700 dark:text-violet-400' : 'border-transparent text-gray-500 hover:text-gray-700 dark:hover:text-gray-300'}`}>
          <Globe size={16} />{t.skills.clawHubTab} <span className="bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 text-xs px-1.5 py-0.5 rounded-full">{CLAWHUB_CATALOG.length}</span>
        </button>
      </div>

      {msg && (
        <div className={`px-4 py-3 rounded-xl text-sm font-medium flex items-center gap-2 ${msg.includes('失败') ? 'bg-red-50 dark:bg-red-900/30 text-red-600' : 'bg-emerald-50 dark:bg-emerald-900/30 text-emerald-600'}`}>
          {msg.includes('失败') ? <X size={16} /> : <Check size={16} />}
          {msg}
        </div>
      )}

      {tab === 'installed' && (
        <div className="space-y-4">
          {/* Filters */}
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div className="relative flex-1 min-w-[240px] max-w-md">
              <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input value={search} onChange={e => setSearch(e.target.value)} placeholder={t.skills.searchInstalled}
                className="w-full pl-9 pr-4 py-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500 transition-all" />
            </div>
            <div className="flex gap-1 bg-gray-100 dark:bg-gray-800 p-1 rounded-lg">
              {(['all', 'enabled', 'disabled'] as const).map(f => (
                <button key={f} onClick={() => setFilter(f)}
                  className={`px-3 py-1.5 text-xs rounded-md transition-all font-medium ${filter === f ? 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm' : 'text-gray-500 hover:text-gray-700 dark:hover:text-gray-300'}`}>
                  {f === 'all' ? t.skills.allFilter : f === 'enabled' ? t.skills.enabledFilter : t.skills.disabledFilter}
                </button>
              ))}
            </div>
          </div>

          {/* Skills list */}
          {loading ? (
            <div className="flex flex-col items-center justify-center py-16 text-gray-400 gap-3">
              <Loader2 size={32} className="animate-spin text-violet-500/50" />
              <p className="text-sm">{t.skills.loadingSkills}</p>
            </div>
          ) : filtered.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-gray-400 border-2 border-dashed border-gray-100 dark:border-gray-800 rounded-xl">
              <Package size={32} className="opacity-20 mb-2" />
              <p className="text-sm">{t.skills.noMatch}</p>
            </div>
          ) : (
            <div className="grid gap-3">
              {filtered.map(skill => (
                <div key={skill.id} className="bg-white dark:bg-gray-800 rounded-xl p-4 shadow-sm border border-gray-100 dark:border-gray-700/50 flex items-center gap-4 hover:shadow-md transition-all group">
                  <div className={`w-12 h-12 rounded-xl flex items-center justify-center shrink-0 shadow-sm ${skill.enabled ? 'bg-gradient-to-br from-violet-500 to-indigo-600' : 'bg-gray-100 dark:bg-gray-700'}`}>
                    <Sparkles size={20} className={skill.enabled ? 'text-white' : 'text-gray-400'} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <span className="text-base font-bold text-gray-900 dark:text-white">{skill.name}</span>
                      {skill.version && <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-gray-500 font-mono">v{skill.version}</span>}
                      {getSourceBadge(skill.source)}
                      {skill.requires && (skill.requires.env || skill.requires.bins) && (
                        <button onClick={() => setConfigSkill(skill)} className="text-[10px] px-2 py-0.5 rounded-full bg-amber-50 dark:bg-amber-900/30 text-amber-600 border border-amber-100 dark:border-amber-800 hover:bg-amber-100 transition-colors">
                          {t.skills.configRequired}
                        </button>
                      )}
                    </div>
                    {skill.description && <p className="text-xs text-gray-500 truncate">{skill.description}</p>}
                  </div>
                  <div className="flex items-center gap-4 border-l border-gray-100 dark:border-gray-700 pl-4">
                    <div className="text-right hidden sm:block">
                      <div className="text-[10px] text-gray-400">{t.common.status}</div>
                      <div className={`text-xs font-medium ${skill.enabled ? 'text-emerald-600' : 'text-gray-400'}`}>
                        {skill.enabled ? t.common.running : t.common.stopped}
                      </div>
                    </div>
                    <button onClick={() => toggleSkill(skill.id)} className="relative group/toggle focus:outline-none">
                      {skill.enabled 
                        ? <ToggleRight size={32} className="text-emerald-500 transition-transform group-hover/toggle:scale-105" /> 
                        : <ToggleLeft size={32} className="text-gray-300 dark:text-gray-600 transition-transform group-hover/toggle:scale-105" />}
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {tab === 'clawhub' && (
        <div className="space-y-4">
          <div className="flex items-center justify-between gap-4 bg-gradient-to-r from-violet-50 to-indigo-50 dark:from-violet-900/20 dark:to-indigo-900/20 p-4 rounded-xl border border-violet-100 dark:border-violet-800/30">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-white dark:bg-gray-800 rounded-lg shadow-sm">
                <Globe size={20} className="text-violet-600" />
              </div>
              <div>
                <h3 className="text-sm font-bold text-gray-900 dark:text-white">{t.skills.clawHubTitle}</h3>
                <p className="text-xs text-gray-500">{t.skills.clawHubSubtitle}</p>
              </div>
            </div>
            <div className="flex gap-2">
              <button onClick={handleSyncClawHub} disabled={syncing}
                className="flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 shadow-sm transition-colors">
                {syncing ? <Loader2 size={14} className="animate-spin" /> : <RefreshCw size={14} />}
                {syncing ? t.skills.syncing : t.skills.syncStore}
              </button>
              <a href="https://clawhub.ai/skills?sort=downloads" target="_blank" rel="noopener noreferrer"
                className="flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 shadow-sm shadow-violet-200 dark:shadow-none transition-colors">
                <ExternalLink size={14} />{t.skills.visitSite}
              </a>
            </div>
          </div>

          <div className="relative">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input value={search} onChange={e => setSearch(e.target.value)} placeholder={t.skills.searchClawHub}
              className="w-full pl-9 pr-4 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500 transition-all" />
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {hubFiltered.length === 0 ? (
              <div className="col-span-full flex flex-col items-center justify-center py-16 text-gray-400 border-2 border-dashed border-gray-100 dark:border-gray-800 rounded-xl">
                <Package size={32} className="opacity-20 mb-2" />
                <p className="text-sm">{t.skills.noSkillsFound}</p>
              </div>
            ) : hubFiltered.map(skill => {
              const isInstalled = installedIds.has(skill.id);
              return (
                <div key={skill.id} className="bg-white dark:bg-gray-800 rounded-xl p-4 shadow-sm border border-gray-100 dark:border-gray-700/50 flex flex-col h-full hover:shadow-md transition-all group">
                  <div className="flex items-start justify-between mb-3">
                    <div className="flex items-center gap-3">
                      <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-blue-50 to-cyan-50 dark:from-blue-900/30 dark:to-cyan-900/30 flex items-center justify-center shrink-0 border border-blue-100 dark:border-blue-800/30">
                        <Globe size={18} className="text-blue-600 dark:text-blue-400" />
                      </div>
                      <div>
                        <h4 className="text-sm font-bold text-gray-900 dark:text-white line-clamp-1" title={skill.name}>{skill.name}</h4>
                        <div className="flex items-center gap-2 mt-0.5">
                          <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-gray-500 font-mono">v{skill.version}</span>
                          <span className="text-[10px] px-1.5 py-0.5 rounded bg-cyan-50 dark:bg-cyan-900/30 text-cyan-600 dark:text-cyan-400">{skill.category}</span>
                        </div>
                      </div>
                    </div>
                  </div>
                  
                  <div className="flex-1 mb-4">
                    <p className="text-xs text-gray-500 line-clamp-2 mb-1" title={skill.description}>{skill.description}</p>
                    {skill.descriptionZh && <p className="text-xs text-gray-400 dark:text-gray-500 line-clamp-2">{skill.descriptionZh}</p>}
                  </div>
                  
                  <div className="flex items-center gap-2 pt-3 border-t border-gray-50 dark:border-gray-800">
                    <a href={`https://clawhub.ai/skills/${skill.id}`} target="_blank" rel="noopener noreferrer"
                      className="p-2 rounded-lg text-gray-400 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/30 transition-colors" title="在 ClawHub 查看详情">
                      <ExternalLink size={16} />
                    </a>
                    {isInstalled ? (
                      <button disabled className="flex-1 flex items-center justify-center gap-1.5 py-2 text-xs font-medium rounded-lg bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600 dark:text-emerald-400 cursor-default">
                        <Check size={14} />{t.common.installed}
                      </button>
                    ) : (
                      <button onClick={() => handleInstallHint(skill.id)} disabled={installing === skill.id}
                        className="flex-1 flex items-center justify-center gap-1.5 py-2 text-xs font-medium rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 hover:bg-gray-800 dark:hover:bg-gray-100 disabled:opacity-50 transition-colors">
                        {installing === skill.id ? <Loader2 size={14} className="animate-spin" /> : <Download size={14} />}
                        {t.common.install}
                      </button>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Config Modal */}
      {configSkill && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4" onClick={() => setConfigSkill(null)}>
          <div className="bg-white dark:bg-gray-900 rounded-2xl shadow-2xl max-w-2xl w-full max-h-[85vh] overflow-hidden flex flex-col border border-gray-100 dark:border-gray-800" onClick={e => e.stopPropagation()}>
            <div className="px-5 py-4 border-b border-gray-100 dark:border-gray-800 flex items-center justify-between bg-gray-50/50 dark:bg-gray-900">
              <h3 className="text-sm font-bold text-gray-900 dark:text-white flex items-center gap-2">
                <Sparkles size={16} className="text-violet-500" />
                {configSkill.name} {t.skills.configRequirements}
              </h3>
              <button onClick={() => setConfigSkill(null)} className="p-1 rounded-md text-gray-400 hover:text-gray-600 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors">
                <X size={18} />
              </button>
            </div>
            <div className="flex-1 overflow-auto p-6 space-y-6">
              {configSkill.requires?.env && configSkill.requires.env.length > 0 && (
                <div className="space-y-3">
                  <h4 className="text-xs font-bold text-gray-500 uppercase tracking-wider flex items-center gap-2">
                    <Key size={14} /> {t.skills.envVars}
                  </h4>
                  {configSkill.requires.env.map(envVar => (
                    <div key={envVar} className="bg-gray-50 dark:bg-gray-800/50 rounded-xl p-4 border border-gray-100 dark:border-gray-800">
                      <div className="flex items-center justify-between mb-2">
                        <code className="text-sm font-bold font-mono text-violet-600 dark:text-violet-400 bg-violet-50 dark:bg-violet-900/30 px-2 py-0.5 rounded">{envVar}</code>
                        <span className="text-[10px] px-2 py-0.5 rounded-full bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400 font-medium">{t.common.required}</span>
                      </div>
                      <p className="text-xs text-gray-600 dark:text-gray-400 mb-3">
                        请在 OpenClaw 配置中设置此环境变量，或在系统配置的"认证密钥"部分添加。
                      </p>
                      <div className="bg-gray-900 dark:bg-black rounded-lg p-3 text-xs font-mono text-gray-300 space-y-2 overflow-x-auto">
                        <div>
                          <span className="text-gray-500 block mb-1"># 方法1: ~/.openclaw/openclaw.json</span>
                          <span className="text-emerald-400">"env"</span>: <span className="text-yellow-300">{"{"}</span> <span className="text-emerald-400">"vars"</span>: <span className="text-yellow-300">{"{"}</span> <span className="text-cyan-300">"{envVar}"</span>: <span className="text-orange-300">"your_key_here"</span> <span className="text-yellow-300">{"}"}</span> <span className="text-yellow-300">{"}"}</span>
                        </div>
                        <div>
                          <span className="text-gray-500 block mb-1"># 方法2: 系统环境变量</span>
                          <span className="text-purple-400">export</span> {envVar}=<span className="text-orange-300">"your_key_here"</span>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
              {configSkill.requires?.bins && configSkill.requires.bins.length > 0 && (
                <div className="space-y-3">
                  <h4 className="text-xs font-bold text-gray-500 uppercase tracking-wider flex items-center gap-2">
                    <Package size={14} /> {t.skills.binTools}
                  </h4>
                  {configSkill.requires.bins.map(bin => (
                    <div key={bin} className="bg-gray-50 dark:bg-gray-800/50 rounded-xl p-4 border border-gray-100 dark:border-gray-800">
                      <div className="flex items-center justify-between mb-2">
                        <code className="text-sm font-bold font-mono text-blue-600 dark:text-blue-400 bg-blue-50 dark:bg-blue-900/30 px-2 py-0.5 rounded">{bin}</code>
                        <span className="text-[10px] px-2 py-0.5 rounded-full bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400 font-medium">{t.skills.cliTool}</span>
                      </div>
                      <p className="text-xs text-gray-600 dark:text-gray-400 mb-3">
                        此技能需要系统中安装 <code className="px-1.5 py-0.5 bg-gray-200 dark:bg-gray-700 rounded text-xs font-bold">{bin}</code> 命令行工具。
                      </p>
                      {configSkill.metadata?.openclaw?.install && (
                        <div className="bg-gray-900 dark:bg-black rounded-lg p-3 text-xs font-mono text-gray-300 space-y-3">
                          {configSkill.metadata.openclaw.install.map((inst: any, idx: number) => (
                            <div key={idx}>
                              <div className="text-gray-500 mb-1"># {inst.label || `安装方法 ${idx + 1}`} ({inst.kind})</div>
                              <div className="flex items-center gap-2">
                                <span className="text-emerald-400">$</span>
                                {inst.kind === 'brew' && <span>brew install {inst.formula}</span>}
                                {inst.kind === 'apt' && <span>sudo apt install -y {inst.package}</span>}
                                {inst.kind === 'npm' && <span>npm install -g {inst.package}</span>}
                                {inst.kind === 'pip' && <span>pip install {inst.package}</span>}
                                {inst.kind === 'go' && <span>go install {inst.package}</span>}
                                {inst.kind === 'cargo' && <span>cargo install {inst.package}</span>}
                              </div>
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              )}
              {configSkill.path && (
                <div className="text-xs text-gray-400 pt-4 border-t border-gray-100 dark:border-gray-800 flex items-center gap-2">
                  <FolderOpen size={14} />
                  <span>{t.skills.installPath}:</span>
                  <code className="text-gray-600 dark:text-gray-300 bg-gray-100 dark:bg-gray-800 px-2 py-0.5 rounded font-mono text-[11px]">{configSkill.path}</code>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

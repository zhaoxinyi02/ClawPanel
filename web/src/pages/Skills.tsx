import { useEffect, useState } from 'react';
import { api } from '../lib/api';
import {
  Sparkles, Search, ToggleLeft, ToggleRight, ExternalLink, Download,
  Settings2, Trash2, RefreshCw, Package, Globe, ChevronRight,
} from 'lucide-react';

interface SkillEntry {
  id: string;
  name: string;
  description?: string;
  enabled: boolean;
  source: 'bundled' | 'installed' | 'clawhub';
  version?: string;
  author?: string;
}

export default function Skills() {
  const [skills, setSkills] = useState<SkillEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState<'all' | 'enabled' | 'disabled'>('all');
  const [tab, setTab] = useState<'installed' | 'clawhub'>('installed');
  const [msg, setMsg] = useState('');

  useEffect(() => {
    loadSkills();
  }, []);

  const loadSkills = async () => {
    setLoading(true);
    try {
      const r = await api.getOpenClawConfig();
      if (r.ok && r.config) {
        const skillEntries = r.config.skills?.entries || {};
        const parsed: SkillEntry[] = Object.entries(skillEntries).map(([id, cfg]: [string, any]) => ({
          id,
          name: id.replace(/-/g, ' ').replace(/\b\w/g, (c: string) => c.toUpperCase()),
          description: cfg.description || '',
          enabled: cfg.enabled !== false,
          source: 'bundled' as const,
          version: cfg.version,
        }));
        // Also scan skills directory
        try {
          const skillsDir = await api.workspaceFiles('../../.openclaw/skills');
          if (skillsDir.ok && skillsDir.files) {
            for (const f of skillsDir.files) {
              if (f.isDirectory && !parsed.find(s => s.id === f.name)) {
                parsed.push({
                  id: f.name,
                  name: f.name.replace(/-/g, ' ').replace(/\b\w/g, (c: string) => c.toUpperCase()),
                  enabled: true,
                  source: 'installed',
                });
              }
            }
          }
        } catch {}
        setSkills(parsed);
      }
    } catch (err) {
      console.error('Failed to load skills:', err);
    } finally {
      setLoading(false);
    }
  };

  const toggleSkill = async (id: string) => {
    const skill = skills.find(s => s.id === id);
    if (!skill) return;
    const newEnabled = !skill.enabled;
    setSkills(prev => prev.map(s => s.id === id ? { ...s, enabled: newEnabled } : s));
    try {
      const r = await api.getOpenClawConfig();
      if (r.ok) {
        const config = r.config || {};
        if (!config.skills) config.skills = {};
        if (!config.skills.entries) config.skills.entries = {};
        config.skills.entries[id] = { ...config.skills.entries[id], enabled: newEnabled };
        await api.updateOpenClawConfig(config);
        setMsg(`${skill.name} 已${newEnabled ? '启用' : '禁用'}`);
        setTimeout(() => setMsg(''), 2000);
      }
    } catch (err) {
      setSkills(prev => prev.map(s => s.id === id ? { ...s, enabled: !newEnabled } : s));
      setMsg('操作失败');
      setTimeout(() => setMsg(''), 2000);
    }
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

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold">技能中心</h2>
          <p className="text-xs text-gray-500 mt-0.5">管理 OpenClaw 的技能插件</p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={loadSkills} className="flex items-center gap-1.5 px-2.5 py-1.5 text-xs rounded-lg border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800 text-gray-600 dark:text-gray-400">
            <RefreshCw size={13} />刷新
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-gray-200 dark:border-gray-800">
        <button onClick={() => setTab('installed')}
          className={`px-4 py-2 text-xs font-medium border-b-2 transition-colors ${tab === 'installed' ? 'border-violet-500 text-violet-600 dark:text-violet-400' : 'border-transparent text-gray-500 hover:text-gray-700'}`}>
          <Package size={13} className="inline mr-1.5" />已安装 ({skills.length})
        </button>
        <button onClick={() => setTab('clawhub')}
          className={`px-4 py-2 text-xs font-medium border-b-2 transition-colors ${tab === 'clawhub' ? 'border-violet-500 text-violet-600 dark:text-violet-400' : 'border-transparent text-gray-500 hover:text-gray-700'}`}>
          <Globe size={13} className="inline mr-1.5" />ClawHub 商店
        </button>
      </div>

      {msg && (
        <div className={`px-3 py-2 rounded-lg text-xs ${msg.includes('失败') ? 'bg-red-50 dark:bg-red-950 text-red-600' : 'bg-emerald-50 dark:bg-emerald-950 text-emerald-600'}`}>
          {msg}
        </div>
      )}

      {tab === 'installed' && (
        <>
          {/* Filters */}
          <div className="flex items-center gap-2">
            <div className="relative flex-1 max-w-xs">
              <Search size={13} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-gray-400" />
              <input value={search} onChange={e => setSearch(e.target.value)} placeholder="搜索技能..."
                className="w-full pl-8 pr-3 py-1.5 text-xs border border-gray-200 dark:border-gray-700 rounded-lg bg-transparent" />
            </div>
            <div className="flex gap-1">
              {(['all', 'enabled', 'disabled'] as const).map(f => (
                <button key={f} onClick={() => setFilter(f)}
                  className={`px-2.5 py-1 text-[11px] rounded-md transition-colors ${filter === f ? 'bg-violet-100 dark:bg-violet-900 text-violet-700 dark:text-violet-300 font-medium' : 'text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800'}`}>
                  {f === 'all' ? '全部' : f === 'enabled' ? '已启用' : '已禁用'}
                </button>
              ))}
            </div>
          </div>

          {/* Skills list */}
          {loading ? (
            <div className="text-center py-12 text-gray-400 text-xs">加载中...</div>
          ) : filtered.length === 0 ? (
            <div className="text-center py-12 text-gray-400 text-xs">
              {search ? '没有匹配的技能' : '暂无已安装的技能'}
            </div>
          ) : (
            <div className="grid gap-2">
              {filtered.map(skill => (
                <div key={skill.id} className="card px-4 py-3 flex items-center gap-3">
                  <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-violet-100 to-indigo-100 dark:from-violet-900 dark:to-indigo-900 flex items-center justify-center shrink-0">
                    <Sparkles size={14} className="text-violet-600 dark:text-violet-400" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium">{skill.name}</span>
                      <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-800 text-gray-500">{skill.id}</span>
                      {skill.source === 'bundled' && <span className="text-[10px] px-1.5 py-0.5 rounded bg-blue-50 dark:bg-blue-950 text-blue-600 dark:text-blue-400">内置</span>}
                    </div>
                    {skill.description && <p className="text-[11px] text-gray-500 mt-0.5 truncate">{skill.description}</p>}
                  </div>
                  <button onClick={() => toggleSkill(skill.id)}
                    className={`shrink-0 ${skill.enabled ? 'text-violet-500' : 'text-gray-300 dark:text-gray-600'}`}>
                    {skill.enabled ? <ToggleRight size={24} /> : <ToggleLeft size={24} />}
                  </button>
                </div>
              ))}
            </div>
          )}
        </>
      )}

      {tab === 'clawhub' && (
        <div className="card p-8 text-center">
          <Globe size={32} className="mx-auto text-gray-300 dark:text-gray-600 mb-3" />
          <h3 className="font-semibold text-sm mb-1">ClawHub 技能商店</h3>
          <p className="text-xs text-gray-500 mb-4">浏览社区贡献的技能，一键安装到你的 OpenClaw</p>
          <a href="https://clawhub.ai/skills" target="_blank" rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 px-4 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700">
            <ExternalLink size={13} />访问 ClawHub
          </a>
        </div>
      )}
    </div>
  );
}

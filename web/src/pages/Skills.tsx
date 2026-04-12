import { useEffect, useState, useRef, useCallback, useMemo } from 'react';
import type { ChangeEvent } from 'react';
import { useOutletContext } from 'react-router-dom';
import { api } from '../lib/api';
import {
  Sparkles, Search, ToggleLeft, ToggleRight, Download,
  RefreshCw, Package, Globe, Check, Loader2, ExternalLink, X, Key, FolderOpen, Plug, Trash2, ArrowUpCircle, CheckSquare, Square, Upload, Star, TrendingUp, Copy,
} from 'lucide-react';
import { useI18n } from '../i18n';
import MobileActionTray from '../components/MobileActionTray';

interface SkillEntry {
  id: string;
  name: string;
  description?: string;
  enabled: boolean;
  source: string;
  version?: string;
  installedAt?: string;
  metadata?: any;
  requires?: { env?: string[]; bins?: string[]; anyBins?: string[]; config?: string[] };
  path?: string;
  skillKey?: string;
  configSchema?: SkillConfigField[];
}

interface SkillConfigOption {
  label?: string;
  value: unknown;
}

interface SkillConfigField {
  key: string;
  label?: string;
  type?: 'text' | 'password' | 'textarea' | 'select' | 'toggle' | 'number';
  placeholder?: string;
  help?: string;
  required?: boolean;
  options?: SkillConfigOption[];
  defaultValue?: unknown;
}

interface PluginEntry {
  id: string;
  name: string;
  description?: string;
  enabled: boolean;
  source: string;
  version?: string;
  installedAt?: string;
  path?: string;
}

interface ClawHubSkill {
  id: string;
  name: string;
  description: string;
  version?: string;
  installedVersion?: string;
  category?: string;
  author?: string;
  installed?: boolean;
}

interface SkillConfigSnapshot {
  skillId: string;
  skillKey?: string;
  configKeys: string[];
  values: Record<string, unknown>;
}

interface PendingSkillConfigImport {
  fileName: string;
  values: Record<string, unknown>;
}

interface SkillHubSkill {
  slug: string;
  name: string;
  description: string;
  description_zh: string;
  version: string;
  homepage?: string;
  installed?: boolean;
  installState?: string;
  installMessage?: string;
  lastInstallAt?: number;
  tags: string[];
  downloads: number;
  stars: number;
  score: number;
  owner: string;
  updated_at: number;
}

interface SkillHubCatalog {
  ok: boolean;
  total: number;
  generatedAt: string;
  featured: string[];
  categories: Record<string, string[]>;
  skills: SkillHubSkill[];
}

interface SkillHubStatus {
  ok: boolean;
  installed: boolean;
  binPath?: string;
  installGuideURL?: string;
  skillInstallCommand?: string;
  error?: string;
  missingPython?: boolean;
  installHint?: string;
}

type SkillScopeFilter = 'all' | 'current-agent' | 'global-shared' | 'built-in' | 'plugin' | 'custom';
type StoreInstallTarget = 'agent' | 'global';

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

function normalizeSkillConfigFields(skill: SkillEntry | null): SkillConfigField[] {
  if (!skill) return [];
  const schema = Array.isArray(skill.configSchema) ? skill.configSchema.filter(field => field && typeof field.key === 'string' && field.key) : [];
  const seen = new Set(schema.map(field => field.key));
  const declared = Array.isArray(skill.requires?.config) ? skill.requires!.config : [];
  const merged = [...schema];
  declared.forEach((key) => {
    if (!key || seen.has(key)) return;
    merged.push({ key, type: 'text' });
  });
  return merged;
}

function normalizeConfigInputValue(field: SkillConfigField, value: unknown): string | number | boolean {
  if (field.type === 'toggle') return value === true;
  if (field.type === 'number') return typeof value === 'number' ? value : (typeof value === 'string' && value.trim() ? Number(value) : '');
  return typeof value === 'string' ? value : (value == null ? '' : String(value));
}

function normalizeClawHubRegistryBase(value: unknown): string {
  if (typeof value !== 'string' || !value.trim()) return '';
  try {
    const parsed = new URL(value);
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return '';
    parsed.username = '';
    parsed.password = '';
    parsed.search = '';
    parsed.hash = '';
    parsed.pathname = parsed.pathname.replace(/\/+$/, '') || '/';
    return parsed.toString().replace(/\/+$/, '');
  } catch {
    return '';
  }
}

function buildClawHubLink(base: string, path: string): string {
  const normalizedBase = normalizeClawHubRegistryBase(base);
  if (!normalizedBase) return '';
  return `${normalizedBase}${path}`;
}

export default function Skills() {
  const { t, locale } = useI18n();
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const modern = uiMode === 'modern';
  const [skills, setSkills] = useState<SkillEntry[]>([]);
  const [plugins, setPlugins] = useState<PluginEntry[]>([]);
  const [clawHubSkills, setClawHubSkills] = useState<ClawHubSkill[]>([]);
  const [clawHubRegistryBase, setClawHubRegistryBase] = useState('');
  const [agents, setAgents] = useState<any[]>([]);
  const [selectedAgent, setSelectedAgent] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [hubLoading, setHubLoading] = useState(false);
  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState<'all' | 'enabled' | 'disabled'>('all');
  const [tab, setTab] = useState<'skills' | 'plugins' | 'clawhub'>('skills');
  const [msg, setMsg] = useState('');
  const [installing, setInstalling] = useState('');
  const [bulkInstallingStore, setBulkInstallingStore] = useState(false);
  const [uninstalling, setUninstalling] = useState('');
  const [updating, setUpdating] = useState('');
  const [confirmUninstall, setConfirmUninstall] = useState<{ id: string; name: string; installTarget: StoreInstallTarget } | null>(null);
  const [detailSkill, setDetailSkill] = useState<any | null>(null);
  const [selectedSkills, setSelectedSkills] = useState<Set<string>>(new Set());
  const [selectedPlugins, setSelectedPlugins] = useState<Set<string>>(new Set());
  const [copyTarget, setCopyTarget] = useState<{ skillId: string; skillName: string } | null>(null);
  const [syncing, setSyncing] = useState(false);
  const [configSkill, setConfigSkill] = useState<SkillEntry | null>(null);
  const [configSnapshot, setConfigSnapshot] = useState<SkillConfigSnapshot | null>(null);
  const [configLoading, setConfigLoading] = useState(false);
  const [configAction, setConfigAction] = useState<'export' | 'import' | 'save' | ''>('');
  const [configDraft, setConfigDraft] = useState<Record<string, unknown>>({});
  const [pendingConfigImport, setPendingConfigImport] = useState<PendingSkillConfigImport | null>(null);
  const [hubCategory, setHubCategory] = useState<string>('all');
  const [hubPage, setHubPage] = useState(1);
  const [hubTotal, setHubTotal] = useState(0);
  const hubLimit = 30;
  const [depResults, setDepResults] = useState<Record<string, { allMet: boolean; missing: string[] }>>({});
  const [scopeFilter, setScopeFilter] = useState<SkillScopeFilter>('all');
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const configImportRef = useRef<HTMLInputElement>(null);

  // SkillHub state
  const [hubSource, setHubSource] = useState<'clawhub' | 'skillhub'>('clawhub');
  const [skillHubCatalog, setSkillHubCatalog] = useState<SkillHubCatalog | null>(null);
  const [skillHubLoading, setSkillHubLoading] = useState(false);
  const [skillHubError, setSkillHubError] = useState('');
  const [skillHubSearch, setSkillHubSearch] = useState('');
  const [skillHubCategory, setSkillHubCategory] = useState<string>('all');
  const [skillHubView, setSkillHubView] = useState<'featured' | 'category' | 'all'>('featured');
  const [skillHubPage, setSkillHubPage] = useState(1);
  const [storeView, setStoreView] = useState<'grid5' | 'list'>('grid5');
  const [storeInstallTarget, setStoreInstallTarget] = useState<StoreInstallTarget>('agent');
  const [skillHubCliStatus, setSkillHubCliStatus] = useState<SkillHubStatus | null>(null);
  const [skillHubCliLoading, setSkillHubCliLoading] = useState(false);
  const [skillHubCliInstalling, setSkillHubCliInstalling] = useState(false);
  const [storeEverLoaded, setStoreEverLoaded] = useState(false);
  const [skillHubStoreEverLoaded, setSkillHubStoreEverLoaded] = useState(false);
  const skillHubPageSize = 30;
  const configFields = useMemo(() => normalizeSkillConfigFields(configSkill), [configSkill]);
  const hasStructuredConfig = configFields.length > 0;

  const debouncedHubSearch = useCallback(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => { loadClawHub(); }, 300);
  }, []);

  useEffect(() => { loadAgents(); }, []);
  useEffect(() => { loadSkills(); }, [selectedAgent]);
  // Store data is loaded on-demand via manual refresh buttons only
  useEffect(() => { return () => { if (debounceRef.current) clearTimeout(debounceRef.current); }; }, []);

  const loadAgents = async () => {
    try {
      const r = await api.getAgentsConfig();
      if (r.ok && r.agents) {
        const list = Array.isArray(r.agents?.list) ? r.agents.list : [];
        const defaultAgent = typeof r.agents?.default === 'string' ? r.agents.default : '';
        setAgents(list);
        setSelectedAgent(prev => prev || defaultAgent || list[0]?.id || '');
      }
    } catch (err) {
      console.error('Failed to load agents:', err);
    }
  };

  const loadSkills = async () => {
    setLoading(true);
    try {
      const r = await api.getSkills(selectedAgent);
      if (r.ok) {
        setSkills(r.skills || []);
        setPlugins(r.plugins || []);
      }
    } catch (err) {
      console.error('Failed to load skills:', err);
    } finally { setLoading(false); }
  };

  const checkDeps = async (skill: any) => {
    if (!skill.requires) return;
    const env = skill.requires.env as string[] | undefined;
    const bins = skill.requires.bins as string[] | undefined;
    const anyBins = skill.requires.anyBins as string[] | undefined;
    if (!env?.length && !bins?.length && !anyBins?.length) return;
    try {
      const r = await api.checkSkillDeps(env, bins, anyBins);
      if (r.ok) {
        const missing: string[] = [];
        (r.env || []).forEach((e: any) => { if (!e.found) missing.push(`env:${e.name}`); });
        (r.bins || []).forEach((b: any) => { if (!b.found) missing.push(`bin:${b.name}`); });
        setDepResults(prev => ({ ...prev, [skill.id]: { allMet: r.allMet, missing } }));
      }
    } catch { /* ignore */ }
  };

  const loadClawHub = async (page?: number) => {
    setHubLoading(true);
    const p = page ?? hubPage;
    try {
      const r = await api.searchClawHub(search, selectedAgent, p, hubLimit, storeInstallTarget);
      if (r.ok && r.skills) {
        setClawHubSkills(r.skills || []);
        setClawHubRegistryBase(normalizeClawHubRegistryBase(r.registryBase));
        if (typeof r.total === 'number') setHubTotal(r.total);
        setStoreEverLoaded(true);
        (r.skills || []).forEach((s: any) => {
          if (s.requires && !s.installed) checkDeps(s);
        });
      }
    } catch (err) {
      console.error('Failed to load ClawHub:', err);
    } finally { setHubLoading(false); }
  };

  const loadSkillHub = async () => {
    setSkillHubLoading(true);
    setSkillHubError('');
    try {
      const r = await api.getSkillHubCatalog(selectedAgent, storeInstallTarget);
      if (r.ok) { setSkillHubCatalog(r as SkillHubCatalog); setSkillHubStoreEverLoaded(true); }
      else setSkillHubError(r.error || t.skills.skillHubLoadError);
    } catch (err) {
      console.error('Failed to load SkillHub:', err);
      setSkillHubError(t.skills.skillHubLoadError);
    } finally { setSkillHubLoading(false); }
  };

  const loadSkillHubStatus = async (force = false) => {
    if (skillHubCliLoading && !force) return;
    setSkillHubCliLoading(true);
    try {
      const r = await api.getSkillHubStatus();
      if (r.ok) setSkillHubCliStatus(r as SkillHubStatus);
      else setSkillHubCliStatus({ ok: false, installed: false, error: r.error });
    } catch (err) {
      console.error('Failed to load SkillHub CLI status:', err);
      setSkillHubCliStatus({ ok: false, installed: false, error: t.skills.skillHubCliRequired });
    } finally { setSkillHubCliLoading(false); }
  };

  // Custom marketplace auto refresh on entry
  useEffect(() => {
    if (tab === 'clawhub') {
      if (hubSource === 'clawhub') {
        loadClawHub(1);
      } else if (!skillHubCliStatus && !skillHubCliLoading) {
        loadSkillHubStatus();
      }
    }
  }, [tab, hubSource, selectedAgent, storeInstallTarget]); // eslint-disable-line react-hooks/exhaustive-deps

  const getSkillScope = (source: string): Exclude<SkillScopeFilter, 'all'> => {
    switch (source) {
      case 'workspace':
      case 'workspace-agent':
      case 'installed':
        return 'current-agent';
      case 'managed':
      case 'global-agent':
        return 'global-shared';
      case 'app-skill':
        return 'built-in';
      case 'plugin-skill':
        return 'plugin';
      default:
        return 'custom';
    }
  };

  const getSkillScopeBadge = (source: string) => {
    const scope = getSkillScope(source);
    const badges: Record<Exclude<SkillScopeFilter, 'all'>, { label: string; color: string }> = {
      'current-agent': { label: t.skills.scopeBadgeCurrentAgent, color: 'bg-blue-100 dark:bg-blue-950 text-blue-700 dark:text-blue-300' },
      'global-shared': { label: t.skills.scopeBadgeGlobalShared, color: 'bg-emerald-100 dark:bg-emerald-950 text-emerald-700 dark:text-emerald-300' },
      'built-in': { label: t.skills.scopeBadgeBuiltIn, color: 'bg-indigo-100 dark:bg-indigo-950 text-indigo-700 dark:text-indigo-300' },
      plugin: { label: t.skills.scopeBadgePlugin, color: 'bg-violet-100 dark:bg-violet-950 text-violet-700 dark:text-violet-300' },
      custom: { label: t.skills.scopeBadgeCustom, color: 'bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300' },
    };
    const badge = badges[scope];
    return <span className={`px-2 py-0.5 rounded-full text-[10px] font-medium ${badge.color}`}>{badge.label}</span>;
  };

  const getLocalSkillInstallTarget = (skill: SkillEntry): StoreInstallTarget => (
    skill.source === 'managed' || skill.source === 'global-agent' ? 'global' : 'agent'
  );

  const isSkillInTarget = (skill: SkillEntry, target: StoreInstallTarget) => getLocalSkillInstallTarget(skill) === target;

  const matchesSkillSlugInTarget = (slug: string, target: StoreInstallTarget) => skills.some(s => (
    isSkillInTarget(s, target) &&
    (s.id === slug || s.skillKey === slug || s.path?.split(/[\\/]/).pop() === slug)
  ));

  // SkillHub client-side filtering
  const filteredSkillHubSkills = useMemo(() => {
    if (!skillHubCatalog) return [];
    let items = skillHubCatalog.skills;

    if (skillHubView === 'featured') {
      const featuredSet = new Set(skillHubCatalog.featured);
      items = items.filter(s => featuredSet.has(s.slug));
    } else if (skillHubView === 'category' && skillHubCategory !== 'all') {
      const tags = skillHubCatalog.categories[skillHubCategory] || [];
      const tagSet = new Set(tags);
      items = items.filter(s => s.tags?.some(t => tagSet.has(t)));
    }

    if (skillHubSearch.trim()) {
      const q = skillHubSearch.toLowerCase();
      items = items.filter(s =>
        s.slug.toLowerCase().includes(q) ||
        s.name.toLowerCase().includes(q) ||
        s.description.toLowerCase().includes(q) ||
        (s.description_zh || '').toLowerCase().includes(q)
      );
    }

    return [...items].sort((a, b) => b.score - a.score);
  }, [skillHubCatalog, skillHubView, skillHubCategory, skillHubSearch]);

  const skillHubTotalPages = Math.ceil(filteredSkillHubSkills.length / skillHubPageSize);
  const skillHubPagedSkills = filteredSkillHubSkills.slice(
    (skillHubPage - 1) * skillHubPageSize,
    skillHubPage * skillHubPageSize
  );

  // Reset page when filters change
  useEffect(() => { setSkillHubPage(1); }, [skillHubView, skillHubCategory, skillHubSearch]);

  const handleInstallSkillHubCLI = async () => {
    if (skillHubCliInstalling) return;
    setSkillHubCliInstalling(true);
    try {
      const r = await api.installSkillHubCLI();
      if (r.ok) {
        setMsg(t.skills.skillHubCliInstallSuccess);
        await loadSkillHubStatus(true);
      } else {
        setMsg(r.installHint || r.error || t.skills.skillHubCliInstallFailed);
        await loadSkillHubStatus(true);
      }
    } catch (err) {
      console.error('Failed to install SkillHub CLI:', err);
      setMsg(t.skills.skillHubCliInstallFailed);
    } finally {
      setSkillHubCliInstalling(false);
      setTimeout(() => setMsg(''), 3000);
    }
  };

  const handleInstallSkillHubSkill = async (slug: string) => {
    if (!skillHubCliStatus?.installed) {
      setMsg(t.skills.skillHubCliRequired);
      setTimeout(() => setMsg(''), 3000);
      return;
    }
    setInstalling(slug);
    try {
      const r = await api.installSkillHubSkill(slug, selectedAgent, storeInstallTarget);
      if (r.ok) {
        setMsg(t.skills.installSuccess.replace('{id}', slug));
        await Promise.all([loadSkills(), loadSkillHub()]);
      } else {
        setMsg(r.error || t.skills.installFailed.replace('{id}', slug));
        await loadSkillHub();
      }
    } catch (err) {
      setMsg(t.skills.installFailed.replace('{id}', slug));
      await loadSkillHub();
    } finally {
      setInstalling('');
      setTimeout(() => setMsg(''), 3000);
    }
  };

  const isSkillHubInstalled = (skill: SkillHubSkill) => !!skill.installed || matchesSkillSlugInTarget(skill.slug, storeInstallTarget);

  const handleBulkInstallStore = async () => {
    if (bulkInstallingStore) return;
    const clawHubTargets = hubFiltered.filter(skill => !skill.installed);
    const skillHubTargets = skillHubPagedSkills.filter(skill => !isSkillHubInstalled(skill));
    const total = hubSource === 'clawhub' ? clawHubTargets.length : skillHubTargets.length;
    if (total === 0) return;

    setBulkInstallingStore(true);
    let ok = 0;
    try {
      if (hubSource === 'clawhub') {
        for (const skill of clawHubTargets) {
          const r = await api.installClawHubSkill(skill.id, selectedAgent, storeInstallTarget);
          if (r.ok) ok++;
        }
        await Promise.all([loadClawHub(hubPage), loadSkills()]);
      } else {
        if (!skillHubCliStatus?.installed) {
          const cli = await api.installSkillHubCLI();
          if (!cli.ok) {
            setMsg(cli.error || t.skills.skillHubCliInstallFailed);
            return;
          }
          await loadSkillHubStatus(true);
        }
        for (const skill of skillHubTargets) {
          const r = await api.installSkillHubSkill(skill.slug, selectedAgent, storeInstallTarget);
          if (r.ok) ok++;
        }
        await Promise.all([loadSkillHub(), loadSkills()]);
      }
      setMsg(t.skills.bulkInstallStoreResult.replace('{ok}', String(ok)).replace('{total}', String(total)));
    } catch (err) {
      console.error('Failed to bulk install store skills:', err);
      setMsg(t.common.operationFailed);
    } finally {
      setBulkInstallingStore(false);
      setTimeout(() => setMsg(''), 3000);
    }
  };

  const toggleSkill = async (skill: SkillEntry) => {
    const key = skill.skillKey || skill.id;
    if (!key) return;
    if (!skill) return;
    const newEnabled = !skill.enabled;
    setSkills(prev => prev.map(s => (s.skillKey || s.id) === key ? { ...s, enabled: newEnabled } : s));
    try {
      const aliases = skill.skillKey && skill.skillKey !== skill.id ? [skill.id] : undefined;
      await api.toggleSkill(key, newEnabled, aliases);
      setMsg(`${skill.name} ${newEnabled ? t.common.enabled : t.common.disabled}`);
      setTimeout(() => setMsg(''), 2000);
    } catch {
      setSkills(prev => prev.map(s => (s.skillKey || s.id) === key ? { ...s, enabled: !newEnabled } : s));
      setMsg(t.common.operationFailed);
      setTimeout(() => setMsg(''), 2000);
    }
  };

  const bulkToggleSkills = async (enable: boolean) => {
    if (selectedSkills.size === 0) return;
    const targets = skills.filter(s => selectedSkills.has(s.skillKey || s.id));
    setSkills(prev => prev.map(s => selectedSkills.has(s.skillKey || s.id) ? { ...s, enabled: enable } : s));
    let ok = 0;
    for (const skill of targets) {
      try {
        const key = skill.skillKey || skill.id;
        const aliases = skill.skillKey && skill.skillKey !== skill.id ? [skill.id] : undefined;
        await api.toggleSkill(key, enable, aliases);
        ok++;
      } catch { /* individual failure handled by reload */ }
    }
    setMsg(`${ok}/${targets.length} ${enable ? t.common.enabled : t.common.disabled}`);
    setTimeout(() => setMsg(''), 2000);
    setSelectedSkills(new Set());
    loadSkills();
  };

  const togglePlugin = async (id: string) => {
    const plugin = plugins.find(p => p.id === id);
    if (!plugin) return;
    const newEnabled = !plugin.enabled;
    setPlugins(prev => prev.map(p => p.id === id ? { ...p, enabled: newEnabled } : p));
    try {
      await api.updatePlugin(id, { enabled: newEnabled });
      setMsg(`${plugin.name} ${newEnabled ? t.common.enabled : t.common.disabled}`);
      setTimeout(() => setMsg(''), 2000);
    } catch {
      setPlugins(prev => prev.map(p => p.id === id ? { ...p, enabled: !newEnabled } : p));
      setMsg(t.common.operationFailed);
      setTimeout(() => setMsg(''), 2000);
    }
  };

  const handleSearchClawHub = async () => {
    if (tab === 'clawhub') {
      setHubPage(1);
      loadClawHub(1);
    }
  };

  const handleInstallSkill = async (skillId: string) => {
    setInstalling(skillId);
    try {
      const r = await api.installClawHubSkill(skillId, selectedAgent, storeInstallTarget);
      if (r.ok) {
        setMsg(t.skills.installSuccess.replace('{id}', skillId));
        // Refresh local skills
        await loadSkills();
        // Refresh ClawHub to update install status
        await loadClawHub();
      } else {
        setMsg(t.skills.installFailed.replace('{id}', skillId));
      }
    } catch (err) {
      setMsg(t.skills.installFailed.replace('{id}', skillId));
    } finally {
      setInstalling('');
      setTimeout(() => setMsg(''), 3000);
    }
  };

  const handleUninstallSkill = async (skillId: string, installTarget: StoreInstallTarget = 'agent') => {
    setConfirmUninstall(null);
    setUninstalling(skillId);
    try {
      const r = await api.uninstallSkill(skillId, selectedAgent, installTarget);
      if (r.ok) {
        setMsg(t.skills.uninstallSuccess.replace('{id}', skillId));
        await loadSkills();
        // 刷新当前商店视图以更新安装状态
        if (tab === 'clawhub') {
          if (hubSource === 'skillhub') await loadSkillHub();
          else await loadClawHub();
        }
      } else {
        setMsg(t.skills.uninstallFailed.replace('{id}', skillId));
      }
    } catch {
      setMsg(t.skills.uninstallFailed.replace('{id}', skillId));
    } finally {
      setUninstalling('');
      setTimeout(() => setMsg(''), 3000);
    }
  };

  const bulkTogglePlugins = async (enable: boolean) => {
    const targets = plugins.filter(p => selectedPlugins.has(p.id));
    let count = 0;
    for (const p of targets) {
      try {
        const r = await api.togglePlugin(p.id, enable);
        if (r.ok) count++;
      } catch {}
    }
    setMsg(`${count}/${targets.length} ${enable ? (locale === 'zh-CN' ? '已启用' : 'enabled') : (locale === 'zh-CN' ? '已禁用' : 'disabled')}`);
    setSelectedPlugins(new Set());
    await loadSkills();
    setTimeout(() => setMsg(''), 3000);
  };

  const bulkUninstallPlugins = async () => {
    if (!confirm(locale === 'zh-CN' ? `确定批量卸载 ${selectedPlugins.size} 个插件？` : `Uninstall ${selectedPlugins.size} plugins?`)) return;
    const targets = plugins.filter(p => selectedPlugins.has(p.id));
    let count = 0;
    for (const p of targets) {
      try {
        const r = await api.uninstallPlugin(p.id, false);
        if (r.ok) count++;
      } catch {}
    }
    setMsg(`${count}/${targets.length} ${locale === 'zh-CN' ? '已卸载' : 'uninstalled'}`);
    setSelectedPlugins(new Set());
    await loadSkills();
    setTimeout(() => setMsg(''), 3000);
  };

  const handleUpdateSkill = async (skillId: string) => {
    setUpdating(skillId);
    try {
      const r = await api.installClawHubSkill(skillId, selectedAgent, storeInstallTarget);
      if (r.ok) {
        setMsg(t.skills.updateSuccess.replace('{id}', skillId));
        await loadSkills();
        if (tab === 'clawhub') await loadClawHub();
      } else {
        setMsg(t.skills.updateFailed.replace('{id}', skillId));
      }
    } catch {
      setMsg(t.skills.updateFailed.replace('{id}', skillId));
    } finally {
      setUpdating('');
      setTimeout(() => setMsg(''), 3000);
    }
  };

  const loadSkillConfigSnapshot = useCallback(async (skill: SkillEntry) => {
    const skillKey = skill.skillKey || skill.id;
    if (!skillKey) return;
    setConfigLoading(true);
    try {
      const r = await api.getSkillConfig(skillKey, selectedAgent);
      if (r.ok) {
        setConfigSnapshot({
          skillId: r.skillId || skill.id,
          skillKey: r.skillKey || skillKey,
          configKeys: Array.isArray(r.configKeys) ? r.configKeys : (skill.requires?.config || []),
          values: isPlainObject(r.values) ? r.values : {},
        });
      } else {
        setConfigSnapshot({
          skillId: skill.id,
          skillKey,
          configKeys: skill.requires?.config || [],
          values: {},
        });
      }
    } catch {
      setConfigSnapshot({
        skillId: skill.id,
        skillKey,
        configKeys: skill.requires?.config || [],
        values: {},
      });
    } finally {
      setConfigLoading(false);
    }
  }, [selectedAgent]);

  const handleExportSkillConfig = async (skill: SkillEntry) => {
    const skillKey = skill.skillKey || skill.id;
    if (!skillKey) return;
    setConfigAction('export');
    try {
      const r = await api.getSkillConfig(skillKey, selectedAgent);
      if (!r.ok) {
        setMsg(t.skills.configExportFailed.replace('{id}', skill.name));
      } else {
        const payload = {
          exportedAt: new Date().toISOString(),
          agentId: selectedAgent || '',
          skillId: r.skillId || skill.id,
          skillKey: r.skillKey || skillKey,
          configKeys: Array.isArray(r.configKeys) ? r.configKeys : (skill.requires?.config || []),
          values: isPlainObject(r.values) ? r.values : {},
        };
        const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `${skillKey}-config-${new Date().toISOString().slice(0, 10)}.json`;
        a.click();
        URL.revokeObjectURL(url);
        setMsg(t.skills.configExportSuccess.replace('{id}', skill.name));
      }
    } catch {
      setMsg(t.skills.configExportFailed.replace('{id}', skill.name));
    } finally {
      setConfigAction('');
      setTimeout(() => setMsg(''), 3000);
    }
  };

  const handleSkillConfigImportSelected = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file || !configSkill) return;
    try {
      const text = await file.text();
      const raw = JSON.parse(text) as unknown;
      const imported = isPlainObject(raw) && isPlainObject(raw.values) ? raw.values : raw;
      if (!isPlainObject(imported)) {
        setMsg(t.skills.configImportInvalid);
        setTimeout(() => setMsg(''), 3000);
        return;
      }
      const importedSkill = isPlainObject(raw)
        ? (typeof raw.skillKey === 'string' ? raw.skillKey : typeof raw.skillId === 'string' ? raw.skillId : '')
        : '';
      const currentSkillKey = configSkill.skillKey || configSkill.id;
      if (importedSkill && importedSkill !== currentSkillKey && importedSkill !== configSkill.id) {
        setMsg(t.skills.configImportMismatch.replace('{id}', configSkill.name));
        setTimeout(() => setMsg(''), 3000);
        return;
      }
      const allowed = new Set(configSkill.requires?.config || []);
      const unknownKeys = Object.keys(imported).filter(key => !allowed.has(key));
      if (unknownKeys.length > 0) {
        setMsg(t.skills.configImportUnknown.replace('{keys}', unknownKeys.join(', ')));
        setTimeout(() => setMsg(''), 4000);
        return;
      }
      if (Object.keys(imported).length === 0) {
        setMsg(t.skills.configImportInvalid);
        setTimeout(() => setMsg(''), 3000);
        return;
      }
      setPendingConfigImport({ fileName: file.name, values: imported });
    } catch {
      setMsg(t.skills.configImportInvalid);
      setTimeout(() => setMsg(''), 3000);
    }
  };

  const handleApplySkillConfigImport = async () => {
    if (!configSkill || !pendingConfigImport) return;
    const skillKey = configSkill.skillKey || configSkill.id;
    if (!skillKey) return;
    setConfigAction('import');
    try {
      const r = await api.updateSkillConfig(skillKey, pendingConfigImport.values, selectedAgent);
      if (r.ok) {
        setConfigSnapshot({
          skillId: r.skillId || configSkill.id,
          skillKey: r.skillKey || skillKey,
          configKeys: Array.isArray(r.configKeys) ? r.configKeys : (configSkill.requires?.config || []),
          values: isPlainObject(r.values) ? r.values : {},
        });
        setPendingConfigImport(null);
        setMsg(t.skills.configImportSuccess.replace('{id}', configSkill.name));
      } else {
        setMsg((r.error as string) || t.skills.configImportFailed.replace('{id}', configSkill.name));
      }
    } catch {
      setMsg(t.skills.configImportFailed.replace('{id}', configSkill.name));
    } finally {
      setConfigAction('');
      setTimeout(() => setMsg(''), 3000);
    }
  };

  const handleSaveSkillConfig = async () => {
    if (!configSkill) return;
    const skillKey = configSkill.skillKey || configSkill.id;
    if (!skillKey || configFields.length === 0) return;
    const values: Record<string, unknown> = {};
    configFields.forEach((field) => {
      if (!Object.prototype.hasOwnProperty.call(configDraft, field.key)) return;
      const rawValue = configDraft[field.key];
      if (field.type === 'number' && rawValue === '') {
        return;
      }
      values[field.key] = rawValue;
    });
    setConfigAction('save');
    try {
      const r = await api.updateSkillConfig(skillKey, values, selectedAgent);
      if (r.ok) {
        setConfigSnapshot({
          skillId: r.skillId || configSkill.id,
          skillKey: r.skillKey || skillKey,
          configKeys: Array.isArray(r.configKeys) ? r.configKeys : (configSkill.requires?.config || []),
          values: isPlainObject(r.values) ? r.values : {},
        });
        setMsg(`${configSkill.name} ${t.common.save}`);
      } else {
        setMsg((r.error as string) || t.common.operationFailed);
      }
    } catch {
      setMsg(t.common.operationFailed);
    } finally {
      setConfigAction('');
      setTimeout(() => setMsg(''), 3000);
    }
  };

  useEffect(() => {
    if (!configSkill) {
      setConfigSnapshot(null);
      setConfigDraft({});
      setPendingConfigImport(null);
      setConfigAction('');
      return;
    }
    if (!configSkill.requires?.config?.length) {
      setConfigSnapshot({
        skillId: configSkill.id,
        skillKey: configSkill.skillKey || configSkill.id,
        configKeys: [],
        values: {},
      });
      setConfigDraft({});
      setPendingConfigImport(null);
      return;
    }
    void loadSkillConfigSnapshot(configSkill);
  }, [configSkill, loadSkillConfigSnapshot]);

  useEffect(() => {
    if (!configSkill) return;
    const nextDraft: Record<string, unknown> = {};
    configFields.forEach((field) => {
      if (configSnapshot && Object.prototype.hasOwnProperty.call(configSnapshot.values, field.key)) {
        nextDraft[field.key] = configSnapshot.values[field.key];
      } else if (field.defaultValue !== undefined) {
        nextDraft[field.key] = field.defaultValue;
      } else if (field.type === 'toggle') {
        nextDraft[field.key] = false;
      }
    });
    setConfigDraft(nextDraft);
  }, [configFields, configSnapshot, configSkill]);

  const filtered = skills.filter(s => {
    if (filter === 'enabled' && !s.enabled) return false;
    if (filter === 'disabled' && s.enabled) return false;
    if (scopeFilter !== 'all' && getSkillScope(s.source) !== scopeFilter) return false;
    if (search) {
      const q = search.toLowerCase();
      return s.id.toLowerCase().includes(q) || s.name.toLowerCase().includes(q) || (s.description || '').toLowerCase().includes(q);
    }
    return true;
  });

  const hubFiltered = clawHubSkills.filter(s => {
    if (hubCategory !== 'all' && (s.category || '') !== hubCategory) return false;
    if (!search) return true;
    const q = search.toLowerCase();
    return s.id.toLowerCase().includes(q) || s.name.toLowerCase().includes(q) || (s.description || '').toLowerCase().includes(q);
  });
  const hubCategories = Array.from(new Set(clawHubSkills.map(s => s.category).filter(Boolean) as string[])).sort();
  const clawHubSiteUrl = buildClawHubLink(clawHubRegistryBase, '/skills?sort=downloads');
  const storeBadgeCount = (hubTotal || clawHubSkills.length) + (skillHubCatalog?.total ?? 0);
  const storeGridClasses = storeView === 'grid5'
    ? 'grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-4'
    : 'grid grid-cols-1 gap-4';
  const clawHubDescriptionClamp = storeView === 'grid5' ? 'line-clamp-3' : 'line-clamp-4';
  const skillHubDescriptionClamp = storeView === 'grid5' ? 'line-clamp-2' : 'line-clamp-4';
  const skillHubTagLimit = storeView === 'grid5' ? 3 : 6;
  const installableStoreCount = hubSource === 'clawhub'
    ? hubFiltered.filter(skill => !skill.installed).length
    : skillHubPagedSkills.filter(skill => !isSkillHubInstalled(skill)).length;
  const scopeFilters: Array<{ key: SkillScopeFilter; label: string }> = [
    { key: 'all', label: t.skills.scopeFilterAll },
    { key: 'current-agent', label: t.skills.scopeFilterCurrentAgent },
    { key: 'global-shared', label: t.skills.scopeFilterGlobalShared },
    { key: 'built-in', label: t.skills.scopeFilterBuiltIn },
    { key: 'plugin', label: t.skills.scopeFilterPlugin },
    { key: 'custom', label: t.skills.scopeFilterCustom },
  ];

  const getSourceBadge = (source: string) => {
    const badges: Record<string, { label: string; color: string }> = {
      installed: { label: t.skills.srcInstalled, color: 'bg-emerald-100 dark:bg-emerald-950 text-emerald-700 dark:text-emerald-300' },
      'config-ext': { label: t.skills.srcDevExt, color: 'bg-blue-100 dark:bg-blue-950 text-blue-700 dark:text-blue-300' },
      skill: { label: t.skills.srcSkill, color: 'bg-purple-100 dark:bg-purple-950 text-purple-700 dark:text-purple-300' },
      managed: { label: t.skills.srcManaged, color: 'bg-purple-100 dark:bg-purple-950 text-purple-700 dark:text-purple-300' },
      'app-skill': { label: t.skills.srcAppSkill, color: 'bg-indigo-100 dark:bg-indigo-950 text-indigo-700 dark:text-indigo-300' },
      workspace: { label: t.skills.srcWorkspace, color: 'bg-teal-100 dark:bg-teal-950 text-teal-700 dark:text-teal-300' },
      'workspace-agent': { label: t.skills.srcWorkspaceAgent, color: 'bg-cyan-100 dark:bg-cyan-950 text-cyan-700 dark:text-cyan-300' },
      'global-agent': { label: t.skills.srcGlobalAgent, color: 'bg-sky-100 dark:bg-sky-950 text-sky-700 dark:text-sky-300' },
      'plugin-skill': { label: t.skills.srcPluginSkill, color: 'bg-violet-100 dark:bg-violet-950 text-violet-700 dark:text-violet-300' },
      'extra-dir': { label: t.skills.srcExtraDir, color: 'bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300' },
    };
    const badge = badges[source] || { label: source, color: 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400' };
    return <span className={`px-2 py-0.5 rounded-full text-[10px] font-medium ${badge.color}`}>{badge.label}</span>;
  };

  return (
    <div className={`space-y-6 ${modern ? 'page-modern' : ''}`}>
      <div className={`${modern ? 'page-modern-header' : 'flex items-center justify-between'}`}>
        <div>
          <h2 className={`${modern ? 'page-modern-title' : 'text-xl font-bold text-gray-900 dark:text-white tracking-tight'}`}>{t.skills.title}</h2>
          <p className={`${modern ? 'page-modern-subtitle' : 'text-sm text-gray-500 mt-1'}`}>{t.skills.subtitle}</p>
        </div>
        <MobileActionTray label={t.skills.refreshList}>
          {agents.length > 0 && (
            <select value={selectedAgent} onChange={e => setSelectedAgent(e.target.value)}
              className="px-3 py-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500 transition-all">
              {agents.map(a => (
                <option key={a.id} value={a.id}>{a.id}</option>
              ))}
            </select>
          )}
          <button onClick={loadSkills} className={`${modern ? 'page-modern-action' : 'flex items-center gap-2 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 transition-colors shadow-sm'}`}>
            <RefreshCw size={14} />{t.skills.refreshList}
          </button>
        </MobileActionTray>
      </div>

      {/* Tabs */}
      <div className={`${modern ? 'inline-flex flex-wrap gap-2 p-1 rounded-2xl border border-blue-100/70 bg-[linear-gradient(145deg,rgba(255,255,255,0.78),rgba(239,246,255,0.62))] dark:bg-[linear-gradient(145deg,rgba(10,20,36,0.82),rgba(30,64,175,0.1))] dark:border-blue-800/20 shadow-sm backdrop-blur-xl' : 'flex gap-6 border-b border-gray-200 dark:border-gray-800'}`}>
        <button onClick={() => setTab('skills')}
          className={`${modern ? 'px-3.5 py-2 rounded-xl text-sm font-medium transition-all flex items-center gap-2 border' : 'pb-3 text-sm font-medium border-b-2 transition-all flex items-center gap-2'} ${tab === 'skills' ? (modern ? 'border-blue-100/80 bg-blue-50/85 dark:bg-blue-900/20 dark:border-blue-800/40 text-blue-700 dark:text-blue-300 shadow-sm' : 'border-violet-600 text-violet-700 dark:text-violet-400') : (modern ? 'border-transparent text-gray-500 hover:bg-white/70 dark:hover:bg-slate-800/70 hover:text-gray-700 dark:hover:text-gray-300' : 'border-transparent text-gray-500 hover:text-gray-700 dark:hover:text-gray-300')}`}>
          <Sparkles size={16} />{t.skills.installedTab} <span className="bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 text-xs px-1.5 py-0.5 rounded-full">{skills.length}</span>
        </button>
        <button onClick={() => setTab('plugins')}
          className={`${modern ? 'px-3.5 py-2 rounded-xl text-sm font-medium transition-all flex items-center gap-2 border' : 'pb-3 text-sm font-medium border-b-2 transition-all flex items-center gap-2'} ${tab === 'plugins' ? (modern ? 'border-blue-100/80 bg-blue-50/85 dark:bg-blue-900/20 dark:border-blue-800/40 text-blue-700 dark:text-blue-300 shadow-sm' : 'border-violet-600 text-violet-700 dark:text-violet-400') : (modern ? 'border-transparent text-gray-500 hover:bg-white/70 dark:hover:bg-slate-800/70 hover:text-gray-700 dark:hover:text-gray-300' : 'border-transparent text-gray-500 hover:text-gray-700 dark:hover:text-gray-300')}`}>
          <Plug size={16} />{t.skills.pluginsTab || '插件'} <span className="bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 text-xs px-1.5 py-0.5 rounded-full">{plugins.length}</span>
        </button>
        <button onClick={() => setTab('clawhub')}
          className={`${modern ? 'px-3.5 py-2 rounded-xl text-sm font-medium transition-all flex items-center gap-2 border' : 'pb-3 text-sm font-medium border-b-2 transition-all flex items-center gap-2'} ${tab === 'clawhub' ? (modern ? 'border-blue-100/80 bg-blue-50/85 dark:bg-blue-900/20 dark:border-blue-800/40 text-blue-700 dark:text-blue-300 shadow-sm' : 'border-violet-600 text-violet-700 dark:text-violet-400') : (modern ? 'border-transparent text-gray-500 hover:bg-white/70 dark:hover:bg-slate-800/70 hover:text-gray-700 dark:hover:text-gray-300' : 'border-transparent text-gray-500 hover:text-gray-700 dark:hover:text-gray-300')}`}>
          <Globe size={16} />{t.skills.storeTab} <span className="bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 text-xs px-1.5 py-0.5 rounded-full">{(storeEverLoaded || skillHubStoreEverLoaded) ? storeBadgeCount : '\u00b7\u00b7\u00b7'}</span>
        </button>
      </div>

      {msg && (
        <div className={`px-4 py-3 rounded-xl text-sm font-medium flex items-center gap-2 ${msg.includes('失败') ? 'bg-red-50 dark:bg-red-900/30 text-red-600' : 'bg-emerald-50 dark:bg-emerald-900/30 text-emerald-600'}`}>
          {msg.includes('失败') ? <X size={16} /> : <Check size={16} />}
          {msg}
        </div>
      )}

      {tab === 'skills' && (
        <div className="space-y-4">
          {/* Filters */}
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div className="relative flex-1 min-w-[240px] max-w-md">
              <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input value={search} onChange={e => setSearch(e.target.value)} placeholder={t.skills.searchInstalled}
                className="w-full pl-9 pr-4 py-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500 transition-all" />
            </div>
            <div className={`${modern ? 'flex gap-1 p-1 rounded-xl border border-blue-100/70 bg-[linear-gradient(145deg,rgba(255,255,255,0.72),rgba(239,246,255,0.56))] dark:bg-[linear-gradient(145deg,rgba(10,20,36,0.82),rgba(30,64,175,0.08))] dark:border-blue-800/20 backdrop-blur-xl' : 'flex gap-1 bg-gray-100 dark:bg-gray-800 p-1 rounded-lg'}`}>
              {(['all', 'enabled', 'disabled'] as const).map(f => (
                <button key={f} onClick={() => setFilter(f)}
                  className={`px-3 py-1.5 text-xs rounded-lg transition-all font-medium border ${filter === f ? (modern ? 'border-blue-100/80 bg-blue-50/80 dark:bg-blue-900/20 dark:border-blue-800/40 text-blue-700 dark:text-blue-300 shadow-sm' : 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm') : (modern ? 'border-transparent text-gray-500 hover:bg-white/70 dark:hover:bg-slate-800/70 hover:text-gray-700 dark:hover:text-gray-300' : 'border-transparent text-gray-500 hover:text-gray-700 dark:hover:text-gray-300')}`}>
                  {f === 'all' ? t.skills.allFilter : f === 'enabled' ? t.skills.enabledFilter : t.skills.disabledFilter}
                </button>
              ))}
            </div>
          </div>
          <div className={`${modern ? 'flex flex-wrap gap-1 p-1 rounded-xl border border-blue-100/70 bg-[linear-gradient(145deg,rgba(255,255,255,0.72),rgba(239,246,255,0.56))] dark:bg-[linear-gradient(145deg,rgba(10,20,36,0.82),rgba(30,64,175,0.08))] dark:border-blue-800/20 backdrop-blur-xl' : 'flex flex-wrap gap-1 bg-gray-100 dark:bg-gray-800 p-1 rounded-lg'}`}>
            {scopeFilters.map(item => (
              <button key={item.key} onClick={() => setScopeFilter(item.key)}
                className={`px-3 py-1.5 text-xs rounded-lg transition-all font-medium border ${scopeFilter === item.key ? (modern ? 'border-blue-100/80 bg-blue-50/80 dark:bg-blue-900/20 dark:border-blue-800/40 text-blue-700 dark:text-blue-300 shadow-sm' : 'bg-white dark:bg-gray-700 text-gray-900 dark:text-white shadow-sm') : (modern ? 'border-transparent text-gray-500 hover:bg-white/70 dark:hover:bg-slate-800/70 hover:text-gray-700 dark:hover:text-gray-300' : 'border-transparent text-gray-500 hover:text-gray-700 dark:hover:text-gray-300')}`}>
                {item.label}
              </button>
            ))}
          </div>

          {/* Bulk actions bar */}
          {selectedSkills.size > 0 && (
            <div className="flex items-center gap-2 px-3 py-2 rounded-lg bg-violet-50 dark:bg-violet-900/20 border border-violet-100 dark:border-violet-800/40">
              <span className="text-xs font-medium text-violet-700 dark:text-violet-300">{t.skills.bulkSelected.replace('{n}', String(selectedSkills.size))}</span>
              <div className="flex-1" />
              <button onClick={() => bulkToggleSkills(true)} className="px-2.5 py-1 text-xs rounded-lg bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-300 hover:bg-emerald-200 transition-colors">{t.skills.bulkEnable}</button>
              <button onClick={() => bulkToggleSkills(false)} className="px-2.5 py-1 text-xs rounded-lg bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-200 transition-colors">{t.skills.bulkDisable}</button>
              <button onClick={() => setSelectedSkills(new Set())} className="p-1 rounded-lg text-gray-400 hover:text-gray-600 transition-colors"><X size={14} /></button>
            </div>
          )}
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
            <div className="grid gap-3 overflow-hidden">
              {/* Select all / Deselect all */}
              <div className="flex items-center gap-2 px-1">
                <button onClick={() => {
                  if (selectedSkills.size === filtered.length) setSelectedSkills(new Set());
                  else setSelectedSkills(new Set(filtered.map(s => s.skillKey || s.id)));
                }} className="text-xs text-gray-500 hover:text-violet-600 transition-colors flex items-center gap-1">
                  {selectedSkills.size === filtered.length && filtered.length > 0 ? <CheckSquare size={14} /> : <Square size={14} />}
                  {selectedSkills.size === filtered.length && filtered.length > 0 ? t.skills.deselectAll : t.skills.selectAll}
                </button>
              </div>
              {filtered.map(skill => (
                <div key={skill.id} className={`${modern ? 'relative overflow-hidden rounded-[24px] p-4 border border-white/65 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.84),rgba(239,246,255,0.62))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.88),rgba(30,64,175,0.10))] shadow-[0_18px_40px_rgba(15,23,42,0.06)] backdrop-blur-xl' : 'bg-white dark:bg-gray-800 rounded-xl p-4 shadow-sm border border-gray-100 dark:border-gray-700/50'} hover:shadow-md transition-all group overflow-hidden`}>
                  {modern && <div className="pointer-events-none absolute inset-x-5 top-0 h-px bg-gradient-to-r from-transparent via-white/90 to-transparent dark:via-slate-200/20" />}
                  <div className="flex items-center gap-3 min-w-0">
                    <button onClick={() => {
                      const key = skill.skillKey || skill.id;
                      setSelectedSkills(prev => { const n = new Set(prev); if (n.has(key)) n.delete(key); else n.add(key); return n; });
                    }} className="shrink-0 text-gray-400 hover:text-violet-600 transition-colors">
                      {selectedSkills.has(skill.skillKey || skill.id) ? <CheckSquare size={18} className="text-violet-600" /> : <Square size={18} />}
                    </button>
                    <div className={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 shadow-sm border ${skill.enabled ? 'bg-[linear-gradient(135deg,rgba(37,99,235,0.18),rgba(14,165,233,0.12))] border-blue-100/80 dark:border-blue-800/40 text-blue-600' : 'bg-gray-100 dark:bg-gray-700 border-transparent'}`}>
                      <Sparkles size={18} className={skill.enabled ? 'text-blue-600 dark:text-blue-300' : 'text-gray-400'} />
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-sm font-bold text-gray-900 dark:text-white truncate">{skill.name}</span>
                        {skill.version && <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-gray-500 font-mono shrink-0">v{skill.version}</span>}
                        {getSkillScopeBadge(skill.source)}
                        {getSourceBadge(skill.source)}
                      </div>
                      {skill.description && <p className="text-xs text-gray-500 truncate mt-0.5">{skill.description}</p>}
                      {skill.path && <p className="text-[10px] text-gray-400 truncate mt-0.5 flex items-center gap-1"><FolderOpen size={10} />{skill.path}</p>}
                    </div>
                    <div className="flex items-center gap-2 shrink-0 ml-2">
                      {skill.requires && (skill.requires.env?.length || skill.requires.bins?.length || skill.requires.anyBins?.length || skill.requires.config?.length) && (
                        <button onClick={() => setConfigSkill(skill)} className="text-[10px] px-2 py-1 rounded-lg bg-amber-50 dark:bg-amber-900/30 text-amber-600 border border-amber-100 dark:border-amber-800 hover:bg-amber-100 transition-colors shrink-0">
                          {t.skills.configRequired}
                        </button>
                      )}
                      {(skill.source === 'workspace' || skill.source === 'installed' || skill.source === 'managed') && (
                        <button
                          onClick={() => setConfirmUninstall({ id: skill.skillKey || skill.id, name: skill.name, installTarget: getLocalSkillInstallTarget(skill) })}
                          disabled={uninstalling === (skill.skillKey || skill.id)}
                          className="p-1.5 rounded-lg text-gray-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/30 transition-colors shrink-0 disabled:opacity-50"
                          title={t.skills.uninstall}
                        >
                          {uninstalling === (skill.skillKey || skill.id) ? <Loader2 size={16} className="animate-spin" /> : <Trash2 size={16} />}
                        </button>
                      )}
                      {(skill.source === 'workspace' || skill.source === 'installed' || skill.source === 'managed') && (
                        <button
                          onClick={() => setCopyTarget({ skillId: skill.skillKey || skill.id, skillName: skill.name })}
                          className="p-1.5 rounded-lg text-gray-400 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/30 transition-colors shrink-0"
                          title={locale === 'zh-CN' ? '复制到其他智能体' : 'Copy to other agent'}
                        >
                          <Copy size={16} />
                        </button>
                      )}
                        <button onClick={() => toggleSkill(skill)} className="relative group/toggle focus:outline-none shrink-0" title={skill.enabled ? t.common.running : t.common.stopped}>
                        {skill.enabled 
                          ? <ToggleRight size={36} className="text-emerald-500 transition-transform group-hover/toggle:scale-105" /> 
                          : <ToggleLeft size={36} className="text-gray-300 dark:text-gray-600 transition-transform group-hover/toggle:scale-105" />}
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {tab === 'plugins' && (
        <div className="space-y-4">
          {loading ? (
            <div className="flex flex-col items-center justify-center py-16 text-gray-400 gap-3">
              <Loader2 size={32} className="animate-spin text-violet-500/50" />
              <p className="text-sm">{t.skills.loadingSkills}</p>
            </div>
          ) : plugins.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-gray-400 border-2 border-dashed border-gray-100 dark:border-gray-800 rounded-xl">
              <Plug size={32} className="opacity-20 mb-2" />
              <p className="text-sm">{t.skills.noMatch}</p>
            </div>
          ) : (
            <div className="grid gap-3">
              <div className="flex items-center justify-between px-1">
                <button onClick={() => {
                  if (selectedPlugins.size === plugins.length && plugins.length > 0) setSelectedPlugins(new Set());
                  else setSelectedPlugins(new Set(plugins.map(p => p.id)));
                }} className="text-xs text-gray-500 hover:text-violet-600 transition-colors flex items-center gap-1">
                  {selectedPlugins.size === plugins.length && plugins.length > 0 ? <CheckSquare size={14} /> : <Square size={14} />}
                  {selectedPlugins.size === plugins.length && plugins.length > 0 ? (locale === 'zh-CN' ? '取消全选' : 'Deselect All') : (locale === 'zh-CN' ? '全选' : 'Select All')}
                </button>
                {selectedPlugins.size > 0 && (
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-gray-500">{locale === 'zh-CN' ? `已选 ${selectedPlugins.size}` : `${selectedPlugins.size} selected`}</span>
                    <button onClick={() => bulkTogglePlugins(true)} className="text-xs px-2.5 py-1 rounded-lg bg-emerald-50 dark:bg-emerald-900/30 text-emerald-600 border border-emerald-100 dark:border-emerald-800 hover:bg-emerald-100 transition-colors">
                      {locale === 'zh-CN' ? '批量启用' : 'Enable'}
                    </button>
                    <button onClick={() => bulkTogglePlugins(false)} className="text-xs px-2.5 py-1 rounded-lg bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 border border-gray-200 dark:border-gray-700 hover:bg-gray-200 transition-colors">
                      {locale === 'zh-CN' ? '批量禁用' : 'Disable'}
                    </button>
                    <button onClick={bulkUninstallPlugins} className="text-xs px-2.5 py-1 rounded-lg bg-red-50 dark:bg-red-900/30 text-red-600 border border-red-100 dark:border-red-800 hover:bg-red-100 transition-colors">
                      {locale === 'zh-CN' ? '批量卸载' : 'Uninstall'}
                    </button>
                  </div>
                )}
              </div>
              {plugins.map(plugin => (
                <div key={plugin.id} className={`${modern ? 'relative overflow-hidden rounded-[24px] p-4 border border-white/65 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.84),rgba(239,246,255,0.62))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.88),rgba(30,64,175,0.10))] shadow-[0_18px_40px_rgba(15,23,42,0.06)] backdrop-blur-xl' : 'bg-white dark:bg-gray-800 rounded-xl p-4 shadow-sm border border-gray-100 dark:border-gray-700/50'} hover:shadow-md transition-all group`}>
                  {modern && <div className="pointer-events-none absolute inset-x-5 top-0 h-px bg-gradient-to-r from-transparent via-white/90 to-transparent dark:via-slate-200/20" />}
                  <div className="flex items-center gap-3">
                    <button onClick={() => {
                      setSelectedPlugins(prev => { const n = new Set(prev); if (n.has(plugin.id)) n.delete(plugin.id); else n.add(plugin.id); return n; });
                    }} className="shrink-0 text-gray-400 hover:text-violet-600 transition-colors">
                      {selectedPlugins.has(plugin.id) ? <CheckSquare size={18} className="text-violet-600" /> : <Square size={18} />}
                    </button>
                    <div className={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 shadow-sm border ${plugin.enabled ? 'bg-[linear-gradient(135deg,rgba(37,99,235,0.18),rgba(14,165,233,0.12))] border-blue-100/80 dark:border-blue-800/40' : 'bg-gray-100 dark:bg-gray-700 border-transparent'}`}>
                      <Plug size={18} className={plugin.enabled ? 'text-blue-600 dark:text-blue-300' : 'text-gray-400'} />
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-sm font-bold text-gray-900 dark:text-white truncate">{plugin.name}</span>
                        {plugin.version && <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-gray-500 font-mono shrink-0">v{plugin.version}</span>}
                        <span className={`px-2 py-0.5 rounded-full text-[10px] font-medium shrink-0 ${plugin.source === 'installed' ? 'bg-emerald-100 dark:bg-emerald-950 text-emerald-700 dark:text-emerald-300' : plugin.source === 'config-ext' ? 'bg-blue-100 dark:bg-blue-950 text-blue-700 dark:text-blue-300' : 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400'}`}>
                          {plugin.source === 'installed' ? t.skills.srcInstalled : plugin.source === 'config-ext' ? t.skills.srcDevExt : t.skills.srcConfig}
                        </span>
                      </div>
                      {plugin.description && <p className="text-xs text-gray-500 truncate mt-0.5">{plugin.description}</p>}
                    </div>
                    <button onClick={() => togglePlugin(plugin.id)} className="relative group/toggle focus:outline-none shrink-0 ml-2" title={plugin.enabled ? t.common.enabled : t.common.disabled}>
                      {plugin.enabled
                        ? <ToggleRight size={36} className="text-emerald-500 transition-transform group-hover/toggle:scale-105" />
                        : <ToggleLeft size={36} className="text-gray-300 dark:text-gray-600 transition-transform group-hover/toggle:scale-105" />}
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
          {/* Store Toolbar */}
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-2 p-1 bg-gray-100 dark:bg-gray-800 rounded-xl w-fit">
              <button onClick={() => setHubSource('clawhub')}
                className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${hubSource === 'clawhub' ? 'bg-white dark:bg-gray-700 text-violet-600 dark:text-violet-300 shadow-sm' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'}`}>
                <Globe size={14} /> ClawPanel 自定义仓库
              </button>
              <button onClick={() => setHubSource('skillhub')}
                className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${hubSource === 'skillhub' ? 'bg-white dark:bg-gray-700 text-blue-600 dark:text-blue-300 shadow-sm' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'}`}>
                <Star size={14} /> {t.skills.tencentSkillHub}
              </button>
            </div>

            <div className="flex items-center gap-2 p-1 bg-gray-100 dark:bg-gray-800 rounded-xl w-fit">
              <span className="px-2 text-xs font-medium text-gray-500 dark:text-gray-400">{t.skills.installTargetLabel}</span>
              <button onClick={() => setStoreInstallTarget('agent')}
                className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${storeInstallTarget === 'agent' ? 'bg-white dark:bg-gray-700 text-violet-600 dark:text-violet-300 shadow-sm' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'}`}>
                {t.skills.installTargetAgent}
              </button>
              <button onClick={() => setStoreInstallTarget('global')}
                className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${storeInstallTarget === 'global' ? 'bg-white dark:bg-gray-700 text-emerald-600 dark:text-emerald-300 shadow-sm' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'}`}>
                {t.skills.installTargetGlobal}
              </button>
            </div>

            <div className="flex items-center gap-2 p-1 bg-gray-100 dark:bg-gray-800 rounded-xl w-fit">
              <button onClick={() => setStoreView('grid5')}
                className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${storeView === 'grid5' ? 'bg-white dark:bg-gray-700 text-violet-600 dark:text-violet-300 shadow-sm' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'}`}>
                {t.skills.storeGridView}
              </button>
              <button onClick={() => setStoreView('list')}
                className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${storeView === 'list' ? 'bg-white dark:bg-gray-700 text-violet-600 dark:text-violet-300 shadow-sm' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'}`}>
                {t.skills.storeListView}
              </button>
            </div>
          </div>

          {/* Custom Skill Store */}
          {hubSource === 'clawhub' && (<>
          <div className={`${modern ? 'relative overflow-hidden flex items-center justify-between gap-4 p-4 rounded-[24px] border border-white/65 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.84),rgba(239,246,255,0.62))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.88),rgba(30,64,175,0.10))] shadow-[0_18px_40px_rgba(15,23,42,0.06)] backdrop-blur-xl' : 'flex items-center justify-between gap-4 bg-gradient-to-r from-violet-50 to-indigo-50 dark:from-violet-900/20 dark:to-indigo-900/20 p-4 rounded-xl border border-violet-100 dark:border-violet-800/30'}`}>
            {modern && <div className="pointer-events-none absolute inset-x-5 top-0 h-px bg-gradient-to-r from-transparent via-white/90 to-transparent dark:via-slate-200/20" />}
            <div className="flex items-center gap-3">
              <div className={`${modern ? 'p-2 rounded-xl border border-blue-100/80 dark:border-blue-800/40 bg-[linear-gradient(135deg,rgba(37,99,235,0.12),rgba(14,165,233,0.08))] dark:bg-[linear-gradient(135deg,rgba(37,99,235,0.2),rgba(14,165,233,0.12))] shadow-sm' : 'p-2 bg-white dark:bg-gray-800 rounded-lg shadow-sm'}`}>
                <Globe size={20} className="text-blue-600 dark:text-blue-300" />
              </div>
              <div>
                <h3 className="text-sm font-bold text-gray-900 dark:text-white">ClawPanel 自定义技能仓库</h3>
                <p className="text-xs text-gray-500">从你的 ClawPanel-Plugins 仓库自动刷新技能列表，并安装到当前工作区</p>
                <p className="text-[11px] text-gray-400 mt-1">{storeInstallTarget === 'agent' ? t.skills.installTargetAgentHint : t.skills.installTargetGlobalHint}</p>
              </div>
            </div>
            <div className="flex gap-2">
              <button onClick={handleBulkInstallStore} disabled={bulkInstallingStore || hubLoading || installableStoreCount === 0}
                className={`${modern ? 'page-modern-action' : 'flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 shadow-sm transition-colors'} disabled:opacity-50`}>
                {bulkInstallingStore ? <Loader2 size={14} className="animate-spin" /> : <Download size={14} />}
                {bulkInstallingStore ? t.skills.bulkInstallingStore : t.skills.bulkInstallStore}
              </button>
              <button onClick={handleSearchClawHub} disabled={hubLoading}
                className={`${modern ? 'page-modern-action' : 'flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 shadow-sm transition-colors'}`}>
                {hubLoading ? <Loader2 size={14} className="animate-spin" /> : <RefreshCw size={14} />}
                {hubLoading ? t.skills.syncing : t.skills.syncStore}
              </button>
              {clawHubSiteUrl ? (
                <a href={clawHubSiteUrl} target="_blank" rel="noopener noreferrer"
                  className={`${modern ? 'page-modern-accent' : 'flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 shadow-sm shadow-violet-200 dark:shadow-none transition-colors'}`}>
                  <ExternalLink size={14} />查看仓库
                </a>
              ) : (
                <button type="button" disabled
                  className={`${modern ? 'page-modern-accent opacity-60 cursor-not-allowed' : 'flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg bg-violet-600/70 text-white cursor-not-allowed shadow-sm shadow-violet-200 dark:shadow-none'}`}>
                  <ExternalLink size={14} />查看仓库
                </button>
              )}
            </div>
          </div>

          <div className="relative">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input value={search} onChange={e => { setSearch(e.target.value); debouncedHubSearch(); }} onKeyDown={e => e.key === 'Enter' && handleSearchClawHub()} placeholder="搜索 ClawPanel 自定义技能"
              className="w-full pl-9 pr-10 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500 transition-all" />
            <button onClick={handleSearchClawHub} className="absolute right-3 top-1/2 -translate-y-1/2 text-violet-600 hover:text-violet-700">
              <Search size={14} />
            </button>
          </div>

          {hubCategories.length > 0 && (
            <div className="flex flex-wrap gap-2">
              <button onClick={() => setHubCategory('all')}
                className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${hubCategory === 'all' ? 'bg-violet-600 text-white' : 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700'}`}>
                {t.skills.allFilter}
              </button>
              {hubCategories.map(cat => (
                <button key={cat} onClick={() => setHubCategory(cat)}
                  className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${hubCategory === cat ? 'bg-violet-600 text-white' : 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700'}`}>
                  {cat}
                </button>
              ))}
            </div>
          )}

          {hubLoading ? (
            <div className="space-y-4">
              <div className="flex items-center gap-2 text-gray-400 text-sm"><Loader2 size={16} className="animate-spin text-violet-500/50" />{t.skills.loadingClawHub}</div>
              <div className={storeGridClasses}>
                {Array.from({ length: 10 }).map((_, i) => (
                  <div key={i} className={`${modern ? 'rounded-[24px] p-4 border border-white/65 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.84),rgba(239,246,255,0.62))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.88),rgba(30,64,175,0.10))] backdrop-blur-xl' : 'bg-white dark:bg-gray-800 rounded-xl p-4 border border-gray-100 dark:border-gray-700/50'} animate-pulse`}>
                    <div className="flex items-center gap-3 mb-3">
                      <div className="w-10 h-10 rounded-xl bg-gray-200 dark:bg-gray-700" />
                      <div className="flex-1 space-y-2">
                        <div className="h-4 w-2/3 rounded bg-gray-200 dark:bg-gray-700" />
                        <div className="h-3 w-1/3 rounded bg-gray-100 dark:bg-gray-800" />
                      </div>
                    </div>
                    <div className="space-y-2 mb-4">
                      <div className="h-3 w-full rounded bg-gray-100 dark:bg-gray-800" />
                      <div className="h-3 w-4/5 rounded bg-gray-100 dark:bg-gray-800" />
                    </div>
                    <div className="flex gap-2 mt-auto">
                      <div className="h-8 w-10 rounded-lg bg-gray-100 dark:bg-gray-800" />
                      <div className="h-8 flex-1 rounded-lg bg-gray-100 dark:bg-gray-800" />
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ) : clawHubSkills.length === 0 && !storeEverLoaded ? (
            <div className="flex flex-col items-center justify-center py-20 text-gray-400 border-2 border-dashed border-gray-200 dark:border-gray-700 rounded-xl">
              <Globe size={36} className="opacity-20 mb-3" />
                  <p className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">自定义技能仓库尚未加载</p>
                  <p className="text-xs text-gray-400 dark:text-gray-500 mb-4">进入页面会自动刷新，也可以手动点击同步</p>
              <button onClick={handleSearchClawHub}
                className="flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 shadow-sm transition-colors">
                <RefreshCw size={14} /> {t.skills.syncStore}
              </button>
            </div>
          ) : (
            <div className={storeGridClasses}>
              {hubFiltered.length === 0 ? (
                <div className="col-span-full flex flex-col items-center justify-center py-16 text-gray-400 border-2 border-dashed border-gray-100 dark:border-gray-800 rounded-xl">
                  <Package size={32} className="opacity-20 mb-2" />
                  <p className="text-sm">{t.skills.noSkillsFound}</p>
                </div>
              ) : hubFiltered.map(skill => {
                const installedVersion = skill.installedVersion || '';
                const isInstalled = !!skill.installed;
                const isInstalling = installing === skill.id;
                const detailUrl = buildClawHubLink(clawHubRegistryBase, `/skills/${encodeURIComponent(skill.id)}`);
                return (
                  <div key={skill.id} className={`${modern ? 'relative overflow-hidden rounded-[24px] p-4 border border-white/65 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.84),rgba(239,246,255,0.62))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.88),rgba(30,64,175,0.10))] shadow-[0_18px_40px_rgba(15,23,42,0.06)] backdrop-blur-xl flex flex-col h-full' : 'bg-white dark:bg-gray-800 rounded-xl p-4 shadow-sm border border-gray-100 dark:border-gray-700/50 flex flex-col h-full'} hover:shadow-md transition-all group`}>
                    {modern && <div className="pointer-events-none absolute inset-x-5 top-0 h-px bg-gradient-to-r from-transparent via-white/90 to-transparent dark:via-slate-200/20" />}
                    <div className="flex items-start justify-between mb-3">
                      <div className="flex items-center gap-3 flex-1 min-w-0">
                        <div className={`${modern ? 'w-10 h-10 rounded-xl bg-[linear-gradient(135deg,rgba(37,99,235,0.12),rgba(14,165,233,0.08))] dark:bg-[linear-gradient(135deg,rgba(37,99,235,0.2),rgba(14,165,233,0.12))] flex items-center justify-center shrink-0 border border-blue-100/80 dark:border-blue-800/30 shadow-sm' : 'w-10 h-10 rounded-lg bg-gradient-to-br from-blue-50 to-cyan-50 dark:from-blue-900/30 dark:to-cyan-900/30 flex items-center justify-center shrink-0 border border-blue-100 dark:border-blue-800/30'}`}>
                          <Globe size={18} className="text-blue-600 dark:text-blue-400" />
                        </div>
                        <div className="flex-1 min-w-0">
                          <h4 className="text-sm font-bold text-gray-900 dark:text-white truncate cursor-pointer hover:text-blue-600 dark:hover:text-blue-400 transition-colors" title={skill.name} onClick={() => setDetailSkill(skill)}>{skill.name}</h4>
                          <div className="flex items-center gap-2 mt-0.5">
                            {skill.version && <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-gray-500 font-mono">v{skill.version}</span>}
                            {isInstalled && installedVersion && (
                              <span className="text-[10px] px-1.5 py-0.5 rounded bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600 dark:text-emerald-400 font-mono">
                                {t.common.installed} v{installedVersion}
                              </span>
                            )}
                            {skill.category && <span className="text-[10px] px-1.5 py-0.5 rounded bg-cyan-50 dark:bg-cyan-900/30 text-cyan-600 dark:text-cyan-400">{skill.category}</span>}
                          </div>
                        </div>
                      </div>
                    </div>

                    <div className="flex-1 mb-4">
                      <p className={`text-xs text-gray-500 mb-1 ${clawHubDescriptionClamp}`} title={skill.description}>{skill.description}</p>
                      {skill.author && <p className="text-xs text-gray-400 dark:text-gray-500 mt-2">by {skill.author}</p>}
                      {depResults[skill.id] && !depResults[skill.id].allMet && (
                        <div className="mt-2 px-2 py-1.5 rounded-lg bg-amber-50 dark:bg-amber-900/20 border border-amber-100 dark:border-amber-800/40">
                          <p className="text-[10px] font-medium text-amber-700 dark:text-amber-400">{t.skills.depsMissing.replace('{n}', String(depResults[skill.id].missing.length))}</p>
                          <p className="text-[10px] text-amber-600/80 dark:text-amber-500/80 mt-0.5 truncate">{depResults[skill.id].missing.join(', ')}</p>
                        </div>
                      )}
                    </div>

                    <div className="flex items-center gap-2 pt-3 border-t border-gray-50 dark:border-gray-800">
                      {detailUrl ? (
                        <a href={detailUrl} target="_blank" rel="noopener noreferrer"
                          className={`${modern ? 'page-modern-action p-2' : 'p-2 rounded-lg text-gray-400 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/30 transition-colors'}`} title="在 ClawHub 查看详情">
                          <ExternalLink size={16} />
                        </a>
                      ) : (
                        <button type="button" disabled
                          className={`${modern ? 'page-modern-action p-2 opacity-60 cursor-not-allowed' : 'p-2 rounded-lg text-gray-300 dark:text-gray-600 cursor-not-allowed'}`} title="ClawHub">
                          <ExternalLink size={16} />
                        </button>
                      )}
                      {isInstalled ? (
                        <>
                          {skill.version && installedVersion && skill.version !== installedVersion ? (
                            <button
                              onClick={() => handleUpdateSkill(skill.id)}
                              disabled={updating === skill.id}
                              className={`${modern ? 'page-modern-accent flex-1 py-2 text-xs' : 'flex-1 flex items-center justify-center gap-1.5 py-2 text-xs font-medium rounded-lg bg-blue-600 dark:bg-blue-500 text-white hover:bg-blue-700 dark:hover:bg-blue-600 disabled:opacity-50 transition-colors'}`}>
                              {updating === skill.id ? <Loader2 size={14} className="animate-spin" /> : <ArrowUpCircle size={14} />}
                              {updating === skill.id ? t.skills.updating : `${t.skills.update} → v${skill.version}`}
                            </button>
                          ) : (
                            <button
                              onClick={() => {
                                const localMatch = skills.find(s => s.id === skill.id || s.skillKey === skill.id);
                                setConfirmUninstall({ id: skill.id, name: skill.name, installTarget: localMatch ? getLocalSkillInstallTarget(localMatch) : storeInstallTarget });
                              }}
                              disabled={uninstalling === skill.id}
                              className={`${modern ? 'page-modern-action flex-1 py-2 text-xs text-red-500 hover:text-red-600' : 'flex-1 flex items-center justify-center gap-1.5 py-2 text-xs font-medium rounded-lg bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400 hover:bg-red-100 dark:hover:bg-red-900/30 transition-colors disabled:opacity-50'}`}>
                              {uninstalling === skill.id ? <Loader2 size={14} className="animate-spin" /> : <Trash2 size={14} />}
                              {uninstalling === skill.id ? t.skills.uninstalling : t.skills.uninstall}
                            </button>
                          )}
                        </>
                      ) : (
                        <button onClick={() => handleInstallSkill(skill.id)} disabled={isInstalling}
                          className={`${modern ? 'page-modern-accent flex-1 py-2 text-xs' : 'flex-1 flex items-center justify-center gap-1.5 py-2 text-xs font-medium rounded-lg bg-violet-600 dark:bg-violet-500 text-white hover:bg-violet-700 dark:hover:bg-violet-600 disabled:opacity-50 transition-colors'}`}>
                          {isInstalling ? <Loader2 size={14} className="animate-spin" /> : <Download size={14} />}
                          {t.common.install}
                        </button>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
          {/* Pagination */}
          {hubTotal > hubLimit && (
            <div className="flex items-center justify-center gap-2 mt-4">
              <button
                onClick={() => { const p = hubPage - 1; setHubPage(p); loadClawHub(p); }}
                disabled={hubPage <= 1 || hubLoading}
                className="px-3 py-1.5 text-xs rounded-lg bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-40 transition-colors"
              >←</button>
              <span className="text-xs text-gray-500">{hubPage} / {Math.ceil(hubTotal / hubLimit)}</span>
              <button
                onClick={() => { const p = hubPage + 1; setHubPage(p); loadClawHub(p); }}
                disabled={hubPage >= Math.ceil(hubTotal / hubLimit) || hubLoading}
                className="px-3 py-1.5 text-xs rounded-lg bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-40 transition-colors"
              >→</button>
            </div>
          )}
          </>)}

          {/* SkillHub Source */}
          {hubSource === 'skillhub' && (<>
          <div className={`${modern ? 'relative overflow-hidden flex items-center justify-between gap-4 p-4 rounded-[24px] border border-white/65 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.84),rgba(237,242,255,0.62))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.88),rgba(30,64,175,0.10))] shadow-[0_18px_40px_rgba(15,23,42,0.06)] backdrop-blur-xl' : 'flex items-center justify-between gap-4 bg-gradient-to-r from-blue-50 to-cyan-50 dark:from-blue-900/20 dark:to-cyan-900/20 p-4 rounded-xl border border-blue-100 dark:border-blue-800/30'}`}>
            {modern && <div className="pointer-events-none absolute inset-x-5 top-0 h-px bg-gradient-to-r from-transparent via-white/90 to-transparent dark:via-slate-200/20" />}
            <div className="flex items-center gap-3">
              <div className={`${modern ? 'p-2 rounded-xl border border-blue-100/80 dark:border-blue-800/40 bg-[linear-gradient(135deg,rgba(37,99,235,0.12),rgba(14,165,233,0.08))] dark:bg-[linear-gradient(135deg,rgba(37,99,235,0.2),rgba(14,165,233,0.12))] shadow-sm' : 'p-2 bg-white dark:bg-gray-800 rounded-lg shadow-sm'}`}>
                <Star size={20} className="text-blue-600 dark:text-blue-300" />
              </div>
              <div>
                <h3 className="text-sm font-bold text-gray-900 dark:text-white">{t.skills.skillHubTitle || 'SkillHub \u2014 Tencent Cloud'}</h3>
                <p className="text-xs text-gray-500">{t.skills.skillHubSubtitle || '\u817e\u8baf\u76ee\u5f55\uff0c\u5b89\u88c5\u8d70\u5b98\u65b9 SkillHub CLI'}</p>
                <p className="text-[11px] text-gray-400 mt-1">{storeInstallTarget === 'agent' ? t.skills.installTargetAgentHint : t.skills.installTargetGlobalHint}</p>
                <div className="mt-1 flex flex-wrap items-center gap-2 text-[11px]">
                  <span className={`px-2 py-0.5 rounded-full ${skillHubCliStatus?.installed ? 'bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600 dark:text-emerald-400' : 'bg-amber-50 dark:bg-amber-900/20 text-amber-600 dark:text-amber-400'}`}>
                    {skillHubCliStatus?.installed ? t.skills.skillHubCliInstalled : t.skills.skillHubCliMissing}
                  </span>
                  <span className="text-gray-400 dark:text-gray-500 font-mono">
                    {skillHubCliStatus?.skillInstallCommand || t.skills.skillHubCliHint}
                  </span>
                </div>
              </div>
            </div>
            <div className="flex gap-2">
              <button onClick={handleBulkInstallStore} disabled={bulkInstallingStore || skillHubLoading || installableStoreCount === 0}
                className={`${modern ? 'page-modern-action' : 'flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 shadow-sm transition-colors'} disabled:opacity-50`}>
                {bulkInstallingStore ? <Loader2 size={14} className="animate-spin" /> : <Download size={14} />}
                {bulkInstallingStore ? t.skills.bulkInstallingStore : t.skills.bulkInstallStore}
              </button>
              <button onClick={loadSkillHub} disabled={skillHubLoading}
                className={`${modern ? 'page-modern-action' : 'flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 shadow-sm transition-colors'}`}>
                {skillHubLoading ? <Loader2 size={14} className="animate-spin" /> : <RefreshCw size={14} />}
                {skillHubLoading ? t.skills.syncing : t.skills.syncStore}
              </button>
              {!skillHubCliStatus?.installed && (
                <button onClick={handleInstallSkillHubCLI} disabled={skillHubCliInstalling || skillHubCliLoading}
                  className={`${modern ? 'page-modern-action' : 'flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 shadow-sm transition-colors'} disabled:opacity-50`}>
                  {skillHubCliInstalling ? <Loader2 size={14} className="animate-spin" /> : <Download size={14} />}
                  {skillHubCliInstalling ? t.skills.skillHubCliInstalling : t.skills.skillHubCliInstall}
                </button>
              )}
              <a href="https://skillhub.tencent.com/" target="_blank" rel="noopener noreferrer"
                className={`${modern ? 'page-modern-accent' : 'flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg bg-blue-600 text-white hover:bg-blue-700 shadow-sm shadow-blue-200 dark:shadow-none transition-colors'}`}>
                <ExternalLink size={14} />{t.skills.visitSite}
              </a>
            </div>
          </div>

          {skillHubError && (
            <div className="px-4 py-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800/40 rounded-lg text-sm text-red-700 dark:text-red-400">{skillHubError}</div>
          )}

          {!skillHubCliStatus?.installed && (skillHubCliStatus?.installHint || skillHubCliStatus?.error) && (
            <div className="px-4 py-3 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800/40 rounded-lg text-sm text-amber-700 dark:text-amber-300">
              {skillHubCliStatus.installHint || skillHubCliStatus.error}
            </div>
          )}

          {/* Search */}
          <div className="relative">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input value={skillHubSearch} onChange={e => setSkillHubSearch(e.target.value)} placeholder={t.skills.searchSkillHub || '\u641c\u7d22 SkillHub \u63d2\u4ef6...'}
              className="w-full pl-9 pr-4 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all" />
          </div>

          {/* View + Category filters */}
          <div className="flex flex-wrap items-center gap-2">
            {(['featured', 'category', 'all'] as const).map(v => (
              <button key={v} onClick={() => setSkillHubView(v)}
                className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${skillHubView === v ? 'bg-blue-600 text-white' : 'bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700'}`}>
                {v === 'featured' ? (t.skills.skillHubFeatured || '\u2b50 \u7cbe\u9009') : v === 'category' ? (t.skills.skillHubByCategory || '\u5206\u7c7b\u6d4f\u89c8') : (t.skills.allFilter)}
              </button>
            ))}
            {skillHubView === 'category' && skillHubCatalog && (
              <>
                <span className="text-gray-300 dark:text-gray-600">|</span>
                <button onClick={() => setSkillHubCategory('all')}
                  className={`px-2.5 py-1 text-xs rounded-lg transition-colors ${skillHubCategory === 'all' ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300' : 'text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800'}`}>
                  {t.skills.allFilter}
                </button>
                {Object.keys(skillHubCatalog.categories).map(cat => (
                  <button key={cat} onClick={() => setSkillHubCategory(cat)}
                    className={`px-2.5 py-1 text-xs rounded-lg transition-colors ${skillHubCategory === cat ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300' : 'text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800'}`}>
                    {cat}
                  </button>
                ))}
              </>
            )}
            {skillHubCatalog && (
              <span className="text-xs text-gray-400 ml-auto">{filteredSkillHubSkills.length} / {skillHubCatalog.total} {t.skills.skillHubSkills || 'skills'}</span>
            )}
          </div>

          {/* Skill cards */}
          {skillHubLoading ? (
            <div className="space-y-4">
              <div className="flex items-center gap-2 text-gray-400 text-sm"><Loader2 size={16} className="animate-spin text-blue-500/50" />{t.skills.skillHubLoading || '\u52a0\u8f7d SkillHub \u76ee\u5f55...'}</div>
              <div className={storeGridClasses}>
                {Array.from({ length: 10 }).map((_, i) => (
                  <div key={i} className={`${modern ? 'rounded-[24px] p-4 border border-white/65 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.84),rgba(239,246,255,0.62))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.88),rgba(30,64,175,0.10))] backdrop-blur-xl' : 'bg-white dark:bg-gray-800 rounded-xl p-4 border border-gray-100 dark:border-gray-700/50'} animate-pulse`}>
                    <div className="flex items-center gap-3 mb-3">
                      <div className="w-10 h-10 rounded-xl bg-gray-200 dark:bg-gray-700" />
                      <div className="flex-1 space-y-2">
                        <div className="h-4 w-2/3 rounded bg-gray-200 dark:bg-gray-700" />
                        <div className="h-3 w-1/3 rounded bg-gray-100 dark:bg-gray-800" />
                      </div>
                    </div>
                    <div className="space-y-2 mb-4">
                      <div className="h-3 w-full rounded bg-gray-100 dark:bg-gray-800" />
                      <div className="h-3 w-4/5 rounded bg-gray-100 dark:bg-gray-800" />
                    </div>
                    <div className="flex gap-2 mt-auto">
                      <div className="h-8 w-10 rounded-lg bg-gray-100 dark:bg-gray-800" />
                      <div className="h-8 flex-1 rounded-lg bg-gray-100 dark:bg-gray-800" />
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ) : !skillHubCatalog ? (
            <div className="flex flex-col items-center justify-center py-20 text-gray-400 border-2 border-dashed border-gray-200 dark:border-gray-700 rounded-xl">
              <Star size={36} className="opacity-20 mb-3" />
              <p className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-1">{t.skills.storeNotLoaded}</p>
              <p className="text-xs text-gray-400 dark:text-gray-500 mb-4">{t.skills.storeNotLoadedHint}</p>
              <button onClick={loadSkillHub}
                className="flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-lg bg-blue-600 text-white hover:bg-blue-700 shadow-sm transition-colors">
                <RefreshCw size={14} /> {t.skills.syncStore}
              </button>
            </div>
          ) : (
            <div className={storeGridClasses}>
              {skillHubPagedSkills.length === 0 ? (
                <div className="col-span-full flex flex-col items-center justify-center py-16 text-gray-400 border-2 border-dashed border-gray-100 dark:border-gray-800 rounded-xl">
                  <Package size={32} className="opacity-20 mb-2" />
                  <p className="text-sm">{t.skills.noSkillsFound}</p>
                </div>
              ) : skillHubPagedSkills.map(skill => {
                const isInstalled = isSkillHubInstalled(skill);
                const isInstalling = installing === skill.slug;
                const installFailed = !isInstalled && skill.installState === 'failed';
                const skillHubCliReady = skillHubCliStatus?.installed === true;
                const skillHubLink = skill.homepage || 'https://skillhub.tencent.com/';
                return (
                  <div key={skill.slug} className={`${modern ? 'relative overflow-hidden rounded-[24px] p-4 border border-white/65 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.84),rgba(237,242,255,0.62))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.88),rgba(30,64,175,0.10))] shadow-[0_18px_40px_rgba(15,23,42,0.06)] backdrop-blur-xl flex flex-col h-full' : 'bg-white dark:bg-gray-800 rounded-xl p-4 shadow-sm border border-gray-100 dark:border-gray-700/50 flex flex-col h-full'} hover:shadow-md transition-all group`}>
                    {modern && <div className="pointer-events-none absolute inset-x-5 top-0 h-px bg-gradient-to-r from-transparent via-white/90 to-transparent dark:via-slate-200/20" />}
                    <div className="flex items-start justify-between mb-3">
                      <div className="flex items-center gap-3 flex-1 min-w-0">
                        <div className={`${modern ? 'w-10 h-10 rounded-xl bg-[linear-gradient(135deg,rgba(37,99,235,0.12),rgba(14,165,233,0.08))] dark:bg-[linear-gradient(135deg,rgba(37,99,235,0.2),rgba(14,165,233,0.12))] flex items-center justify-center shrink-0 border border-blue-100/80 dark:border-blue-800/30 shadow-sm' : 'w-10 h-10 rounded-lg bg-gradient-to-br from-blue-50 to-cyan-50 dark:from-blue-900/30 dark:to-cyan-900/30 flex items-center justify-center shrink-0 border border-blue-100 dark:border-blue-800/30'}`}>
                          <Package size={18} className="text-blue-600 dark:text-blue-400" />
                        </div>
                        <div className="flex-1 min-w-0">
                          <h4 className="text-sm font-bold text-gray-900 dark:text-white truncate" title={skill.name}>{skill.name}</h4>
                          <div className="flex items-center gap-2 mt-0.5">
                            {skill.version && <span className="text-[10px] px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-gray-500 font-mono">v{skill.version}</span>}
                            {skill.score > 0 && <span className="text-[10px] px-1.5 py-0.5 rounded bg-amber-50 dark:bg-amber-900/20 text-amber-600 dark:text-amber-400 flex items-center gap-0.5"><TrendingUp size={9} />{skill.score}</span>}
                            {isInstalled && <span className="text-[10px] px-1.5 py-0.5 rounded bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600 dark:text-emerald-400">{t.common.installed}</span>}
                            {installFailed && <span className="text-[10px] px-1.5 py-0.5 rounded bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400" title={skill.installMessage || t.skills.skillHubInstallFailedTag}>{t.skills.skillHubInstallFailedTag}</span>}
                          </div>
                        </div>
                      </div>
                    </div>
                    <div className="flex-1 mb-4">
                      <p className={`text-xs text-gray-500 mb-1 ${skillHubDescriptionClamp}`} title={skill.description_zh || skill.description}>{skill.description_zh || skill.description}</p>
                      {skill.tags?.length > 0 && (
                        <div className="flex flex-wrap gap-1 mt-2">
                          {skill.tags.slice(0, skillHubTagLimit).map(tag => (
                            <span key={tag} className="text-[10px] px-1.5 py-0.5 rounded bg-blue-50 dark:bg-blue-900/20 text-blue-500 dark:text-blue-400">{tag}</span>
                          ))}
                          {skill.tags.length > skillHubTagLimit && <span className="text-[10px] text-gray-400">+{skill.tags.length - skillHubTagLimit}</span>}
                        </div>
                      )}
                      {installFailed && skill.installMessage && (
                        <p className="text-[11px] text-red-500 dark:text-red-400 mt-2 line-clamp-2" title={skill.installMessage}>{skill.installMessage}</p>
                      )}
                      {skill.owner && <p className="text-xs text-gray-400 dark:text-gray-500 mt-2">by {skill.owner}</p>}
                    </div>
                    <div className="flex items-center gap-2 pt-3 border-t border-gray-50 dark:border-gray-800">
                      <a href={skillHubLink} target="_blank" rel="noopener noreferrer"
                        className={`${modern ? 'page-modern-action p-2' : 'p-2 rounded-lg text-gray-400 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/30 transition-colors'}`} title="SkillHub">
                        <ExternalLink size={16} />
                      </a>
                      {isInstalled ? (
                        <span className="flex-1 flex items-center justify-center gap-1.5 py-2 text-xs font-medium text-emerald-600 dark:text-emerald-400">
                          <Check size={14} />{t.common.installed}
                        </span>
                      ) : (
                        <button onClick={() => { if (skillHubCliReady) handleInstallSkillHubSkill(skill.slug); else handleInstallSkillHubCLI(); }} disabled={isInstalling || skillHubCliInstalling || skillHubCliLoading}
                          className={`${modern ? 'page-modern-accent flex-1 py-2 text-xs' : 'flex-1 flex items-center justify-center gap-1.5 py-2 text-xs font-medium rounded-lg bg-blue-600 dark:bg-blue-500 text-white hover:bg-blue-700 dark:hover:bg-blue-600 disabled:opacity-50 transition-colors'}`}>
                          {isInstalling || skillHubCliInstalling ? <Loader2 size={14} className="animate-spin" /> : <Download size={14} />}
                          {skillHubCliReady
                            ? t.skills.skillHubNativeInstall
                            : (skillHubCliInstalling ? t.skills.skillHubCliInstalling : t.skills.skillHubCliInstall)}
                        </button>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}

          {/* SkillHub Pagination */}
          {skillHubTotalPages > 1 && (
            <div className="flex items-center justify-center gap-2 mt-4">
              <button onClick={() => setSkillHubPage(p => Math.max(1, p - 1))} disabled={skillHubPage <= 1}
                className="px-3 py-1.5 text-xs rounded-lg bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-40 transition-colors">{'\u2190'}</button>
              <span className="text-xs text-gray-500">{skillHubPage} / {skillHubTotalPages}</span>
              <button onClick={() => setSkillHubPage(p => Math.min(skillHubTotalPages, p + 1))} disabled={skillHubPage >= skillHubTotalPages}
                className="px-3 py-1.5 text-xs rounded-lg bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-40 transition-colors">{'\u2192'}</button>
            </div>
          )}
          </>)}
        </div>
      )}

      {/* Config Modal */}
      {configSkill && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4" onClick={() => setConfigSkill(null)}>
            <div className="bg-white dark:bg-gray-900 rounded-2xl shadow-2xl max-w-2xl w-full max-h-[85vh] overflow-hidden flex flex-col border border-gray-100 dark:border-gray-800" onClick={e => e.stopPropagation()}>
            <div className={`${modern ? 'px-5 py-4 border-b border-blue-100/70 dark:border-blue-800/20 flex items-center justify-between gap-3 bg-[linear-gradient(145deg,rgba(255,255,255,0.82),rgba(239,246,255,0.6))] dark:bg-[linear-gradient(145deg,rgba(10,20,36,0.86),rgba(30,64,175,0.1))]' : 'px-5 py-4 border-b border-gray-100 dark:border-gray-800 flex items-center justify-between gap-3 bg-gray-50/50 dark:bg-gray-900'}`}>
              <h3 className="text-sm font-bold text-gray-900 dark:text-white flex items-center gap-2">
                <Sparkles size={16} className="text-blue-500" />
                {configSkill.name} {t.skills.configRequirements}
              </h3>
              <div className="flex items-center gap-2 shrink-0">
                {configSkill.requires?.config?.length ? (
                  <>
                    <button
                      onClick={() => handleExportSkillConfig(configSkill)}
                      disabled={configLoading || !!configAction}
                      className={`${modern ? 'page-modern-action px-3 py-1.5 text-xs' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors'} disabled:opacity-50`}
                    >
                      {configAction === 'export' ? <Loader2 size={14} className="animate-spin" /> : <Download size={14} />}
                      {t.skills.configExport}
                    </button>
                    <button
                      onClick={() => configImportRef.current?.click()}
                      disabled={!!configAction}
                      className={`${modern ? 'page-modern-action px-3 py-1.5 text-xs' : 'flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors'} disabled:opacity-50`}
                    >
                      {configAction === 'import' ? <Loader2 size={14} className="animate-spin" /> : <Upload size={14} />}
                      {t.skills.configImport}
                    </button>
                    <input ref={configImportRef} type="file" accept="application/json" className="hidden" onChange={handleSkillConfigImportSelected} />
                  </>
                ) : null}
                <button onClick={() => setConfigSkill(null)} className={`${modern ? 'page-modern-action p-1.5' : 'p-1 rounded-md text-gray-400 hover:text-gray-600 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors'}`}>
                  <X size={18} />
                </button>
              </div>
            </div>
              <div className="flex-1 overflow-auto p-6 space-y-6">
              {hasStructuredConfig && (
                <div className="space-y-3">
                  <div className="flex items-center justify-between">
                    <h4 className="text-xs font-bold text-gray-500 uppercase tracking-wider flex items-center gap-2">
                      <FolderOpen size={14} /> {t.skills.configCurrentValues}
                    </h4>
                    <div className="flex items-center gap-2">
                      {configLoading ? <span className="text-[11px] text-gray-400">{t.common.loading}</span> : null}
                      <button
                        onClick={handleSaveSkillConfig}
                        disabled={configLoading || !!configAction}
                        className={`${modern ? 'page-modern-accent px-3 py-1.5 text-xs' : 'px-3 py-1.5 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 transition-colors'} disabled:opacity-50`}
                      >
                        {configAction === 'save' ? <Loader2 size={14} className="animate-spin inline mr-1" /> : null}
                        {t.common.save}
                      </button>
                    </div>
                  </div>
                  <div className="space-y-3">
                    {configFields.map(field => {
                      const hasValue = Object.prototype.hasOwnProperty.call(configSnapshot?.values || {}, field.key);
                      const inputValue = normalizeConfigInputValue(field, configDraft[field.key]);
                      return (
                        <div key={field.key} className="bg-gray-50 dark:bg-gray-800/50 rounded-xl p-4 border border-gray-100 dark:border-gray-800 space-y-3">
                          <div className="flex items-center justify-between mb-2">
                            <div className="space-y-1">
                              <div className="flex items-center gap-2 flex-wrap">
                                <span className="text-sm font-bold text-gray-900 dark:text-white">{field.label || field.key}</span>
                                {field.required ? <span className="text-[10px] px-2 py-0.5 rounded-full bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400 font-medium">{t.common.required}</span> : null}
                              </div>
                              <code className="text-xs font-bold font-mono text-violet-600 dark:text-violet-400 bg-violet-50 dark:bg-violet-900/30 px-2 py-0.5 rounded inline-block">{field.key}</code>
                            </div>
                            <span className={`text-[10px] px-2 py-0.5 rounded-full font-medium ${hasValue ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-400' : 'bg-gray-100 dark:bg-gray-800 text-gray-500 dark:text-gray-400'}`}>
                              {hasValue ? t.skills.configValueSet : t.skills.configValueUnset}
                            </span>
                          </div>
                          {field.help ? <p className="text-xs text-gray-500 dark:text-gray-400">{field.help}</p> : null}
                          {field.type === 'textarea' ? (
                            <textarea
                              value={String(inputValue)}
                              onChange={e => setConfigDraft(prev => ({ ...prev, [field.key]: e.target.value }))}
                              placeholder={field.placeholder || field.key}
                              rows={4}
                              className="w-full px-3 py-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500 transition-all"
                            />
                          ) : field.type === 'select' ? (
                            <select
                              value={String(inputValue)}
                              onChange={e => {
                                const matched = (field.options || []).find(option => String(option.value) === e.target.value);
                                setConfigDraft(prev => ({ ...prev, [field.key]: matched ? matched.value : e.target.value }));
                              }}
                              className="w-full px-3 py-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500 transition-all"
                            >
                              <option value="">{field.placeholder || field.key}</option>
                              {(field.options || []).map((option, idx) => (
                                <option key={`${field.key}-${idx}`} value={String(option.value)}>{option.label || String(option.value)}</option>
                              ))}
                            </select>
                          ) : field.type === 'toggle' ? (
                            <label className="flex items-center justify-between gap-3 rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2 cursor-pointer">
                              <span className="text-sm text-gray-700 dark:text-gray-300">{field.help || field.label || field.key}</span>
                              <input
                                type="checkbox"
                                checked={Boolean(inputValue)}
                                onChange={e => setConfigDraft(prev => ({ ...prev, [field.key]: e.target.checked }))}
                                className="h-4 w-4 rounded border-gray-300 text-violet-600 focus:ring-violet-500"
                              />
                            </label>
                          ) : (
                            <input
                              type={field.type === 'password' ? 'password' : field.type === 'number' ? 'number' : 'text'}
                              value={field.type === 'number' && inputValue === '' ? '' : String(inputValue)}
                              onChange={e => setConfigDraft(prev => ({
                                ...prev,
                                [field.key]: field.type === 'number'
                                  ? (e.target.value === '' ? '' : Number(e.target.value))
                                  : e.target.value,
                              }))}
                              placeholder={field.placeholder || field.key}
                              className="w-full px-3 py-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500 transition-all"
                            />
                          )}
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}
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
              {configSkill.requires?.anyBins && configSkill.requires.anyBins.length > 0 && (
                <div className="space-y-3">
                  <h4 className="text-xs font-bold text-gray-500 uppercase tracking-wider flex items-center gap-2">
                    <Package size={14} /> {t.skills.anyBinTools}
                  </h4>
                  {configSkill.requires.anyBins.map(bin => (
                    <div key={bin} className="bg-gray-50 dark:bg-gray-800/50 rounded-xl p-4 border border-gray-100 dark:border-gray-800">
                      <div className="flex items-center justify-between mb-2">
                        <code className="text-sm font-bold font-mono text-sky-600 dark:text-sky-400 bg-sky-50 dark:bg-sky-900/30 px-2 py-0.5 rounded">{bin}</code>
                        <span className="text-[10px] px-2 py-0.5 rounded-full bg-sky-100 dark:bg-sky-900/30 text-sky-700 dark:text-sky-400 font-medium">{t.skills.anyCliTool}</span>
                      </div>
                      <p className="text-xs text-gray-600 dark:text-gray-400">
                        {t.skills.anyBinHint}
                      </p>
                    </div>
                  ))}
                </div>
              )}
              {configSkill.requires?.config && configSkill.requires.config.length > 0 && (
                <div className="space-y-3">
                  <h4 className="text-xs font-bold text-gray-500 uppercase tracking-wider flex items-center gap-2">
                    <FolderOpen size={14} /> {t.skills.configKeys}
                  </h4>
                  {configSkill.requires.config.map(configKey => (
                    <div key={configKey} className="bg-gray-50 dark:bg-gray-800/50 rounded-xl p-4 border border-gray-100 dark:border-gray-800">
                      <div className="flex items-center justify-between mb-2">
                        <code className="text-sm font-bold font-mono text-violet-600 dark:text-violet-400 bg-violet-50 dark:bg-violet-900/30 px-2 py-0.5 rounded">{configKey}</code>
                        <span className="text-[10px] px-2 py-0.5 rounded-full bg-violet-100 dark:bg-violet-900/30 text-violet-700 dark:text-violet-400 font-medium">{t.skills.configField}</span>
                      </div>
                      <p className="text-xs text-gray-600 dark:text-gray-400">
                        {t.skills.configKeyHint}
                      </p>
                    </div>
                  ))}
                </div>
              )}
              {pendingConfigImport && (
                <div className="space-y-3 rounded-2xl border border-blue-100 dark:border-blue-800/30 bg-blue-50/60 dark:bg-blue-900/10 p-4">
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <h4 className="text-sm font-bold text-blue-700 dark:text-blue-300">{t.skills.configImportPreview}</h4>
                      <p className="text-xs text-blue-600/80 dark:text-blue-300/80 mt-1">{t.skills.configImportSource.replace('{file}', pendingConfigImport.fileName)}</p>
                    </div>
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => setPendingConfigImport(null)}
                        className={`${modern ? 'page-modern-action px-3 py-1.5 text-xs' : 'px-3 py-1.5 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors'}`}
                      >
                        {t.common.cancel}
                      </button>
                      <button
                        onClick={handleApplySkillConfigImport}
                        disabled={configAction === 'import'}
                        className={`${modern ? 'page-modern-accent px-3 py-1.5 text-xs' : 'px-3 py-1.5 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 transition-colors'} disabled:opacity-50`}
                      >
                        {configAction === 'import' ? <Loader2 size={14} className="animate-spin inline mr-1" /> : null}
                        {t.skills.configImportConfirm}
                      </button>
                    </div>
                  </div>
                  <pre className="bg-gray-900 dark:bg-black rounded-lg p-3 text-[11px] font-mono text-gray-300 whitespace-pre-wrap break-all overflow-x-auto">{JSON.stringify(pendingConfigImport.values, null, 2)}</pre>
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

      {/* Uninstall Confirmation Dialog */}
      {confirmUninstall && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4" onClick={() => setConfirmUninstall(null)}>
          <div className="bg-white dark:bg-gray-900 rounded-2xl shadow-2xl max-w-md w-full overflow-hidden border border-gray-100 dark:border-gray-800" onClick={e => e.stopPropagation()}>
            <div className={`${modern ? 'px-5 py-4 border-b border-red-100/70 dark:border-red-800/20 bg-[linear-gradient(145deg,rgba(255,255,255,0.82),rgba(254,226,226,0.6))] dark:bg-[linear-gradient(145deg,rgba(10,20,36,0.86),rgba(153,27,27,0.1))]' : 'px-5 py-4 border-b border-gray-100 dark:border-gray-800 bg-red-50/50 dark:bg-gray-900'}`}>
              <h3 className="text-sm font-bold text-gray-900 dark:text-white flex items-center gap-2">
                <Trash2 size={16} className="text-red-500" />
                {t.skills.uninstallConfirm}
              </h3>
            </div>
            <div className="p-5">
              <p className="text-sm text-gray-600 dark:text-gray-400 mb-5">
                {t.skills.uninstallConfirmMsg.replace('{id}', confirmUninstall.name)}
              </p>
              <div className="flex justify-end gap-3">
                <button onClick={() => setConfirmUninstall(null)}
                  className={`${modern ? 'page-modern-action px-4 py-2 text-xs' : 'px-4 py-2 text-xs font-medium rounded-lg bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors'}`}>
                  {t.common.cancel}
                </button>
                <button onClick={() => handleUninstallSkill(confirmUninstall.id, confirmUninstall.installTarget)}
                  className={`${modern ? 'page-modern-accent px-4 py-2 text-xs bg-red-600 hover:bg-red-700' : 'px-4 py-2 text-xs font-medium rounded-lg bg-red-600 text-white hover:bg-red-700 transition-colors'}`}>
                  {t.skills.uninstall}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Skill Detail Modal */}
      {detailSkill && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 backdrop-blur-sm" onClick={() => setDetailSkill(null)}>
          <div className="bg-white dark:bg-gray-900 rounded-2xl shadow-2xl max-w-lg w-full mx-4 overflow-hidden border border-gray-100 dark:border-gray-800" onClick={e => e.stopPropagation()}>
            <div className={`${modern ? 'px-5 py-4 border-b border-blue-100/70 dark:border-blue-800/20 bg-[linear-gradient(145deg,rgba(255,255,255,0.82),rgba(219,234,254,0.6))] dark:bg-[linear-gradient(145deg,rgba(10,20,36,0.86),rgba(37,99,235,0.1))]' : 'px-5 py-4 border-b border-gray-100 dark:border-gray-800 bg-blue-50/50 dark:bg-gray-900'}`}>
              <h3 className="text-sm font-bold text-gray-900 dark:text-white flex items-center gap-2">
                <Sparkles size={16} className="text-blue-600" />
                {t.skills.detailTitle}
              </h3>
            </div>
            <div className="p-5 space-y-3 max-h-[70vh] overflow-y-auto">
              <div>
                <h4 className="text-base font-bold text-gray-900 dark:text-white">{detailSkill.name}</h4>
                <p className="text-sm text-gray-500 mt-1">{detailSkill.description || t.skills.detailNoDescription}</p>
              </div>
              <div className="grid grid-cols-2 gap-2 text-xs">
                {detailSkill.version && (
                  <div className="px-3 py-2 rounded-lg bg-gray-50 dark:bg-gray-800">
                    <span className="text-gray-400">{t.skills.detailVersion}</span>
                    <span className="ml-2 font-mono text-gray-700 dark:text-gray-300">v{detailSkill.version}</span>
                  </div>
                )}
                {detailSkill.author && (
                  <div className="px-3 py-2 rounded-lg bg-gray-50 dark:bg-gray-800">
                    <span className="text-gray-400">{t.skills.detailAuthor}</span>
                    <span className="ml-2 text-gray-700 dark:text-gray-300">{detailSkill.author}</span>
                  </div>
                )}
                {detailSkill.category && (
                  <div className="px-3 py-2 rounded-lg bg-gray-50 dark:bg-gray-800">
                    <span className="text-gray-400">{t.skills.detailCategory}</span>
                    <span className="ml-2 text-gray-700 dark:text-gray-300">{detailSkill.category}</span>
                  </div>
                )}
              </div>
              {detailSkill.requires && (detailSkill.requires.env?.length || detailSkill.requires.bins?.length || detailSkill.requires.anyBins?.length) && (
                <div className="px-3 py-2 rounded-lg bg-amber-50 dark:bg-amber-900/20 border border-amber-100 dark:border-amber-800/40">
                  <p className="text-xs font-medium text-amber-700 dark:text-amber-400 mb-1">{t.skills.detailRequirements}</p>
                  {detailSkill.requires.env?.map((e: string) => <span key={e} className="inline-block text-[10px] mr-1 mb-1 px-1.5 py-0.5 rounded bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-300 font-mono">{e}</span>)}
                  {detailSkill.requires.bins?.map((b: string) => <span key={b} className="inline-block text-[10px] mr-1 mb-1 px-1.5 py-0.5 rounded bg-orange-100 dark:bg-orange-900/30 text-orange-700 dark:text-orange-300 font-mono">{b}</span>)}
                  {detailSkill.requires.anyBins?.map((b: string) => <span key={b} className="inline-block text-[10px] mr-1 mb-1 px-1.5 py-0.5 rounded bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300 font-mono">{b}</span>)}
                </div>
              )}
              {depResults[detailSkill.id] && (
                <div className={`px-3 py-2 rounded-lg border text-xs ${depResults[detailSkill.id].allMet ? 'bg-emerald-50 dark:bg-emerald-900/20 border-emerald-100 dark:border-emerald-800/40 text-emerald-700 dark:text-emerald-400' : 'bg-red-50 dark:bg-red-900/20 border-red-100 dark:border-red-800/40 text-red-700 dark:text-red-400'}`}>
                  {depResults[detailSkill.id].allMet ? t.skills.depsMet : `${t.skills.depsMissing.replace('{n}', String(depResults[detailSkill.id].missing.length))}: ${depResults[detailSkill.id].missing.join(', ')}`}
                </div>
              )}
              {detailSkill.path && (
                <div className="px-3 py-2 rounded-lg bg-gray-50 dark:bg-gray-800 text-xs">
                  <span className="text-gray-400">{t.skills.installPath}</span>
                  <span className="ml-2 font-mono text-gray-600 dark:text-gray-400 break-all">{detailSkill.path}</span>
                </div>
              )}
            </div>
            <div className="px-5 pb-4 flex justify-end">
              <button onClick={() => setDetailSkill(null)}
                className={`${modern ? 'page-modern-action px-4 py-2 text-xs' : 'px-4 py-2 text-xs font-medium rounded-lg bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors'}`}>
                {t.common.close}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Copy Skill to Other Agent Modal */}
      {copyTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm" onClick={() => setCopyTarget(null)}>
          <div className={`${modern ? 'relative overflow-hidden rounded-[24px] p-6 border border-white/65 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.92),rgba(239,246,255,0.72))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.95),rgba(30,64,175,0.12))] shadow-2xl backdrop-blur-xl' : 'bg-white dark:bg-gray-900 rounded-2xl shadow-2xl p-6 border border-gray-200 dark:border-gray-700'} max-w-sm w-full mx-4`} onClick={e => e.stopPropagation()}>
            {modern && <div className="pointer-events-none absolute inset-x-5 top-0 h-px bg-gradient-to-r from-transparent via-white/90 to-transparent dark:via-slate-200/20" />}
            <h3 className="text-base font-bold text-gray-900 dark:text-white mb-1 flex items-center gap-2">
              <Copy size={16} className="text-blue-600" />
              {locale === 'zh-CN' ? '复制技能' : 'Copy Skill'}
            </h3>
            <p className="text-sm text-gray-500 mb-4">{copyTarget.skillName}</p>
            <div className="space-y-3">
              <div>
                <label className="text-xs font-semibold text-gray-600 dark:text-gray-400 block mb-1">{locale === 'zh-CN' ? '目标智能体' : 'Target Agent'}</label>
                <select id="copy-target-select" className="w-full px-3 py-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-800 text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-blue-500">
                  <option value="__global__">{locale === 'zh-CN' ? '全局 (Global)' : 'Global'}</option>
                  {agents.filter(a => a.id !== selectedAgent).map(a => (
                    <option key={a.id} value={a.id}>{a.id}{a.default ? ' ⭐' : ''}</option>
                  ))}
                </select>
              </div>
            </div>
            <div className="flex gap-2 mt-5">
              <button onClick={() => setCopyTarget(null)} className={`flex-1 ${modern ? 'page-modern-action px-4 py-2 text-sm' : 'px-4 py-2 text-sm font-medium rounded-lg bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors'}`}>
                {t.common.cancel}
              </button>
              <button onClick={async () => {
                const sel = (document.getElementById('copy-target-select') as HTMLSelectElement)?.value;
                if (!sel) return;
                const target = sel === '__global__' ? 'global' : 'agent';
                const agentId = sel === '__global__' ? '' : sel;
                try {
                  const r = await api.copySkill(copyTarget.skillId, selectedAgent, agentId, target as any);
                  if (r.ok) {
                    setMsg((locale === 'zh-CN' ? '已复制到 ' : 'Copied to ') + (sel === '__global__' ? 'Global' : sel));
                  } else {
                    setMsg(locale === 'zh-CN' ? '复制失败' : 'Copy failed');
                  }
                } catch {
                  setMsg(locale === 'zh-CN' ? '复制失败' : 'Copy failed');
                }
                setCopyTarget(null);
                await loadSkills();
                setTimeout(() => setMsg(''), 3000);
              }} className={`flex-1 ${modern ? 'page-modern-accent px-4 py-2 text-sm' : 'px-4 py-2 text-sm font-medium rounded-lg bg-blue-600 text-white hover:bg-blue-700 transition-colors'}`}>
                {locale === 'zh-CN' ? '复制' : 'Copy'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

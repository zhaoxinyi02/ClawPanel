import { useState, useEffect, useRef, useCallback } from 'react';
import { api } from '../lib/api';
import {
  FolderOpen, File, Trash2, Upload, FolderPlus, RefreshCw,
  ChevronRight, Home, Settings2, Clock, HardDrive, AlertTriangle, Check, X,
  FileText, FileImage, FileVideo, FileAudio, FileArchive, FileCode, Download,
  Eye, Edit3, ArrowUpDown, ChevronUp, ChevronDown, MessageSquare,
} from 'lucide-react';
import { useI18n } from '../i18n';

interface WsFile {
  name: string; path: string; size: number; sizeHuman: string;
  isDirectory: boolean; modifiedAt: string; extension: string; ageDays: number;
}
interface WsConfig { autoCleanEnabled: boolean; autoCleanDays: number; excludePatterns: string[]; }
interface WsStats { totalFiles: number; totalSize: number; totalSizeHuman: string; oldFiles: number; }

type SortKey = 'name' | 'size' | 'modifiedAt' | 'ageDays';
type SortDir = 'asc' | 'desc';

const IMG = ['.jpg','.jpeg','.png','.gif','.webp','.bmp','.svg','.ico'];
const VID = ['.mp4','.avi','.mov','.mkv','.webm'];
const AUD = ['.mp3','.wav','.ogg','.flac','.amr','.silk'];
const ARC = ['.zip','.tar','.gz','.rar','.7z','.tgz'];
const CODE = ['.ts','.js','.py','.sh','.json','.yaml','.yml','.xml','.html','.css'];
const TXT = ['.md','.txt','.log','.jsonl'];
const PREVIEWABLE_IMG = IMG;
const PREVIEWABLE_TXT = ['.txt','.md','.log','.json','.jsonl','.js','.ts','.py','.sh','.yaml','.yml','.xml','.html','.css','.csv','.ini','.conf','.toml','.env'];

function FIcon({ f }: { f: WsFile }) {
  if (f.isDirectory) return <FolderOpen size={18} className="text-amber-500" />;
  const e = f.extension;
  if (IMG.includes(e)) return <FileImage size={18} className="text-pink-500" />;
  if (VID.includes(e)) return <FileVideo size={18} className="text-purple-500" />;
  if (AUD.includes(e)) return <FileAudio size={18} className="text-green-500" />;
  if (ARC.includes(e)) return <FileArchive size={18} className="text-orange-500" />;
  if (CODE.includes(e)) return <FileCode size={18} className="text-blue-500" />;
  if (TXT.includes(e)) return <FileText size={18} className="text-gray-500" />;
  return <File size={18} className="text-gray-400" />;
}

function relTime(iso: string) {
  const d = new Date(iso);
  const now = Date.now();
  const diff = now - d.getTime();
  if (diff < 60000) return '刚刚';
  if (diff < 3600000) return Math.floor(diff / 60000) + ' 分钟前';
  if (diff < 86400000) return Math.floor(diff / 3600000) + ' 小时前';
  if (diff < 2592000000) return Math.floor(diff / 86400000) + ' 天前';
  return d.toLocaleDateString();
}

function canPreview(ext: string) {
  return PREVIEWABLE_IMG.includes(ext) || PREVIEWABLE_TXT.includes(ext);
}

function simpleMarkdown(md: string): string {
  let html = md
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    // Headers
    .replace(/^######\s+(.+)$/gm, '<h6>$1</h6>')
    .replace(/^#####\s+(.+)$/gm, '<h5>$1</h5>')
    .replace(/^####\s+(.+)$/gm, '<h4>$1</h4>')
    .replace(/^###\s+(.+)$/gm, '<h3>$1</h3>')
    .replace(/^##\s+(.+)$/gm, '<h2>$1</h2>')
    .replace(/^#\s+(.+)$/gm, '<h1>$1</h1>')
    // Bold & italic
    .replace(/\*\*\*(.+?)\*\*\*/g, '<strong><em>$1</em></strong>')
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.+?)\*/g, '<em>$1</em>')
    // Inline code
    .replace(/`([^`]+)`/g, '<code class="bg-gray-100 dark:bg-gray-800 px-1 py-0.5 rounded text-xs">$1</code>')
    // Links
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener" class="text-indigo-600 hover:underline">$1</a>')
    // Horizontal rule
    .replace(/^---+$/gm, '<hr class="my-4 border-gray-200 dark:border-gray-700" />')
    // Unordered list items
    .replace(/^[-*]\s+(.+)$/gm, '<li>$1</li>')
    // Ordered list items
    .replace(/^\d+\.\s+(.+)$/gm, '<li>$1</li>')
    // Blockquote
    .replace(/^&gt;\s+(.+)$/gm, '<blockquote class="border-l-4 border-gray-300 dark:border-gray-600 pl-3 text-gray-600 dark:text-gray-400 italic">$1</blockquote>');
  // Wrap consecutive <li> in <ul>
  html = html.replace(/((?:<li>.*<\/li>\n?)+)/g, '<ul class="list-disc pl-5 space-y-1">$1</ul>');
  // Paragraphs: wrap lines that aren't already HTML tags
  html = html.split('\n').map(line => {
    const trimmed = line.trim();
    if (!trimmed) return '';
    if (/^<(h[1-6]|ul|ol|li|blockquote|hr|pre|code|div|table)/.test(trimmed)) return line;
    return '<p>' + line + '</p>';
  }).join('\n');
  // Code blocks (```...```)
  html = html.replace(/<p>```(\w*)<\/p>\n([\s\S]*?)\n<p>```<\/p>/g,
    '<pre class="bg-gray-100 dark:bg-gray-800 rounded-lg p-3 text-xs font-mono overflow-x-auto my-2"><code>$2</code></pre>');
  return html;
}

export default function Workspace() {
  const { t } = useI18n();
  const [files, setFiles] = useState<WsFile[]>([]);
  const [curPath, setCurPath] = useState('');
  const [parentPath, setParentPath] = useState<string | null>(null);
  const [stats, setStats] = useState<WsStats | null>(null);
  const [config, setConfig] = useState<WsConfig | null>(null);
  const [notes, setNotes] = useState<Record<string, string>>({});
  const [sel, setSel] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);
  const [showCfg, setShowCfg] = useState(false);
  const [showMk, setShowMk] = useState(false);
  const [mkName, setMkName] = useState('');
  const [uploading, setUploading] = useState(false);
  const [toast, setToast] = useState<{ t: string; ok: boolean } | null>(null);
  const [sortKey, setSortKey] = useState<SortKey>('name');
  const [sortDir, setSortDir] = useState<SortDir>('asc');
  const [editingNote, setEditingNote] = useState<string | null>(null);
  const [noteText, setNoteText] = useState('');
  const [preview, setPreview] = useState<{ path: string; type: 'image' | 'text'; content?: string } | null>(null);
  const [mdRender, setMdRender] = useState(true);
  const fRef = useRef<HTMLInputElement>(null);

  const flash = (t: string, ok = true) => { setToast({ t, ok }); setTimeout(() => setToast(null), 3000); };

  const load = useCallback(async (p?: string) => {
    setLoading(true); setSel(new Set());
    try { const r = await api.workspaceFiles(p || ''); if (r.ok) { setFiles(r.files); setCurPath(r.currentPath); setParentPath(r.parentPath); } } catch {}
    setLoading(false);
  }, []);

  const loadStats = useCallback(async () => { try { const r = await api.workspaceStats(); if (r.ok) setStats(r); } catch {} }, []);
  const loadCfg = useCallback(async () => { try { const r = await api.workspaceConfig(); if (r.ok) setConfig(r.config); } catch {} }, []);
  const loadNotes = useCallback(async () => { try { const r = await api.workspaceNotes(); if (r.ok) setNotes(r.notes || {}); } catch {} }, []);

  useEffect(() => { load(); loadStats(); loadCfg(); loadNotes(); }, [load, loadStats, loadCfg, loadNotes]);

  const nav = (p: string) => { load(p); setPreview(null); };

  const toggle = (p: string) => setSel(prev => { const n = new Set(prev); n.has(p) ? n.delete(p) : n.add(p); return n; });
  const selAll = () => setSel(prev => prev.size === sortedFiles.length ? new Set() : new Set(sortedFiles.map(f => f.path)));

  const handleSort = (key: SortKey) => {
    if (sortKey === key) setSortDir(d => d === 'asc' ? 'desc' : 'asc');
    else { setSortKey(key); setSortDir(key === 'name' ? 'asc' : 'desc'); }
  };

  const sortedFiles = [...files].sort((a, b) => {
    if (a.isDirectory !== b.isDirectory) return a.isDirectory ? -1 : 1;
    let cmp = 0;
    switch (sortKey) {
      case 'name': cmp = a.name.localeCompare(b.name); break;
      case 'size': cmp = a.size - b.size; break;
      case 'modifiedAt': cmp = new Date(a.modifiedAt).getTime() - new Date(b.modifiedAt).getTime(); break;
      case 'ageDays': cmp = a.ageDays - b.ageDays; break;
    }
    return sortDir === 'asc' ? cmp : -cmp;
  });

  const SortIcon = ({ k }: { k: SortKey }) => {
    if (sortKey !== k) return <ArrowUpDown size={12} className="text-gray-300" />;
    return sortDir === 'asc' ? <ChevronUp size={12} className="text-indigo-500" /> : <ChevronDown size={12} className="text-indigo-500" />;
  };

  const handleDel = async () => {
    if (sel.size === 0) return;
    if (!confirm(t.workspace.deleteConfirm)) return;
    const r = await api.workspaceDelete(Array.from(sel));
    if (r.ok) { flash(t.workspace.deleteSuccess); load(curPath); loadStats(); }
    else flash(r.error || t.workspace.deleteFailed, false);
  };

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const fl = e.target.files; if (!fl || fl.length === 0) return;
    setUploading(true);
    try {
      const r = await api.workspaceUpload(Array.from(fl), curPath || undefined);
      if (r.ok) { flash(t.workspace.uploadSuccess); load(curPath); loadStats(); }
      else flash(r.error || t.workspace.uploadFailed, false);
    } catch (err) { flash(t.workspace.uploadFailed, false); }
    setUploading(false);
    if (fRef.current) fRef.current.value = '';
  };

  const handleMk = async () => {
    if (!mkName.trim()) return;
    const r = await api.workspaceMkdir(mkName.trim(), curPath || undefined);
    if (r.ok) { flash(t.workspace.createSuccess); setMkName(''); setShowMk(false); load(curPath); }
    else flash(r.error || t.workspace.createFailed, false);
  };

  const handleClean = async () => {
    if (!confirm(t.workspace.deleteConfirm)) return;
    const r = await api.workspaceClean();
    if (r.ok) { flash(t.workspace.deleteSuccess); load(curPath); loadStats(); }
    else flash(r.error || t.workspace.deleteFailed, false);
  };

  const saveCfg = async () => {
    if (!config) return;
    const r = await api.workspaceUpdateConfig(config);
    if (r.ok) { setConfig(r.config); flash(t.common.success); loadStats(); }
    else flash(r.error || t.common.failed, false);
  };

  const saveNote = async (filePath: string) => {
    await api.workspaceSetNote(filePath, noteText);
    setNotes(prev => {
      const n = { ...prev };
      if (noteText) n[filePath] = noteText; else delete n[filePath];
      return n;
    });
    setEditingNote(null);
    flash(t.common.success);
  };

  const openPreview = async (f: WsFile) => {
    if (f.isDirectory) return;
    const ext = f.extension;
    if (PREVIEWABLE_IMG.includes(ext)) {
      setPreview({ path: f.path, type: 'image' });
    } else if (PREVIEWABLE_TXT.includes(ext)) {
      try {
        const r = await api.workspacePreview(f.path);
        if (r.ok && r.type === 'text') {
          setPreview({ path: f.path, type: 'text', content: r.content });
          setMdRender(ext === '.md');
        } else {
          flash(r.error || '无法预览', false);
        }
      } catch { flash('预览失败', false); }
    }
  };

  const crumbs = () => {
    const parts = curPath ? curPath.split('/').filter(Boolean) : [];
    const c: { l: string; p: string }[] = [{ l: t.workspace.title, p: '' }];
    let acc = '';
    for (const x of parts) { acc = acc ? acc + '/' + x : x; c.push({ l: x, p: acc }); }
    return c;
  };

  return (
    <div className="space-y-6 h-full flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between shrink-0">
        <div>
          <h1 className="text-xl font-bold text-gray-900 dark:text-white tracking-tight">{t.workspace.title}</h1>
          <p className="text-sm text-gray-500 mt-1">{t.workspace.subtitle}</p>
        </div>
        <button onClick={() => setShowCfg(!showCfg)} className="flex items-center gap-2 px-3 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300 transition-colors shadow-sm">
          <Settings2 size={14} /> {t.nav.systemConfig}
        </button>
      </div>

      {/* Toast */}
      {toast && (
        <div className={`flex items-center gap-2 px-4 py-3 rounded-xl text-sm font-medium animate-in fade-in slide-in-from-top-2 duration-200 ${toast.ok ? 'bg-emerald-50 dark:bg-emerald-900/30 text-emerald-600 dark:text-emerald-400' : 'bg-red-50 dark:bg-red-900/30 text-red-600 dark:text-red-400'}`}>
          {toast.ok ? <Check size={16} /> : <AlertTriangle size={16} />} {toast.t}
        </div>
      )}

      {/* Stats */}
      {stats && (
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 shrink-0">
          {[
            { icon: <HardDrive size={18} />, label: t.workspace.totalSize, val: stats.totalSizeHuman, color: 'text-blue-500', bg: 'bg-blue-50 dark:bg-blue-900/20' },
            { icon: <File size={18} />, label: t.workspace.totalFiles, val: String(stats.totalFiles), color: 'text-violet-500', bg: 'bg-violet-50 dark:bg-violet-900/20' },
            { icon: <Clock size={18} />, label: t.common.status, val: String(stats.oldFiles), color: 'text-amber-500', bg: 'bg-amber-50 dark:bg-amber-900/20' },
            { icon: <AlertTriangle size={18} />, label: t.workspace.autoClean, val: config?.autoCleanEnabled ? `${config.autoCleanDays}d` : t.common.off, color: config?.autoCleanEnabled ? 'text-emerald-500' : 'text-gray-400', bg: config?.autoCleanEnabled ? 'bg-emerald-50 dark:bg-emerald-900/20' : 'bg-gray-100 dark:bg-gray-800' },
          ].map((s, i) => (
            <div key={i} className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 p-4 flex items-center gap-4 transition-all hover:shadow-md">
              <div className={`p-2.5 rounded-xl ${s.bg} ${s.color}`}>
                {s.icon}
              </div>
              <div>
                <p className="text-[10px] font-semibold text-gray-500 uppercase tracking-wider">{s.label}</p>
                <p className="text-lg font-bold text-gray-900 dark:text-white mt-0.5">{s.val}</p>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Config panel */}
      {showCfg && config && (
        <div className="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-100 dark:border-gray-700/50 p-5 space-y-5 animate-in fade-in slide-in-from-top-4 duration-200">
          <div className="flex items-center gap-2 pb-2 border-b border-gray-100 dark:border-gray-800">
            <Settings2 size={16} className="text-violet-500" />
            <h3 className="font-bold text-gray-900 dark:text-white">自动清理配置</h3>
          </div>
          
          <div className="flex items-center gap-6">
            <label className="flex items-center gap-3 p-3 rounded-lg border border-gray-100 dark:border-gray-800 bg-gray-50/50 dark:bg-gray-900/30 cursor-pointer hover:border-violet-200 dark:hover:border-violet-800 transition-colors">
              <div className="relative flex items-center">
                <input type="checkbox" checked={config.autoCleanEnabled} onChange={e => setConfig({ ...config, autoCleanEnabled: e.target.checked })} 
                  className="peer sr-only" />
                <div className="w-9 h-5 bg-gray-300 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-violet-300 dark:peer-focus:ring-violet-800 rounded-full peer dark:bg-gray-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-violet-600"></div>
              </div>
              <span className="text-sm font-medium text-gray-700 dark:text-gray-300">启用自动清理</span>
            </label>
            
            <div className="flex items-center gap-3">
              <span className="text-sm text-gray-500">过期天数</span>
              <div className="relative">
                <input type="number" min={1} max={365} value={config.autoCleanDays} onChange={e => setConfig({ ...config, autoCleanDays: parseInt(e.target.value) || 30 })}
                  className="w-20 pl-3 pr-8 py-1.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500 text-center" />
                <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-gray-400">天</span>
              </div>
            </div>
          </div>
          
          <div>
            <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider block mb-2">排除文件（每行一个模式）</span>
            <textarea value={config.excludePatterns.join('\n')} onChange={e => setConfig({ ...config, excludePatterns: e.target.value.split('\n').filter(Boolean) })}
              rows={3} className="w-full px-3.5 py-2.5 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500 font-mono resize-none" />
          </div>
          
          <div className="flex gap-3 pt-2">
            <button onClick={saveCfg} className="px-5 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 shadow-sm shadow-violet-200 dark:shadow-none transition-all hover:shadow-md hover:shadow-violet-200 dark:hover:shadow-none">
              保存配置
            </button>
            <button onClick={handleClean} className="px-5 py-2 text-xs font-medium rounded-lg bg-white dark:bg-gray-800 border border-red-200 dark:border-red-900/30 text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors">
              立即清理过期文件
            </button>
          </div>
        </div>
      )}

      {/* Toolbar */}
      <div className="flex flex-wrap items-center justify-between gap-4 bg-white dark:bg-gray-800 p-2 rounded-xl border border-gray-100 dark:border-gray-700/50 shadow-sm shrink-0">
        <div className="flex items-center gap-1.5 text-sm flex-1 min-w-0 overflow-x-auto px-2 scrollbar-hide">
          {crumbs().map((c, i, a) => (
            <span key={c.p + i} className="flex items-center gap-1.5 shrink-0">
              {i > 0 && <ChevronRight size={14} className="text-gray-300 dark:text-gray-600" />}
              <button onClick={() => nav(c.p)} 
                className={`flex items-center gap-1.5 px-2 py-1 rounded-md transition-colors ${i === a.length - 1 ? 'bg-violet-50 dark:bg-violet-900/20 text-violet-700 dark:text-violet-300 font-semibold' : 'text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700 hover:text-gray-900 dark:hover:text-gray-100'}`}>
                {i === 0 ? <Home size={14} /> : c.l}
              </button>
            </span>
          ))}
        </div>
        
        <div className="flex items-center gap-2 pl-2 border-l border-gray-100 dark:border-gray-700/50">
          <button onClick={() => { load(curPath); loadStats(); }} className="p-2 rounded-lg text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700 hover:text-gray-900 dark:hover:text-gray-100 transition-colors" title={t.common.refresh}>
            <RefreshCw size={16} className={loading ? 'animate-spin' : ''} />
          </button>
          
          <button onClick={() => setShowMk(true)} className="flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-200 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors">
            <FolderPlus size={14} /> {t.workspace.newFolder}
          </button>
          
          <label className={`flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg bg-violet-600 text-white hover:bg-violet-700 shadow-sm shadow-violet-200 dark:shadow-none transition-all cursor-pointer ${uploading ? 'opacity-50 pointer-events-none' : ''}`}>
            <Upload size={14} /> {uploading ? t.common.loading : t.workspace.uploadFile}
            <input ref={fRef} type="file" multiple className="hidden" onChange={handleUpload} />
          </label>
          
          {sel.size > 0 && (
            <button onClick={handleDel} className="flex items-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg bg-red-50 dark:bg-red-900/20 text-red-600 hover:bg-red-100 dark:hover:bg-red-900/40 border border-red-100 dark:border-red-900/30 transition-colors animate-in zoom-in-95 duration-200">
              <Trash2 size={14} /> {t.common.delete} ({sel.size})
            </button>
          )}
        </div>
      </div>

      {/* Mkdir */}
      {showMk && (
        <div className="flex items-center gap-3 bg-white dark:bg-gray-800 border border-violet-200 dark:border-violet-800 rounded-xl p-3 shadow-sm animate-in fade-in slide-in-from-top-2 duration-200">
          <div className="p-1.5 rounded-lg bg-violet-100 dark:bg-violet-900/30 text-violet-600">
            <FolderPlus size={16} />
          </div>
          <input autoFocus value={mkName} onChange={e => setMkName(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') handleMk(); if (e.key === 'Escape') { setShowMk(false); setMkName(''); } }}
            placeholder={t.workspace.folderNamePlaceholder} className="flex-1 px-3 py-1.5 text-sm bg-transparent border-b border-gray-200 dark:border-gray-700 focus:border-violet-500 outline-none transition-colors" />
          <div className="flex gap-1">
            <button onClick={handleMk} className="p-1.5 text-emerald-600 hover:bg-emerald-50 dark:hover:bg-emerald-900/30 rounded-lg transition-colors"><Check size={16} /></button>
            <button onClick={() => { setShowMk(false); setMkName(''); }} className="p-1.5 text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"><X size={16} /></button>
          </div>
        </div>
      )}

      {/* Preview modal */}
      {preview && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4" onClick={() => setPreview(null)}>
          <div className="bg-white dark:bg-gray-900 rounded-2xl shadow-2xl max-w-4xl w-full max-h-[85vh] overflow-hidden flex flex-col border border-gray-100 dark:border-gray-800" onClick={e => e.stopPropagation()}>
            <div className="flex items-center justify-between px-5 py-4 border-b border-gray-100 dark:border-gray-800 bg-gray-50/50 dark:bg-gray-900">
              <div className="flex items-center gap-3 min-w-0">
                <div className="p-1.5 rounded-lg bg-gray-100 dark:bg-gray-800 text-gray-500">
                  <Eye size={16} />
                </div>
                <span className="text-sm font-bold text-gray-900 dark:text-white truncate">{preview.path.split('/').pop()}</span>
              </div>
              <div className="flex items-center gap-3 shrink-0">
                {preview.type === 'text' && preview.path.endsWith('.md') && (
                  <div className="flex items-center rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-800">
                    <button onClick={() => setMdRender(true)}
                      className={`px-3 py-1.5 text-xs font-medium transition-colors ${mdRender ? 'bg-violet-100 dark:bg-violet-900/30 text-violet-700 dark:text-violet-300' : 'text-gray-500 hover:bg-gray-50 dark:hover:bg-gray-700'}`}>
                      <Eye size={12} className="inline mr-1.5" />渲染
                    </button>
                    <div className="w-px bg-gray-200 dark:bg-gray-700 self-stretch"></div>
                    <button onClick={() => setMdRender(false)}
                      className={`px-3 py-1.5 text-xs font-medium transition-colors ${!mdRender ? 'bg-violet-100 dark:bg-violet-900/30 text-violet-700 dark:text-violet-300' : 'text-gray-500 hover:bg-gray-50 dark:hover:bg-gray-700'}`}>
                      <FileCode size={12} className="inline mr-1.5" />源码
                    </button>
                  </div>
                )}
                <a href={api.workspaceDownloadUrl(preview.path)} className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/40 transition-colors">
                  <Download size={12} /> {t.workspace.download}
                </a>
                <button onClick={() => setPreview(null)} className="p-1.5 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg text-gray-400 transition-colors">
                  <X size={18} />
                </button>
              </div>
            </div>
            <div className="flex-1 overflow-auto p-6 bg-gray-50/30 dark:bg-black/20">
              {preview.type === 'image' ? (
                <div className="flex items-center justify-center h-full">
                  <img src={api.workspacePreviewUrl(preview.path)} alt={preview.path} className="max-w-full max-h-full rounded-lg shadow-sm" />
                </div>
              ) : preview.path.endsWith('.md') && mdRender ? (
                <div className="prose prose-sm dark:prose-invert max-w-none bg-white dark:bg-gray-900 p-6 rounded-xl shadow-sm border border-gray-100 dark:border-gray-800" dangerouslySetInnerHTML={{ __html: simpleMarkdown(preview.content || '') }} />
              ) : (
                <pre className="text-xs font-mono whitespace-pre-wrap break-all text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-900 p-4 rounded-xl border border-gray-100 dark:border-gray-800 shadow-sm h-full overflow-auto">{preview.content}</pre>
              )}
            </div>
          </div>
        </div>
      )}

      {/* File table */}
      <div className="bg-white dark:bg-gray-800 rounded-xl border border-gray-100 dark:border-gray-700/50 overflow-hidden shadow-sm flex-1 flex flex-col min-h-0">
        <div className="hidden sm:grid grid-cols-[40px_1fr_minmax(120px,1.5fr)_90px_120px_100px] gap-4 px-5 py-3 text-xs font-bold text-gray-500 dark:text-gray-400 border-b border-gray-100 dark:border-gray-800 bg-gray-50/50 dark:bg-gray-900/50 uppercase tracking-wider">
          <div className="flex items-center justify-center"><input type="checkbox" checked={sortedFiles.length > 0 && sel.size === sortedFiles.length} onChange={selAll} className="rounded w-3.5 h-3.5 border-gray-300 text-violet-600 focus:ring-violet-500" /></div>
          <button onClick={() => handleSort('name')} className="flex items-center gap-1.5 hover:text-gray-700 dark:hover:text-gray-200 text-left transition-colors">名称 <SortIcon k="name" /></button>
          <div className="flex items-center gap-1.5"><MessageSquare size={12} /> 备注</div>
          <button onClick={() => handleSort('size')} className="flex items-center gap-1.5 hover:text-gray-700 dark:hover:text-gray-200 transition-colors">大小 <SortIcon k="size" /></button>
          <button onClick={() => handleSort('modifiedAt')} className="flex items-center gap-1.5 hover:text-gray-700 dark:hover:text-gray-200 transition-colors">修改时间 <SortIcon k="modifiedAt" /></button>
          <div className="text-right pr-2">操作</div>
        </div>

        <div className="flex-1 overflow-y-auto min-h-0">
          {loading ? (
            <div className="flex flex-col items-center justify-center h-full text-gray-400 gap-3">
              <RefreshCw size={24} className="animate-spin text-violet-500/50" />
              <p className="text-sm">加载文件列表中...</p>
            </div>
          ) : sortedFiles.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-gray-400 gap-3">
              <FolderOpen size={32} className="opacity-20" />
              <p className="text-sm">此文件夹为空</p>
            </div>
          ) : (
            <div className="divide-y divide-gray-50 dark:divide-gray-800/50">
              {sortedFiles.map(f => (
                <div key={f.path}
                  className={`grid grid-cols-[40px_1fr_minmax(120px,1.5fr)_90px_120px_100px] gap-4 px-5 py-2.5 items-center text-sm hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors group ${sel.has(f.path) ? 'bg-violet-50/60 dark:bg-violet-900/10' : ''}`}>
                  <div className="flex items-center justify-center">
                    <input type="checkbox" checked={sel.has(f.path)} onChange={() => toggle(f.path)} className="rounded w-3.5 h-3.5 border-gray-300 text-violet-600 focus:ring-violet-500" />
                  </div>
                  <div className="flex items-center gap-3 min-w-0">
                    <div className="shrink-0 transition-transform group-hover:scale-110 duration-200">
                      <FIcon f={f} />
                    </div>
                    {f.isDirectory ? (
                      <button onClick={() => nav(f.path)} className="truncate text-left font-semibold text-gray-700 dark:text-gray-200 hover:text-violet-600 dark:hover:text-violet-400 transition-colors">{f.name}</button>
                    ) : (
                      <button onClick={() => canPreview(f.extension) ? openPreview(f) : undefined}
                        className={`truncate text-left ${canPreview(f.extension) ? 'text-gray-700 dark:text-gray-200 hover:text-violet-600 dark:hover:text-violet-400 cursor-pointer font-medium' : 'text-gray-600 dark:text-gray-400'}`}>{f.name}</button>
                    )}
                  </div>
                  <div className="min-w-0">
                    {editingNote === f.path ? (
                      <div className="flex items-center gap-1.5 bg-white dark:bg-gray-900 border border-violet-200 dark:border-violet-800 rounded-lg p-1 shadow-sm">
                        <input autoFocus value={noteText} onChange={e => setNoteText(e.target.value)}
                          onKeyDown={e => { if (e.key === 'Enter') saveNote(f.path); if (e.key === 'Escape') setEditingNote(null); }}
                          className="flex-1 px-2 py-0.5 text-xs bg-transparent min-w-0 outline-none" placeholder="备注..." />
                        <button onClick={() => saveNote(f.path)} className="text-emerald-500 hover:bg-emerald-50 dark:hover:bg-emerald-900/30 p-0.5 rounded"><Check size={12} /></button>
                        <button onClick={() => setEditingNote(null)} className="text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 p-0.5 rounded"><X size={12} /></button>
                      </div>
                    ) : (
                      <button onClick={() => { setEditingNote(f.path); setNoteText(notes[f.path] || ''); }}
                        className="text-xs text-gray-400 hover:text-violet-500 truncate block max-w-full text-left transition-colors py-1 px-1 rounded hover:bg-gray-100 dark:hover:bg-gray-800/50 -ml-1"
                        title={notes[f.path] || '点击添加备注'}>
                        {notes[f.path] || <span className="opacity-0 group-hover:opacity-100 flex items-center gap-1"><Edit3 size={10} /> 添加备注</span>}
                      </button>
                    )}
                  </div>
                  <div className="text-xs text-gray-500 font-mono">{f.isDirectory ? '-' : f.sizeHuman}</div>
                  <div className="text-xs text-gray-500">{relTime(f.modifiedAt)}</div>
                  <div className="flex items-center justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    {!f.isDirectory && canPreview(f.extension) && (
                      <button onClick={() => openPreview(f)} className="p-1.5 rounded-lg text-gray-400 hover:text-violet-600 hover:bg-violet-50 dark:hover:bg-violet-900/20 transition-colors" title="预览">
                        <Eye size={14} />
                      </button>
                    )}
                    {!f.isDirectory && (
                      <a href={api.workspaceDownloadUrl(f.path)} className="p-1.5 rounded-lg text-gray-400 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/20 transition-colors" title="下载">
                        <Download size={14} />
                      </a>
                    )}
                    <button onClick={() => { setSel(new Set([f.path])); setTimeout(handleDel, 0); }} className="p-1.5 rounded-lg text-gray-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors" title="删除">
                      <Trash2 size={14} />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

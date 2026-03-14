import { ReactNode, useMemo, useState, useEffect, useRef } from 'react';
import { Search, ChevronDown, ChevronRight, Copy, Check, X, ExternalLink, PanelLeftClose, PanelLeftOpen } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useI18n } from '../i18n';
import { copyTextWithFallback, fetchHelpMarkdown } from '../constants/help';

// 目录项类型定义
interface TocItem {
  id: string;
  text: string;
  level: number;
  children: TocItem[];
}

interface SearchIndexItem {
  sectionId: string;
  sectionTitle: string;
  sectionPath: string;
  content: string;
}

interface HeadingEntry {
  id: string;
  text: string;
  level: number;
}

const HEADING_KEY_SEPARATOR = '::';

// 生成标题锚点 ID，兼容中文等非 ASCII 文本
const slugifyHeading = (text: string): string => {
  const normalized = text.trim().toLowerCase().replace(/\s+/g, '-');
  // 使用 encodeURIComponent 将中文等字符转为 ASCII，再去掉 %，保证 React id 安全、唯一
  const encoded = encodeURIComponent(normalized).replace(/%/g, '');
  return encoded || `heading-${Math.random().toString(36).slice(2, 8)}`;
};

const normalizeHeadingText = (text: string): string => {
  return text
    .replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/<[^>]+>/g, '')
    .replace(/[>*_~]+/g, '')
    .trim();
};

const extractTextFromNode = (node: ReactNode): string => {
  if (typeof node === 'string' || typeof node === 'number') return String(node);
  if (Array.isArray(node)) return node.map(extractTextFromNode).join('');
  if (node && typeof node === 'object' && 'props' in node) {
    return extractTextFromNode((node as { props?: { children?: ReactNode } }).props?.children);
  }
  return '';
};

const buildHeadingEntries = (md: string): HeadingEntry[] => {
  const lines = md.split('\n');
  const entries: HeadingEntry[] = [];
  const slugCounts = new Map<string, number>();
  let inCodeFence = false;

  lines.forEach((line) => {
    if (/^```/.test(line.trim())) {
      inCodeFence = !inCodeFence;
      return;
    }
    if (inCodeFence) return;

    const match = line.match(/^(#{1,6})\s+(.*)$/);
    if (!match) return;

    const level = match[1].length;
    const text = normalizeHeadingText(match[2]);
    const baseId = slugifyHeading(text);
    const count = slugCounts.get(baseId) || 0;
    slugCounts.set(baseId, count + 1);

    entries.push({
      id: count === 0 ? baseId : `${baseId}-${count + 1}`,
      text,
      level,
    });
  });

  return entries;
};

const getHeadingLookupKey = (level: number, text: string): string => `${level}${HEADING_KEY_SEPARATOR}${text}`;
const REPO_BLOB_BASE = 'https://github.com/zhaoxinyi02/ClawPanel/blob/main/';
const REPO_TREE_BASE = 'https://github.com/zhaoxinyi02/ClawPanel/tree/main/';

// 代码块组件，带复制功能
interface CodeBlockProps {
  code: string;
  language: string;
  locale: string;
}

function CodeBlock({ code, language, locale }: CodeBlockProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      const copiedOk = await copyTextWithFallback(code);
      if (!copiedOk) throw new Error('copy failed');
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error('Failed to copy code:', err);
    }
  };

  return (
    <div className="my-4 overflow-hidden rounded-xl border border-gray-900 bg-black shadow-[0_10px_30px_rgba(0,0,0,0.28)]">
      <div className="flex items-center justify-between border-b border-gray-800 bg-black/95 px-4 py-2.5">
        <span className="text-xs font-medium uppercase tracking-wide text-gray-400">
          {language || 'text'}
        </span>
        <button
          onClick={handleCopy}
          className="flex items-center gap-1 text-xs font-medium text-gray-400 transition-colors hover:text-white"
        >
          {copied ? (
            <>
              <Check size={14} />
              <span>{locale === 'zh-CN' ? '已复制' : 'Copied'}</span>
            </>
          ) : (
            <>
              <Copy size={14} />
              <span>{locale === 'zh-CN' ? '复制' : 'Copy'}</span>
            </>
          )}
        </button>
      </div>
      <pre className="m-0 overflow-x-auto bg-black px-4 py-4 text-gray-100">
        <code className="block whitespace-pre text-sm leading-6 font-mono">{code}</code>
      </pre>
    </div>
  );
}

// 图片预览组件
interface ImageWithPreviewProps {
  src: string;
  alt: string;
  title?: string;
  [key: string]: any;
}

function ImageWithPreview({ src, alt, title, ...props }: ImageWithPreviewProps) {
  const [previewOpen, setPreviewOpen] = useState(false);

  // 监听Esc键关闭预览
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && previewOpen) {
        setPreviewOpen(false);
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [previewOpen]);

  return (
    <>
      <img
        src={src}
        alt={alt}
        title={title}
        {...props}
        className="cursor-pointer hover:opacity-90 transition-opacity"
        onClick={() => setPreviewOpen(true)}
      />
      {previewOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4"
          onClick={() => setPreviewOpen(false)}
        >
          <div className="relative max-w-4xl max-h-[90vh]">
            <img
              src={src}
              alt={alt}
              title={title}
              className="max-w-full max-h-[90vh] object-contain"
            />
            <button
              className="absolute top-4 right-4 bg-white/20 hover:bg-white/30 rounded-full p-2 transition-colors"
              onClick={(e) => {
                e.stopPropagation();
                setPreviewOpen(false);
              }}
            >
              <X size={24} className="text-white" />
            </button>
          </div>
        </div>
      )}
    </>
  );
}

export default function HelpDocs() {
  const { t, locale } = useI18n();
  // 完全不使用context的uiMode，避免样式冲突
  const [query, setQuery] = useState('');
  const [markdown, setMarkdown] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [toc, setToc] = useState<TocItem[]>([]);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [progress, setProgress] = useState(0);
  const [activeSectionId, setActiveSectionId] = useState<string | null>(null);
  const [currentSectionId, setCurrentSectionId] = useState<string>('');
  const [searchIndex, setSearchIndex] = useState<SearchIndexItem[]>([]);
  const [searchResults, setSearchResults] = useState<SearchIndexItem[]>([]);
  const [searchOpen, setSearchOpen] = useState(false);
  const [pendingExternalUrl, setPendingExternalUrl] = useState<string | null>(null);
  const pageRef = useRef<HTMLDivElement>(null);
  const contentRef = useRef<HTMLDivElement>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);

  const getScrollContainer = () => {
    return pageRef.current?.closest('.overflow-y-auto') as HTMLElement | null;
  };

  const isExternalHref = (href: string) => /^https?:\/\//i.test(href);

  const title = locale === 'zh-CN' ? 'ClawPanel 在线帮助系统' : 'ClawPanel Online Help';

  const headingEntries = useMemo(() => buildHeadingEntries(markdown), [markdown]);
  const headingIdLookup = useMemo(() => {
    const map = new Map<string, string[]>();

    headingEntries.forEach((entry) => {
      const key = getHeadingLookupKey(entry.level, entry.text);
      const ids = map.get(key) || [];
      ids.push(entry.id);
      map.set(key, ids);
    });

    return map;
  }, [headingEntries]);

  // 解析Markdown内容，提取标题层级结构
  const parseToc = (entries: HeadingEntry[]): TocItem[] => {
    const toc: TocItem[] = [];
    const stack: TocItem[] = [];

    entries.forEach(({ id, text, level }) => {
      const item: TocItem = { id, text, level, children: [] };

      if (level === 1) {
        stack.length = 0;
        toc.push(item);
        stack.push(item);
      } else {
        while (stack.length > 0 && stack[stack.length - 1].level >= level) {
          stack.pop();
        }
        if (stack.length > 0) {
          stack[stack.length - 1].children.push(item);
          stack.push(item);
        } else {
          toc.push(item);
          stack.push(item);
        }
      }
    });

    return toc;
  };

  // 为搜索建立简易索引：按段落归属最近的标题路径
  const buildSearchIndex = (md: string): SearchIndexItem[] => {
    const lines = md.split('\n');
    const index: SearchIndexItem[] = [];
    const stack: HeadingEntry[] = [];
    const headingQueue = [...headingEntries];
    let inCodeFence = false;

    lines.forEach(raw => {
      if (/^```/.test(raw.trim())) {
        inCodeFence = !inCodeFence;
        return;
      }
      if (inCodeFence) return;

      const headingMatch = raw.match(/^(#{1,6})\s+(.*)$/);
      if (headingMatch) {
        const nextHeading = headingQueue.shift();
        if (!nextHeading) return;
        const { level, text, id } = nextHeading;
        while (stack.length > 0 && stack[stack.length - 1].level >= level) {
          stack.pop();
        }
        stack.push({ id, text, level });
        return;
      }

      const line = raw.trim();
      if (!line) return;
      // 跳过 Markdown 表格分隔行
      if (/^\|?(\s*:?-{3,}:?\s*\|)+\s*$/.test(line)) return;

      const current = stack[stack.length - 1];
      if (!current) return;

      const path = stack.map(s => s.text).join(' > ');
      index.push({
        sectionId: current.id,
        sectionTitle: current.text,
        sectionPath: path,
        content: line.slice(0, 400),
      });
    });

    return index;
  };

  // 处理目录项点击
  const handleTocClick = (id: string) => {
    const element = document.getElementById(id);
    if (element) {
      element.scrollIntoView({ behavior: 'smooth', block: 'start' });
      if (window.history?.replaceState) {
        window.history.replaceState(null, '', `#${id}`);
      } else {
        window.location.hash = `#${id}`;
      }
      setActiveSectionId(id);
      setCurrentSectionId(id);
      setTocOpen(false);
      window.setTimeout(() => setActiveSectionId(prev => (prev === id ? null : prev)), 2000);
    }
  };

  const handleSearchResultClick = (id: string) => {
    handleTocClick(id);
    setSearchOpen(false);
  };

  const resolveAnchorTarget = (href: string): string => {
    const raw = decodeURIComponent(href.replace(/^#/, '')).trim();
    if (!raw) return '';

    const exact = headingEntries.find(entry => entry.id === raw || entry.text === raw);
    if (exact) return exact.id;

    const normalizedRaw = normalizeHeadingText(raw).toLowerCase();
    const partial = headingEntries.find((entry) => {
      const text = normalizeHeadingText(entry.text).toLowerCase();
      return text === normalizedRaw || text.endsWith(normalizedRaw) || text.includes(normalizedRaw);
    });

    return partial?.id || slugifyHeading(raw);
  };

  const resolveRelativeDocHref = (href: string): string => {
    const cleanHref = href.replace(/^\.\//, '').replace(/^\//, '');
    if (!cleanHref) return href;
    if (cleanHref.endsWith('/')) return `${REPO_TREE_BASE}${cleanHref}`;
    return `${REPO_BLOB_BASE}${cleanHref}`;
  };

  // 处理目录项展开/折叠
  const toggleExpand = (id: string) => {
    setExpanded(prev => ({
      ...prev,
      [id]: !prev[id]
    }));
  };

  // 监听滚动，高亮当前目录项并更新阅读进度
  useEffect(() => {
    const handleScroll = () => {
      if (!contentRef.current) return;
      const scrollContainer = getScrollContainer();
      if (!scrollContainer) return;

      // 计算阅读进度
      const contentHeight = scrollContainer.scrollHeight - scrollContainer.clientHeight;
      const scrollTop = scrollContainer.scrollTop;
      const newProgress = Math.min((scrollTop / contentHeight) * 100, 100);
      setProgress(newProgress);

      // 高亮当前目录项
      const headings = contentRef.current.querySelectorAll('h1, h2, h3, h4, h5, h6');
      let currentId = '';

      headings.forEach(heading => {
        const rect = heading.getBoundingClientRect();
        const containerRect = scrollContainer.getBoundingClientRect();
        if (rect.top - containerRect.top <= 120) {
          currentId = heading.id || '';
        }
      });

      setCurrentSectionId(currentId);
    };

    const scrollContainer = getScrollContainer();
    if (!scrollContainer) return;
    scrollContainer.addEventListener('scroll', handleScroll);
    handleScroll();
    return () => scrollContainer.removeEventListener('scroll', handleScroll);
  }, []);

  // 快捷键支持
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      // Ctrl/Cmd + K 或 Ctrl/Cmd + F: 打开搜索框
      if ((event.ctrlKey || event.metaKey) && (event.key === 'k' || event.key === 'f')) {
        event.preventDefault();
        searchInputRef.current?.focus();
      }

      // Esc: 关闭搜索框或图片预览
      if (event.key === 'Escape') {
        searchInputRef.current?.blur();
      }

      // 箭头键: 导航目录（如果目录有焦点）
      // 这里可以添加目录导航的逻辑
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  useEffect(() => {
    // 加载帮助 Markdown 文档（支持多路径）
    const loadREADME = async () => {
      try {
        setLoading(true);
        setError(null);
        let text = await fetchHelpMarkdown();
        // 简单清理部分 HTML 包裹标签，避免影响 Markdown 解析
        // 去掉外层 <div align="center"> ... </div>，保留内部内容
        text = text.replace(/<div[^>]*>/gi, '').replace(/<\/div>/gi, '');
        // 去掉单独一行的 <img ...>，在页面顶部用 React 自己渲染 Logo
        text = text.replace(/^\s*<img[^>]*>\s*$/gim, '');
        setMarkdown(text);
      } catch (err) {
        console.error('Failed to load README.md:', err);
        setError(locale === 'zh-CN' ? '加载README.md文件失败' : 'Failed to load README.md file');
      } finally {
        setLoading(false);
      }
    };

    loadREADME();
  }, [locale]);

  useEffect(() => {
    setToc(parseToc(headingEntries));
    setSearchIndex(buildSearchIndex(markdown));
  }, [headingEntries, markdown]);

  // 根据搜索词实时生成结果
  useEffect(() => {
    const q = query.trim().toLowerCase();
    if (!q) {
      setSearchResults([]);
      return;
    }

    let active = true;
    const runSearch = () => {
      const results = searchIndex.filter(item => {
        const content = item.content.toLowerCase();
        const title = item.sectionTitle.toLowerCase();
        return content.includes(q) || title.includes(q);
      }).slice(0, 20);
      if (active) setSearchResults(results);
    };

    runSearch();

    return () => {
      active = false;
    };
  }, [query, searchIndex]);

  // 初次加载时如果 URL 带有 hash，则滚动到对应标题
  useEffect(() => {
    if (!markdown) return;
    const hash = window.location.hash.replace('#', '');
    if (!hash) return;
    window.setTimeout(() => {
      const el = document.getElementById(hash);
      if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'start' });
      }
    }, 0);
  }, [markdown]);

  // 渲染目录项
  const renderTocItem = (item: TocItem, level: number = 0) => {
    const isExpanded = expanded[item.id] !== false;
    const hasChildren = item.children.length > 0;
    const active = currentSectionId === item.id;

    return (
      <div key={item.id} className="mb-1">
        <div 
          className={`flex items-center gap-2 cursor-pointer py-1 px-2 rounded-md transition-colors ${active ? 'bg-blue-50 text-blue-700 dark:bg-blue-500/10 dark:text-blue-200' : 'hover:bg-gray-100 dark:hover:bg-gray-800'}`}
          style={{ paddingLeft: `${0.5 + level * 0.85}rem` }}
          onClick={() => handleTocClick(item.id)}
        >
          {hasChildren && (
            <button
              className="p-0.5"
              onClick={(e) => {
                e.stopPropagation();
                toggleExpand(item.id);
              }}
            >
              {isExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
            </button>
          )}
          {!hasChildren && <div className="w-4"></div>}
          <span className={`text-sm ${level === 0 ? 'font-semibold' : 'font-medium'}`}>
            {item.text}
          </span>
        </div>
        {hasChildren && isExpanded && (
          <div className="pl-2">
            {item.children.map(child => renderTocItem(child, level + 1))}
          </div>
        )}
      </div>
    );
  };

  const [tocOpen, setTocOpen] = useState(false);
  const renderHeadingCounts = new Map<string, number>();

  const getRenderedHeadingId = (level: number, children: ReactNode): string => {
    const text = normalizeHeadingText(extractTextFromNode(children));
    const key = getHeadingLookupKey(level, text);
    const ids = headingIdLookup.get(key);
    const count = renderHeadingCounts.get(key) || 0;
    renderHeadingCounts.set(key, count + 1);
    return ids?.[count] || slugifyHeading(text);
  };

  return (
    <div ref={pageRef} className="p-4 help-docs">
      {/* 阅读进度条 */}
      <div className="fixed top-0 left-0 right-0 h-1 bg-gray-200 dark:bg-gray-800 z-50">
        <div 
          className="h-full bg-blue-500 dark:bg-blue-400 transition-all duration-200"
          style={{ width: `${progress}%` }}
        />
      </div>
      <div className="max-w-7xl mx-auto">
        <div className="mb-6">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <h2 className="font-bold tracking-tight text-2xl text-gray-900 dark:text-white">
                {title}
              </h2>
              <p className="text-sm mt-1 text-gray-500 dark:text-gray-400">
                {locale === 'zh-CN'
                  ? '在线帮助系统，展示 ClawPanel 的完整文档内容。'
                  : 'Online help system, displaying ClawPanel\'s complete documentation.'}
              </p>
            </div>
            <div className="inline-flex items-center gap-2 rounded-full border border-blue-100 bg-blue-50/80 px-3 py-1.5 text-xs font-medium text-blue-700 dark:border-blue-500/20 dark:bg-blue-500/10 dark:text-blue-200">
              <ExternalLink size={13} />
              <span>{locale === 'zh-CN' ? '外部链接会先提示，不会直接跳出面板' : 'External links prompt first and do not leave the panel immediately'}</span>
            </div>
          </div>
        </div>

        <div className="mb-6 relative">
          <div className="flex items-center gap-3 rounded-2xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800 px-4 py-3 shadow-sm">
            <Search size={16} className="text-gray-400 dark:text-gray-500" />
            <input
              ref={searchInputRef}
              value={query}
              onChange={e => {
                setQuery(e.target.value);
                setSearchOpen(true);
              }}
              onFocus={() => setSearchOpen(true)}
              placeholder={
                locale === 'zh-CN'
                  ? '搜索帮助文档（命令、报错、关键词）…'
                  : 'Search help docs (commands, errors, keywords)…'
              }
              className="w-full bg-transparent outline-none text-sm text-gray-700 dark:text-gray-100 placeholder:text-gray-500 dark:placeholder:text-gray-500"
            />
          </div>
          {query.trim() && (
            <div className="mt-2 text-xs text-gray-500 dark:text-gray-400">
              {locale === 'zh-CN' ? `找到 ${searchResults.length} 条相关结果` : `${searchResults.length} matching results`}
            </div>
          )}
          {searchOpen && query.trim() && (
            <div className="absolute mt-3 left-0 right-0 z-40 rounded-2xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-900 shadow-lg max-h-80 overflow-y-auto">
              {searchResults.length === 0 ? (
                <div className="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                  {locale === 'zh-CN' ? '未找到相关内容' : 'No results found'}
                </div>
              ) : (
                searchResults.map((item) => {
                  const q = query.trim();
                  const highlight = (text: string) => {
                    const lower = text.toLowerCase();
                    const idx = lower.indexOf(q.toLowerCase());
                    if (idx === -1) return text;
                    const before = text.slice(0, idx);
                    const match = text.slice(idx, idx + q.length);
                    const after = text.slice(idx + q.length);
                    return (
                      <>
                        {before}
                        <mark className="bg-yellow-200 dark:bg-yellow-700 rounded px-0.5">
                          {match}
                        </mark>
                        {after}
                      </>
                    );
                  };
                  return (
                    <button
                      key={`${item.sectionId}-${item.content}`}
                      type="button"
                      onClick={() => handleSearchResultClick(item.sectionId)}
                      className="w-full px-4 py-3 text-left text-sm hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors border-b last:border-b-0 border-gray-100 dark:border-gray-800"
                    >
                      <div className="text-xs text-gray-400 dark:text-gray-500 mb-1 truncate">
                        {item.sectionPath}
                      </div>
                      <div className="font-medium text-gray-900 dark:text-gray-100 mb-1">
                        {highlight(item.sectionTitle)}
                      </div>
                      <div className="text-xs text-gray-600 dark:text-gray-300 line-clamp-2">
                        {highlight(item.content)}
                      </div>
                    </button>
                  );
                })
              )}
            </div>
          )}
        </div>

        {/* 移动端目录切换按钮 */}
        <div className="lg:hidden mb-4">
          <button
            onClick={() => setTocOpen(!tocOpen)}
            className="w-full flex items-center justify-between gap-2 rounded-2xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800 px-4 py-3 shadow-sm"
          >
            <span className="text-sm font-medium text-gray-900 dark:text-white">
              {locale === 'zh-CN' ? '目录导航' : 'Table of Contents'}
            </span>
            {tocOpen ? <PanelLeftClose size={16} className="text-gray-500" /> : <PanelLeftOpen size={16} className="text-gray-500" />}
          </button>
        </div>

        {tocOpen && (
          <div className="fixed inset-0 z-40 bg-slate-950/40 backdrop-blur-sm lg:hidden" onClick={() => setTocOpen(false)}>
            <div className="absolute left-0 top-0 h-full w-[86vw] max-w-[320px] bg-white p-4 shadow-2xl dark:bg-gray-900" onClick={(e) => e.stopPropagation()}>
              <div className="mb-4 flex items-center justify-between">
                <div className="text-sm font-semibold text-gray-900 dark:text-white">
                  {locale === 'zh-CN' ? '目录导航' : 'Table of Contents'}
                </div>
                <button type="button" onClick={() => setTocOpen(false)} className="rounded-full p-2 text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800">
                  <X size={16} />
                </button>
              </div>
              <div className="max-h-[calc(100vh-5rem)] overflow-y-auto pr-1">
                {toc.length > 0 ? toc.map(item => renderTocItem(item)) : (
                  <div className="text-sm text-gray-500 dark:text-gray-400">
                    {locale === 'zh-CN' ? '暂无目录' : 'No table of contents'}
                  </div>
                )}
              </div>
            </div>
          </div>
        )}

        <div className="flex flex-col lg:flex-row gap-6">
          {/* 左侧目录导航 */}
          <div className="hidden lg:block lg:w-64 shrink-0">
            <div className="bg-white dark:bg-gray-800 shadow-sm border border-gray-100 dark:border-gray-700/50 rounded-2xl p-4 lg:sticky lg:top-4">
              <h3 className="font-semibold text-sm text-gray-900 dark:text-white mb-3 lg:block hidden">
                {locale === 'zh-CN' ? '目录导航' : 'Table of Contents'}
              </h3>
              <div className="max-h-[80vh] overflow-y-auto pr-2">
                {toc.length > 0 ? (
                  toc.map(item => renderTocItem(item))
                ) : (
                  <div className="text-sm text-gray-500 dark:text-gray-400">
                    {locale === 'zh-CN' ? '暂无目录' : 'No table of contents'}
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* 右侧文档内容 */}
          <div className="flex-1">
            <div className="bg-white dark:bg-gray-800 shadow-sm border border-gray-100 dark:border-gray-700/50 rounded-2xl p-4 sm:p-6">
              <div className="max-w-3xl" ref={contentRef}>
                {loading && (
                  <div className="flex items-center justify-center py-12">
                    <div className="text-sm text-gray-500 dark:text-gray-400">
                      {locale === 'zh-CN' ? '加载文档中…' : 'Loading documentation…'}
                    </div>
                  </div>
                )}

                {error && (
                  <div className="flex items-center justify-center py-12">
                    <div className="text-sm text-red-500 dark:text-red-400">
                      {error}
                    </div>
                  </div>
                )}

                {!loading && !error && (
                  <div className="max-w-none text-gray-800 dark:text-gray-100 [&_h1]:mb-5 [&_h1]:mt-8 [&_h1]:text-3xl [&_h1]:font-extrabold [&_h1]:tracking-tight [&_h2]:mb-4 [&_h2]:mt-8 [&_h2]:border-b [&_h2]:border-gray-200 [&_h2]:pb-2 [&_h2]:text-2xl [&_h2]:font-bold dark:[&_h2]:border-gray-700 [&_h3]:mb-3 [&_h3]:mt-7 [&_h3]:text-xl [&_h3]:font-bold [&_h4]:mb-2 [&_h4]:mt-6 [&_h4]:text-lg [&_h4]:font-semibold [&_h5]:mb-2 [&_h5]:mt-5 [&_h5]:text-base [&_h5]:font-semibold [&_h6]:mb-2 [&_h6]:mt-5 [&_h6]:text-sm [&_h6]:font-semibold [&_p]:my-4 [&_p]:leading-7 [&_ul]:my-4 [&_ul]:list-disc [&_ul]:pl-6 [&_ol]:my-4 [&_ol]:list-decimal [&_ol]:pl-6 [&_li]:my-2 [&_li]:leading-7 [&_hr]:my-8 [&_hr]:border-gray-200 dark:[&_hr]:border-gray-700 [&_table]:my-6 [&_table]:w-full [&_table]:overflow-hidden [&_table]:rounded-xl [&_table]:border-collapse [&_table]:text-sm [&_thead]:bg-slate-100 dark:[&_thead]:bg-slate-800/70 [&_th]:border [&_th]:border-slate-300 [&_th]:px-3 [&_th]:py-2 [&_th]:text-left [&_th]:font-semibold dark:[&_th]:border-slate-700 [&_td]:border [&_td]:border-slate-300 [&_td]:px-3 [&_td]:py-2 dark:[&_td]:border-slate-700 [&_a]:font-medium [&_a]:text-blue-600 [&_a]:underline [&_a]:underline-offset-4 dark:[&_a]:text-blue-300">
                    {/* 顶部美化标题区，替代 README 中的 HTML 头部 */}
                    <div className="mb-6 pb-4 border-b border-gray-200 dark:border-gray-700">
                      <div className="flex items-center gap-4">
                        <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-blue-600 text-white shadow-lg shadow-blue-500/30">
                          <span className="text-xl font-bold">CP</span>
                        </div>
                        <div>
                          <h1 className="m-0 text-2xl font-extrabold tracking-tight">
                            ClawPanel
                          </h1>
                          <p className="mt-1 text-sm text-gray-600 dark:text-gray-300">
                            OpenClaw 智能管理面板 — 单文件部署、跨平台、全功能可视化管理（含多智能体控制台）
                          </p>
                        </div>
                      </div>
                    </div>

                    <ReactMarkdown
                      remarkPlugins={[remarkGfm]}
                      components={{
                        pre: ({ children }) => <>{children}</>,
                        h1: ({ children, ...props }) => {
                          const id = getRenderedHeadingId(1, children);
                          return (
                              <h1
                                id={id}
                                {...props}
                                className={`scroll-mt-24 ${(props as any).className || ''} ${activeSectionId === id ? 'bg-yellow-100/60 dark:bg-yellow-900/30 rounded-md px-1 -mx-1 transition-colors' : ''}`}
                              >
                              {children}
                            </h1>
                          );
                        },
                        h2: ({ children, ...props }) => {
                          const id = getRenderedHeadingId(2, children);
                          return (
                              <h2
                                id={id}
                                {...props}
                                className={`scroll-mt-24 ${(props as any).className || ''} ${activeSectionId === id ? 'bg-yellow-100/60 dark:bg-yellow-900/30 rounded-md px-1 -mx-1 transition-colors' : ''}`}
                              >
                              {children}
                            </h2>
                          );
                        },
                        h3: ({ children, ...props }) => {
                          const id = getRenderedHeadingId(3, children);
                          return (
                              <h3
                                id={id}
                                {...props}
                                className={`scroll-mt-24 ${(props as any).className || ''} ${activeSectionId === id ? 'bg-yellow-100/60 dark:bg-yellow-900/30 rounded-md px-1 -mx-1 transition-colors' : ''}`}
                              >
                              {children}
                            </h3>
                          );
                        },
                        h4: ({ children, ...props }) => {
                          const id = getRenderedHeadingId(4, children);
                          return (
                              <h4
                                id={id}
                                {...props}
                                className={`scroll-mt-24 ${(props as any).className || ''} ${activeSectionId === id ? 'bg-yellow-100/60 dark:bg-yellow-900/30 rounded-md px-1 -mx-1 transition-colors' : ''}`}
                              >
                              {children}
                            </h4>
                          );
                        },
                        h5: ({ children, ...props }) => {
                          const id = getRenderedHeadingId(5, children);
                          return (
                              <h5
                                id={id}
                                {...props}
                                className={`scroll-mt-24 ${(props as any).className || ''} ${activeSectionId === id ? 'bg-yellow-100/60 dark:bg-yellow-900/30 rounded-md px-1 -mx-1 transition-colors' : ''}`}
                              >
                              {children}
                            </h5>
                          );
                        },
                        h6: ({ children, ...props }) => {
                          const id = getRenderedHeadingId(6, children);
                          return (
                              <h6
                                id={id}
                                {...props}
                                className={`scroll-mt-24 ${(props as any).className || ''} ${activeSectionId === id ? 'bg-yellow-100/60 dark:bg-yellow-900/30 rounded-md px-1 -mx-1 transition-colors' : ''}`}
                              >
                              {children}
                            </h6>
                          );
                        },
                        a: ({ href = '', children, ...props }) => {
                          if (!href) return <span>{children}</span>;

                          if (href.startsWith('#')) {
                            const targetId = resolveAnchorTarget(href);
                            return (
                              <button
                                type="button"
                                onClick={() => handleTocClick(targetId)}
                                className="font-medium text-blue-600 underline underline-offset-4 dark:text-blue-300"
                              >
                                {children}
                              </button>
                            );
                          }

                          if (isExternalHref(href)) {
                            return (
                              <button
                                type="button"
                                onClick={() => setPendingExternalUrl(href)}
                                className="inline-flex items-center gap-1 font-medium text-blue-600 underline underline-offset-4 dark:text-blue-300"
                              >
                                <span>{children}</span>
                                <ExternalLink size={14} />
                              </button>
                            );
                          }

                          if (!href.startsWith('mailto:')) {
                            const targetHref = resolveRelativeDocHref(href);
                            return (
                              <button
                                type="button"
                                onClick={() => setPendingExternalUrl(targetHref)}
                                className="inline-flex items-center gap-1 font-medium text-blue-600 underline underline-offset-4 dark:text-blue-300"
                              >
                                <span>{children}</span>
                                <ExternalLink size={14} />
                              </button>
                            );
                          }

                          return <a href={href} {...props}>{children}</a>;
                        },
                        code: ({ node, className, children, ...props }) => {
                          const isInline =
                            !node?.position?.start?.line ||
                            node.position.start.line === node.position.end.line;
                          if (isInline) {
                            return (
                              <code className="rounded-md border border-zinc-700 bg-[linear-gradient(180deg,rgba(24,24,27,0.98),rgba(39,39,42,0.96))] px-1.5 py-0.5 text-sm font-mono text-zinc-100 shadow-[inset_0_1px_0_rgba(255,255,255,0.05)]" {...props}>
                                {children}
                              </code>
                            );
                          }

                          return (
                            <CodeBlock
                              code={String(children).replace(/\n$/, '')}
                              language={className?.replace(/language-/, '') || ''}
                              locale={locale}
                            />
                          );
                        },
                        img: ({ src, alt, title, ...props }) => (
                          <ImageWithPreview
                            src={src || ''}
                            alt={alt || ''}
                            title={title}
                            {...props}
                          />
                        ),
                        blockquote: ({ children }) => (
                          <blockquote className="my-5 rounded-2xl border border-zinc-700 bg-[linear-gradient(135deg,rgba(9,9,11,0.98),rgba(39,39,42,0.95))] px-5 py-4 text-zinc-100 shadow-[inset_0_1px_0_rgba(255,255,255,0.06),0_12px_30px_rgba(0,0,0,0.14)] [&_*]:text-inherit [&_p]:my-2 [&_p:first-child]:mt-0 [&_p:last-child]:mb-0">
                            {children}
                          </blockquote>
                        ),
                      }}
                    >
                      {markdown}
                    </ReactMarkdown>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>
      {pendingExternalUrl && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/45 p-4 backdrop-blur-sm">
          <div className="w-full max-w-lg rounded-3xl border border-slate-200 bg-white p-5 shadow-2xl dark:border-slate-700 dark:bg-slate-900">
            <div className="flex items-start justify-between gap-4">
              <div>
                <div className="text-base font-semibold text-slate-900 dark:text-white">
                  {locale === 'zh-CN' ? '准备打开外部文档' : 'Open external documentation'}
                </div>
                <p className="mt-2 break-all text-sm leading-6 text-slate-600 dark:text-slate-300">
                  {pendingExternalUrl}
                </p>
                <p className="mt-2 text-xs text-slate-500 dark:text-slate-400">
                  {locale === 'zh-CN' ? '为避免直接离开面板，这里先给出提示，确认后再新开标签页。' : 'To avoid leaving the panel unexpectedly, confirm here before opening a new tab.'}
                </p>
              </div>
              <button type="button" onClick={() => setPendingExternalUrl(null)} className="rounded-full p-2 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800">
                <X size={16} />
              </button>
            </div>
            <div className="mt-5 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <button type="button" onClick={() => setPendingExternalUrl(null)} className="rounded-xl border border-slate-200 px-4 py-2.5 text-sm font-medium text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800">
                {locale === 'zh-CN' ? '取消' : 'Cancel'}
              </button>
              <button type="button" onClick={() => { window.open(pendingExternalUrl, '_blank', 'noopener,noreferrer'); setPendingExternalUrl(null); }} className="inline-flex items-center justify-center gap-2 rounded-xl bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-blue-700">
                <ExternalLink size={15} />
                {locale === 'zh-CN' ? '新标签打开' : 'Open in new tab'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

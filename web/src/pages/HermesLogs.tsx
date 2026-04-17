import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useOutletContext } from 'react-router-dom';
import { api } from '../lib/api';
import { RefreshCw, ScrollText, TerminalSquare, Activity, Clock3, HardDrive, FileTerminal, ShieldAlert } from 'lucide-react';
import { useI18n } from '../i18n';

interface HermesLogFile {
  name: string;
  path: string;
  size?: number;
  modifiedAt?: string;
}

function formatBytes(size?: number) {
  if (!size || size <= 0) return '-';
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function logTone(line: string) {
  const content = line.toLowerCase();
  if (content.includes('error') || content.includes('traceback') || content.includes('failed')) {
    return 'text-red-300';
  }
  if (content.includes('warn') || content.includes('timeout')) {
    return 'text-amber-300';
  }
  if (content.includes('connected') || content.includes('started') || content.includes('running')) {
    return 'text-emerald-300';
  }
  return 'text-gray-200';
}

export default function HermesLogs() {
  const { locale } = useI18n();
  const { uiMode } = (useOutletContext() as { uiMode?: 'modern' }) || {};
  const navigate = useNavigate();
  const modern = uiMode === 'modern';
  const [files, setFiles] = useState<HermesLogFile[]>([]);
  const [selectedPath, setSelectedPath] = useState('');
  const [lines, setLines] = useState<string[]>([]);
  const [lineLimit, setLineLimit] = useState(120);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState('');

  const load = async (path?: string, linesCount = lineLimit) => {
    setLoading(true);
    setErr('');
    try {
      const res = await api.getHermesLogs(path, linesCount);
      if (!res?.ok) {
        setErr(res?.error || (locale === 'zh-CN' ? '加载 Hermes 日志失败' : 'Failed to load Hermes logs'));
        return;
      }
      const nextFiles = Array.isArray(res.files) ? res.files : [];
      setFiles(nextFiles);
      const nextPath = res.selectedPath || path || nextFiles[0]?.path || '';
      setSelectedPath(nextPath);
      setLines(Array.isArray(res.lines) ? res.lines : []);
    } catch {
      setErr(locale === 'zh-CN' ? '加载 Hermes 日志失败' : 'Failed to load Hermes logs');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const selectedFile = useMemo(
    () => files.find(file => file.path === selectedPath) || null,
    [files, selectedPath],
  );
  const highlightedLineCount = useMemo(
    () => lines.filter(line => /error|warn|traceback|failed|timeout/i.test(line)).length,
    [lines],
  );

  return (
    <div className={`space-y-6 ${modern ? 'page-modern' : ''}`}>
      <div className={`${modern ? 'page-modern-header' : 'flex items-center justify-between'}`}>
        <div>
          <h2 className={`${modern ? 'page-modern-title text-xl' : 'text-xl font-bold text-gray-900 dark:text-white'}`}>
            {locale === 'zh-CN' ? 'Hermes 日志中心' : 'Hermes Logs'}
          </h2>
          <p className={`${modern ? 'page-modern-subtitle mt-1 text-sm' : 'mt-1 text-sm text-gray-500'}`}>
            {locale === 'zh-CN'
              ? '直接查看 Hermes 日志目录里的文件，优先用于排查 gateway、doctor 和平台接入问题。'
              : 'Inspect Hermes log files directly for gateway, doctor, and platform troubleshooting.'}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <select
            value={lineLimit}
            onChange={e => {
              const next = Number(e.target.value);
              setLineLimit(next);
              void load(selectedPath || undefined, next);
            }}
            className="rounded-xl border border-gray-100 bg-white/80 px-3 py-2 text-xs text-gray-700 outline-none dark:border-gray-700/50 dark:bg-gray-900/40 dark:text-gray-200"
          >
            {[120, 300, 800, 1500].map(value => <option key={value} value={value}>{value} lines</option>)}
          </select>
          <button
            onClick={() => void load(selectedPath || undefined, lineLimit)}
            className={`${modern ? 'page-modern-action px-3 py-2 text-xs' : 'rounded-lg bg-gray-100 px-3 py-2 text-xs dark:bg-gray-800'} inline-flex items-center gap-2`}
          >
            <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
            {locale === 'zh-CN' ? '刷新' : 'Refresh'}
          </button>
        </div>
      </div>

      {err && (
        <div className="rounded-2xl border border-red-100 bg-red-50/80 px-4 py-3 text-sm text-red-700 dark:border-red-900/30 dark:bg-red-900/10 dark:text-red-300">
          {err}
        </div>
      )}

      <div className={`${modern ? 'rounded-[30px] border border-white/60 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.9),rgba(239,246,255,0.66))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.9),rgba(30,64,175,0.12))] backdrop-blur-xl shadow-[0_24px_54px_rgba(15,23,42,0.08)]' : 'rounded-2xl border border-gray-100 bg-white dark:bg-gray-800 dark:border-gray-700/50'} p-5`}>
        <div className="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
          <div className="space-y-2">
            <div className="inline-flex items-center gap-2 rounded-full border border-blue-100/70 bg-white/80 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.18em] text-blue-700 dark:border-blue-400/15 dark:bg-slate-900/50 dark:text-blue-200">
              <FileTerminal size={13} />
              {locale === 'zh-CN' ? '运行日志视图' : 'Runtime Log View'}
            </div>
            <div className="text-sm text-gray-600 dark:text-gray-300">
              {locale === 'zh-CN'
                ? '优先看最近异常、网关重连和平台接入日志。切文件后会自动拉取尾部内容，适合快速定位问题。'
                : 'Focus on recent exceptions, gateway reconnects, and platform access logs. Switching files automatically tails the latest content.'}
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {[
              { label: locale === 'zh-CN' ? '日志文件' : 'Files', value: files.length, icon: ScrollText, tone: 'text-blue-600 bg-blue-50 dark:bg-blue-900/20' },
              { label: locale === 'zh-CN' ? '尾部行数' : 'Tail', value: lines.length, icon: TerminalSquare, tone: 'text-violet-600 bg-violet-50 dark:bg-violet-900/20' },
              { label: locale === 'zh-CN' ? '异常提示' : 'Highlights', value: highlightedLineCount, icon: ShieldAlert, tone: 'text-amber-600 bg-amber-50 dark:bg-amber-900/20' },
              { label: locale === 'zh-CN' ? '当前大小' : 'Size', value: formatBytes(selectedFile?.size), icon: HardDrive, tone: 'text-emerald-600 bg-emerald-50 dark:bg-emerald-900/20' },
            ].map(card => (
              <div key={card.label} className="rounded-2xl border border-white/70 bg-white/75 p-4 shadow-[0_12px_30px_rgba(15,23,42,0.06)] dark:border-slate-700/50 dark:bg-slate-900/45">
                <div className="flex items-center justify-between gap-3">
                  <span className={`inline-flex h-10 w-10 items-center justify-center rounded-2xl ${card.tone}`}>
                    <card.icon size={18} />
                  </span>
                  <span className="text-[10px] font-semibold uppercase tracking-[0.18em] text-slate-400">{card.label}</span>
                </div>
                <div className="mt-4 text-xl font-bold tracking-tight text-slate-900 dark:text-white break-all">{card.value}</div>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        {[
          { label: locale === 'zh-CN' ? '日志文件' : 'Log Files', value: files.length },
          { label: locale === 'zh-CN' ? '当前文件' : 'Selected File', value: selectedFile?.name || '-' },
          { label: locale === 'zh-CN' ? '尾部行数' : 'Tail Lines', value: lines.length },
          { label: locale === 'zh-CN' ? '文件大小' : 'File Size', value: formatBytes(selectedFile?.size) },
        ].map(card => (
          <div key={card.label} className={`${modern ? 'page-modern-card' : 'bg-white dark:bg-gray-800'} rounded-2xl border border-gray-100 p-4 dark:border-gray-700/50`}>
            <div className="text-xs font-semibold uppercase tracking-wider text-gray-500">{card.label}</div>
            <div className="mt-3 text-lg font-bold text-gray-900 dark:text-white break-all">{card.value}</div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[320px_1fr] gap-6">
        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} rounded-2xl p-4 space-y-3`}>
          <div className="flex items-center justify-between gap-2">
            <div className="font-semibold text-gray-900 dark:text-white">{locale === 'zh-CN' ? '日志文件列表' : 'Log Files'}</div>
            <button
              onClick={() => navigate('/hermes/tasks')}
              className={`${modern ? 'page-modern-action px-3 py-2 text-xs' : 'rounded-lg bg-gray-100 px-3 py-2 text-xs dark:bg-gray-800'}`}
            >
              {locale === 'zh-CN' ? '打开任务' : 'Open Tasks'}
            </button>
          </div>
          <div className="space-y-2">
            {files.length === 0 && !loading && (
              <div className="rounded-xl border border-dashed border-gray-200 px-4 py-6 text-sm text-gray-500 dark:border-gray-700/50 dark:text-gray-400">
                {locale === 'zh-CN' ? '当前没有可读日志文件' : 'No readable log files'}
              </div>
            )}
            {files.map(file => (
              <button
                key={file.path}
                onClick={() => void load(file.path, lineLimit)}
                className={`w-full rounded-xl border px-4 py-3 text-left transition-colors ${
                  selectedPath === file.path
                    ? 'border-blue-300 bg-[linear-gradient(145deg,rgba(239,246,255,0.95),rgba(219,234,254,0.8))] shadow-[0_12px_28px_rgba(59,130,246,0.12)] dark:border-blue-700 dark:bg-blue-900/20'
                    : 'border-gray-100 hover:bg-gray-50 dark:border-gray-700/50 dark:hover:bg-gray-900/40'
                }`}
              >
                <div className="flex items-start gap-3">
                  <ScrollText size={16} className="mt-0.5 shrink-0 text-blue-500" />
                  <div className="min-w-0 flex-1">
                    <div className="truncate font-medium text-gray-900 dark:text-white">{file.name}</div>
                    <div className="mt-1 truncate text-xs text-gray-500">{file.path}</div>
                    <div className="mt-2 flex items-center gap-2 text-[11px] text-gray-400">
                      <span>{formatBytes(file.size)}</span>
                      <span>·</span>
                      <span className="inline-flex items-center gap-1">
                        <Clock3 size={11} />
                        {file.modifiedAt || '-'}
                      </span>
                    </div>
                  </div>
                </div>
              </button>
            ))}
          </div>
        </div>

        <div className={`${modern ? 'page-modern-panel' : 'bg-white dark:bg-gray-800 border border-gray-100 dark:border-gray-700/50'} overflow-hidden rounded-2xl`}>
          <div className="flex items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-gray-700/50">
            <div className="min-w-0">
              <div className="flex items-center gap-2 font-semibold text-gray-900 dark:text-white">
                <TerminalSquare size={17} className="text-blue-500" />
                <span className="truncate">{selectedFile?.name || (locale === 'zh-CN' ? '日志预览' : 'Log Preview')}</span>
              </div>
              <div className="mt-1 text-xs text-gray-500 break-all">{selectedFile?.path || '-'}</div>
            </div>
            <button
              onClick={() => navigate('/hermes/actions')}
              className={`${modern ? 'page-modern-action px-3 py-2 text-xs' : 'rounded-lg bg-gray-100 px-3 py-2 text-xs dark:bg-gray-800'} shrink-0 inline-flex items-center gap-2`}
            >
              <Activity size={14} />
              {locale === 'zh-CN' ? '查看动作' : 'Actions'}
            </button>
          </div>
          <div className="border-b border-gray-100 px-5 py-3 text-xs text-gray-500 dark:border-gray-700/50 dark:text-gray-400">
            {locale === 'zh-CN'
              ? '高亮规则：错误为红色、警告为黄色、正常启动/连接为绿色。'
              : 'Highlighting: errors in red, warnings in amber, startup/connected lines in green.'}
          </div>
          <div className="max-h-[70vh] overflow-auto bg-gray-950 px-4 py-4 font-mono text-xs leading-6 text-gray-200">
            {lines.length === 0 && !loading ? (
              <div className="rounded-xl border border-dashed border-gray-800 px-4 py-6 text-gray-500">
                {locale === 'zh-CN' ? '当前文件暂无可显示内容' : 'No content available for the selected file'}
              </div>
            ) : (
              lines.map((line, index) => (
                <div key={`${index}-${line.slice(0, 24)}`} className={`whitespace-pre-wrap break-all py-px ${logTone(line)}`}>
                  <span className="mr-3 select-none text-gray-600">{String(index + 1).padStart(4, '0')}</span>
                  <span>{line}</span>
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

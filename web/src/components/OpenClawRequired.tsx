import { Brain, Download, Loader2, X } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { api } from '../lib/api';

interface Props {
  configured: boolean;
  children: React.ReactNode;
}

export default function OpenClawRequired({ configured, children }: Props) {
  const { pathname } = useLocation();
  const dismissKey = `openclaw-required-dismissed:${pathname}`;
  const [installing, setInstalling] = useState(false);
  const [dismissed, setDismissed] = useState(() => sessionStorage.getItem(dismissKey) === '1');

  useEffect(() => {
    if (configured) {
      const keysToClear: string[] = [];
      for (let i = 0; i < sessionStorage.length; i += 1) {
        const key = sessionStorage.key(i);
        if (key?.startsWith('openclaw-required-dismissed:')) keysToClear.push(key);
      }
      keysToClear.forEach(key => sessionStorage.removeItem(key));
      setDismissed(false);
    }
  }, [configured, dismissKey]);

  useEffect(() => {
    setDismissed(sessionStorage.getItem(dismissKey) === '1');
  }, [dismissKey]);

  if (configured) return <>{children}</>;

  const handleInstall = async () => {
    setInstalling(true);
    try { await api.installSoftware('openclaw'); } catch {}
    finally { setInstalling(false); }
  };

  const dismiss = () => {
    sessionStorage.setItem(dismissKey, '1');
    setDismissed(true);
  };

  const reopen = () => {
    sessionStorage.removeItem(dismissKey);
    setDismissed(false);
  };

  if (dismissed) {
    return (
      <div className="space-y-3">
        <div className="rounded-2xl border border-amber-200/80 dark:border-amber-900/50 bg-amber-50/90 dark:bg-amber-950/20 px-4 py-3 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <div className="text-sm font-semibold text-amber-900 dark:text-amber-200">OpenClaw 尚未就绪</div>
            <p className="text-xs text-amber-700 dark:text-amber-300 mt-1">
              已暂时关闭阻断提示，便于前端调试；当前页面的实时数据和保存能力可能不完整。
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              onClick={handleInstall}
              disabled={installing}
              className="inline-flex items-center gap-2 px-4 py-2 text-xs font-medium rounded-xl bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50 transition-all"
            >
              {installing ? <Loader2 size={14} className="animate-spin" /> : <Download size={14} />}
              {installing ? '安装中...' : '安装 OpenClaw'}
            </button>
            <button
              onClick={reopen}
              className="px-4 py-2 text-xs font-medium rounded-xl border border-amber-300/80 dark:border-amber-800 text-amber-700 dark:text-amber-300 hover:bg-amber-100/70 dark:hover:bg-amber-900/30 transition-colors"
            >
              重新显示提示
            </button>
          </div>
        </div>
        {children}
      </div>
    );
  }

  return (
    <div className="relative">
      {/* Greyed out content */}
      <div className="opacity-20 pointer-events-none select-none blur-[2px]">
        {children}
      </div>
      {/* Overlay */}
      <div className="absolute inset-0 flex items-center justify-center z-10">
        <div className="relative bg-white/95 dark:bg-gray-900/95 backdrop-blur-sm rounded-2xl shadow-2xl border border-gray-200 dark:border-gray-700 p-8 max-w-md text-center space-y-4">
          <button
            onClick={dismiss}
            className="absolute top-4 right-4 p-2 rounded-xl text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
            title="关闭提示并继续查看页面"
          >
            <X size={16} />
          </button>
          <div className="w-14 h-14 mx-auto rounded-2xl bg-amber-100 dark:bg-amber-900/30 flex items-center justify-center">
            <Brain size={28} className="text-amber-600 dark:text-amber-400" />
          </div>
          <div>
            <h3 className="text-lg font-bold text-gray-900 dark:text-white">需要安装或配置 OpenClaw</h3>
            <p className="text-sm text-gray-500 mt-1">
              此功能依赖 OpenClaw AI 引擎。你可以先安装 / 配置，也可以先关闭提示继续调试页面结构。
            </p>
          </div>
          <div className="flex flex-col sm:flex-row gap-3 justify-center">
            <button onClick={handleInstall} disabled={installing}
              className="page-modern-accent inline-flex items-center justify-center gap-2 px-6 py-3 text-sm disabled:opacity-50">
              {installing ? <Loader2 size={16} className="animate-spin" /> : <Download size={16} />}
              {installing ? '安装中...' : '一键安装 OpenClaw'}
            </button>
            <button
              onClick={dismiss}
              className="inline-flex items-center justify-center gap-2 px-6 py-3 text-sm font-medium rounded-xl border border-gray-200 dark:border-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
            >
              关闭提示，继续查看页面
            </button>
          </div>
          <p className="text-[11px] text-gray-400">安装进度可在右上角铃铛中的消息中心实时查看</p>
        </div>
      </div>
    </div>
  );
}

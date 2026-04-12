import { Brain, Download, Loader2, X } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { api } from '../lib/api';
import { ensureOpenClawInstallPrerequisites, getOpenClawInstallPrerequisiteStatus } from '../lib/openclawPrereq';
import { resolveOpenClawRuntime } from '../lib/openclawRuntime';

interface Props {
  openclawStatus?: any;
  processStatus?: any;
  children: React.ReactNode;
}

export default function OpenClawRequired({ openclawStatus, processStatus, children }: Props) {
  const { pathname } = useLocation();
  const dismissKey = `openclaw-required-dismissed:${pathname}`;
  const [detectedConfigured, setDetectedConfigured] = useState(false);
  const configured = !!openclawStatus?.configured || detectedConfigured;
  const isLiteEdition = openclawStatus?.edition === 'lite';
  const runtime = resolveOpenClawRuntime(openclawStatus, processStatus);
  const [installing, setInstalling] = useState(false);
  const [dismissed, setDismissed] = useState(() => sessionStorage.getItem(dismissKey) === '1');
  const [installBlocked, setInstallBlocked] = useState(false);
  const [installBlockedMessage, setInstallBlockedMessage] = useState('');
  const [installFeedback, setInstallFeedback] = useState('');
  const [installError, setInstallError] = useState('');
  const [nodeUrl, setNodeUrl] = useState('https://nodejs.org');
  const [gitUrl, setGitUrl] = useState('https://git-scm.com/downloads');

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

  const pollOpenClawReady = async () => {
    for (let i = 0; i < 12; i += 1) {
      await new Promise(resolve => window.setTimeout(resolve, 5000));
      try {
        const status = await api.getStatus();
        if (status?.ok && status?.openclaw?.configured) {
          setDetectedConfigured(true);
          setInstallFeedback('OpenClaw 已检测到，页面状态已自动刷新。');
          setInstallError('');
          return;
        }
      } catch {
        // ignore transient polling errors
      }
    }
  };

  useEffect(() => {
    if (isLiteEdition) {
      setInstallBlocked(true);
      setInstallBlockedMessage('Lite 版已内置 OpenClaw；若当前未就绪，请检查内置 runtime 是否完整或重新安装 Lite。');
      return;
    }
    let active = true;
    getOpenClawInstallPrerequisiteStatus().then(status => {
      if (!active) return;
      setInstallBlocked(status.requiresManualInstall);
      setInstallBlockedMessage(status.message || '');
      setNodeUrl(status.nodeUrl);
      setGitUrl(status.gitUrl);
    }).catch(() => {
      if (!active) return;
      setInstallBlocked(false);
      setInstallBlockedMessage('');
    });
    return () => { active = false; };
  }, [isLiteEdition]);

  if (configured && runtime.healthy) return <>{children}</>;

  // Layout already renders the global runtime banner. Keep route content visible
  // here to avoid duplicating the same offline warning inside each page.
  if (configured && !runtime.healthy) return <>{children}</>;

  const handleInstall = async () => {
    setInstalling(true);
    setInstallFeedback('');
    setInstallError('');
    try {
      if (isLiteEdition) return;
      const status = await ensureOpenClawInstallPrerequisites();
      if (status.requiresManualInstall) {
        setInstallBlocked(true);
        setInstallBlockedMessage(status.message || '请先手动安装 Node.js 与 Git');
        return;
      }
      const r = await api.installSoftware('openclaw');
      if (!r?.ok) {
        setInstallError(r?.error || 'OpenClaw 安装任务创建失败');
        return;
      }
      setInstallFeedback(r?.message || 'OpenClaw 安装任务已创建，请在右上角消息中心查看实时进度。安装完成后会自动重新检测。');
      void pollOpenClawReady();
    } catch {
      setInstallError('OpenClaw 安装请求失败，请检查网络或稍后重试');
    } finally {
      setInstalling(false);
    }
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
        <div className="rounded-2xl border border-amber-200/80 bg-amber-50/90 px-4 py-3 dark:border-amber-900/50 dark:bg-amber-950/20 lg:flex lg:items-center lg:justify-between">
          <div>
            <div className="text-sm font-semibold text-amber-900 dark:text-amber-200">OpenClaw 尚未安装或配置</div>
            <p className="mt-1 text-xs text-amber-700 dark:text-amber-300">
              当前页面仍可浏览，但部分实时数据和保存功能可能暂时不可用。
            </p>
            {installBlockedMessage && <p className="mt-2 text-xs text-amber-700 dark:text-amber-300">{installBlockedMessage}</p>}
            {installFeedback && <p className="mt-2 text-xs text-emerald-700 dark:text-emerald-300">{installFeedback}</p>}
            {installError && <p className="mt-2 text-xs text-red-700 dark:text-red-300">{installError}</p>}
          </div>
          <div className="mt-3 flex flex-wrap gap-2 lg:mt-0">
            <button
              onClick={handleInstall}
              disabled={installing || installBlocked}
              className="inline-flex items-center gap-2 rounded-xl bg-violet-600 px-4 py-2 text-xs font-medium text-white transition-all hover:bg-violet-700 disabled:opacity-50"
            >
              {installing ? <Loader2 size={14} className="animate-spin" /> : <Download size={14} />}
              {installing ? '安装中...' : (isLiteEdition ? 'Lite 已内置 OpenClaw' : '安装 OpenClaw')}
            </button>
            {installBlocked && (
              <>
                {!isLiteEdition && <button onClick={() => window.open(nodeUrl, '_blank', 'noopener,noreferrer')} className="rounded-xl border border-blue-200 px-4 py-2 text-xs font-medium text-blue-700 transition-colors hover:bg-blue-50">下载 Node.js</button>}
                {!isLiteEdition && <button onClick={() => window.open(gitUrl, '_blank', 'noopener,noreferrer')} className="rounded-xl border border-blue-200 px-4 py-2 text-xs font-medium text-blue-700 transition-colors hover:bg-blue-50">下载 Git</button>}
              </>
            )}
            <button
              onClick={reopen}
              className="rounded-xl border border-amber-300/80 px-4 py-2 text-xs font-medium text-amber-700 transition-colors hover:bg-amber-100/70 dark:border-amber-800 dark:text-amber-300 dark:hover:bg-amber-900/30"
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
      <div className="absolute inset-0 z-10 flex items-center justify-center">
        <div className="relative max-w-md space-y-4 rounded-2xl border border-gray-200 bg-white/95 p-8 text-center shadow-2xl backdrop-blur-sm dark:border-gray-700 dark:bg-gray-900/95">
          <button
            onClick={dismiss}
            className="absolute right-4 top-4 rounded-xl p-2 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-800 dark:hover:text-gray-200"
            title="关闭提示并继续查看页面"
          >
            <X size={16} />
          </button>
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-amber-100 dark:bg-amber-900/30">
            <Brain size={28} className="text-amber-600 dark:text-amber-400" />
          </div>
          <div>
            <h3 className="text-lg font-bold text-gray-900 dark:text-white">
              {isLiteEdition ? 'Lite 内置 OpenClaw 未就绪' : '需要安装或配置 OpenClaw'}
            </h3>
            <p className="mt-1 text-sm text-gray-500">
              {isLiteEdition
                ? 'Lite 版默认自带 OpenClaw。若当前仍不可用，请检查安装包是否完整，或重新安装 Lite。'
                : '此功能依赖 OpenClaw AI 引擎。你可以先安装或配置，也可以先关闭提示继续调试页面结构。'}
            </p>
            {installBlockedMessage && <p className="mt-3 text-xs leading-5 text-amber-600 dark:text-amber-300">{installBlockedMessage}</p>}
            {installFeedback && <p className="mt-3 text-xs leading-5 text-emerald-600 dark:text-emerald-300">{installFeedback}</p>}
            {installError && <p className="mt-3 text-xs leading-5 text-red-600 dark:text-red-300">{installError}</p>}
          </div>
          <div className="flex flex-col justify-center gap-3 sm:flex-row">
            <button
              onClick={handleInstall}
              disabled={installing || installBlocked}
              className="page-modern-accent inline-flex items-center justify-center gap-2 px-6 py-3 text-sm disabled:opacity-50"
            >
              {installing ? <Loader2 size={16} className="animate-spin" /> : <Download size={16} />}
              {installing ? '安装中...' : (isLiteEdition ? 'Lite 已内置 OpenClaw' : '一键安装 OpenClaw')}
            </button>
            {installBlocked && (
              <>
                {!isLiteEdition && <button onClick={() => window.open(nodeUrl, '_blank', 'noopener,noreferrer')} className="inline-flex items-center justify-center gap-2 rounded-xl border border-blue-200 px-6 py-3 text-sm font-medium text-blue-700 transition-colors hover:bg-blue-50">下载 Node.js</button>}
                {!isLiteEdition && <button onClick={() => window.open(gitUrl, '_blank', 'noopener,noreferrer')} className="inline-flex items-center justify-center gap-2 rounded-xl border border-blue-200 px-6 py-3 text-sm font-medium text-blue-700 transition-colors hover:bg-blue-50">下载 Git</button>}
              </>
            )}
            <button
              onClick={dismiss}
              className="inline-flex items-center justify-center gap-2 rounded-xl border border-gray-200 px-6 py-3 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
            >
              关闭提示，继续查看页面
            </button>
          </div>
          <p className="text-[11px] text-gray-400">安装进度可在右上角消息中心实时查看</p>
        </div>
      </div>
      {children}
    </div>
  );
}

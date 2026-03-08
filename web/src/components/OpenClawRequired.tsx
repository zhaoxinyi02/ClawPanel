import { Brain, Download, Loader2, X } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { api } from '../lib/api';
import { ensureOpenClawInstallPrerequisites, getOpenClawInstallPrerequisiteStatus } from '../lib/openclawPrereq';

interface Props {
  configured: boolean;
  children: React.ReactNode;
}

export default function OpenClawRequired({ configured, children }: Props) {
  const { pathname } = useLocation();
  const dismissKey = `openclaw-required-dismissed:${pathname}`;
  const [installing, setInstalling] = useState(false);
  const [dismissed, setDismissed] = useState(() => sessionStorage.getItem(dismissKey) === '1');
  const [installBlocked, setInstallBlocked] = useState(false);
  const [installBlockedMessage, setInstallBlockedMessage] = useState('');
  const [openClawInstalled, setOpenClawInstalled] = useState<boolean | null>(null);

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

  useEffect(() => {
    let active = true;
    getOpenClawInstallPrerequisiteStatus().then(status => {
      if (!active) return;
      setInstallBlocked(status.requiresManualInstall);
      setInstallBlockedMessage(status.message || '');
    }).catch(() => {
      if (!active) return;
      setInstallBlocked(false);
      setInstallBlockedMessage('');
    });
    return () => { active = false; };
  }, []);

  useEffect(() => {
    let active = true;
    api.getSoftwareList().then(r => {
      if (!active || !r?.ok) return;
      const software = Array.isArray(r.software) ? r.software : [];
      const openclaw = software.find((s: any) => s.id === 'openclaw');
      setOpenClawInstalled(!!openclaw?.installed);
      if (!openclaw?.installed) {
        sessionStorage.removeItem(dismissKey);
        setDismissed(false);
      }
    }).catch(() => {
      if (!active) return;
      setOpenClawInstalled(null);
    });
    return () => { active = false; };
  }, [dismissKey]);

  const forceBlock = openClawInstalled === false;
  const ready = configured && !forceBlock;

  if (ready) return <>{children}</>;

  const handleInstall = async () => {
    setInstalling(true);
    try {
      const okToInstall = await ensureOpenClawInstallPrerequisites();
      if (!okToInstall) return;
      const r = await api.installSoftware('openclaw');
      if (!r?.ok) {
        window.alert(r?.error || '安装 OpenClaw 失败');
      }
    } catch (err: any) {
      window.alert(err?.message || '安装 OpenClaw 请求失败');
    }
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

  if (dismissed && !forceBlock) {
    return (
      <div className="space-y-3">
        <div className="rounded-2xl border border-amber-200/80 dark:border-amber-900/50 bg-amber-50/90 dark:bg-amber-950/20 px-4 py-3 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <div className="text-sm font-semibold text-amber-900 dark:text-amber-200">OpenClaw 尚未就绪</div>
            <p className="text-xs text-amber-700 dark:text-amber-300 mt-1">
              已暂时关闭阻断提示，便于前端调试；当前页面的实时数据和保存能力可能不完整。
            </p>
            {installBlockedMessage && (
              <p className="text-xs text-amber-700 dark:text-amber-300 mt-2">{installBlockedMessage}</p>
            )}
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              onClick={handleInstall}
              disabled={installing || installBlocked}
              className="inline-flex items-center gap-2 px-4 py-2 text-xs font-medium rounded-xl bg-violet-600 text-white hover:bg-violet-700 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
              title={installBlockedMessage || '安装 OpenClaw'}
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
          {!forceBlock && (
            <button
              onClick={dismiss}
              className="absolute top-4 right-4 p-2 rounded-xl text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
              title="关闭提示并继续查看页面"
            >
              <X size={16} />
            </button>
          )}
          <div className="w-14 h-14 mx-auto rounded-2xl bg-amber-100 dark:bg-amber-900/30 flex items-center justify-center">
            <Brain size={28} className="text-amber-600 dark:text-amber-400" />
          </div>
          <div>
            <h3 className="text-lg font-bold text-gray-900 dark:text-white">需要安装或配置 OpenClaw</h3>
             <p className="text-sm text-gray-500 mt-1">
               {forceBlock
                 ? '此功能依赖 OpenClaw AI 引擎。检测到当前未安装 OpenClaw，请先完成安装后再继续使用。'
                 : '此功能依赖 OpenClaw AI 引擎。你可以先安装 / 配置，也可以先关闭提示继续调试页面结构。'}
             </p>
            {installBlockedMessage && (
              <p className="text-xs text-amber-600 dark:text-amber-300 mt-3">{installBlockedMessage}</p>
            )}
          </div>
          <div className="flex flex-col sm:flex-row gap-3 justify-center">
            <button onClick={handleInstall} disabled={installing || installBlocked}
              title={installBlockedMessage || '一键安装 OpenClaw'}
              className="page-modern-accent px-6 py-3 text-sm disabled:opacity-50 disabled:cursor-not-allowed">
              {installing ? <Loader2 size={16} className="animate-spin" /> : <Download size={16} />}
              {installing ? '安装中...' : '一键安装 OpenClaw'}
            </button>
            {!forceBlock && (
              <button
                onClick={dismiss}
                className="inline-flex items-center justify-center gap-2 px-6 py-3 text-sm font-medium rounded-xl border border-gray-200 dark:border-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
              >
                关闭提示，继续查看页面
              </button>
            )}
          </div>
          <p className="text-[11px] text-gray-400">安装进度可在右上角铃铛中的消息中心实时查看</p>
        </div>
      </div>
    </div>
  );
}

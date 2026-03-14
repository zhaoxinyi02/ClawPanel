import { useEffect, useMemo, useState, useCallback, useRef } from 'react';
import { X, ChevronLeft, Copy, ArrowRight } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { useI18n } from '../i18n';
import { DEFAULT_ONBOARDING_FORCE_TOKEN, DEFAULT_ONBOARDING_VERSION, HelpFaqItem, setOnboardingSeen } from '../constants/help';

interface OnboardingModalProps {
  open: boolean;
  onClose: () => void;
  version?: string;
  forceToken?: string;
  updateAvailable?: boolean;
  latestVersion?: string;
}

const TOTAL_STEPS = 5;

export default function OnboardingModal({ open, onClose, version = DEFAULT_ONBOARDING_VERSION, forceToken = DEFAULT_ONBOARDING_FORCE_TOKEN, updateAvailable = false, latestVersion }: OnboardingModalProps) {
  const { t, locale } = useI18n();
  const navigate = useNavigate();
  const [step, setStep] = useState(1);
  const [navIndex, setNavIndex] = useState(0);
  const [spot, setSpot] = useState<{ top: number; left: number; width: number; height: number } | null>(null);
  const [faqItems] = useState<HelpFaqItem[]>([
    { question: '安装后 `systemctl start` 需要密码', answer: '需要 sudo 权限，输入 Linux 系统密码，不是面板密码。' },
    { question: '面板默认登录密码', answer: '`clawpanel`，首次登录后建议立即修改。' },
    { question: '访问面板显示空白 / 无法连接', answer: '检查服务状态、防火墙和 19527 端口是否放行。' },
    { question: 'macOS 安装报错“无法验证开发者”', answer: '运行 `sudo xattr -d com.apple.quarantine /opt/clawpanel/clawpanel`。' },
    { question: '检查更新显示“服务器错误”', answer: '确认服务器可访问 `39.102.53.188:16198`。' },
  ]);
  const [faqExpanded, setFaqExpanded] = useState<number[]>([]);
  const dialogRef = useRef<HTMLDivElement | null>(null);
  const previousFocusRef = useRef<HTMLElement | null>(null);

  const completeOnboarding = useCallback(() => {
    setOnboardingSeen(version, forceToken);
  }, [forceToken, version]);

  // 所有Hook必须放在最前面，不能有条件返回
  const navTargets = useMemo(() => ([
    {
      id: 'nav-dashboard',
      title: t.onboarding?.coreNavDashboardTitle ?? (locale === 'zh-CN' ? '仪表盘' : 'Dashboard'),
      desc: locale === 'zh-CN' ? '查看系统总览' : 'See system overview',
    },
    {
      id: 'nav-channels',
      title: t.onboarding?.coreNavChannelsTitle ?? (locale === 'zh-CN' ? '通道管理' : 'Channels'),
      desc: locale === 'zh-CN' ? '管理消息通道' : 'Manage channels',
    },
    {
      id: 'nav-config',
      title: t.onboarding?.coreNavConfigTitle ?? (locale === 'zh-CN' ? '配置中心' : 'Config'),
      desc: locale === 'zh-CN' ? '调整系统配置' : 'Adjust settings',
    },
    {
      id: 'nav-skills',
      title: t.onboarding?.coreNavSkillsTitle ?? (locale === 'zh-CN' ? '技能中心' : 'Skills'),
      desc: locale === 'zh-CN' ? '启用技能扩展' : 'Enable skills',
    },
    {
      id: 'nav-plugins',
      title: t.onboarding?.coreNavPluginsTitle ?? (locale === 'zh-CN' ? '插件中心' : 'Plugins'),
      desc: locale === 'zh-CN' ? '安装更新插件' : 'Manage plugins',
    },
    ...(updateAvailable ? [{
      id: 'update-entry',
      title: locale === 'zh-CN' ? '更新入口' : 'Update Entry',
      desc: locale === 'zh-CN'
        ? `检测到新版本${latestVersion ? ` ${latestVersion}` : ''}，这里可以快速前往升级页面。`
        : `A new version${latestVersion ? ` ${latestVersion}` : ''} is available. Use this entry to jump to the upgrade flow.`,
    }] : []),
  ]), [latestVersion, locale, t.onboarding, updateAvailable]);

  useEffect(() => {
    if (!open) return;
    // Reset state every time it opens to avoid getting stuck in a later step.
        setStep(1);
    setNavIndex(0);
    setSpot(null);
    setFaqExpanded([]);
  }, [open]);

  useEffect(() => {
    if (!open) return;

    previousFocusRef.current = document.activeElement as HTMLElement | null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';

    const focusTimer = window.setTimeout(() => {
      const firstFocusable = dialogRef.current?.querySelector<HTMLElement>('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])');
      firstFocusable?.focus();
    }, 0);

    return () => {
      window.clearTimeout(focusTimer);
      document.body.style.overflow = previousOverflow;
      previousFocusRef.current?.focus?.();
    };
  }, [open, step, navIndex]);

  useEffect(() => {
    if (!open) return;

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Tab' || !dialogRef.current) return;

      const focusables = Array.from(dialogRef.current.querySelectorAll<HTMLElement>('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'))
        .filter(el => !el.hasAttribute('disabled') && el.tabIndex !== -1);

      if (focusables.length === 0) return;

      const first = focusables[0];
      const last = focusables[focusables.length - 1];
      const active = document.activeElement as HTMLElement | null;

      if (event.shiftKey) {
        if (active === first || !dialogRef.current.contains(active)) {
          event.preventDefault();
          last.focus();
        }
        return;
      }

      if (active === last) {
        event.preventDefault();
        first.focus();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [open]);

  const handleSkip = () => {
    void completeOnboarding();
    onClose();
  };

  const handleStart = () => {
    if (step === 3) {
      if (navIndex < navTargets.length - 1) {
        setNavIndex(i => i + 1);
      } else {
        setStep(4);
        setNavIndex(0);
      }
      return;
    }

    if (step < TOTAL_STEPS) setStep(step + 1);
    else {
      completeOnboarding();
      onClose();
    }
  };

  const handlePrev = () => {
    if (step === 3) {
      if (navIndex > 0) setNavIndex(i => i - 1);
      else setStep(2);
      return;
    }
    if (step > 1) setStep(step - 1);
  };

  const toggleFaqItem = useCallback((index: number) => {
    setFaqExpanded(prev => (prev.includes(index) ? prev.filter(item => item !== index) : [...prev, index]));
  }, []);

  const handleViewFullFaq = useCallback(() => {
    navigate('/docs#faq');
    onClose();
  }, [navigate, onClose]);

  const handleFinishAndClose = useCallback(() => {
    completeOnboarding();
    onClose();
  }, [completeOnboarding, onClose]);

  const handleFinishAndOpenDocs = useCallback(() => {
    completeOnboarding();
    navigate('/docs');
    onClose();
  }, [completeOnboarding, navigate, onClose]);

  // 所有Hook必须放在条件返回之前
  useEffect(() => {
    if (!open || step !== 3) return;

    const id = navTargets[navIndex]?.id;
    if (!id) return;

    const update = () => {
      const candidates = Array.from(document.querySelectorAll(`[data-tour=\"${id}\"]`)) as HTMLElement[];
      const el = candidates.find((node) => {
        const rect = node.getBoundingClientRect();
        const style = window.getComputedStyle(node);
        return rect.width > 0 && rect.height > 0 && style.visibility !== 'hidden' && style.display !== 'none';
      }) || null;
      if (!el) {
        setSpot(null);
        return;
      }
      const r = el.getBoundingClientRect();
      setSpot({ top: r.top, left: r.left, width: r.width, height: r.height });
    };

    update();

    window.addEventListener('resize', update);
    window.addEventListener('scroll', update, true);
    const t = window.setInterval(update, 500);
    return () => {
      window.removeEventListener('resize', update);
      window.removeEventListener('scroll', update, true);
      window.clearInterval(t);
    };
  }, [navIndex, navTargets, open, step]);

  if (!open) return null;

  const title = t.onboarding?.welcomeTitle ?? (locale === 'zh-CN' ? '欢迎使用 ClawPanel' : 'Welcome to ClawPanel');
  const skipLabel = t.onboarding?.skipGuide ?? (locale === 'zh-CN' ? '跳过引导' : 'Skip Guide');
  const startLabel = t.onboarding?.startGuide ?? (locale === 'zh-CN' ? '开始引导' : 'Start Guide');
  const slogan = t.onboarding?.slogan ?? (locale === 'zh-CN'
    ? 'OpenClaw 智能管理面板 — 单文件部署、跨平台、全功能可视化管理（含多智能体控制台）'
    : 'OpenClaw Management Panel — Single-file deploy, cross-platform, full-featured (incl. multi-agent)');
  const stepHint = t.onboarding?.stepHint ?? (locale === 'zh-CN'
    ? '首次使用？跟随引导快速了解 ClawPanel 的核心功能。'
    : 'First time? Follow the guide to quickly learn ClawPanel.');
  const prevLabel = t.onboarding?.prevStep ?? (locale === 'zh-CN' ? '上一步' : 'Previous');
  const installTitle = locale === 'zh-CN' ? '更新与入口说明' : 'Update and Entry Guide';
  const installHint = t.onboarding?.installHint ?? (locale === 'zh-CN'
    ? '安装完成后访问 http://localhost:19527，默认密码 clawpanel。'
    : 'After install, open http://localhost:19527 and use the default password clawpanel.');

  const coreNavTitle = t.onboarding?.coreNavTitle ?? (locale === 'zh-CN' ? '核心功能导航' : 'Core Navigation');
  const coreNavProgress = t.onboarding?.coreNavProgress ?? (locale === 'zh-CN' ? '高亮指引' : 'Spotlight guide');

  const tags = [
    t.onboarding?.tagSingleBinary ?? (locale === 'zh-CN' ? '单二进制部署' : 'Single Binary'),
    t.onboarding?.tagChannels ?? (locale === 'zh-CN' ? '20+ 通道' : '20+ Channels'),
    t.onboarding?.tagAgents ?? (locale === 'zh-CN' ? '多智能体' : 'Multi-Agent'),
    t.onboarding?.tagWorkflow ?? 'Workflow Center',
  ];

  const welcomeHighlights = [
    {
      title: t.onboarding?.tagSingleBinary ?? (locale === 'zh-CN' ? '单二进制部署' : 'Single Binary'),
      desc: locale === 'zh-CN' ? '一条命令完成安装与启动' : 'Install and launch with one command',
      route: locale === 'zh-CN' ? '对应内容: 快速安装' : 'Maps to: Quick Install',
    },
    {
      title: t.onboarding?.tagChannels ?? (locale === 'zh-CN' ? '20+ 通道' : '20+ Channels'),
      desc: locale === 'zh-CN' ? '统一管理 QQ/微信等通道' : 'Manage QQ, WeChat, and more in one place',
      route: locale === 'zh-CN' ? '对应入口: 通道管理' : 'Entry: Channels',
    },
    {
      title: t.onboarding?.tagAgents ?? (locale === 'zh-CN' ? '多智能体' : 'Multi-Agent'),
      desc: locale === 'zh-CN' ? '按角色拆分智能体能力' : 'Split capabilities across specialized agents',
      route: locale === 'zh-CN' ? '对应入口: 智能体' : 'Entry: Agents',
    },
    {
      title: t.onboarding?.tagWorkflow ?? 'Workflow Center',
      desc: locale === 'zh-CN' ? '自动化复杂任务执行流程' : 'Automate multi-step task execution',
      route: locale === 'zh-CN' ? '对应入口: 工作流中心' : 'Entry: Workflows',
    },
  ];

  if (step === 3) {
    const current = navTargets[navIndex] ?? navTargets[0];
    const pad = 10;
    const rect = spot
      ? {
          top: Math.max(8, spot.top - pad),
          left: Math.max(8, spot.left - pad),
          width: spot.width + pad * 2,
          height: spot.height + pad * 2,
        }
      : null;

    const bubble = (() => {
      if (!rect) {
        const bw = 360;
        const bh = 140;
        return {
          top: Math.max(16, Math.round((window.innerHeight - bh) / 2)),
          left: Math.max(16, Math.round((window.innerWidth - bw) / 2)),
          placement: 'center' as const,
        };
      }
      const bw = 320;
      const bh = 120;
      const gap = 16;
      const margin = 16;

      const right = rect.left + rect.width + gap;
      const left = rect.left - bw - gap;
      const canRight = right + bw <= window.innerWidth - margin;
      const canLeft = left >= margin;

      const top = Math.min(
        window.innerHeight - margin - bh,
        Math.max(margin, rect.top),
      );

      if (canRight) return { top, left: right, placement: 'right' as const };
      if (canLeft) return { top, left, placement: 'left' as const };
      return { top: Math.min(window.innerHeight - margin - bh, rect.top + rect.height + gap), left: margin, placement: 'below' as const };
    })();

    return (
      <div
        ref={dialogRef}
        className="fixed inset-0 z-[200] flex items-center justify-center bg-slate-900/30 backdrop-blur-sm p-4 sm:p-6"
        role="dialog"
        aria-modal="true"
        aria-label={coreNavTitle}
      >
        {/* Always-visible close button */}
        <button
          onClick={handleSkip}
          className="fixed top-4 right-4 z-[210] rounded-2xl border border-white/40 dark:border-slate-700/50 bg-white/70 dark:bg-slate-900/60 p-2.5 text-slate-600 dark:text-slate-200 shadow-[0_12px_30px_rgba(15,23,42,0.18)] backdrop-blur-xl hover:bg-white/85 dark:hover:bg-slate-900/75 transition-colors"
          aria-label={skipLabel}
          title={skipLabel}
        >
          <X size={18} />
        </button>

        {rect && (
          <div
            className="fixed rounded-[18px] pointer-events-none"
            style={{
              top: rect.top,
              left: rect.left,
              width: rect.width,
              height: rect.height,
              boxShadow: '0 0 0 9999px rgba(15,23,42,0.38)',
              border: '2px solid rgba(96,165,250,0.95)',
              background: 'rgba(255,255,255,0.03)',
            }}
          />
        )}

        {/* Tooltip bubble */}
        <div
          className="fixed w-[360px] max-w-[calc(100vw-32px)] rounded-2xl border border-white/60 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.92),rgba(239,246,255,0.86))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.92),rgba(30,64,175,0.18))] p-4 shadow-[0_18px_44px_rgba(15,23,42,0.16)] backdrop-blur-xl"
          style={{ top: bubble.top, left: bubble.left }}
        >
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <div className="text-sm font-bold text-slate-900 dark:text-white">
                {current.title}
                <span className="ml-2 text-xs font-medium text-slate-400 dark:text-slate-500">
                  {coreNavProgress} {navIndex + 1}/{navTargets.length}
                </span>
              </div>
              <div className="mt-1 text-sm text-slate-600 dark:text-slate-300 leading-relaxed">
                {current.desc}
              </div>
            </div>
            {/* Close button moved to fixed top-right for visibility */}
          </div>
          <div className="mt-4 flex items-center justify-between gap-3">
            <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-slate-200/80 dark:bg-slate-700/80">
              <div
                className="h-full rounded-full bg-[#165DFF] transition-all duration-300"
                style={{ width: `${((navIndex + 1) / navTargets.length) * 100}%` }}
              />
            </div>
            <button
              type="button"
              onClick={handleStart}
              className="shrink-0 rounded-xl bg-[#165DFF] px-4 py-2 text-sm font-semibold text-white shadow-lg shadow-blue-500/30 hover:bg-[#0d4ad9] transition-colors"
            >
              {locale === 'zh-CN' ? '我知道了' : 'Got it'}
            </button>
          </div>
        </div>

        {/* Bottom controls */}
        <div className="fixed inset-x-0 bottom-6 flex justify-center px-4">
          <div className="w-full max-w-[560px] rounded-2xl border border-white/60 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.88),rgba(239,246,255,0.76))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.9),rgba(30,64,175,0.14))] px-4 py-3 shadow-[0_16px_40px_rgba(15,23,42,0.14)] backdrop-blur-xl">
            <div className="flex items-center justify-between gap-3">
              <div className="min-w-0">
                <div className="text-sm font-semibold text-slate-900 dark:text-white truncate">{coreNavTitle}</div>
                <div className="text-xs text-slate-500 dark:text-slate-400">{navIndex + 1}/{navTargets.length}</div>
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={handlePrev}
                  className="flex items-center gap-2 rounded-xl border border-slate-200 dark:border-slate-600 px-4 py-2 text-sm font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800/60 transition-colors"
                >
                  <ChevronLeft size={16} />
                  {prevLabel}
                </button>
                <button
                  onClick={handleStart}
                  className="rounded-xl bg-[#165DFF] px-5 py-2 text-sm font-semibold text-white shadow-lg shadow-blue-500/30 hover:bg-[#0d4ad9] transition-colors"
                >
                  {locale === 'zh-CN' ? '下一步' : 'Next'}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div
      ref={dialogRef}
      className="fixed inset-0 z-[200] flex items-center justify-center bg-slate-900/30 backdrop-blur-sm p-4 sm:p-6"
      role="dialog"
      aria-modal="true"
      aria-labelledby="onboarding-title"
    >
      <div
        className="relative w-full max-w-[560px] rounded-[20px] border border-white/60 dark:border-slate-700/50 bg-[linear-gradient(145deg,rgba(255,255,255,0.94),rgba(239,246,255,0.88))] dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.94),rgba(30,64,175,0.18))] p-5 sm:p-6 shadow-[0_16px_40px_rgba(15,23,42,0.14)] backdrop-blur-xl"
        onClick={e => e.stopPropagation()}
      >
        {/* Progress */}
        <div className="mb-6 flex items-center justify-between">
          <span className="text-xs font-medium text-slate-500 dark:text-slate-400">
            {step}/{TOTAL_STEPS}
          </span>
          <div className="h-1.5 flex-1 mx-4 max-w-[120px] rounded-full bg-slate-200 dark:bg-slate-700 overflow-hidden">
            <div
              className="h-full rounded-full bg-[#165DFF] transition-all duration-300"
              style={{ width: `${(step / TOTAL_STEPS) * 100}%` }}
            />
          </div>
        </div>

        {step === 1 && (
          <div className="flex flex-col items-center gap-5 sm:gap-6">
            <img
              src="/logo.jpg"
              alt="ClawPanel"
              className="h-16 w-16 sm:h-20 sm:w-20 rounded-2xl object-cover shadow-lg shrink-0"
            />
            <h2
              id="onboarding-title"
              className="text-lg sm:text-xl font-bold tracking-tight text-slate-900 dark:text-white text-center"
            >
              {title}
            </h2>
            <p className="text-center text-sm sm:text-base text-slate-600 dark:text-slate-300 leading-relaxed max-w-[520px]">
              {slogan}
            </p>
            <div className="flex flex-wrap justify-center gap-2">
              {tags.map((tag) => (
                <span
                  key={tag}
                  className="inline-flex items-center rounded-xl border border-blue-200/80 dark:border-blue-800/50 bg-blue-50/80 dark:bg-blue-900/25 px-3 py-1.5 text-xs font-medium text-blue-700 dark:text-blue-300"
                >
                  {tag}
                </span>
              ))}
            </div>
            <div className="grid w-full gap-3 sm:grid-cols-2">
              {welcomeHighlights.map((item) => (
                <div
                  key={item.title}
                  className="rounded-2xl border border-white/60 dark:border-slate-700/50 bg-white/60 dark:bg-slate-900/35 px-4 py-3 text-left shadow-[0_10px_24px_rgba(15,23,42,0.06)] backdrop-blur-xl"
                >
                  <div className="text-sm font-semibold text-slate-900 dark:text-white">{item.title}</div>
                  <p className="mt-1 text-xs leading-5 text-slate-600 dark:text-slate-300">{item.desc}</p>
                  <div className="mt-2 inline-flex items-center gap-1 text-[11px] font-medium text-blue-600 dark:text-blue-300">
                    <ArrowRight size={12} />
                    <span>{item.route}</span>
                  </div>
                </div>
              ))}
            </div>
            <p className="text-center text-sm text-slate-500 dark:text-slate-400">
              {stepHint}
            </p>
          </div>
        )}

        {step === 2 && (
          <div className="flex flex-col gap-4">
            <h3 className="text-base font-semibold text-slate-900 dark:text-white">
              {installTitle}
            </h3>
            <div className="space-y-4 text-sm leading-relaxed text-slate-600 dark:text-slate-300">
              <div className="grid gap-3 sm:grid-cols-2">
                <div className="rounded-2xl border border-white/60 dark:border-slate-700/50 bg-white/60 dark:bg-slate-900/35 px-4 py-4 shadow-[0_10px_24px_rgba(15,23,42,0.06)] backdrop-blur-xl">
                  <div className="text-sm font-semibold text-slate-900 dark:text-white">
                    {locale === 'zh-CN' ? '更新入口在哪' : 'Where to update'}
                  </div>
                  <p className="mt-2 text-sm leading-6 text-slate-600 dark:text-slate-300">
                    {locale === 'zh-CN'
                      ? '进入左侧导航的「配置中心」，在系统管理相关区域查看版本与更新入口。'
                      : 'Open the left navigation and go to “Config” to find version and update actions in system management.'}
                  </p>
                  <div className="mt-3 inline-flex items-center gap-1 text-[11px] font-medium text-blue-600 dark:text-blue-300">
                    <ArrowRight size={12} />
                    <span>{locale === 'zh-CN' ? '路径：配置中心 -> 系统管理 / 更新入口' : 'Path: Config -> System Management / Update Entry'}</span>
                  </div>
                </div>
                <div className="rounded-2xl border border-white/60 dark:border-slate-700/50 bg-white/60 dark:bg-slate-900/35 px-4 py-4 shadow-[0_10px_24px_rgba(15,23,42,0.06)] backdrop-blur-xl">
                  <div className="text-sm font-semibold text-slate-900 dark:text-white">
                    {locale === 'zh-CN' ? '怎么更新' : 'How to update'}
                  </div>
                  <ul className="mt-2 list-disc space-y-1 pl-5 text-sm leading-6 text-slate-600 dark:text-slate-300">
                    <li>{locale === 'zh-CN' ? '先查看当前版本和可用新版本。' : 'Check the current version and available release first.'}</li>
                    <li>{locale === 'zh-CN' ? '点击一键更新或前往更新页执行升级。' : 'Use one-click update or jump to the update page to upgrade.'}</li>
                    <li>{locale === 'zh-CN' ? '更新后返回仪表盘确认服务状态恢复正常。' : 'Return to the dashboard and confirm services are healthy after the upgrade.'}</li>
                  </ul>
                </div>
              </div>
              <div className="rounded-2xl border border-blue-200/80 dark:border-blue-800/50 bg-blue-50/80 dark:bg-blue-900/20 px-4 py-3 text-sm text-slate-700 dark:text-slate-200">
                <span className="font-medium text-slate-900 dark:text-white">
                  {locale === 'zh-CN' ? '访问入口' : 'Access URL'}:
                </span>{' '}
                <code className="rounded bg-white/70 dark:bg-slate-800/80 px-1.5 py-0.5 text-[13px] text-blue-700 dark:text-blue-300">
                  http://localhost:19527
                </code>
                {locale === 'zh-CN' ? '，默认密码 ' : ', default password '}
                <code className="rounded bg-white/70 dark:bg-slate-800/80 px-1.5 py-0.5 text-[13px] text-blue-700 dark:text-blue-300">
                  clawpanel
                </code>
              </div>
              <p className="text-sm text-slate-600 dark:text-slate-400 leading-relaxed break-words">
                {installHint}
              </p>
            </div>
          </div>
        )}

        {step === 4 && (
          <div className="flex flex-col gap-4">
            <div>
              <h3 className="text-base font-semibold text-slate-900 dark:text-white">
                {locale === 'zh-CN' ? '常见问题速览' : 'FAQ Quick View'}
              </h3>
              <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
                {locale === 'zh-CN' ? '先看 5 条最常见问题，遇到相同情况时能更快回到文档定位。' : 'Scan the top 5 common issues before diving into the full docs.'}
              </p>
            </div>
            <div className="space-y-3">
              {faqItems.map((item, index) => {
                const expanded = faqExpanded.includes(index);
                const longAnswer = item.answer.length > 36;
                return (
                  <button
                    key={item.question}
                    type="button"
                    onClick={() => longAnswer && toggleFaqItem(index)}
                    className="w-full rounded-2xl border border-white/60 dark:border-slate-700/50 bg-white/60 dark:bg-slate-900/35 px-4 py-3 text-left shadow-[0_10px_24px_rgba(15,23,42,0.06)] backdrop-blur-xl"
                  >
                    <div className="text-sm font-semibold text-slate-900 dark:text-white">{item.question}</div>
                    <p className={`mt-1 text-sm leading-6 text-slate-600 dark:text-slate-300 ${!expanded ? 'line-clamp-1' : ''}`}>
                      {item.answer}
                    </p>
                    {longAnswer && (
                      <span className="mt-2 inline-flex text-xs font-medium text-blue-600 dark:text-blue-300">
                        {expanded ? (locale === 'zh-CN' ? '收起' : 'Collapse') : (locale === 'zh-CN' ? '展开' : 'Expand')}
                      </span>
                    )}
                  </button>
                );
              })}
            </div>
            <button
              type="button"
              onClick={handleViewFullFaq}
              className="inline-flex items-center justify-center rounded-xl border border-blue-200/80 dark:border-blue-800/50 bg-blue-50/80 dark:bg-blue-900/20 px-4 py-3 text-sm font-medium text-blue-700 dark:text-blue-300 hover:bg-blue-100/80 dark:hover:bg-blue-900/35 transition-colors"
            >
              {locale === 'zh-CN' ? '查看完整 FAQ' : 'View Full FAQ'}
            </button>
          </div>
        )}

        {step === 5 && (
          <div className="flex flex-col items-center gap-5 text-center">
            <div className="flex h-16 w-16 items-center justify-center rounded-full bg-blue-100 text-[#165DFF] shadow-[0_12px_30px_rgba(59,130,246,0.18)] dark:bg-blue-500/15 dark:text-blue-300">
              <svg viewBox="0 0 24 24" className="h-8 w-8" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <path d="M20 6 9 17l-5-5" />
              </svg>
            </div>
            <div className="space-y-2">
              <h3 className="text-lg font-bold tracking-tight text-slate-900 dark:text-white">
                {locale === 'zh-CN' ? '引导完成，开始使用 ClawPanel 吧' : 'Guide complete, start using ClawPanel'}
              </h3>
              <p className="max-w-[520px] text-sm leading-6 text-slate-600 dark:text-slate-300">
                {locale === 'zh-CN'
                  ? '你已经了解了安装方式和核心功能，后续遇到问题也可以随时打开帮助文档和 FAQ。'
                  : 'You now know the install flow and core entry points, and you can reopen the docs or FAQ anytime.'}
              </p>
            </div>
            <div className="grid w-full gap-3 sm:grid-cols-2">
              <button
                type="button"
                onClick={handleFinishAndOpenDocs}
                className="inline-flex items-center justify-center rounded-xl border border-slate-200 dark:border-slate-600 px-4 py-3 text-sm font-medium text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-800/70 transition-colors"
              >
                {locale === 'zh-CN' ? '查看完整文档' : 'View Full Docs'}
              </button>
              <button
                type="button"
                onClick={handleFinishAndClose}
                className="inline-flex items-center justify-center rounded-xl bg-[#165DFF] px-4 py-3 text-sm font-semibold text-white shadow-lg shadow-blue-500/30 hover:bg-[#0d4ad9] transition-colors"
              >
                {locale === 'zh-CN' ? '开始使用' : 'Get Started'}
              </button>
            </div>
          </div>
        )}

        {step !== 5 && (
          <div className="mt-8 flex w-full flex-col gap-3 sm:flex-row sm:justify-between sm:items-center">
          <div className="flex gap-2 order-2 sm:order-1">
            {step > 1 && (
              <button
                onClick={handlePrev}
                className="flex items-center justify-center gap-2 rounded-xl border border-slate-200 dark:border-slate-600 px-4 py-2.5 text-sm font-medium text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800/70 transition-colors"
              >
                <ChevronLeft size={16} />
                {prevLabel}
              </button>
            )}
            <button
              onClick={handleSkip}
              className="flex items-center justify-center gap-2 rounded-xl border border-slate-200 dark:border-slate-600 px-4 py-2.5 text-sm font-medium text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800/70 transition-colors"
            >
              <X size={16} />
              {skipLabel}
            </button>
          </div>
          <button
            onClick={handleStart}
            className="order-1 sm:order-2 rounded-xl bg-[#165DFF] px-6 py-2.5 text-sm font-semibold text-white shadow-lg shadow-blue-500/30 hover:bg-[#0d4ad9] transition-colors w-full sm:w-auto"
          >
            {step === 1 ? startLabel : (step === TOTAL_STEPS ? (locale === 'zh-CN' ? '开始使用' : 'Get Started') : (locale === 'zh-CN' ? '下一步' : 'Next'))}
          </button>
          </div>
        )}
      </div>
    </div>
  );
}

import { useEffect, useState } from 'react';
import { Lock, Sparkles } from 'lucide-react';
import { useI18n } from '../i18n';

export default function Login({ onLogin }: { onLogin: (pw: string) => Promise<boolean> }) {
  const { t } = useI18n();
  const isDemo = import.meta.env.VITE_DEMO === 'true';
  const [pw, setPw] = useState('');
  const [err, setErr] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    document.body.dataset.uiMode = 'modern';
    return () => {
      delete document.body.dataset.uiMode;
    };
  }, []);

  const submit = async () => {
    if (loading || !pw) return;
    setLoading(true);
    setErr('');
    const ok = await onLogin(pw);
    if (!ok) setErr(t.login.wrongPassword);
    setLoading(false);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    void submit();
  };

  return (
    <div className="min-h-screen overflow-hidden bg-transparent p-4 sm:p-6">
      <div className="relative mx-auto flex min-h-[calc(100vh-2rem)] w-full max-w-6xl items-center justify-center">
        <div className="pointer-events-none absolute inset-0 overflow-hidden">
          <div className="absolute left-[10%] top-[12%] h-40 w-40 rounded-full bg-blue-400/15 blur-3xl" />
          <div className="absolute right-[8%] top-[18%] h-52 w-52 rounded-full bg-cyan-300/14 blur-3xl" />
          <div className="absolute bottom-[10%] left-[18%] h-44 w-44 rounded-full bg-indigo-400/12 blur-3xl" />
          <div className="absolute left-1/2 top-[28%] h-64 w-64 -translate-x-1/2 rounded-full bg-white/35 blur-[110px]" />
        </div>

        <div className="relative z-10 w-full max-w-[30rem]">
          <div className="mb-6 flex flex-col items-center text-center">
            <div className="relative mb-3">
              <div className="absolute inset-x-12 bottom-3 top-8 rounded-full bg-blue-500/10 blur-3xl" />
              <img src="/logo.png" alt="ClawPanel" className="relative w-48 max-w-full h-auto object-contain drop-shadow-[0_16px_26px_rgba(59,130,246,0.14)] sm:w-56" />
            </div>
            <div className="inline-flex items-center gap-2 rounded-full border border-white/70 bg-white/45 px-3 py-1 text-[11px] font-medium text-slate-500 shadow-[0_10px_24px_rgba(148,163,184,0.12)] backdrop-blur-2xl dark:border-slate-700/60 dark:bg-slate-900/40 dark:text-slate-300">
              <Sparkles size={12} className="text-blue-500" />
              <span>{t.login.subtitle}</span>
            </div>
          </div>

          <form onSubmit={handleSubmit} className="relative overflow-hidden rounded-[34px] border border-white/60 bg-[linear-gradient(150deg,rgba(255,255,255,0.5),rgba(255,255,255,0.2))] p-6 shadow-[0_28px_70px_rgba(15,23,42,0.12)] backdrop-blur-[28px] dark:border-slate-700/50 dark:bg-[linear-gradient(150deg,rgba(15,23,42,0.75),rgba(30,41,59,0.42))] sm:p-7">
            <div className="pointer-events-none absolute inset-0">
              <div className="absolute inset-x-0 top-0 h-20 bg-[linear-gradient(180deg,rgba(255,255,255,0.62),rgba(255,255,255,0))]" />
              <div className="absolute -left-10 top-10 h-32 w-32 rounded-full bg-cyan-200/30 blur-3xl" />
              <div className="absolute -right-6 bottom-6 h-28 w-28 rounded-full bg-blue-300/24 blur-3xl" />
              <div className="absolute inset-[1px] rounded-[33px] border border-white/45" />
            </div>

            <div className="relative space-y-5">
              <div className="space-y-2">
                <label className="ml-1 text-xs font-semibold tracking-wide text-slate-700 dark:text-slate-200">{t.login.passwordLabel}</label>
                <div className="rounded-[24px] border border-white/70 bg-[linear-gradient(145deg,rgba(255,255,255,0.58),rgba(255,255,255,0.26))] p-1 shadow-[inset_0_1px_0_rgba(255,255,255,0.8),0_14px_32px_rgba(148,163,184,0.08)] backdrop-blur-2xl dark:border-slate-700/60 dark:bg-[linear-gradient(145deg,rgba(15,23,42,0.55),rgba(30,41,59,0.35))]">
                  <div className="relative">
                    <div className="absolute left-4 top-1/2 -translate-y-1/2 text-slate-400">
                      <Lock size={16} />
                    </div>
                    <input
                      type="password"
                      value={pw}
                      onChange={e => setPw(e.target.value)}
                      onKeyDown={e => {
                        if (e.key === 'Enter') {
                          e.preventDefault();
                          void submit();
                        }
                      }}
                      placeholder={t.login.passwordPlaceholder}
                      className="w-full rounded-[20px] border-0 bg-transparent px-11 py-3.5 text-sm text-slate-800 outline-none ring-0 placeholder:text-slate-400 focus:outline-none focus:ring-0 dark:text-slate-100 dark:placeholder:text-slate-500"
                      autoComplete="current-password"
                      autoFocus
                    />
                  </div>
                </div>
                {isDemo && (
                  <p className="ml-1 text-xs font-medium text-blue-700 dark:text-blue-300">
                    {t.login.demoPasswordHint}
                  </p>
                )}
              </div>

              {err && (
                <div className="rounded-2xl border border-red-100/80 bg-red-50/70 px-4 py-3 text-center text-xs font-medium text-red-600 backdrop-blur-2xl dark:border-red-900/40 dark:bg-red-950/30 dark:text-red-300">
                  {err}
                </div>
              )}

              <button
                type="submit"
                disabled={loading || !pw}
                className="w-full rounded-[22px] border border-white/45 bg-[linear-gradient(135deg,rgba(59,130,246,0.9),rgba(37,99,235,0.82))] py-3.5 text-sm font-semibold text-white shadow-[0_18px_42px_rgba(37,99,235,0.26),inset_0_1px_0_rgba(255,255,255,0.28)] transition-all hover:-translate-y-0.5 hover:shadow-[0_22px_48px_rgba(37,99,235,0.32)] active:translate-y-0 disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:translate-y-0"
              >
                {loading ? t.login.loggingIn : t.login.loginButton}
              </button>
            </div>
          </form>

          <p className="mt-6 text-center text-[11px] text-slate-400 dark:text-slate-500">
            {t.login.poweredBy}
          </p>
        </div>
      </div>
    </div>
  );
}

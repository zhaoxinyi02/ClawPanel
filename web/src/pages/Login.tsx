import { useState } from 'react';
import { Lock } from 'lucide-react';
import { useI18n } from '../i18n';

export default function Login({ onLogin }: { onLogin: (pw: string) => Promise<boolean> }) {
  const { t } = useI18n();
  const [pw, setPw] = useState('');
  const [err, setErr] = useState('');
  const [loading, setLoading] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setErr('');
    const ok = await onLogin(pw);
    if (!ok) setErr(t.login.wrongPassword);
    setLoading(false);
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-950 p-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="flex flex-col items-center">
          <img src="/logo.jpg" alt="ClawPanel" className="w-16 h-16 rounded-2xl shadow-lg mb-4" />
          <h1 className="text-2xl font-bold tracking-tight text-gray-900 dark:text-white">ClawPanel</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">{t.login.subtitle}</p>
        </div>
        
        <form onSubmit={submit} className="bg-white dark:bg-gray-900 rounded-2xl shadow-xl border border-gray-100 dark:border-gray-800 p-8 space-y-6">
          <div className="space-y-4">
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-gray-700 dark:text-gray-300 ml-1">{t.login.passwordLabel}</label>
              <div className="relative">
                <div className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400">
                  <Lock size={16} />
                </div>
                <input 
                  type="password" 
                  value={pw} 
                  onChange={e => setPw(e.target.value)} 
                  placeholder={t.login.passwordPlaceholder} 
                  className="w-full pl-10 pr-4 py-2.5 rounded-xl border border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800 text-sm focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500 transition-all" 
                  autoFocus 
                />
              </div>
            </div>
            
            {err && (
              <div className="p-3 rounded-lg bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400 text-xs font-medium text-center">
                {err}
              </div>
            )}
            
            <button type="submit" disabled={loading || !pw} 
              className="w-full py-2.5 rounded-xl bg-violet-600 hover:bg-violet-700 text-white text-sm font-semibold shadow-lg shadow-violet-200 dark:shadow-none transition-all hover:scale-[1.02] active:scale-[0.98] disabled:opacity-50 disabled:hover:scale-100">
              {loading ? t.login.loggingIn : t.login.loginButton}
            </button>
          </div>
        </form>
        
        <p className="text-center text-[10px] text-gray-400">
          {t.login.poweredBy}
        </p>
      </div>
    </div>
  );
}

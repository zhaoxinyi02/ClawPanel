import { createContext, useContext, useState, useCallback, type ReactNode } from 'react';
import type { Locale, Translations } from './types';
import zhCN from './zh-CN';
import en from './en';

const translations: Record<Locale, Translations> = { 'zh-CN': zhCN, en };

interface I18nContextType {
  locale: Locale;
  t: Translations;
  setLocale: (locale: Locale) => void;
}

const I18nContext = createContext<I18nContextType>({
  locale: 'zh-CN',
  t: zhCN,
  setLocale: () => {},
});

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(() => {
    const saved = localStorage.getItem('clawpanel-locale');
    if (saved === 'en' || saved === 'zh-CN') return saved;
    // Auto-detect from browser
    const lang = navigator.language;
    if (lang.startsWith('zh')) return 'zh-CN';
    return 'en';
  });

  const setLocale = useCallback((l: Locale) => {
    localStorage.setItem('clawpanel-locale', l);
    setLocaleState(l);
  }, []);

  const t = translations[locale];

  return (
    <I18nContext.Provider value={{ locale, t, setLocale }}>
      {children}
    </I18nContext.Provider>
  );
}

export function useI18n() {
  return useContext(I18nContext);
}

export type { Locale, Translations };

import { ReactNode, useEffect, useState } from 'react';
import i18n from 'i18next';
import { initReactI18next, I18nextProvider } from 'react-i18next';
import { ru } from './ru';
import { en } from './en';

const DEFAULT = (import.meta.env.VITE_DEFAULT_LOCALE as string) || 'ru';

void i18n.use(initReactI18next).init({
  resources: { ru: { translation: ru }, en: { translation: en } },
  lng: localStorage.getItem('ar-drive.locale') || DEFAULT,
  fallbackLng: 'en',
  interpolation: { escapeValue: false },
});

export function I18nProvider({ children }: { children: ReactNode }) {
  const [ready, setReady] = useState(i18n.isInitialized);
  useEffect(() => {
    if (!ready) i18n.on('initialized', () => setReady(true));
    const onChange = (lng: string) => localStorage.setItem('ar-drive.locale', lng);
    i18n.on('languageChanged', onChange);
    return () => {
      i18n.off('languageChanged', onChange);
    };
  }, [ready]);
  return <I18nextProvider i18n={i18n}>{children}</I18nextProvider>;
}

import { Routes, Route, Navigate, NavLink } from 'react-router-dom';
import { useTheme } from './ThemeProvider';
import { useTranslation } from 'react-i18next';
import { SetupPage } from '@pages/Setup/SetupPage';
import { RegistriesPage } from '@pages/Registries/RegistriesPage';
import { FileBrowserPage } from '@pages/FileBrowser/FileBrowserPage';
import { SharesPage } from '@pages/Shares/SharesPage';
import { useCredentialsStore } from '@shared/store/credentials';

export function App() {
  const { theme, toggle } = useTheme();
  const { t, i18n } = useTranslation();
  const hasCreds = useCredentialsStore((s) => Boolean(s.creds));

  return (
    <div className="ar-layout">
      <header className="ar-header">
        <NavLink to="/" className="ar-header__brand">
          <b>AR</b> Drive
        </NavLink>
        <nav className="ar-header__controls" aria-label="primary">
          {hasCreds && (
            <>
              <NavLink to="/registries">{t('nav.registries')}</NavLink>
              <NavLink to="/shares">{t('nav.shares')}</NavLink>
            </>
          )}
          <button onClick={toggle} aria-label="toggle theme">
            {theme === 'dark' ? '☀︎' : '☾'}
          </button>
          <button
            onClick={() => i18n.changeLanguage(i18n.language === 'ru' ? 'en' : 'ru')}
            aria-label="toggle language"
          >
            {i18n.language === 'ru' ? 'EN' : 'RU'}
          </button>
        </nav>
      </header>

      <main className="ar-main">
        <Routes>
          <Route path="/" element={<Navigate to={hasCreds ? '/registries' : '/setup'} replace />} />
          <Route path="/setup" element={<SetupPage />} />
          <Route
            path="/registries"
            element={hasCreds ? <RegistriesPage /> : <Navigate to="/setup" replace />}
          />
          <Route
            path="/registries/:id/*"
            element={hasCreds ? <FileBrowserPage /> : <Navigate to="/setup" replace />}
          />
          <Route
            path="/shares"
            element={hasCreds ? <SharesPage /> : <Navigate to="/setup" replace />}
          />
          <Route path="*" element={<div className="ar-empty">404</div>} />
        </Routes>
      </main>
    </div>
  );
}

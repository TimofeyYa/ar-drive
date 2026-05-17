import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useCredentialsStore } from '@shared/store/credentials';
import { api, APIError } from '@shared/api/client';

export function SetupPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const setCreds = useCredentialsStore((s) => s.setCreds);

  const [clientId, setClientId] = useState('');
  const [clientSecret, setClientSecret] = useState('');
  const [projectId, setProjectId] = useState('');
  const [passphrase, setPassphrase] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showHelp, setShowHelp] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      // 1) Сначала сохраним креды локально, чтобы /verify смог их подхватить из заголовков.
      await setCreds({ clientId, clientSecret, projectId }, passphrase);
      // 2) Проверяем — backend идёт в IAM Cloud.ru.
      await api.verify({ client_id: clientId, client_secret: clientSecret, project_id: projectId });
      navigate('/registries', { replace: true });
    } catch (err) {
      if (err instanceof APIError) {
        setError(err.message);
      } else {
        setError(String(err));
      }
      // Очищаем неудачные креды.
      useCredentialsStore.getState().clear();
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="ar-card" style={{ maxWidth: 520, margin: '40px auto' }}>
      <h1 style={{ marginTop: 0 }}>{t('setup.title')}</h1>
      <p style={{ opacity: 0.8 }}>{t('setup.intro')}</p>
      <form onSubmit={onSubmit}>
        <Field label={t('setup.clientId')} value={clientId} onChange={setClientId} autoFocus />
        <Field label={t('setup.clientSecret')} value={clientSecret} onChange={setClientSecret} type="password" />
        <Field label={t('setup.projectId')} value={projectId} onChange={setProjectId} placeholder="00000000-0000-0000-0000-000000000000" />
        <Field label={t('setup.passphrase')} value={passphrase} onChange={setPassphrase} type="password" />
        {error && (
          <div role="alert" style={{ color: 'crimson', marginTop: 12 }}>
            {t('setup.error')}: {error}
          </div>
        )}
        <div style={{ display: 'flex', gap: 8, marginTop: 16, alignItems: 'center' }}>
          <button type="submit" disabled={busy || !clientId || !clientSecret || !projectId}>
            {busy ? '…' : t('setup.verify')}
          </button>
          <button type="button" onClick={() => setShowHelp((v) => !v)}>
            {t('setup.whereKeys')}
          </button>
        </div>
        {showHelp && (
          <div className="ar-card" style={{ marginTop: 16, fontSize: 14 }}>
            <ol>
              <li>
                Зайдите в личный кабинет Cloud.ru → «Сервисные аккаунты» (
                <a
                  href="https://cloud.ru/docs/console_api/ug/topics/quickstart"
                  target="_blank"
                  rel="noreferrer"
                >
                  быстрый старт
                </a>
                ).
              </li>
              <li>Создайте сервисный аккаунт и сгенерируйте Key ID + Key Secret.</li>
              <li>
                Назначьте сервисному аккаунту роль{' '}
                <code>artifact-registry.editor</code> в нужном проекте.
              </li>
              <li>Скопируйте UUID проекта со страницы «Проекты».</li>
            </ol>
          </div>
        )}
      </form>
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
  type = 'text',
  autoFocus,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  type?: string;
  autoFocus?: boolean;
  placeholder?: string;
}) {
  return (
    <label style={{ display: 'block', marginTop: 12 }}>
      <span style={{ display: 'block', fontSize: 13, marginBottom: 4 }}>{label}</span>
      <input
        type={type}
        value={value}
        autoFocus={autoFocus}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        style={{
          width: '100%',
          padding: '10px 12px',
          borderRadius: 8,
          border: '1px solid var(--ar-border)',
          background: 'transparent',
          color: 'var(--ar-fg)',
          fontFamily: 'var(--ar-mono-font)',
          fontSize: 14,
        }}
      />
    </label>
  );
}

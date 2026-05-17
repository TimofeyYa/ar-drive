import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { api, Registry } from '@shared/api/client';

export function RegistriesPage() {
  const { t } = useTranslation();
  const { data, isLoading, error } = useQuery({
    queryKey: ['registries'],
    queryFn: () => api.listRegistries(),
  });

  if (isLoading) return <div className="ar-empty">Loading…</div>;
  if (error) return <div className="ar-empty">Error: {String(error)}</div>;

  const generic = (data?.items ?? []).filter((r) => r.registryType === 'GENERIC');
  const others = (data?.items ?? []).filter((r) => r.registryType !== 'GENERIC');

  return (
    <div>
      <h1>{t('registries.title')}</h1>
      <p style={{ opacity: 0.7 }}>{t('registries.onlyGeneric')}</p>
      {generic.length === 0 && <div className="ar-empty">{t('registries.empty')}</div>}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: 16 }}>
        {generic.map((r) => (
          <RegistryCard key={r.id} r={r} t={t} />
        ))}
      </div>
      {others.length > 0 && (
        <details style={{ marginTop: 32 }}>
          <summary>Прочие реестры (только просмотр)</summary>
          <ul>
            {others.map((r) => (
              <li key={r.id}>
                <code>{r.name}</code> — {r.registryType}
              </li>
            ))}
          </ul>
        </details>
      )}
    </div>
  );
}

function RegistryCard({ r, t }: { r: Registry; t: (k: string) => string }) {
  const statusColor = r.status === 'ACTIVE' ? 'green' : r.status === 'ERROR' ? 'crimson' : 'goldenrod';
  return (
    <Link to={`/registries/${r.id}`} className="ar-card" style={{ textDecoration: 'none', color: 'inherit' }}>
      <div className="ar-truncate" style={{ fontWeight: 600 }}>
        {r.name}
      </div>
      <div style={{ display: 'flex', gap: 8, marginTop: 8, fontSize: 13 }}>
        <span style={{ color: statusColor }}>● {r.status}</span>
        <span style={{ opacity: 0.6 }}>{r.tariff}</span>
        {r.isPublic && <span style={{ opacity: 0.6 }}>public</span>}
      </div>
      <div style={{ marginTop: 12, fontSize: 12, opacity: 0.6 }}>
        {t('registries.columns.updated')}: {new Date(r.updatedAt).toLocaleString()}
      </div>
    </Link>
  );
}

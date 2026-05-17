import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { api } from '@shared/api/client';

export function SharesPage() {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({ queryKey: ['shares'], queryFn: () => api.listShares() });
  const revoke = useMutation({
    mutationFn: (id: string) => api.revokeShare(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['shares'] }),
  });

  if (isLoading) return <div className="ar-empty">…</div>;
  const items = data?.items ?? [];
  if (items.length === 0) return <div className="ar-empty">{t('shares.empty')}</div>;

  return (
    <div>
      <h1>{t('shares.title')}</h1>
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr style={{ textAlign: 'left', opacity: 0.7, fontSize: 12 }}>
            <th style={{ padding: 8 }}>Файл</th>
            <th style={{ padding: 8 }}>URL</th>
            <th style={{ padding: 8 }}>{t('shares.expires')}</th>
            <th style={{ padding: 8 }}>{t('shares.downloads')}</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {items.map((s) => (
            <tr key={s.short_id} style={{ borderTop: '1px solid var(--ar-border)' }}>
              <td className="ar-truncate" style={{ padding: 8, maxWidth: 300 }}>{s.file_path}</td>
              <td className="ar-mono" style={{ padding: 8 }}>
                <a href={s.url} target="_blank" rel="noreferrer">
                  {s.url}
                </a>
              </td>
              <td style={{ padding: 8 }}>{s.expires_at ? new Date(s.expires_at).toLocaleString() : '∞'}</td>
              <td style={{ padding: 8 }}>
                {s.downloads}
                {s.max_downloads ? ` / ${s.max_downloads}` : ''}
              </td>
              <td style={{ padding: 8, textAlign: 'right' }}>
                {s.revoked ? (
                  <span style={{ opacity: 0.6 }}>{t('shares.revoked')}</span>
                ) : (
                  <button onClick={() => revoke.mutate(s.short_id)}>{t('shares.revoke')}</button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

import { useState } from 'react';
import { api, FileEntry } from '@shared/api/client';

type Props = {
  file: FileEntry;
  registryID: string;
  onClose: () => void;
};

const TTL_OPTIONS = [
  { label: '1 час', sec: 3600 },
  { label: '1 день', sec: 86400 },
  { label: '7 дней', sec: 7 * 86400 },
  { label: '30 дней', sec: 30 * 86400 },
  { label: 'Без срока', sec: 0 },
];

export function ShareLinkModal({ file, registryID, onClose }: Props) {
  const [ttl, setTtl] = useState(7 * 86400);
  const [password, setPassword] = useState('');
  const [maxDownloads, setMaxDownloads] = useState<number>(0);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<string | null>(null);
  const [err, setErr] = useState<string | null>(null);

  async function create() {
    setBusy(true);
    setErr(null);
    try {
      const r = await api.createShare({
        registry_id: registryID,
        file_path: file.path,
        password: password || undefined,
        ttl_seconds: ttl || undefined,
        max_downloads: maxDownloads || undefined,
      });
      setResult(r.url);
      try {
        await navigator.clipboard.writeText(r.url);
      } catch {
        /* ignore */
      }
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.4)',
        display: 'grid',
        placeItems: 'center',
        zIndex: 60,
      }}
      onClick={onClose}
    >
      <div className="ar-card" style={{ width: 460 }} onClick={(e) => e.stopPropagation()}>
        <h2 style={{ marginTop: 0 }}>Ссылка на «{file.name || file.path}»</h2>
        <label style={{ display: 'block', marginTop: 8 }}>
          Срок жизни
          <select value={ttl} onChange={(e) => setTtl(Number(e.target.value))} style={{ width: '100%' }}>
            {TTL_OPTIONS.map((o) => (
              <option key={o.sec} value={o.sec}>
                {o.label}
              </option>
            ))}
          </select>
        </label>
        <label style={{ display: 'block', marginTop: 8 }}>
          Пароль (опционально)
          <input value={password} onChange={(e) => setPassword(e.target.value)} type="password" style={{ width: '100%' }} />
        </label>
        <label style={{ display: 'block', marginTop: 8 }}>
          Лимит скачиваний (0 = без лимита)
          <input
            value={maxDownloads}
            onChange={(e) => setMaxDownloads(Number(e.target.value) || 0)}
            type="number"
            min={0}
            style={{ width: '100%' }}
          />
        </label>
        {result && (
          <div style={{ marginTop: 12 }}>
            <strong>Готово, ссылка скопирована:</strong>
            <div className="ar-mono ar-truncate" style={{ marginTop: 4 }}>
              <a href={result} target="_blank" rel="noreferrer">
                {result}
              </a>
            </div>
          </div>
        )}
        {err && <div role="alert" style={{ color: 'crimson', marginTop: 8 }}>{err}</div>}
        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 16 }}>
          <button onClick={onClose}>Закрыть</button>
          <button onClick={create} disabled={busy}>
            {busy ? '…' : 'Создать ссылку'}
          </button>
        </div>
      </div>
    </div>
  );
}

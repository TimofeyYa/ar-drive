import { useEffect, useMemo, useState } from 'react';
import { api, FileEntry } from '@shared/api/client';

type Props = {
  file: FileEntry;
  registryID: string;
  onClose: () => void;
};

const TEXT_LIMIT = 5 * 1024 * 1024;

function extOf(name: string): string {
  const i = name.lastIndexOf('.');
  return i >= 0 ? name.slice(i + 1).toLowerCase() : '';
}

function kindOf(name: string): 'image' | 'video' | 'audio' | 'text' | 'json' | 'markdown' | 'pdf' | 'binary' {
  const ext = extOf(name);
  if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'avif'].includes(ext)) return 'image';
  if (['mp4', 'webm', 'mov'].includes(ext)) return 'video';
  if (['mp3', 'wav', 'ogg'].includes(ext)) return 'audio';
  if (['json'].includes(ext)) return 'json';
  if (['md', 'markdown'].includes(ext)) return 'markdown';
  if (ext === 'pdf') return 'pdf';
  if (
    [
      'txt', 'log', 'yml', 'yaml', 'toml', 'ini', 'env', 'conf', 'js', 'ts', 'jsx', 'tsx',
      'go', 'py', 'rb', 'rs', 'java', 'cpp', 'cs', 'php', 'sh', 'sql', 'dockerfile', 'html', 'css',
    ].includes(ext)
  ) {
    return 'text';
  }
  return 'binary';
}

export function PreviewDrawer({ file, registryID, onClose }: Props) {
  const kind = useMemo(() => kindOf(file.name || file.path), [file]);
  const url = api.downloadURL(registryID, file.path);

  const [text, setText] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if ((kind === 'text' || kind === 'json' || kind === 'markdown') && file.size <= TEXT_LIMIT) {
      setLoading(true);
      fetch(url, { credentials: 'include' })
        .then((r) => r.text())
        .then(setText)
        .finally(() => setLoading(false));
    }
  }, [kind, file.size, url]);

  return (
    <div
      role="dialog"
      aria-modal="true"
      style={{
        position: 'fixed',
        top: 0,
        right: 0,
        width: 'min(900px, 92vw)',
        height: '100vh',
        background: 'var(--ar-bg)',
        borderLeft: '1px solid var(--ar-border)',
        boxShadow: '-8px 0 32px rgba(0,0,0,0.2)',
        zIndex: 50,
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      <header
        style={{
          padding: '12px 16px',
          borderBottom: '1px solid var(--ar-border)',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
        }}
      >
        <strong className="ar-truncate">{file.name || file.path}</strong>
        <div style={{ display: 'flex', gap: 8 }}>
          <a href={url} download>Скачать</a>
          <button onClick={onClose}>✕</button>
        </div>
      </header>
      <div style={{ flex: 1, overflow: 'auto', padding: 16 }}>
        {kind === 'image' && <img src={url} alt={file.name} style={{ maxWidth: '100%' }} />}
        {kind === 'video' && <video src={url} controls style={{ maxWidth: '100%' }} />}
        {kind === 'audio' && <audio src={url} controls style={{ width: '100%' }} />}
        {kind === 'pdf' && <iframe src={url} title={file.name} style={{ width: '100%', height: '100%', border: 0 }} />}
        {(kind === 'text' || kind === 'json' || kind === 'markdown') &&
          (loading ? (
            <div>Loading…</div>
          ) : text === null ? (
            <div>
              Файл слишком большой ({Math.round(file.size / 1024 / 1024)} MB).{' '}
              <a href={url} download>
                Скачать
              </a>
            </div>
          ) : (
            <pre
              style={{
                margin: 0,
                padding: 12,
                fontFamily: 'var(--ar-mono-font)',
                fontSize: 13,
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word',
              }}
            >
              {kind === 'json' ? safeFormatJSON(text) : text}
            </pre>
          ))}
        {kind === 'binary' && (
          <div>
            <p>
              Бинарный файл — предпросмотр недоступен. Размер: {file.size} байт.
              <br />
              SHA-256: <code>{file.sha256 || '—'}</code>
            </p>
            <a href={url} download>Скачать</a>
          </div>
        )}
      </div>
    </div>
  );
}

function safeFormatJSON(s: string): string {
  try {
    return JSON.stringify(JSON.parse(s), null, 2);
  } catch {
    return s;
  }
}

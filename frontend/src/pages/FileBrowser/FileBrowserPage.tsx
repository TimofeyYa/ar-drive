import { useCallback, useMemo, useRef, useState } from 'react';
import { useParams, useSearchParams, Link } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { api, FileEntry, FolderEntry } from '@shared/api/client';
import { PreviewDrawer } from '@features/preview/PreviewDrawer';
import { ShareLinkModal } from '@features/share-link/ShareLinkModal';

function joinPath(prefix: string, child: string) {
  return (prefix.endsWith('/') ? prefix : prefix + '/') + child;
}
function basename(p: string) {
  const t = p.endsWith('/') ? p.slice(0, -1) : p;
  return t.split('/').pop() || t;
}

export function FileBrowserPage() {
  const { t } = useTranslation();
  const { id: registryID = '' } = useParams();
  const [sp, setSp] = useSearchParams();
  const path = sp.get('path') || '';
  const qc = useQueryClient();

  const [previewing, setPreviewing] = useState<FileEntry | null>(null);
  const [sharingFile, setSharingFile] = useState<FileEntry | null>(null);
  const [uploads, setUploads] = useState<Record<string, { loaded: number; total: number }>>({});
  const dropRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['files', registryID, path],
    queryFn: () => api.listFiles(registryID, path || undefined),
  });

  const deleteFile = useMutation({
    mutationFn: (p: string) => api.deleteFile(registryID, p),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['files', registryID] }),
  });
  const deleteFolder = useMutation({
    mutationFn: (p: string) => api.deleteFolder(registryID, p),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['files', registryID] }),
  });
  const createFolder = useMutation({
    mutationFn: (p: string) => api.createFolder(registryID, p),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['files', registryID] }),
  });

  const onCrumbClick = (p: string) => setSp(p ? { path: p } : {});

  const crumbs = useMemo(() => {
    const parts = path.split('/').filter(Boolean);
    const out: { label: string; path: string }[] = [{ label: 'root', path: '' }];
    let acc = '';
    for (const p of parts) {
      acc = acc ? `${acc}/${p}` : p;
      out.push({ label: p, path: acc });
    }
    return out;
  }, [path]);

  const onUpload = useCallback(
    async (files: FileList | File[]) => {
      const list = Array.from(files);
      for (const f of list) {
        const id = `${f.name}-${Date.now()}`;
        setUploads((u) => ({ ...u, [id]: { loaded: 0, total: f.size } }));
        try {
          await api.uploadFile(registryID, f, path, (loaded, total) =>
            setUploads((u) => ({ ...u, [id]: { loaded, total } })),
          );
        } catch (e) {
          alert('Upload failed: ' + String(e));
        } finally {
          setUploads((u) => {
            const { [id]: _omit, ...rest } = u;
            return rest;
          });
        }
      }
      refetch();
    },
    [registryID, path, refetch],
  );

  const onDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      if (e.dataTransfer.files?.length) {
        onUpload(e.dataTransfer.files);
      }
    },
    [onUpload],
  );

  const copyLink = useCallback(
    async (f: FileEntry) => {
      const url = window.location.origin + api.downloadURL(registryID, f.path);
      await navigator.clipboard.writeText(url);
      alert('Ссылка скопирована');
    },
    [registryID],
  );

  return (
    <div className="ar-fb">
      <aside className="ar-fb__tree">
        <FolderTree
          registryID={registryID}
          currentPath={path}
          onSelect={(p) => onCrumbClick(p)}
          folders={data?.folders ?? []}
        />
      </aside>

      <section className="ar-fb__main">
        <nav aria-label="breadcrumbs" className="ar-mono">
          {crumbs.map((c, i) => (
            <span key={c.path}>
              {i > 0 && ' / '}
              <Link to={`?path=${encodeURIComponent(c.path)}`}>{c.label}</Link>
            </span>
          ))}
        </nav>

        <div className="ar-fb__toolbar">
          <button onClick={() => fileInputRef.current?.click()}>{t('fb.upload')}</button>
          <input
            ref={fileInputRef}
            type="file"
            multiple
            style={{ display: 'none' }}
            onChange={(e) => e.target.files && onUpload(e.target.files)}
          />
          <button
            onClick={() => {
              const name = prompt('Имя новой папки');
              if (name && registryID) createFolder.mutate(joinPath(path, name));
            }}
          >
            {t('fb.newFolder')}
          </button>
          <button onClick={() => refetch()}>{t('fb.refresh')}</button>
        </div>

        {Object.entries(uploads).map(([id, p]) => (
          <div key={id} style={{ fontSize: 12 }}>
            {id}: {Math.round((p.loaded / p.total) * 100)}%
          </div>
        ))}

        <div
          ref={dropRef}
          className="ar-fb__list"
          onDragOver={(e) => e.preventDefault()}
          onDrop={onDrop}
        >
          {isLoading && <div className="ar-empty">…</div>}
          {!isLoading && (data?.folders?.length ?? 0) === 0 && (data?.files?.length ?? 0) === 0 && (
            <div className="ar-empty">{t('fb.emptyFolder')}</div>
          )}
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ textAlign: 'left', opacity: 0.7, fontSize: 12 }}>
                <th style={{ padding: 8 }}>Имя</th>
                <th style={{ padding: 8 }}>Размер</th>
                <th style={{ padding: 8 }}>Изменён</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {(data?.folders ?? []).map((f: FolderEntry) => (
                <tr key={'fld-' + f.path} style={{ borderTop: '1px solid var(--ar-border)' }}>
                  <td style={{ padding: 8 }}>
                    📁{' '}
                    <Link to={`?path=${encodeURIComponent(f.path.replace(/\/$/, ''))}`}>
                      {basename(f.path)}
                    </Link>
                    {f.virtual && (
                      <span title="Виртуальная (пустая) папка" style={{ marginLeft: 8, opacity: 0.5 }}>
                        ✦
                      </span>
                    )}
                  </td>
                  <td />
                  <td />
                  <td style={{ padding: 8, textAlign: 'right' }}>
                    <button
                      onClick={() => {
                        if (confirm(t('fb.confirmDelete'))) deleteFolder.mutate(f.path);
                      }}
                    >
                      {t('fb.delete')}
                    </button>
                  </td>
                </tr>
              ))}
              {(data?.files ?? []).map((f) => (
                <tr key={'fl-' + f.path} style={{ borderTop: '1px solid var(--ar-border)' }}>
                  <td style={{ padding: 8 }}>
                    📄{' '}
                    <button onClick={() => setPreviewing(f)} style={{ background: 'none', border: 'none', color: 'inherit', cursor: 'pointer', textDecoration: 'underline' }}>
                      {f.name || basename(f.path)}
                    </button>
                  </td>
                  <td style={{ padding: 8 }}>{formatSize(f.size)}</td>
                  <td style={{ padding: 8 }}>{new Date(f.updatedAt).toLocaleString()}</td>
                  <td style={{ padding: 8, textAlign: 'right', whiteSpace: 'nowrap' }}>
                    <a href={api.downloadURL(registryID, f.path)} download>
                      {t('fb.download')}
                    </a>{' '}
                    <button onClick={() => copyLink(f)}>🔗</button>{' '}
                    <button onClick={() => setSharingFile(f)}>{t('fb.copyLink')}</button>{' '}
                    <button
                      onClick={() => {
                        if (confirm(t('fb.confirmDelete'))) deleteFile.mutate(f.path);
                      }}
                    >
                      {t('fb.delete')}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {previewing && (
        <PreviewDrawer
          file={previewing}
          registryID={registryID}
          onClose={() => setPreviewing(null)}
        />
      )}
      {sharingFile && (
        <ShareLinkModal
          file={sharingFile}
          registryID={registryID}
          onClose={() => setSharingFile(null)}
        />
      )}
    </div>
  );
}

function FolderTree({
  currentPath,
  folders,
  onSelect,
}: {
  registryID: string;
  currentPath: string;
  onSelect: (p: string) => void;
  folders: FolderEntry[];
}) {
  return (
    <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
      <li>
        <button onClick={() => onSelect('')} style={{ fontWeight: currentPath === '' ? 700 : 400 }}>
          📂 root
        </button>
      </li>
      {folders.map((f) => (
        <li key={f.path} style={{ marginLeft: 16 }}>
          <button onClick={() => onSelect(f.path.replace(/\/$/, ''))}>📁 {basename(f.path)}</button>
        </li>
      ))}
    </ul>
  );
}

function formatSize(bytes: number): string {
  if (!bytes) return '—';
  const units = ['B', 'KB', 'MB', 'GB'];
  let n = bytes;
  let i = 0;
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024;
    i++;
  }
  return `${n.toFixed(n >= 100 || i === 0 ? 0 : 1)} ${units[i]}`;
}

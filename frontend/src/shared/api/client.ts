// HTTP-клиент к backend AR Drive. Учётные данные Cloud.ru передаются в заголовках
// X-Cloudru-Client-Id / X-Cloudru-Client-Secret / X-Cloudru-Project-Id.
// Это упрощает MVP — backend не хранит ключи между сессиями.
import { useCredentialsStore } from '@shared/store/credentials';

const BASE = (import.meta.env.VITE_API_BASE_URL as string) || '/api/v1';

function authHeaders(): Record<string, string> {
  const c = useCredentialsStore.getState().creds;
  if (!c) return {};
  return {
    'X-Cloudru-Client-Id': c.clientId,
    'X-Cloudru-Client-Secret': c.clientSecret,
    'X-Cloudru-Project-Id': c.projectId,
  };
}

export class APIError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(BASE + path, {
    ...init,
    headers: {
      Accept: 'application/json',
      ...(init.body && !(init.body instanceof FormData) ? { 'Content-Type': 'application/json' } : {}),
      ...authHeaders(),
      ...(init.headers || {}),
    },
  });
  if (!res.ok) {
    let body: { error?: string; code?: string } = {};
    try {
      body = await res.json();
    } catch {
      /* ignore */
    }
    throw new APIError(res.status, body.code || 'http_error', body.error || res.statusText);
  }
  if (res.status === 204) return undefined as unknown as T;
  const ct = res.headers.get('Content-Type') || '';
  if (ct.includes('application/json')) return (await res.json()) as T;
  return (await res.text()) as unknown as T;
}

// --- Types ---
export type Registry = {
  id: string;
  name: string;
  projectId: string;
  registryType: 'DOCKER' | 'DEBIAN' | 'RPM' | 'GENERIC';
  status: 'CREATING' | 'ACTIVE' | 'ERROR';
  isPublic: boolean;
  tariff: 'BASIC' | 'PREMIUM';
  createdAt: string;
  updatedAt: string;
};

export type FileEntry = {
  path: string;
  name: string;
  size: number;
  sha256?: string;
  contentType?: string;
  updatedAt: string;
};

export type FolderEntry = { path: string; virtual: boolean };

export type ShareItem = {
  short_id: string;
  url: string;
  project_id: string;
  registry_id: string;
  file_path: string;
  has_password: boolean;
  max_downloads?: number;
  downloads: number;
  expires_at?: string;
  revoked: boolean;
  created_at: string;
};

// --- API methods ---
export const api = {
  verify: (c: { client_id: string; client_secret: string; project_id: string }) =>
    request<{ ok: boolean; user_key: string }>('/auth/verify', {
      method: 'POST',
      body: JSON.stringify(c),
    }),

  listRegistries: (pageToken?: string) =>
    request<{ items: Registry[]; nextPageToken?: string }>(
      `/registries${pageToken ? `?pageToken=${encodeURIComponent(pageToken)}` : ''}`,
    ),

  listFiles: (registryID: string, path?: string) =>
    request<{ files: FileEntry[]; folders: FolderEntry[]; nextPageToken?: string }>(
      `/registries/${encodeURIComponent(registryID)}/files${path ? `?path=${encodeURIComponent(path)}` : ''}`,
    ),

  uploadFile: (registryID: string, file: File, path: string, onProgress?: (loaded: number, total: number) => void) =>
    uploadXHR(registryID, file, path, onProgress),

  deleteFile: (registryID: string, path: string) =>
    request<void>(`/registries/${encodeURIComponent(registryID)}/files?path=${encodeURIComponent(path)}`, {
      method: 'DELETE',
    }),

  downloadURL: (registryID: string, path: string) =>
    `${BASE}/registries/${encodeURIComponent(registryID)}/files/download?path=${encodeURIComponent(path)}`,

  createFolder: (registryID: string, path: string) =>
    request<{ path: string; virtual: boolean }>(`/registries/${encodeURIComponent(registryID)}/folders`, {
      method: 'POST',
      body: JSON.stringify({ path }),
    }),

  deleteFolder: (registryID: string, path: string) =>
    request<{ deleted_files: number; path: string }>(
      `/registries/${encodeURIComponent(registryID)}/folders?path=${encodeURIComponent(path)}`,
      { method: 'DELETE' },
    ),

  listShares: () => request<{ items: ShareItem[] }>('/shares'),
  createShare: (input: {
    registry_id: string;
    file_path: string;
    password?: string;
    ttl_seconds?: number;
    max_downloads?: number;
  }) => request<ShareItem>('/shares', { method: 'POST', body: JSON.stringify(input) }),
  revokeShare: (shortID: string) =>
    request<void>(`/shares/${encodeURIComponent(shortID)}`, { method: 'DELETE' }),
};

function uploadXHR(
  registryID: string,
  file: File,
  path: string,
  onProgress?: (loaded: number, total: number) => void,
): Promise<FileEntry> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    const form = new FormData();
    form.append('file', file);
    form.append('path', path);
    xhr.open('POST', `${BASE}/registries/${encodeURIComponent(registryID)}/files`);
    const headers = authHeaders();
    for (const [k, v] of Object.entries(headers)) xhr.setRequestHeader(k, v);
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && onProgress) onProgress(e.loaded, e.total);
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          resolve(JSON.parse(xhr.responseText));
        } catch {
          resolve({ path, name: file.name, size: file.size, updatedAt: new Date().toISOString() });
        }
      } else {
        let msg = xhr.responseText;
        try {
          msg = JSON.parse(xhr.responseText).error;
        } catch {
          /* ignore */
        }
        reject(new APIError(xhr.status, 'upload_failed', msg || 'Upload failed'));
      }
    };
    xhr.onerror = () => reject(new APIError(0, 'network', 'Network error'));
    xhr.send(form);
  });
}

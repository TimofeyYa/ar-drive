// AES-GCM-256 для шифрования учётных данных в localStorage.
// Ключ деривируется PBKDF2-SHA256 (200 000 итераций) из парольной фразы +
// device-salt (хранится в localStorage как открытое значение).
//
// TS 5.7+ ужесточил типы Uint8Array<ArrayBufferLike> — WebCrypto ожидает
// строго ArrayBuffer (а не SharedArrayBuffer). Поэтому везде, где байты
// уходят в crypto.subtle, явно нормализуем буфер через toArrayBuffer().

const SALT_KEY = 'ar-drive.salt';
const ITERATIONS = 200_000;

/** Приводит ArrayBufferLike к чистому ArrayBuffer (копированием). */
function toArrayBuffer(view: ArrayBufferView | ArrayBufferLike): ArrayBuffer {
  if (view instanceof ArrayBuffer) return view;
  if (ArrayBuffer.isView(view)) {
    const out = new ArrayBuffer(view.byteLength);
    new Uint8Array(out).set(new Uint8Array(view.buffer, view.byteOffset, view.byteLength));
    return out;
  }
  // SharedArrayBuffer и т.п.
  const out = new ArrayBuffer((view as ArrayBufferLike).byteLength);
  new Uint8Array(out).set(new Uint8Array(view as ArrayBufferLike));
  return out;
}

function b64encode(buf: ArrayBufferLike): string {
  return btoa(String.fromCharCode(...new Uint8Array(toArrayBuffer(buf))));
}
function b64decode(s: string): ArrayBuffer {
  return toArrayBuffer(Uint8Array.from(atob(s), (c) => c.charCodeAt(0)));
}

function getOrCreateSalt(): ArrayBuffer {
  const stored = localStorage.getItem(SALT_KEY);
  if (stored) return b64decode(stored);
  const saltBytes = crypto.getRandomValues(new Uint8Array(16));
  const salt = toArrayBuffer(saltBytes);
  localStorage.setItem(SALT_KEY, b64encode(salt));
  return salt;
}

async function deriveKey(passphrase: string): Promise<CryptoKey> {
  const enc = new TextEncoder();
  const passBytes = toArrayBuffer(
    enc.encode(passphrase || `${navigator.userAgent}::ar-drive-default`),
  );
  const baseKey = await crypto.subtle.importKey('raw', passBytes, 'PBKDF2', false, ['deriveKey']);
  return crypto.subtle.deriveKey(
    {
      name: 'PBKDF2',
      salt: getOrCreateSalt(),
      iterations: ITERATIONS,
      hash: 'SHA-256',
    },
    baseKey,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt', 'decrypt'],
  );
}

export async function encryptJSON(value: unknown, passphrase: string): Promise<string> {
  const key = await deriveKey(passphrase);
  const ivBytes = crypto.getRandomValues(new Uint8Array(12));
  const iv = toArrayBuffer(ivBytes);
  const data = toArrayBuffer(new TextEncoder().encode(JSON.stringify(value)));
  const ct = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, key, data);
  // Формат: base64(iv) + "." + base64(ct)
  return `${b64encode(iv)}.${b64encode(ct)}`;
}

export async function decryptJSON<T>(payload: string, passphrase: string): Promise<T> {
  const [ivB64, ctB64] = payload.split('.');
  if (!ivB64 || !ctB64) throw new Error('invalid encrypted payload');
  const key = await deriveKey(passphrase);
  const pt = await crypto.subtle.decrypt(
    { name: 'AES-GCM', iv: b64decode(ivB64) },
    key,
    b64decode(ctB64),
  );
  return JSON.parse(new TextDecoder().decode(pt)) as T;
}
